package casdoor

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/actionlab-ai/aisphere-kit/authn"
)

// LoginURL builds a Casdoor OAuth authorization URL. It does not exchange code for token;
// services should let their web frontend redirect the browser and then handle the callback.
func (a *Adapter) LoginURL(req authn.LoginRequest) (string, error) {
	if a == nil {
		return "", fmt.Errorf("casdoor adapter is nil")
	}
	if strings.TrimSpace(req.RedirectURI) == "" {
		return "", fmt.Errorf("redirect_uri is required")
	}
	base, err := url.Parse(strings.TrimRight(a.cfg.Endpoint, "/") + "/login/oauth/authorize")
	if err != nil {
		return "", err
	}
	q := base.Query()
	q.Set("client_id", a.cfg.ClientID)
	q.Set("response_type", "code")
	q.Set("redirect_uri", req.RedirectURI)
	if req.Scope != "" {
		q.Set("scope", req.Scope)
	} else {
		q.Set("scope", "read")
	}
	if req.State != "" {
		q.Set("state", req.State)
	}
	if req.Prompt != "" {
		q.Set("prompt", req.Prompt)
	}
	for k, vs := range req.Extra {
		for _, v := range vs {
			q.Add(k, v)
		}
	}
	base.RawQuery = q.Encode()
	return base.String(), nil
}

// LogoutURL builds a Casdoor logout URL. Casdoor deployments may customize logout behavior;
// this helper keeps services from duplicating common URL construction.
func (a *Adapter) LogoutURL(req authn.LogoutRequest) (string, error) {
	if a == nil {
		return "", fmt.Errorf("casdoor adapter is nil")
	}
	base, err := url.Parse(strings.TrimRight(a.cfg.Endpoint, "/") + "/logout")
	if err != nil {
		return "", err
	}
	q := base.Query()
	if req.PostLogoutRedirectURI != "" {
		q.Set("post_logout_redirect_uri", req.PostLogoutRedirectURI)
		q.Set("redirect_uri", req.PostLogoutRedirectURI)
	}
	if req.IDTokenHint != "" {
		q.Set("id_token_hint", req.IDTokenHint)
	}
	if req.State != "" {
		q.Set("state", req.State)
	}
	for k, vs := range req.Extra {
		for _, v := range vs {
			q.Add(k, v)
		}
	}
	base.RawQuery = q.Encode()
	return base.String(), nil
}
