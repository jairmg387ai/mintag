package parser

import (
	"regexp"
	"strings"
	"testing"
)

func mustNoErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// TestParse_VTT_BasicParsing verifies that WEBVTT headers, timestamp cues,
// and VTT tags are stripped from the output.
func TestParse_VTT_BasicParsing(t *testing.T) {
	vttContent := `WEBVTT

1
00:00:01.000 --> 00:00:03.000
Alice: Hello everyone.

2
00:00:04.000 --> 00:00:06.000
Bob: Good morning.
`
	pm := Parse("2026-01-15_meeting.vtt", vttContent)

	if pm.Title == "" {
		t.Error("expected non-empty Title")
	}
	if strings.Contains(pm.Content, "WEBVTT") {
		t.Errorf("Content must not contain 'WEBVTT', got: %q", pm.Content)
	}
	if strings.Contains(pm.Content, "-->") {
		t.Errorf("Content must not contain '-->', got: %q", pm.Content)
	}
	if !strings.Contains(pm.Content, "Hello everyone") {
		t.Errorf("Content should contain spoken text, got: %q", pm.Content)
	}
}

// TestParse_TXT_PassThrough verifies that plain text files are returned
// with only trailing whitespace trimmed per line.
func TestParse_TXT_PassThrough(t *testing.T) {
	raw := "Line one\nLine two\nLine three"
	pm := Parse("notes.txt", raw)

	// cleanPlainText trims trailing whitespace per line and rejoins with \n
	want := "Line one\nLine two\nLine three"
	if pm.Content != want {
		t.Errorf("expected content %q, got %q", want, pm.Content)
	}
}

// TestParse_Date_ExtractedFromFilename checks that known date patterns are
// extracted correctly and that unknown filenames fall back to today in
// YYYY-MM-DD format.
func TestParse_Date_ExtractedFromFilename(t *testing.T) {
	dateRe := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

	cases := []struct {
		filename    string
		wantDate    string // exact date expected, empty means "any YYYY-MM-DD"
	}{
		{"2026-01-15_meeting.vtt", "2026-01-15"},
		{"meeting_2026-05-30.txt", "2026-05-30"},
		{"20260115_standup.vtt", "2026-01-15"},
		{"no-date.vtt", ""}, // falls back to today — only check format
	}

	for _, tc := range cases {
		t.Run(tc.filename, func(t *testing.T) {
			pm := Parse(tc.filename, "")
			if !dateRe.MatchString(pm.Date) {
				t.Errorf("filename %q: Date %q does not match YYYY-MM-DD", tc.filename, pm.Date)
			}
			if tc.wantDate != "" && pm.Date != tc.wantDate {
				t.Errorf("filename %q: expected date %q, got %q", tc.filename, tc.wantDate, pm.Date)
			}
		})
	}
}

// TestParse_VTT_CollapsesConsecutiveSpeaker verifies that two consecutive
// cues from the same speaker are collapsed into a single paragraph.
func TestParse_VTT_CollapsesConsecutiveSpeaker(t *testing.T) {
	vttContent := `WEBVTT

1
00:00:01.000 --> 00:00:03.000
Alice: First part of the sentence.

2
00:00:03.500 --> 00:00:05.000
Alice: Second part continues here.

3
00:00:06.000 --> 00:00:08.000
Bob: Different speaker now.
`
	pm := Parse("meeting.vtt", vttContent)

	// Alice's two lines should be collapsed into one paragraph
	aliceCount := strings.Count(pm.Content, "Alice:")
	if aliceCount != 1 {
		t.Errorf("expected Alice's lines collapsed into 1 paragraph, found %d 'Alice:' occurrences in: %q", aliceCount, pm.Content)
	}

	// Bob should appear separately
	if !strings.Contains(pm.Content, "Bob:") {
		t.Errorf("expected Bob's line in content, got: %q", pm.Content)
	}

	// Both pieces of Alice's text should be present
	if !strings.Contains(pm.Content, "First part") {
		t.Errorf("expected 'First part' in content, got: %q", pm.Content)
	}
	if !strings.Contains(pm.Content, "Second part") {
		t.Errorf("expected 'Second part' in content, got: %q", pm.Content)
	}
}
