package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/Gentleman-Programming/mintag/internal/mcp"
	"github.com/Gentleman-Programming/mintag/internal/server"
	"github.com/Gentleman-Programming/mintag/internal/store"
)

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
	default:
		usage()
	}
}

func runServe(dbPath string) {
	port := envOr("MINTAG_PORT", "7430")
	st, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("cannot open db: %v", err)
	}
	defer st.Close()

	srv := server.New(st)
	addr := ":"+port
	fmt.Printf("mintag listening on http://localhost%s  (db: %s)\n", addr, dbPath)
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

func usage() {
	fmt.Fprint(os.Stderr, `mintag — Meeting Task Tracker

Commands:
  mintag serve   Start the web portal (default port 7430)
  mintag mcp     Start MCP stdio server for Claude integration

Environment:
  MINTAG_DB     Path to SQLite database (default: ~/.mintag/mintag.db)
  MINTAG_PORT   HTTP port for serve command (default: 7430)
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
