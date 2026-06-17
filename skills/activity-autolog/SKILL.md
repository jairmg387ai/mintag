---
name: activity-autolog
description: >
  Auto-log daily work activities via Mintag MCP tools. Two modes:
  (1) Passive: after completing any meaningful work unit, log automatically.
  (2) Explicit: when the user says "registra X horas/minutos de X", apply
  project detection + category mapping + anti-default rules + confirmation gate
  before calling activity_log. Never use "Soporte a producción" as a default.
license: Apache-2.0
metadata:
  author: mintag
  version: "2.0"
---

## Activation Contract

**Passive mode**: After completing any meaningful work unit in a session, call `activity_log` automatically — without waiting for the user to ask.

**Explicit mode**: Triggered when the user says "registra X horas/minutos de X", "logea actividad", "registra tiempo de", "anota X hora", or any natural-language time-logging request. In this mode: detect project + category, apply anti-default rules, ask if ambiguous, confirm before registering.

## Hard Rules (both modes)

- Never invent a project name not in the catalog below.
- Never log idle conversation, simple questions, or trivial lookups.
- Never duplicate an entry already logged this session — if unsure, call `activity_list` first.
- Always omit `date` (defaults to today) and `source` (defaults to "llm_auto").
- Round hours to the nearest 0.25h; minimum loggable unit is 0.25h.
- **NEVER** use `Soporte a producción` unless the activity is an active production incident being resolved in real time.

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
| Completed a bug fix | log with category "Gestión y resolución de defectos" |
| Completed a feature or code change | log with category "Actividades de arquitectura, diseño y código" |
| Attended or ran a meeting | log with category "Reuniones y sesiones de trabajo" |
| Wrote or refined specs/requirements | log with category "Gestión y análisis de requisitos" |
| Deployed or set up a pipeline | log with category "Gestión de despliegues y liberaciones" |
| Ran or wrote tests | log with category "Ejecución y gestión de pruebas" |
| Handled a prod incident | log with category "Soporte a producción" |
| Work does not clearly fit any category | do not log |
| Cannot map to a catalog project | default to Transversales |
| Already logged a very similar entry this session | do not log (avoid duplicates) |

### Passive registro_diario format

`{PROJECT}/{CATEGORY}/{brief description of what was done}`

Example: `RNCEA/Gestión y resolución de defectos/Fixed null pointer in task assignment flow`

---

## MODE 2 — Explicit Request Protocol

Triggered by natural-language time-logging requests. Applies full categorization judgment before registering. **Always confirm before calling activity_log.**

### Step 1 — Parse the request

Extract:
- **hours**: convert minutes to decimal (30 min → 0.5, 90 min → 1.5)
- **date**: today unless specified (YYYY-MM-DD)
- **raw activity description**: free-text of what was done
- **project hint**: any project name, acronym, or context clue

### Step 2 — Detect project

Match to the exact strings below. No invention, no approximation.

| If user says… | Exact project string |
|---|---|
| RNCEA | `RNCEA` |
| RNMA | `RNMA` |
| RNRYS | `RNRYS` |
| RNIT | `RNIT` |
| RNAT | `RNAT` |
| RNA Core / RNA principal | `RNA Core` |
| RNA Otros / RNA tramites otros | `RNA Otros` |
| RNA Importacion / importación temporal | `RNA Importacion temporal` |
| RNA OT | `RNA Otros Tramites OT` |
| RNC Convalidación / convalidación | `RNC Convalidación` |
| Tramites RNC / trámites RNC | `Trámites RNC` |
| RNET | `RNET (1,2,3,4,5)` |
| APP RUNT / app runt | `APP RUNT` |
| Bridge / hallazgos bridge | `Bridge` |
| Blockchain | `Blockchain` |
| Escuelas / escuelas ZD | `Escuelas` |
| Transversal / transversales | `Transversales` |
| soporte a la operación / soporte operación | `Soporte a la Operación` |
| CALES | `CALES` |
| CRC | `CRC` |
| PCR | `PCR` |
| FUEC | `FUEC` |
| Fuec Movil / fuec móvil | `Fuec Movil` |
| PVO | `PVO` |
| CIA | `CIA` |
| Recaudo | `Recaudo` |
| Sede Electrónica / sede electronica | `Sede Electrónica` |
| RNEC | `RNEC` |
| Kioskos / kioscos | `Kioskos APP` |
| Sagir / SAGIR | `Sagir` |
| FTH | `FTH` |
| Blindaje | `Blindaje` |
| PNJ | `PNJ` |
| RTM | `RTM` |
| Garantías Mobiliarias / garantías | `Garantias Mobiliarias` |
| Validador Central | `Validador Central` |
| Prevalidador | `Prevalidador (Validador de Requisitos)` |
| Contenedor / CSE | `Contenedor de Servicios Estratégicos` |
| WS Homologados | `WS Homologados` |
| Cargues / carga de archivo | `Cargues de archivo` |
| TO / Tarjeta de Operación | `Tarjeta de Operación` |
| Migración Oracle / oracle 19 | `Migración Oracle 19` |
| Organismos de Apoyo / OA | `Organismos de Apoyo OA` |
| Parametrización | `Parametrización` |
| SSIS | `SSIS` |
| Utilidades HQ | `Utilidades HQ` |
| Histórico / conductor | `Histórico Vehicular - Conductor` |
| mintag, este repo, personal tooling | `Transversales` |

**Project ambiguity — ALWAYS ask before registering:**
- "RNC" without qualifier → ask: "`Trámites RNC` o `RNC Convalidación`?"
- "RNA" without qualifier → ask which RNA project
- No project mentioned → ask: "¿A qué proyecto corresponde esta actividad?"
- Multiple plausible matches → list 2-3 options and ask

### Step 3 — Choose category (exact string required)

| Category (exact) | Use when the activity involves… |
|---|---|
| `Actividades de arquitectura, diseño y código` | development, coding, implementation, technical design, architecture, code review, refactoring |
| `Gestión y análisis de requisitos` | requirements gathering, analysis, levantamiento, user stories, functional analysis |
| `Ejecución y gestión de pruebas` | running tests, writing test cases, QA testing, regression, test plan execution |
| `Gestión de despliegues y liberaciones` | deployments, releases, pases a producción, environment promotions, CI/CD |
| `Gestión y resolución de defectos` | fixing bugs, defects, root cause analysis, patch delivery |
| `Soporte a producción` | **ONLY** active incidents IN PRODUCTION being resolved in real time |
| `Soporte transversal y operación` | supporting another team (QA, ops, other squads) on technical matters |
| `Reuniones y sesiones de trabajo` | meetings, calls, standups, planning, retrospectives, workshops |
| `Gestión de oficios y comunicaciones` | sending emails, official documents (oficios), formal communications, forwarding docs |
| `Aseguramiento, seguridad y calidad` | security reviews, QA assurance (not test execution), IP validation, compliance |
| `Gestión de procesos de calidad` | SGC, ISO, audits, quality management processes |
| `Gestión administrativa` | administrative tasks, non-technical paperwork |
| `Gestión de indicadores` | metrics, dashboards, KPIs, indicator reports |
| `Gestión de acciones de mejoramiento` | improvement actions, PQRS follow-up, corrective actions |
| `Novedad: Incapacidad` | sick leave |
| `Novedad: Permiso` | personal leave |
| `Novedad: Vacaciones` | vacation |
| `Novedad: Licencias` | special license |
| `Novedad: Elecciones` | election day |
| `Novedad: Día de la familia` | family day |

### Step 4 — Anti-default rules (CRITICAL)

**NEVER** assign `Soporte a producción` when:

| Activity contains… | Correct category instead |
|---|---|
| correo, email, oficio, comunicación, envío de documentación | `Gestión de oficios y comunicaciones` |
| documentación, documento, informe, especificación, manual | `Actividades de arquitectura, diseño y código` or `Gestión y análisis de requisitos` |
| reunión, meeting, sesión, call, standup, videoconferencia | `Reuniones y sesiones de trabajo` |
| soporte a QA, soporte al equipo, ayuda a otro equipo | `Soporte transversal y operación` |
| desarrollo, implementación, código, codificación, programación | `Actividades de arquitectura, diseño y código` |
| análisis, requisito, requerimiento, levantamiento | `Gestión y análisis de requisitos` |
| pruebas, testing, QA, casos de prueba | `Ejecución y gestión de pruebas` |
| despliegue, liberación, deploy, pase | `Gestión de despliegues y liberaciones` |
| validación, aseguramiento, seguridad, IP | `Aseguramiento, seguridad y calidad` |

### Step 5 — Format registro_diario for Azure

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

### Step 6 — Confirm before registering

Show summary and wait for explicit confirmation:

```
Voy a registrar:
- Fecha:      {date}
- Horas:      {hours}
- Proyecto:   {project}
- Categoría:  {category}
- Comentario: {registro_diario}

¿Confirmás?
```

Accept: yes / sí / dale / ok / confirmar. If user corrects anything, update and show summary again.

### Step 7 — Call activity_log

Only after confirmation:

```
mcp__mintag__activity_log(
  date="{YYYY-MM-DD}",
  hours={float},
  project="{exact project string}",
  category="{exact category string}",
  registro_diario="{formatted comment}",
  source="llm_auto"
)
```

---

## Decision Examples

### Example 1 — explicit, project missing
**Prompt:** "registra 30 minutos de envio correo con documentacion de validacion de IP"
- hours: 0.5
- project: not mentioned → **ASK before anything else**
- category: correo + documentación + validación → `Gestión de oficios y comunicaciones`
- registro_diario: `Envío de correo con documentación de validación de IP`

### Example 2 — explicit, cross-team support
**Prompt:** "registra como actividad 1.5 hora de soporte sobre este tema a equipo de QA Proyecto RNCEA"
- hours: 1.5
- project: RNCEA → `RNCEA` ✓
- category: soporte + equipo QA (another team) → `Soporte transversal y operación`
  - Anti-default: NOT `Soporte a producción` (not a prod incident)
  - NOT `Ejecución y gestión de pruebas` (user supported QA, didn't run tests)
- registro_diario: "sobre este tema" is vague → ask: "¿Sobre qué tema fue el soporte al equipo de QA?"

### Example 3 — explicit, unambiguous
**Prompt:** "registra 2 horas de desarrollo del login en RNA Core"
- hours: 2.0
- project: RNA Core → `RNA Core` ✓
- category: desarrollo → `Actividades de arquitectura, diseño y código` ✓
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

## Output Contract

- **Passive**: one-line confirmation, e.g. `Logged 1.0h → RNCEA / Gestión y resolución de defectos`
- **Explicit**: confirmation summary → wait → log → one-line result
- **EOD**: formatted table + total + pending reminder
