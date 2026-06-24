package authn

import (
	"context"
	"errors"
	"strings"

	"github.com/actionlab-ai/aisphere-kit/principal"
)

var (
	ErrNotConfigured  = errors.New("authenticator is nil")
	ErrTokenMissing   = errors.New("missing bearer token")
	ErrPrincipalEmpty = errors.New("token produced empty principal")
)

type Authenticator interface {
	Authenticate(ctx context.Context, token string) (*principal.Principal, error)
}

type Options struct {
	AllowAnonymous bool
	TokenExtractor TokenExtractor
}

type TokenExtractor func(ctx context.Context) (string, bool)

func ExtractBearer(header string) (string, bool) {
	header = strings.TrimSpace(header)
	if header == "" {
		return "", false
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") && strings.TrimSpace(parts[1]) != "" {
		return strings.TrimSpace(parts[1]), true
	}
	return "", false
}

func AuthenticateToken(ctx context.Context, a Authenticator, token string, allowAnonymous bool) (context.Context, *principal.Principal, error) {
	if a == nil {
		if allowAnonymous {
			p := principal.Anonymous()
			return principal.NewContext(ctx, p), p, nil
		}
		return ctx, nil, ErrNotConfigured
	}
	if token == "" {
		if allowAnonymous {
			p := principal.Anonymous()
			return principal.NewContext(ctx, p), p, nil
		}
		return ctx, nil, ErrTokenMissing
	}
	p, err := a.Authenticate(ctx, token)
	if err != nil {
		return ctx, nil, err
	}
	if p == nil || p.IsZero() {
		return ctx, nil, ErrPrincipalEmpty
	}
	if p.Token == "" {
		p.Token = token
	}
	return principal.NewContext(ctx, p), p, nil
}
