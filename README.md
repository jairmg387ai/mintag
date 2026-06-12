# mintag

Personal knowledge graph, meeting task tracker, and daily work-log tool — all in a single local binary.

- Import meeting transcriptions (`.vtt` / `.txt`), extract tasks, and track them across standups
- Build an architectural knowledge graph linking repos, portals, use-cases, and teams
- Log daily work activities throughout the day and upload them to Azure DevOps TimeLog at end-of-day

All data lives in a local SQLite database. Runs as a web portal and as an MCP server so Claude can read and write everything directly inside a conversation.

---

## How it works

### Meetings & Tasks

1. **Import** a meeting transcription — the parser strips timestamps and collapses speaker lines into clean paragraphs
2. **Create tasks** linked to that meeting — manually through the portal or via Claude using MCP tools
3. **Update status** as work progresses — every change is recorded in `task_history` with the source meeting that triggered it
4. **Add rich notes** to any meeting — paste Markdown or HTML summaries and they render in a dedicated tab

### Knowledge Graph

1. **Upsert nodes** for any architectural entity: repos, portals, menu options, use-cases, team projects, API clients
2. **Link them** with typed edges (`exposes`, `consumes`, `implements`, `belongs_to`, …)
3. **Explore impact** — ask "what breaks if this repo changes?" via `graph_impact`

### Daily Time Log

1. **Log activities** throughout the day (via Claude auto-logging or the portal)
2. **Review and approve** entries at end-of-day
3. **Upload** approved entries to Azure DevOps TimeLog with one command

---

## Requirements

| Tool | Version |
|------|---------|
| Go | 1.23+ |
| Node.js | 18+ (only needed to rebuild the frontend) |

No CGo. The SQLite driver is pure Go (`modernc.org/sqlite`).

---

## Installation

### Option A — download a release binary

Download `mintag.exe` from [Releases](https://github.com/jairmg387ai/mintag/releases) and place it anywhere on your `PATH`.

### Option B — build from source

```bash
git clone https://github.com/jairmg387ai/mintag
cd mintag

# Install frontend deps (first time only)
make frontend-install

# Build frontend + Go binary
make build
```

This produces `mintag.exe` in the project root. The frontend is compiled to `internal/web/static/` and embedded directly into the binary via `go:embed`.

> If you only need the binary and don't care about modifying the UI, you can skip `make frontend-install` and `make frontend-build` — the pre-built static files are already committed.

---

## Usage

```bash
# Start the web portal (http://localhost:7430)
mintag.exe serve

# Start MCP stdio server (pipe to Claude Code)
mintag.exe mcp

# List bundled skills
mintag.exe skills list

# Install a bundled skill for Claude, Gemini, and OpenCode
mintag.exe skills install vtt-task-extractor
```

### Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MINTAG_DB` | `~/.mintag/mintag.db` | Path to SQLite database |
| `MINTAG_PORT` | `7430` | HTTP port for `serve` |
| `MINTAG_AZURE_PAT` | — | Azure DevOps Personal Access Token for TimeLog upload |

---

## Skill installer

Mintag can bundle reusable AI skills and install them into supported local agent folders.

```bash
# Install for all supported targets
mintag.exe skills install vtt-task-extractor

# Install only for Claude and OpenCode
mintag.exe skills install vtt-task-extractor --targets claude,opencode

# Overwrite an existing installation
mintag.exe skills install vtt-task-extractor --force
```

Current target directories:

| Target | Destination |
|--------|-------------|
| Claude | `~/.claude/skills/<skill-name>` |
| Gemini | `~/.gemini/skills/<skill-name>` |
| OpenCode | `~/.config/opencode/skills/<skill-name>` |

---

## Development

Two processes in parallel — the Go API server and the Vite HMR dev server:

```bash
# Terminal 1 — Go API on :7430
make dev-api

# Terminal 2 — Vite with HMR on :5173 (proxies /api to :7430)
make dev-web
```

Open `http://localhost:5173` for live-reload during UI development.

### Make targets

| Target | Description |
|--------|-------------|
| `make build` | Full production build: frontend → `internal/web/static/`, then `go build` |
| `make frontend-install` | `npm install` inside `frontend/` |
| `make frontend-build` | Vite build only (emits to `internal/web/static/`) |
| `make dev-api` | `go run ./cmd/mintag serve` |
| `make dev-web` | `npm run dev` inside `frontend/` |

---

## MCP Integration

Register mintag as an MCP server in `~/.claude/settings.json`:

```json
{
  "mcpServers": {
    "mintag": {
      "command": "C:/path/to/mintag.exe",
      "args": ["mcp"]
    }
  }
}
```

Restart Claude Code after editing the settings file.

### Meetings & Tasks tools

| Tool | Description |
|------|-------------|
| `project_create` | Create a project to group meetings and tasks |
| `project_list` | List all projects |
| `meeting_import` | Import a `.vtt` or `.txt` file from disk |
| `meeting_find_or_create` | Find an existing meeting by title/date or create it |
| `meeting_search` | Full-text search across transcriptions |
| `meeting_set_rich_content` | Attach a Markdown or HTML summary to a meeting |
| `task_create` | Create a task linked to a meeting and/or project |
| `task_upsert` | Create-or-update a task by title (dedup-safe) |
| `task_update` | Update status, owner, priority — every change is logged |
| `task_search` | Full-text search across tasks |
| `task_history` | Full change timeline for a task |
| `tasks_by_project` | List tasks filtered by project and/or status |

### Knowledge Graph tools

| Tool | Description |
|------|-------------|
| `graph_stats` | Node and edge counts by kind — good first call to discover graph content |
| `graph_search` | Full-text search for nodes by name/key |
| `graph_node` | Full 1-hop context for a node (attributes + all relations) |
| `graph_neighbors` | Lightweight: one relation type for a node |
| `graph_impact` | Blast-radius analysis — every node that transitively depends on the given node |
| `graph_upsert_node` | Create or update an architectural node |
| `graph_upsert_edge` | Create or update a typed edge between two nodes |

### Activities (TimeLog) tools

| Tool | Description |
|------|-------------|
| `activity_log` | Record a work activity entry (status starts as `pending`) |
| `activity_list` | List entries filtered by date and/or status |
| `activity_approve` | Approve one or more pending entries by ID |
| `activity_update` | Edit hours, project, category, or description of a pending entry |
| `activity_upload` | Upload approved entries for a date to Azure DevOps TimeLog |

### Example Claude workflow

```
You: log 2h on "API Gateway / Backend Dev / implemented rate limiting middleware"
Claude: [calls activity_log] → Activity recorded as pending

You: show today's activities
Claude: [calls activity_list] → 3 pending entries, 1.5h total

You: approve all and upload to Azure
Claude: [calls activity_approve, activity_upload] → 3 entries uploaded
```

---

## Web Portal

Open `http://localhost:7430` after running `mintag.exe serve`.

| View | Description |
|------|-------------|
| Dashboard | Summary stats and active/blocked tasks at a glance |
| Tasks (List) | Filterable task list by project and status |
| Tasks (Kanban) | Kanban board grouped by status column |
| Meetings | Meeting cards with task count and search |
| Meeting detail | Transcript, task list, and rich content tab (Markdown/HTML) |
| Task detail | Full edit form + change history timeline |
| Graph | Architectural knowledge graph explorer with hierarchy view |
| Activities | Daily time-log: review, approve, and upload entries to Azure DevOps |

---

## REST API

All endpoints under `/api`:

```
GET  /api/stats
GET  /api/search?q=<query>

GET  /api/projects
POST /api/projects

GET  /api/meetings?project_id=<id>
POST /api/meetings/import
GET  /api/meetings/:id
PUT  /api/meetings/:id/rich-content

GET  /api/tasks?project_id=<id>&status=<status>
POST /api/tasks
GET  /api/tasks/:id
PUT  /api/tasks/:id
GET  /api/tasks/:id/history

GET  /api/graph/search?q=&namespace=&kind=&limit=
GET  /api/graph/nodes?kind=&namespace=
GET  /api/graph/nodes/:id
GET  /api/graph/nodes/:id/neighbors?relation=&direction=&limit=
GET  /api/graph/nodes/:id/impact?max_depth=&limit=&kind_filter=
GET  /api/graph/stats?namespace=
GET  /api/graph/namespaces

GET   /api/activities?date=YYYY-MM-DD&status=
POST  /api/activities
GET   /api/activities/catalog
PATCH /api/activities/:id          (action: approve | unapprove | edit fields)
POST  /api/activities/upload?date=YYYY-MM-DD
```

---

## Architecture

```
cmd/mintag/          CLI entrypoint — serve / mcp / skills subcommands
internal/
  store/             SQLite schema, FTS5 indexes, CRUD (meetings, tasks, graph, activities)
  parser/            VTT/TXT → clean text, date extracted from filename
  azure/             Azure DevOps TimeLog API client
  mcp/               MCP stdio server (mark3labs/mcp-go v0.44+)
  server/            HTTP REST API (chi v5)
  web/static/        Embedded dark SPA (go:embed)
  setup/             `mintag setup` guided config wizard
  skillinstall/      Skill installer (Claude / Gemini / OpenCode targets)
frontend/            Vite + React 18 + TypeScript + Tailwind v4 source
```

Dependency order is strict: `parser` → `store` ← `mcp`, `server`. Neither `mcp` nor `server` imports the other. `azure` is imported only by `store` and `server`.

The schema migrates automatically on every `store.Open()` via idempotent `CREATE TABLE IF NOT EXISTS` calls — no migration files needed.

---

## Data model

### Task

| Field | Values |
|-------|--------|
| `status` | `todo` · `in_progress` · `blocked` · `done` · `cancelled` |
| `priority` | `low` · `medium` · `high` · `critical` |

Every `task_update` appends a row to `task_history` with `old_status`, `new_status`, `note`, `author`, and `source_meeting_id`.

### Graph node kinds

`repo` · `portal` · `menu_option` · `use_case` · `team_project` · `api_client` · `note`

### Graph edge relations

`exposes` · `consumes` · `uses_client` · `implemented_by` · `implements` · `belongs_to` · `relates_to`

Direction convention: **source depends on target** (e.g. `menu_option consumes repo`).

### Daily activity

| Field | Values |
|-------|--------|
| `status` | `pending` → `approved` → `uploaded` |
| `source` | `manual` · `llm_auto` |

Approved entries can be unapproved back to `pending` for editing. Uploaded entries are immutable.
