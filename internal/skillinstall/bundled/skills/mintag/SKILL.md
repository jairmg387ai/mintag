---
name: mintag
description: "Trigger: Mintag, preguntar a Mintag, caso de uso, CU, use case, qué repos implementan, portal, opción de menú, microfront, team project, dependencias entre repos, qué se rompe si cambio X, tabla, reuniones, tareas, actividades/TimeLog, ventana de despliegue. Entry point that routes any Mintag question to the right MCP tools."
license: Apache-2.0
metadata:
  author: gentleman-programming
  version: "1.0"
---

## Activation Contract

Use this skill whenever a question can be answered from Mintag: RUNT2 architecture (use cases, repos, portals, menu options, microfronts, team projects, repo-to-repo calls), database table usage, meetings and tasks, daily activities, or deployment windows. Ask Mintag before grepping repositories or guessing from memory.

## Hard Rules

- Never invent node keys. Resolve them with `graph_search` (or `table_search` for tables) and reuse the exact `kind` and `key` returned.
- The RUNT2 inventory lives in namespace `runt2`; pass it explicitly when several namespaces exist.
- Edge direction is SOURCE → TARGET. `direction=out` follows edges leaving the node; `direction=in` follows edges arriving at it.
- Inventory edges come from static code analysis. Say so when the user is deciding a migration, deletion, or deployment.
- Report answers in terms of the user's question, not raw JSON. Say when a result was truncated and how to narrow it.

## RUNT2 Graph Model

| Edge | Meaning |
|---|---|
| `use_case -implemented_in-> repo` | Repos with code evidence for the use case |
| `source_file -implements-> use_case` | Files that reference the use case code |
| `source_file -in_repo-> repo` | File ownership |
| `source_file -uses_table-> table` | Files reading/writing a table (use `table_search`) |
| `repo -belongs_to-> team_project` | Team project of a repo |
| `repo -calls-> repo / messaging / external_service` | Runtime calls between services |
| `repo -uses_client-> repo / api_client` | API client libraries used |
| `repo -consumes-> repo`, `microfront -consumes-> repo` | Backend consumed by a front |
| `repo -has_app-> microfront` | Microfront apps in an MR repo |
| `portal -exposes-> repo`, `portal -has_shell-> repo` | Remotes and shell of a portal |
| `menu_option -belongs_to-> portal`, `menu_option -implemented_by-> repo` | Portal menu and the repo behind it |

Use case keys are codes such as `CU0123`; menu option keys are `<PORTAL>:<clave>`.

## Decision Gates

| Question | Tools |
|---|---|
| Which repos implement use case CU#### | `graph_search` (kind=use_case) → `graph_neighbors` relation=implemented_in direction=out |
| Which files implement CU#### | `graph_neighbors` on the use case, relation=implements direction=in |
| Which use cases does repo R implement | `graph_neighbors` on the repo, relation=implemented_in direction=in |
| Which tables does use case CU#### touch | Get its files (relation=implements, in), then `table_search` per candidate table or by `repo`; this is an inference from shared files, not a direct edge — say so |
| Which repos/files use table X | `table_search` (see the `mintag-table-search` skill) |
| What a portal exposes / its menu | `graph_neighbors` on the portal: relation=exposes direction=out; relation=belongs_to direction=in for menu options |
| What X depends on | `graph_node` on X, read the `out` relations |
| What breaks if X changes | `graph_impact` on X (use `kind_filter`, e.g. portal or use_case) |
| Unknown graph contents | `graph_stats` |
| Meetings and tasks | `meeting_search`, `task_search`, `tasks_by_project`, `task_history` (transcripts: `vtt-task-extractor` skill) |
| Logging work time | `activity_log` / `activity_list` (see the `activity-autolog` skill) |
| Deployment windows | `dw_list`, `dw_get`, `dw_export_markdown` |

## Execution Steps

1. Classify the question with the Decision Gates table.
2. Resolve exact keys (`graph_search`, `table_search`); if several nodes match, list them and ask which one, or answer for each.
3. Call the narrowest tool that answers it (`graph_neighbors` before `graph_node`, `graph_impact` only for blast radius).
4. Combine results across tools when the question spans areas (use case → files → tables), stating which links are inferred.
5. Answer in the user's language with counts, the key items, and any truncation.
