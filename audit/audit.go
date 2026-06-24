package audit

import (
	"context"
	"fmt"
	"time"

	"github.com/actionlab-ai/aisphere-kit/logx"
	"github.com/actionlab-ai/aisphere-kit/principal"
)

const (
	ResultSuccess = "success"
	ResultFailed  = "failed"
)

type Event struct {
	Name       string               `json:"name"`
	Actor      *principal.Principal `json:"actor,omitempty"`
	Action     string               `json:"action"`
	Resource   string               `json:"resource"`
	Result     string               `json:"result"`
	Message    string               `json:"message,omitempty"`
	RequestID  string               `json:"request_id,omitempty"`
	TraceID    string               `json:"trace_id,omitempty"`
	ClientIP   string               `json:"client_ip,omitempty"`
	UserAgent  string               `json:"user_agent,omitempty"`
	Method     string               `json:"method,omitempty"`
	URI        string               `json:"uri,omitempty"`
	Operation  string               `json:"operation,omitempty"`
	StatusCode int                  `json:"status_code,omitempty"`
	Component  string               `json:"component,omitempty"`
	OrgID      string               `json:"org_id,omitempty"`
	ProjectID  string               `json:"project_id,omitempty"`
	StartedAt  time.Time            `json:"started_at"`
	FinishedAt time.Time            `json:"finished_at"`
	Duration   time.Duration        `json:"duration"`
	Metadata   map[string]string    `json:"metadata,omitempty"`
}

type Recorder interface {
	Record(ctx context.Context, event Event) error
}

type Resolver func(ctx context.Context, req any, reply any, err error) Event

type Options struct {
	Component    string
	Resolver     Resolver
	Async        bool
	AsyncLimit   int
	AsyncTimeout time.Duration
}

func Normalize(ctx context.Context, base Event, err error) Event {
	if base.StartedAt.IsZero() {
		base.StartedAt = time.Now()
	}
	if base.FinishedAt.IsZero() {
		base.FinishedAt = time.Now()
	}
	base.Duration = base.FinishedAt.Sub(base.StartedAt)
	if base.Result == "" {
		if err != nil {
			base.Result = ResultFailed
		} else {
			base.Result = ResultSuccess
		}
	}
	if err != nil {
		if base.StatusCode == 0 {
			base.StatusCode = 500
		}
		if base.Message == "" {
			base.Message = err.Error()
		}
	} else if base.StatusCode == 0 {
		base.StatusCode = 200
	}
	if p, ok := principal.FromContext(ctx); ok {
		base.Actor = p
		if base.OrgID == "" {
			base.OrgID = p.OrgID
		}
		if base.ProjectID == "" {
			base.ProjectID = p.ProjectID
		}
	}
	if base.Name == "" {
		base.Name = fmt.Sprintf("%s:%s", firstNonEmpty(base.Action, base.Operation), base.Resource)
	}
	return base
}

func Record(ctx context.Context, r Recorder, ev Event) error {
	if r == nil {
		return nil
	}
	logx.FromContext(ctx).Debug("audit record started", "action", ev.Action, "resource", ev.Resource, "result", ev.Result)
	err := r.Record(ctx, ev)
	if err != nil {
		logx.FromContext(ctx).Warn("audit record failed", "error", err, "action", ev.Action, "resource", ev.Resource)
	} else {
		logx.FromContext(ctx).Debug("audit record completed", "action", ev.Action, "resource", ev.Resource)
	}
	return err
}

func RecordAsync(ctx context.Context, r Recorder, ev Event, limit chan struct{}, timeout time.Duration) {
	if r == nil {
		return
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	if limit != nil {
		select {
		case limit <- struct{}{}:
			defer func() { <-limit }()
		default:
			logx.FromContext(ctx).Warn("audit queue full; dropping event", "action", ev.Action, "resource", ev.Resource, "async_limit", cap(limit))
			return
		}
	}
	baseCtx := context.WithoutCancel(ctx)
	rctx, cancel := context.WithTimeout(baseCtx, timeout)
	defer cancel()
	logx.FromContext(ctx).Debug("audit async record started", "action", ev.Action, "resource", ev.Resource)
	if err := r.Record(rctx, ev); err != nil {
		logx.FromContext(ctx).Warn("audit async record failed", "error", err, "action", ev.Action, "resource", ev.Resource)
	} else {
		logx.FromContext(ctx).Debug("audit async record completed", "action", ev.Action, "resource", ev.Resource)
	}
}

func Merge(base, override Event) Event {
	if override.Name != "" {
		base.Name = override.Name
	}
	if override.Actor != nil {
		base.Actor = override.Actor
	}
	if override.Action != "" {
		base.Action = override.Action
	}
	if override.Resource != "" {
		base.Resource = override.Resource
	}
	if override.Result != "" {
		base.Result = override.Result
	}
	if override.Message != "" {
		base.Message = override.Message
	}
	if override.RequestID != "" {
		base.RequestID = override.RequestID
	}
	if override.TraceID != "" {
		base.TraceID = override.TraceID
	}
	if override.ClientIP != "" {
		base.ClientIP = override.ClientIP
	}
	if override.UserAgent != "" {
		base.UserAgent = override.UserAgent
	}
	if override.Method != "" {
		base.Method = override.Method
	}
	if override.URI != "" {
		base.URI = override.URI
	}
	if override.Operation != "" {
		base.Operation = override.Operation
	}
	if override.StatusCode != 0 {
		base.StatusCode = override.StatusCode
	}
	if override.Component != "" {
		base.Component = override.Component
	}
	if override.OrgID != "" {
		base.OrgID = override.OrgID
	}
	if override.ProjectID != "" {
		base.ProjectID = override.ProjectID
	}
	if !override.StartedAt.IsZero() {
		base.StartedAt = override.StartedAt
	}
	if !override.FinishedAt.IsZero() {
		base.FinishedAt = override.FinishedAt
	}
	if override.Duration != 0 {
		base.Duration = override.Duration
	}
	if override.Metadata != nil {
		base.Metadata = override.Metadata
	}
	return base
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
