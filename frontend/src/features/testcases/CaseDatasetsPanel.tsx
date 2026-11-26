import { useCallback, useEffect, useState } from 'react'
import { Database, Pencil, Plus, Trash2, X } from 'lucide-react'
import { api, Dataset } from '../../api/client'
import { useWorkspace } from '../../context/WorkspaceContext'

// CaseDatasetsPanel manages data-driven datasets of a test case.
export function CaseDatasetsPanel({ caseId }: { caseId: number }) {
  const { notify } = useWorkspace()
  const [items, setItems] = useState<Dataset[]>([])
  const [name, setName] = useState('')
  const [params, setParams] = useState('{}')
  const [editingId, setEditingId] = useState<number | null>(null)

  const load = useCallback(async () => {
    try {
      const res = await api.listCaseDatasets(caseId)
      setItems(res.items)
    } catch {
      // ignore
    }
  }, [caseId])

  useEffect(() => {
    void load()
  }, [load])

  const save = async () => {
    if (!name.trim()) return
    try {
      if (editingId) {
        await api.updateDataset(editingId, { name: name.trim(), params })
        notify('Dataset updated')
      } else {
        await api.createDataset(caseId, { name: name.trim(), params })
        notify('Dataset added')
      }
      setName('')
      setParams('{}')
      setEditingId(null)
      await load()
    } catch {
      notify('Params must be valid JSON', 'error')
    }
  }

  const remove = async (id: number) => {
    try {
      await api.deleteDataset(id)
      notify('Dataset deleted')
      await load()
    } catch {
      notify('Failed to delete dataset', 'error')
    }
  }

  return (
    <div style={{ display: 'grid', gap: '0.45rem' }}>
      <strong style={{ fontSize: '0.85rem', display: 'inline-flex', alignItems: 'center', gap: '0.35rem' }}>
        <Database size={14} /> Datasets (data-driven)
      </strong>
      {items.map((d) => (
        <div key={d.id} style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', fontSize: '0.82rem' }}>
          <span className="badge">{d.name}</span>
          <code style={{ color: 'var(--muted)', fontSize: '0.72rem', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 260 }}>
            {d.params}
          </code>
          <span style={{ marginLeft: 'auto', display: 'flex', gap: '0.2rem' }}>
            <button
              type="button"
              className="btn btn-sm"
              aria-label={`Edit dataset ${d.name}`}
              onClick={() => {
                setEditingId(d.id)
                setName(d.name)
                setParams(d.params)
              }}
            >
              <Pencil size={12} />
            </button>
            <button type="button" className="btn btn-sm btn-danger" aria-label={`Delete dataset ${d.name}`} onClick={() => remove(d.id)}>
              <Trash2 size={12} />
            </button>
          </span>
        </div>
      ))}
      <div style={{ display: 'flex', gap: '0.35rem', alignItems: 'flex-start', flexWrap: 'wrap' }}>
        <input className="input" placeholder="Name (e.g. admin user)" value={name} onChange={(e) => setName(e.target.value)} style={{ width: 150, fontSize: '0.8rem' }} />
        <input className="input" placeholder='Params JSON, e.g. {"user":"admin"}' value={params} onChange={(e) => setParams(e.target.value)} style={{ flex: 1, minWidth: 180, fontSize: '0.8rem', fontFamily: 'var(--font-mono)' }} />
        <button type="button" className="btn btn-sm" onClick={save} disabled={!name.trim()}>
          {editingId ? 'Save' : <><Plus size={13} /> Add</>}
        </button>
        {editingId && (
          <button type="button" className="btn btn-sm" onClick={() => { setEditingId(null); setName(''); setParams('{}') }} aria-label="Cancel edit">
            <X size={13} />
          </button>
        )}
      </div>
    </div>
  )
}
