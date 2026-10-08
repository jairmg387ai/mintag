package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// FieldSubarea is the "Subárea" picklist on Task work items of the shared
// CMMI process. CMMI sets it on the bug correction Task it creates (e.g.
// "Desarrollo"); its allowed values are read dynamically per team project
// via FetchTaskFieldAllowedValues rather than hardcoded.
const FieldSubarea = "Custom.860becab-e144-442b-8283-c2dd65cc7191"

// correctionTaskTitlePrefix is how the correction Task under a Bug is named
// ("Atención y/o Corrección del defecto <bugId>"); it identifies existing
// correction tasks among a bug's children.
const correctionTaskTitlePrefix = "Atención y/o Corrección"

// CorrectionTaskTitle is the title mintag suggests for a bug's correction
// Task. It always uses the BUG id (CMMI has been observed using the task's
// own id instead, which is wrong).
func CorrectionTaskTitle(bugID int) string {
	return fmt.Sprintf("%s del defecto %d", correctionTaskTitlePrefix, bugID)
}

// IsCorrectionTaskTitle reports whether title names a bug correction Task
// (case-insensitive prefix match, leading spaces ignored).
func IsCorrectionTaskTitle(title string) bool {
	t := strings.ToLower(strings.TrimSpace(title))
	return t != "" && strings.HasPrefix(t, strings.ToLower(correctionTaskTitlePrefix))
}

// IdentityRef is an Azure DevOps identity as embedded in System.AssignedTo.
// UniqueName (the login/email) is what a json-patch System.AssignedTo write
// accepts unambiguously.
type IdentityRef struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	UniqueName  string `json:"unique_name"`
}

// CorrectionTaskSummary is one existing correction Task under a Bug.
type CorrectionTaskSummary struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	State string `json:"state"`
}

// BugCorrectionTaskDraft is everything the correction-task form needs to
// prefill from a Bug: where to create the Task (team project, area), who to
// assign it to, and which correction tasks already exist (duplicate guard).
type BugCorrectionTaskDraft struct {
	BugID                   int
	BugTitle                string
	Type                    string
	State                   string
	TeamProject             string
	AreaPath                string
	IterationPath           string
	AssignedTo              IdentityRef
	SuggestedTitle          string
	ExistingCorrectionTasks []CorrectionTaskSummary
}

// IsBug reports whether the drafted work item is actually a Bug.
func (d *BugCorrectionTaskDraft) IsBug() bool { return isBugType(d.Type) }

// FetchBugCorrectionTaskDraft reads work item bugID with $expand=relations
// and, when it is a Bug, batch-reads its children to list existing
// correction Tasks. A 400/404 means the work item does not exist and is
// reported as (nil, nil), same contract as FetchWorkItemHierarchy. A non-Bug
// work item is returned as-is (no children read) so callers can reject it
// with a specific error.
func (c *Client) FetchBugCorrectionTaskDraft(ctx context.Context, bugID int) (*BugCorrectionTaskDraft, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("Azure TimeLog token is not configured")
	}

	endpoint := fmt.Sprintf("https://dev.azure.com/%s/_apis/wit/workitems/%d?$expand=relations&api-version=7.1", c.cfg.Org, bugID)
	respBody, status, err := c.getJSON(ctx, endpoint, "bug")
	if err != nil {
		return nil, err
	}
	if status == http.StatusBadRequest || status == http.StatusNotFound {
		return nil, nil
	}

	var parsed struct {
		ID     int `json:"id"`
		Fields struct {
			Title         string `json:"System.Title"`
			Type          string `json:"System.WorkItemType"`
			State         string `json:"System.State"`
			TeamProject   string `json:"System.TeamProject"`
			AreaPath      string `json:"System.AreaPath"`
			IterationPath string `json:"System.IterationPath"`
			AssignedTo    *struct {
				ID          string `json:"id"`
				DisplayName string `json:"displayName"`
				UniqueName  string `json:"uniqueName"`
			} `json:"System.AssignedTo"`
		} `json:"fields"`
		Relations []struct {
			Rel string `json:"rel"`
			URL string `json:"url"`
		} `json:"relations"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("azure: decode bug response: %w", err)
	}

	d := &BugCorrectionTaskDraft{
		BugID:          parsed.ID,
		BugTitle:       parsed.Fields.Title,
		Type:           parsed.Fields.Type,
		State:          parsed.Fields.State,
		TeamProject:    parsed.Fields.TeamProject,
		AreaPath:       parsed.Fields.AreaPath,
		IterationPath:  parsed.Fields.IterationPath,
		SuggestedTitle: CorrectionTaskTitle(parsed.ID),
	}
	if a := parsed.Fields.AssignedTo; a != nil {
		d.AssignedTo = IdentityRef{ID: a.ID, DisplayName: a.DisplayName, UniqueName: a.UniqueName}
	}
	if !d.IsBug() {
		return d, nil
	}

	var childIDs []int
	for _, r := range parsed.Relations {
		if r.Rel != relHierarchyForward {
			continue
		}
		if id, ok := workItemIDFromURL(r.URL); ok {
			childIDs = append(childIDs, id)
		}
	}
	if len(childIDs) == 0 {
		return d, nil
	}
	children, err := c.fetchWorkItemDetails(ctx, childIDs)
	if err != nil {
		return nil, err
	}
	for _, ch := range children {
		if strings.EqualFold(strings.TrimSpace(ch.Type), "Task") && IsCorrectionTaskTitle(ch.Title) {
			d.ExistingCorrectionTasks = append(d.ExistingCorrectionTasks, CorrectionTaskSummary{ID: ch.ID, Title: ch.Title, State: ch.State})
		}
	}
	return d, nil
}

// FetchTaskFieldAllowedValues returns the allowed values of fieldRef on the
// Task work item type in the given team project (picklist fields such as
// FieldSubarea).
func (c *Client) FetchTaskFieldAllowedValues(ctx context.Context, project, fieldRef string) ([]string, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("Azure TimeLog token is not configured")
	}
	if strings.TrimSpace(project) == "" {
		return nil, fmt.Errorf("azure: team project is required")
	}

	endpoint := fmt.Sprintf(
		"https://dev.azure.com/%s/%s/_apis/wit/workitemtypes/Task/fields/%s?$expand=allowedValues&api-version=7.1",
		c.cfg.Org, url.PathEscape(project), url.PathEscape(fieldRef),
	)
	respBody, status, err := c.getJSON(ctx, endpoint, "field allowed values")
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("azure: unexpected field allowed values status %d%s", status, sanitizedResponseMessage(respBody))
	}

	var parsed struct {
		AllowedValues []string `json:"allowedValues"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("azure: decode field allowed values response: %w", err)
	}
	if parsed.AllowedValues == nil {
		return []string{}, nil
	}
	return parsed.AllowedValues, nil
}

// getJSON performs an authenticated GET and returns the body and status.
// Non-2xx statuses other than 400/404 are turned into errors; 400/404 are
// returned to the caller (with a nil error) so it can decide whether they
// mean "not found". An HTML sign-in page on a 2xx is always an error.
func (c *Client) getJSON(ctx context.Context, endpoint, what string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("azure: build %s request: %w", what, err)
	}
	c.setAuthHeader(req)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("azure: %s http request: %w", what, err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusNotFound {
		return respBody, resp.StatusCode, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.StatusCode, fmt.Errorf("azure: unexpected %s status %d%s", what, resp.StatusCode, sanitizedResponseMessage(respBody))
	}
	if isHTMLResponse(resp.Header.Get("Content-Type"), respBody) {
		return nil, resp.StatusCode, fmt.Errorf("azure: Azure returned HTML/sign-in response; token may be expired or auth mode invalid")
	}
	return respBody, resp.StatusCode, nil
}

// BugCorrectionTaskInput is the caller-provided field set for a bug
// correction Task. Every field except Description is required.
type BugCorrectionTaskInput struct {
	BugID            int
	TeamProject      string // the bug's System.TeamProject
	Title            string
	Description      string // optional
	AreaPath         string
	IterationPath    string
	AssignedTo       string // uniqueName (login/email)
	Subarea          string // one of FieldSubarea's allowed values
	OriginalEstimate float64
}

func (in BugCorrectionTaskInput) validate() error {
	switch {
	case in.BugID <= 0:
		return fmt.Errorf("azure: bug id is required")
	case strings.TrimSpace(in.TeamProject) == "":
		return fmt.Errorf("azure: team project is required")
	case strings.TrimSpace(in.Title) == "":
		return fmt.Errorf("azure: title is required")
	case strings.TrimSpace(in.AreaPath) == "":
		return fmt.Errorf("azure: area path is required")
	case strings.TrimSpace(in.IterationPath) == "":
		return fmt.Errorf("azure: iteration path is required")
	case strings.TrimSpace(in.AssignedTo) == "":
		return fmt.Errorf("azure: assignee is required")
	case strings.TrimSpace(in.Subarea) == "":
		return fmt.Errorf("azure: subarea is required")
	case in.OriginalEstimate <= 0:
		return fmt.Errorf("azure: original estimate must be greater than 0")
	}
	return nil
}

// buildBugCorrectionTaskOps builds the json-patch document matching what the
// CMMI team sets on a correction Task: Proposed/New, Planned, priority 2,
// estimate as both Original Estimate and Remaining Work, Subárea, and a
// Hierarchy-Reverse relation making the Bug its parent.
func (c *Client) buildBugCorrectionTaskOps(in BugCorrectionTaskInput) []patchOp {
	ops := []patchOp{
		{Op: "add", Path: "/fields/System.Title", Value: strings.TrimSpace(in.Title)},
	}
	if desc := strings.TrimSpace(in.Description); desc != "" {
		ops = append(ops, patchOp{Op: "add", Path: "/fields/System.Description", Value: desc})
	}
	return append(ops,
		patchOp{Op: "add", Path: "/fields/System.AreaPath", Value: strings.TrimSpace(in.AreaPath)},
		patchOp{Op: "add", Path: "/fields/System.IterationPath", Value: strings.TrimSpace(in.IterationPath)},
		patchOp{Op: "add", Path: "/fields/System.AssignedTo", Value: strings.TrimSpace(in.AssignedTo)},
		patchOp{Op: "add", Path: "/fields/System.State", Value: "Proposed"},
		patchOp{Op: "add", Path: "/fields/System.Reason", Value: "New"},
		patchOp{Op: "add", Path: "/fields/Microsoft.VSTS.Scheduling.OriginalEstimate", Value: in.OriginalEstimate},
		patchOp{Op: "add", Path: "/fields/Microsoft.VSTS.Scheduling.RemainingWork", Value: in.OriginalEstimate},
		patchOp{Op: "add", Path: "/fields/Microsoft.VSTS.CMMI.TaskType", Value: "Planned"},
		patchOp{Op: "add", Path: "/fields/Microsoft.VSTS.Common.Priority", Value: 2},
		patchOp{Op: "add", Path: "/fields/" + FieldSubarea, Value: strings.TrimSpace(in.Subarea)},
		patchOp{Op: "add", Path: "/relations/-", Value: map[string]any{
			"rel": relHierarchyReverse,
			"url": fmt.Sprintf("https://dev.azure.com/%s/_apis/wit/workItems/%d", c.cfg.Org, in.BugID),
		}},
	)
}

// CreateBugCorrectionTask creates the correction Task under a Bug in the
// bug's own team project. It is left in Proposed (not activated), matching
// what CMMI does when it creates these tasks.
func (c *Client) CreateBugCorrectionTask(ctx context.Context, in BugCorrectionTaskInput) (CreatedWorkItem, error) {
	if !c.Enabled() {
		return CreatedWorkItem{}, fmt.Errorf("Azure TimeLog token is not configured")
	}
	if err := in.validate(); err != nil {
		return CreatedWorkItem{}, err
	}
	id, err := c.postNewWorkItem(ctx, strings.TrimSpace(in.TeamProject), "Task", c.buildBugCorrectionTaskOps(in))
	if err != nil {
		return CreatedWorkItem{}, err
	}
	return CreatedWorkItem{ID: id, State: "Proposed"}, nil
}
