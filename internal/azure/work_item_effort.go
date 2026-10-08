package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// FetchWorkItemEstimates resolves Microsoft.VSTS.Scheduling.OriginalEstimate
// for a set of work item ids, batching at Azure's 200-item limit (same
// request shape as fetchWorkItemDetails). Every returned work item is present
// in the map; a missing estimate field maps to 0. Read-only.
func (c *Client) FetchWorkItemEstimates(ctx context.Context, ids []int) (map[int]float64, error) {
	const maxWorkItemsPerRequest = 200

	out := make(map[int]float64, len(ids))
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
			"https://dev.azure.com/%s/_apis/wit/workitems?ids=%s&fields=System.Id,Microsoft.VSTS.Scheduling.OriginalEstimate&api-version=7.1",
			c.cfg.Org, strings.Join(idStrs, ","),
		)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("azure: build workitem estimates request: %w", err)
		}
		c.setAuthHeader(req)
		req.Header.Set("Accept", "application/json")

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("azure: workitem estimates http request: %w", err)
		}
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("azure: unexpected workitem estimates status %d%s", resp.StatusCode, sanitizedResponseMessage(respBody))
		}
		if isHTMLResponse(resp.Header.Get("Content-Type"), respBody) {
			return nil, fmt.Errorf("azure: Azure returned HTML/sign-in response; token may be expired or auth mode invalid")
		}

		var parsed struct {
			Value []struct {
				ID     int `json:"id"`
				Fields struct {
					OriginalEstimate float64 `json:"Microsoft.VSTS.Scheduling.OriginalEstimate"`
				} `json:"fields"`
			} `json:"value"`
		}
		if err := json.Unmarshal(respBody, &parsed); err != nil {
			return nil, fmt.Errorf("azure: decode workitem estimates response: %w", err)
		}
		for _, v := range parsed.Value {
			out[v.ID] = v.Fields.OriginalEstimate
		}
	}
	return out, nil
}
