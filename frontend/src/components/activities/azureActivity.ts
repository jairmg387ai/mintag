import type { AzureActivity } from '../../types'

// Shared by resolveAzureActivityLabel below and by the "Use default (...)"
// option in NewActivityModal/EditActivityModal, so there's one place that
// decides what counts as "the default" instead of each caller re-deriving
// it with its own .find(a => a.is_default).
export function findDefaultAzureActivity(azureActivities: AzureActivity[]): AzureActivity | undefined {
  return azureActivities.find(a => a.is_default)
}

// Resolves the display label for a daily activity's assigned Azure work
// item. A null/undefined azure_activity_id means the activity uploads to
// whichever azure_activities row is currently the default at upload time.
export function resolveAzureActivityLabel(
  azureActivityId: number | null | undefined,
  azureActivities: AzureActivity[],
): string {
  if (azureActivityId == null) {
    const def = findDefaultAzureActivity(azureActivities)
    return def ? `${def.label} (predeterminada)` : 'Predeterminada'
  }
  const match = azureActivities.find(a => a.id === azureActivityId)
  return match ? match.label : `#${azureActivityId}`
}

// Most azure_activities store-layer errors are already user-appropriate
// domain messages (e.g. "org is required", "azure activity 5 is inactive
// and cannot be assigned"). The one path that isn't is the rare race where a
// unique-constraint violation (concurrent inserts both trying to become the
// first/default row) surfaces the raw SQLite driver error. Catch that shape
// specifically instead of passing every backend error straight through to
// the UI, so a request that reaches the network fails with a message the
// user can act on either way.
export function friendlyCatalogErrorMessage(e: unknown, fallback: string): string {
  const message = e instanceof Error ? e.message : ''
  if (/sqlite|constraint|no such table|UNIQUE/i.test(message)) {
    return fallback
  }
  return message || fallback
}
