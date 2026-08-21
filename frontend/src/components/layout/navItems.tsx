import { LayoutDashboard, CheckSquare, Users, Network, Clock, Rocket, FilePlus2 } from 'lucide-react'
import type { ViewName } from '../../types'

export interface NavEntry {
  id: ViewName
  label: string
  icon: React.ReactNode
}

// Shared between Sidebar (renders the filtered nav) and the Settings
// menu-options toggle list (needs the same Spanish labels) — the frontend
// keeps its own labels here and matches server-side MenuOptionStatus
// entries purely by id. Kept in its own module (not Sidebar.tsx) so
// react-refresh/only-export-components doesn't flag a component file for
// exporting a plain constant.
export const NAV_ITEMS: NavEntry[] = [
  { id: 'dashboard', label: 'Panel',           icon: <LayoutDashboard size={18} strokeWidth={1.75} /> },
  { id: 'tasks',     label: 'Tareas',          icon: <CheckSquare    size={18} strokeWidth={1.75} /> },
  { id: 'meetings',  label: 'Reuniones',       icon: <Users          size={18} strokeWidth={1.75} /> },
  { id: 'graph',      label: 'Grafo de Conocimiento', icon: <Network size={18} strokeWidth={1.75} /> },
  { id: 'activities',          label: 'Actividades',   icon: <Clock   size={18} strokeWidth={1.75} /> },
  { id: 'deployment-windows', label: 'Ventanas',     icon: <Rocket  size={18} strokeWidth={1.75} /> },
  { id: 'work-items', label: 'Work Items', icon: <FilePlus2 size={18} strokeWidth={1.75} /> },
]
