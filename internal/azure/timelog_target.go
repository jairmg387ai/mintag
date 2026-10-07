package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// Azure DevOps link types for the parent/child hierarchy, as they appear in a
// work item's "relations" (rel) when fetched with $expand=relations.
const (
	relHierarchyForward = "System.LinkTypes.Hierarchy-Forward" // this item -> child
	relHierarchyReverse = "System.LinkTypes.Hierarchy-Reverse" // this item -> parent
)

// WorkItemHierarchy is a single work item plus its direct parent/child links,
// as needed to decide whether TimeLog hours may be logged against it (see
// EvaluateTimeLogTarget). ParentID is 0 when the item has no parent.
type WorkItemHierarchy struct {
	ID                    int
	Title                 string
	Type                  string
	State                 string
	AssignedToID          string // see AssignedWorkItem's doc comment — compare this, not AssignedToDisplayName
	AssignedToDisplayName string
	OriginalEstimate      float64 // 0 when the work item has no estimate
	ParentID              int
	ChildIDs              []int
}

// TimeLogTargetError is a rule rejection from EvaluateTimeLogTarget /
// CheckTimeLogTarget: the work item exists and was read fine, but hours must
// not be logged on it. Callers use errors.As to tell it apart from a
// transport/auth failure reading Azure. Error() is the user-facing message.
type TimeLogTargetError struct {
	WorkItemID int
	Message    string
}

func (e *TimeLogTargetError) Error() string { return e.Message }

// FetchWorkItemHierarchy reads one work item with $expand=relations, org-scoped
// like FetchWorkItemFull. Azure rejects `fields` combined with `$expand`, so
// the full field set comes back and only what WorkItemHierarchy needs is
// decoded. A 400 or 404 response means the work item does not exist and is
// reported as (nil, nil), same contract as FetchWorkItemFull.
func (c *Client) FetchWorkItemHierarchy(ctx context.Context, id int) (*WorkItemHierarchy, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("Azure TimeLog token is not configured")
	}

	url := fmt.Sprintf("https://dev.azure.com/%s/_apis/wit/workitems/%d?$expand=relations&api-version=7.1", c.cfg.Org, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("azure: build work item hierarchy request: %w", err)
	}
	c.setAuthHeader(req)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("azure: work item hierarchy http request: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("azure: unexpected work item hierarchy status %d%s", resp.StatusCode, sanitizedResponseMessage(respBody))
	}
	if isHTMLResponse(resp.Header.Get("Content-Type"), respBody) {
		return nil, fmt.Errorf("azure: Azure returned HTML/sign-in response; token may be expired or auth mode invalid")
	}

	var parsed struct {
		ID     int `json:"id"`
		Fields struct {
			Title            string            `json:"System.Title"`
			Type             string            `json:"System.WorkItemType"`
			State            string            `json:"System.State"`
			OriginalEstimate float64           `json:"Microsoft.VSTS.Scheduling.OriginalEstimate"`
			AssignedTo       *azureIdentityRef `json:"System.AssignedTo"`
		} `json:"fields"`
		Relations []struct {
			Rel string `json:"rel"`
			URL string `json:"url"`
		} `json:"relations"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("azure: decode work item hierarchy response: %w", err)
	}

	assignedToID, assignedToDisplayName := parsed.Fields.AssignedTo.resolve()
	h := &WorkItemHierarchy{
		ID:                    parsed.ID,
		Title:                 parsed.Fields.Title,
		Type:                  parsed.Fields.Type,
		State:                 parsed.Fields.State,
		AssignedToID:          assignedToID,
		AssignedToDisplayName: assignedToDisplayName,
		OriginalEstimate:      parsed.Fields.OriginalEstimate,
	}
	for _, r := range parsed.Relations {
		linkedID, ok := workItemIDFromURL(r.URL)
		if !ok {
			continue
		}
		switch r.Rel {
		case relHierarchyForward:
			h.ChildIDs = append(h.ChildIDs, linkedID)
		case relHierarchyReverse:
			h.ParentID = linkedID
		}
	}
	return h, nil
}

// workItemIDFromURL extracts the trailing work item id from a relation URL
// (".../_apis/wit/workItems/171306"). Non-work-item relations (attachments,
// hyperlinks) don't end in an integer and are reported as !ok.
func workItemIDFromURL(u string) (int, bool) {
	id, err := strconv.Atoi(u[strings.LastIndex(u, "/")+1:])
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

// CheckTimeLogTarget reads the work item's hierarchy from Azure and applies
// EvaluateTimeLogTarget with the client's own identity (Config.UserID). It
// costs one call for a work item that is neither a Bug nor parented, and two
// otherwise (the item, then a batch read of its children or its parent).
//
// A nil error means hours may be logged. A *TimeLogTargetError is a rule
// rejection (including a work item that doesn't exist); any other error is a
// failure to read Azure and says nothing about the work item itself.
//
// The work item it read is returned whenever the read succeeded (allowed or
// rejected), so callers can reuse its fields — e.g. OriginalEstimate for the
// post-upload effort sync — instead of reading it again; it is nil when the
// work item doesn't exist or Azure couldn't be read.
func (c *Client) CheckTimeLogTarget(ctx context.Context, id int) (*WorkItemHierarchy, error) {
	item, err := c.FetchWorkItemHierarchy(ctx, id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, &TimeLogTargetError{WorkItemID: id, Message: fmt.Sprintf("work item %d was not found in Azure DevOps", id)}
	}

	var parent *AssignedWorkItem
	var children []AssignedWorkItem
	switch {
	case isBugType(item.Type):
		if len(item.ChildIDs) > 0 {
			if children, err = c.FetchWorkItemsByIDs(ctx, item.ChildIDs); err != nil {
				return item, err
			}
		}
	case item.ParentID != 0:
		parents, err := c.FetchWorkItemsByIDs(ctx, []int{item.ParentID})
		if err != nil {
			return item, err
		}
		if len(parents) > 0 {
			parent = &parents[0]
		}
	}
	return item, EvaluateTimeLogTarget(*item, parent, children, c.cfg.UserID)
}

// EvaluateTimeLogTarget is the pure TimeLog bug-guard rule, kept free of HTTP
// so it is unit-testable on its own:
//   - a Bug is always rejected; the message points at its child Tasks
//     assigned to the caller, so the user knows where the hours belong;
//   - a Task whose parent is a Bug is allowed only when assigned to the
//     caller;
//   - anything else (standalone task, task under a user story, ...) is
//     allowed unchanged.
//
// parent is the item's parent (nil when it has none or it couldn't be read)
// and children are the item's children; only the one relevant to item's type
// is consulted. An empty callerID (identity never resolved — see
// FetchIdentity) can't be compared against assignees, so — same trade-off as
// the server's ensureAssignedToCaller — the assignee check is skipped and a
// Bug's message lists every child Task instead of only the caller's.
func EvaluateTimeLogTarget(item WorkItemHierarchy, parent *AssignedWorkItem, children []AssignedWorkItem, callerID string) error {
	callerID = strings.TrimSpace(callerID)

	if isBugType(item.Type) {
		var tasks, mine []AssignedWorkItem
		for _, ch := range children {
			if !strings.EqualFold(ch.Type, "Task") {
				continue
			}
			tasks = append(tasks, ch)
			if callerID == "" || ch.AssignedToID == callerID {
				mine = append(mine, ch)
			}
		}
		sort.Slice(mine, func(i, j int) bool { return mine[i].ID < mine[j].ID })

		var msg string
		switch {
		case len(tasks) == 0:
			msg = fmt.Sprintf("work item %d is a Bug and has no child tasks; log hours on a child task assigned to you", item.ID)
		case len(mine) == 0:
			msg = fmt.Sprintf("work item %d is a Bug; log hours on one of its child tasks, but none of them is assigned to you", item.ID)
		case len(mine) == 1:
			msg = fmt.Sprintf("work item %d is a Bug; log hours on its child task: %s", item.ID, describeTask(mine[0]))
		default:
			listed := make([]string, len(mine))
			for i, ch := range mine {
				listed[i] = describeTask(ch)
			}
			msg = fmt.Sprintf("work item %d is a Bug; log hours on one of its child tasks: %s", item.ID, strings.Join(listed, ", "))
		}
		return &TimeLogTargetError{WorkItemID: item.ID, Message: msg}
	}

	if parent != nil && isBugType(parent.Type) && callerID != "" && item.AssignedToID != callerID {
		assignee := item.AssignedToDisplayName
		if assignee == "" {
			assignee = "no one (unassigned)"
		}
		return &TimeLogTargetError{
			WorkItemID: item.ID,
			Message: fmt.Sprintf("work item %d is a task of Bug %d assigned to %s, not you; log hours only on bug tasks assigned to you",
				item.ID, parent.ID, assignee),
		}
	}
	return nil
}

func isBugType(t string) bool { return strings.EqualFold(strings.TrimSpace(t), "Bug") }

func describeTask(w AssignedWorkItem) string {
	return strings.TrimSpace(fmt.Sprintf("#%d %s", w.ID, w.Title))
}
