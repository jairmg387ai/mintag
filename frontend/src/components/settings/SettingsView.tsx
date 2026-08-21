import type { ComponentType } from 'react'
import { Card, CardHeader } from '../ui/Card'
import { MenuOptionsSection } from './MenuOptionsSection'
import { CatalogRetentionSection } from './CatalogRetentionSection'

interface SettingsSection {
  id: string
  label: string
  component: ComponentType
}

// SETTINGS_SECTIONS is the extension point for future settings — e.g. moving
// the existing Azure TimeLog config here later is one new entry, not a
// rewrite of this view.
const SETTINGS_SECTIONS: SettingsSection[] = [
  { id: 'menu-options', label: 'Secciones del menú', component: MenuOptionsSection },
  { id: 'catalog-retention', label: 'Retención de catálogos', component: CatalogRetentionSection },
]

export function SettingsView() {
  return (
    <div className="content-pad">
      <div style={{ display: 'flex', flexDirection: 'column', gap: 20, maxWidth: 640 }}>
        {SETTINGS_SECTIONS.map(section => {
          const Section = section.component
          return (
            <Card key={section.id}>
              <CardHeader>{section.label}</CardHeader>
              <div style={{ padding: 18 }}>
                <Section />
              </div>
            </Card>
          )
        })}
      </div>
    </div>
  )
}
