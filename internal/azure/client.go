// Package azure provides a minimal client for posting time entries to the
// Azure DevOps TimeLog extension. The PAT is never logged.
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
	"time"
)

// Config holds the Azure TimeLog client configuration. All fields have
// sensible defaults applied by NewClientFromEnv.
type Config struct {
	PAT        string // MINTAG_AZURE_PAT — empty means uploads are disabled
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

// NewClientFromEnv reads MINTAG_AZURE_PAT (and optional overrides) from the
// environment and applies defaults for omitted values.
func NewClientFromEnv() *Client {
	cfg := Config{
		PAT:        os.Getenv("MINTAG_AZURE_PAT"),
		Org:        envOrDefault("MINTAG_AZURE_ORG", "RUNT2PSW"),
		WorkItemID: 156263,
		User:       envOrDefault("MINTAG_AZURE_USER", "Jair Reinel Muñoz Gomez"),
		UserID:     envOrDefault("MINTAG_AZURE_USER_ID", "781ef5a8-e9fc-63f2-9c64-ea9193bcbd6d"),
		EntryType:  envOrDefault("MINTAG_AZURE_ENTRY_TYPE", "Desarrollo de Software (Codificación)"),
	}
	return NewClient(cfg)
}

// Enabled reports whether the client has a PAT configured.
func (c *Client) Enabled() bool { return c.cfg.PAT != "" }

// SetHTTPClient replaces the internal HTTP client. Used in tests to redirect
// requests to a local httptest.Server without modifying production code paths.
func (c *Client) SetHTTPClient(hc *http.Client) { c.http = hc }

// PostTimeEntry builds and sends a single time-log POST to the Azure TimeLog
// extension. It returns an error on any non-2xx response, including the raw
// response body in the error message. The PAT is never written to any log.
func (c *Client) PostTimeEntry(ctx context.Context, e TimeEntry) error {
	if !c.Enabled() {
		return fmt.Errorf("MINTAG_AZURE_PAT is not configured")
	}

	minutes := int(math.Round(e.Hours * 60))
	dateWeek, err := isoWeekString(e.Date)
	if err != nil {
		return err
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
		return fmt.Errorf("azure: marshal payload: %w", err)
	}

	url := fmt.Sprintf(
		"https://extmgmt.dev.azure.com/%s/_apis/ExtensionManagement/InstalledExtensions/TimeLog/time-logging-extension/Data/Scopes/Default/Current/Collections/TimeLogData/Documents",
		c.cfg.Org,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("azure: build request: %w", err)
	}

	// Basic auth: base64(":{PAT}") — PAT must not appear in logs.
	token := base64.StdEncoding.EncodeToString([]byte(":" + c.cfg.PAT))
	req.Header.Set("Authorization", "Basic "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json;api-version=3.1-preview.1")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("azure: http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("azure: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
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
