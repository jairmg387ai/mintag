package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// AttachWorkItemParents fills ParentID/ParentTitle/ParentType on each item
// from its Azure parent link (System.LinkTypes.Hierarchy-Reverse). The parent
// always comes from Azure relations, never from the item's title, so it keeps
// working regardless of who creates the child task or how it is named.
//
// It costs one batched relations read for all items plus, when any item has a
// parent, one batched fields read of the distinct parent ids. Items without a
// parent get their parent fields cleared.
//
// On error the items are left exactly as they were, so callers can treat the
// lookup as best-effort and must not persist parent data from a failed call.
func (c *Client) AttachWorkItemParents(ctx context.Context, items []AssignedWorkItem) error {
	if len(items) == 0 {
		return nil
	}
	if strings.TrimSpace(c.cfg.Token) == "" {
		return fmt.Errorf("azure: token is not configured")
	}

	ids := make([]int, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}
	parentOf, err := c.fetchWorkItemParentIDs(ctx, ids)
	if err != nil {
		return err
	}

	seen := map[int]bool{}
	var parentIDs []int
	for _, pid := range parentOf {
		if !seen[pid] {
			seen[pid] = true
			parentIDs = append(parentIDs, pid)
		}
	}
	parents := map[int]AssignedWorkItem{}
	if len(parentIDs) > 0 {
		details, err := c.fetchWorkItemDetails(ctx, parentIDs)
		if err != nil {
			return err
		}
		for _, p := range details {
			parents[p.ID] = p
		}
	}

	for i := range items {
		pid := parentOf[items[i].ID]
		p := parents[pid]
		items[i].ParentID = pid
		items[i].ParentTitle = p.Title
		items[i].ParentType = p.Type
	}
	return nil
}

// fetchWorkItemParentIDs reads ids in batches with $expand=relations and
// returns child id -> parent id for every item that has a parent link. Azure
// rejects `fields` combined with `$expand`, so no fields filter is sent.
// errorPolicy=omit keeps one deleted/inaccessible id from failing the batch
// (Azure returns null for it, which is skipped).
func (c *Client) fetchWorkItemParentIDs(ctx context.Context, ids []int) (map[int]int, error) {
	const maxWorkItemsPerRequest = 200

	out := map[int]int{}
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
			"https://dev.azure.com/%s/_apis/wit/workitems?ids=%s&$expand=relations&errorPolicy=omit&api-version=7.1",
			c.cfg.Org, strings.Join(idStrs, ","),
		)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("azure: build work item relations request: %w", err)
		}
		c.setAuthHeader(req)
		req.Header.Set("Accept", "application/json")

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("azure: work item relations http request: %w", err)
		}
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("azure: unexpected work item relations status %d%s", resp.StatusCode, sanitizedResponseMessage(respBody))
		}
		if isHTMLResponse(resp.Header.Get("Content-Type"), respBody) {
			return nil, fmt.Errorf("azure: Azure returned HTML/sign-in response; token may be expired or auth mode invalid")
		}

		var parsed struct {
			Value []*struct {
				ID        int `json:"id"`
				Relations []struct {
					Rel string `json:"rel"`
					URL string `json:"url"`
				} `json:"relations"`
			} `json:"value"`
		}
		if err := json.Unmarshal(respBody, &parsed); err != nil {
			return nil, fmt.Errorf("azure: decode work item relations response: %w", err)
		}
		for _, v := range parsed.Value {
			if v == nil {
				continue
			}
			for _, r := range v.Relations {
				if r.Rel != relHierarchyReverse {
					continue
				}
				if pid, ok := workItemIDFromURL(r.URL); ok {
					out[v.ID] = pid
					break
				}
			}
		}
	}
	return out, nil
}
