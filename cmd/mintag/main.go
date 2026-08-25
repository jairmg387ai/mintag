package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Gentleman-Programming/mintag/internal/mcp"
	"github.com/Gentleman-Programming/mintag/internal/server"
	"github.com/Gentleman-Programming/mintag/internal/setup"
	"github.com/Gentleman-Programming/mintag/internal/skillinstall"
	"github.com/Gentleman-Programming/mintag/internal/store"
)

// version is set at build time via -ldflags "-X main.version=..." (see .goreleaser.yaml).
var version = "dev"

func main() {
	if len(os.Args) < 2 {
		usage()
	}

	dbPath := envOr("MINTAG_DB", filepath.Join(userHome(), ".mintag", "mintag.db"))

	switch os.Args[1] {
	case "serve":
		runServe(dbPath)
	case "mcp":
		runMCP(dbPath)
	case "skills":
		runSkills(os.Args[2:])
	case "setup":
		runSetup(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Println("mintag " + version)
	default:
		usage()
	}
}

func runServe(dbPath string) {
	port := envOr("MINTAG_PORT", "7430")
	host := envOr("MINTAG_HOST", "127.0.0.1")
	st, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("cannot open db: %v", err)
	}
	defer st.Close()

	srv := server.New(st)
	srv.Version = version
	addr := host + ":" + port
	displayHost := host
	if displayHost == "127.0.0.1" || displayHost == "::1" {
		displayHost = "localhost"
	}
	fmt.Printf("mintag listening on http://%s:%s  (db: %s)\n", displayHost, port, dbPath)
	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func runMCP(dbPath string) {
	st, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("cannot open db: %v", err)
	}
	defer st.Close()
	if err := mcp.Serve(st); err != nil {
		log.Fatalf("mcp error: %v", err)
	}
}

func runSkills(args []string) {
	if len(args) == 0 {
		skillsUsage()
	}

	switch args[0] {
	case "list":
		for _, skillName := range skillinstall.AvailableSkills() {
			files, err := skillinstall.SkillFilePaths(skillName)
			if err != nil {
				log.Fatalf("skills list error: %v", err)
			}
			fmt.Printf("%s -> %s\n", skillName, strings.Join(files, ", "))
		}
	case "install":
		installSkills(args[1:])
	default:
		skillsUsage()
	}
}

func installSkills(args []string) {
	fs := flag.NewFlagSet("skills install", flag.ExitOnError)
	targetsArg := fs.String("targets", "claude,gemini,opencode", "comma-separated targets: claude,gemini,opencode")
	force := fs.Bool("force", false, "overwrite an existing installed skill")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage:
	  mintag skills install <skill> [<skill> ...] [--targets claude,gemini,opencode] [--force]

Examples:
	  mintag skills install vtt-task-extractor
	  mintag skills install vtt-task-extractor --targets claude,opencode
	  mintag skills install vtt-task-extractor --targets gemini --force
`)
	}
	if err := fs.Parse(args); err != nil {
		log.Fatalf("skills install parse error: %v", err)
	}

	skillNames := fs.Args()
	if len(skillNames) == 0 {
		fs.Usage()
		os.Exit(1)
	}

	targets, err := skillinstall.ParseTargets(*targetsArg)
	if err != nil {
		log.Fatalf("invalid targets: %v", err)
	}

	homeDir := userHome()
	results, err := skillinstall.Install(skillNames, targets, homeDir, *force)
	if err != nil {
		log.Fatalf("skills install error: %v", err)
	}

	fmt.Println(skillinstall.FormatResults(results))
}

func runSetup(args []string) {
	if len(args) == 0 {
		setupUsage()
	}

	agent := args[0]

	execPath, err := os.Executable()
	if err != nil {
		log.Fatalf("cannot resolve executable path: %v", err)
	}

	homeDir := userHome()

	fmt.Printf("Configuring mintag MCP for %s...\n", agent)
	if err := setup.Run(agent, homeDir, execPath); err != nil {
		log.Fatalf("setup error: %v", err)
	}
	fmt.Printf("Done. Restart %s to apply changes.\n", agent)
}

func setupUsage() {
	fmt.Fprintf(os.Stderr, `mintag setup <agent>

Agents:
  %s

Examples:
  mintag setup claude
  mintag setup opencode
  mintag setup gemini
`, setup.SupportedAgentsList())
	os.Exit(1)
}

func usage() {
	fmt.Fprint(os.Stderr, `mintag — Meeting Task Tracker

Commands:
  mintag serve    Start the web portal (default port 7430)
  mintag mcp      Start MCP stdio server for Claude integration
  mintag skills   List or install bundled AI skills
  mintag setup    Configure MCP integration for an AI agent (claude, opencode, gemini)
  mintag version  Print the mintag version

Environment:
  MINTAG_DB     Path to SQLite database (default: ~/.mintag/mintag.db)
  MINTAG_PORT   HTTP port for serve command (default: 7430)
  MINTAG_HOST   HTTP bind host for serve command (default: 127.0.0.1)
`)
	os.Exit(1)
}

func skillsUsage() {
	fmt.Fprint(os.Stderr, `mintag skills

Commands:
  mintag skills list
  mintag skills install <skill> [<skill> ...] [--targets claude,gemini,opencode] [--force]
`)
	os.Exit(1)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func userHome() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return h
}
