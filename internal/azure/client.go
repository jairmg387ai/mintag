// Package azure provides a minimal client for posting time entries to the
// Azure DevOps TimeLog extension. Credentials are never logged.
package azure

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
	"time"
)

// Config holds the Azure TimeLog client configuration. All fields have
// sensible defaults applied by NewClientFromEnv.
type Config struct {
	Token      string // MINTAG_AZURE_TIMELOG_TOKEN, or PAT fallback — empty means uploads are disabled
	AuthMode   string // "bearer" or "basic"; PAT fallback defaults to "basic"
	Org        string // default: "RUNT2PSW"
	WorkItemID int    // default: 156263
	User       string // default: "Jair Reinel Muñoz Gomez"
	UserID     string // default: "781ef5a8-e9fc-63f2-9c64-ea9193bcbd6d"
	EntryType  string // default: "Desarrollo de Software (Codificación)"
}

// TimeEntry is the payload for a single time-log upload.
type TimeEntry struct {
	Date           string  // YYYY-MM-DD
	Hours          float64 // will be converted to integer minutes
	RegistroDiario string  // sent as "comment"
}

// Client is the Azure TimeLog HTTP client.
type Client struct {
	cfg  Config
	http *http.Client
}

// NewClient creates a Client with the provided Config.
func NewClient(cfg Config) *Client {
	return &Client{cfg: cfg, http: &http.Client{Timeout: 30 * time.Second}}
}

const (
	AuthModeBearer = "bearer"
	AuthModeBasic  = "basic"
	AuthModeOAuth  = "oauth"
)

// NewClientFromEnv reads MINTAG_AZURE_TIMELOG_TOKEN (or the legacy
// MINTAG_AZURE_TIMELOG_PAT fallback) and optional overrides from the environment.
func NewClientFromEnv() *Client {
	token := os.Getenv("MINTAG_AZURE_TIMELOG_TOKEN")
	authMode := envOrDefault("MINTAG_AZURE_TIMELOG_AUTH_MODE", AuthModeBearer)
	if token == "" {
		token = os.Getenv("MINTAG_AZURE_TIMELOG_PAT")
		authMode = AuthModeBasic
	}
	cfg := Config{
		Token:      token,
		AuthMode:   authMode,
		Org:        envOrDefault("MINTAG_AZURE_ORG", "RUNT2PSW"),
		WorkItemID: 156263,
		User:       envOrDefault("MINTAG_AZURE_USER", "Jair Reinel Muñoz Gomez"),
		UserID:     envOrDefault("MINTAG_AZURE_USER_ID", "781ef5a8-e9fc-63f2-9c64-ea9193bcbd6d"),
		EntryType:  envOrDefault("MINTAG_AZURE_ENTRY_TYPE", "Desarrollo de Software (Codificación)"),
	}
	return NewClient(cfg)
}

// Enabled reports whether the client has an upload credential configured.
func (c *Client) Enabled() bool { return c.cfg.Token != "" }

// Config returns a copy of the client configuration.
func (c *Client) Config() Config { return c.cfg }

// SetHTTPClient replaces the internal HTTP client. Used in tests to redirect
// requests to a local httptest.Server without modifying production code paths.
func (c *Client) SetHTTPClient(hc *http.Client) { c.http = hc }

// PostTimeEntry builds and sends a single time-log POST to the Azure TimeLog
// extension. It returns the Azure document id on real success. A 2xx response
// without a non-empty JSON id is treated as an error because local rows must not
// be marked uploaded unless Azure confirms document creation.
func (c *Client) PostTimeEntry(ctx context.Context, e TimeEntry) (string, error) {
	if !c.Enabled() {
		return "", fmt.Errorf("Azure TimeLog token is not configured")
	}

	minutes := int(math.Round(e.Hours * 60))
	dateWeek, err := isoWeekString(e.Date)
	if err != nil {
		return "", err
	}

	payload := map[string]any{
		"minutes":    minutes,
		"user":       c.cfg.User,
		"userId":     c.cfg.UserID,
		"date":       e.Date,
		"dateWeek":   dateWeek,
		"workItemId": c.cfg.WorkItemID,
		"type":       c.cfg.EntryType,
		"comment":    e.RegistroDiario,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("azure: marshal payload: %w", err)
	}

	url := fmt.Sprintf(
		"https://extmgmt.dev.azure.com/%s/_apis/ExtensionManagement/InstalledExtensions/TimeLog/time-logging-extension/Data/Scopes/Default/Current/Collections/TimeLogData/Documents",
		c.cfg.Org,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("azure: build request: %w", err)
	}

	if c.cfg.AuthMode == AuthModeBasic {
		// Basic auth: base64(":{PAT}") — the credential must not appear in logs.
		token := base64.StdEncoding.EncodeToString([]byte(":" + c.cfg.Token))
		req.Header.Set("Authorization", "Basic "+token)
	} else {
		req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json;api-version=3.1-preview.1;excludeUrls=true")
	req.Header.Set("X-TFS-FedAuthRedirect", "Suppress")
	req.Header.Set("Origin", "https://dev.azure.com")
	req.Header.Set("Referer", "https://dev.azure.com/")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("azure: http request: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("azure: unexpected status %d%s", resp.StatusCode, sanitizedResponseMessage(respBody))
	}
	if isHTMLResponse(resp.Header.Get("Content-Type"), respBody) {
		return "", fmt.Errorf("azure: Azure returned HTML/sign-in response; token may be expired or auth mode invalid")
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(respBody, &created); err != nil {
		return "", fmt.Errorf("azure: decode success response: %w", err)
	}
	created.ID = strings.TrimSpace(created.ID)
	if created.ID == "" {
		return "", fmt.Errorf("azure: success response missing document id")
	}
	return created.ID, nil
}

func isHTMLResponse(contentType string, body []byte) bool {
	if strings.Contains(strings.ToLower(contentType), "text/html") {
		return true
	}
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	return strings.HasPrefix(lower, "<html") || strings.HasPrefix(lower, "<!doctype html")
}

func sanitizedResponseMessage(body []byte) string {
	var payload struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	msg := strings.TrimSpace(payload.Message)
	if msg == "" {
		msg = strings.TrimSpace(payload.Error)
	}
	if msg == "" {
		return ""
	}
	msg = strings.ReplaceAll(msg, "\r", " ")
	msg = strings.ReplaceAll(msg, "\n", " ")
	if len(msg) > 300 {
		msg = msg[:300] + "..."
	}
	return ": " + msg
}

// isoWeekString derives the ISO 8601 week string (e.g. "2026-W01") for the
// given YYYY-MM-DD date. Uses time.ISOWeek() so year-boundary cases are correct.
func isoWeekString(date string) (string, error) {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return "", fmt.Errorf("invalid date %q: %w", date, err)
	}
	year, week := t.ISOWeek()
	return fmt.Sprintf("%d-W%02d", year, week), nil
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
