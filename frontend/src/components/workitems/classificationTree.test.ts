import { describe, expect, it } from 'vitest'
import type { ClassificationNode } from '../../types'
import { flattenClassificationTree, filterClassificationPaths } from './classificationTree'

const tree: ClassificationNode = {
  name: 'RUNTPRO',
  children: [
    {
      name: 'RNET',
      children: [{ name: 'FUEC' }, { name: 'Backend' }],
    },
    { name: 'Sprint 1' },
  ],
}

describe('flattenClassificationTree', () => {
  it('produces backslash-joined full paths for every node, root included', () => {
    expect(flattenClassificationTree(tree)).toEqual([
      'RUNTPRO',
      'RUNTPRO\\RNET',
      'RUNTPRO\\RNET\\FUEC',
      'RUNTPRO\\RNET\\Backend',
      'RUNTPRO\\Sprint 1',
    ])
  })

  it('returns just the root path for a leaf node', () => {
    expect(flattenClassificationTree({ name: 'Solo' })).toEqual(['Solo'])
  })
})

describe('filterClassificationPaths', () => {
  const paths = flattenClassificationTree(tree)

  it('returns every path for a blank query', () => {
    expect(filterClassificationPaths(paths, '')).toEqual(paths)
    expect(filterClassificationPaths(paths, '   ')).toEqual(paths)
  })

  it('matches case-insensitively against the full path, not just the leaf segment', () => {
    expect(filterClassificationPaths(paths, 'rnet')).toEqual(['RUNTPRO\\RNET', 'RUNTPRO\\RNET\\FUEC', 'RUNTPRO\\RNET\\Backend'])
  })

  it('matches a leaf segment', () => {
    expect(filterClassificationPaths(paths, 'fuec')).toEqual(['RUNTPRO\\RNET\\FUEC'])
  })

  it('returns an empty list when nothing matches', () => {
    expect(filterClassificationPaths(paths, 'does-not-exist')).toEqual([])
  })
})
