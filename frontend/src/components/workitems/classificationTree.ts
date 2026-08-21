import type { ClassificationNode } from '../../types'

// flattenClassificationTree turns an Area/Iteration classification node tree
// into the list of full backslash-joined paths a picker can search over
// (e.g. "RUNTPRO\RNET\FUEC"), root included. Ported from the coworker
// reference app's flattenPaths, kept here as a pure/testable helper instead
// of inline DOM-building so ClassificationTreePicker only has to render it.
export function flattenClassificationTree(node: ClassificationNode, prefix = ''): string[] {
  const path = prefix ? `${prefix}\\${node.name}` : node.name
  const out = [path]
  for (const child of node.children ?? []) {
    out.push(...flattenClassificationTree(child, path))
  }
  return out
}

// filterClassificationPaths keeps paths whose full string contains query
// (case-insensitive), so searching "RNET" also surfaces its descendants —
// matching AzureActivityCombobox's filter semantics in azureActivity.ts.
export function filterClassificationPaths(paths: string[], query: string): string[] {
  const q = query.trim().toLowerCase()
  if (!q) return paths
  return paths.filter(p => p.toLowerCase().includes(q))
}
