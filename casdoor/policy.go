package casdoor

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/actionlab-ai/aisphere-kit/permission"
	"github.com/actionlab-ai/aisphere-kit/resource"
	"github.com/actionlab-ai/aisphere-kit/retry"
	casdoorsdk "github.com/casdoor/casdoor-go-sdk/casdoorsdk"
)

func (a *Adapter) Grant(ctx context.Context, grant permission.Grant) error {
	g, err := permission.NormalizeGrant(grant)
	if err != nil {
		return err
	}
	if g.Action == "" {
		return a.GrantRole(ctx, g)
	}
	return a.addPolicy(ctx, g.Subject, g.Resource.String(), g.Action)
}

func (a *Adapter) GrantRole(ctx context.Context, grant permission.Grant) error {
	g, err := permission.NormalizeGrant(grant)
	if err != nil {
		return err
	}
	actions, err := permission.RoleActions(g.Role, g.Actions)
	if err != nil {
		return err
	}
	for _, act := range actions {
		if err := a.addPolicy(ctx, g.Subject, g.Resource.String(), act); err != nil {
			return err
		}
	}
	return nil
}

func (a *Adapter) Share(ctx context.Context, req permission.ShareRequest) error {
	g, err := permission.NormalizeShare(req)
	if err != nil {
		return err
	}
	return a.GrantRole(ctx, g)
}

func (a *Adapter) Revoke(ctx context.Context, grant permission.Grant) error {
	g, err := permission.NormalizeGrant(grant)
	if err != nil {
		return err
	}
	actions := []string{g.Action}
	if g.Action == "" {
		actions, err = permission.RoleActions(g.Role, g.Actions)
		if err != nil {
			return err
		}
	}
	for _, act := range actions {
		if err := a.removePolicy(ctx, g.Subject, g.Resource.String(), act); err != nil {
			return err
		}
	}
	return nil
}

func (a *Adapter) Check(ctx context.Context, req permission.CheckRequest) (bool, error) {
	subject := strings.TrimSpace(req.Subject)
	if subject == "" && req.Principal != nil {
		subject = permission.SubjectFromPrincipal(req.Principal)
	}
	if subject == "" {
		return false, permission.ErrEmptySubject
	}
	if req.Resource.String() == "" {
		return false, permission.ErrEmptyResource
	}
	if req.Action == "" {
		return false, permission.ErrEmptyAction
	}

	started := time.Now()
	l := a.logger.With("subject", subject, "resource", req.Resource.String(), "action", req.Action)
	l.Debug("casdoor permission check started")
	permissionID, modelID, resourceID, enforcerID, owner := a.enforceSelectors()
	casbinReq := casdoorsdk.CasbinRequest{subject, req.Resource.String(), req.Action}
	if req.Domain != "" {
		casbinReq = casdoorsdk.CasbinRequest{subject, req.Domain, req.Resource.String(), req.Action}
	}
	if len(req.Extra) > 0 {
		casbinReq = append(casbinReq, req.Extra...)
	}
	var allowed bool
	err := retry.Do(ctx, a.retry, func() error {
		return callWithContext(ctx, func() error {
			v, err := a.client.Enforce(permissionID, modelID, resourceID, enforcerID, owner, casbinReq)
			allowed = v
			return err
		})
	})
	if a.metrics != nil {
		a.metrics.ObserveDependency("casdoor", "permission_check", started, err)
	}
	if err != nil {
		l.Warn("casdoor permission check failed", "error", err, "elapsed", time.Since(started).String())
		return false, err
	}
	l.Debug("casdoor permission check completed", "allowed", allowed, "elapsed", time.Since(started).String())
	return allowed, nil
}

func (a *Adapter) List(ctx context.Context, filter permission.ListFilter) ([]permission.Grant, error) {
	enforcer, adapterID, err := a.policyTarget()
	if err != nil {
		return nil, err
	}
	started := time.Now()
	l := a.logger.With("enforcer", enforcer.Name, "adapter_id", adapterID, "resource", filter.Resource.String())
	l.Debug("casdoor list policies started")
	var rules []*casdoorsdk.CasbinRule
	err = retry.Do(ctx, a.retry, func() error {
		return callWithContext(ctx, func() error {
			var e error
			rules, e = a.client.GetPolicies(enforcer.Name, adapterID)
			return e
		})
	})
	if a.metrics != nil {
		a.metrics.ObserveDependency("casdoor", "get_policies", started, err)
	}
	if err != nil {
		l.Warn("casdoor list policies failed", "error", err, "elapsed", time.Since(started).String())
		return nil, err
	}
	grants := make([]permission.Grant, 0, len(rules))
	for _, r := range rules {
		if r == nil || r.Ptype != "p" {
			continue
		}
		g := permission.Grant{Subject: r.V0, Resource: resource.Name(r.V1), Action: r.V2}
		if filter.Subject != "" && filter.Subject != g.Subject {
			continue
		}
		if filter.Resource.String() != "" && filter.Resource != g.Resource {
			continue
		}
		if filter.Action != "" && filter.Action != g.Action {
			continue
		}
		grants = append(grants, g)
	}
	l.Debug("casdoor list policies completed", "matched", len(grants), "elapsed", time.Since(started).String())
	return grants, nil
}

func (a *Adapter) DeleteResourcePolicies(ctx context.Context, res resource.Name) error {
	return a.DeleteResourcePoliciesEx(ctx, permission.DeleteResourceRequest{Resource: res})
}

func (a *Adapter) DeleteResourcePoliciesEx(ctx context.Context, req permission.DeleteResourceRequest) error {
	if req.Resource.String() == "" {
		return permission.ErrEmptyResource
	}
	grants, err := a.List(ctx, permission.ListFilter{Resource: req.Resource})
	if err != nil {
		return err
	}
	l := a.logger.With("resource", req.Resource.String(), "policy_count", len(grants), "reason", req.Reason, "actor", req.Actor)
	l.Info("casdoor delete resource policies started")
	for _, g := range grants {
		if err := a.removePolicy(ctx, g.Subject, g.Resource.String(), g.Action); err != nil {
			l.Warn("casdoor delete resource policy failed", "subject", g.Subject, "action", g.Action, "error", err)
			return err
		}
	}
	l.Info("casdoor delete resource policies completed")
	return nil
}

func (a *Adapter) addPolicy(ctx context.Context, subject, object, action string) error {
	enforcer, _, err := a.policyTarget()
	if err != nil {
		return err
	}
	rule := &casdoorsdk.CasbinRule{Ptype: "p", V0: subject, V1: object, V2: action}
	started := time.Now()
	l := a.logger.With("subject", subject, "resource", object, "action", action, "enforcer", enforcer.Name)
	l.Debug("casdoor add policy started")
	err = retry.Do(ctx, a.retry, func() error {
		return callWithContext(ctx, func() error {
			ok, err := a.client.AddPolicy(enforcer, rule)
			if err != nil {
				return err
			}
			if !ok {
				l.Debug("casdoor add policy returned false; treating as no-op/idempotent")
			}
			return nil
		})
	})
	if a.metrics != nil {
		a.metrics.ObserveDependency("casdoor", "add_policy", started, err)
	}
	if err != nil {
		l.Warn("casdoor add policy failed", "error", err, "elapsed", time.Since(started).String())
		return err
	}
	l.Info("casdoor add policy completed", "elapsed", time.Since(started).String())
	return nil
}

func (a *Adapter) removePolicy(ctx context.Context, subject, object, action string) error {
	enforcer, _, err := a.policyTarget()
	if err != nil {
		return err
	}
	rule := &casdoorsdk.CasbinRule{Ptype: "p", V0: subject, V1: object, V2: action}
	started := time.Now()
	l := a.logger.With("subject", subject, "resource", object, "action", action, "enforcer", enforcer.Name)
	l.Debug("casdoor remove policy started")
	err = retry.Do(ctx, a.retry, func() error {
		return callWithContext(ctx, func() error {
			ok, err := a.client.RemovePolicy(enforcer, rule)
			if err != nil {
				return err
			}
			if !ok {
				l.Debug("casdoor remove policy returned false; treating as no-op/idempotent")
			}
			return nil
		})
	})
	if a.metrics != nil {
		a.metrics.ObserveDependency("casdoor", "remove_policy", started, err)
	}
	if err != nil {
		l.Warn("casdoor remove policy failed", "error", err, "elapsed", time.Since(started).String())
		return err
	}
	l.Info("casdoor remove policy completed", "elapsed", time.Since(started).String())
	return nil
}

func (a *Adapter) policyTarget() (*casdoorsdk.Enforcer, string, error) {
	name := strings.TrimSpace(firstNonEmpty(a.cfg.PolicyEnforcer, a.cfg.EnforcerID))
	if name == "" {
		return nil, "", fmt.Errorf("casdoor policy_enforcer or enforcer_id is required for policy management")
	}
	adapterID := strings.TrimSpace(a.cfg.PolicyAdapterID)
	return &casdoorsdk.Enforcer{Owner: a.cfg.Organization, Name: name}, adapterID, nil
}

func logPolicyDebug(l *slog.Logger, msg string, args ...any) {
	if l != nil {
		l.Debug(msg, args...)
	}
}
