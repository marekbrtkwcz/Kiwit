import { useEffect, useState } from 'react'
import { api, AttachmentItem } from '../../api/client'
import { useI18n } from '../../i18n'

interface Props {
  entityType: 'test_case' | 'test_execution'
  entityId: number
}

export function AttachmentsPanel({ entityType, entityId }: Props) {
  const { t } = useI18n()
  const [items, setItems] = useState<AttachmentItem[]>([])
  const [error, setError] = useState<string | null>(null)

  const load = async () => {
    try {
      const res = await api.listAttachments(entityType, entityId)
      setItems(res.items)
      setError(null)
    } catch {
      // Ignore initial load errors when unauthenticated
    }
  }

  useEffect(() => {
    void load()
  }, [entityType, entityId])

  const handleFile = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    try {
      await api.uploadAttachment(entityType, entityId, file)
      e.target.value = ''
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errors.uploadAttachment)
    }
  }

  return (
    <div style={{ marginTop: '0.4rem', fontSize: '0.85rem' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
        <span style={{ color: '#6b7280' }}>{t.attachments.title}:</span>
        <label style={{ cursor: 'pointer', color: '#2563eb', textDecoration: 'underline' }}>
          {t.attachments.upload}
          <input type="file" onChange={(e) => void handleFile(e)} style={{ display: 'none' }} />
        </label>
      </div>
      {error && <div style={{ color: '#b91c1c' }}>{error}</div>}
      {items.length > 0 && (
        <ul style={{ listStyle: 'none', padding: 0, margin: '0.25rem 0 0', display: 'flex', flexWrap: 'wrap', gap: '0.5rem' }}>
          {items.map((a) => (
            <li key={a.id} style={{ display: 'inline-flex', alignItems: 'center', gap: '0.25rem' }}>
              <button
                type="button"
                onClick={async () => {
                  try {
                    await api.downloadAttachment(a.id, a.filename)
                    setError(null)
                  } catch (err) {
                    setError(err instanceof Error ? err.message : t.errors.uploadAttachment)
                  }
                }}
                style={{ background: 'none', border: 'none', padding: 0, color: '#111827', textDecoration: 'underline', cursor: 'pointer', fontSize: 'inherit' }}
              >
                {a.filename} ({Math.ceil(a.size_bytes / 1024)} KB)
              </button>
              <button
                type="button"
                aria-label={`Delete ${a.filename}`}
                onClick={async () => {
                  try {
                    await api.deleteAttachment(a.id)
                    await load()
                  } catch (err) {
                    setError(err instanceof Error ? err.message : t.errors.uploadAttachment)
                  }
                }}
                style={{ background: 'none', border: 'none', padding: '0 0.2rem', color: '#b91c1c', cursor: 'pointer', fontSize: '0.85rem' }}
              >
                ×
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
