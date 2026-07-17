---
name: vtt-task-extractor
description: "Trigger: extraer tareas de una transcripcion, analizar un .vtt, generar reporte HTML de reunion, subir tareas a mintag. Extrae compromisos, resume decisiones, arma un informe HTML con Mermaid y persiste tareas en Mintag cuando esta disponible."
license: Apache-2.0
metadata:
  author: gentleman-programming
  version: "2.2"
---

## Activation Contract

Use this skill when the user wants to extract action items, commitments, decisions, or process flows from a meeting transcript in `.vtt` or `.txt` format.

## Hard Rules

- Read the full transcript before extracting tasks.
- Preserve explicit ownership only when the speaker or assignee is clear.
- If ownership is ambiguous, write `Pendiente de confirmar` instead of inventing a responsible person.
- Use `template_informe_reunion.html` as the base when generating the final HTML report.
- Keep the Mermaid diagram empty or omit the flow section only when the conversation does not describe a process.
- When Mintag tools are available, persist the extracted tasks instead of leaving them only in the generated report.
- Before creating any task, run `task_search` for it within the project. `task_upsert`'s own dedup only catches an exact title match — a task mentioned with different wording than the one already in Mintag will not be found and will silently create a duplicate. If `task_search` surfaces a similar-but-not-identical task, ask the user whether to link/update it instead of creating a new one — do not call `task_upsert` to create until they answer. Skip this check only when the user has explicitly stated that everything discussed in the meeting is new work.

## Decision Gates

| Situation | Action |
| --- | --- |
| Direct commitment appears (`voy a`, `me encargo`, `quedamos en`) | Create a task row |
| Process sequence appears (`primero`, `luego`, `despues`) | Draft a Mermaid flow |
| Decision or agreement appears | Include it in the executive summary |
| Mintag tools are available | Pre-search candidates (meeting_search / task_search), then call meeting_find_or_create and task_upsert passing the best candidate_id as a hint (dedup-aware) |
| `task_search` returns an exact title match for an extracted task | Pass it as `candidate_id` to `task_upsert` — treat as the same task |
| `task_search` returns a similar-but-not-identical task (same intent, different wording) | Ask the user to confirm: link to that existing task, or confirm it's genuinely new — do NOT call `task_upsert` to create until they answer |
| User has explicitly stated everything discussed in the meeting is new work | Skip the per-task confirmation and create directly |
| Transcript is noisy or incomplete | Prefer concise summary over speculative detail |

## Execution Steps

1. Clean the transcript and identify speakers, decisions, and commitments.
2. Draft a short executive summary with the main outcomes.
3. Extract tasks into rows with `Tarea`, `Responsable`, and `Contexto / Referencia`.
4. Build Mermaid flow code only if the meeting describes a real sequence or interaction.
5. Inject the summary, Mermaid code, metadata, and task rows into `template_informe_reunion.html`.
6. When Mintag is available: (a) search existing meetings to find likely matches, then call meeting_find_or_create(filename, title, date, content, project_id, candidate_id?) — returns {action, id}; (b) for each extracted task call task_search(title, project_id) first — mandatory unless the user already said everything discussed is new work; (c) exact title match found -> pass it as candidate_id to task_upsert; (d) similar-but-not-identical match found -> stop and ask the user whether to link it or confirm it's new — do NOT call task_upsert to create until they answer; (e) no match -> call task_upsert(title, project_id, status, priority, owner, description, source_meeting_id, candidate_id?) directly — returns {action: created|updated|skipped|ambiguous}; (f) if task_upsert itself returns action: ambiguous, surface the candidate IDs to the user — do NOT create; (g) attach summary via meeting_set_rich_content when appropriate. Prefer these dedup tools over meeting_import/task_create.

## Output Contract

Return a single `.html` report that contains:
- Institutional header and meeting metadata.
- Executive summary of the discussion.
- Mermaid flow or sequence diagram when applicable.
- Task/commitment table extracted from the transcript.

When Mintag is available, also return or record the meeting/task creation result so the extracted work is persisted in the system of record.

## References

- `template_informe_reunion.html` - Base HTML template for the generated report.
- `README.md` - Mintag MCP tools available for meeting import, task creation, and rich content updates.
