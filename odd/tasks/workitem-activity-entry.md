# Work item activity entry

## Objective
From an active, catalogued Task row in WorkItems, open the existing New Activity form with that Task selected, so a manual time entry is attributed to it without visiting Activities first.

## Scope and constraints
- Reuse NewActivityModal; keep the Activities page's unselected form unchanged.
- Date defaults to the local current day on opening; other inputs remain user-entered. Apply the same mapped project/category autofill as a manual work-item selection.
- Do not offer the shortcut for uncatalogued assigned items, Bugs, inactive catalog entries or editing rows. Refresh displayed effort after a successful save.
- Preserve pre-existing unrelated changes in frontend WorkItems files and internal/web/static asset; no build should overwrite generated assets.
- Route: delegated direct writer, because behavior and tests cross multiple non-trivial files. Forecast: approximately 100 authored changed lines, one cohesive work unit. Delivery strategy: ask-on-risk if forecast/running count exceeds ~400.

## Tasks
- [x] WIA-1 Add test-first support for preselected work item in NewActivityModal; preserve existing defaults. Check: targeted RED then GREEN, project/category mapping and creation payload tested.
- [x] WIA-2 Add Task-row activity shortcut and refresh effort on save. Check: targeted RED then GREEN for WorkItems, no action on non-Task/inactive rows, focused suites and typecheck; no generated asset changes. Commit 907ee15.
- [x] WIA-3 Reconcile concurrent worktree changes before candidate review; do not include another feature or overwrite its assets. Split into separate work units on branch `feat/activity-calendar-workitem-entry`.

## Progress and evidence
- Existing catalog supplies AzureActivity with catalog ID; NewActivityModal owns form state and accepts catalog/azureActivities; WorkItemsView already owns those inputs and effort reload key.
- Pre-existing WorkItemsView.tsx/test.tsx edits are unrelated to this feature and must remain intact.
- WIA-1: delegated writer observed RED for new preselection tests and GREEN for focused modal tests; mapped project/category and selected catalog ID are submitted. Reopen reset and unchanged default selection are covered. Follow-up removed an act warning; focused suites now 48/48 with no React warnings.
- WIA-2: implementation and targeted RED/GREEN observed, but final broader checks still pending. Focused typecheck `npx tsc --noEmit -p tsconfig.app.json` passed and `git diff --check` passed before follow-up.
- Focused ESLint fails with three pre-existing WorkItemsView effect violations (lines 122, 127, 133); no new lint findings observed. Preserve this baseline separately.
- Independent verifier: complete frontend suite 198/198 across 22 files, TypeScript `npx tsc --noEmit -p tsconfig.app.json`, and `git diff --check` passed before subsequent concurrent edits. Potential edge case: mapped category can remain blank if the activity catalog resolves after opening; user may fill it manually.
- Concurrent worktree mutation appeared after verification and before native review START: unrelated App, Activities calendar, store, server and generated assets changed, with new untracked files. START returned `consent-binding-stale`, `lineage_created:false`, `mutation_performed:false`; do not retry START on a mixed candidate. The current scope no longer matches our feature. Review not completed.
- 2026-10-15 reconciliation: the concurrent changes were the activity calendar feature (incl. `activities-calendar` menu option) and the pre-existing WorkItems parent-bug search. Split by hunk into one branch: 3ff9a09 `feat(activities)` calendar, 426d775 parent-bug search, 907ee15 this feature, 71ee409 rebuilt assets.
- Checks on the combined tree before committing: `go test ./internal/store/... ./internal/server/...` ok, `npx tsc --noEmit -p tsconfig.app.json` ok, `npx vitest run` 209/209. The parent-bug search commit was tested in isolation (WorkItems tests 55/55).
- Native review was not run on the separated candidate; delivered by user decision via PR.

## Next step
Pending PR review and merge. Pre-existing WorkItemsView ESLint baseline remains as a separate follow-up.
