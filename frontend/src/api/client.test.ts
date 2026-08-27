import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { fetchBugEvidence, patchBugEvidence, addBugComment, BugEvidenceApiError } from './client'

// This file closes a coverage gap flagged by PR4's verify: BugEvidenceApiError's
// parsing/classification logic in requestBugEvidence (client.ts) had zero
// coverage — BugEvidencePanel.test.tsx fully mocks this module, hiding the
// real JSON-parse-and-classify branching. These tests exercise it directly
// against fetch-mocked responses shaped exactly like
// internal/server/bug_evidence.go's writeAPIError bodies.
describe('BugEvidenceApiError parsing (requestBugEvidence)', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  function mockErrorResponse(status: number, body: Record<string, unknown>) {
    vi.mocked(fetch).mockResolvedValue(
      new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } }),
    )
  }

  it('parses a 409 rev_conflict body into a BugEvidenceApiError with remote/changed_fields extras', async () => {
    mockErrorResponse(409, {
      code: 'rev_conflict',
      remote: { causa_raiz: 'from azure' },
      changed_fields: { causa_raiz: 'from azure' },
    })

    await expect(patchBugEvidence(170277, 5, { causa_raiz: 'mine' })).rejects.toMatchObject({
      name: 'BugEvidenceApiError',
      code: 'rev_conflict',
      extra: { remote: { causa_raiz: 'from azure' }, changed_fields: { causa_raiz: 'from azure' } },
    })
  })

  it('parses a 409 state_not_editable body into a BugEvidenceApiError with the state extra', async () => {
    mockErrorResponse(409, { code: 'state_not_editable', state: 'Cerrado' })

    await expect(patchBugEvidence(170277, 5, { causa_raiz: 'mine' })).rejects.toMatchObject({
      name: 'BugEvidenceApiError',
      code: 'state_not_editable',
      extra: { state: 'Cerrado' },
    })
  })

  it('parses a 422 root_cause_required body into a BugEvidenceApiError', async () => {
    mockErrorResponse(422, { code: 'root_cause_required' })

    await expect(patchBugEvidence(170277, 5, { causa_raiz_identificada: true })).rejects.toMatchObject({
      name: 'BugEvidenceApiError',
      code: 'root_cause_required',
    })
  })

  it('parses a 403 insufficient_scope body into a BugEvidenceApiError', async () => {
    mockErrorResponse(403, { code: 'insufficient_scope' })

    await expect(fetchBugEvidence(170277)).rejects.toMatchObject({
      name: 'BugEvidenceApiError',
      code: 'insufficient_scope',
    })
  })

  it('produces a real BugEvidenceApiError instance (instanceof check works for callers)', async () => {
    mockErrorResponse(409, { code: 'rev_conflict' })

    try {
      await patchBugEvidence(170277, 5, { causa_raiz: 'mine' })
      throw new Error('expected patchBugEvidence to reject')
    } catch (e) {
      expect(e).toBeInstanceOf(BugEvidenceApiError)
    }
  })

  it('falls back to a generic Error when the error body has no "code" field', async () => {
    mockErrorResponse(500, { message: 'internal error' })

    await expect(fetchBugEvidence(170277)).rejects.not.toBeInstanceOf(BugEvidenceApiError)
  })

  it('falls back to a generic Error when the error body is not JSON', async () => {
    vi.mocked(fetch).mockResolvedValue(new Response('not json', { status: 500 }))

    await expect(addBugComment(170277, 'key-1', 'hola')).rejects.not.toBeInstanceOf(BugEvidenceApiError)
  })
})
