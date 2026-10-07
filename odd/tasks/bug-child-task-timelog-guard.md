# Feature: bug-child-task-timelog-guard

## Objective
Prevent logging/uploading TimeLog hours directly against an Azure DevOps **Bug**. Hours for a bug must go to one of its child Tasks (`System.LinkTypes.Hierarchy-Forward`) that is assigned to the current user. Tasks without a Bug parent keep working as today.

## Problem / Why
The team's new process creates specific child Tasks under each Bug (e.g. Bug 171191 → Tasks 171306 "Atención y/o Corrección del defecto", 171323 "Verificación y validación del defecto"). Mintag currently resolves an activity to any catalogued work item (`resolveAzureWorkItemID`, `internal/store/upload.go:100`) without checking its type or hierarchy, so hours can land on the Bug itself.

## Scope (authorized)
- Configurable guard stored in the `setting` table, **enabled by default**, toggleable from REST, MCP and the UI.
- Guard ON:
  - Work item type `Bug` → reject; message lists the bug's child Tasks assigned to the current user.
  - Task whose parent (`Hierarchy-Reverse`) is a `Bug` → allowed only if assigned to the current user.
  - Task without a Bug parent → allowed (unchanged).
- Guard OFF → current behavior.
- Fail-fast at registration time using the cached catalog `work_item_type`; authoritative check at upload time using live Azure data.

## Constraints
- Dependency order `parser → store ← mcp, server`; `azure` imported only by `store` and `server`.
- MCP errors as `{"error":"..."}` text; use `req.GetString`/`req.GetBool`.
- Upload partial-failure semantics: failed rows stay `approved` for retry.

## Tasks
- [x] T1 — Store: guard setting (get/set, default ON) + tests. Evidence: `go test ./internal/store/ -run TimeLogBugGuard` RED (undefined symbols) → GREEN (2 PASS). Key `activity.validation.block_bug_work_item`.
- [x] T2 — Azure client: fetch work item hierarchy (type, assignee id, parent id/type, child tasks with assignee) via `$expand=relations` + tests. Evidence: `go test ./internal/azure/` RED (undefined WorkItemHierarchy) → GREEN (ok). Adds `FetchWorkItemHierarchy`, `CheckTimeLogTarget`, pure `EvaluateTimeLogTarget`, `*TimeLogTargetError`.
- [x] T3 — Upload guard: enforce rules per activity in `UploadActivities`; failed row with actionable message + tests. Evidence: `go test ./...` all ok, `go vet ./...` clean.
- [ ] T4 — Registration fail-fast: reject activity create/update pointing at a catalogued `Bug` when guard ON (store/server/MCP) + tests.
- [ ] T5 — Config surface: REST GET/PUT, MCP tool, UI toggle; update `activity-autolog` skill to resolve bug → assigned child task.

## Acceptance criteria
- Guard ON by default on a fresh DB.
- Uploading an activity resolved to a Bug fails that row with a message naming child Tasks assigned to me.
- Uploading to a bug child Task assigned to someone else fails; assigned to me succeeds.
- Uploading to a standalone Task succeeds.
- Guard OFF restores previous behavior.

## Checks
`go test ./...`, `go vet ./...`, `npm run lint` + `npm run build` in `frontend/` (T5).

## Delivery
Strategy: ask-on-risk. Forecast ~600–800 authored lines (exceeds 400 → chain strategy decision before crossing budget).

## Progress / Evidence
- Branch `feat/bug-child-task-timelog-guard` created from `master`.
- T1 committed 2ac8fd2; T2 committed 33aa35e.
- T3 committed bfcd467: guard enforced in `UploadActivities` (per-upload cache, fail-closed on Azure read errors — accepted). RED observed first (guard tests saw 0 work item reads). Server route-test fakes now answer work item GETs with a standalone Task. `go vet ./...` exit 0; `go test ./...` all ok.
- Decisions: T5 adds the guard as a 4th field of `ActivityValidationSettings`; fail-closed accepted.

## Route log
- T1–T4: delegated writer (2+ non-trivial files, preparation reading).
