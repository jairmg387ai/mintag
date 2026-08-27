import { useAppState } from '../../store/AppContext'
import { Modal } from '../modals/Modal'
import { BugEvidencePanel } from './BugEvidencePanel'

// BugEvidenceModal is the thin wrapper that plugs BugEvidencePanel into this
// project's existing global modal mechanism — the same
// activeXxxId + activeModal === 'xxx' convention MeetingDetailModal uses
// (see WorkItemsView's "Evidencia DSW-PR-017" action, which calls
// setActiveBugEvidenceId(id); openModal('bug-evidence')). No new ViewName,
// no new route — consistent with this project's no-router pattern.
export function BugEvidenceModal() {
  const { activeBugEvidenceId } = useAppState()

  if (activeBugEvidenceId == null) return null

  return (
    <Modal title={`Evidencia DSW-PR-017 — Bug #${activeBugEvidenceId}`} maxWidth={760}>
      <BugEvidencePanel bugId={activeBugEvidenceId} />
    </Modal>
  )
}
