# mintag

Meeting task tracker — import VTT/TXT transcriptions, extract tasks, and track them across meetings.

Replaces the manual workflow of copying notes into todo lists after each standup. All data lives in a local SQLite database with FTS5 full-text search. Runs as a web portal and as an MCP server so Claude can read and update tasks directly inside a conversation.

---

## How it works

1. **Import** a meeting transcription (`.vtt` or `.txt`) — the parser strips timestamps and collapses speaker lines into clean paragraphs
2. **Create tasks** linked to that meeting — manually through the portal or via Claude using the MCP tools
3. **Update status** as work progresses — every change is recorded in `task_history` with the source meeting that triggered it
4. **Add rich notes** to any meeting — paste Markdown or HTML summaries and they render in a dedicated tab
5. **Search everything** with full-text search across transcriptions and task titles/descriptions

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

Bundled skills are listed with:

```bash
mintag.exe skills list
```

---

## Development

Two processes in parallel — the Go API server and the Vite HMR dev server:

```bash
# Terminal 1 — Go API on :7430
make dev-api

# Terminal 2 — Vite with HMR on :5173 (proxies /api to :7430)
make dev-web
```

Open `http://localhost:5173` for live-reload during UI development. The Vite proxy forwards all `/api/*` requests to the Go server, so you get real data with hot-reload on the frontend.

### Make targets

| Target | Description |
|--------|-------------|
| `make build` | Full production build: frontend → `internal/web/static/`, then `go build` |
| `make frontend-install` | `npm install` inside `frontend/` |
| `make frontend-build` | Vite build only (emits to `internal/web/static/`) |
| `make dev-api` | `go run ./cmd/mintag serve` (hot-reload via external runner) |
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

Restart Claude Code after editing the settings file. Once registered, Claude has access to:

| Tool | Description |
|------|-------------|
| `project_create` | Create a project to group meetings and tasks |
| `project_list` | List all projects |
| `meeting_import` | Import a `.vtt` or `.txt` file from disk |
| `meeting_search` | Full-text search across transcriptions |
| `meeting_set_rich_content` | Attach a Markdown or HTML summary to a meeting |
| `task_create` | Create a task linked to a meeting and/or project |
| `task_update` | Update status, owner, priority — every change is logged |
| `task_search` | Full-text search across tasks |
| `task_history` | Full change timeline for a task |
| `tasks_by_project` | List tasks filtered by project and/or status |

### Example Claude workflow

```
You: import this week's standup from ~/meetings/2025-06-04-standup.vtt
Claude: [calls meeting_import] → Imported "Standup 2025-06-04" with 12 action items

You: create tasks for the backend migration items
Claude: [calls task_create ×3] → Tasks created and linked to the meeting

You: mark the auth refactor as in_progress, owner @jair
Claude: [calls task_update] → Updated. Change recorded in history.
```

---

## Web Portal

Open `http://localhost:7430` after running `mintag.exe serve`.

| View | Description |
|------|-------------|
| Dashboard | Summary stats and active/blocked tasks at a glance |
| Tasks (List) | Filterable task list by project and status |
| Tasks (Kanban) | Drag-free kanban grouped by status column |
| Meetings | Meeting cards with task count and search |
| Meeting detail | Transcript, task list, and rich content tab (Markdown/HTML) |
| Task detail | Full edit form + change history timeline |

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
```

---

## Architecture

```
cmd/mintag/          CLI entrypoint — serve / mcp subcommands
internal/
  store/             SQLite schema, FTS5 indexes, CRUD
  parser/            VTT/TXT → clean text, date extracted from filename
  mcp/               MCP stdio server (mark3labs/mcp-go v0.44+)
  server/            HTTP REST API (chi v5)
  web/static/        Embedded dark SPA (go:embed)
frontend/            Vite + React 19 + TypeScript + Tailwind v4 source
```

Dependency order is strict: `parser` → `store` ← `mcp`, `server`. Neither `mcp` nor `server` imports the other.

The schema migrates automatically on every `store.Open()` via idempotent `CREATE TABLE IF NOT EXISTS` and `ALTER TABLE ... ADD COLUMN IF NOT EXISTS` calls — no migration files needed.

---

## Data model

### Task

| Field | Values |
|-------|--------|
| `status` | `todo` · `in_progress` · `blocked` · `done` · `cancelled` |
| `priority` | `low` · `medium` · `high` · `critical` |

Every `task_update` appends a row to `task_history` with `old_status`, `new_status`, `note`, `author`, and `source_meeting_id` — so the full audit trail is always queryable.

### Meeting rich content

A meeting can optionally carry a `rich_content` blob with a `content_type` of `markdown` or `html`. This is separate from the raw transcript (`raw_content`) and is meant for curated summaries written by Claude or a human after the fact.
