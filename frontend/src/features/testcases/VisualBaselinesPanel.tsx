import { useCallback, useEffect, useState } from 'react'
import { Eye, Trash2 } from 'lucide-react'
import { api, AttachmentItem, VisualBaseline } from '../../api/client'
import { useWorkspace } from '../../context/WorkspaceContext'

// VisualBaselinesPanel manages visual regression baselines of a test case.
export function VisualBaselinesPanel({ caseId }: { caseId: number }) {
  const { notify } = useWorkspace()
  const [baselines, setBaselines] = useState<VisualBaseline[]>([])
  const [images, setImages] = useState<AttachmentItem[]>([])
  const [label, setLabel] = useState('')

  const load = useCallback(async () => {
    try {
      const [b, a] = await Promise.all([
        api.listVisualBaselines(caseId),
        api.listAttachments('test_case', caseId),
      ])
      setBaselines(b.items ?? [])
      setImages((a.items ?? []).filter((x) => x.content_type.startsWith('image/')))
    } catch {
      // ignore
    }
  }, [caseId])

  useEffect(() => {
    void load()
  }, [load])

  const promote = async (attachmentId: number) => {
    try {
      await api.addVisualBaseline(caseId, attachmentId, label || undefined)
      setLabel('')
      notify('Baseline added')
      await load()
    } catch {
      notify('Failed to add baseline', 'error')
    }
  }

  const remove = async (id: number) => {
    try {
      await api.deleteVisualBaseline(id)
      notify('Baseline deleted')
      await load()
    } catch {
      notify('Failed to delete baseline', 'error')
    }
  }

  return (
    <div style={{ display: 'grid', gap: '0.45rem' }}>
      <strong style={{ fontSize: '0.85rem', display: 'inline-flex', alignItems: 'center', gap: '0.35rem' }}>
        <Eye size={14} /> Visual baselines ({baselines.length})
      </strong>
      {baselines.map((b) => (
        <div key={b.id} style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', fontSize: '0.82rem' }}>
          <span className="badge">#{b.attachment_id} {b.label || 'baseline'}</span>
          <button type="button" className="btn btn-sm btn-danger" style={{ marginLeft: 'auto' }} onClick={() => remove(b.id)} aria-label="Delete baseline">
            <Trash2 size={12} />
          </button>
        </div>
      ))}
      {images.length > 0 && (
        <div style={{ display: 'flex', gap: '0.35rem', alignItems: 'center', flexWrap: 'wrap', borderTop: '1px solid var(--border)', paddingTop: '0.5rem' }}>
          <span style={{ fontSize: '0.8rem', color: 'var(--muted)' }}>Promote image to baseline:</span>
          {images.map((a) => (
            <button key={a.id} type="button" className="btn btn-sm" onClick={() => promote(a.id)} title={a.filename}>
              #{a.id} {a.filename.slice(0, 18)}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
