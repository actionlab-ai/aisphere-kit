package authn

import "net/url"

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

// LoginURLBuilder is implemented by providers that can start a browser login flow.
type LoginURLBuilder interface {
	LoginURL(req LoginRequest) (string, error)
}

// LogoutURLBuilder is implemented by providers that can produce a browser logout URL.
type LogoutURLBuilder interface {
	LogoutURL(req LogoutRequest) (string, error)
}
