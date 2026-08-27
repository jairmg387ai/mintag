package store

import (
	"context"
	"testing"
)

func TestBeginBugCommentUpload_FreshKey_InsertsPendingRow(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	id, alreadyPosted, azureCommentID, err := s.BeginBugCommentUpload(ctx, 4242, "key-1", "root cause identified")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == 0 {
		t.Error("expected non-zero id")
	}
	if alreadyPosted {
		t.Error("expected alreadyPosted=false for a fresh key")
	}
	if azureCommentID != 0 {
		t.Errorf("expected azureCommentID=0 for a fresh key, got %d", azureCommentID)
	}
}

func TestBeginBugCommentUpload_PostedKeyReplay_ShortCircuitsWithoutNewRow(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	id, _, _, err := s.BeginBugCommentUpload(ctx, 4242, "key-posted", "posted comment body")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.MarkBugCommentPosted(ctx, id, 999); err != nil {
		t.Fatal(err)
	}

	replayID, alreadyPosted, azureCommentID, err := s.BeginBugCommentUpload(ctx, 4242, "key-posted", "posted comment body")
	if err != nil {
		t.Fatalf("unexpected error on replay: %v", err)
	}
	if !alreadyPosted {
		t.Error("expected alreadyPosted=true when replaying a posted idempotency key")
	}
	if replayID != id {
		t.Errorf("expected replay to return the same row id %d, got %d", id, replayID)
	}
	if azureCommentID != 999 {
		t.Errorf("expected the stored azure_comment_id 999 to be returned, got %d", azureCommentID)
	}

	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM azure_bug_comment_uploads WHERE idempotency_key = ?`, "key-posted").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 row for the idempotency key (no duplicate insert on replay), got %d", count)
	}
}

func TestBeginBugCommentUpload_PendingKeyReplay_ReusesSameRow(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	id, _, _, err := s.BeginBugCommentUpload(ctx, 4242, "key-pending", "still pending body")
	if err != nil {
		t.Fatal(err)
	}

	replayID, alreadyPosted, _, err := s.BeginBugCommentUpload(ctx, 4242, "key-pending", "still pending body")
	if err != nil {
		t.Fatalf("unexpected error on pending replay: %v", err)
	}
	if alreadyPosted {
		t.Error("expected alreadyPosted=false while the row is still pending")
	}
	if replayID != id {
		t.Errorf("expected the pending row to be reused (same id %d), got %d", id, replayID)
	}
}

func TestMarkBugCommentPosted_SetsStatusPostedAtAndAzureCommentID(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	id, _, _, err := s.BeginBugCommentUpload(ctx, 4242, "key-2", "body")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.MarkBugCommentPosted(ctx, id, 555); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var status string
	var postedAt *string
	var azureCommentID int64
	err = s.db.QueryRow(`SELECT status, posted_at, azure_comment_id FROM azure_bug_comment_uploads WHERE id = ?`, id).
		Scan(&status, &postedAt, &azureCommentID)
	if err != nil {
		t.Fatal(err)
	}
	if status != "posted" {
		t.Errorf("expected status=posted, got %q", status)
	}
	if postedAt == nil || *postedAt == "" {
		t.Error("expected posted_at to be set")
	}
	if azureCommentID != 555 {
		t.Errorf("expected azure_comment_id=555, got %d", azureCommentID)
	}
}

func TestMarkBugCommentFailed_SetsStatusFailedAndLastError(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	id, _, _, err := s.BeginBugCommentUpload(ctx, 4242, "key-3", "body")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.MarkBugCommentFailed(ctx, id, "azure returned 503"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var status string
	var lastError *string
	err = s.db.QueryRow(`SELECT status, last_error FROM azure_bug_comment_uploads WHERE id = ?`, id).Scan(&status, &lastError)
	if err != nil {
		t.Fatal(err)
	}
	if status != "failed" {
		t.Errorf("expected status=failed, got %q", status)
	}
	if lastError == nil || *lastError != "azure returned 503" {
		t.Errorf("expected last_error to be stored, got %v", lastError)
	}

	// The row must still be queryable (not deleted) — a failed upload stays
	// visible for diagnostics/retry, per the design's "row stays queryable as
	// failed, don't delete it" contract.
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM azure_bug_comment_uploads WHERE id = ?`, id).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected the failed row to still exist, got count=%d", count)
	}
}

func TestMarkBugCommentPosted_UnknownID_ReturnsError(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if err := s.MarkBugCommentPosted(context.Background(), 999999, 1); err == nil {
		t.Error("expected error when marking an unknown id posted")
	}
}
