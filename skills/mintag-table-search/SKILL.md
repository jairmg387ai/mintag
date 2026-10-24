---
name: mintag-table-search
description: "Trigger: buscar tabla, database table, qué repos usan la tabla X, which repos/files read or write table X, table usage, tabla Oracle, schema.table, JPA @Table, SQL FROM/JOIN/INSERT/UPDATE on a table. Use Mintag table_search before grepping repositories."
license: Apache-2.0
metadata:
  author: gentleman-programming
  version: "1.0"
---

## Activation Contract

Use this skill when the user asks where a database table is used: which repos, source files, or lines read or write it, or which tables a repo touches. The table inventory lives in the Mintag knowledge graph (imported from reviewed code evidence), so Mintag answers faster and more completely than searching repositories file by file.

## Hard Rules

- Call `table_search` first; only fall back to grepping code when Mintag is unavailable or returns no tables.
- Pass the table name the user gave, as-is or as a fragment (`PA_DEPARTAME`, `RUNTPROD.PA_DEPARTAME`, `RUNTPROD.PA_`). Matching is a case-insensitive substring on `SCHEMA.TABLE`.
- Never invent tables, repos, files, or lines — report only what `table_search` returns.
- When the result says `truncated`, tell the user and narrow with `repo`, a more specific `query`, or a higher `limit` / `max_files`.
- Evidence is static code analysis (`jpa_table`, `sql_from`, `sql_join`, `sql_into`, `sql_update`); it does not prove runtime usage. Say so when the user is deciding a migration or deletion.

## Decision Gates

| Situation | Action |
|---|---|
| "Which repos/files use table X?" | `table_search` with `query=X` |
| "Does repo R use table X?" | `table_search` with `query=X`, `repo=R` |
| Name is ambiguous (several tables match) | List the matching tables and ask which one, or answer for each |
| No tables found | Say the table is not in the imported inventory; offer a code search |
| Need what depends on a repo found here | Use the `mintag` skill (`graph_impact`) on that repo |

## Execution Steps

1. Call `table_search(query, namespace?, repo?, limit?, max_files?)`; the RUNT2 inventory lives in namespace `runt2`.
2. For each table returned, summarize repo count and file count, then list repos with their files and the evidence lines (`path:line` and type).
3. Mention truncation and how to narrow it.
4. Report in terms of the user's question, not raw JSON.
