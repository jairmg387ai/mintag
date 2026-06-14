import { useState } from 'react'
import { Download } from 'lucide-react'
import { exportDWMarkdown } from '../../api/client'

interface ExportButtonProps {
  dwId: number
  title: string
}

export function ExportButton({ dwId, title }: ExportButtonProps) {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function handleExport() {
    setLoading(true)
    setError(null)
    try {
      const markdown = await exportDWMarkdown(dwId)
      const blob = new Blob([markdown], { type: 'text/markdown' })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `ventana-${dwId}.md`
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      URL.revokeObjectURL(url)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Error al exportar')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div style={{ display: 'inline-flex', flexDirection: 'column', gap: 4 }}>
      <button
        className="btn btn-ghost btn-sm"
        onClick={handleExport}
        disabled={loading}
        title={`Exportar "${title}" como Markdown`}
      >
        <Download size={14} strokeWidth={1.75} />
        {loading ? 'Exportando...' : 'Exportar Markdown'}
      </button>
      {error && (
        <span style={{ font: 'var(--text-caption)', color: 'var(--block-solid)' }}>{error}</span>
      )}
    </div>
  )
}
