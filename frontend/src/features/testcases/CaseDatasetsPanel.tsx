import { Database, Pencil, Plus, Sparkles, Trash2, X } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
import { api, type Dataset } from '../../api/client'
import { useWorkspace } from '../../context/WorkspaceContext'
import { useI18n } from '../../i18n'

type Proposal = { name: string; params: string; rationale: string }

// CaseDatasetsPanel manages data-driven datasets of a test case.
export function CaseDatasetsPanel({ caseId }: { caseId: number }) {
  const { notify } = useWorkspace()
  const { t } = useI18n()
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
        notify(t.datasets.updated)
      } else {
        await api.createDataset(caseId, { name: name.trim(), params })
        notify(t.datasets.added)
      }
      setName('')
      setParams('{}')
      setEditingId(null)
      await load()
    } catch {
      notify(t.datasets.paramsInvalid, 'error')
    }
  }

  const remove = async (id: number) => {
    try {
      await api.deleteDataset(id)
      notify(t.datasets.deleted)
      await load()
    } catch {
      notify(t.datasets.failedDelete, 'error')
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
      notify(e instanceof Error ? e.message : t.datasets.aiFailed, 'error')
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
      notify(t.datasets.added)
      setProposals(null)
      await load()
    } catch {
      notify(t.datasets.paramsInvalid, 'error')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div style={{ display: 'grid', gap: '0.45rem' }}>
      <strong style={{ fontSize: '0.85rem', display: 'inline-flex', alignItems: 'center', gap: '0.35rem' }}>
        <Database size={14} /> {t.datasets.title}
        <button
          type="button"
          className="btn btn-sm"
          onClick={generate}
          disabled={aiBusy}
          aria-label={t.datasets.aiGenerate}
        >
          <Sparkles size={12} /> {aiBusy ? t.datasets.aiGenerating : t.datasets.aiGenerate}
        </button>
      </strong>
      {items.map((d) => (
        <div key={d.id} style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', fontSize: '0.82rem' }}>
          <span className="badge">{d.name}</span>
          <code
            style={{
              color: 'var(--muted)',
              fontSize: '0.72rem',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
              maxWidth: 260,
            }}
          >
            {d.params}
          </code>
          <span style={{ marginLeft: 'auto', display: 'flex', gap: '0.2rem' }}>
            <button
              type="button"
              className="btn btn-sm"
              aria-label={`${t.datasets.editAria} ${d.name}`}
              onClick={() => {
                setEditingId(d.id)
                setName(d.name)
                setParams(d.params)
              }}
            >
              <Pencil size={12} />
            </button>
            <button
              type="button"
              className="btn btn-sm btn-danger"
              aria-label={`${t.datasets.deleteAria} ${d.name}`}
              onClick={() => remove(d.id)}
            >
              <Trash2 size={12} />
            </button>
          </span>
        </div>
      ))}
      <div style={{ display: 'flex', gap: '0.35rem', alignItems: 'flex-start', flexWrap: 'wrap' }}>
        <input
          className="input"
          placeholder={t.datasets.namePlaceholder}
          value={name}
          onChange={(e) => setName(e.target.value)}
          style={{ width: 150, fontSize: '0.8rem' }}
        />
        <input
          className="input"
          placeholder={t.datasets.paramsPlaceholder}
          value={params}
          onChange={(e) => setParams(e.target.value)}
          style={{ flex: 1, minWidth: 180, fontSize: '0.8rem', fontFamily: 'var(--font-mono)' }}
        />
        <button type="button" className="btn btn-sm" onClick={save} disabled={!name.trim()}>
          {editingId ? (
            t.common.save
          ) : (
            <>
              <Plus size={13} /> {t.common.add}
            </>
          )}
        </button>
        {editingId && (
          <button
            type="button"
            className="btn btn-sm"
            onClick={() => {
              setEditingId(null)
              setName('')
              setParams('{}')
            }}
            aria-label={t.common.cancel}
          >
            <X size={13} />
          </button>
        )}
      </div>
      {proposals && (
        <div className="modal-backdrop" onClick={() => setProposals(null)}>
          <div
            className="modal"
            onClick={(e) => e.stopPropagation()}
            role="dialog"
            aria-modal="true"
            aria-label={t.datasets.proposals}
          >
            <h2>{t.datasets.proposals}</h2>
            <div style={{ display: 'grid', gap: '0.5rem', maxHeight: '50vh', overflowY: 'auto' }}>
              {proposals.map((p, i) => (
                <label
                  key={i}
                  style={{
                    display: 'grid',
                    gap: '0.15rem',
                    padding: '0.35rem',
                    border: '1px solid var(--border)',
                    borderRadius: 6,
                  }}
                >
                  <span style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', fontSize: '0.85rem' }}>
                    <input
                      type="checkbox"
                      checked={!!selected[i]}
                      onChange={(e) => setSelected((s) => ({ ...s, [i]: e.target.checked }))}
                    />
                    <strong>{p.name}</strong>
                  </span>
                  <code style={{ fontSize: '0.75rem', fontFamily: 'var(--font-mono)' }}>{p.params}</code>
                  <span style={{ fontSize: '0.75rem', color: 'var(--muted)' }}>{p.rationale}</span>
                </label>
              ))}
            </div>
            <div style={{ display: 'flex', gap: '0.4rem', justifyContent: 'flex-end', marginTop: '0.75rem' }}>
              <button type="button" className="btn" onClick={() => setProposals(null)}>
                {t.common.cancel}
              </button>
              <button
                type="button"
                className="btn btn-primary"
                onClick={saveProposals}
                disabled={saving || !Object.values(selected).some(Boolean)}
              >
                {saving ? t.datasets.saving : t.datasets.addSelected}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
