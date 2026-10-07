# Feature: workitem-parent-bug

## Objective
Make catalogued Azure work items self-explanatory: always link each one to Azure DevOps, and for Tasks that are children of a Bug, show the parent Bug (id, title, link) in the "Work Items registrados" table and as a tooltip in the activities UI.

## Problem / Why
Bug child tasks are always titled "Atención y/o Corrección del defecto 171308", so the catalog does not tell which bug they belong to. The Azure link in the table only appears after "Refrescar estados" because it depends on `azureLinkBase` (team project from the states response), even though `azureWorkItemUrl()` builds a valid URL from the row's `org`.

## Scope (authorized)
- Azure link always visible in "Work Items registrados" (use `azureWorkItemUrl`).
- Persist parent info in the Azure activity catalog: `parent_work_item_id`, `parent_title`, `parent_type` (populated on states refresh from `System.LinkTypes.Hierarchy-Reverse`; cleared when the parent link disappears).
- Show the parent Bug (icon, `#id — title`, link) under the task label in "Work Items registrados".
- Show the parent Bug in the activities UI as a `title` tooltip wherever an Azure activity is referenced/selected.
- "Asignados en Azure sin catalogar": each row shows an Azure link and the parent Bug line; the assigned endpoint returns optional `parent_id`/`parent_title`/`parent_type` (same batch relations helper), and adding the row to the catalog persists the parent (optional add-body fields).
- Design note: the parent must come from Azure relations, not from the task title, so it keeps working if TLs (not CMMI) create the bug correction tasks in the future.

## Out of scope (follow-up, not authorized yet)
- Creating the "Atención y/o Corrección del defecto <id>" child Task with estimate from the portal for a given Bug.

## Constraints
- Dependency order `parser → store ← mcp, server`; `azure` imported only by `store` and `server`.
- Schema is inline idempotent `migrate()`; new columns must be added idempotently.
- Refresh stays best-effort: a parent lookup failure must not fail the states refresh.

## Tasks
- [ ] T1 — Backend: fetch parent (id/title/type) during states refresh, persist in catalog, expose in AzureActivity JSON and states response + tests. Route: delegated (writer, 2+ non-trivial files).
- [ ] T2 — Frontend: link always visible, parent Bug subline in WorkItemsView, tooltip in activities UI + tests. Route: delegated (same writer).

## Acceptance criteria
- Every row in "Work Items registrados" has an Azure link on load, without refreshing.
- After a states refresh, a Task under a Bug shows `Bug #<id> — <title>` with a link, and still shows it after reload.
- Hovering an Azure activity in the activities UI shows the parent bug when known.

## Checks
`go test ./...`, `go vet ./...`; in `frontend/`: `npm test`, `npx tsc -b`, `npm run lint` (compare to base 32 pre-existing errors).
