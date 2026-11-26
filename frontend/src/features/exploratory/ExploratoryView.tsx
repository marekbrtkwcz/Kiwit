import { useCallback, useEffect, useState } from 'react'
import { Compass, Pencil, Plus, Sparkles, Trash2 } from 'lucide-react'
import { api, ExploratorySession, Product } from '../../api/client'
import { ConfirmDialog, EmptyState, LoadingSkeleton } from '../../components/UiPrimitives'
import { useWorkspace } from '../../context/WorkspaceContext'
import { useI18n } from '../../i18n'

interface FormState {
  product_id: number
  charter: string
  timebox_minutes: number
  notes: string
  findings: string
}

export function ExploratoryView() {
  const { t } = useI18n()
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
      notify(t.exploratory.loadFailed, 'error')
    } finally {
      setLoading(false)
    }
  }, [productId, notify, t])

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
        notify(t.exploratory.updatedToast)
      } else {
        await api.createExploratory(body)
        notify(t.exploratory.startedToast)
      }
      setShowForm(false)
      await load()
    } catch {
      notify(t.exploratory.saveFailed, 'error')
    }
  }

  const doDelete = async (s: ExploratorySession) => {
    try {
      await api.deleteExploratory(s.id)
      setConfirmDelete(null)
      notify(t.exploratory.deletedToast)
      await load()
    } catch {
      notify(t.exploratory.deleteFailed, 'error')
    }
  }

  return (
    <div className="view">
      <div className="view-header">
        <h1>{t.exploratory.title}</h1>
        <button type="button" className="btn btn-primary" onClick={openCreate}>
          <Plus size={16} /> {t.exploratory.newSession}
        </button>
      </div>

      <div style={{ marginBottom: '0.75rem' }}>
        <select className="select" value={productId} onChange={(e) => setProductId(Number(e.target.value))}>
          <option value={0}>{t.shell.allProducts}</option>
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
          title={t.exploratory.empty}
          description={t.exploratory.emptyDescription}
          actionLabel={t.exploratory.newSession}
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
                  <span className="badge">{t.exploratory.minutesBadge(s.timebox_minutes)}</span>
                  {s.test_run_id ? <span className="badge">{t.exploratory.runBadge(s.test_run_id)}</span> : null}
                  <span style={{ marginLeft: 'auto', display: 'flex', gap: '0.25rem' }}>
                    <button type="button" className="btn btn-sm" onClick={() => openEdit(s)} aria-label={t.exploratory.editAria}><Pencil size={13} /></button>
                    <button type="button" className="btn btn-sm btn-danger" onClick={() => setConfirmDelete(s)} aria-label={t.exploratory.deleteAria}><Trash2 size={13} /></button>
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
          <div className="modal" onClick={(e) => e.stopPropagation()} role="dialog" aria-modal="true" aria-label={t.exploratory.formAria}>
            <h2>{editing ? t.exploratory.editSession : t.exploratory.newSessionTitle}</h2>
            <div style={{ display: 'grid', gap: '0.5rem' }}>
              <select className="select" value={form.product_id} onChange={(e) => setForm({ ...form, product_id: Number(e.target.value) })}>
                {products.map((p) => (
                  <option key={p.id} value={p.id}>{p.name}</option>
                ))}
              </select>
              <div style={{ display: 'flex', gap: '0.4rem', alignItems: 'center' }}>
                <input style={{ flex: 1 }} className="input" placeholder={t.exploratory.charterPlaceholder} value={form.charter} onChange={(e) => setForm({ ...form, charter: e.target.value })} />
                <button
                  type="button"
                  className="btn btn-sm"
                  disabled={aiBusy === 'charter'}
                  title={t.exploratory.aiCharterHint}
                  onClick={async () => {
                    const objective = form.charter.trim() || window.prompt(t.exploratory.aiObjectivePrompt) || ''
                    if (!objective.trim()) return
                    setAiBusy('charter')
                    try {
                      const res = await api.aiGenerateCharter(objective)
                      setForm((f) => ({ ...f, charter: res.charter }))
                    } catch (err) {
                      notify(err instanceof Error ? err.message : t.exploratory.aiCharterFailed, 'error')
                    } finally {
                      setAiBusy('')
                    }
                  }}
                >
                  <Sparkles size={13} /> {aiBusy === 'charter' ? '…' : t.exploratory.aiCharter}
                </button>
              </div>
              <input className="input" type="number" min={5} step={5} value={form.timebox_minutes} onChange={(e) => setForm({ ...form, timebox_minutes: Number(e.target.value) })} />
              <textarea className="input" rows={4} placeholder={t.exploratory.notesPlaceholder} value={form.notes} onChange={(e) => setForm({ ...form, notes: e.target.value })} />
              <div>
                <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: '0.3rem' }}>
                  <button
                    type="button"
                    className="btn btn-sm"
                    disabled={aiBusy === 'findings' || !editing}
                    title={editing ? t.exploratory.aiFindingsHint : t.exploratory.aiFindingsHintDisabled}
                    onClick={async () => {
                      if (!editing) return
                      setAiBusy('findings')
                      try {
                        const res = await api.aiNotesToFindings(editing.id)
                        setForm((f) => ({ ...f, findings: JSON.stringify(res.findings, null, 2) }))
                      } catch (err) {
                        notify(err instanceof Error ? err.message : t.exploratory.aiFindingsFailed, 'error')
                      } finally {
                        setAiBusy('')
                      }
                    }}
                  >
                    <Sparkles size={13} /> {aiBusy === 'findings' ? '…' : t.exploratory.aiFindings}
                  </button>
                </div>
                <textarea className="input" rows={3} placeholder={t.exploratory.findingsPlaceholder} value={form.findings} onChange={(e) => setForm({ ...form, findings: e.target.value })} />
              </div>
            </div>
            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.5rem', marginTop: '0.75rem' }}>
              <button type="button" className="btn" onClick={() => setShowForm(false)}>{t.common.cancel}</button>
              <button type="button" className="btn btn-primary" onClick={submit} disabled={!form.charter || !form.product_id}>
                {editing ? t.common.save : t.exploratory.start}
              </button>
            </div>
          </div>
        </div>
      )}

      {confirmDelete && (
        <ConfirmDialog
          open
          title={t.exploratory.confirmDeleteTitle}
          message={t.exploratory.confirmDeleteMessage(confirmDelete.charter)}
          confirmLabel={t.common.delete}
          onConfirm={() => void doDelete(confirmDelete)}
          onCancel={() => setConfirmDelete(null)}
        />
      )}
    </div>
  )
}
