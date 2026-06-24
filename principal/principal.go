package principal

import (
	"context"
	"errors"
)

const (
	SubjectUser    = "user"
	SubjectService = "service"
	SubjectAPIKey  = "api_key"
	SubjectAnon    = "anonymous"
)

type Principal struct {
	SubjectType string            `json:"subject_type"`
	SubjectID   string            `json:"subject_id"`
	Name        string            `json:"name"`
	DisplayName string            `json:"display_name"`
	Email       string            `json:"email"`
	Avatar      string            `json:"avatar"`
	OrgID       string            `json:"org_id"`
	ProjectID   string            `json:"project_id"`
	Roles       []string          `json:"roles"`
	Groups      []string          `json:"groups"`
	Token       string            `json:"-"`
	TokenID     string            `json:"token_id"`
	Claims      map[string]any    `json:"claims,omitempty"`
	Attributes  map[string]string `json:"attributes,omitempty"`
}

func (p Principal) IsZero() bool { return p.SubjectID == "" && p.SubjectType == "" }

func (p Principal) IsAuthenticated() bool {
	return p.SubjectID != "" && p.SubjectType != "" && p.SubjectType != SubjectAnon
}

func (p Principal) Subject() string {
	if p.SubjectType == "" {
		return p.SubjectID
	}
	if p.SubjectID == "" {
		return p.SubjectType
	}
	return p.SubjectType + ":" + p.SubjectID
}

func Anonymous() *Principal { return &Principal{SubjectType: SubjectAnon, SubjectID: "anonymous"} }

var ErrMissingPrincipal = errors.New("principal missing from context")

type contextKey struct{}

func NewContext(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, contextKey{}, p)
}

func FromContext(ctx context.Context) (*Principal, bool) {
	if ctx == nil {
		return nil, false
	}
	p, ok := ctx.Value(contextKey{}).(*Principal)
	return p, ok && p != nil
}

func RequireFromContext(ctx context.Context) (*Principal, error) {
	p, ok := FromContext(ctx)
	if !ok || p == nil || p.IsZero() {
		return nil, ErrMissingPrincipal
	}
	return p, nil
}

func MustFromContext(ctx context.Context) *Principal {
	p, err := RequireFromContext(ctx)
	if err != nil {
		panic(err)
	}
	return p
}
