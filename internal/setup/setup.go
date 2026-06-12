package setup

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Run configures the Mintag MCP server for the given agent.
// agent must be "claude", "opencode", or "gemini".
// homeDir is the user's home directory.
// execPath is the absolute path to the mintag binary.
func Run(agent, homeDir, execPath string) error {
	switch agent {
	case "claude":
		return setupClaude(homeDir, execPath)
	case "opencode":
		return setupOpenCode(homeDir, execPath)
	case "gemini":
		return setupGemini(homeDir, execPath)
	default:
		return fmt.Errorf("unknown agent %q — supported: claude, opencode, gemini", agent)
	}
}

// setupClaude writes ~/.claude/mcp/mintag.json and adds mcp__mintag__* permissions
// to ~/.claude/settings.json.
func setupClaude(homeDir, execPath string) error {
	mcpDir := filepath.Join(homeDir, ".claude", "mcp")
	if err := os.MkdirAll(mcpDir, 0o755); err != nil {
		return fmt.Errorf("create ~/.claude/mcp: %w", err)
	}

	mcpEntry := map[string]any{
		"mintag": map[string]any{
			"command": execPath,
			"args":    []string{"mcp"},
		},
	}
	if err := writeJSON(filepath.Join(mcpDir, "mintag.json"), mcpEntry); err != nil {
		return fmt.Errorf("write mintag.json: %w", err)
	}
	fmt.Printf("  wrote %s\n", filepath.Join(mcpDir, "mintag.json"))

	settingsPath := filepath.Join(homeDir, ".claude", "settings.json")
	if err := addClaudePermissions(settingsPath); err != nil {
		return fmt.Errorf("patch settings.json: %w", err)
	}
	fmt.Printf("  patched %s (mcp__mintag__* permissions)\n", settingsPath)
	return nil
}

var mintTagToolNames = []string{
	"project_create", "project_list",
	"meeting_import", "meeting_search", "meeting_find_or_create", "meeting_set_rich_content",
	"task_create", "task_update", "task_search", "task_history", "task_upsert", "tasks_by_project",
	"graph_search", "graph_node", "graph_neighbors", "graph_impact",
	"graph_upsert_node", "graph_upsert_edge", "graph_stats",
}

func addClaudePermissions(settingsPath string) error {
	data, err := os.ReadFile(settingsPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	var settings map[string]any
	if len(data) > 0 {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("parse settings.json: %w", err)
		}
	} else {
		settings = map[string]any{}
	}

	perms, _ := settings["permissions"].(map[string]any)
	if perms == nil {
		perms = map[string]any{}
		settings["permissions"] = perms
	}

	allow := toStringSlice(perms["allow"])
	existing := map[string]bool{}
	for _, v := range allow {
		existing[v] = true
	}

	for _, tool := range mintTagToolNames {
		entry := "mcp__mintag__" + tool
		if !existing[entry] {
			allow = append(allow, entry)
		}
	}
	perms["allow"] = allow

	return writeJSON(settingsPath, settings)
}

// setupOpenCode injects the mintag MCP entry into ~/.config/opencode/opencode.json.
func setupOpenCode(homeDir, execPath string) error {
	dir := filepath.Join(homeDir, ".config", "opencode")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create opencode config dir: %w", err)
	}

	cfgPath := filepath.Join(dir, "opencode.json")
	var cfg map[string]any
	if data, err := os.ReadFile(cfgPath); err == nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("parse opencode.json: %w", err)
		}
	} else {
		cfg = map[string]any{}
	}

	mcpBlock, _ := cfg["mcp"].(map[string]any)
	if mcpBlock == nil {
		mcpBlock = map[string]any{}
		cfg["mcp"] = mcpBlock
	}

	mcpBlock["mintag"] = map[string]any{
		"type":    "local",
		"command": []string{execPath, "mcp"},
		"enabled": true,
	}

	if err := writeJSON(cfgPath, cfg); err != nil {
		return fmt.Errorf("write opencode.json: %w", err)
	}
	fmt.Printf("  patched %s\n", cfgPath)
	return nil
}

// setupGemini injects the mintag MCP entry into ~/.gemini/settings.json and writes
// a graph protocol instruction file to ~/.gemini/mintag-instructions.md.
func setupGemini(homeDir, execPath string) error {
	geminiDir := filepath.Join(homeDir, ".gemini")
	if err := os.MkdirAll(geminiDir, 0o755); err != nil {
		return fmt.Errorf("create ~/.gemini: %w", err)
	}

	settingsPath := filepath.Join(geminiDir, "settings.json")
	var settings map[string]any
	if data, err := os.ReadFile(settingsPath); err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("parse gemini settings.json: %w", err)
		}
	} else {
		settings = map[string]any{}
	}

	servers, _ := settings["mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
		settings["mcpServers"] = servers
	}
	servers["mintag"] = map[string]any{
		"command": execPath,
		"args":    []string{"mcp"},
	}

	if err := writeJSON(settingsPath, settings); err != nil {
		return fmt.Errorf("write gemini settings.json: %w", err)
	}
	fmt.Printf("  patched %s\n", settingsPath)

	instrPath := filepath.Join(geminiDir, "mintag-instructions.md")
	if err := os.WriteFile(instrPath, []byte(geminiInstructions), 0o644); err != nil {
		return fmt.Errorf("write mintag-instructions.md: %w", err)
	}
	fmt.Printf("  wrote %s\n", instrPath)
	return nil
}

// --- helpers ---

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func toStringSlice(v any) []string {
	if v == nil {
		return nil
	}
	raw, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

const geminiInstructions = `# Mintag — Knowledge Graph & Meeting Task Tracker

Mintag is available as an MCP server. Use its tools to answer architecture questions and manage meeting tasks.

## When to use graph tools

- User asks what depends on X → graph_impact
- User asks what X uses or depends on → graph_node (look at "out" relations)
- User asks what a portal exposes → graph_neighbors with relation=exposes
- User asks which repos implement a use case → graph_neighbors with relation=implemented_by
- Unknown graph contents → graph_stats first
- Unknown node key → graph_search first, then use exact key returned

## Workflow

1. graph_stats — discover what the graph contains
2. graph_search — find exact node keys
3. graph_node / graph_neighbors — get node context and direct relations
4. graph_impact — blast-radius: every transitive dependent of a node

## Edge direction convention

SOURCE DEPENDS ON TARGET.
- "out" edges = dependencies of the node
- "in" edges = nodes that depend on this one

## Node kinds

repo | portal | menu_option | use_case | team_project | api_client | note

## Relation types

exposes | consumes | uses_client | implemented_by | implements | belongs_to | relates_to
`

// SupportedAgents lists valid agent names for display in help text.
var SupportedAgents = []string{"claude", "opencode", "gemini"}

// SupportedAgentsList returns a comma-joined string of supported agents.
func SupportedAgentsList() string {
	return strings.Join(SupportedAgents, ", ")
}
