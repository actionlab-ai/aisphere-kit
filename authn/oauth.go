package authn

import (
	"context"
	"net/url"
)

const DefaultLoginScope = "openid profile email"

// LoginRequest describes the minimum information required to start an OAuth/OIDC login flow.
type LoginRequest struct {
	RedirectURI string
	State       string
	Scope       string
	Prompt      string
	Extra       url.Values
}

// LogoutRequest describes logout URL construction. Providers may ignore unsupported fields.
type LogoutRequest struct {
	IDTokenHint           string
	PostLogoutRedirectURI string
	State                 string
	Extra                 url.Values
}

// ExchangeCodeRequest exchanges an OAuth authorization code for tokens.
type ExchangeCodeRequest struct {
	Code        string
	RedirectURI string
	Extra       url.Values
}

// RefreshTokenRequest exchanges a refresh token for a new access token.
type RefreshTokenRequest struct {
	RefreshToken string
	Scope        string
	Extra        url.Values
}

// TokenResponse is a provider-neutral OAuth/OIDC token response.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	TokenType    string `json:"token_type,omitempty"`
	ExpiresIn    int64  `json:"expires_in,omitempty"`
	Scope        string `json:"scope,omitempty"`
	Raw          []byte `json:"-"`
}

// LoginURLBuilder is implemented by providers that can start a browser login flow.
type LoginURLBuilder interface {
	LoginURL(req LoginRequest) (string, error)
}

// LogoutURLBuilder is implemented by providers that can produce a browser logout URL.
type LogoutURLBuilder interface {
	LogoutURL(req LogoutRequest) (string, error)
}

// OAuthExchanger is implemented by providers that can exchange OAuth codes and refresh tokens.
type OAuthExchanger interface {
	ExchangeCode(ctx context.Context, req ExchangeCodeRequest) (*TokenResponse, error)
	RefreshToken(ctx context.Context, req RefreshTokenRequest) (*TokenResponse, error)
}
