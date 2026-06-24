package authz

import (
	"context"
	"errors"
	"fmt"

	"github.com/actionlab-ai/aisphere-kit/principal"
)

var (
	ErrDenied             = errors.New("permission denied")
	ErrNotConfigured      = errors.New("authorizer is nil")
	ErrEmptyAuthorization = errors.New("authorization resolver returned empty resource or action")
	ErrResolverMissing    = errors.New("authz resolver is required")
)

type Request struct {
	Principal *principal.Principal
	Resource  string
	Action    string
	Domain    string
	Extra     []any
}

type Authorizer interface {
	Authorize(ctx context.Context, req Request) (bool, error)
}

type Resolver func(ctx context.Context, req any) (resource string, action string, err error)

type MiddlewareOptions struct {
	AllowEmpty bool
}

func Require(ctx context.Context, a Authorizer, resource, action string) error {
	p, err := principal.RequireFromContext(ctx)
	if err != nil {
		return err
	}
	return RequirePrincipal(ctx, a, p, resource, action)
}

func RequirePrincipal(ctx context.Context, a Authorizer, p *principal.Principal, resource, action string) error {
	if p == nil || p.IsZero() {
		return principal.ErrMissingPrincipal
	}
	if a == nil {
		return ErrNotConfigured
	}
	if resource == "" || action == "" {
		return ErrEmptyAuthorization
	}
	ok, err := a.Authorize(ctx, Request{Principal: p, Resource: resource, Action: action})
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: %s cannot %s %s", ErrDenied, p.Subject(), action, resource)
	}
	return nil
}
