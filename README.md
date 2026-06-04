# mintag

Meeting task tracker — import VTT/TXT transcriptions, extract tasks, track them across meetings.

Replaces the manual workflow of updating HTML files after each meeting. All data lives in a local SQLite database with FTS5 full-text search.

## How it works

1. Import a meeting transcription (`.vtt` or `.txt`) — the parser strips timestamps and collapses speaker lines
2. Create tasks linked to that meeting (manually or via Claude through MCP)
3. Update task status as meetings progress — every change is recorded in history with the source meeting
4. Query or browse everything through the web portal or via Claude tools

## Installation

```bash
git clone https://github.com/jairmg387ai/mintag
cd mintag
go build -o mintag.exe ./cmd/mintag
```

Requires Go 1.23+. No CGo, no external dependencies.

## Usage

```bash
# Start the web portal (http://localhost:7430)
mintag.exe serve

# Start MCP server (for Claude integration)
mintag.exe mcp
```

Environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `MINTAG_DB` | `~/.mintag/mintag.db` | Path to SQLite database |
| `MINTAG_PORT` | `7430` | HTTP port for `serve` |

## MCP Integration

Register in `~/.claude/settings.json`:

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

Available tools once registered:

| Tool | Description |
|------|-------------|
| `project_create` | Create a project to group meetings and tasks |
| `project_list` | List all projects |
| `meeting_import` | Import a `.vtt` or `.txt` file from disk |
| `meeting_search` | Full-text search across transcriptions |
| `task_create` | Create a task (linked to meeting/project) |
| `task_update` | Update status, owner, priority — every change logged |
| `task_search` | Full-text search across tasks |
| `task_history` | Full change timeline for a task |
| `tasks_by_project` | List tasks filtered by project and/or status |

## Architecture

```
cmd/mintag/          CLI entrypoint — serve / mcp subcommands
internal/
  store/             SQLite schema, FTS5, CRUD (projects, meetings, tasks, task_history)
  parser/            VTT/TXT → clean text, extracts date from filename
  mcp/               MCP stdio server (mark3labs/mcp-go v0.44+)
  server/            HTTP REST API (chi v5)
  web/static/        Embedded dark SPA (dashboard, kanban, history timeline)
```

The database schema is applied automatically on first run via `store.Open()`. FTS5 indexes are kept in sync through triggers.

## Task model

- **Status**: `todo` · `in_progress` · `blocked` · `done` · `cancelled`
- **Priority**: `low` · `medium` · `high` · `critical`
- Every `task_update` appends a row to `task_history` with `source_meeting_id` — so you can trace which meeting caused each status change.

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

GET  /api/tasks?project_id=<id>&status=<status>
POST /api/tasks
GET  /api/tasks/:id
PUT  /api/tasks/:id
GET  /api/tasks/:id/history
```
