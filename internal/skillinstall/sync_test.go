package skillinstall

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeSkill creates a skill directory on disk with the given files.
func writeSkill(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// withShippedDigest temporarily marks the on-disk content of dir as a
// version Mintag once shipped for skill, simulating an older bundle.
func withShippedDigest(t *testing.T, skill, dir string) {
	t.Helper()
	digest, err := DigestDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	prev := shippedDigests[skill]
	shippedDigests[skill] = append(append([]string(nil), prev...), digest)
	t.Cleanup(func() { shippedDigests[skill] = prev })
}

func findAction(t *testing.T, actions []SyncAction, skill string, target Target) SyncAction {
	t.Helper()
	for _, a := range actions {
		if a.Skill == skill && a.Target == target {
			return a
		}
	}
	t.Fatalf("no sync action for %s/%s in %+v", skill, target, actions)
	return SyncAction{}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestInstall_RecordsManifestEntry(t *testing.T) {
	homeDir := t.TempDir()
	if _, err := Install([]string{"mintag"}, []Target{TargetClaude}, homeDir, false); err != nil {
		t.Fatal(err)
	}
	m, err := readManifest(homeDir)
	if err != nil {
		t.Fatal(err)
	}
	want, err := bundledDigest("mintag")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Entries) != 1 || m.Entries[0].Skill != "mintag" || m.Entries[0].Target != TargetClaude || m.Entries[0].Digest != want {
		t.Fatalf("unexpected manifest entries: %+v (want digest %s)", m.Entries, want)
	}
}

func TestInstall_RetiredSkillSuggestsReplacement(t *testing.T) {
	_, err := Install([]string{"mintag-graph"}, []Target{TargetClaude}, t.TempDir(), false)
	if err == nil || !strings.Contains(err.Error(), "retired") || !strings.Contains(err.Error(), `"mintag"`) {
		t.Fatalf("expected retired-skill error pointing to mintag, got %v", err)
	}
}

func TestShippedDigests_IncludeCurrentBundle(t *testing.T) {
	for _, skill := range AvailableSkills() {
		digest, err := bundledDigest(skill)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, d := range shippedDigests[skill] {
			found = found || d == digest
		}
		if !found {
			t.Fatalf("current bundle digest %s of %s is missing from shippedDigests; regenerate shipped_digests.go", digest, skill)
		}
	}
}

func TestSync_RemovesRetiredSkillShippedByMintag(t *testing.T) {
	homeDir := t.TempDir()
	dir := filepath.Join(homeDir, ".claude", "skills", "mintag-graph")
	writeSkill(t, dir, map[string]string{"SKILL.md": "old graph skill\r\n"})
	withShippedDigest(t, "mintag-graph", dir)

	actions, err := Sync(nil, homeDir, false)
	if err != nil {
		t.Fatal(err)
	}
	if a := findAction(t, actions, "mintag-graph", TargetClaude); a.Op != SyncRemove {
		t.Fatalf("expected remove, got %+v", a)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("retired skill dir should be removed, stat err=%v", err)
	}
}

func TestSync_RemovesRetiredSkillOwnedByManifest(t *testing.T) {
	homeDir := t.TempDir()
	dir := filepath.Join(homeDir, ".config", "opencode", "skills", "mintag-graph")
	writeSkill(t, dir, map[string]string{"SKILL.md": "graph skill installed by mintag"})
	digest, err := DigestDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeManifest(homeDir, manifest{Schema: manifestSchema, Entries: []manifestEntry{
		{Skill: "mintag-graph", Target: TargetOpenCode, Path: dir, Digest: digest},
	}}); err != nil {
		t.Fatal(err)
	}

	actions, err := Sync([]Target{TargetOpenCode}, homeDir, false)
	if err != nil {
		t.Fatal(err)
	}
	if a := findAction(t, actions, "mintag-graph", TargetOpenCode); a.Op != SyncRemove {
		t.Fatalf("expected remove, got %+v", a)
	}
	m, err := readManifest(homeDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Entries) != 0 {
		t.Fatalf("removed skill should leave the manifest, got %+v", m.Entries)
	}
}

func TestSync_KeepsUserModifiedRetiredSkill(t *testing.T) {
	homeDir := t.TempDir()
	dir := filepath.Join(homeDir, ".claude", "skills", "mintag-graph")
	writeSkill(t, dir, map[string]string{"SKILL.md": "my own edits"})

	actions, err := Sync([]Target{TargetClaude}, homeDir, false)
	if err != nil {
		t.Fatal(err)
	}
	if a := findAction(t, actions, "mintag-graph", TargetClaude); a.Op != SyncSkipModified {
		t.Fatalf("expected skip_modified, got %+v", a)
	}
	if got := readFile(t, filepath.Join(dir, "SKILL.md")); got != "my own edits" {
		t.Fatalf("user-modified skill was changed: %q", got)
	}
}

func TestSync_UpdatesStaleSkillShippedByMintag(t *testing.T) {
	homeDir := t.TempDir()
	dir := filepath.Join(homeDir, ".claude", "skills", "activity-autolog")
	writeSkill(t, dir, map[string]string{"SKILL.md": "older activity-autolog version"})
	withShippedDigest(t, "activity-autolog", dir)

	actions, err := Sync([]Target{TargetClaude}, homeDir, false)
	if err != nil {
		t.Fatal(err)
	}
	if a := findAction(t, actions, "activity-autolog", TargetClaude); a.Op != SyncUpdate {
		t.Fatalf("expected update, got %+v", a)
	}
	got, err := DigestDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	want, err := bundledDigest("activity-autolog")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("updated skill digest %s, want bundle %s", got, want)
	}
	if _, err := os.Stat(dir + stagingSuffix); !os.IsNotExist(err) {
		t.Fatalf("staging dir left behind: %v", err)
	}
	if _, err := os.Stat(dir + backupSuffix); !os.IsNotExist(err) {
		t.Fatalf("backup dir left behind: %v", err)
	}
	m, err := readManifest(homeDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Entries) != 1 || m.Entries[0].Digest != want {
		t.Fatalf("manifest should record the updated digest, got %+v", m.Entries)
	}
}

func TestSync_SkipsUserModifiedBundledSkill(t *testing.T) {
	homeDir := t.TempDir()
	if _, err := Install([]string{"mintag"}, []Target{TargetClaude}, homeDir, false); err != nil {
		t.Fatal(err)
	}
	skillFile := filepath.Join(homeDir, ".claude", "skills", "mintag", "SKILL.md")
	if err := os.WriteFile(skillFile, []byte("customized"), 0o644); err != nil {
		t.Fatal(err)
	}

	actions, err := Sync([]Target{TargetClaude}, homeDir, false)
	if err != nil {
		t.Fatal(err)
	}
	if a := findAction(t, actions, "mintag", TargetClaude); a.Op != SyncSkipModified {
		t.Fatalf("expected skip_modified, got %+v", a)
	}
	if got := readFile(t, skillFile); got != "customized" {
		t.Fatalf("user-modified skill was overwritten: %q", got)
	}
}

func TestSync_UnchangedSkillIsReportedAndAdopted(t *testing.T) {
	homeDir := t.TempDir()
	if _, err := Install([]string{"mintag"}, []Target{TargetClaude}, homeDir, false); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(manifestPath(homeDir)); err != nil {
		t.Fatal(err)
	}

	actions, err := Sync([]Target{TargetClaude}, homeDir, false)
	if err != nil {
		t.Fatal(err)
	}
	if a := findAction(t, actions, "mintag", TargetClaude); a.Op != SyncUnchanged {
		t.Fatalf("expected unchanged, got %+v", a)
	}
	m, err := readManifest(homeDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Entries) != 1 || m.Entries[0].Skill != "mintag" {
		t.Fatalf("an identical copy should be adopted into the manifest, got %+v", m.Entries)
	}
}

func TestSync_DryRunWritesNothing(t *testing.T) {
	homeDir := t.TempDir()
	retired := filepath.Join(homeDir, ".claude", "skills", "mintag-graph")
	writeSkill(t, retired, map[string]string{"SKILL.md": "old graph skill"})
	withShippedDigest(t, "mintag-graph", retired)
	stale := filepath.Join(homeDir, ".claude", "skills", "activity-autolog")
	writeSkill(t, stale, map[string]string{"SKILL.md": "older activity-autolog"})
	withShippedDigest(t, "activity-autolog", stale)

	actions, err := Sync([]Target{TargetClaude}, homeDir, true)
	if err != nil {
		t.Fatal(err)
	}
	if a := findAction(t, actions, "mintag-graph", TargetClaude); a.Op != SyncRemove {
		t.Fatalf("dry-run should plan remove, got %+v", a)
	}
	if a := findAction(t, actions, "activity-autolog", TargetClaude); a.Op != SyncUpdate {
		t.Fatalf("dry-run should plan update, got %+v", a)
	}
	if got := readFile(t, filepath.Join(retired, "SKILL.md")); got != "old graph skill" {
		t.Fatalf("dry-run changed retired skill: %q", got)
	}
	if got := readFile(t, filepath.Join(stale, "SKILL.md")); got != "older activity-autolog" {
		t.Fatalf("dry-run changed stale skill: %q", got)
	}
	if _, err := os.Stat(manifestPath(homeDir)); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote the manifest: %v", err)
	}
}

func TestSync_LeavesUnrelatedSkillsAndMissingTargetsAlone(t *testing.T) {
	homeDir := t.TempDir()
	other := filepath.Join(homeDir, ".claude", "skills", "someone-elses-skill")
	writeSkill(t, other, map[string]string{"SKILL.md": "not mintag"})

	actions, err := Sync(nil, homeDir, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range actions {
		if a.Skill == "someone-elses-skill" {
			t.Fatalf("sync must ignore skills it never shipped, got %+v", a)
		}
	}
	if got := readFile(t, filepath.Join(other, "SKILL.md")); got != "not mintag" {
		t.Fatalf("unrelated skill changed: %q", got)
	}
	if _, err := os.Stat(filepath.Join(homeDir, ".gentle-shell")); !os.IsNotExist(err) {
		t.Fatalf("sync must not create missing agent dirs: %v", err)
	}
}

func TestSync_SkipsSymlinkedSkillDir(t *testing.T) {
	homeDir := t.TempDir()
	real := filepath.Join(t.TempDir(), "elsewhere")
	writeSkill(t, real, map[string]string{"SKILL.md": "old graph skill"})
	withShippedDigest(t, "mintag-graph", real)
	root := filepath.Join(homeDir, ".claude", "skills")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "mintag-graph")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	actions, err := Sync([]Target{TargetClaude}, homeDir, false)
	if err != nil {
		t.Fatal(err)
	}
	if a := findAction(t, actions, "mintag-graph", TargetClaude); a.Op != SyncSkipUnowned {
		t.Fatalf("expected skip_unowned for symlink, got %+v", a)
	}
	if got := readFile(t, filepath.Join(real, "SKILL.md")); got != "old graph skill" {
		t.Fatalf("symlink target changed: %q", got)
	}
}
