import { useCallback, useEffect, useState } from 'react'
import { ListPlus, Plus, Trash2 } from 'lucide-react'
import { api, CustomFieldDef } from '../../api/client'
import { useWorkspace } from '../../context/WorkspaceContext'
import { useI18n } from '../../i18n'

const ENTITY_TYPES = ['test_case', 'test_plan', 'test_run', 'test_execution', 'requirement', 'milestone']
const FIELD_TYPES = ['text', 'number', 'date', 'list', 'multiselect', 'boolean']

// CustomFieldDefsSection manages custom field definitions (admin).
export function CustomFieldDefsSection() {
  const { notify } = useWorkspace()
  const { t } = useI18n()
  const [defs, setDefs] = useState<CustomFieldDef[]>([])
  const [entityType, setEntityType] = useState('test_case')
  const [name, setName] = useState('')
  const [fieldType, setFieldType] = useState('text')
  const [options, setOptions] = useState('[]')
  const [required, setRequired] = useState(false)

  const load = useCallback(async () => {
    try {
      const res = await api.listCustomFieldDefs()
      setDefs(res.items ?? [])
    } catch {
      // ignore
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  const add = async () => {
    if (!name.trim()) return
    try {
      await api.createCustomFieldDef({
        entity_type: entityType,
        name: name.trim(),
        field_type: fieldType,
        options: fieldType === 'list' || fieldType === 'multiselect' ? options : '[]',
        required,
      })
      notify(t.customFields.defined)
      setName('')
      setOptions('[]')
      await load()
    } catch (err) {
      notify(err instanceof Error ? err.message : t.customFields.createFailed, 'error')
    }
  }

  const remove = async (id: number) => {
    try {
      await api.deleteCustomFieldDef(id)
      notify(t.customFields.deleted)
      await load()
    } catch {
      notify(t.customFields.deleteFailed, 'error')
    }
  }

  return (
    <div className="card">
      <div className="card-header">
        <h3 className="card-title"><ListPlus size={15} /> {t.customFields.title}</h3>
      </div>
      <div style={{ display: 'grid', gap: '0.5rem' }}>
        {defs.length === 0 && <p style={{ color: 'var(--muted)', fontSize: '0.85rem', margin: 0 }}>{t.customFields.empty}</p>}
        {defs.map((d) => (
          <div key={d.id} style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', fontSize: '0.85rem' }}>
            <span className="badge">{d.entity_type}</span>
            <strong>{d.name}</strong>
            <span style={{ color: 'var(--muted)' }}>{d.field_type}</span>
            {d.field_type === 'list' || d.field_type === 'multiselect' ? (
              <code style={{ color: 'var(--muted)', fontSize: '0.75rem' }}>{d.options}</code>
            ) : null}
            {d.required && <span className="badge pill-failed">{t.customFields.required}</span>}
            <button type="button" className="btn btn-sm btn-danger" style={{ marginLeft: 'auto' }} onClick={() => remove(d.id)} aria-label={t.customFields.deleteAria(d.name)}>
              <Trash2 size={13} />
            </button>
          </div>
        ))}
        <div style={{ display: 'flex', gap: '0.4rem', flexWrap: 'wrap', alignItems: 'center', borderTop: '1px solid var(--border)', paddingTop: '0.6rem' }}>
          <select className="select" value={entityType} onChange={(e) => setEntityType(e.target.value)} style={{ fontSize: '0.8rem' }}>
            {ENTITY_TYPES.map((t) => (
              <option key={t} value={t}>{t}</option>
            ))}
          </select>
          <input className="input" placeholder={t.customFields.namePlaceholder} value={name} onChange={(e) => setName(e.target.value)} style={{ width: 140, fontSize: '0.8rem' }} />
          <select className="select" value={fieldType} onChange={(e) => setFieldType(e.target.value)} style={{ fontSize: '0.8rem' }}>
            {FIELD_TYPES.map((t) => (
              <option key={t} value={t}>{t}</option>
            ))}
          </select>
          {(fieldType === 'list' || fieldType === 'multiselect') && (
            <input className="input" placeholder={t.customFields.optionsPlaceholder} value={options} onChange={(e) => setOptions(e.target.value)} style={{ width: 180, fontSize: '0.8rem', fontFamily: 'var(--font-mono)' }} />
          )}
          <label style={{ display: 'inline-flex', alignItems: 'center', gap: '0.3rem', fontSize: '0.8rem' }}>
            <input type="checkbox" checked={required} onChange={(e) => setRequired(e.target.checked)} /> {t.customFields.required}
          </label>
          <button type="button" className="btn btn-sm" onClick={add} disabled={!name.trim()}>
            <Plus size={13} /> {t.common.add}
          </button>
        </div>
      </div>
    </div>
  )
}
