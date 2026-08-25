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
	"net/url"
	"os"
	"strings"
	"time"
)

// Config holds the Azure TimeLog client configuration. All fields have
// sensible defaults applied by NewClientFromEnv.
type Config struct {
	Token       string // MINTAG_AZURE_TIMELOG_TOKEN, or PAT fallback — empty means uploads are disabled
	AuthMode    string // "bearer" or "basic"; PAT fallback defaults to "basic"
	Org         string // default: "RUNT2PSW"
	WorkItemID  int    // default: 156263
	User        string // no default — must be resolved per-identity (see FetchIdentity) or set via MINTAG_AZURE_USER
	UserID      string // no default — must be resolved per-identity (see FetchIdentity) or set via MINTAG_AZURE_USER_ID
	EntryType   string // default: "Desarrollo de Software (Codificación)"
	TeamProject string // Azure DevOps team project (distinct from mintag's own "project" concept), default: "RUNTPRO"
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
		Token:       token,
		AuthMode:    authMode,
		Org:         envOrDefault("MINTAG_AZURE_ORG", "RUNT2PSW"),
		WorkItemID:  156263,
		User:        os.Getenv("MINTAG_AZURE_USER"),
		UserID:      os.Getenv("MINTAG_AZURE_USER_ID"),
		EntryType:   envOrDefault("MINTAG_AZURE_ENTRY_TYPE", "Desarrollo de Software (Codificación)"),
		TeamProject: envOrDefault("MINTAG_AZURE_TEAM_PROJECT", "RUNTPRO"),
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

// DeleteTimeEntry deletes one Azure TimeLog document. Missing remote documents
// are treated as already deleted so retrying after a partial local failure is safe.
func (c *Client) DeleteTimeEntry(ctx context.Context, documentID string) error {
	documentID = strings.TrimSpace(documentID)
	if documentID == "" {
		return fmt.Errorf("azure: document id is required")
	}
	if !c.Enabled() {
		return fmt.Errorf("Azure TimeLog token is not configured")
	}

	endpoint := fmt.Sprintf(
		"https://extmgmt.dev.azure.com/%s/_apis/ExtensionManagement/InstalledExtensions/TimeLog/time-logging-extension/Data/Scopes/Default/Current/Collections/TimeLogData/Documents/%s",
		c.cfg.Org,
		url.PathEscape(documentID),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return fmt.Errorf("azure: build delete request: %w", err)
	}

	c.setAuthHeader(req)
	req.Header.Set("Accept", "application/json;api-version=3.1-preview.1;excludeUrls=true")
	req.Header.Set("X-TFS-FedAuthRedirect", "Suppress")
	req.Header.Set("Origin", "https://dev.azure.com")
	req.Header.Set("Referer", "https://dev.azure.com/")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("azure: http delete request: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if isHTMLResponse(resp.Header.Get("Content-Type"), respBody) {
		return fmt.Errorf("azure: Azure returned HTML/sign-in response; token may be expired or auth mode invalid")
	}
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("azure: unexpected delete status %d%s", resp.StatusCode, sanitizedResponseMessage(respBody))
	}
	return nil
}

// setAuthHeader applies the client's configured credential (PAT via Basic, or
// OAuth via Bearer) to an outgoing request. Shared by TimeLog and identity
// requests so all Azure calls use identical auth.
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
//
// AssignedToID/AssignedToDisplayName are populated from System.AssignedTo —
// both are blank for an unassigned work item. AssignedToID is the same
// identity id FetchIdentity resolves (Config.UserID), so callers enforcing
// "only the assignee may do X" (see work_items.go's close/recreate handlers)
// must compare against AssignedToID, not the display name, which is not
// guaranteed unique.
type AssignedWorkItem struct {
	ID                    int    `json:"id"`
	Title                 string `json:"title"`
	Type                  string `json:"type"`  // System.WorkItemType, e.g. "Bug", "Task"
	State                 string `json:"state"` // System.State, e.g. "Active", "New"
	AssignedToID          string `json:"assigned_to_id,omitempty"`
	AssignedToDisplayName string `json:"assigned_to_display_name,omitempty"`
}

// azureIdentityRef is the identity reference shape Azure DevOps embeds for
// fields like System.AssignedTo (json:"id" is the same VSSPS-derived id
// FetchIdentity resolves — see AssignedWorkItem's doc comment). Shared by
// fetchWorkItemDetails and FetchWorkItemFull, both of which request
// System.AssignedTo.
type azureIdentityRef struct {
	DisplayName string `json:"displayName"`
	ID          string `json:"id"`
}

// resolve extracts (id, displayName) from a possibly-nil identity ref — nil
// means the work item is unassigned, which is a normal state, not an error.
func (r *azureIdentityRef) resolve() (id, displayName string) {
	if r == nil {
		return "", ""
	}
	return r.ID, r.DisplayName
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

// FetchWorkItemsByIDs resolves current title/type/state for an arbitrary set
// of work item ids (e.g. the local Azure activity catalog), regardless of
// assignment or state — unlike FetchAssignedWorkItems, which is scoped to
// @Me and open states only.
func (c *Client) FetchWorkItemsByIDs(ctx context.Context, ids []int) ([]AssignedWorkItem, error) {
	if strings.TrimSpace(c.cfg.Token) == "" {
		return nil, fmt.Errorf("azure: token is not configured")
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
			"https://dev.azure.com/%s/_apis/wit/workitems?ids=%s&fields=System.Id,System.Title,System.WorkItemType,System.State,System.AssignedTo&api-version=7.1",
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
					Title      string            `json:"System.Title"`
					Type       string            `json:"System.WorkItemType"`
					State      string            `json:"System.State"`
					AssignedTo *azureIdentityRef `json:"System.AssignedTo"`
				} `json:"fields"`
			} `json:"value"`
		}
		if err := json.Unmarshal(respBody, &parsed); err != nil {
			return nil, fmt.Errorf("azure: decode workitems response: %w", err)
		}

		for _, v := range parsed.Value {
			assignedToID, assignedToDisplayName := v.Fields.AssignedTo.resolve()
			out = append(out, AssignedWorkItem{
				ID:                    v.ID,
				Title:                 v.Fields.Title,
				Type:                  v.Fields.Type,
				State:                 v.Fields.State,
				AssignedToID:          assignedToID,
				AssignedToDisplayName: assignedToDisplayName,
			})
		}
	}
	return out, nil
}

// colombiaLocation is a fixed UTC-5 offset (no DST) used to derive the
// "optimistic date" fields the shared Azure process template expects
// (Custom.InicioOptimista/FinOptimista, Scheduling.StartDate) as midnight in
// Colombia, regardless of what timezone this binary happens to run in.
var colombiaLocation = time.FixedZone("COT", -5*3600)

// timeNow is a seam for tests to freeze "now" when asserting the
// optimistic-date fields CreateWorkItem stamps onto a new work item.
var timeNow = time.Now

// patchOp is one operation in an Azure DevOps json-patch document. A struct
// (rather than map[string]any) so field order in the marshaled JSON is
// always op/path/value, matching Azure's own examples and keeping test
// fixtures diff-friendly.
type patchOp struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value"`
}

// CreateWorkItemInput is the caller-provided subset of fields for a new Task
// work item; every other field (state, CMMI process fields, optimistic
// dates) is fixed by CreateWorkItem to match the shared process template.
type CreateWorkItemInput struct {
	Title            string
	Description      string // optional
	AreaPath         string
	IterationPath    string
	OriginalEstimate float64 // hours; <=0 defaults to 24
}

// CreatedWorkItem is the outcome of CreateWorkItem/CreateAndActivateWorkItem.
type CreatedWorkItem struct {
	ID    int
	State string
}

// ActivationError wraps a failure to activate a work item that WAS
// successfully created. Callers must not treat this as a hard failure: the
// work item exists in Azure (in "Proposed" state) and its ID must not be
// lost — errors.As lets a caller recover WorkItem to report a partial
// success instead of pretending nothing happened.
type ActivationError struct {
	WorkItem CreatedWorkItem
	Err      error
}

func (e *ActivationError) Error() string {
	return fmt.Sprintf("work item %d created but activation failed: %v", e.WorkItem.ID, e.Err)
}

func (e *ActivationError) Unwrap() error { return e.Err }

// ClassificationNode is one node of an Azure DevOps Area/Iteration path
// tree, as returned by the classificationnodes API (only the fields this
// client needs to flatten into selectable paths are mapped).
type ClassificationNode struct {
	Name     string               `json:"name"`
	Children []ClassificationNode `json:"children,omitempty"`
}

// toOptimisticDate formats t (expected to already be in colombiaLocation) as
// the "midnight in Colombia" timestamp the shared CMMI process template
// expects: YYYY-MM-DDT05:00:00Z.
func toOptimisticDate(t time.Time) string {
	return t.Format("2006-01-02") + "T05:00:00Z"
}

// CreateWorkItem creates a new Task work item in Proposed/New, matching the
// field set the team's shared Azure DevOps CMMI process template requires
// (see the coworker reference implementation this was ported from). It does
// NOT activate the item — callers that want it usable immediately should use
// CreateAndActivateWorkItem.
func (c *Client) CreateWorkItem(ctx context.Context, in CreateWorkItemInput) (CreatedWorkItem, error) {
	if !c.Enabled() {
		return CreatedWorkItem{}, fmt.Errorf("Azure TimeLog token is not configured")
	}
	if strings.TrimSpace(c.cfg.User) == "" {
		return CreatedWorkItem{}, fmt.Errorf("azure: identity not resolved — reconnect the Azure token (FetchIdentity) before creating work items")
	}

	estimate := in.OriginalEstimate
	if estimate <= 0 {
		estimate = 24
	}

	now := timeNow().In(colombiaLocation)
	lastDayOfMonth := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, colombiaLocation)

	ops := []patchOp{
		{Op: "add", Path: "/fields/System.Title", Value: in.Title},
	}
	if desc := strings.TrimSpace(in.Description); desc != "" {
		ops = append(ops, patchOp{Op: "add", Path: "/fields/System.Description", Value: desc})
	}
	ops = append(ops,
		patchOp{Op: "add", Path: "/fields/System.AreaPath", Value: in.AreaPath},
		patchOp{Op: "add", Path: "/fields/System.IterationPath", Value: in.IterationPath},
		patchOp{Op: "add", Path: "/fields/System.AssignedTo", Value: c.cfg.User},
		patchOp{Op: "add", Path: "/fields/System.State", Value: "Proposed"},
		patchOp{Op: "add", Path: "/fields/System.Reason", Value: "New"},
		patchOp{Op: "add", Path: "/fields/Microsoft.VSTS.Scheduling.OriginalEstimate", Value: estimate},
		patchOp{Op: "add", Path: "/fields/Microsoft.VSTS.Scheduling.RemainingWork", Value: estimate},
		patchOp{Op: "add", Path: "/fields/Microsoft.VSTS.Scheduling.CompletedWork", Value: 0},
		patchOp{Op: "add", Path: "/fields/Microsoft.VSTS.Common.Triage", Value: "Pending"},
		patchOp{Op: "add", Path: "/fields/Microsoft.VSTS.Common.Discipline", Value: "Desarrollo de Software (Codificación)"},
		patchOp{Op: "add", Path: "/fields/Microsoft.VSTS.CMMI.Blocked", Value: "No"},
		patchOp{Op: "add", Path: "/fields/Microsoft.VSTS.CMMI.TaskType", Value: "Planned"},
		patchOp{Op: "add", Path: "/fields/Microsoft.VSTS.CMMI.RequiresReview", Value: "No"},
		patchOp{Op: "add", Path: "/fields/Microsoft.VSTS.CMMI.RequiresTest", Value: "No"},
		patchOp{Op: "add", Path: "/fields/Microsoft.VSTS.Scheduling.StartDate", Value: toOptimisticDate(now)},
		patchOp{Op: "add", Path: "/fields/Custom.InicioOptimista", Value: toOptimisticDate(now)},
		patchOp{Op: "add", Path: "/fields/Custom.FinOptimista", Value: toOptimisticDate(lastDayOfMonth)},
	)

	body, err := json.Marshal(ops)
	if err != nil {
		return CreatedWorkItem{}, fmt.Errorf("azure: marshal create work item payload: %w", err)
	}

	url := fmt.Sprintf(
		"https://dev.azure.com/%s/%s/_apis/wit/workitems/$Task?api-version=7.1-preview.3",
		c.cfg.Org, c.cfg.TeamProject,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return CreatedWorkItem{}, fmt.Errorf("azure: build create work item request: %w", err)
	}
	c.setAuthHeader(req)
	req.Header.Set("Content-Type", "application/json-patch+json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return CreatedWorkItem{}, fmt.Errorf("azure: create work item http request: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return CreatedWorkItem{}, fmt.Errorf("azure: unexpected create work item status %d%s", resp.StatusCode, sanitizedResponseMessage(respBody))
	}
	if isHTMLResponse(resp.Header.Get("Content-Type"), respBody) {
		return CreatedWorkItem{}, fmt.Errorf("azure: Azure returned HTML/sign-in response; token may be expired or auth mode invalid")
	}

	var created struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(respBody, &created); err != nil {
		return CreatedWorkItem{}, fmt.Errorf("azure: decode create work item response: %w", err)
	}
	if created.ID == 0 {
		return CreatedWorkItem{}, fmt.Errorf("azure: success response missing work item id")
	}
	return CreatedWorkItem{ID: created.ID, State: "Proposed"}, nil
}

// ActivateWorkItem transitions a Task from Proposed to Active/Accepted. This
// is a non-obvious two-step PATCH: the first PATCH sets State=Active plus
// Reason/Custom.Resolucion=Accepted, but Azure's workflow resets Reason to
// its own default as a side effect of the state transition — so a second
// PATCH re-affirms Reason/Custom.Resolucion=Accepted for it to actually
// stick. Both must succeed; if the first fails, the second is not attempted.
func (c *Client) ActivateWorkItem(ctx context.Context, id int) error {
	if err := c.patchWorkItem(ctx, id, []patchOp{
		{Op: "add", Path: "/fields/System.State", Value: "Active"},
		{Op: "add", Path: "/fields/System.Reason", Value: "Accepted"},
		{Op: "add", Path: "/fields/Custom.Resolucion", Value: "Accepted"},
	}); err != nil {
		return err
	}
	return c.patchWorkItem(ctx, id, []patchOp{
		{Op: "add", Path: "/fields/System.Reason", Value: "Accepted"},
		{Op: "add", Path: "/fields/Custom.Resolucion", Value: "Accepted"},
	})
}

// CreateAndActivateWorkItem creates a Task and immediately activates it. If
// creation fails, no work item exists and the error is returned as-is. If
// creation succeeds but activation fails, the work item still exists in
// Azure (Proposed) — that is reported as a partial success: the returned
// CreatedWorkItem carries the real ID/State and the error is an
// *ActivationError a caller can unwrap for the underlying activation cause.
func (c *Client) CreateAndActivateWorkItem(ctx context.Context, in CreateWorkItemInput) (CreatedWorkItem, error) {
	created, err := c.CreateWorkItem(ctx, in)
	if err != nil {
		return CreatedWorkItem{}, err
	}
	if err := c.ActivateWorkItem(ctx, created.ID); err != nil {
		return created, &ActivationError{WorkItem: created, Err: err}
	}
	created.State = "Active"
	return created, nil
}

// patchWorkItem sends one json-patch PATCH to an existing work item. Shared
// by the two ActivateWorkItem steps so the request-building/error-handling
// code exists in exactly one place.
func (c *Client) patchWorkItem(ctx context.Context, id int, ops []patchOp) error {
	body, err := json.Marshal(ops)
	if err != nil {
		return fmt.Errorf("azure: marshal patch work item payload: %w", err)
	}

	url := fmt.Sprintf(
		"https://dev.azure.com/%s/%s/_apis/wit/workitems/%d?api-version=7.1-preview.3",
		c.cfg.Org, c.cfg.TeamProject, id,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("azure: build patch work item request: %w", err)
	}
	c.setAuthHeader(req)
	req.Header.Set("Content-Type", "application/json-patch+json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("azure: patch work item http request: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("azure: unexpected patch work item status %d%s", resp.StatusCode, sanitizedResponseMessage(respBody))
	}
	if isHTMLResponse(resp.Header.Get("Content-Type"), respBody) {
		return fmt.Errorf("azure: Azure returned HTML/sign-in response; token may be expired or auth mode invalid")
	}
	return nil
}

// FetchClassificationTree returns the full Area or Iteration path tree
// (kind must be "areas" or "iterations") for the configured team project, so
// callers can offer a searchable picker instead of requiring a hand-typed
// path.
func (c *Client) FetchClassificationTree(ctx context.Context, kind string) (ClassificationNode, error) {
	if kind != "areas" && kind != "iterations" {
		return ClassificationNode{}, fmt.Errorf("azure: kind must be %q or %q, got %q", "areas", "iterations", kind)
	}
	if !c.Enabled() {
		return ClassificationNode{}, fmt.Errorf("Azure TimeLog token is not configured")
	}

	url := fmt.Sprintf(
		"https://dev.azure.com/%s/%s/_apis/wit/classificationnodes/%s?$depth=10&api-version=7.1-preview.2",
		c.cfg.Org, c.cfg.TeamProject, kind,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ClassificationNode{}, fmt.Errorf("azure: build classification nodes request: %w", err)
	}
	c.setAuthHeader(req)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return ClassificationNode{}, fmt.Errorf("azure: classification nodes http request: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ClassificationNode{}, fmt.Errorf("azure: unexpected classification nodes status %d%s", resp.StatusCode, sanitizedResponseMessage(respBody))
	}
	if isHTMLResponse(resp.Header.Get("Content-Type"), respBody) {
		return ClassificationNode{}, fmt.Errorf("azure: Azure returned HTML/sign-in response; token may be expired or auth mode invalid")
	}

	var node ClassificationNode
	if err := json.Unmarshal(respBody, &node); err != nil {
		return ClassificationNode{}, fmt.Errorf("azure: decode classification nodes response: %w", err)
	}
	return node, nil
}

// WorkItemFull is the richer single-work-item read used by the Close/Recreate
// flows (title, description, area/iteration paths, and estimate) — unlike
// AssignedWorkItem, which only carries the fields the list/table view needs.
type WorkItemFull struct {
	ID                    int
	Title                 string
	Description           string
	Type                  string
	State                 string
	AreaPath              string
	IterationPath         string
	OriginalEstimate      float64
	AssignedToID          string // see AssignedWorkItem's doc comment — compare this, not AssignedToDisplayName
	AssignedToDisplayName string
}

// FetchWorkItemFull resolves the full field set for a single work item by
// id, org-scoped (no team project segment — Azure DevOps work item ids are
// unique per-organization, same as fetchWorkItemDetails above). A 400 or 404
// response means the work item does not exist and is reported as (nil, nil),
// not an error, so callers can distinguish "doesn't exist" from a real
// failure without inspecting error text.
func (c *Client) FetchWorkItemFull(ctx context.Context, id int) (*WorkItemFull, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("Azure TimeLog token is not configured")
	}

	const fields = "System.Title,System.Description,System.WorkItemType,System.State," +
		"System.AreaPath,System.IterationPath,Microsoft.VSTS.Scheduling.OriginalEstimate,System.AssignedTo"
	url := fmt.Sprintf(
		"https://dev.azure.com/%s/_apis/wit/workitems/%d?fields=%s&api-version=7.1-preview.3",
		c.cfg.Org, id, fields,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("azure: build fetch work item request: %w", err)
	}
	c.setAuthHeader(req)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("azure: fetch work item http request: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("azure: unexpected fetch work item status %d%s", resp.StatusCode, sanitizedResponseMessage(respBody))
	}
	if isHTMLResponse(resp.Header.Get("Content-Type"), respBody) {
		return nil, fmt.Errorf("azure: Azure returned HTML/sign-in response; token may be expired or auth mode invalid")
	}

	var parsed struct {
		ID     int `json:"id"`
		Fields struct {
			Title            string            `json:"System.Title"`
			Description      string            `json:"System.Description"`
			Type             string            `json:"System.WorkItemType"`
			State            string            `json:"System.State"`
			AreaPath         string            `json:"System.AreaPath"`
			IterationPath    string            `json:"System.IterationPath"`
			OriginalEstimate float64           `json:"Microsoft.VSTS.Scheduling.OriginalEstimate"`
			AssignedTo       *azureIdentityRef `json:"System.AssignedTo"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("azure: decode fetch work item response: %w", err)
	}
	assignedToID, assignedToDisplayName := parsed.Fields.AssignedTo.resolve()
	return &WorkItemFull{
		ID:                    parsed.ID,
		Title:                 parsed.Fields.Title,
		Description:           parsed.Fields.Description,
		Type:                  parsed.Fields.Type,
		State:                 parsed.Fields.State,
		AreaPath:              parsed.Fields.AreaPath,
		IterationPath:         parsed.Fields.IterationPath,
		OriginalEstimate:      parsed.Fields.OriginalEstimate,
		AssignedToID:          assignedToID,
		AssignedToDisplayName: assignedToDisplayName,
	}, nil
}

// TimeLogDocument is one entry from the TimeLog extension's Documents
// collection — the same collection PostTimeEntry/DeleteTimeEntry write to.
type TimeLogDocument struct {
	WorkItemID int `json:"workItemId"`
	Minutes    int `json:"minutes"`
}

// FetchTimeLogDocuments lists every TimeLog document in the configured org's
// collection (not scoped to one work item or one user — the same collection
// GET the coworker reference implementation uses to compute registered
// hours per work item, see fetchTimeLogs in that codebase). The response is
// either a bare JSON array or an {"items": [...]} envelope; both are
// accepted since the collection API has been observed to return either
// shape.
func (c *Client) FetchTimeLogDocuments(ctx context.Context) ([]TimeLogDocument, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("Azure TimeLog token is not configured")
	}

	url := fmt.Sprintf(
		"https://extmgmt.dev.azure.com/%s/_apis/ExtensionManagement/InstalledExtensions/TimeLog/time-logging-extension/Data/Scopes/Default/Current/Collections/TimeLogData/Documents",
		c.cfg.Org,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("azure: build fetch time log documents request: %w", err)
	}
	c.setAuthHeader(req)
	req.Header.Set("Accept", "application/json;api-version=3.1-preview.1;excludeUrls=true")
	req.Header.Set("X-TFS-FedAuthRedirect", "Suppress")
	req.Header.Set("Origin", "https://dev.azure.com")
	req.Header.Set("Referer", "https://dev.azure.com/")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("azure: fetch time log documents http request: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("azure: unexpected fetch time log documents status %d%s", resp.StatusCode, sanitizedResponseMessage(respBody))
	}
	if isHTMLResponse(resp.Header.Get("Content-Type"), respBody) {
		return nil, fmt.Errorf("azure: Azure returned HTML/sign-in response; token may be expired or auth mode invalid")
	}

	var docs []TimeLogDocument
	if err := json.Unmarshal(respBody, &docs); err != nil {
		var wrapped struct {
			Items []TimeLogDocument `json:"items"`
		}
		if err2 := json.Unmarshal(respBody, &wrapped); err2 != nil {
			return nil, fmt.Errorf("azure: decode time log documents response: %w", err)
		}
		docs = wrapped.Items
	}
	return docs, nil
}

// SyncEffortFromTimeLog reconciles a work item's CompletedWork/RemainingWork
// fields with hours actually logged in TimeLog: CompletedWork becomes the
// total logged (rounded to 2 decimals), RemainingWork becomes
// max(0, originalEstimate-total) — floored at 0 so over-logging never
// produces a negative remaining value. Returns the total hours synced.
func (c *Client) SyncEffortFromTimeLog(ctx context.Context, id int, originalEstimate float64) (float64, error) {
	docs, err := c.FetchTimeLogDocuments(ctx)
	if err != nil {
		return 0, err
	}
	var minutes int
	for _, d := range docs {
		if d.WorkItemID == id {
			minutes += d.Minutes
		}
	}
	total := math.Round(float64(minutes)/60*100) / 100

	remaining := 0.0
	if originalEstimate > 0 {
		remaining = math.Max(0, math.Round((originalEstimate-total)*100)/100)
	}

	if err := c.patchWorkItem(ctx, id, []patchOp{
		{Op: "add", Path: "/fields/Microsoft.VSTS.Scheduling.CompletedWork", Value: total},
		{Op: "add", Path: "/fields/Microsoft.VSTS.Scheduling.RemainingWork", Value: remaining},
	}); err != nil {
		return total, err
	}
	return total, nil
}

// IsClosedState reports whether state is a terminal closed state, in either
// language the shared process template uses. Exported so callers (e.g. the
// REST handler layer) can short-circuit a redundant close attempt without
// duplicating this comparison.
func IsClosedState(state string) bool {
	return state == "Closed" || state == "Cerrado"
}

// closeReason is the fixed CMMI resolution value the shared process template
// requires for a closed Task — used for both System.Reason and
// Custom.Resolucion, must match verbatim (case and wording).
const closeReason = "Complete and Does Not Require Review/Test"

// CloseWorkItem transitions a Task to Closed/"Complete and Does Not Require
// Review/Test". Already-closed states are a no-op. Proposed tasks have no
// direct transition to Closed, so they are activated first (same two-step
// activation as ActivateWorkItem). The close itself is also a two-step PATCH
// — Azure resets Reason as a side effect of the state transition, so a
// second PATCH re-affirms Reason/Custom.Resolucion, mirroring
// ActivateWorkItem's identical gotcha.
func (c *Client) CloseWorkItem(ctx context.Context, id int, currentState string) error {
	if IsClosedState(currentState) {
		return nil
	}
	if currentState == "Proposed" {
		if err := c.ActivateWorkItem(ctx, id); err != nil {
			return err
		}
	}
	if err := c.patchWorkItem(ctx, id, []patchOp{
		{Op: "add", Path: "/fields/System.State", Value: "Closed"},
		{Op: "add", Path: "/fields/System.Reason", Value: closeReason},
		{Op: "add", Path: "/fields/Custom.Resolucion", Value: closeReason},
	}); err != nil {
		return err
	}
	return c.patchWorkItem(ctx, id, []patchOp{
		{Op: "add", Path: "/fields/System.Reason", Value: closeReason},
		{Op: "add", Path: "/fields/Custom.Resolucion", Value: closeReason},
	})
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
