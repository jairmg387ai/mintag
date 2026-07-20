# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Mintag is a single Go binary that combines: a meeting/task tracker, a code knowledge graph, a daily time-log tool with Azure DevOps TimeLog upload, and a deployment-window ("ventana de despliegue" / interventoría) document generator. It ships both a web portal and an MCP server so Claude can read/write the same SQLite database directly.

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

# Guided config wizard (Azure creds, etc.)
mintag.exe setup

# Install bundled skills into an agent's skills dir
mintag.exe skills list
mintag.exe skills install <skill> [--targets claude,gemini,opencode] [--force]

# Run tests
go test ./...

# Run a single package's tests
go test ./internal/parser/...

# Tidy dependencies
go mod tidy
```

There is no `make lint` / `make test` / `make clean` target — the Makefile only wraps the build/dev commands above. Frontend lint is `npm run lint` (run from `frontend/`) using the flat ESLint config at `frontend/eslint.config.js`. There is no Go lint config (`.golangci.yml`) in the repo — `go vet` / `gofmt` are the baseline.

`go test ./...` does **not** cover `internal/mcp`, `internal/setup`, or `internal/web` — none of those packages have test files. Everything else under `internal/` does.

Requires Go 1.23+ and, only if you're modifying the UI, Node 18+. `internal/web/static/` (the built frontend) is committed, so a fresh clone builds the Go binary without ever touching `frontend/`. SQLite access is pure Go (`modernc.org/sqlite`) — no CGo.

Environment variables:
- `MINTAG_DB` — path to SQLite database (default: `~/.mintag/mintag.db`)
- `MINTAG_PORT` — HTTP port for `serve` (default: `7430`)
- `MINTAG_AZURE_TIMELOG_PAT` — Azure DevOps PAT used for TimeLog upload
- `MINTAG_AZURE_TENANT`, `MINTAG_AZURE_CLIENT_ID`, `MINTAG_AZURE_SCOPE` — override the default Azure AD device-code OAuth app registration (see `internal/azure/oauth.go`)

## Frontend

Located in `frontend/`. Built with Vite + React 19 + TypeScript + Tailwind v4.

- `npm run build` (`tsc -b && vite build`) emits directly to `internal/web/static/` (Vite outDir)
- `internal/web/static/` is committed to the repo — `go:embed` requires it at compile time
- `frontend/node_modules/` and `frontend/dist/` are gitignored
- State: React Context + useState (no external state library)
- Routing: view-enum in AppContext (no react-router)
- Notable libs: `@dnd-kit` (Kanban drag-and-drop), `d3-hierarchy` (graph explorer layout), `react-markdown` + `remark-gfm` + `rehype-highlight` (rich content rendering), `dompurify` (sanitizing rendered HTML), `lucide-react` (icons)

## Architecture

Two runtime modes from a single binary (`cmd/mintag/main.go`):

1. **`serve`** — HTTP server with embedded SPA
2. **`mcp`** — stdio-based MCP server for Claude integration

Dependency order is strict: `parser` → `store` ← `mcp`, `server`. `mcp` and `server` never import each other; `azure` is imported only by `store` and `server`.

```
cmd/mintag/main.go
    ├── internal/store         SQLite via modernc.org/sqlite (pure Go, no CGo)
    ├── internal/parser        VTT/TXT → clean text
    ├── internal/azure         Azure DevOps TimeLog client + AD device-code OAuth
    ├── internal/mcp           MCP tools over stdio (mark3labs/mcp-go v0.44+)
    ├── internal/server        REST API via chi v5
    ├── internal/web           Embedded dark SPA (go:embed static/)
    ├── internal/setup         `mintag setup` guided config wizard
    └── internal/skillinstall  `mintag skills` installer (copies bundled skills to ~/.claude, ~/.gemini, ~/.config/opencode)
```

### Store (`internal/store`)

Single `*Store` wraps a `*sql.DB`. Schema is applied inline via `migrate()` on every `Open()` (idempotent `CREATE TABLE IF NOT EXISTS` — there are no migration files).

Core tables: `projects`, `meetings`, `tasks`, `task_history`, plus graph, activity, deployment-window, and a generic `setting` key/value table (used for Azure OAuth config and tokens — see `internal/store/azure_config.go`).
FTS5 virtual tables: `meetings_fts`, `tasks_fts` — kept in sync via `AFTER INSERT / AFTER UPDATE` triggers.

Task status values: `todo | in_progress | blocked | done | cancelled`
Priority values: `low | medium | high | critical`
Deployment window states: `draft → submitted → approved → deployed` (rejection goes `submitted → draft` and requires a note).

Every `UpdateTask` call appends a row to `task_history`, making the full change timeline queryable. The `source_meeting_id` field links a history entry to the meeting that triggered the update.

### Parser (`internal/parser`)

`Parse(filename, content)` returns a `ParsedMeeting` with title, date (extracted from filename via regex), and clean content. For `.vtt` files it strips WebVTT headers, timestamp cues, and collapses consecutive lines from the same speaker into one paragraph.

### Azure (`internal/azure`)

Two independent pieces: a TimeLog upload client authenticated via PAT (`MINTAG_AZURE_TIMELOG_PAT`), and `DeviceAuthClient`, an OAuth 2.0 device-authorization-code flow against Azure AD (states: `Pending/Complete/Declined/Expired`, sentinel `ErrAuthorizationPending` for polling). Tokens are persisted in the `store`'s `setting` table via `SaveAzureOAuthConfig`/`SaveAzureOAuthTokens`, with `tokenExpiresSoon` gating refresh.

### MCP Server (`internal/mcp`)

Uses `req.RequireString()` / `req.GetString()` from mcp-go v0.44+ — **do not** index `req.Params.Arguments` directly (it is `any`, not a map).

Tools exposed, by file:
- **`mcp.go`** (Meetings/Tasks): `project_create`, `project_list`, `meeting_import`, `meeting_find_or_create`, `meeting_search`, `meeting_set_rich_content`, `task_create`, `task_upsert`, `task_update`, `task_search`, `task_history`, `tasks_by_project`
- **`graph.go`** (Knowledge Graph): `graph_stats`, `graph_search`, `graph_node`, `graph_neighbors`, `graph_impact`, `graph_upsert_node`, `graph_upsert_edge`
- **`deployment_windows.go`** (Deployment Windows): `dw_create`, `dw_list`, `dw_get`, `dw_update_state`, `dw_add_task`, `dw_remove_task`, `dw_add_repo`, `dw_update_repo`, `dw_remove_repo`, `dw_add_artifact`, `dw_update_artifact`, `dw_remove_artifact`, `dw_add_test_scenario`, `dw_update_test_scenario`, `dw_remove_test_scenario`, `dw_sign_off_scenario`, `dw_export_markdown`
- **`activities.go`** (Activities/TimeLog + catalog): `activity_log`, `activity_list`, `activity_approve`, `activity_update`, `activity_upload`, `activity_delete`, `catalog_project_add`, `catalog_project_remove`, `catalog_category_add`, `catalog_category_remove`, `catalog_azure_activity_add`, `catalog_azure_activity_list`, `catalog_azure_activity_set_default`, `catalog_azure_activity_remove`

All tools serialize results as JSON text via `jsonResult()`. Errors are returned as `{"error":"..."}` text, never as Go errors, so Claude can read them.

MCP registration lives in `C:\Users\jmunoz\.claude\settings.json` under key `"mintag"`.

### Web SPA (`internal/web`)

Single `index.html` embedded via `//go:embed static`. The `Handler()` falls back to `index.html` for any path that doesn't match a real file — standard SPA pattern. The UI fetches `/api/*` endpoints directly.

### REST API (`internal/server`)

All routes under `/api`. Pattern: decode JSON body → call `store.*` method → `writeJSON()`. Non-zero query params (`project_id`, `status`) are optional filters passed as `*int64` / `string` to the store.

Router uses chi with `middleware.Logger`, `middleware.Recoverer`, and a custom `localCORSMiddleware` restricting cross-origin requests to localhost. A separate, stricter `requireLocalRequest` middleware (checks Origin, Host, and RemoteAddr are all loopback) exists for extra guarding — check its call sites before assuming every route has the same trust boundary.
