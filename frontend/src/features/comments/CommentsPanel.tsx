import { useEffect, useState } from 'react'
import { api, type CommentItem } from '../../api/client'
import { useI18n } from '../../i18n'

interface Props {
  entityType: 'test_case' | 'test_execution'
  entityId: number
}

export function CommentsPanel({ entityType, entityId }: Props) {
  const { t } = useI18n()
  const [items, setItems] = useState<CommentItem[]>([])
  const [body, setBody] = useState('')
  const [error, setError] = useState<string | null>(null)

  const load = async () => {
    try {
      const res = await api.listComments(entityType, entityId)
      setItems(res.items)
      setError(null)
    } catch {
      // Ignore when unauthenticated
    }
  }

  useEffect(() => {
    void load()
  }, [entityType, entityId])

  const handleAdd = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!body.trim()) return
    try {
      await api.createComment(entityType, entityId, body.trim())
      setBody('')
      setError(null)
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : t.comments.failed)
    }
  }

  return (
    <div style={{ display: 'grid', gap: '0.5rem', fontSize: '0.84rem' }}>
      {error && (
        <div className="alert-error" style={{ margin: 0 }}>
          {error}
        </div>
      )}
      <form onSubmit={handleAdd} style={{ display: 'flex', gap: '0.4rem' }}>
        <input
          className="input"
          placeholder={t.comments.placeholder}
          value={body}
          onChange={(e) => setBody(e.target.value)}
          style={{ flex: 1 }}
        />
        <button type="submit" className="btn btn-primary btn-sm">
          {t.comments.send}
        </button>
      </form>
      {items.length > 0 ? (
        <ul style={{ listStyle: 'none', padding: 0, margin: 0, display: 'grid', gap: '0.35rem' }}>
          {items.map((c) => (
            <li
              key={c.id}
              style={{
                color: 'var(--text)',
                background: 'var(--bg-subtle)',
                border: '1px solid var(--border)',
                padding: '0.45rem 0.65rem',
                borderRadius: 8,
              }}
            >
              {c.body}
            </li>
          ))}
        </ul>
      ) : (
        <span style={{ color: 'var(--muted)', fontSize: '0.8rem' }}>{t.comments.empty}</span>
      )}
    </div>
  )
}
