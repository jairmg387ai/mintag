---
name: vtt-task-extractor
description: "Trigger: extraer tareas de una transcripcion, analizar un .vtt, generar reporte HTML de reunion, subir tareas a mintag. Extrae compromisos, resume decisiones, arma un informe HTML con Mermaid y persiste tareas en Mintag cuando esta disponible."
license: Apache-2.0
metadata:
  author: jair-muñóz
  version: "2.1"
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

## Decision Gates

| Situation | Action |
| --- | --- |
| Direct commitment appears (`voy a`, `me encargo`, `quedamos en`) | Create a task row |
| Process sequence appears (`primero`, `luego`, `despues`) | Draft a Mermaid flow |
| Decision or agreement appears | Include it in the executive summary |
| Mintag tools are available | Create or update the related meeting and tasks in Mintag |
| Transcript is noisy or incomplete | Prefer concise summary over speculative detail |

## Execution Steps

1. Clean the transcript and identify speakers, decisions, and commitments.
2. Draft a short executive summary with the main outcomes.
3. Extract tasks into rows with `Tarea`, `Responsable`, and `Contexto / Referencia`.
4. Build Mermaid flow code only if the meeting describes a real sequence or interaction.
5. Inject the summary, Mermaid code, metadata, and task rows into `template_informe_reunion.html`.
6. When Mintag is available, import or locate the meeting, create the extracted tasks in Mintag, and attach the generated summary as meeting rich content when appropriate.

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
