package skillinstall

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseTargets_DefaultsToAll(t *testing.T) {
	targets, err := ParseTargets("")
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 3 {
		t.Fatalf("expected 3 default targets, got %d", len(targets))
	}
}

func TestParseTargets_RejectsUnknownTarget(t *testing.T) {
	_, err := ParseTargets("claude,wat")
	if err == nil {
		t.Fatal("expected error for unknown target")
	}
}

func TestInstall_CopiesBundledSkill(t *testing.T) {
	homeDir := t.TempDir()
	results, err := Install([]string{"vtt-task-extractor"}, []Target{TargetClaude}, homeDir, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 install result, got %d", len(results))
	}
	if results[0].Files != 2 {
		t.Fatalf("expected 2 bundled files, got %d", results[0].Files)
	}

	skillDir := filepath.Join(homeDir, ".claude", "skills", "vtt-task-extractor")
	skillData, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(skillData), "template_informe_reunion.html") {
		t.Fatalf("installed skill manifest did not include expected template reference")
	}

	templateData, err := os.ReadFile(filepath.Join(skillDir, "template_informe_reunion.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(templateData), "{{FILAS_TAREAS}}") {
		t.Fatalf("installed template did not contain expected placeholder")
	}
}

func TestInstall_UnknownSkillNameWritesNothing(t *testing.T) {
	homeDir := t.TempDir()
	_, err := Install([]string{"vtt-task-extractor", "--targets"}, []Target{TargetClaude}, homeDir, false)
	if err == nil {
		t.Fatal("expected error for unknown skill name")
	}

	skillDir := filepath.Join(homeDir, ".claude", "skills", "vtt-task-extractor")
	if _, statErr := os.Stat(skillDir); !os.IsNotExist(statErr) {
		t.Fatalf("expected no files written for the valid skill when a later name is invalid, but found %s", skillDir)
	}
}

func TestInstall_RequiresForceWhenDestinationExists(t *testing.T) {
	homeDir := t.TempDir()
	_, err := Install([]string{"vtt-task-extractor"}, []Target{TargetGemini}, homeDir, false)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Install([]string{"vtt-task-extractor"}, []Target{TargetGemini}, homeDir, false)
	if err == nil {
		t.Fatal("expected overwrite error without force")
	}
	if !strings.Contains(err.Error(), "use --force") {
		t.Fatalf("expected overwrite hint, got %v", err)
	}

	_, err = Install([]string{"vtt-task-extractor"}, []Target{TargetGemini}, homeDir, true)
	if err != nil {
		t.Fatal(err)
	}
}
