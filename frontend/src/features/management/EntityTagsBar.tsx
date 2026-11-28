import { useEffect, useState } from 'react'
import { api, type TagItem } from '../../api/client'
import { useI18n } from '../../i18n'

interface Props {
  entity: 'testcases' | 'testplans' | 'testruns'
  entityId: number
}

export function EntityTagsBar({ entity, entityId }: Props) {
  const { t } = useI18n()
  const [attached, setAttached] = useState<TagItem[]>([])
  const [allTags, setAllTags] = useState<TagItem[]>([])
  const [selectedTagId, setSelectedTagId] = useState<number>(0)
  const [err, setErr] = useState<string | null>(null)

  const load = async () => {
    try {
      const [att, all] = await Promise.all([api.listEntityTags(entity, entityId), api.listTags()])
      setAttached(att.items)
      setAllTags(all.items)
      if (!selectedTagId && all.items.length > 0) setSelectedTagId(all.items[0].id)
      setErr(null)
    } catch {
      // Ignore when unauthenticated
    }
  }

  useEffect(() => {
    void load()
  }, [entity, entityId])

  const handleAttach = async () => {
    if (!selectedTagId) return
    try {
      await api.attachEntityTag(entity, entityId, selectedTagId)
      await load()
    } catch (e) {
      setErr(e instanceof Error ? e.message : t.tags.attachFailed)
    }
  }

  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: '0.4rem',
        flexWrap: 'wrap',
        marginTop: '0.45rem',
        fontSize: '0.82rem',
      }}
    >
      <span style={{ color: 'var(--muted)' }}>{t.tags.label}</span>
      {attached.map((tg) => (
        <span key={tg.id} className="badge">
          {tg.name}
        </span>
      ))}
      {attached.length === 0 && <span style={{ color: 'var(--muted)' }}>—</span>}
      {allTags.length > 0 && (
        <>
          <select
            className="select"
            value={selectedTagId}
            onChange={(e) => setSelectedTagId(Number(e.target.value))}
            style={{ padding: '0.18rem 0.4rem', fontSize: '0.8rem' }}
          >
            {allTags.map((tg) => (
              <option key={tg.id} value={tg.id}>
                {tg.name}
              </option>
            ))}
          </select>
          <button type="button" className="btn btn-sm" onClick={() => void handleAttach()}>
            {t.tags.add}
          </button>
        </>
      )}
      {err && <span style={{ color: 'var(--danger)' }}>{err}</span>}
    </div>
  )
}
