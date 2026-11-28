import { Settings2 } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
import { api, type CustomFieldWithValue } from '../api/client'
import { useWorkspace } from '../context/WorkspaceContext'
import { useI18n } from '../i18n'

// CustomFieldsPanel renders and edits custom field values for one entity.
export function CustomFieldsPanel({ entityType, entityId }: { entityType: string; entityId: number }) {
  const { notify } = useWorkspace()
  const { t } = useI18n()
  const [fields, setFields] = useState<CustomFieldWithValue[]>([])
  const [values, setValues] = useState<Record<number, string>>({})

  const load = useCallback(async () => {
    try {
      const res = await api.getEntityCustomFields(entityType, entityId)
      setFields(res.items ?? [])
      const v: Record<number, string> = {}
      for (const f of res.items ?? []) v[f.id] = f.value ?? ''
      setValues(v)
    } catch {
      // ignore
    }
  }, [entityType, entityId])

  useEffect(() => {
    void load()
  }, [load])

  if (fields.length === 0) return null

  const save = async (fieldId: number) => {
    try {
      await api.setEntityCustomField(entityType, entityId, fieldId, values[fieldId] ?? '')
      notify(t.customFields.saved)
    } catch (err) {
      notify(err instanceof Error ? err.message : t.customFields.saveFailed, 'error')
    }
  }

  const renderInput = (f: CustomFieldWithValue) => {
    const val = values[f.id] ?? ''
    const set = (v: string) => setValues((prev) => ({ ...prev, [f.id]: v }))
    switch (f.field_type) {
      case 'boolean':
        return (
          <select
            id={`cf-${f.id}`}
            className="select"
            value={val || 'false'}
            onChange={(e) => {
              set(e.target.value)
            }}
            style={{ fontSize: '0.8rem' }}
          >
            <option value="false">false</option>
            <option value="true">true</option>
          </select>
        )
      case 'date':
        return (
          <input
            id={`cf-${f.id}`}
            className="input"
            type="date"
            value={val}
            onChange={(e) => set(e.target.value)}
            style={{ fontSize: '0.8rem' }}
          />
        )
      case 'list': {
        let opts: string[] = []
        try {
          opts = JSON.parse(f.options || '[]')
        } catch {
          opts = []
        }
        return (
          <select
            id={`cf-${f.id}`}
            className="select"
            value={val}
            onChange={(e) => set(e.target.value)}
            style={{ fontSize: '0.8rem' }}
          >
            <option value="">—</option>
            {opts.map((o) => (
              <option key={o} value={o}>
                {o}
              </option>
            ))}
          </select>
        )
      }
      case 'number':
        return (
          <input
            id={`cf-${f.id}`}
            className="input"
            type="number"
            value={val}
            onChange={(e) => set(e.target.value)}
            style={{ fontSize: '0.8rem' }}
          />
        )
      default:
        return (
          <input
            id={`cf-${f.id}`}
            className="input"
            value={val}
            onChange={(e) => set(e.target.value)}
            style={{ fontSize: '0.8rem' }}
          />
        )
    }
  }

  return (
    <div style={{ display: 'grid', gap: '0.45rem' }}>
      <strong style={{ fontSize: '0.85rem', display: 'inline-flex', alignItems: 'center', gap: '0.35rem' }}>
        <Settings2 size={14} /> Custom fields
      </strong>
      {fields.map((f) => (
        <div key={f.id} style={{ display: 'flex', gap: '0.4rem', alignItems: 'center' }}>
          <label
            style={{
              width: 140,
              fontSize: '0.8rem',
              color: 'var(--muted)',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
            }}
            title={f.name}
            htmlFor={`cf-${f.id}`}
          >
            {f.name}
            {f.required ? ' *' : ''}
          </label>
          <div style={{ flex: 1 }}>{renderInput(f)}</div>
          <button type="button" className="btn btn-sm" onClick={() => save(f.id)}>
            Save
          </button>
        </div>
      ))}
    </div>
  )
}
