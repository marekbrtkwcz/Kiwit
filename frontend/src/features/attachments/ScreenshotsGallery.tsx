import { useEffect, useState } from 'react'
import { Image as ImageIcon } from 'lucide-react'
import { api, AttachmentItem } from '../../api/client'

// ScreenshotsGallery renders image attachments of a test execution as inline thumbnails.
export function ScreenshotsGallery({ executionId }: { executionId: number }) {
  const [items, setItems] = useState<AttachmentItem[]>([])
  const [urls, setUrls] = useState<Record<number, string>>({})
  const [loaded, setLoaded] = useState(false)

  useEffect(() => {
    let cancelled = false
    const created: string[] = []
    api.listExecutionScreenshots(executionId)
      .then((res) => {
        if (cancelled) return
        setItems(res.items)
        setLoaded(true)
        res.items.forEach((a) => {
          api.fetchAttachmentBlob(a.id)
            .then((u) => {
              created.push(u)
              if (!cancelled) setUrls((prev) => ({ ...prev, [a.id]: u }))
            })
            .catch(() => {})
        })
      })
      .catch(() => setLoaded(true))
    return () => {
      cancelled = true
      created.forEach((u) => URL.revokeObjectURL(u))
    }
  }, [executionId])

  if (!loaded || items.length === 0) return null

  return (
    <div style={{ display: 'grid', gap: '0.4rem' }}>
      <strong style={{ fontSize: '0.85rem', display: 'inline-flex', alignItems: 'center', gap: '0.35rem' }}>
        <ImageIcon size={14} /> Screenshots ({items.length})
      </strong>
      <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}>
        {items.map((a) => (
          <a
            key={a.id}
            href={urls[a.id] ?? '#'}
            target="_blank"
            rel="noopener noreferrer"
            title={a.filename}
            style={{ display: 'block', border: '1px solid var(--border)', borderRadius: 6, overflow: 'hidden', width: 140 }}
          >
            {urls[a.id] ? (
              <img src={urls[a.id]} alt={a.filename} style={{ width: '100%', height: 90, objectFit: 'cover', display: 'block' }} />
            ) : (
              <div style={{ width: '100%', height: 90, display: 'grid', placeItems: 'center', color: 'var(--muted)' }}>
                <ImageIcon size={20} />
              </div>
            )}
            <div style={{ fontSize: '0.7rem', padding: '0.2rem 0.35rem', color: 'var(--muted)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
              {a.filename}
            </div>
          </a>
        ))}
      </div>
    </div>
  )
}
