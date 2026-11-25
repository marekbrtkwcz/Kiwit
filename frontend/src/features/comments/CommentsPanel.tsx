import { useEffect, useState } from 'react'
import { api, CommentItem } from '../../api/client'

interface Props {
  entityType: 'test_case' | 'test_execution'
  entityId: number
}

export function CommentsPanel({ entityType, entityId }: Props) {
  const [items, setItems] = useState<CommentItem[]>([])
  const [body, setBody] = useState('')

  const load = async () => {
    try {
      const res = await api.listComments(entityType, entityId)
      setItems(res.items)
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
    await api.createComment(entityType, entityId, body.trim())
    setBody('')
    await load()
  }

  return (
    <div style={{ marginTop: '0.5rem', fontSize: '0.85rem' }}>
      <form onSubmit={handleAdd} style={{ display: 'flex', gap: '0.35rem' }}>
        <input
          placeholder="Comment..."
          value={body}
          onChange={(e) => setBody(e.target.value)}
          style={{ flex: 1, padding: '0.25rem 0.5rem', borderRadius: 4, border: '1px solid #d1d5db' }}
        />
        <button type="submit" style={{ padding: '0.25rem 0.6rem', borderRadius: 4, border: '1px solid #d1d5db', background: '#fff', cursor: 'pointer' }}>
          Send
        </button>
      </form>
      {items.length > 0 && (
        <ul style={{ listStyle: 'none', padding: 0, margin: '0.3rem 0 0', display: 'grid', gap: '0.2rem' }}>
          {items.map((c) => (
            <li key={c.id} style={{ color: '#374151', background: '#f9fafb', padding: '0.25rem 0.4rem', borderRadius: 4 }}>
              {c.body}
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
