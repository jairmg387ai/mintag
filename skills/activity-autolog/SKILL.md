---
name: activity-autolog
description: >
  Auto-log daily work activities via Mintag MCP tools. Two modes:
  (1) Passive: after completing any meaningful work unit, log automatically.
  (2) Explicit: when the user says "registra X horas/minutos de X", resolve
  project + category against the live TimeLog/Azure catalog (never a
  hardcoded list), apply anti-default rules, ask if ambiguous, confirm
  before registering.
license: Apache-2.0
metadata:
  author: mintag
  version: "3.0"
---

## Activation Contract

**Passive mode**: After completing any meaningful work unit in a session, call `activity_log` automatically — without waiting for the user to ask.

**Explicit mode**: Triggered when the user says "registra X horas/minutos de X", "logea actividad", "registra tiempo de", "anota X hora", or any natural-language time-logging request. In this mode: resolve project + category from the live catalog, apply anti-default rules, ask if ambiguous, confirm before registering.

## Hard Rules (both modes)

- **The catalog is dynamic — never hardcode or remember project/category names across sessions.** Projects, categories, and Azure work items live in the database and can be added/removed at any time (`catalog_project_add/remove`, `catalog_category_add/remove`, `catalog_azure_activity_add/remove`). Always resolve against a fresh tool call for the current request; never invent a name not returned by the catalog.
- Never log idle conversation, simple questions, or trivial lookups.
- Never duplicate an entry already logged this session — if unsure, call `activity_list` first.
- Always omit `date` (defaults to today) and `source` (defaults to "llm_auto").
- Round hours to the nearest 0.25h; minimum loggable unit is 0.25h.
- **NEVER** use `Soporte a producción` unless the activity is an active production incident being resolved in real time.
- **Never log hours on a Bug while the bug guard is on.** The TimeLog bug guard (`block_bug_work_item`, read it with `activity_validation_get`) is ON by default: hours for a Bug go to its child Task assigned to the user, never to the Bug itself. Mintag enforces it twice — linking an activity to a catalogued Bug is rejected, and `activity_upload` fails any row whose work item is a Bug (the error lists the child tasks assigned to you) or a Bug's child task assigned to someone else. Resolve to the child Task up front (see Step 2a) instead of relying on those rejections.

---

## MODE 1 — Passive Auto-Log

Runs silently after work is done. No confirmation needed. One-line confirmation after logging.

### Hours Estimation Guide

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

### Passive Decision Gates

| Situation | Action |
|---|---|
| Completed a bug fix | log with category matching "defectos"/"bugs" in the live category list |
| Completed a feature or code change | log with category matching "arquitectura, diseño y código" |
| Attended or ran a meeting | log with category matching "reuniones y sesiones" |
| Wrote or refined specs/requirements | log with category matching "análisis de requisitos" |
| Deployed or set up a pipeline | log with category matching "despliegues y liberaciones" |
| Ran or wrote tests | log with category matching "ejecución y gestión de pruebas" |
| Handled a prod incident | log with category matching "soporte a producción" |
| Work does not clearly fit any category | do not log |
| Cannot resolve a project from context or an open Azure work item | default to `Transversales` if that project exists in the catalog; otherwise ask |
| Already logged a very similar entry this session | do not log (avoid duplicates) |

Resolve category names the same way as Step 3 in Mode 2 below (`catalog_category_list`, best match) — do not assume these labels are exact catalog strings.

### Passive registro_diario format

`{PROJECT}/{CATEGORY}/{brief description of what was done}`

Example: `RNCEA/Gestión y resolución de defectos/Fixed null pointer in task assignment flow`

---

## MODE 2 — Explicit Request Protocol

Triggered by natural-language time-logging requests. Resolves project and category against the live catalog before registering. **Always confirm before calling activity_log.**

### Step 1 — Parse the request

Extract:
- **hours**: convert minutes to decimal (30 min → 0.5, 90 min → 1.5)
- **date**: today unless specified (YYYY-MM-DD)
- **raw activity description**: free-text of what was done
- **project hint / activity hint**: any project name, Azure work item ID, bug label, or context clue

### Step 2 — Resolve project + category

Try these in order. Stop at the first one that resolves cleanly.

**2a. Existing Azure activity mapping (fastest path).**
If the request references a specific Azure work item (an ID, a bug label, "el bug de X") or an activity that was clearly logged before, call `catalog_azure_activity_list` (default `include_inactive=false`). Match the hint against `label` / `work_item_id`.
- If exactly one entry matches and it has both `project` and `category_id` set → that mapping **is** the project/category. Use `project` directly, and resolve `category_id` to a name via `catalog_category_list` (match by `id`). Set `reference_id` to `work_item_id` when calling `activity_log`. **Skip 2b/2c — no catalog listing needed.**
- If it matches but `project` or `category_id` is missing, fall through to 2b/2c only for the missing piece.
- If nothing matches, fall through to 2b.
- **Bug → child Task.** If the hint is a Bug (the matched entry has `work_item_type` `Bug`, or the user says "el bug X") and `activity_validation_get` reports `block_bug_work_item: true`, do NOT use the Bug's entry or ID. Look in the same `catalog_azure_activity_list` result for the bug's child Task assigned to the user (Task-type entries whose label references the bug, e.g. "Atención y/o Corrección del defecto 171306" under Bug 171191) and use that Task's mapping and `work_item_id` as `reference_id`. If none is catalogued or several fit, ask: "¿En cuál tarea hija del bug <id> asignada a ti registro las horas?" — if the user gives a task ID not in the catalog, offer to add it with `catalog_azure_activity_add` (`work_item_type` `Task`) first.

**2b. Resolve project from the live catalog.**
Call `catalog_project_list` (default `include_inactive=false`) and match the project hint against the returned names:
- Exact or unambiguous case-insensitive/partial match (e.g. hint "RNCEA" matches catalog entry `RNCEA`) → use it directly, no need to ask.
- No hint given at all → ask: "¿A qué proyecto corresponde esta actividad?" (optionally show the catalog list if short).
- Hint matches 2+ catalog entries about equally well (e.g. "RNC" could be `Trámites RNC` or `RNC Convalidación`) → list the plausible matches and ask which one.
- Hint matches nothing in the catalog → tell the user no matching project exists and ask them to confirm the name (do not silently create one; if they confirm a new project, use `catalog_project_add` first).
- "mintag, este repo, personal tooling" or similar self-referential work → match against `Transversales` if present in the catalog.

**2c. Resolve category from the live catalog.**
Call `catalog_category_list` (default `include_inactive=false`) and match the activity description's nature (development, testing, meeting, deployment, defect fix, requirements, etc.) against the returned category names/descriptions the same way as 2b: unambiguous match → use it; ambiguous or no match → ask.

### Step 3 — Anti-default rule (CRITICAL)

**NEVER** resolve to a category equivalent to `Soporte a producción` unless the activity is an active production incident being resolved in real time. If the description matches communications, documentation, meetings, cross-team support, development, requirements analysis, testing, deployment, or security/QA assurance work instead, resolve to the catalog category matching that nature — do not fall back to production support just because it's unclear.

### Step 4 — Format registro_diario for Azure

This field becomes the Azure `comment`. Rules:
- Professional Spanish (neutral, not colloquial)
- First letter capitalized
- 1-2 sentences max, describes what was done
- Do NOT include hours, project code, or category

| Raw input | registro_diario to use |
|---|---|
| "envio correo con documentacion de validacion de IP" | `Envío de correo con documentación de validación de IP` |
| "soporte sobre este tema a equipo de QA Proyecto RNCEA" | `Soporte técnico al equipo de QA sobre {tema}` — if "tema" is vague, ask |
| "reunion de seguimiento con el cliente" | `Reunión de seguimiento con el cliente` |
| "desarrollo del módulo de autenticación" | `Desarrollo del módulo de autenticación` |

If the description is too vague (e.g., "sobre este tema", "lo de ayer"), ask for a clearer description before proceeding.

### Step 5 — Confirm before registering

Show summary and wait for explicit confirmation:

```
Voy a registrar:
- Fecha:      {date}
- Horas:      {hours}
- Proyecto:   {project}
- Categoría:  {category}
- Comentario: {registro_diario}
{- Work item: {work_item_id} (si vino de un mapping de Azure)}

¿Confirmás?
```

Accept: yes / sí / dale / ok / confirmar. If user corrects anything, update and show summary again.

### Step 6 — Call activity_log

Only after confirmation:

```
mcp__mintag__activity_log(
  date="{YYYY-MM-DD}",
  hours={float},
  project="{exact project string from the catalog}",
  category="{exact category string from the catalog}",
  registro_diario="{formatted comment}",
  source="llm_auto",
  reference_id="{work_item_id, only if resolved via 2a}"
)
```

---

## Decision Examples

### Example 1 — explicit, project missing
**Prompt:** "registra 30 minutos de envio correo con documentacion de validacion de IP"
- hours: 0.5
- project: not mentioned → **ASK before anything else** (Step 2b, no hint)
- category: correo + documentación + validación → resolve via `catalog_category_list`, match "oficios y comunicaciones"
- registro_diario: `Envío de correo con documentación de validación de IP`

### Example 2 — explicit, cross-team support
**Prompt:** "registra como actividad 1.5 hora de soporte sobre este tema a equipo de QA Proyecto RNCEA"
- hours: 1.5
- project: hint "RNCEA" → `catalog_project_list` returns an unambiguous match → `RNCEA` ✓
- category: soporte + equipo QA (another team) → `catalog_category_list` match on "soporte transversal y operación"
  - Anti-default: NOT a production-support category (not a prod incident)
  - NOT testing-execution category (user supported QA, didn't run tests)
- registro_diario: "sobre este tema" is vague → ask: "¿Sobre qué tema fue el soporte al equipo de QA?"

### Example 3 — explicit, resolved via existing Azure mapping
**Prompt:** "registra 2 horas a la tarea 156263"
- hours: 2.0
- `catalog_azure_activity_list` has an entry with `work_item_id=156263` (`work_item_type` `Task`), `project="RNA Core"`, `category_id` set → use both directly, no listing needed for project/category
- `category_id` resolved to a name via `catalog_category_list`
- registro_diario: ask user what was done if not stated
- `reference_id` = `156263` in the `activity_log` call

### Example 3b — explicit, user names a Bug (guard on)
**Prompt:** "registra 3 horas al bug 171191"
- `catalog_azure_activity_list`: 171191 is `work_item_type` `Bug`; `activity_validation_get` → `block_bug_work_item: true`
- The catalog also has Task 171306 "Atención y/o Corrección del defecto 171306" assigned to the user → resolve to it: its mapping for project/category, `reference_id` = `171306`
- Confirmation summary says the hours go to task 171306 (child of bug 171191), not to the bug

### Example 4 — explicit, unambiguous, no Azure mapping
**Prompt:** "registra 2 horas de desarrollo del login en RNA Core"
- hours: 2.0
- project: hint "RNA Core" → `catalog_project_list` unambiguous match → `RNA Core` ✓
- category: desarrollo → `catalog_category_list` match on "arquitectura, diseño y código" ✓
- registro_diario: `Desarrollo del módulo de login`
- → No ambiguity → go directly to confirmation summary

---

## EOD Summary Trigger

When user says: "qué hice hoy", "resumen del día", "show today's activities", "what did I do today", "activity summary":

1. Call `activity_list` (no args — defaults to today).
2. Present as table: time | project | category | description.
3. Sum total hours logged.
4. If total < 8h: "Total logged: Xh — may have Yh unaccounted."
5. If pending entries: "Run activity_approve then activity_upload to submit to TimeLog."

After `activity_upload`, Mintag syncs each uploaded work item's Completed Work in Azure to the total hours logged in TimeLog for it. That sync is best-effort: failures come back in `effort_sync_errors` while the hours stay uploaded — report them, don't retry the upload.

## Output Contract

- **Passive**: one-line confirmation, e.g. `Logged 1.0h → RNCEA / Gestión y resolución de defectos`
- **Explicit**: confirmation summary → wait → log → one-line result
- **EOD**: formatted table + total + pending reminder
