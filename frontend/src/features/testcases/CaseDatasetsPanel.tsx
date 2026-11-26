import { useCallback, useEffect, useState } from 'react'
import { Database, Pencil, Plus, Sparkles, Trash2, X } from 'lucide-react'
import { api, Dataset } from '../../api/client'
import { useWorkspace } from '../../context/WorkspaceContext'

type Proposal = { name: string; params: string; rationale: string }

// CaseDatasetsPanel manages data-driven datasets of a test case.
export function CaseDatasetsPanel({ caseId }: { caseId: number }) {
  const { notify } = useWorkspace()
  const [items, setItems] = useState<Dataset[]>([])
  const [name, setName] = useState('')
  const [params, setParams] = useState('{}')
  const [editingId, setEditingId] = useState<number | null>(null)
  const [aiBusy, setAiBusy] = useState(false)
  const [proposals, setProposals] = useState<Proposal[] | null>(null)
  const [selected, setSelected] = useState<Record<number, boolean>>({})
  const [saving, setSaving] = useState(false)

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

  const generate = async () => {
    setAiBusy(true)
    setProposals(null)
    try {
      const res = await api.aiGenerateDatasets(caseId)
      setProposals(res.datasets)
      setSelected(Object.fromEntries(res.datasets.map((_, i) => [i, true])))
    } catch (e) {
      notify(e instanceof Error ? e.message : 'AI generation failed', 'error')
    } finally {
      setAiBusy(false)
    }
  }

  const saveProposals = async () => {
    if (!proposals) return
    setSaving(true)
    try {
      for (let i = 0; i < proposals.length; i++) {
        if (selected[i]) {
          await api.createDataset(caseId, { name: proposals[i].name, params: proposals[i].params })
        }
      }
      notify('Datasets added')
      setProposals(null)
      await load()
    } catch {
      notify('Params must be valid JSON', 'error')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div style={{ display: 'grid', gap: '0.45rem' }}>
      <strong style={{ fontSize: '0.85rem', display: 'inline-flex', alignItems: 'center', gap: '0.35rem' }}>
        <Database size={14} /> Datasets (data-driven)
        <button type="button" className="btn btn-sm" onClick={generate} disabled={aiBusy} aria-label="Generate datasets with AI">
          <Sparkles size={12} /> {aiBusy ? 'Generating…' : 'AI'}
        </button>
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
      {proposals && (
        <div className="modal-backdrop" onClick={() => setProposals(null)}>
          <div className="modal" onClick={(e) => e.stopPropagation()} role="dialog" aria-modal="true" aria-label="AI dataset proposals">
            <h2>AI dataset proposals</h2>
            <div style={{ display: 'grid', gap: '0.5rem', maxHeight: '50vh', overflowY: 'auto' }}>
              {proposals.map((p, i) => (
                <label key={i} style={{ display: 'grid', gap: '0.15rem', padding: '0.35rem', border: '1px solid var(--border)', borderRadius: 6 }}>
                  <span style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', fontSize: '0.85rem' }}>
                    <input type="checkbox" checked={!!selected[i]} onChange={(e) => setSelected((s) => ({ ...s, [i]: e.target.checked }))} />
                    <strong>{p.name}</strong>
                  </span>
                  <code style={{ fontSize: '0.75rem', fontFamily: 'var(--font-mono)' }}>{p.params}</code>
                  <span style={{ fontSize: '0.75rem', color: 'var(--muted)' }}>{p.rationale}</span>
                </label>
              ))}
            </div>
            <div style={{ display: 'flex', gap: '0.4rem', justifyContent: 'flex-end', marginTop: '0.75rem' }}>
              <button type="button" className="btn" onClick={() => setProposals(null)}>Cancel</button>
              <button type="button" className="btn btn-primary" onClick={saveProposals} disabled={saving || !Object.values(selected).some(Boolean)}>
                {saving ? 'Saving…' : 'Add selected'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
