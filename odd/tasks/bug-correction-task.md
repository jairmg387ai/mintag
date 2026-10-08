# Feature: bug-correction-task

## Objective
Let a TL create, from the portal, the bug correction child Task "Atención y/o Corrección del defecto <bugId>" for a Bug, through a form, with its estimate.

## Problem / Why
Today CMMI creates the bug child tasks. The process may move to TLs. The existing "Crear work item" flow (`CreateWorkItem`, `internal/azure/client.go:563`) creates no parent link, no TaskType, and no Subárea, always assigns to the caller and uses the configured TeamProject (default `RUNTPRO`), while bugs live in the bug's own team project (e.g. `ControlesDeCambio`).

## Evidence (read-only inspection of Azure, 2026-10-07)
Correction task 171306 (parent Bug 171191), created by CMMI:
- `System.Title` = "Atención y/o Corrección del defecto <n>" (CMMI wrongly puts the task's own id; we always use the BUG id)
- `Custom.860becab-e144-442b-8283-c2dd65cc7191` ("Subárea", picklist, not required) = "Desarrollo". Allowed values (Task type, ControlesDeCambio): Administración de la Configuración, Arquitectura, Desarrollo, Desarrollo PL, Líderes Funcionales, Proyectos, QA, Requisitos — load dynamically via `_apis/wit/workitemtypes/Task/fields/{ref}?$expand=allowedValues`.
- `Microsoft.VSTS.CMMI.TaskType` = Planned, `Microsoft.VSTS.Common.Priority` = 2
- `System.State` = Proposed, `System.Reason` = New
- `OriginalEstimate` = `RemainingWork` = estimate (6h)
- `System.AreaPath` = the bug's AreaPath; `System.IterationPath` = current sprint (e.g. `ControlesDeCambio\Sprint 7 Semana 41`)
- `System.TeamProject` = the bug's team project
- `System.AssignedTo` = the developer (the bug's assignee in observed cases)
- Relation `System.LinkTypes.Hierarchy-Reverse` → the bug

## Scope (authorized)
- Only the correction task (not the QA "Verificación y validación" task).
- A form (modal) where the user selects/enters: Subárea (required, chosen by the user, no silent default), estimate (required), iteration (required), area (prefilled from bug), assignee (prefilled from bug's assignee), title (prefilled with bug id, editable), optional description.
- Entry points: Bug rows in "Work Items registrados" and in "Asignados en Azure sin catalogar".
- Created in the bug's team project, linked to the bug as parent.
- Optionally add the created task to the catalog with its parent info.

## Out of scope
- QA verification task. Auto-creation without a form.

## Tasks
- [x] T1 — Backend: bug prefill endpoint (bug title/area/team project/assignee/iteration), Subárea allowed-values endpoint, create-correction-task endpoint (parent link + fields above) + tests (httptest; never hit real Azure). Route: delegated writer.
- [x] T2 — Frontend: correction-task form modal + entry points + tests. Route: delegated writer.
- [x] T3 — Frontend follow-up: `ClassificationTreePicker` `teamProject` prop, tree pickers for Iteración/Área in the correction form, and fix R3-draft-effect-refetch-clobbers-edits. Route: delegated writer.
- [x] T4 — Review fixes R3-children-read-blocks-create + R3-missing-server-failure-path-tests: best-effort duplicate lookup with `existing_tasks_error`, create independent of the children read, server failure-path tests, UI warning. Route: delegated writer (scope addition authorized by the user).

## Checks
`go vet ./...`, `go test ./...`; in `frontend/`: `npm test`, `npx tsc -b`, `npm run lint` (baseline 32 errors).

## Progress / Evidence
- T1 — commit `9317d57` feat(azure): create bug correction task with parent link and subarea. Route: delegated writer.
  - New `internal/azure/bug_correction_task.go`: `FieldSubarea` constant, `FetchBugCorrectionTaskDraft` (bug + child correction Tasks via $expand=relations), `FetchTaskFieldAllowedValues`, `CreateBugCorrectionTask` (Proposed/New, Planned, Priority 2, Original=Remaining estimate, Subárea, `Hierarchy-Reverse` relation to the bug, created in the bug's team project, not activated).
  - `CreateWorkItem` now shares `postNewWorkItem(project, type, ops)`; existing behavior/tests unchanged. `FetchClassificationTreeForProject` added; `GET /api/activities/azure-classification-nodes/{kind}?team_project=X` (default unchanged).
  - Routes (local-only): `GET /api/azure/bugs/{id}/correction-task-draft`, `POST /api/azure/bugs/{id}/correction-task`, `GET /api/azure/work-item-fields/subarea/allowed-values?team_project=X`. Errors: `not_a_bug`, `validation_error` (+`field`), catalog failure non-fatal as `catalog_error`; catalog add stores parent bug via `SyncAzureActivityParent`.
  - RED observed (build failure on missing symbols; 404 on missing routes) → GREEN. `go vet ./...` clean; `go test ./...` all ok. All tests use httptest fakes; real Azure never called.
- T2 — commit `747cc41` feat(web): add bug correction task form. Route: delegated writer.
  - `CreateBugCorrectionTaskModal` (draft prefill, duplicate warning, Subárea with no preselection, required estimate/iteration/assignee gating, catalog checkbox default by assignee == connected identity, project/category when cataloguing, created id + Azure link). Entry points: Bug rows in "Work Items registrados" and Bug items in "Asignados en Azure sin catalogar" (`ParentWorkItemLine` skipped: child rows are usually the correction task already).
  - RED observed for the WorkItemsView entry-point tests before wiring; modal tests were written before the component but passed on its first run (no separate RED run).
  - `npm test` 16 files / 139 tests pass; `npx tsc -b` clean; `npm run lint` 32 errors (= baseline, none in touched files).
  - Deviation: Iteración/Área use a `<select>` of flattened tree paths for the bug's team project instead of `ClassificationTreePicker`, because that picker has no team-project prop and `ClassificationTreePicker.tsx` was outside the authorized edit surface. Follow-up: add an optional `teamProject` prop to it and reuse it here.
- T3 — commit `9cf8b22` feat(web): tree pickers and stable draft in bug correction form. Route: delegated writer.
  - `ClassificationTreePicker` gains optional `teamProject`; loads `fetchClassificationTree(kind, teamProject)` (single-arg call when omitted, so `CreateWorkItemModal` is unchanged), reloads on change, clears its load error on success.
  - `CreateBugCorrectionTaskModal`: Iteración (required) and Área (prefilled with the bug's area) now use `ClassificationTreePicker` with `teamProject={draft.team_project}`; the in-modal `usePathOptions` tree loading was removed.
  - R3 fix: the draft/subárea effect depends only on `[open, bugId]`; `loadError` is cleared on a successful load; "Agregar al catálogo" is derived (`userChoice ?? assignee == identity`) so a late `currentUserDisplayName` applies the default without clobbering an explicit toggle, and title/area/assignee edits are never refetched over. No synchronous setState in effects.
  - Tests: new `ClassificationTreePicker.test.tsx` (default vs teamProject call, reload on change); modal tests for pickers, custom title submitted, identity change keeps edits (single draft/subárea fetch), late identity default, load error cleared; `client.test.ts` covers the `team_project` query string.
  - RED observed: 10 failing (picker reload + modal tests) before implementation; the client query-string tests passed immediately because `fetchClassificationTree` already accepted `teamProject` from T1. GREEN: `npm test` 17 files / 148 tests pass; `npx tsc -b` clean; `npm run lint` 32 errors (= baseline, none in touched files).
- T4 — commit `efbeefb` fix(azure): make duplicate check best-effort in bug correction task. Route: delegated writer.
  - `internal/azure`: new `FetchBugForCorrectionTask` (bug read only, no children); `FetchBugCorrectionTaskDraft` builds on it and treats the children batch read as best-effort, setting `ExistingTasksError` instead of failing.
  - `internal/server`: draft returns 200 with bug data, empty `existing_correction_tasks` and `existing_tasks_error` (sanitized) on a children failure; POST correction-task now uses `FetchBugForCorrectionTask`, so it never reads the children.
  - Frontend: `existing_tasks_error?` on `BugCorrectionTaskDraft`; the modal shows "No se pudo verificar si ya existe una tarea de corrección" and still allows creating.
  - Tests: azure (children failure is best-effort; bug-only read skips children); server (bug read 500→502 / 404→404 on draft and create, children failure on draft → 200 + warning, on create → created with zero children calls, Azure create failure → 502 with no catalog entry, Subárea failure → 502; the existing catalog-failure test covers `catalog_error`); modal (warning shown / absent).
  - RED observed: Go build failure on the missing symbols; server draft and create returned 502 on a children failure; the modal warning test failed. GREEN: `go vet ./...` clean; `go test ./...` all ok; `npm test` 17 files / 150 tests pass; `npx tsc -b` clean; `npm run lint` 32 errors (= baseline, none in touched files).

## Next step
Native review / PR decision by the user; optional follow-up for `ClassificationTreePicker` team-project support.
