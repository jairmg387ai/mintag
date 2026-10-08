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
- [x] T1 — Backend: fetch parent (id/title/type) during states refresh, persist in catalog, expose in AzureActivity JSON and states response + tests. Route: delegated (writer, 2+ non-trivial files).
  - Commit `a4975ac` feat(azure): persist parent work item in activity catalog. Also covers the assigned list (`parent_id`/`parent_title`/`parent_type`) and optional parent fields on catalog add.
  - Approach: `AttachWorkItemParents` (internal/azure/work_items_parent.go) = one batched `$expand=relations&errorPolicy=omit` read + one batched read of distinct parent ids. On lookup failure the states refresh still succeeds and stored parents are left untouched; a successful lookup with no parent link clears them.
  - Evidence: RED = compile failures in new azure/store/server tests; GREEN = `go vet ./...` clean, `go test -count=1 ./...` all ok.
- [x] T2 — Frontend: link always visible, parent Bug subline in WorkItemsView, tooltip in activities UI + tests. Route: delegated (same writer).
  - Commit `ef46437` feat(web): show azure link and parent bug for work items. Includes the "Asignados en Azure sin catalogar" rows (link + parent line; add persists parent). ActivitiesView/ActivityDetailModal get the tooltip through `AzureActivityReference` (no direct edits needed).
  - Evidence: RED = 9 new vitest tests failing; GREEN = `npm test` 130/130 passed, `npx tsc -b` clean, `npm run lint` 32 errors / 2 warnings (unchanged from base; the 3 in WorkItemsView.tsx are pre-existing `set-state-in-effect`).

## Acceptance criteria
- Every row in "Work Items registrados" has an Azure link on load, without refreshing.
- After a states refresh, a Task under a Bug shows `Bug #<id> — <title>` with a link, and still shows it after reload.
- Hovering an Azure activity in the activities UI shows the parent bug when known.

## Checks
`go test ./...`, `go vet ./...`; in `frontend/`: `npm test`, `npx tsc -b`, `npm run lint` (compare to base 32 pre-existing errors).

## T3 (user request, affb444)
- [x] T3 — Bug child Task rows (catalogued and "Asignados en Azure sin catalogar") show an "Evidencia" action on the parent Bug line that opens the DSW-PR-017 evidence panel (tracking, comments, root cause) for the parent bug id. Why: hours now go to the correction Task, so the Bug itself is no longer catalogued and its evidence action was unreachable. The evidence endpoint only needs the bug id (no catalog dependency). Route: inline (one file + test). Evidence: RED (1 failing test) → GREEN; `npm test` 132/132, `tsc -b` ok, eslint on touched file: 3 pre-existing set-state-in-effect errors only.
