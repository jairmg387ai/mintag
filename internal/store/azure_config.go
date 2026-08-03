package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Gentleman-Programming/mintag/internal/azure"
)

const (
	settingAzureTimeLogToken              = "azure.timelog.token"
	settingAzureTimeLogAuthMode           = "azure.timelog.auth_mode"
	settingAzureOAuthRefreshToken         = "azure.oauth.refresh_token"
	settingAzureOAuthAccessToken          = "azure.oauth.access_token"
	settingAzureOAuthAccessTokenExpiresAt = "azure.oauth.access_token_expires_at"
	settingAzureOAuthTenant               = "azure.oauth.tenant"
	settingAzureOAuthClientID             = "azure.oauth.client_id"
	settingAzureOAuthScope                = "azure.oauth.scope"
	settingAzureIdentityUserID            = "azure.identity.user_id"
	settingAzureIdentityDisplayName       = "azure.identity.display_name"
)

// AzureTimeLogConfigStatus is safe to return through APIs because it never
// includes the raw token.
type AzureTimeLogConfigStatus struct {
	Configured                bool   `json:"configured"`
	AuthMode                  string `json:"auth_mode"`
	Source                    string `json:"source"`
	OAuthConnected            bool   `json:"oauth_connected"`
	OAuthAccessTokenExpiresAt string `json:"oauth_access_token_expires_at,omitempty"`
	OAuthTenant               string `json:"oauth_tenant,omitempty"`
	OAuthClientID             string `json:"oauth_client_id,omitempty"`
	UserDisplayName           string `json:"user_display_name,omitempty"`
}

// AzureTimeLogConfig returns the effective upload config. Local DB settings win
// over environment variables so the UI can update credentials without restart.
// Manual Bearer/Basic credentials take precedence over OAuth; OAuth is used only
// when no manual token is configured.
func (s *Store) AzureTimeLogConfig(ctx context.Context) (azure.Config, string, error) {
	cfg := azure.NewClientFromEnv().Config()
	source := "env"

	if token, ok, err := s.setting(ctx, settingAzureTimeLogToken); err != nil {
		return cfg, source, err
	} else if ok && token != "" {
		cfg.Token = token
		source = "local_db"
		if mode, ok, err := s.setting(ctx, settingAzureTimeLogAuthMode); err != nil {
			return cfg, source, err
		} else if ok && mode != "" {
			cfg.AuthMode = normalizeAzureAuthMode(mode)
		} else {
			cfg.AuthMode = azure.AuthModeBearer
		}
	}
	if cfg.Token != "" {
		cfg.AuthMode = normalizeAzureAuthMode(cfg.AuthMode)
		return s.withResolvedAzureIdentity(ctx, cfg, source)
	}

	if refreshToken, ok, err := s.setting(ctx, settingAzureOAuthRefreshToken); err != nil {
		return cfg, source, err
	} else if ok && strings.TrimSpace(refreshToken) != "" {
		source = "oauth"
		cfg.AuthMode = azure.AuthModeOAuth
		if accessToken, ok, err := s.setting(ctx, settingAzureOAuthAccessToken); err != nil {
			return cfg, source, err
		} else if ok {
			cfg.Token = accessToken
		}
	}

	if cfg.Token == "" && source != "oauth" {
		source = "none"
	}
	cfg.AuthMode = normalizeAzureAuthMode(cfg.AuthMode)
	return s.withResolvedAzureIdentity(ctx, cfg, source)
}

// withResolvedAzureIdentity overrides cfg.User/UserID with the identity
// persisted by SaveAzureIdentity, if any. That identity is resolved (via
// Client.FetchIdentity) from whichever token is actually configured, so it
// always wins over the env var — a value someone else may have left in the
// shell no longer determines who uploads are attributed to.
func (s *Store) withResolvedAzureIdentity(ctx context.Context, cfg azure.Config, source string) (azure.Config, string, error) {
	if userID, ok, err := s.setting(ctx, settingAzureIdentityUserID); err != nil {
		return cfg, source, err
	} else if ok && userID != "" {
		cfg.UserID = userID
	}
	if displayName, ok, err := s.setting(ctx, settingAzureIdentityDisplayName); err != nil {
		return cfg, source, err
	} else if ok && displayName != "" {
		cfg.User = displayName
	}
	return cfg, source, nil
}

// SaveAzureIdentity persists the Azure DevOps identity (id + display name)
// resolved from the currently configured token via Client.FetchIdentity. It
// is what AzureTimeLogConfig injects into every future upload's Config, so
// uploads are attributed to whoever's token is actually configured instead of
// a stale or missing value.
func (s *Store) SaveAzureIdentity(ctx context.Context, userID, displayName string) error {
	userID = strings.TrimSpace(userID)
	displayName = strings.TrimSpace(displayName)
	if userID == "" || displayName == "" {
		return fmt.Errorf("userID and displayName are required")
	}
	if err := s.setSetting(ctx, settingAzureIdentityUserID, userID); err != nil {
		return err
	}
	return s.setSetting(ctx, settingAzureIdentityDisplayName, displayName)
}

func (s *Store) AzureTimeLogConfigStatus(ctx context.Context) (*AzureTimeLogConfigStatus, error) {
	cfg, source, err := s.AzureTimeLogConfig(ctx)
	if err != nil {
		return nil, err
	}
	oauthCfg := s.AzureOAuthConfig(ctx)
	expiresAt, _, err := s.setting(ctx, settingAzureOAuthAccessTokenExpiresAt)
	if err != nil {
		return nil, err
	}
	_, hasRefresh, err := s.setting(ctx, settingAzureOAuthRefreshToken)
	if err != nil {
		return nil, err
	}
	return &AzureTimeLogConfigStatus{
		Configured:                cfg.Token != "" || hasRefresh,
		AuthMode:                  cfg.AuthMode,
		Source:                    source,
		OAuthConnected:            hasRefresh,
		OAuthAccessTokenExpiresAt: expiresAt,
		OAuthTenant:               oauthCfg.Tenant,
		OAuthClientID:             oauthCfg.ClientID,
		UserDisplayName:           cfg.User,
	}, nil
}

func (s *Store) SaveAzureTimeLogConfig(ctx context.Context, token, authMode string) (*AzureTimeLogConfigStatus, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("token is required")
	}
	if err := s.setSetting(ctx, settingAzureTimeLogToken, token); err != nil {
		return nil, err
	}
	if err := s.setSetting(ctx, settingAzureTimeLogAuthMode, normalizeAzureAuthMode(authMode)); err != nil {
		return nil, err
	}
	if err := s.deleteSettings(ctx,
		settingAzureOAuthRefreshToken,
		settingAzureOAuthAccessToken,
		settingAzureOAuthAccessTokenExpiresAt,
		settingAzureIdentityUserID,
		settingAzureIdentityDisplayName,
	); err != nil {
		return nil, err
	}
	return s.AzureTimeLogConfigStatus(ctx)
}

func (s *Store) ClearAzureTimeLogConfig(ctx context.Context) (*AzureTimeLogConfigStatus, error) {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM app_settings WHERE key IN (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		settingAzureTimeLogToken,
		settingAzureTimeLogAuthMode,
		settingAzureOAuthRefreshToken,
		settingAzureOAuthAccessToken,
		settingAzureOAuthAccessTokenExpiresAt,
		settingAzureOAuthTenant,
		settingAzureOAuthClientID,
		settingAzureOAuthScope,
		settingAzureIdentityUserID,
		settingAzureIdentityDisplayName,
	); err != nil {
		return nil, err
	}
	return s.AzureTimeLogConfigStatus(ctx)
}

func (s *Store) NewAzureTimeLogClient(ctx context.Context) (*azure.Client, error) {
	cfg, source, err := s.AzureTimeLogConfig(ctx)
	if err != nil {
		return nil, err
	}
	if source != "oauth" {
		return azure.NewClient(cfg), nil
	}

	if refreshToken, ok, err := s.setting(ctx, settingAzureOAuthRefreshToken); err != nil {
		return nil, err
	} else if ok && strings.TrimSpace(refreshToken) != "" {
		accessToken, _, err := s.setting(ctx, settingAzureOAuthAccessToken)
		if err != nil {
			return nil, err
		}
		expiresAt, _, err := s.setting(ctx, settingAzureOAuthAccessTokenExpiresAt)
		if err != nil {
			return nil, err
		}
		if accessToken == "" || tokenExpiresSoon(expiresAt, 5*time.Minute) {
			oauth := s.newAzureOAuthClient(ctx, s.AzureOAuthConfig(ctx))
			tok, err := oauth.RefreshToken(ctx, refreshToken)
			if err != nil {
				return nil, err
			}
			if err := s.saveAzureOAuthTokens(ctx, tok, oauth.Config(), false); err != nil {
				return nil, err
			}
			accessToken = tok.AccessToken
		}
		cfg.Token = accessToken
		cfg.AuthMode = azure.AuthModeBearer
		return azure.NewClient(cfg), nil
	}
	return azure.NewClient(cfg), nil
}

func (s *Store) AzureOAuthConfig(ctx context.Context) azure.OAuthConfig {
	cfg := azure.OAuthConfigFromEnv()
	if tenant, ok, err := s.setting(ctx, settingAzureOAuthTenant); err == nil && ok && tenant != "" {
		cfg.Tenant = tenant
	}
	if clientID, ok, err := s.setting(ctx, settingAzureOAuthClientID); err == nil && ok && clientID != "" {
		cfg.ClientID = clientID
	}
	if scope, ok, err := s.setting(ctx, settingAzureOAuthScope); err == nil && ok && scope != "" {
		cfg.Scope = scope
	}
	return cfg
}

// SetAzureOAuthClientFactory replaces the OAuth client factory. It is intended
// for tests that need deterministic token endpoints.
func (s *Store) SetAzureOAuthClientFactory(factory func(context.Context, azure.OAuthConfig) *azure.DeviceAuthClient) {
	if factory == nil {
		s.newAzureOAuthClient = func(_ context.Context, cfg azure.OAuthConfig) *azure.DeviceAuthClient {
			return azure.NewDeviceAuthClient(cfg)
		}
		return
	}
	s.newAzureOAuthClient = factory
}

func (s *Store) SaveAzureOAuthConfig(ctx context.Context, cfg azure.OAuthConfig) error {
	if err := s.setSetting(ctx, settingAzureOAuthTenant, strings.TrimSpace(cfg.Tenant)); err != nil {
		return err
	}
	if err := s.setSetting(ctx, settingAzureOAuthClientID, strings.TrimSpace(cfg.ClientID)); err != nil {
		return err
	}
	return s.setSetting(ctx, settingAzureOAuthScope, strings.TrimSpace(cfg.Scope))
}

func (s *Store) SaveAzureOAuthTokens(ctx context.Context, token *azure.TokenResponse, cfg azure.OAuthConfig) error {
	return s.saveAzureOAuthTokens(ctx, token, cfg, true)
}

func (s *Store) saveAzureOAuthTokens(ctx context.Context, token *azure.TokenResponse, cfg azure.OAuthConfig, clearIdentity bool) error {
	if token == nil || strings.TrimSpace(token.AccessToken) == "" {
		return fmt.Errorf("oauth access token is required")
	}
	keys := []string{settingAzureTimeLogToken}
	if clearIdentity {
		keys = append(keys, settingAzureIdentityUserID, settingAzureIdentityDisplayName)
	}
	if err := s.deleteSettings(ctx, keys...); err != nil {
		return err
	}
	if err := s.SaveAzureOAuthConfig(ctx, cfg); err != nil {
		return err
	}
	if err := s.setSetting(ctx, settingAzureOAuthAccessToken, token.AccessToken); err != nil {
		return err
	}
	if !token.ExpiresAt.IsZero() {
		if err := s.setSetting(ctx, settingAzureOAuthAccessTokenExpiresAt, token.ExpiresAt.UTC().Format(time.RFC3339)); err != nil {
			return err
		}
	}
	if strings.TrimSpace(token.RefreshToken) != "" {
		if err := s.setSetting(ctx, settingAzureOAuthRefreshToken, token.RefreshToken); err != nil {
			return err
		}
	}
	return s.setSetting(ctx, settingAzureTimeLogAuthMode, azure.AuthModeOAuth)
}

func (s *Store) setting(ctx context.Context, key string) (string, bool, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

func (s *Store) setSetting(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value, time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

func (s *Store) deleteSettings(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(keys)), ",")
	args := make([]any, len(keys))
	for i, key := range keys {
		args[i] = key
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM app_settings WHERE key IN (`+placeholders+`)`, args...)
	return err
}

func normalizeAzureAuthMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case azure.AuthModeBasic:
		return azure.AuthModeBasic
	case azure.AuthModeOAuth:
		return azure.AuthModeOAuth
	default:
		return azure.AuthModeBearer
	}
}

func tokenExpiresSoon(expiresAt string, window time.Duration) bool {
	expiresAt = strings.TrimSpace(expiresAt)
	if expiresAt == "" {
		return true
	}
	t, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return true
	}
	return time.Now().UTC().Add(window).After(t)
}
