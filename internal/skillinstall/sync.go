package skillinstall

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Sync keeps installed Mintag skills in step with the bundle, in the spirit
// of gentle-ai's managed-asset sync: a skill directory is only rewritten or
// deleted when Mintag can prove it owns it, either because the manifest
// recorded the exact content Mintag installed or because the content matches
// a version Mintag once shipped (shippedDigests). Anything else, including a
// copy the user edited, is reported and left alone.

const (
	manifestSchema = "mintag.skills-manifest/v1"
	stagingSuffix  = ".mintag-sync-new"
	backupSuffix   = ".mintag-sync-old"
)

// retiredSkills maps skills Mintag no longer ships to their replacement.
// Sync deletes owned copies of them; Install rejects them with a hint.
var retiredSkills = map[string]string{
	"mintag-graph": "mintag",
}

type SyncOp string

const (
	SyncUpdate       SyncOp = "update"
	SyncRemove       SyncOp = "remove"
	SyncUnchanged    SyncOp = "unchanged"
	SyncSkipModified SyncOp = "skip_modified"
	SyncSkipUnowned  SyncOp = "skip_unowned"
)

type SyncAction struct {
	Skill  string
	Target Target
	Path   string
	Op     SyncOp
	Reason string
}

type manifestEntry struct {
	Skill  string `json:"skill"`
	Target Target `json:"target"`
	Path   string `json:"path"`
	Digest string `json:"digest"`
}

type manifest struct {
	Schema  string          `json:"schema"`
	Entries []manifestEntry `json:"entries"`
}

func manifestPath(homeDir string) string {
	return filepath.Join(homeDir, ".mintag", "skills-manifest.json")
}

func readManifest(homeDir string) (manifest, error) {
	data, err := os.ReadFile(manifestPath(homeDir))
	if errors.Is(err, os.ErrNotExist) {
		return manifest{Schema: manifestSchema}, nil
	}
	if err != nil {
		return manifest{}, fmt.Errorf("read skills manifest: %w", err)
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return manifest{}, fmt.Errorf("parse skills manifest %s: %w", manifestPath(homeDir), err)
	}
	if m.Schema != manifestSchema {
		return manifest{}, fmt.Errorf("unsupported skills manifest schema %q in %s", m.Schema, manifestPath(homeDir))
	}
	return m, nil
}

// writeManifest replaces the manifest atomically (temp file + rename).
func writeManifest(homeDir string, m manifest) error {
	m.Schema = manifestSchema
	if m.Entries == nil {
		m.Entries = []manifestEntry{}
	}
	sort.Slice(m.Entries, func(i, j int) bool {
		if m.Entries[i].Target != m.Entries[j].Target {
			return m.Entries[i].Target < m.Entries[j].Target
		}
		return m.Entries[i].Skill < m.Entries[j].Skill
	})
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	path := manifestPath(homeDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create manifest dir: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write skills manifest: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace skills manifest: %w", err)
	}
	return nil
}

func (m *manifest) find(skill string, target Target) (manifestEntry, bool) {
	for _, e := range m.Entries {
		if e.Skill == skill && e.Target == target {
			return e, true
		}
	}
	return manifestEntry{}, false
}

func (m *manifest) set(entry manifestEntry) {
	m.drop(entry.Skill, entry.Target)
	m.Entries = append(m.Entries, entry)
}

func (m *manifest) drop(skill string, target Target) {
	kept := m.Entries[:0]
	for _, e := range m.Entries {
		if e.Skill != skill || e.Target != target {
			kept = append(kept, e)
		}
	}
	m.Entries = kept
}

// recordInstalls adds freshly installed skills to the manifest.
func recordInstalls(homeDir string, results []Result) error {
	m, err := readManifest(homeDir)
	if err != nil {
		return err
	}
	for _, r := range results {
		digest, err := bundledDigest(r.Skill)
		if err != nil {
			return err
		}
		m.set(manifestEntry{Skill: r.Skill, Target: r.Target, Path: r.Destination, Digest: digest})
	}
	return writeManifest(homeDir, m)
}

// digestFS hashes every regular file under root as (slash path, sha256 of
// content with CRLF normalized to LF), so the same skill hashes alike whether
// it was built on Windows or Linux. Symlinks and other special files make the
// tree unhashable, which callers treat as "not provably Mintag's".
func digestFS(fsys fs.FS, root string) (string, error) {
	h := sha256.New()
	err := fs.WalkDir(fsys, root, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("%s is not a regular file", current)
		}
		data, err := fs.ReadFile(fsys, current)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n")))
		rel := current
		if root != "." {
			rel = strings.TrimPrefix(strings.TrimPrefix(current, root), "/")
		}
		h.Write([]byte(rel))
		h.Write([]byte{0})
		h.Write(sum[:])
		return nil
	})
	if err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// DigestDir returns the content digest of an installed skill directory.
func DigestDir(dir string) (string, error) {
	return digestFS(os.DirFS(dir), ".")
}

func bundledDigest(skill string) (string, error) {
	sourceDir, ok := skillDirs[skill]
	if !ok {
		return "", fmt.Errorf("unknown skill %q", skill)
	}
	return digestFS(bundled, sourceDir)
}

func isShipped(skill, digest string) bool {
	for _, d := range shippedDigests[skill] {
		if d == digest {
			return true
		}
	}
	return false
}

// Sync updates owned copies of bundled skills and removes owned copies of
// retired ones across targets (all known targets when empty). It never
// installs a skill that is not already present and never creates agent dirs.
// With dryRun it only reports the plan.
func Sync(targets []Target, homeDir string, dryRun bool) ([]SyncAction, error) {
	if homeDir == "" {
		return nil, errors.New("home directory is required")
	}
	if len(targets) == 0 {
		targets = append([]Target(nil), knownTargets...)
	}
	m, err := readManifest(homeDir)
	if err != nil {
		return nil, err
	}

	names := AvailableSkills()
	for name := range retiredSkills {
		names = append(names, name)
	}
	sort.Strings(names)

	// Plan every action before touching the filesystem.
	var actions []SyncAction
	for _, target := range targets {
		root, err := targetDir(homeDir, target)
		if err != nil {
			return nil, err
		}
		for _, name := range names {
			action, ok := planSkill(&m, name, target, filepath.Join(root, name))
			if ok {
				actions = append(actions, action)
			}
		}
	}
	if dryRun {
		return actions, nil
	}

	for i, a := range actions {
		var err error
		switch a.Op {
		case SyncRemove:
			err = removeOwned(a.Path)
			if err == nil {
				m.drop(a.Skill, a.Target)
			}
		case SyncUpdate:
			err = replaceOwned(a.Skill, a.Path)
			if err == nil {
				digest, digestErr := bundledDigest(a.Skill)
				err = digestErr
				m.set(manifestEntry{Skill: a.Skill, Target: a.Target, Path: a.Path, Digest: digest})
			}
		case SyncUnchanged:
			digest, digestErr := bundledDigest(a.Skill)
			err = digestErr
			m.set(manifestEntry{Skill: a.Skill, Target: a.Target, Path: a.Path, Digest: digest})
		}
		if err != nil {
			// Keep the manifest truthful about what was already applied.
			if writeErr := writeManifest(homeDir, m); writeErr != nil {
				err = errors.Join(err, writeErr)
			}
			return actions[:i], fmt.Errorf("sync %s for %s: %w", a.Skill, a.Target, err)
		}
	}
	return actions, writeManifest(homeDir, m)
}

// planSkill decides what Sync does with one skill directory. It returns false
// when there is nothing at path (stale manifest rows are dropped).
func planSkill(m *manifest, name string, target Target, path string) (SyncAction, bool) {
	action := SyncAction{Skill: name, Target: target, Path: path}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		m.drop(name, target)
		return action, false
	}
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		action.Op, action.Reason = SyncSkipUnowned, "not a plain directory"
		return action, true
	}
	digest, err := DigestDir(path)
	if err != nil {
		action.Op, action.Reason = SyncSkipUnowned, "contains links or unreadable files"
		return action, true
	}

	entry, tracked := m.find(name, target)
	owned := (tracked && entry.Digest == digest) || isShipped(name, digest)
	replacement, retired := retiredSkills[name]

	switch {
	case retired && owned:
		action.Op, action.Reason = SyncRemove, fmt.Sprintf("retired; use %q", replacement)
	case retired:
		action.Op, action.Reason = SyncSkipModified, fmt.Sprintf("retired but edited locally; remove it by hand and use %q", replacement)
	default:
		current, err := bundledDigest(name)
		switch {
		case err != nil:
			action.Op, action.Reason = SyncSkipUnowned, err.Error()
		case digest == current:
			action.Op = SyncUnchanged
		case owned:
			action.Op, action.Reason = SyncUpdate, "older Mintag version"
		default:
			action.Op, action.Reason = SyncSkipModified, "edited locally; reinstall with --force to replace"
		}
	}
	return action, true
}

// removeOwned deletes a skill directory, re-checking it is still a plain
// directory right before deleting.
func removeOwned(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s is no longer a plain directory", path)
	}
	return os.RemoveAll(path)
}

// replaceOwned swaps in the bundled skill: copy to a staging dir, move the
// old copy aside, move staging into place, then drop the old copy. On a
// failed swap the old copy is put back.
func replaceOwned(skill, path string) error {
	staging, backup := path+stagingSuffix, path+backupSuffix
	for _, p := range []string{staging, backup} {
		if _, err := os.Lstat(p); err == nil {
			return fmt.Errorf("leftover %s from an interrupted sync; remove it and retry", p)
		}
	}
	if _, err := copySkillDir(skillDirs[skill], staging, false); err != nil {
		_ = os.RemoveAll(staging)
		return err
	}
	if err := os.Rename(path, backup); err != nil {
		_ = os.RemoveAll(staging)
		return err
	}
	if err := os.Rename(staging, path); err != nil {
		restoreErr := os.Rename(backup, path)
		_ = os.RemoveAll(staging)
		return errors.Join(err, restoreErr)
	}
	return os.RemoveAll(backup)
}

// FormatSyncActions renders sync actions for the terminal.
func FormatSyncActions(actions []SyncAction, dryRun bool) string {
	if len(actions) == 0 {
		return "No Mintag skills installed in the selected targets."
	}
	var b strings.Builder
	if dryRun {
		b.WriteString("Dry run — nothing was changed.\n")
	}
	for _, a := range actions {
		fmt.Fprintf(&b, "%-13s %s for %s -> %s", a.Op, a.Skill, a.Target, a.Path)
		if a.Reason != "" {
			fmt.Fprintf(&b, " (%s)", a.Reason)
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}
