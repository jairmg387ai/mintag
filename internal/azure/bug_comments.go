package azure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// BugComment is a single Azure DevOps work item comment, projected down to
// the fields the bug-evidence comment timeline needs.
type BugComment struct {
	ID          int64
	Text        string
	CreatedBy   string
	CreatedDate string
}

// bugCommentWire is the wire shape of one comment as returned by both the
// comments-list endpoint and the create-comment endpoint.
type bugCommentWire struct {
	ID        int64  `json:"id"`
	Text      string `json:"text"`
	CreatedBy struct {
		DisplayName string `json:"displayName"`
	} `json:"createdBy"`
	CreatedDate string `json:"createdDate"`
}

func (w bugCommentWire) toBugComment() BugComment {
	return BugComment{
		ID:          w.ID,
		Text:        w.Text,
		CreatedBy:   w.CreatedBy.DisplayName,
		CreatedDate: w.CreatedDate,
	}
}

// bugCommentsListResponse is the wire shape of the GET .../comments list
// endpoint response.
type bugCommentsListResponse struct {
	Comments []bugCommentWire `json:"comments"`
}

// ListBugComments fetches the full comment timeline for a Bug work item.
// teamProject is an explicit parameter (resolved by the caller from the
// Bug's own System.TeamProject, e.g. via FetchBugEvidence) rather than
// c.cfg.TeamProject: the comments API is project-scoped, and the
// configured default project may not match the Bug's actual project.
func (c *Client) ListBugComments(ctx context.Context, id int, teamProject string) ([]BugComment, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("Azure TimeLog token is not configured")
	}

	url := fmt.Sprintf(
		"https://dev.azure.com/%s/%s/_apis/wit/workitems/%d/comments?api-version=7.1-preview.3",
		c.cfg.Org, teamProject, id,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("azure: build list bug comments request: %w", err)
	}
	c.setAuthHeader(req)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("azure: list bug comments http request: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("azure: unexpected list bug comments status %d%s", resp.StatusCode, sanitizedResponseMessage(respBody))
	}
	if isHTMLResponse(resp.Header.Get("Content-Type"), respBody) {
		return nil, fmt.Errorf("azure: Azure returned HTML/sign-in response; token may be expired or auth mode invalid")
	}

	var parsed bugCommentsListResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("azure: decode list bug comments response: %w", err)
	}

	comments := make([]BugComment, len(parsed.Comments))
	for i, w := range parsed.Comments {
		comments[i] = w.toBugComment()
	}
	return comments, nil
}

// AddBugComment posts a new comment to a Bug work item. Like
// ListBugComments, teamProject is an explicit parameter resolved from the
// Bug's own System.TeamProject, not c.cfg.TeamProject.
func (c *Client) AddBugComment(ctx context.Context, id int, teamProject, text string) (*BugComment, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("Azure TimeLog token is not configured")
	}

	payload, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return nil, fmt.Errorf("azure: marshal add bug comment payload: %w", err)
	}

	url := fmt.Sprintf(
		"https://dev.azure.com/%s/%s/_apis/wit/workitems/%d/comments?api-version=7.1-preview.3",
		c.cfg.Org, teamProject, id,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("azure: build add bug comment request: %w", err)
	}
	c.setAuthHeader(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("azure: add bug comment http request: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	// Errors are run through classifyBugEvidencePatchError (shared with
	// PatchBugEvidence) so a caller can detect an auth/scope rejection via
	// errors.Is(err, ErrInsufficientScope) the same way it does for a field
	// write — a rev conflict never applies to a comment POST, but the
	// underlying 401/403/HTML-sign-in detection is identical.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, classifyBugEvidencePatchError(&patchStatusError{
			StatusCode: resp.StatusCode,
			Body:       respBody,
			err:        fmt.Errorf("azure: unexpected add bug comment status %d%s", resp.StatusCode, sanitizedResponseMessage(respBody)),
		})
	}
	if isHTMLResponse(resp.Header.Get("Content-Type"), respBody) {
		return nil, classifyBugEvidencePatchError(&patchStatusError{
			StatusCode: resp.StatusCode,
			Body:       respBody,
			IsHTML:     true,
			err:        fmt.Errorf("azure: Azure returned HTML/sign-in response; token may be expired or auth mode invalid"),
		})
	}

	var parsed bugCommentWire
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("azure: decode add bug comment response: %w", err)
	}
	comment := parsed.toBugComment()
	return &comment, nil
}
