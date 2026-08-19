import type { ActivityCatalog, AzureActivity } from '../../types'

// Pure decision function for the work-item-driven project/category autofill.
// No React, no I/O — every input is explicit so this is trivially unit
// testable (see activityAutofill.test.ts). Called only from an explicit
// Azure work-item selection handler, never from a modal's open-gated
// initialization effect.
export interface AutofillPatch {
  project?: string
  category?: string
}

export interface ResolveAutofillInput {
  // undefined when the selection resolves to "" or an unknown id.
  activity: AzureActivity | undefined
  catalog: ActivityCatalog | null
  projectTouched: boolean
  categoryTouched: boolean
}

export function resolveAutofill({
  activity,
  catalog,
  projectTouched,
  categoryTouched,
}: ResolveAutofillInput): AutofillPatch {
  const patch: AutofillPatch = {}

  // An untouched field always mirrors the *currently* selected work item —
  // filled when it maps to a valid value, blanked back to '' otherwise (no
  // selection, no mapping, an unknown/dangling reference, or a value the
  // loaded catalog no longer recognizes). Without this, switching from a
  // mapped work item to an unmapped one would leave the previous item's
  // stale autofill sitting in the field instead of clearing it. Once the
  // user hand-edits a field (touched), it's excluded from the patch
  // entirely and never touched again by this function, mapped or not.
  if (!projectTouched) {
    const project = activity?.project?.trim()
    // In select-mode (catalog !== null) only a value that has a matching
    // <option> may be written — the loaded catalog is the source of truth
    // for what's selectable. Free-text mode (catalog === null) has no such
    // constraint, so the raw value is written as-is.
    const valid = project && (catalog === null || catalog.projects.includes(project))
    patch.project = valid ? project : ''
  }

  if (!categoryTouched) {
    // Categories are hard-deleted, so a mapped category_id can dangle; a
    // null catalog (free-text mode) has no id -> name source at all. Both
    // resolve to blank rather than an unresolvable/invalid value.
    const category = activity?.category_id != null
      ? catalog?.categories.find(c => c.id === activity.category_id)?.name
      : undefined
    patch.category = category ?? ''
  }

  return patch
}
