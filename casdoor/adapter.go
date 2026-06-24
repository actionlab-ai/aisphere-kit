package casdoor

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/actionlab-ai/aisphere-kit/audit"
	"github.com/actionlab-ai/aisphere-kit/authz"
	"github.com/actionlab-ai/aisphere-kit/metrics"
	"github.com/actionlab-ai/aisphere-kit/permission"
	"github.com/actionlab-ai/aisphere-kit/principal"
	"github.com/actionlab-ai/aisphere-kit/retry"
	casdoorsdk "github.com/casdoor/casdoor-go-sdk/casdoorsdk"
)

var (
	_ permission.Manager = (*Adapter)(nil)
)

type Adapter struct {
	cfg        Config
	client     *casdoorsdk.Client
	retry      retry.Policy
	logger     *slog.Logger
	metrics    *metrics.Metrics
	timeout    time.Duration
	httpClient *http.Client
}

type Option func(*options)
type options struct {
	logger  *slog.Logger
	metrics *metrics.Metrics
}

func WithLogger(l *slog.Logger) Option      { return func(o *options) { o.logger = l } }
func WithMetrics(m *metrics.Metrics) Option { return func(o *options) { o.metrics = m } }

func New(cfg Config, opts ...Option) *Adapter {
	var opt options
	for _, fn := range opts {
		if fn != nil {
			fn(&opt)
		}
	}
	l := opt.logger
	if l == nil {
		l = slog.Default()
	}
	l = l.With("component", "casdoor", "endpoint", cfg.Endpoint, "organization", cfg.Organization, "application", cfg.Application)
	timeout := 10 * time.Second
	if cfg.HTTPTimeout != "" {
		if d, err := time.ParseDuration(cfg.HTTPTimeout); err == nil && d > 0 {
			timeout = d
		} else if err != nil {
			l.Warn("casdoor http_timeout parse failed; using default", "value", cfg.HTTPTimeout, "error", err)
		}
	}
	// Casdoor SDK exposes SetHttpClient as a package-level setting. Configure it
	// once during adapter construction so SDK HTTP calls have a real network
	// timeout instead of relying only on the outer context wrapper.
	httpClient := &http.Client{Timeout: timeout}
	casdoorsdk.SetHttpClient(httpClient)
	l.Info("casdoor adapter creating", "client_id", cfg.ClientID, "allow_anonymous", cfg.AllowAnonymous, "http_timeout", timeout.String())
	return &Adapter{cfg: cfg, client: casdoorsdk.NewClient(cfg.Endpoint, cfg.ClientID, cfg.ClientSecret, cfg.Certificate, cfg.Organization, cfg.Application), retry: retry.NewPolicy(retry.Config{Attempts: cfg.RetryAttempts, Backoff: cfg.RetryBackoff}), logger: l, metrics: opt.metrics, timeout: timeout, httpClient: httpClient}
}

func (a *Adapter) Client() *casdoorsdk.Client { return a.client }

func (a *Adapter) Ping(ctx context.Context) error {
	started := time.Now()
	a.logger.Debug("casdoor ping started")
	err := callWithContext(ctx, func() error {
		_, err := a.client.GetApplication(a.cfg.Application)
		return err
	})
	if a.metrics != nil {
		a.metrics.ObserveDependency("casdoor", "ping", started, err)
	}
	if err != nil {
		a.logger.Warn("casdoor ping failed", "error", err, "elapsed", time.Since(started).String())
	} else {
		a.logger.Debug("casdoor ping ok", "elapsed", time.Since(started).String())
	}
	return err
}

func (a *Adapter) Authenticate(ctx context.Context, token string) (*principal.Principal, error) {
	started := time.Now()
	a.logger.Debug("casdoor authenticate started", "token_present", token != "")
	claims, err := a.client.ParseJwtToken(token)
	if a.metrics != nil {
		a.metrics.ObserveDependency("casdoor", "parse_jwt", started, err)
	}
	if err != nil {
		a.logger.Warn("casdoor authenticate failed", "error", err, "elapsed", time.Since(started).String())
		return nil, err
	}
	select {
	case <-ctx.Done():
		a.logger.Warn("casdoor authenticate context done", "error", ctx.Err())
		return nil, ctx.Err()
	default:
	}
	p := &principal.Principal{SubjectType: principal.SubjectUser, SubjectID: claims.Name, Name: claims.Name, DisplayName: claims.DisplayName, Email: claims.Email, Avatar: claims.Avatar, OrgID: claims.Owner, Token: token, Claims: map[string]any{"issuer": claims.Issuer, "subject": claims.Subject, "audience": fmt.Sprint(claims.Audience)}}
	if p.SubjectID == "" {
		p.SubjectID = claims.Subject
	}
	if p.OrgID == "" {
		p.OrgID = a.cfg.Organization
	}
	a.logger.Info("casdoor authenticate succeeded", "subject", p.Subject(), "org_id", p.OrgID, "elapsed", time.Since(started).String())
	return p, nil
}

func (a *Adapter) Authorize(ctx context.Context, req authz.Request) (bool, error) {
	started := time.Now()
	if req.Principal == nil {
		return false, principal.ErrMissingPrincipal
	}
	l := a.logger.With("subject", req.Principal.Subject(), "resource", req.Resource, "action", req.Action)
	l.Debug("casdoor authorize started")
	permissionID, modelID, resourceID, enforcerID, owner := a.enforceSelectors()
	if permissionID == "" && modelID == "" && resourceID == "" && enforcerID == "" && owner == "" {
		err := fmt.Errorf("one of casdoor permission_id/model_id/resource_id/enforcer_id/owner is required")
		l.Error("casdoor authorize config invalid", "error", err)
		return false, err
	}
	casbinReq := casdoorsdk.CasbinRequest{req.Principal.Subject(), req.Resource, req.Action}
	if req.Domain != "" {
		casbinReq = casdoorsdk.CasbinRequest{req.Principal.Subject(), req.Domain, req.Resource, req.Action}
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
		a.metrics.ObserveDependency("casdoor", "enforce", started, err)
	}
	if err != nil {
		l.Warn("casdoor authorize failed", "error", err, "elapsed", time.Since(started).String())
		return false, err
	}
	if !allowed {
		l.Warn("casdoor authorize denied", "elapsed", time.Since(started).String())
	} else {
		l.Debug("casdoor authorize allowed", "elapsed", time.Since(started).String())
	}
	return allowed, nil
}

func (a *Adapter) Record(ctx context.Context, event audit.Event) error {
	started := time.Now()
	actor := "anonymous"
	org := a.cfg.Organization
	if event.Actor != nil {
		actor = event.Actor.SubjectID
		if event.Actor.OrgID != "" {
			org = event.Actor.OrgID
		}
	}
	name := event.Name
	if name == "" {
		name = randomName("record")
	}
	objectBytes, err := json.Marshal(event)
	if err != nil {
		a.logger.Warn("casdoor audit event marshal failed", "error", err)
		objectBytes = []byte(`{}`)
	}
	record := &casdoorsdk.Record{Owner: a.cfg.Organization, Name: name, CreatedTime: time.Now().Format(time.RFC3339), Organization: org, ClientIp: event.ClientIP, User: actor, Method: event.Method, RequestUri: firstNonEmpty(event.URI, event.Operation), Action: event.Action, StatusCode: event.StatusCode, Response: event.Message, Object: string(objectBytes), IsTriggered: true}
	l := a.logger.With("audit_name", name, "actor", actor, "action", event.Action, "resource", event.Resource, "result", event.Result)
	l.Debug("casdoor audit record started")
	err = retry.Do(ctx, a.retry, func() error {
		return callWithContext(ctx, func() error { _, err := a.client.AddRecord(record); return err })
	})
	if a.metrics != nil {
		a.metrics.ObserveDependency("casdoor", "add_record", started, err)
	}
	if err != nil {
		l.Warn("casdoor audit record failed", "error", err, "elapsed", time.Since(started).String())
	} else {
		l.Debug("casdoor audit record completed", "elapsed", time.Since(started).String())
	}
	return err
}

func callWithContext(ctx context.Context, fn func() error) error {
	if ctx == nil {
		return fn()
	}
	done := make(chan error, 1)
	go func() { done <- fn() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func randomName(prefix string) string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
	}
	return prefix + "_" + hex.EncodeToString(b)
}
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func (a *Adapter) enforceSelectors() (permissionID, modelID, resourceID, enforcerID, owner string) {
	// Casdoor /api/enforce expects exactly one selector. The SDK signature accepts
	// all selector slots, so normalize our config to a single active selector.
	// Priority is intentionally permission_id first because it is the most explicit
	// and safest selector for production authorization.
	if a.cfg.PermissionID != "" {
		return a.cfg.PermissionID, "", "", "", ""
	}
	if a.cfg.ModelID != "" {
		return "", a.cfg.ModelID, "", "", ""
	}
	if a.cfg.ResourceID != "" {
		return "", "", a.cfg.ResourceID, "", ""
	}
	if a.cfg.EnforcerID != "" {
		return "", "", "", a.cfg.EnforcerID, ""
	}
	if a.cfg.Owner != "" {
		return "", "", "", "", a.cfg.Owner
	}
	return "", "", "", "", ""
}
