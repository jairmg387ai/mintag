package setup_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Gentleman-Programming/mintag/internal/setup"
)

const testExec = "/opt/mintag/mintag"

func readJSONFile(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return out
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func wantMintagEntry() map[string]any {
	return map[string]any{"command": testExec, "args": []any{"mcp"}}
}

func mcpServers(t *testing.T, cfg map[string]any) map[string]any {
	t.Helper()
	servers, ok := cfg["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("mcpServers missing or not an object: %#v", cfg["mcpServers"])
	}
	return servers
}

func gentleShellPath(home string) string {
	return filepath.Join(home, ".gentle-shell", "agent", "mcp.json")
}

func antigravityPath(home string) string {
	return filepath.Join(home, ".gemini", "config", "mcp_config.json")
}

func TestGentleShellCreatesConfigFromScratch(t *testing.T) {
	home := t.TempDir()
	if err := setup.Run("gentle-shell", home, testExec); err != nil {
		t.Fatalf("Run: %v", err)
	}
	cfg := readJSONFile(t, gentleShellPath(home))
	want := map[string]any{"mcpServers": map[string]any{"mintag": wantMintagEntry()}}
	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("config = %#v, want %#v", cfg, want)
	}
}

func TestGentleShellPreservesOthersAndReplacesStaleEntry(t *testing.T) {
	home := t.TempDir()
	writeFile(t, gentleShellPath(home), `{
  "theme": "dark",
  "mcpServers": {
    "context7": {"command": "npx", "args": ["-y", "@upstash/context7-mcp"]},
    "mintag": {"command": "/old/mintag", "args": ["serve"], "env": {"X": "1"}}
  }
}`)
	if err := setup.Run("gentle-shell", home, testExec); err != nil {
		t.Fatalf("Run: %v", err)
	}
	cfg := readJSONFile(t, gentleShellPath(home))
	if cfg["theme"] != "dark" {
		t.Fatalf("top-level key not preserved: %#v", cfg["theme"])
	}
	servers := mcpServers(t, cfg)
	wantCtx := map[string]any{"command": "npx", "args": []any{"-y", "@upstash/context7-mcp"}}
	if !reflect.DeepEqual(servers["context7"], wantCtx) {
		t.Fatalf("context7 = %#v, want %#v", servers["context7"], wantCtx)
	}
	if !reflect.DeepEqual(servers["mintag"], wantMintagEntry()) {
		t.Fatalf("mintag = %#v, want %#v", servers["mintag"], wantMintagEntry())
	}
	if len(servers) != 2 {
		t.Fatalf("unexpected server count %d: %#v", len(servers), servers)
	}
}

func TestAntigravityAndAgyAliasWriteMCPConfig(t *testing.T) {
	for _, agent := range []string{"antigravity", "agy"} {
		t.Run(agent, func(t *testing.T) {
			home := t.TempDir()
			writeFile(t, antigravityPath(home), `{"mcpServers": {
  "engram": {"command": "engram", "args": ["mcp"]},
  "context7": {"serverUrl": "https://mcp.context7.com/mcp"}
}}`)
			if err := setup.Run(agent, home, testExec); err != nil {
				t.Fatalf("Run: %v", err)
			}
			servers := mcpServers(t, readJSONFile(t, antigravityPath(home)))
			want := map[string]any{
				"engram":   map[string]any{"command": "engram", "args": []any{"mcp"}},
				"context7": map[string]any{"serverUrl": "https://mcp.context7.com/mcp"},
				"mintag":   wantMintagEntry(),
			}
			if !reflect.DeepEqual(servers, want) {
				t.Fatalf("servers = %#v, want %#v", servers, want)
			}
		})
	}
}

func TestAntigravityCreatesConfigFromScratch(t *testing.T) {
	home := t.TempDir()
	if err := setup.Run("agy", home, testExec); err != nil {
		t.Fatalf("Run: %v", err)
	}
	cfg := readJSONFile(t, antigravityPath(home))
	want := map[string]any{"mcpServers": map[string]any{"mintag": wantMintagEntry()}}
	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("config = %#v, want %#v", cfg, want)
	}
}

func TestInvalidJSONReturnsErrorAndLeavesFileUnchanged(t *testing.T) {
	cases := map[string]func(string) string{
		"gentle-shell": gentleShellPath,
		"antigravity":  antigravityPath,
	}
	for agent, pathFn := range cases {
		t.Run(agent, func(t *testing.T) {
			home := t.TempDir()
			path := pathFn(home)
			original := `{"mcpServers": {"broken": `
			writeFile(t, path, original)
			err := setup.Run(agent, home, testExec)
			if err == nil {
				t.Fatal("expected error for invalid JSON")
			}
			if !strings.Contains(err.Error(), filepath.Base(path)) {
				t.Fatalf("error %q does not mention %s", err, filepath.Base(path))
			}
			got, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if !bytes.Equal(got, []byte(original)) {
				t.Fatalf("file modified: %q", got)
			}
		})
	}
}

func TestUnknownAgentListsSupportedAgents(t *testing.T) {
	err := setup.Run("vscode", t.TempDir(), testExec)
	if err == nil {
		t.Fatal("expected error for unknown agent")
	}
	for _, name := range []string{"claude", "opencode", "gemini", "gentle-shell", "antigravity", "agy"} {
		if !strings.Contains(err.Error(), name) {
			t.Fatalf("error %q does not mention %q", err, name)
		}
	}
	want := []string{"claude", "opencode", "gemini", "gentle-shell", "antigravity"}
	if !reflect.DeepEqual(setup.SupportedAgents, want) {
		t.Fatalf("SupportedAgents = %v, want %v", setup.SupportedAgents, want)
	}
}

// --- regression pins for pre-existing agents ---

func TestClaudeSetupWritesMCPFileAndPermissionOnce(t *testing.T) {
	home := t.TempDir()
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	writeFile(t, settingsPath, `{"model": "opus", "permissions": {"allow": ["Bash(ls)"]}}`)

	for i := 0; i < 2; i++ {
		if err := setup.Run("claude", home, testExec); err != nil {
			t.Fatalf("Run #%d: %v", i+1, err)
		}
	}

	mcp := readJSONFile(t, filepath.Join(home, ".claude", "mcp", "mintag.json"))
	if want := map[string]any{"mintag": wantMintagEntry()}; !reflect.DeepEqual(mcp, want) {
		t.Fatalf("mintag.json = %#v, want %#v", mcp, want)
	}

	settings := readJSONFile(t, settingsPath)
	if settings["model"] != "opus" {
		t.Fatalf("settings key not preserved: %#v", settings)
	}
	perms := settings["permissions"].(map[string]any)
	want := []any{"Bash(ls)", "mcp__mintag__*"}
	if !reflect.DeepEqual(perms["allow"], want) {
		t.Fatalf("allow = %#v, want %#v", perms["allow"], want)
	}
}

func TestOpenCodeSetupWritesLocalMCPEntry(t *testing.T) {
	home := t.TempDir()
	cfgPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	writeFile(t, cfgPath, `{"theme": "x", "mcp": {"other": {"type": "remote"}}}`)
	if err := setup.Run("opencode", home, testExec); err != nil {
		t.Fatalf("Run: %v", err)
	}
	cfg := readJSONFile(t, cfgPath)
	if cfg["theme"] != "x" {
		t.Fatalf("theme not preserved: %#v", cfg)
	}
	mcp := cfg["mcp"].(map[string]any)
	want := map[string]any{
		"other":  map[string]any{"type": "remote"},
		"mintag": map[string]any{"type": "local", "command": []any{testExec, "mcp"}, "enabled": true},
	}
	if !reflect.DeepEqual(mcp, want) {
		t.Fatalf("mcp = %#v, want %#v", mcp, want)
	}
}

func TestGeminiSetupWritesSettingsAndInstructions(t *testing.T) {
	home := t.TempDir()
	settingsPath := filepath.Join(home, ".gemini", "settings.json")
	writeFile(t, settingsPath, `{"theme": "y", "mcpServers": {"engram": {"command": "engram", "args": ["mcp"]}}}`)
	if err := setup.Run("gemini", home, testExec); err != nil {
		t.Fatalf("Run: %v", err)
	}
	got, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	// Pin exact bytes: writeJSON uses 2-space indent, sorted keys, trailing newline.
	want := "{\n" +
		"  \"mcpServers\": {\n" +
		"    \"engram\": {\n" +
		"      \"args\": [\n" +
		"        \"mcp\"\n" +
		"      ],\n" +
		"      \"command\": \"engram\"\n" +
		"    },\n" +
		"    \"mintag\": {\n" +
		"      \"args\": [\n" +
		"        \"mcp\"\n" +
		"      ],\n" +
		"      \"command\": \"" + testExec + "\"\n" +
		"    }\n" +
		"  },\n" +
		"  \"theme\": \"y\"\n" +
		"}\n"
	if string(got) != want {
		t.Fatalf("settings.json =\n%s\nwant\n%s", got, want)
	}
	instr, err := os.ReadFile(filepath.Join(home, ".gemini", "mintag-instructions.md"))
	if err != nil {
		t.Fatalf("instructions not written: %v", err)
	}
	if !strings.HasPrefix(string(instr), "# Mintag") {
		t.Fatalf("unexpected instructions content: %q", instr[:min(40, len(instr))])
	}
}
