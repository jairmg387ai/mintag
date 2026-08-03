package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Gentleman-Programming/mintag/internal/azure"
	"github.com/Gentleman-Programming/mintag/internal/store"
)

func TestAzureConfigContract_NeverEchoesToken(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")

	base, _ := newTestServer(t)
	secret := "server-test-secret-token"

	putResp := doJSON(t, http.MethodPut, base+"/api/activities/azure-config", map[string]any{
		"token":     secret,
		"auth_mode": "basic",
	})
	assertStatus(t, putResp, http.StatusOK)
	assertBodyDoesNotContain(t, putResp, secret)

	getResp := get(t, base+"/api/activities/azure-config")
	body := assertBodyDoesNotContain(t, getResp, secret)
	var status map[string]any
	if err := json.Unmarshal(body, &status); err != nil {
		t.Fatalf("decode config status: %v", err)
	}
	if status["configured"] != true {
		t.Fatalf("expected configured=true, got %v", status["configured"])
	}
	if status["auth_mode"] != "basic" {
		t.Fatalf("expected auth_mode=basic, got %v", status["auth_mode"])
	}
	if _, ok := status["token"]; ok {
		t.Fatal("config response must not contain token field")
	}
}

func TestAzureConfigClearFallsBackToEnvWithoutEchoingToken(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "env-secret-token")
	t.Setenv("MINTAG_AZURE_TIMELOG_AUTH_MODE", "bearer")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")

	base, _ := newTestServer(t)
	localSecret := "local-secret-token"
	putResp := doJSON(t, http.MethodPut, base+"/api/activities/azure-config", map[string]any{"token": localSecret, "auth_mode": "basic"})
	assertStatus(t, putResp, http.StatusOK)

	deleteResp := doJSON(t, http.MethodDelete, base+"/api/activities/azure-config", nil)
	assertStatus(t, deleteResp, http.StatusOK)
	body := assertBodyDoesNotContain(t, deleteResp, localSecret, "env-secret-token")

	var status map[string]any
	if err := json.Unmarshal(body, &status); err != nil {
		t.Fatalf("decode clear status: %v", err)
	}
	if status["configured"] != true || status["source"] != "env" || status["auth_mode"] != "bearer" {
		t.Fatalf("expected env fallback status, got %#v", status)
	}
}

func TestActivityUploadRouteUsesStoreBackedAzureConfig(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")

	var gotAuthMu sync.Mutex
	var gotAuth string
	azureServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthMu.Lock()
		gotAuth = r.Header.Get("Authorization")
		gotAuthMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if strings.Contains(r.URL.Path, "connectiondata") {
			_, _ = w.Write([]byte(`{"authenticatedUser":{"id":"route-user-id","providerDisplayName":"Route User"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"route-doc-123"}`))
	}))
	defer azureServer.Close()

	base, _ := newTestServerWithAzureRedirect(t, azureServer.URL)

	missingResp := postJSONRaw(t, base+"/api/activities/upload?date=2026-06-12", nil)
	assertStatus(t, missingResp, http.StatusServiceUnavailable)

	putResp := doJSON(t, http.MethodPut, base+"/api/activities/azure-config", map[string]any{"token": "db-token", "auth_mode": "bearer"})
	assertStatus(t, putResp, http.StatusOK)

	createResp := doJSON(t, http.MethodPost, base+"/api/activities", map[string]any{
		"date":            "2026-06-12",
		"hours":           1.25,
		"project":         "RNCEA",
		"category":        "Actividades de arquitectura, diseño y código",
		"registro_diario": "Route-level upload test",
		"source":          "manual",
	})
	assertStatus(t, createResp, http.StatusCreated)
	var created struct {
		ID int64 `json:"id"`
	}
	mustNoErr(t, json.NewDecoder(createResp.Body).Decode(&created))
	mustNoErr(t, createResp.Body.Close())

	approveResp := doJSON(t, http.MethodPatch, base+"/api/activities/"+strconv.FormatInt(created.ID, 10), map[string]any{"action": "approve"})
	assertStatus(t, approveResp, http.StatusOK)
	_, _ = io.Copy(io.Discard, approveResp.Body)
	mustNoErr(t, approveResp.Body.Close())

	uploadResp := postJSONRaw(t, base+"/api/activities/upload?date=2026-06-12", nil)
	assertStatus(t, uploadResp, http.StatusOK)
	body := assertBodyDoesNotContain(t, uploadResp, "db-token")
	if !strings.Contains(string(body), `"uploaded_count":1`) {
		t.Fatalf("expected one-upload result, got %s", string(body))
	}
	gotAuthMu.Lock()
	authHeader := gotAuth
	gotAuthMu.Unlock()
	if authHeader != "Bearer db-token" {
		t.Fatalf("expected route upload to use DB-backed bearer token, got %q", authHeader)
	}

	listResp := get(t, base+"/api/activities?date=2026-06-12&status=uploaded")
	var uploaded []struct {
		ID              int64   `json:"id"`
		Status          string  `json:"status"`
		AzureDocumentID *string `json:"azure_document_id"`
	}
	decodeJSON(t, listResp, &uploaded)
	if len(uploaded) != 1 {
		t.Fatalf("expected one uploaded activity, got %d", len(uploaded))
	}
	if uploaded[0].ID != created.ID || uploaded[0].Status != "uploaded" {
		t.Fatalf("expected activity %d uploaded, got %#v", created.ID, uploaded[0])
	}
	if uploaded[0].AzureDocumentID == nil || *uploaded[0].AzureDocumentID != "route-doc-123" {
		t.Fatalf("expected azure_document_id route-doc-123, got %#v", uploaded[0].AzureDocumentID)
	}
}

func TestAzureDeviceAuthRoutesStartAndCompleteWithoutLeakingTokens(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")

	oauthServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/devicecode"):
			if r.PostForm.Get("client_id") == "" || r.PostForm.Get("scope") == "" {
				t.Fatalf("missing device-code form fields: %#v", r.PostForm)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"device_code":      "local-device-code",
				"user_code":        "ABCD-EFGH",
				"verification_uri": "https://microsoft.com/devicelogin",
				"expires_in":       900,
				"interval":         5,
				"message":          "Use code ABCD-EFGH",
			})
		case strings.HasSuffix(r.URL.Path, "/token"):
			if r.PostForm.Get("grant_type") != "urn:ietf:params:oauth:grant-type:device_code" || r.PostForm.Get("device_code") != "local-device-code" {
				t.Fatalf("unexpected token form: %#v", r.PostForm)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "server-access-token",
				"refresh_token": "server-refresh-token",
				"expires_in":    3600,
			})
		default:
			t.Fatalf("unexpected oauth path: %s", r.URL.Path)
		}
	}))
	defer oauthServer.Close()

	st, err := store.OpenInMemory()
	mustNoErr(t, err)
	srv := New(st)
	srv.newAzureOAuthClient = func(_ context.Context, cfg azure.OAuthConfig) *azure.DeviceAuthClient {
		client := azure.NewDeviceAuthClient(cfg)
		client.SetBaseURL(oauthServer.URL)
		return client
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(func() { ts.Close(); st.Close() })

	startResp := doJSON(t, http.MethodPost, ts.URL+"/api/activities/azure-auth/device/start", map[string]any{})
	assertStatus(t, startResp, http.StatusOK)
	startBody := assertBodyDoesNotContain(t, startResp, "server-access-token", "server-refresh-token")
	if !strings.Contains(string(startBody), "local-device-code") || !strings.Contains(string(startBody), "ABCD-EFGH") {
		t.Fatalf("expected device instructions, got %s", string(startBody))
	}

	completeResp := doJSON(t, http.MethodPost, ts.URL+"/api/activities/azure-auth/device/complete", map[string]any{"device_code": "local-device-code"})
	assertStatus(t, completeResp, http.StatusOK)
	completeBody := assertBodyDoesNotContain(t, completeResp, "server-access-token", "server-refresh-token")
	if !strings.Contains(string(completeBody), `"status":"complete"`) {
		t.Fatalf("expected complete status, got %s", string(completeBody))
	}

	configResp := get(t, ts.URL+"/api/activities/azure-config")
	configBody := assertBodyDoesNotContain(t, configResp, "server-access-token", "server-refresh-token")
	var configStatus map[string]any
	mustNoErr(t, json.Unmarshal(configBody, &configStatus))
	if configStatus["oauth_connected"] != true {
		t.Fatalf("expected oauth-connected config, got %s", string(configBody))
	}
	if _, ok := configStatus["access_token"]; ok {
		t.Fatalf("config response must not contain access_token: %s", string(configBody))
	}
	if _, ok := configStatus["refresh_token"]; ok {
		t.Fatalf("config response must not contain refresh_token: %s", string(configBody))
	}
}

func TestAzureDeviceAuthCompleteMapsPendingDeclinedExpired(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")

	cases := []struct {
		name       string
		oauthError string
		wantStatus string
	}{
		{name: "pending", oauthError: "authorization_pending", wantStatus: azure.DeviceAuthStatusPending},
		{name: "slow down", oauthError: "slow_down", wantStatus: azure.DeviceAuthStatusPending},
		{name: "declined", oauthError: "authorization_declined", wantStatus: azure.DeviceAuthStatusDeclined},
		{name: "expired", oauthError: "expired_token", wantStatus: azure.DeviceAuthStatusExpired},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			oauthServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !strings.HasSuffix(r.URL.Path, "/token") {
					t.Fatalf("unexpected oauth path: %s", r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"` + tt.oauthError + `","access_token":"must-not-leak"}`))
			}))
			defer oauthServer.Close()

			st, err := store.OpenInMemory()
			mustNoErr(t, err)
			srv := New(st)
			srv.newAzureOAuthClient = func(_ context.Context, cfg azure.OAuthConfig) *azure.DeviceAuthClient {
				client := azure.NewDeviceAuthClient(cfg)
				client.SetBaseURL(oauthServer.URL)
				return client
			}
			ts := httptest.NewServer(srv.Handler())
			t.Cleanup(func() { ts.Close(); st.Close() })

			resp := doJSON(t, http.MethodPost, ts.URL+"/api/activities/azure-auth/device/complete", map[string]any{"device_code": "device-code"})
			assertStatus(t, resp, http.StatusOK)
			body := assertBodyDoesNotContain(t, resp, "must-not-leak")
			if !strings.Contains(string(body), `"status":"`+tt.wantStatus+`"`) {
				t.Fatalf("expected status %q, got %s", tt.wantStatus, string(body))
			}
		})
	}
}

func TestActivityUploadRouteRefreshesOAuthTokenBeforeUpload(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")

	var gotUploadAuth string
	azureServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUploadAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"oauth-doc-123"}`))
	}))
	defer azureServer.Close()

	var gotRefreshToken string
	oauthServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		gotRefreshToken = r.PostForm.Get("refresh_token")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "refreshed-access-token",
			"refresh_token": "rotated-refresh-token",
			"expires_in":    3600,
		})
	}))
	defer oauthServer.Close()

	st, err := store.OpenInMemory()
	mustNoErr(t, err)
	mustNoErr(t, st.SaveAzureOAuthTokens(context.Background(), &azure.TokenResponse{
		AccessToken:  "expired-access-token",
		RefreshToken: "old-refresh-token",
		ExpiresAt:    time.Now().UTC().Add(-time.Minute),
	}, azure.OAuthConfig{Tenant: "tenant", ClientID: "client", Scope: "scope offline_access"}))
	mustNoErr(t, st.SaveAzureIdentity(context.Background(), "oauth-user-id", "OAuth User"))
	base := newTestServerWithStoreAzureRedirect(t, st, azureServer.URL, oauthServer.URL)

	createResp := doJSON(t, http.MethodPost, base+"/api/activities", map[string]any{
		"date":            "2026-06-12",
		"hours":           1,
		"project":         "RNCEA",
		"category":        "Actividades de arquitectura, diseño y código",
		"registro_diario": "OAuth upload refresh test",
		"source":          "manual",
	})
	assertStatus(t, createResp, http.StatusCreated)
	var created struct {
		ID int64 `json:"id"`
	}
	mustNoErr(t, json.NewDecoder(createResp.Body).Decode(&created))
	mustNoErr(t, createResp.Body.Close())

	approveResp := doJSON(t, http.MethodPatch, base+"/api/activities/"+strconv.FormatInt(created.ID, 10), map[string]any{"action": "approve"})
	assertStatus(t, approveResp, http.StatusOK)
	_, _ = io.Copy(io.Discard, approveResp.Body)
	mustNoErr(t, approveResp.Body.Close())

	uploadResp := postJSONRaw(t, base+"/api/activities/upload?date=2026-06-12", nil)
	assertStatus(t, uploadResp, http.StatusOK)
	body := assertBodyDoesNotContain(t, uploadResp, "refreshed-access-token", "rotated-refresh-token", "old-refresh-token")
	if !strings.Contains(string(body), `"uploaded_count":1`) {
		t.Fatalf("expected upload success, got %s", string(body))
	}
	if gotRefreshToken != "old-refresh-token" {
		t.Fatalf("expected refresh flow to use saved refresh token, got %q", gotRefreshToken)
	}
	if gotUploadAuth != "Bearer refreshed-access-token" {
		t.Fatalf("expected upload to use refreshed bearer token, got %q", gotUploadAuth)
	}

	status, err := st.AzureTimeLogConfigStatus(context.Background())
	mustNoErr(t, err)
	if !status.OAuthConnected || status.OAuthAccessTokenExpiresAt == "" {
		t.Fatalf("expected OAuth status with refreshed expiry, got %#v", status)
	}
	if _, err := time.Parse(time.RFC3339, status.OAuthAccessTokenExpiresAt); err != nil {
		t.Fatalf("expected RFC3339 expiry saved, got %q: %v", status.OAuthAccessTokenExpiresAt, err)
	}
}

func TestLocalProtectionRejectsNonLocalOriginAndHost(t *testing.T) {
	_, st := newTestServer(t)
	h := New(st).Handler()

	req := httptest.NewRequest(http.MethodPut, "http://127.0.0.1/api/activities/azure-config", strings.NewReader(`{"token":"x"}`))
	req.Host = "evil.example"
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for non-local origin/host, got %d", rec.Code)
	}
}

func TestCORSAllowsLocalhostDevOriginOnly(t *testing.T) {
	_, st := newTestServer(t)
	h := New(st).Handler()

	req := httptest.NewRequest(http.MethodOptions, "http://127.0.0.1/api/activities/azure-config", nil)
	req.Host = "127.0.0.1"
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Origin", "http://localhost:5173")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected local preflight success, got %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("expected localhost origin echo, got %q", got)
	}

	req = httptest.NewRequest(http.MethodOptions, "http://127.0.0.1/api/activities/azure-config", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden preflight for non-local origin, got %d", rec.Code)
	}
}

func doJSON(t *testing.T, method, url string, body any) *http.Response {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		mustNoErr(t, err)
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, reader)
	mustNoErr(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	mustNoErr(t, err)
	return resp
}

func postJSONRaw(t *testing.T, url string, body []byte) *http.Response {
	t.Helper()
	if body == nil {
		body = []byte(`{}`)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	mustNoErr(t, err)
	return resp
}

func TestListAssignedAzureWorkItems(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")

	azureServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		switch {
		case strings.Contains(r.URL.Path, "connectiondata"):
			_, _ = w.Write([]byte(`{"authenticatedUser":{"id":"route-user-id","providerDisplayName":"Route User"}}`))
		case strings.Contains(r.URL.Path, "/_apis/wit/wiql"):
			_, _ = w.Write([]byte(`{"workItems":[{"id":555}]}`))
		case strings.Contains(r.URL.Path, "/_apis/wit/workitems"):
			_, _ = w.Write([]byte(`{"count":1,"value":[{"id":555,"fields":{"System.Title":"Fix crash on save","System.WorkItemType":"Bug","System.State":"Active"}}]}`))
		default:
			t.Fatalf("unexpected azure path: %s", r.URL.Path)
		}
	}))
	defer azureServer.Close()

	base, _ := newTestServerWithAzureRedirect(t, azureServer.URL)

	unconfiguredResp, err := http.Get(base + "/api/activities/azure-work-items/assigned")
	mustNoErr(t, err)
	assertStatus(t, unconfiguredResp, http.StatusServiceUnavailable)

	putResp := doJSON(t, http.MethodPut, base+"/api/activities/azure-config", map[string]any{"token": "db-token", "auth_mode": "bearer"})
	assertStatus(t, putResp, http.StatusOK)

	resp, err := http.Get(base + "/api/activities/azure-work-items/assigned")
	mustNoErr(t, err)
	assertStatus(t, resp, http.StatusOK)

	var body struct {
		Org   string `json:"org"`
		Items []struct {
			ID    int    `json:"id"`
			Title string `json:"title"`
			Type  string `json:"type"`
			State string `json:"state"`
		} `json:"items"`
	}
	mustNoErr(t, json.NewDecoder(resp.Body).Decode(&body))
	if len(body.Items) != 1 || body.Items[0].ID != 555 || body.Items[0].Title != "Fix crash on save" {
		t.Fatalf("unexpected response body: %+v", body)
	}
}

func newTestServerWithAzureRedirect(t *testing.T, azureBaseURL string) (baseURL string, st *store.Store) {
	t.Helper()
	st, err := store.OpenInMemory()
	mustNoErr(t, err)
	srv := New(st)
	srv.newAzureTimeLogClient = func(ctx context.Context) (*azure.Client, error) {
		client, err := st.NewAzureTimeLogClient(ctx)
		if err != nil {
			return nil, err
		}
		client.SetHTTPClient(&http.Client{Transport: azureRedirectToServer(azureBaseURL)})
		return client, nil
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(func() { ts.Close(); st.Close() })
	return ts.URL, st
}

func newTestServerWithStoreAzureRedirect(t *testing.T, st *store.Store, azureBaseURL, oauthBaseURL string) string {
	t.Helper()
	srv := New(st)
	srv.newAzureTimeLogClient = func(ctx context.Context) (*azure.Client, error) {
		client, err := st.NewAzureTimeLogClient(ctx)
		if err != nil {
			return nil, err
		}
		client.SetHTTPClient(&http.Client{Transport: azureRedirectToServer(azureBaseURL)})
		return client, nil
	}
	st.SetAzureOAuthClientFactory(func(_ context.Context, cfg azure.OAuthConfig) *azure.DeviceAuthClient {
		client := azure.NewDeviceAuthClient(cfg)
		client.SetBaseURL(oauthBaseURL)
		return client
	})
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(func() { ts.Close(); st.Close() })
	return ts.URL
}

type azureRedirectTransport struct {
	base    http.RoundTripper
	baseURL string
}

func azureRedirectToServer(baseURL string) http.RoundTripper {
	return &azureRedirectTransport{base: http.DefaultTransport, baseURL: baseURL}
}

func (rt *azureRedirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r2 := req.Clone(req.Context())
	r2.URL.Scheme = "http"
	r2.URL.Host = strings.TrimPrefix(rt.baseURL, "http://")
	return rt.base.RoundTrip(r2)
}

func assertStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("expected status %d, got %d — %s", want, resp.StatusCode, string(body))
	}
}

func assertBodyDoesNotContain(t *testing.T, resp *http.Response, forbidden ...string) []byte {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	mustNoErr(t, err)
	for _, value := range forbidden {
		if value != "" && strings.Contains(string(body), value) {
			t.Fatalf("response leaked forbidden value %q in %s", value, string(body))
		}
	}
	return body
}
