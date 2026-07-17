package azure

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestDeviceAuthStart_RequestShape(t *testing.T) {
	var gotPath string
	var gotForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		gotForm = r.PostForm
		_ = json.NewEncoder(w).Encode(map[string]any{
			"device_code":      "device-secret",
			"user_code":        "ABCD-EFGH",
			"verification_uri": "https://microsoft.com/devicelogin",
			"expires_in":       900,
			"interval":         5,
			"message":          "Use code ABCD-EFGH",
		})
	}))
	defer srv.Close()

	c := NewDeviceAuthClient(OAuthConfig{Tenant: "tenant-id", ClientID: "client-id", Scope: "scope-a offline_access"})
	c.SetBaseURL(srv.URL)

	resp, err := c.StartDeviceCode(context.Background())
	if err != nil {
		t.Fatalf("StartDeviceCode: %v", err)
	}
	if gotPath != "/tenant-id/oauth2/v2.0/devicecode" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	if gotForm.Get("client_id") != "client-id" || gotForm.Get("scope") != "scope-a offline_access" {
		t.Fatalf("unexpected form: %#v", gotForm)
	}
	if resp.UserCode != "ABCD-EFGH" || resp.DeviceCode != "device-secret" {
		t.Fatalf("unexpected device response: %#v", resp)
	}
}

func TestDeviceAuthPoll_PendingThenSuccess(t *testing.T) {
	call := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call++
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.PostForm.Get("grant_type") != "urn:ietf:params:oauth:grant-type:device_code" {
			t.Fatalf("unexpected grant_type: %s", r.PostForm.Get("grant_type"))
		}
		if r.PostForm.Get("client_id") != "client-id" || r.PostForm.Get("device_code") != "device-secret" {
			t.Fatalf("unexpected form: %#v", r.PostForm)
		}
		if call == 1 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"authorization_pending","access_token":"must-not-leak"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "access-token",
			"refresh_token": "refresh-token",
			"expires_in":    3600,
		})
	}))
	defer srv.Close()

	c := NewDeviceAuthClient(OAuthConfig{Tenant: "tenant-id", ClientID: "client-id", Scope: "scope-a"})
	c.SetBaseURL(srv.URL)

	token, status, err := c.PollDeviceCode(context.Background(), "device-secret")
	if !errors.Is(err, ErrAuthorizationPending) || status != DeviceAuthStatusPending || token != nil {
		t.Fatalf("expected pending, got token=%#v status=%q err=%v", token, status, err)
	}

	token, status, err = c.PollDeviceCode(context.Background(), "device-secret")
	if err != nil || status != DeviceAuthStatusComplete {
		t.Fatalf("expected success, status=%q err=%v", status, err)
	}
	if token.AccessToken != "access-token" || token.RefreshToken != "refresh-token" || token.ExpiresAt.IsZero() {
		t.Fatalf("unexpected token: %#v", token)
	}
}

func TestDeviceAuthPoll_SlowDownIsRetryablePending(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"slow_down","error_description":"poll less frequently","access_token":"must-not-leak"}`))
	}))
	defer srv.Close()

	c := NewDeviceAuthClient(OAuthConfig{Tenant: "tenant-id", ClientID: "client-id", Scope: "scope-a"})
	c.SetBaseURL(srv.URL)
	token, status, err := c.PollDeviceCode(context.Background(), "device-secret")
	if !errors.Is(err, ErrAuthorizationPending) || status != DeviceAuthStatusPending || token != nil {
		t.Fatalf("expected retryable pending slow_down, got token=%#v status=%q err=%v", token, status, err)
	}
}

func TestRefreshToken_RequestShape(t *testing.T) {
	var gotForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		gotForm = r.PostForm
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "new-access",
			"refresh_token": "rotated-refresh",
			"expires_in":    120,
		})
	}))
	defer srv.Close()

	c := NewDeviceAuthClient(OAuthConfig{Tenant: "tenant-id", ClientID: "client-id", Scope: "scope-a offline_access"})
	c.SetBaseURL(srv.URL)
	token, err := c.RefreshToken(context.Background(), "old-refresh")
	if err != nil {
		t.Fatalf("RefreshToken: %v", err)
	}
	if gotForm.Get("grant_type") != "refresh_token" || gotForm.Get("client_id") != "client-id" || gotForm.Get("scope") != "scope-a offline_access" || gotForm.Get("refresh_token") != "old-refresh" {
		t.Fatalf("unexpected refresh form: %#v", gotForm)
	}
	if token.AccessToken != "new-access" || token.RefreshToken != "rotated-refresh" {
		t.Fatalf("unexpected token: %#v", token)
	}
}
