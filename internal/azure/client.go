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
	User       string // no default — must be resolved per-identity (see FetchIdentity) or set via MINTAG_AZURE_USER
	UserID     string // no default — must be resolved per-identity (see FetchIdentity) or set via MINTAG_AZURE_USER_ID
	EntryType  string // default: "Desarrollo de Software (Codificación)"
}

// TimeEntry is the payload for a single time-log upload.
type TimeEntry struct {
	Date           string  // YYYY-MM-DD
	Hours          float64 // will be converted to integer minutes
	RegistroDiario string  // sent as "comment"
	WorkItemID     int     // per-entry override; 0 means fall back to Config.WorkItemID
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
		User:       os.Getenv("MINTAG_AZURE_USER"),
		UserID:     os.Getenv("MINTAG_AZURE_USER_ID"),
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
	if strings.TrimSpace(c.cfg.UserID) == "" || strings.TrimSpace(c.cfg.User) == "" {
		return "", fmt.Errorf("azure: identity not resolved — reconnect the Azure token (FetchIdentity) before uploading, so entries are attributed to the right person")
	}

	minutes := int(math.Round(e.Hours * 60))
	dateWeek, err := isoWeekString(e.Date)
	if err != nil {
		return "", err
	}

	// Per-entry WorkItemID overrides the batch-level default so a single
	// client/token can upload entries targeting different work items.
	// >0 (not !=0) so a negative value is also treated as unset rather than
	// forwarded to Azure as an invalid id.
	workItemID := c.cfg.WorkItemID
	if e.WorkItemID > 0 {
		workItemID = e.WorkItemID
	}

	payload := map[string]any{
		"minutes":    minutes,
		"user":       c.cfg.User,
		"userId":     c.cfg.UserID,
		"date":       e.Date,
		"dateWeek":   dateWeek,
		"workItemId": workItemID,
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

	c.setAuthHeader(req)
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

// setAuthHeader applies the client's configured credential (PAT via Basic, or
// OAuth via Bearer) to an outgoing request. Shared by PostTimeEntry and
// FetchIdentity so both hit Azure with identical auth.
func (c *Client) setAuthHeader(req *http.Request) {
	if c.cfg.AuthMode == AuthModeBasic {
		// Basic auth: base64(":{PAT}") — the credential must not appear in logs.
		token := base64.StdEncoding.EncodeToString([]byte(":" + c.cfg.Token))
		req.Header.Set("Authorization", "Basic "+token)
	} else {
		req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	}
}

// FetchIdentity resolves the Azure DevOps identity (id + display name) behind
// the client's configured credential, via the connectiondata endpoint. This is
// the same id ADO extensions read as their web-context user id — NOT the same
// id returned by the VSSPS profile API (app.vssps.visualstudio.com/_apis/profile),
// which is a different identifier system and must not be used here.
func (c *Client) FetchIdentity(ctx context.Context) (userID, displayName string, err error) {
	if strings.TrimSpace(c.cfg.Token) == "" {
		return "", "", fmt.Errorf("azure: token is not configured")
	}

	url := fmt.Sprintf("https://dev.azure.com/%s/_apis/connectiondata?api-version=6.0-preview.1", c.cfg.Org)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", "", fmt.Errorf("azure: build identity request: %w", err)
	}
	c.setAuthHeader(req)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("azure: identity http request: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("azure: unexpected identity status %d%s", resp.StatusCode, sanitizedResponseMessage(respBody))
	}
	if isHTMLResponse(resp.Header.Get("Content-Type"), respBody) {
		return "", "", fmt.Errorf("azure: Azure returned HTML/sign-in response; token may be expired or auth mode invalid")
	}

	var parsed struct {
		AuthenticatedUser struct {
			ID                  string `json:"id"`
			ProviderDisplayName string `json:"providerDisplayName"`
		} `json:"authenticatedUser"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", "", fmt.Errorf("azure: decode identity response: %w", err)
	}
	userID = strings.TrimSpace(parsed.AuthenticatedUser.ID)
	displayName = strings.TrimSpace(parsed.AuthenticatedUser.ProviderDisplayName)
	if userID == "" || displayName == "" {
		return "", "", fmt.Errorf("azure: identity response missing id or display name")
	}
	return userID, displayName, nil
}

// AssignedWorkItem is a work item (bug/task/etc.) currently assigned to the
// caller in Azure Boards, as returned by FetchAssignedWorkItems.
type AssignedWorkItem struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Type  string `json:"type"`  // System.WorkItemType, e.g. "Bug", "Task"
	State string `json:"state"` // System.State, e.g. "Active", "New"
}

// FetchAssignedWorkItems returns the work items currently assigned to the
// caller (identified by the configured credential) in Azure Boards, across
// all types and open states. Uses the same credential as PostTimeEntry /
// FetchIdentity, but hits the Work Item Tracking REST API instead of the
// TimeLog extension endpoint — no extra OAuth scope is needed since the
// configured token already grants full user_impersonation.
func (c *Client) FetchAssignedWorkItems(ctx context.Context) ([]AssignedWorkItem, error) {
	if strings.TrimSpace(c.cfg.Token) == "" {
		return nil, fmt.Errorf("azure: token is not configured")
	}

	ids, err := c.queryAssignedWorkItemIDs(ctx)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []AssignedWorkItem{}, nil
	}
	return c.fetchWorkItemDetails(ctx, ids)
}

// queryAssignedWorkItemIDs runs a WIQL query for work items assigned to the
// current identity (@Me is resolved by Azure server-side from the
// credential) that are not in a closed/removed state, most recently changed
// first.
func (c *Client) queryAssignedWorkItemIDs(ctx context.Context) ([]int, error) {
	query := map[string]string{
		"query": "SELECT [System.Id] FROM WorkItems WHERE [System.AssignedTo] = @Me " +
			"AND [System.State] <> 'Closed' AND [System.State] <> 'Removed' AND [System.State] <> 'Cerrado' " +
			"AND [System.WorkItemType] IN ('Task', 'Bug') " +
			"ORDER BY [System.ChangedDate] DESC",
	}
	body, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("azure: marshal wiql query: %w", err)
	}

	url := fmt.Sprintf("https://dev.azure.com/%s/_apis/wit/wiql?api-version=7.1", c.cfg.Org)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("azure: build wiql request: %w", err)
	}
	c.setAuthHeader(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("azure: wiql http request: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("azure: unexpected wiql status %d%s", resp.StatusCode, sanitizedResponseMessage(respBody))
	}
	if isHTMLResponse(resp.Header.Get("Content-Type"), respBody) {
		return nil, fmt.Errorf("azure: Azure returned HTML/sign-in response; token may be expired or auth mode invalid")
	}

	var parsed struct {
		WorkItems []struct {
			ID int `json:"id"`
		} `json:"workItems"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("azure: decode wiql response: %w", err)
	}
	ids := make([]int, len(parsed.WorkItems))
	for i, wi := range parsed.WorkItems {
		ids[i] = wi.ID
	}
	return ids, nil
}

// fetchWorkItemDetails resolves title/type/state for a batch of work item
// ids, splitting requests at Azure's 200-item limit. ids must be non-empty —
// the Azure endpoint rejects an empty ids list.
func (c *Client) fetchWorkItemDetails(ctx context.Context, ids []int) ([]AssignedWorkItem, error) {
	const maxWorkItemsPerRequest = 200

	out := make([]AssignedWorkItem, 0, len(ids))
	for start := 0; start < len(ids); start += maxWorkItemsPerRequest {
		end := start + maxWorkItemsPerRequest
		if end > len(ids) {
			end = len(ids)
		}

		idStrs := make([]string, end-start)
		for i, id := range ids[start:end] {
			idStrs[i] = fmt.Sprintf("%d", id)
		}

		url := fmt.Sprintf(
			"https://dev.azure.com/%s/_apis/wit/workitems?ids=%s&fields=System.Id,System.Title,System.WorkItemType,System.State&api-version=7.1",
			c.cfg.Org, strings.Join(idStrs, ","),
		)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("azure: build workitems request: %w", err)
		}
		c.setAuthHeader(req)
		req.Header.Set("Accept", "application/json")

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("azure: workitems http request: %w", err)
		}
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("azure: unexpected workitems status %d%s", resp.StatusCode, sanitizedResponseMessage(respBody))
		}
		if isHTMLResponse(resp.Header.Get("Content-Type"), respBody) {
			return nil, fmt.Errorf("azure: Azure returned HTML/sign-in response; token may be expired or auth mode invalid")
		}

		var parsed struct {
			Value []struct {
				ID     int `json:"id"`
				Fields struct {
					Title string `json:"System.Title"`
					Type  string `json:"System.WorkItemType"`
					State string `json:"System.State"`
				} `json:"fields"`
			} `json:"value"`
		}
		if err := json.Unmarshal(respBody, &parsed); err != nil {
			return nil, fmt.Errorf("azure: decode workitems response: %w", err)
		}

		for _, v := range parsed.Value {
			out = append(out, AssignedWorkItem{
				ID:    v.ID,
				Title: v.Fields.Title,
				Type:  v.Fields.Type,
				State: v.Fields.State,
			})
		}
	}
	return out, nil
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
