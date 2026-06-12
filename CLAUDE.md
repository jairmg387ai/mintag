# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Full production build (frontend + Go binary)
make build

# Install frontend deps (first time only)
make frontend-install

# Build frontend only (outputs to internal/web/static/)
make frontend-build

# Dev: run Go API server on :7430
make dev-api

# Dev: run Vite HMR dev server on :5173 (proxies /api to :7430)
make dev-web

# Run web portal directly (no rebuild)
mintag.exe serve

# Run MCP stdio server
mintag.exe mcp

# Run tests
go test ./...

# Run a single package's tests
go test ./internal/parser/...

# Tidy dependencies
go mod tidy
```

Environment variables:
- `MINTAG_DB` — path to SQLite database (default: `~/.mintag/mintag.db`)
- `MINTAG_PORT` — HTTP port for `serve` (default: `7430`)

## Frontend

Located in `frontend/`. Built with Vite + React 18 + TypeScript + Tailwind v4.

- `npm run build` emits directly to `internal/web/static/` (Vite outDir)
- `internal/web/static/` is committed to the repo — `go:embed` requires it at compile time
- `frontend/node_modules/` and `frontend/dist/` are gitignored
- State: React Context + useState (no external state library)
- Routing: view-enum in AppContext (no react-router)

## Architecture

Two runtime modes from a single binary (`cmd/mintag/main.go`):

1. **`serve`** — HTTP server with embedded SPA
2. **`mcp`** — stdio-based MCP server for Claude integration

The four internal packages form a strict dependency order: `parser` → `store` ← `mcp`, `server`

```
cmd/mintag/main.go
    ├── internal/store      SQLite via modernc.org/sqlite (pure Go, no CGo)
    ├── internal/parser     VTT/TXT → clean text
    ├── internal/mcp        MCP tools over stdio (mark3labs/mcp-go v0.44+)
    ├── internal/server     REST API via chi v5
    └── internal/web        Embedded dark SPA (go:embed static/)
```

### Store (`internal/store`)

Single `*Store` wraps a `*sql.DB`. Schema is applied inline via `migrate()` on every `Open()` (idempotent `CREATE IF NOT EXISTS`).

Tables: `projects`, `meetings`, `tasks`, `task_history`  
FTS5 virtual tables: `meetings_fts`, `tasks_fts` — kept in sync via `AFTER INSERT / AFTER UPDATE` triggers.

Task status values: `todo | in_progress | blocked | done | cancelled`  
Priority values: `low | medium | high | critical`

Every `UpdateTask` call appends a row to `task_history`, making the full change timeline queryable. The `source_meeting_id` field links a history entry to the meeting that triggered the update.

### Parser (`internal/parser`)

`Parse(filename, content)` returns a `ParsedMeeting` with title, date (extracted from filename via regex), and clean content. For `.vtt` files it strips WebVTT headers, timestamp cues, and collapses consecutive lines from the same speaker into one paragraph.

### MCP Server (`internal/mcp`)

Uses `req.RequireString()` / `req.GetString()` from mcp-go v0.44+ — **do not** index `req.Params.Arguments` directly (it is `any`, not a map).

Tools exposed:
- **Meetings/Tasks**: `project_create`, `project_list`, `meeting_import`, `meeting_find_or_create`, `meeting_search`, `meeting_set_rich_content`, `task_create`, `task_upsert`, `task_update`, `task_search`, `task_history`, `tasks_by_project`
- **Graph**: `graph_stats`, `graph_search`, `graph_node`, `graph_neighbors`, `graph_impact`, `graph_upsert_node`, `graph_upsert_edge`
- **Activities (TimeLog)**: `activity_log`, `activity_list`, `activity_approve`, `activity_update`, `activity_upload`

All tools serialize results as JSON text via `jsonResult()`. Errors are returned as `{"error":"..."}` text, never as Go errors, so Claude can read them.

MCP registration lives in `C:\Users\jmunoz\.claude\settings.json` under key `"mintag"`.

### Web SPA (`internal/web`)

Single `index.html` embedded via `//go:embed static`. The `Handler()` falls back to `index.html` for any path that doesn't match a real file — standard SPA pattern. The UI fetches `/api/*` endpoints directly.

### REST API (`internal/server`)

All routes under `/api`. Pattern: decode JSON body → call `store.*` method → `writeJSON()`. Non-zero query params (`project_id`, `status`) are optional filters passed as `*int64` / `string` to the store.
