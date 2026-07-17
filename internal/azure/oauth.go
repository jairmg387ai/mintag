package azure

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultAzureTenant   = "46faf528-03f4-450a-acfc-8e5d059619d0"
	DefaultAzureClientID = "499b84ac-1321-427f-aa17-267ca6975798"
	DefaultAzureScope    = "499b84ac-1321-427f-aa17-267ca6975798/user_impersonation openid profile offline_access"

	DeviceAuthStatusPending  = "pending"
	DeviceAuthStatusComplete = "complete"
	DeviceAuthStatusDeclined = "declined"
	DeviceAuthStatusExpired  = "expired"
)

var ErrAuthorizationPending = errors.New("azure oauth: authorization pending")

type OAuthConfig struct {
	Tenant   string
	ClientID string
	Scope    string
}

func OAuthConfigFromEnv() OAuthConfig {
	return OAuthConfig{
		Tenant:   envOrDefault("MINTAG_AZURE_TENANT", DefaultAzureTenant),
		ClientID: envOrDefault("MINTAG_AZURE_CLIENT_ID", DefaultAzureClientID),
		Scope:    envOrDefault("MINTAG_AZURE_SCOPE", DefaultAzureScope),
	}
}

type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
	Message         string `json:"message"`
}

type TokenResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresIn    int       `json:"expires_in"`
	ExpiresAt    time.Time `json:"-"`
}

type DeviceAuthClient struct {
	cfg     OAuthConfig
	http    *http.Client
	baseURL string
}

func NewDeviceAuthClient(cfg OAuthConfig) *DeviceAuthClient {
	cfg.Tenant = strings.TrimSpace(cfg.Tenant)
	cfg.ClientID = strings.TrimSpace(cfg.ClientID)
	cfg.Scope = strings.TrimSpace(cfg.Scope)
	if cfg.Tenant == "" {
		cfg.Tenant = DefaultAzureTenant
	}
	if cfg.ClientID == "" {
		cfg.ClientID = DefaultAzureClientID
	}
	if cfg.Scope == "" {
		cfg.Scope = DefaultAzureScope
	}
	return &DeviceAuthClient{cfg: cfg, http: &http.Client{Timeout: 30 * time.Second}, baseURL: "https://login.microsoftonline.com"}
}

func NewDeviceAuthClientFromEnv() *DeviceAuthClient { return NewDeviceAuthClient(OAuthConfigFromEnv()) }

func (c *DeviceAuthClient) Config() OAuthConfig { return c.cfg }

func (c *DeviceAuthClient) SetHTTPClient(hc *http.Client) { c.http = hc }

func (c *DeviceAuthClient) SetBaseURL(baseURL string) { c.baseURL = strings.TrimRight(baseURL, "/") }

func (c *DeviceAuthClient) StartDeviceCode(ctx context.Context) (*DeviceCodeResponse, error) {
	form := url.Values{}
	form.Set("client_id", c.cfg.ClientID)
	form.Set("scope", c.cfg.Scope)

	var out DeviceCodeResponse
	if err := c.postForm(ctx, c.endpoint("devicecode"), form, &out); err != nil {
		return nil, err
	}
	if strings.TrimSpace(out.DeviceCode) == "" || strings.TrimSpace(out.UserCode) == "" {
		return nil, fmt.Errorf("azure oauth: device code response missing required fields")
	}
	return &out, nil
}

func (c *DeviceAuthClient) PollDeviceCode(ctx context.Context, deviceCode string) (*TokenResponse, string, error) {
	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	form.Set("client_id", c.cfg.ClientID)
	form.Set("device_code", strings.TrimSpace(deviceCode))

	token, err := c.postTokenForm(ctx, form)
	if err == nil {
		return token, DeviceAuthStatusComplete, nil
	}
	var oauthErr *oauthError
	if !errors.As(err, &oauthErr) {
		return nil, "", err
	}
	switch oauthErr.Code {
	case "authorization_pending", "slow_down":
		return nil, DeviceAuthStatusPending, ErrAuthorizationPending
	case "authorization_declined", "bad_verification_code":
		return nil, DeviceAuthStatusDeclined, nil
	case "expired_token":
		return nil, DeviceAuthStatusExpired, nil
	default:
		return nil, "", oauthErr
	}
}

func (c *DeviceAuthClient) RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", c.cfg.ClientID)
	form.Set("scope", c.cfg.Scope)
	form.Set("refresh_token", strings.TrimSpace(refreshToken))
	return c.postTokenForm(ctx, form)
}

func (c *DeviceAuthClient) postTokenForm(ctx context.Context, form url.Values) (*TokenResponse, error) {
	var out TokenResponse
	if err := c.postForm(ctx, c.endpoint("token"), form, &out); err != nil {
		return nil, err
	}
	if strings.TrimSpace(out.AccessToken) == "" {
		return nil, fmt.Errorf("azure oauth: token response missing access token")
	}
	if out.ExpiresIn <= 0 {
		out.ExpiresIn = 3600
	}
	out.ExpiresAt = time.Now().UTC().Add(time.Duration(out.ExpiresIn) * time.Second)
	return &out, nil
}

func (c *DeviceAuthClient) postForm(ctx context.Context, endpoint string, form url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("azure oauth: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("azure oauth: http request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return parseOAuthError(resp.StatusCode, body)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("azure oauth: decode response: %w", err)
	}
	return nil
}

func (c *DeviceAuthClient) endpoint(name string) string {
	tenant := url.PathEscape(c.cfg.Tenant)
	return c.baseURL + "/" + tenant + "/oauth2/v2.0/" + name
}

type oauthError struct {
	Code string
	Desc string
}

func (e *oauthError) Error() string {
	if e.Desc != "" {
		return "azure oauth: " + e.Code + ": " + e.Desc
	}
	return "azure oauth: " + e.Code
}

func parseOAuthError(status int, body []byte) error {
	var payload struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	_ = json.Unmarshal(body, &payload)
	code := sanitizeOAuthText(payload.Error)
	if code == "" {
		code = "http_" + strconv.Itoa(status)
	}
	return &oauthError{Code: code, Desc: sanitizeOAuthText(payload.ErrorDescription)}
}

func sanitizeOAuthText(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 300 {
		s = s[:300] + "..."
	}
	return s
}
