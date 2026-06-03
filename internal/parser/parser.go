package parser

import (
	"bufio"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"
)

type ParsedMeeting struct {
	Title   string
	Date    string
	Content string // clean text, speakers stripped of timestamps
}

var (
	vttTimestamp = regexp.MustCompile(`^\d{2}:\d{2}:\d{2}[.,]\d{3}\s+-->\s+\d{2}:\d{2}:\d{2}[.,]\d{3}`)
	speakerLine  = regexp.MustCompile(`^([^:]+):\s+(.+)$`)
	datePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(\d{4})(\d{2})(\d{2})`),                       // 20260529
		regexp.MustCompile(`(\d{2})(\d{2})(\d{4})`),                       // 29052026
		regexp.MustCompile(`(\d{4})[.\-/](\d{2})[.\-/](\d{2})`),          // 2026-05-29
	}
)

// Parse handles .vtt and .txt files, returning clean content and metadata.
func Parse(filename, content string) *ParsedMeeting {
	ext := strings.ToLower(filepath.Ext(filename))
	base := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))

	pm := &ParsedMeeting{
		Title: cleanTitle(base),
		Date:  extractDate(base),
	}

	switch ext {
	case ".vtt":
		pm.Content = parseVTT(content)
	default:
		pm.Content = cleanPlainText(content)
	}

	return pm
}

// parseVTT strips WebVTT headers and timestamps, collapses speaker lines.
func parseVTT(raw string) string {
	scanner := bufio.NewScanner(strings.NewReader(raw))
	var sb strings.Builder
	skip := true // skip WEBVTT header block

	var lastSpeaker, lastLine string

	flush := func() {
		if lastLine == "" {
			return
		}
		if lastSpeaker != "" {
			sb.WriteString(lastSpeaker)
			sb.WriteString(": ")
		}
		sb.WriteString(lastLine)
		sb.WriteString("\n")
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if skip {
			if line == "" {
				skip = false
			}
			continue
		}

		// skip cue identifiers (pure numbers or UUIDs)
		if isNumeric(line) || isUUID(line) {
			continue
		}
		// skip timestamp lines
		if vttTimestamp.MatchString(line) {
			continue
		}
		// skip empty lines between cues — but flush pending
		if line == "" {
			continue
		}
		// remove VTT tags like <v Speaker> or <00:01:02.000>
		line = stripVTTTags(line)
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// detect speaker prefix "Name: text"
		if m := speakerLine.FindStringSubmatch(line); m != nil {
			speaker := strings.TrimSpace(m[1])
			text := strings.TrimSpace(m[2])
			if isSpeakerName(speaker) {
				if speaker == lastSpeaker {
					// continuation — append
					lastLine += " " + text
				} else {
					flush()
					lastSpeaker = speaker
					lastLine = text
				}
				continue
			}
		}

		// plain cue text, no speaker
		if lastSpeaker == "" {
			lastLine += " " + line
		} else {
			// continuation of same speaker block
			lastLine += " " + line
		}
	}
	flush()
	return strings.TrimSpace(sb.String())
}

func cleanPlainText(s string) string {
	lines := strings.Split(s, "\n")
	var out []string
	for _, l := range lines {
		l = strings.TrimRightFunc(l, unicode.IsSpace)
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

// extractDate tries to find a date in the filename.
func extractDate(name string) string {
	for _, re := range datePatterns {
		m := re.FindStringSubmatch(name)
		if m == nil {
			continue
		}
		var y, mo, d string
		if len(m[1]) == 4 {
			y, mo, d = m[1], m[2], m[3]
		} else {
			d, mo, y = m[1], m[2], m[3]
		}
		t, err := time.Parse("2006-01-02", y+"-"+mo+"-"+d)
		if err == nil {
			return t.Format("2006-01-02")
		}
	}
	return time.Now().Format("2006-01-02")
}

// cleanTitle turns a filename into a human-readable title.
func cleanTitle(name string) string {
	// replace underscores and hyphens with spaces
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, "-", " ")
	// remove date patterns
	for _, re := range datePatterns {
		name = re.ReplaceAllString(name, "")
	}
	// collapse whitespace
	fields := strings.Fields(name)
	return strings.Join(fields, " ")
}

func stripVTTTags(s string) string {
	// remove <v Name>, </v>, <c>, <00:00:00.000>
	re := regexp.MustCompile(`<[^>]+>`)
	return re.ReplaceAllString(s, "")
}

func isNumeric(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return len(s) > 0
}

func isUUID(s string) bool {
	return regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-`).MatchString(strings.ToLower(s))
}

func isSpeakerName(s string) bool {
	// heuristic: speaker names are typically short (< 50 chars) and don't end with punctuation
	if len(s) > 50 {
		return false
	}
	if strings.ContainsAny(s, ".!?;") {
		return false
	}
	return true
}
