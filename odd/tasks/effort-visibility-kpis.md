# Feature: effort-visibility-kpis

## Objective
Show how many hours have been logged on each work item and how many remain, at the moment of logging time, and add time-log KPIs to the dashboard so weekly sprints can be tracked.

## Problem / Why
While logging time the user cannot tell whether a work item still has hours available. The backend already computes TimeLog hours per work item (`azure.TimeLogHours`) and syncs Completed/Remaining Work after upload, but nothing surfaces it in the UI. The dashboard only shows monthly totals and compliance; there is no breakdown by category/project or by week.

## Evidence
- `internal/azure/client.go:1011` `TimeLogHours`, `:934` `FetchTimeLogDocuments` (org-wide snapshot, only workItemId + minutes).
- `internal/azure/client.go:436` batch work-item fetch requests no OriginalEstimate.
- Activities link to work items via `daily_activities.azure_activity_id` -> `azure_activities.work_item_id`.
- `frontend/src/components/dashboard/Dashboard.tsx:79` client-side KPIs only for the month.

## Scope (authorized)
- Effort per work item: original estimate, hours uploaded to TimeLog, local hours not yet uploaded (pending + approved), remaining = max(0, estimate - uploaded - local).
- Show effort in WorkItems view and in the new-activity modal, warning when the entry would exceed the remaining hours.
- Dashboard: period selector (current week / current month), hours by category and by project.
- Dashboard extra KPIs: approved-but-not-uploaded hours, business days below the daily target, active work items with no hours this week, work items with >= 80% of estimate consumed.

## Out of scope
- Writing to Azure (all new calls are read-only GETs).
- Holiday calendar, sprint/iteration date fetching from Azure.

## Delivery
- Strategy: `ask-on-risk` (default). Forecast ~900 authored lines > 400: chain strategy to be asked before opening any PR.

## Tasks
- [x] T1 — Backend: read-only effort endpoint `GET /api/activities/azure-work-items/effort?ids=` (estimate batch fetch + TimeLog snapshot + store local unuploaded hours per work item) + tests. Route: delegated writer (3+ non-trivial files).
- [ ] T2 — Frontend: effort display in WorkItemsView and NewActivityModal with over-estimate warning + tests. Route: delegated writer.
- [ ] T3 — Frontend: Dashboard period selector + hours by category/project + tests. Route: delegated writer.
- [ ] T4 — Frontend: Dashboard extra KPIs (pending upload, gap days, work items without hours this week, >=80% consumed) + tests. Route: delegated writer.

## Checks
- `go vet ./...`, `go test ./...`
- `frontend/`: `npm test`, `npx tsc -b`, `npm run lint` (baseline 32 errors, must not grow)

## Progress / Evidence
- Branch `feat/effort-visibility-kpis` created from master 48874e7.
- RDD: on (global). Per-commit `gentle-ai review assess` recorded per task.

- T1 commit `b72dd23` feat(activities): add read-only work item effort endpoint. RED: build failures (FetchWorkItemEstimates/LocalUnuploadedHoursByWorkItem undefined) + 404 on new route; GREEN: go vet ok, go test azure/store/server ok (parent spot check ok).
- T1 review: assess risk=medium, review_due=true (slice_budget_reached). Preflight STATUS blocked at `intended_untracked_selection_required`; every submitted selection JSON refused with `invalid_request` (schema not exact). Review pending, user decision needed.

## Next step
Resolve T1 review block, then T2.
