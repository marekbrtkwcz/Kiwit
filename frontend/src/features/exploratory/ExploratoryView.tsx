import { useCallback, useEffect, useState } from 'react'
import { Compass, Pencil, Plus, Sparkles, Trash2 } from 'lucide-react'
import { api, ExploratorySession, Product } from '../../api/client'
import { ConfirmDialog, EmptyState, LoadingSkeleton } from '../../components/UiPrimitives'
import { useWorkspace } from '../../context/WorkspaceContext'

interface FormState {
  product_id: number
  charter: string
  timebox_minutes: number
  notes: string
  findings: string
}

export function ExploratoryView() {
  const { notify } = useWorkspace()
  const [items, setItems] = useState<ExploratorySession[]>([])
  const [products, setProducts] = useState<Product[]>([])
  const [productId, setProductId] = useState(0)
  const [loading, setLoading] = useState(true)
  const [showForm, setShowForm] = useState(false)
  const [editing, setEditing] = useState<ExploratorySession | null>(null)
  const [confirmDelete, setConfirmDelete] = useState<ExploratorySession | null>(null)
  const [form, setForm] = useState<FormState>({ product_id: 0, charter: '', timebox_minutes: 60, notes: '', findings: '[]' })
  const [aiBusy, setAiBusy] = useState('')

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const [res, prods] = await Promise.all([
        api.listExploratory(productId || undefined),
        api.listProducts(),
      ])
      setItems(res.items ?? [])
      setProducts(prods.items ?? [])
    } catch {
      notify('Failed to load sessions', 'error')
    } finally {
      setLoading(false)
    }
  }, [productId, notify])

  useEffect(() => {
    void load()
  }, [load])

  const openCreate = () => {
    setEditing(null)
    setForm({ product_id: productId || (products[0]?.id ?? 0), charter: '', timebox_minutes: 60, notes: '', findings: '[]' })
    setShowForm(true)
  }

  const openEdit = (s: ExploratorySession) => {
    setEditing(s)
    setForm({ product_id: s.product_id, charter: s.charter, timebox_minutes: s.timebox_minutes, notes: s.notes ?? '', findings: s.findings || '[]' })
    setShowForm(true)
  }

  const submit = async () => {
    try {
      const body = { ...form, product_id: Number(form.product_id) }
      if (editing) {
        await api.updateExploratory(editing.id, body)
        notify('Session updated')
      } else {
        await api.createExploratory(body)
        notify('Session started')
      }
      setShowForm(false)
      await load()
    } catch {
      notify('Failed to save session', 'error')
    }
  }

  const doDelete = async (s: ExploratorySession) => {
    try {
      await api.deleteExploratory(s.id)
      setConfirmDelete(null)
      notify('Session deleted')
      await load()
    } catch {
      notify('Failed to delete session', 'error')
    }
  }

  return (
    <div className="view">
      <div className="view-header">
        <h1>Exploratory Testing</h1>
        <button type="button" className="btn btn-primary" onClick={openCreate}>
          <Plus size={16} /> New Session
        </button>
      </div>

      <div style={{ marginBottom: '0.75rem' }}>
        <select className="select" value={productId} onChange={(e) => setProductId(Number(e.target.value))}>
          <option value={0}>All products</option>
          {products.map((p) => (
            <option key={p.id} value={p.id}>{p.name}</option>
          ))}
        </select>
      </div>

      {loading ? (
        <LoadingSkeleton rows={4} />
      ) : items.length === 0 ? (
        <EmptyState
          icon={<Compass size={28} />}
          title="No exploratory sessions yet"
          description="Charter-based, timeboxed sessions with notes and findings."
          actionLabel="New Session"
          onAction={openCreate}
        />
      ) : (
        <div style={{ display: 'grid', gap: '0.6rem' }}>
          {items.map((s) => {
            let findings: string[] = []
            try { findings = JSON.parse(s.findings || '[]') } catch { findings = [] }
            return (
              <div key={s.id} className="card" style={{ display: 'grid', gap: '0.35rem' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                  <Compass size={15} />
                  <strong style={{ fontSize: '0.9rem' }}>{s.charter}</strong>
                  <span className="badge">{s.timebox_minutes} min</span>
                  {s.test_run_id ? <span className="badge">run #{s.test_run_id}</span> : null}
                  <span style={{ marginLeft: 'auto', display: 'flex', gap: '0.25rem' }}>
                    <button type="button" className="btn btn-sm" onClick={() => openEdit(s)} aria-label="Edit session"><Pencil size={13} /></button>
                    <button type="button" className="btn btn-sm btn-danger" onClick={() => setConfirmDelete(s)} aria-label="Delete session"><Trash2 size={13} /></button>
                  </span>
                </div>
                {s.notes && <div style={{ fontSize: '0.82rem', color: 'var(--muted)' }}>{s.notes}</div>}
                {findings.length > 0 && (
                  <ul style={{ margin: 0, paddingLeft: '1.1rem', fontSize: '0.82rem' }}>
                    {findings.map((f, i) => (
                      <li key={i}>{f}</li>
                    ))}
                  </ul>
                )}
                <div style={{ fontSize: '0.72rem', color: 'var(--muted)' }}>{new Date(s.created_at).toLocaleString()}</div>
              </div>
            )
          })}
        </div>
      )}

      {showForm && (
        <div className="modal-backdrop" onClick={() => setShowForm(false)}>
          <div className="modal" onClick={(e) => e.stopPropagation()} role="dialog" aria-modal="true" aria-label="Exploratory session form">
            <h2>{editing ? 'Edit Session' : 'New Exploratory Session'}</h2>
            <div style={{ display: 'grid', gap: '0.5rem' }}>
              <select className="select" value={form.product_id} onChange={(e) => setForm({ ...form, product_id: Number(e.target.value) })}>
                {products.map((p) => (
                  <option key={p.id} value={p.id}>{p.name}</option>
                ))}
              </select>
              <div style={{ display: 'flex', gap: '0.4rem', alignItems: 'center' }}>
                <input style={{ flex: 1 }} className="input" placeholder="Charter (mission, e.g. Explore checkout flow for edge cases)" value={form.charter} onChange={(e) => setForm({ ...form, charter: e.target.value })} />
                <button
                  type="button"
                  className="btn btn-sm"
                  disabled={aiBusy === 'charter'}
                  title="AI drafts a session charter from your objective"
                  onClick={async () => {
                    const objective = form.charter.trim() || window.prompt('Describe the session objective (area + what worries you)') || ''
                    if (!objective.trim()) return
                    setAiBusy('charter')
                    try {
                      const res = await api.aiGenerateCharter(objective)
                      setForm((f) => ({ ...f, charter: res.charter }))
                    } catch (err) {
                      notify(err instanceof Error ? err.message : 'AI charter failed', 'error')
                    } finally {
                      setAiBusy('')
                    }
                  }}
                >
                  <Sparkles size={13} /> {aiBusy === 'charter' ? '…' : 'AI charter'}
                </button>
              </div>
              <input className="input" type="number" min={5} step={5} value={form.timebox_minutes} onChange={(e) => setForm({ ...form, timebox_minutes: Number(e.target.value) })} />
              <textarea className="input" rows={4} placeholder="Notes during the session" value={form.notes} onChange={(e) => setForm({ ...form, notes: e.target.value })} />
              <div>
                <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: '0.3rem' }}>
                  <button
                    type="button"
                    className="btn btn-sm"
                    disabled={aiBusy === 'findings' || !editing}
                    title={editing ? 'AI extracts structured findings from this session notes' : 'Save the session first, then use AI findings'}
                    onClick={async () => {
                      if (!editing) return
                      setAiBusy('findings')
                      try {
                        const res = await api.aiNotesToFindings(editing.id)
                        setForm((f) => ({ ...f, findings: JSON.stringify(res.findings, null, 2) }))
                      } catch (err) {
                        notify(err instanceof Error ? err.message : 'AI findings failed', 'error')
                      } finally {
                        setAiBusy('')
                      }
                    }}
                  >
                    <Sparkles size={13} /> {aiBusy === 'findings' ? '…' : 'AI findings'}
                  </button>
                </div>
                <textarea className="input" rows={3} placeholder='Findings as JSON array: ["bug: coupon applied twice"]' value={form.findings} onChange={(e) => setForm({ ...form, findings: e.target.value })} />
              </div>
            </div>
            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.5rem', marginTop: '0.75rem' }}>
              <button type="button" className="btn" onClick={() => setShowForm(false)}>Cancel</button>
              <button type="button" className="btn btn-primary" onClick={submit} disabled={!form.charter || !form.product_id}>
                {editing ? 'Save' : 'Start'}
              </button>
            </div>
          </div>
        </div>
      )}

      {confirmDelete && (
        <ConfirmDialog
          open
          title="Delete session?"
          message={`"${confirmDelete.charter}" will be removed.`}
          confirmLabel="Delete"
          onConfirm={() => void doDelete(confirmDelete)}
          onCancel={() => setConfirmDelete(null)}
        />
      )}
    </div>
  )
}
