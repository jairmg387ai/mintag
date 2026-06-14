import { FileEdit, Send, CheckCircle, Rocket } from 'lucide-react'
import type { DWState } from '../../types'

interface DWStateBadgeProps {
  state: DWState
}

type StateMeta = { label: string; cls: string; icon: React.ElementType }

const STATE_META: Record<DWState, StateMeta> = {
  draft:     { label: 'Borrador',     cls: 'chip-todo',    icon: FileEdit },
  submitted: { label: 'En revisión',  cls: 'chip-test',    icon: Send },
  approved:  { label: 'Aprobada',     cls: 'chip-done',    icon: CheckCircle },
  deployed:  { label: 'Desplegada',   cls: 'chip-prog',    icon: Rocket },
}

export function DWStateBadge({ state }: DWStateBadgeProps) {
  const meta = STATE_META[state] ?? STATE_META.draft
  const Icon = meta.icon
  return (
    <span className={`chip ${meta.cls}`}>
      <Icon size={13} strokeWidth={1.75} />
      {meta.label}
    </span>
  )
}
