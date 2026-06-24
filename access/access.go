// Package access provides a component-reusable access guard that combines
// principal lookup, authorization checks, and audit recording.
//
// Business modules should depend on *access.Guard instead of directly wiring
// authz.Authorizer and audit.Recorder in every component. The guard is still
// provider-neutral: Casdoor/Casbin is only one possible implementation behind
// the authz/audit interfaces.
package access

import (
	"context"
	"log/slog"
	"time"

	"github.com/actionlab-ai/aisphere-kit/audit"
	"github.com/actionlab-ai/aisphere-kit/authz"
	"github.com/actionlab-ai/aisphere-kit/principal"
)

const (
	ResultSuccess = audit.ResultSuccess
	ResultFailed  = audit.ResultFailed
)

// Check describes one authorization decision.
type Check struct {
	Resource string
	Action   string
	Domain   string
	Reason   string
	Extra    []any
}

// Event is the business-facing audit event shape. It intentionally mirrors
// audit.Event for common fields but keeps this package as the standard entry
// point for usecases.
type Event struct {
	Name       string
	Action     string
	Resource   string
	Result     string
	Message    string
	RequestID  string
	TraceID    string
	ClientIP   string
	UserAgent  string
	Method     string
	URI        string
	Operation  string
	StatusCode int
	Component  string
	OrgID      string
	ProjectID  string
	StartedAt  time.Time
	FinishedAt time.Time
	Metadata   map[string]string
}

// Options wires access dependencies. Authz and Audit may be nil depending on
// feature flags; Require will fail closed when Authz is nil, while Record is a
// no-op when Audit is nil.
type Options struct {
	Authz     authz.Authorizer
	Audit     audit.Recorder
	Logger    *slog.Logger
	Component string
}

// Guard is the reusable access-control facade for all AI Sphere services.
type Guard struct {
	authz     authz.Authorizer
	audit     audit.Recorder
	logger    *slog.Logger
	component string
}

func NewGuard(opts Options) *Guard {
	return &Guard{
		authz:     opts.Authz,
		audit:     opts.Audit,
		logger:    opts.Logger,
		component: opts.Component,
	}
}

func (g *Guard) Principal(ctx context.Context) (*principal.Principal, error) {
	return principal.RequireFromContext(ctx)
}

func (g *Guard) Can(ctx context.Context, check Check) (bool, error) {
	p, err := principal.RequireFromContext(ctx)
	if err != nil {
		return false, err
	}
	if g == nil || g.authz == nil {
		return false, authz.ErrNotConfigured
	}
	if check.Resource == "" || check.Action == "" {
		return false, authz.ErrEmptyAuthorization
	}
	return g.authz.Authorize(ctx, authz.Request{
		Principal: p,
		Resource:  check.Resource,
		Action:    check.Action,
		Domain:    check.Domain,
		Extra:     check.Extra,
	})
}

func (g *Guard) Require(ctx context.Context, check Check) (*principal.Principal, error) {
	p, err := principal.RequireFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if g == nil || g.authz == nil {
		return nil, authz.ErrNotConfigured
	}
	if check.Resource == "" || check.Action == "" {
		return nil, authz.ErrEmptyAuthorization
	}
	if err := authz.RequirePrincipal(ctx, g.authz, p, check.Resource, check.Action); err != nil {
		return nil, err
	}
	return p, nil
}

func (g *Guard) Record(ctx context.Context, ev Event) error {
	if g == nil || g.audit == nil {
		return nil
	}
	now := time.Now()
	if ev.StartedAt.IsZero() {
		ev.StartedAt = now
	}
	if ev.FinishedAt.IsZero() {
		ev.FinishedAt = now
	}
	aev := audit.Event{
		Name:       ev.Name,
		Action:     ev.Action,
		Resource:   ev.Resource,
		Result:     ev.Result,
		Message:    ev.Message,
		RequestID:  ev.RequestID,
		TraceID:    ev.TraceID,
		ClientIP:   ev.ClientIP,
		UserAgent:  ev.UserAgent,
		Method:     ev.Method,
		URI:        ev.URI,
		Operation:  ev.Operation,
		StatusCode: ev.StatusCode,
		Component:  firstNonEmpty(ev.Component, g.component),
		OrgID:      ev.OrgID,
		ProjectID:  ev.ProjectID,
		StartedAt:  ev.StartedAt,
		FinishedAt: ev.FinishedAt,
		Metadata:   ev.Metadata,
	}
	aev = audit.Normalize(ctx, aev, nil)
	return audit.Record(ctx, g.audit, aev)
}

// RequireAndAudit performs an authorization check and records the result. It is
// useful for admin/debug operations and simple CRUD handlers where deny/success
// audit semantics are identical.
func (g *Guard) RequireAndAudit(ctx context.Context, check Check, ev Event) (*principal.Principal, error) {
	p, err := g.Require(ctx, check)
	if err != nil {
		ev.Result = ResultFailed
		if ev.Message == "" {
			ev.Message = err.Error()
		}
		if ev.StatusCode == 0 {
			ev.StatusCode = 403
		}
		_ = g.Record(ctx, ev)
		return nil, err
	}
	ev.Result = firstNonEmpty(ev.Result, ResultSuccess)
	_ = g.Record(ctx, ev)
	return p, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
