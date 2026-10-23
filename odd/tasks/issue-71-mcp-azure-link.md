# Issue 71: MCP Azure activity link
Branch: fix/issue-71-mcp-azure-link
Scope: one reviewable fix with tests and bundled skill documentation.

## Specs
S1. User: "corrige el iusse 71". Issue #71: "Add an optional `azure_activity_id` parameter to `activity_log` and `activity_update` in `internal/mcp/activities.go`, wired to `Store.SetActivityAzureActivity` (which already validates existence, active state, and the bug guard)."
S2. Issue #71: "Update `skills/activity-autolog/SKILL.md` and `internal/skillinstall/bundled/skills/activity-autolog/SKILL.md` to pass the catalog link instead of relying on `reference_id`."
S3. Issue #71: "Tests: MCP `activity_log` with `azure_activity_id` persists the FK; an invalid or inactive ID is rejected; `reference_id` behavior is unchanged."
S4. Issue #71: "Optionally accept `work_item_id` and resolve it to the catalog entry, since agents usually know the work item ID rather than the internal catalog ID." (Optional: not in this fix.)

## Tasks
T1 [S1-S3] DONE: implement/test the MCP link and synchronized skill instructions; route: bounded writer + parent atomicity correction; commit: 103666a.
T2 [S1-S3] DONE: independently verify risk and run focused/full checks; route: parent + high-risk verifier; commit: 103666a (same work unit as T1).

## Log
L1. corrige el iusse 71
L2. Issue #71: https://github.com/jairmg387ai/mintag/issues/71; one open bug. Existing REST link validates via store; MCP lacks the FK argument. Preserve free-text reference_id; do not implement optional work_item_id lookup. Ensure rejected input does not leave a created activity or partially update one.
L3. T1 RED and GREEN observed by worker with registered MCP handlers; parent replaced precheck/write gap with atomic linked store create/update. Full `go test ./...` and `git diff --check` passed.
L4. Independent verifier found explicit empty `azure_activity_id` silently followed the unlinked path. Parent reproduced RED for both MCP handlers and changed parsing to distinguish omitted from explicit empty/non-string values; focused/full tests, build, and independent S1 recheck passed.
L5. Work unit 103666ac4d668514059364d5ec2823e45294edd4 (326 authored changed lines) includes MCP implementation, registered-handler tests and mirrored skill guidance. Native review is separate from this functional verification.
