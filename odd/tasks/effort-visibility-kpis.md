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

## Decision (2026-10-07, user)
User always logs time from Mintag and is the only person logging on their tasks (others get their own task). Older Mintag versions did not maintain Completed/Remaining Work in Azure, so Azure effort fields are unreliable. => Effort is computed from the Mintag DB: estimate persisted in `azure_activities.original_estimate`, used = all local activities for the work item, remaining = estimate - used. Azure is only read on explicit refresh/sync to update the estimate. T1b (TimeLog per-user filter) was discarded uncommitted.

## Tasks
- [x] T1 — Backend: read-only effort endpoint `GET /api/activities/azure-work-items/effort?ids=` (estimate batch fetch + TimeLog snapshot + store local unuploaded hours per work item) + tests. Route: delegated writer (3+ non-trivial files).
- [x] T1c — Backend: persist `original_estimate` on azure_activities (migration; set on Mintag work-item creation and on states refresh); effort endpoint reads DB only (no Azure, no TimeLog) + tests. Also resolves review findings R3-batch-404 and R3-modal-ignores-timelog-error. Route: delegated writer.
- [x] T2 — Frontend: effort display in WorkItemsView and NewActivityModal with over-estimate warning + tests. Route: delegated writer.
- [x] T3 — Frontend: Dashboard period selector + hours by category/project + tests. Route: delegated writer.
- [x] T4 — Frontend: Dashboard extra KPIs (pending upload, gap days, work items without hours this week, >=80% consumed) + tests. Route: delegated writer.

- [x] T5 — Review follow-ups (dashboard slice): idle alert must group by work_item_id (sibling catalog rows); "Pendiente de subir" must count all approved activities (not only loaded range); skip effort call when no open ids; dashboard "today" refresh across midnight (cheap fix only); test bug-correction effective estimate persistence. Route: delegated writer.

## Checks
- `go vet ./...`, `go test ./...`
- `frontend/`: `npm test`, `npx tsc -b`, `npm run lint` (baseline 32 errors, must not grow)

## Progress / Evidence
- Branch `feat/effort-visibility-kpis` created from master 48874e7.
- RDD: on (global). Per-commit `gentle-ai review assess` recorded per task.

- T1 commit `b72dd23` feat(activities): add read-only work item effort endpoint. RED: build failures (FetchWorkItemEstimates/LocalUnuploadedHoursByWorkItem undefined) + 404 on new route; GREEN: go vet ok, go test azure/store/server ok (parent spot check ok).
- T1 review: assess risk=medium, review_due=true (slice_budget_reached). Preflight STATUS blocked at `intended_untracked_selection_required`; every submitted selection JSON refused with `invalid_request` (schema not exact). Review pending, user decision needed.

- T1 review declined by user (candidate sha256:6b2db294...).
- T2 commit `4d2b701` feat(workitems): show logged and remaining hours per work item. RED 7 failing tests; GREEN npm test 164 passed, tsc ok, lint 32 errors (baseline).
- Review of T1+T2 (lineage review-4c5a8e14fc1bbf04): granted, approved, acknowledged. Advisory follow-ups: R3-batch-404-fails-page, R3-modal-ignores-timelog-error, R3-weak-failure-test (NewActivityModal.test.tsx:336-342).

- T1c: effort computed from DB only (`azure_activities.original_estimate`, filled on create/recreate/bug-correction and on states refresh). RED: compile errors (OriginalEstimate / SetAzureActivityEstimate undefined); GREEN: go vet ok, go test ./... ok, npm test 163 passed, tsc ok, lint 32 (baseline). Response shape now `{items}`. Bug-correction estimate persistence has no dedicated test.

- T1c review (lineage review-ea9cd3338da5b9b5): granted, approved, acknowledged. Follow-ups: R3-bugfix-estimate-not-effective fixed (bug correction caches azure.EffectiveOriginalEstimate); R3-no-estimate-backfill accepted: existing catalog rows show "Sin estimado" until the user runs "Refrescar estados" once (communicated to user).

- T3+T4 (one delegated writer, one commit since both live in Dashboard): period selector week/month (localStorage), HoursBreakdown by category/project, Pendiente de subir, Dias por debajo de la meta, WorkItemAlerts (no hours this week, >=80% consumed). RED: missing modules + 2 failing Dashboard tests; GREEN: dashboard 26 tests, npm test 189 passed, tsc ok, lint 32 (baseline). Static assets rebuilt with npm run build.

- Split commit: `bf05302` feat(dashboard) source + `4d2953f` chore(web) rebuilt assets (first combined candidate hit lens_context_budget_exceeded because of the bundle).
- Review baa769a..bf05302 (lineage review-2ebb92fb42f512b0): granted, approved, acknowledged. Warnings -> T5.

- T5: idle grouped by work_item_id, Pendiente de subir over all dates (2000-01-01..today, status approved/pending), no effort call without open ids, todayStr refresh (60s + focus), Go test for bug-correction effective estimate. RED 5/30 dashboard tests failing; GREEN dashboard 30, npm test 193, tsc ok, lint 32, go test server ok. Go test passed first run (covers existing behavior).

- T5 commit `e6b9854`; review (lineage review-3509a1ca9963f233) granted, approved, acknowledged. Fixed R3-stale-backlog-race (cancellation guards). Accepted: R3-backlog-failure-untested, R3-unbounded-backlog-query (approved backlog stays small).

## Next step
Rebuild assets, then decide PR strategy (forecast exceeded 400 lines).
