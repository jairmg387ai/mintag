---
name: activity-autolog
description: >
  Auto-log daily work activities via Mintag MCP tools. Trigger: after completing
  any meaningful work unit (bug fix, feature, meeting, code review, analysis,
  deploy, QA, requirements). Maps work type to project and category automatically.
license: Apache-2.0
metadata:
  author: mintag
  version: "1.0"
---

## Activation Contract

After completing any meaningful work unit in a session, call `activity_log` automatically — without waiting for the user to ask. The goal is to accumulate an accurate daily log so the user never has to reconstruct what they did at end of day.

This skill is always active during work sessions. Do not ask for permission to log.

## Hard Rules

- Never invent a project name not in the catalog below.
- Never log idle conversation, simple questions, or trivial lookups.
- Never duplicate an entry already logged this session — if unsure, call `activity_list` first.
- Always omit `date` (defaults to today) and `source` (defaults to "llm_auto").
- Round hours to the nearest 0.25h; minimum loggable unit is 0.25h.
- `registro_diario` MUST follow the format: `{PROJECT}/{CATEGORY}/{description}`.

## Project Catalog (use exactly these names)

Common projects (use the closest match):

| Context clues | Project name |
|---|---|
| mintag, personal tooling, this repo, Gentleman tooling | Transversales |
| RNCEA, 169xxx, módulo rentas | RNCEA |
| RNMA, matrícula | RNMA |
| Bridge, integración Bridge | Bridge |
| CALES, calendar, scheduling | CALES |
| APP RUNT, RUNT app | APP RUNT |
| RNA Core, RNA | RNA Core |
| Unknown / cannot determine | Transversales |

When unsure, always default to "Transversales" — never invent a name.

## Category Mapping

| Work type | Category |
|---|---|
| Bug fix, defect resolution, error correction, hotfix | Gestión y resolución de defectos |
| Feature implementation, refactor, code review, PR, architecture design, coding | Actividades de arquitectura, diseño y código |
| Meeting, standup, daily, sync, follow-up, demo, retrospective | Reuniones y sesiones de trabajo |
| Requirements analysis, spec writing, design doc, RFC, user story refinement | Gestión y análisis de requisitos |
| Deploy, release, pipeline, CI/CD, environment setup | Gestión de despliegues y liberaciones |
| QA, testing, test execution, test plan, automated tests | Ejecución y gestión de pruebas |
| Production incident, production support, on-call | Soporte a producción |

## Hours Estimation Guide

| Work size | Hours |
|---|---|
| Quick fix, trivial task, small lookup with action | 0.25 |
| Small task, focused fix, targeted review | 0.5 |
| Medium implementation, analysis, feature slice | 1.0 |
| Substantial implementation or investigation | 1.5 |
| Large feature, complex refactor, long session | 2.0–3.0 |
| Short meeting (standup, sync) | 0.5 |
| Standard meeting | 1.0 |
| Long meeting or workshop | 1.5–2.0 |

When estimating, factor in the actual conversation depth and work performed — not just elapsed wall time.

## Decision Gates

| Situation | Action |
|---|---|
| Completed a bug fix | log with category "Gestión y resolución de defectos" |
| Completed a feature or code change | log with category "Actividades de arquitectura, diseño y código" |
| Attended or ran a meeting | log with category "Reuniones y sesiones de trabajo" |
| Wrote or refined specs/requirements | log with category "Gestión y análisis de requisitos" |
| Deployed or set up a pipeline | log with category "Gestión de despliegues y liberaciones" |
| Ran or wrote tests | log with category "Ejecución y gestión de pruebas" |
| Handled a prod incident | log with category "Soporte a producción" |
| Work does not clearly fit any category | do not log |
| Cannot map to a catalog project | do not log |
| Already logged a very similar entry this session | do not log (avoid duplicates) |

## Execution Steps

1. Identify the completed work unit (what was done, how long it took).
2. Map to a project from the catalog using context clues (repo name, user mentions, file paths).
3. Map to a category from the table above.
4. Estimate hours using the guide above.
5. Build `registro_diario`: `{PROJECT}/{CATEGORY}/{brief description of what was done}`.
   - Description should be specific enough for monthly reporting (not just "fixed bug" — say "fixed null pointer in RNCEA task assignment flow").
6. Call `activity_log(hours, project, category, registro_diario)`.
7. Confirm silently (no verbose announcement needed — one line like "Logged 1.0h to RNCEA" is enough).

## EOD Summary Trigger

When the user says any of: "qué hice hoy", "resumen del día", "show today's activities", "what did I do today", "activity summary", or similar:

1. Call `activity_list` (no args — defaults to today).
2. Present the entries as a table: time, project, category, description.
3. Sum total hours logged.
4. If total < 8h, mention: "Total logged: Xh — you may have Yh unaccounted."
5. If any entries are pending approval, remind: "Run activity_approve then activity_upload to submit to TimeLog."

## Output Contract

- After auto-logging: one-line confirmation, e.g. `Logged 1.0h → RNCEA / Gestión y resolución de defectos`.
- After EOD summary: formatted table + total hours + any pending-approval reminder.
- Never produce verbose log announcements mid-task — keep it compact.
