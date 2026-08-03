package store

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Gentleman-Programming/mintag/internal/azure"
)

func TestSaveAzureTimeLogConfigClearsOAuthAndActivatesBasic(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")

	ctx := context.Background()
	s := openTestDB(t)
	mustNoErr(t, s.SaveAzureOAuthTokens(ctx, &azure.TokenResponse{
		AccessToken:  "oauth-access-token",
		RefreshToken: "oauth-refresh-token",
		ExpiresAt:    time.Now().UTC().Add(time.Hour),
	}, azure.OAuthConfig{Tenant: "tenant", ClientID: "client", Scope: "scope"}))

	status, err := s.SaveAzureTimeLogConfig(ctx, "manual-pat", azure.AuthModeBasic)
	mustNoErr(t, err)
	if !status.Configured || status.Source != "local_db" || status.AuthMode != azure.AuthModeBasic || status.OAuthConnected {
		t.Fatalf("expected local basic manual config to be active, got %#v", status)
	}
	if _, ok, err := s.setting(ctx, settingAzureOAuthRefreshToken); err != nil || ok {
		t.Fatalf("expected refresh token cleared, ok=%v err=%v", ok, err)
	}

	client, err := s.NewAzureTimeLogClient(ctx)
	mustNoErr(t, err)
	if got := client.Config().AuthMode; got != azure.AuthModeBasic {
		t.Fatalf("expected basic auth mode, got %q", got)
	}
}

func TestSaveAzureOAuthTokensClearsPersistedIdentity(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")
	t.Setenv("MINTAG_AZURE_USER", "")
	t.Setenv("MINTAG_AZURE_USER_ID", "")

	ctx := context.Background()
	s := openTestDB(t)
	mustNoErr(t, s.SaveAzureIdentity(ctx, "stale-user-id", "Stale User"))

	mustNoErr(t, s.SaveAzureOAuthTokens(ctx, &azure.TokenResponse{
		AccessToken:  "oauth-access-token",
		RefreshToken: "oauth-refresh-token",
		ExpiresAt:    time.Now().UTC().Add(time.Hour),
	}, azure.OAuthConfig{Tenant: "tenant", ClientID: "client", Scope: "scope"}))

	for _, key := range []string{settingAzureIdentityUserID, settingAzureIdentityDisplayName} {
		if value, ok, err := s.setting(ctx, key); err != nil || ok {
			t.Fatalf("expected %s cleared, value=%q ok=%v err=%v", key, value, ok, err)
		}
	}

	cfg, source, err := s.AzureTimeLogConfig(ctx)
	mustNoErr(t, err)
	if source != "oauth" || cfg.UserID != "" || cfg.User != "" {
		t.Fatalf("expected oauth config without persisted identity, source=%q userID=%q user=%q", source, cfg.UserID, cfg.User)
	}
}

func TestNewAzureTimeLogClientRefreshesOAuthTokenAndPreservesIdentity(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")
	t.Setenv("MINTAG_AZURE_USER", "")
	t.Setenv("MINTAG_AZURE_USER_ID", "")

	ctx := context.Background()
	s := openTestDB(t)
	mustNoErr(t, s.setSetting(ctx, settingAzureOAuthRefreshToken, "old-refresh-token"))
	mustNoErr(t, s.setSetting(ctx, settingAzureOAuthAccessTokenExpiresAt, time.Now().UTC().Add(-time.Minute).Format(time.RFC3339)))
	mustNoErr(t, s.SaveAzureOAuthConfig(ctx, azure.OAuthConfig{Tenant: "tenant", ClientID: "client", Scope: "scope offline_access"}))
	mustNoErr(t, s.SaveAzureIdentity(ctx, "user-id", "Current User"))

	var gotRefresh string
	oauthServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		gotRefresh = r.PostForm.Get("refresh_token")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "refreshed-access-token",
			"refresh_token": "rotated-refresh-token",
			"expires_in":    7200,
		})
	}))
	defer oauthServer.Close()
	s.newAzureOAuthClient = func(_ context.Context, cfg azure.OAuthConfig) *azure.DeviceAuthClient {
		client := azure.NewDeviceAuthClient(cfg)
		client.SetBaseURL(oauthServer.URL)
		return client
	}

	client, err := s.NewAzureTimeLogClient(ctx)
	mustNoErr(t, err)
	if gotRefresh != "old-refresh-token" {
		t.Fatalf("expected refresh request to use old refresh token, got %q", gotRefresh)
	}
	if client.Config().Token != "refreshed-access-token" {
		t.Fatalf("expected client to use refreshed access token, got %q", client.Config().Token)
	}
	if got, _, err := s.setting(ctx, settingAzureOAuthRefreshToken); err != nil || got != "rotated-refresh-token" {
		t.Fatalf("expected rotated refresh token saved, got %q err=%v", got, err)
	}
	if expiresAt, _, err := s.setting(ctx, settingAzureOAuthAccessTokenExpiresAt); err != nil || !tokenExpiresSoon(expiresAt, 3*time.Hour) {
		t.Fatalf("expected refreshed expiry to be saved near 2h from now, got %q err=%v", expiresAt, err)
	}
	cfg, source, err := s.AzureTimeLogConfig(ctx)
	mustNoErr(t, err)
	if source != "oauth" || cfg.UserID != "user-id" || cfg.User != "Current User" {
		t.Fatalf("expected refresh to preserve persisted identity, source=%q userID=%q user=%q", source, cfg.UserID, cfg.User)
	}
}

func TestAzureClientUsesBasicAuthorizationForManualPAT(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")

	ctx := context.Background()
	s := openTestDB(t)
	_, err := s.SaveAzureTimeLogConfig(ctx, "manual-pat", azure.AuthModeBasic)
	mustNoErr(t, err)
	mustNoErr(t, s.SaveAzureIdentity(ctx, "uid-1", "Test User"))

	client, err := s.NewAzureTimeLogClient(ctx)
	mustNoErr(t, err)
	var gotAuth string
	azureServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"doc-1"}`))
	}))
	defer azureServer.Close()
	client.SetHTTPClient(&http.Client{Transport: rewriteAzureHost(azureServer.URL)})
	_, err = client.PostTimeEntry(ctx, azure.TimeEntry{Date: "2026-07-09", Hours: 1, RegistroDiario: "test"})
	mustNoErr(t, err)

	want := "Basic " + base64.StdEncoding.EncodeToString([]byte(":"+"manual-pat"))
	if gotAuth != want {
		t.Fatalf("expected %q, got %q", want, gotAuth)
	}
}

type rewriteAzureHostTransport struct {
	baseURL string
}

func rewriteAzureHost(baseURL string) http.RoundTripper {
	return rewriteAzureHostTransport{baseURL: baseURL}
}

func (rt rewriteAzureHostTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r2 := req.Clone(req.Context())
	r2.URL.Scheme = "http"
	r2.URL.Host = rt.baseURL[len("http://"):]
	return http.DefaultTransport.RoundTrip(r2)
}
