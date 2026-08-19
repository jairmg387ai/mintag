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
  if (activity == null) return {}

  const patch: AutofillPatch = {}

  if (!projectTouched) {
    const project = activity.project?.trim()
    if (project) {
      // In select-mode (catalog !== null) only a value that has a matching
      // <option> may be written — the loaded catalog is the source of truth
      // for what's selectable. Free-text mode (catalog === null) has no
      // such constraint, so the raw value is written as-is.
      if (catalog === null || catalog.projects.includes(project)) {
        patch.project = project
      }
    }
  }

  if (!categoryTouched && activity.category_id != null) {
    // Categories are hard-deleted, so a mapped category_id can dangle; a
    // null catalog (free-text mode) has no id -> name source at all. Both
    // cases leave the category field untouched rather than write an
    // unresolvable/invalid value.
    const category = catalog?.categories.find(c => c.id === activity.category_id)?.name
    if (category) {
      patch.category = category
    }
  }

  return patch
}
