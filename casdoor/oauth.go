package casdoor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/actionlab-ai/aisphere-kit/authn"
)

// LoginURL builds a Casdoor OAuth authorization URL. It does not exchange code for token;
// services should redirect the browser to this URL and then exchange the returned code.
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
	scope := strings.TrimSpace(req.Scope)
	if scope == "" {
		scope = strings.TrimSpace(a.cfg.DefaultScope)
	}
	if scope == "" {
		scope = authn.DefaultLoginScope
	}
	q.Set("scope", scope)
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

// ExchangeCode exchanges an OAuth authorization code for Casdoor tokens.
func (a *Adapter) ExchangeCode(ctx context.Context, req authn.ExchangeCodeRequest) (*authn.TokenResponse, error) {
	if a == nil {
		return nil, fmt.Errorf("casdoor adapter is nil")
	}
	if strings.TrimSpace(req.Code) == "" {
		return nil, fmt.Errorf("code is required")
	}
	if strings.TrimSpace(req.RedirectURI) == "" {
		return nil, fmt.Errorf("redirect_uri is required")
	}
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", a.cfg.ClientID)
	form.Set("client_secret", a.cfg.ClientSecret)
	form.Set("code", req.Code)
	form.Set("redirect_uri", req.RedirectURI)
	for k, vs := range req.Extra {
		for _, v := range vs {
			form.Add(k, v)
		}
	}
	return a.exchangeToken(ctx, "authorization_code", form)
}

// RefreshToken exchanges a refresh token for new Casdoor tokens.
func (a *Adapter) RefreshToken(ctx context.Context, req authn.RefreshTokenRequest) (*authn.TokenResponse, error) {
	if a == nil {
		return nil, fmt.Errorf("casdoor adapter is nil")
	}
	if strings.TrimSpace(req.RefreshToken) == "" {
		return nil, fmt.Errorf("refresh_token is required")
	}
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", a.cfg.ClientID)
	form.Set("client_secret", a.cfg.ClientSecret)
	form.Set("refresh_token", req.RefreshToken)
	if req.Scope != "" {
		form.Set("scope", req.Scope)
	}
	for k, vs := range req.Extra {
		for _, v := range vs {
			form.Add(k, v)
		}
	}
	return a.exchangeToken(ctx, "refresh_token", form)
}

func (a *Adapter) exchangeToken(ctx context.Context, grantType string, form url.Values) (*authn.TokenResponse, error) {
	started := time.Now()
	endpoint := strings.TrimRight(a.cfg.Endpoint, "/") + "/api/login/oauth/access_token"
	client := a.httpClient
	if client == nil {
		client = &http.Client{Timeout: a.timeout}
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	a.logger.Debug("casdoor token exchange started", "grant_type", grantType)
	resp, err := client.Do(httpReq)
	if a.metrics != nil {
		a.metrics.ObserveDependency("casdoor", "oauth_"+grantType, started, err)
	}
	if err != nil {
		a.logger.Warn("casdoor token exchange failed", "grant_type", grantType, "error", err, "elapsed", time.Since(started).String())
		return nil, err
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if readErr != nil {
		return nil, readErr
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("casdoor token exchange failed: status=%d body=%s", resp.StatusCode, string(bytes.TrimSpace(body)))
	}
	var out authn.TokenResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decode casdoor token response: %w", err)
	}
	out.Raw = body
	if out.TokenType == "" && out.AccessToken != "" {
		out.TokenType = "Bearer"
	}
	if out.AccessToken == "" {
		return nil, fmt.Errorf("casdoor token response missing access_token")
	}
	a.logger.Info("casdoor token exchange completed", "grant_type", grantType, "expires_in", out.ExpiresIn, "elapsed", time.Since(started).String())
	return &out, nil
}
