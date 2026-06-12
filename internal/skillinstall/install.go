package skillinstall

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed bundled/skills/* bundled/skills/*/*
var bundled embed.FS

type Target string

const (
	TargetClaude   Target = "claude"
	TargetGemini   Target = "gemini"
	TargetOpenCode Target = "opencode"
)

var allTargets = []Target{TargetClaude, TargetGemini, TargetOpenCode}

var skillDirs = map[string]string{
	"mintag-graph":       "bundled/skills/mintag-graph",
	"vtt-task-extractor": "bundled/skills/vtt-task-extractor",
}

type Result struct {
	Skill       string
	Target      Target
	Destination string
	Files       int
}

func AvailableSkills() []string {
	list := make([]string, 0, len(skillDirs))
	for name := range skillDirs {
		list = append(list, name)
	}
	sort.Strings(list)
	return list
}

func Install(skillNames []string, targets []Target, homeDir string, force bool) ([]Result, error) {
	if len(skillNames) == 0 {
		return nil, errors.New("at least one skill name is required")
	}
	if homeDir == "" {
		return nil, errors.New("home directory is required")
	}
	if len(targets) == 0 {
		targets = append([]Target(nil), allTargets...)
	}

	results := make([]Result, 0, len(skillNames)*len(targets))
	for _, skillName := range skillNames {
		sourceDir, ok := skillDirs[skillName]
		if !ok {
			return nil, fmt.Errorf("unknown skill %q", skillName)
		}

		for _, target := range targets {
			destRoot, err := targetDir(homeDir, target)
			if err != nil {
				return nil, err
			}
			destDir := filepath.Join(destRoot, skillName)
			files, err := copySkillDir(sourceDir, destDir, force)
			if err != nil {
				return nil, err
			}
			results = append(results, Result{
				Skill:       skillName,
				Target:      target,
				Destination: destDir,
				Files:       files,
			})
		}
	}

	return results, nil
}

func ParseTargets(raw string) ([]Target, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return append([]Target(nil), allTargets...), nil
	}

	seen := map[Target]bool{}
	parts := strings.Split(raw, ",")
	targets := make([]Target, 0, len(parts))
	for _, part := range parts {
		target := Target(strings.ToLower(strings.TrimSpace(part)))
		switch target {
		case TargetClaude, TargetGemini, TargetOpenCode:
			if !seen[target] {
				seen[target] = true
				targets = append(targets, target)
			}
		default:
			return nil, fmt.Errorf("unknown target %q", part)
		}
	}

	if len(targets) == 0 {
		return nil, errors.New("no valid targets provided")
	}

	return targets, nil
}

func targetDir(homeDir string, target Target) (string, error) {
	switch target {
	case TargetClaude:
		return filepath.Join(homeDir, ".claude", "skills"), nil
	case TargetGemini:
		return filepath.Join(homeDir, ".gemini", "skills"), nil
	case TargetOpenCode:
		return filepath.Join(homeDir, ".config", "opencode", "skills"), nil
	default:
		return "", fmt.Errorf("unsupported target %q", target)
	}
}

func copySkillDir(sourceDir, destDir string, force bool) (int, error) {
	if _, err := fs.Stat(bundled, sourceDir); err != nil {
		return 0, fmt.Errorf("skill bundle %q is unavailable: %w", sourceDir, err)
	}

	if _, err := os.Stat(destDir); err == nil {
		if !force {
			return 0, fmt.Errorf("destination already exists: %s (use --force to overwrite)", destDir)
		}
		if err := os.RemoveAll(destDir); err != nil {
			return 0, fmt.Errorf("remove existing destination: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return 0, fmt.Errorf("inspect destination: %w", err)
	}

	count := 0
	err := fs.WalkDir(bundled, sourceDir, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel := strings.TrimPrefix(current, sourceDir)
		rel = strings.TrimPrefix(rel, "/")
		targetPath := destDir
		if rel != "" {
			targetPath = filepath.Join(destDir, filepath.FromSlash(rel))
		}

		if entry.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}

		data, err := bundled.ReadFile(current)
		if err != nil {
			return fmt.Errorf("read bundled file %s: %w", current, err)
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(targetPath, data, 0o644); err != nil {
			return fmt.Errorf("write file %s: %w", targetPath, err)
		}
		count++
		return nil
	})
	if err != nil {
		return 0, err
	}

	return count, nil
}

func FormatResults(results []Result) string {
	var b strings.Builder
	for _, result := range results {
		fmt.Fprintf(&b, "Installed %s for %s -> %s (%d files)\n", result.Skill, result.Target, result.Destination, result.Files)
	}
	return strings.TrimRight(b.String(), "\n")
}

func SkillFilePaths(skillName string) ([]string, error) {
	sourceDir, ok := skillDirs[skillName]
	if !ok {
		return nil, fmt.Errorf("unknown skill %q", skillName)
	}

	paths := []string{}
	err := fs.WalkDir(bundled, sourceDir, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		paths = append(paths, path.Base(current))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}
