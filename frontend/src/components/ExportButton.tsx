import { Download } from 'lucide-react'
import { useState } from 'react'
import { downloadExport, downloadExportSelection, type ExportFormat } from '../api/client'
import { useI18n } from '../i18n'

interface ExportButtonProps {
  /** API path, e.g. "/api/v1/testcases" or "/api/v1/telemetry" */
  path: string
  /** download filename without extension */
  filename: string
  /** query parameters applied when no IDs are selected */
  params?: Record<string, string | number | undefined>
  /** when non-empty, a POST selection export is used instead of the filtered GET */
  ids?: number[]
  onError?: (message: string) => void
  small?: boolean
  /** formats offered in the menu; defaults to csv/json/md */
  formats?: ExportFormat[]
}

const DEFAULT_FORMATS: ExportFormat[] = ['csv', 'json', 'md']

const FORMAT_LABEL: Record<ExportFormat, 'exportCsv' | 'exportJson' | 'exportMd' | 'exportXlsx' | 'exportFeature'> = {
  csv: 'exportCsv',
  json: 'exportJson',
  md: 'exportMd',
  xlsx: 'exportXlsx',
  feature: 'exportFeature',
}

/**
 * Unified export control: renders one button that downloads the current
 * list (or the current selection) as CSV, JSON or Markdown.
 */
export function ExportButton({ path, filename, params, ids, onError, small, formats }: ExportButtonProps) {
  const menu = formats && formats.length > 0 ? formats : DEFAULT_FORMATS
  const { t } = useI18n()
  const [open, setOpen] = useState(false)
  const [busy, setBusy] = useState(false)

  const run = async (format: ExportFormat) => {
    setOpen(false)
    setBusy(true)
    try {
      if (ids && ids.length > 0) {
        await downloadExportSelection(path, format, ids, `${filename}.${format}`)
      } else {
        await downloadExport(path, format, `${filename}.${format}`, params)
      }
    } catch (err) {
      onError?.(err instanceof Error ? err.message : t.common.exportFailed)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div style={{ position: 'relative' }}>
      <button
        type="button"
        className={`btn${small ? ' btn-sm' : ''}`}
        disabled={busy}
        onClick={() => setOpen((v) => !v)}
      >
        <Download size={14} />
        {t.common.export}
      </button>
      {open && (
        <>
          <div style={{ position: 'fixed', inset: 0, zIndex: 40 }} onClick={() => setOpen(false)} />
          <div
            style={{
              position: 'absolute',
              right: 0,
              top: 'calc(100% + 4px)',
              zIndex: 50,
              display: 'grid',
              gap: 2,
              padding: '0.3rem',
              background: 'var(--surface)',
              border: '1px solid var(--border)',
              borderRadius: 'var(--radius)',
              boxShadow: 'var(--shadow-xs)',
              minWidth: 120,
            }}
          >
            {menu.map((f) => (
              <button
                key={f}
                type="button"
                className="btn btn-ghost btn-sm"
                style={{ justifyContent: 'flex-start' }}
                onClick={() => void run(f)}
              >
                {t.common[FORMAT_LABEL[f]]}
              </button>
            ))}
          </div>
        </>
      )}
    </div>
  )
}
