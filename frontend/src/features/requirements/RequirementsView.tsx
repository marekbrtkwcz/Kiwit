import { useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  Bot,
  ChevronDown,
  ChevronUp,
  Crosshair,
  Link2,
  Pencil,
  Plus,
  Search,
  ShieldCheck,
  Trash2,
  X,
} from 'lucide-react'
import {
  api,
  CoverageEntry,
  Product,
  Requirement,
  TestCase,
} from '../../api/client'
import { ConfirmDialog, EmptyState, LoadingSkeleton } from '../../components/UiPrimitives'
import { ExportButton } from '../../components/ExportButton'
import { useWorkspace } from '../../context/WorkspaceContext'
import { useI18n } from '../../i18n'

const STATUSES = ['draft', 'approved', 'obsolete'] as const
const PRIORITIES = ['low', 'medium', 'high', 'critical']

interface FormState {
  product_id: number
  key: string
  name: string
  description: string
  version: string
  status: string
  priority: string
  category: string
}

type AiPanelState = { kind: 'quality' | 'regression'; req: Requirement; data: unknown } | 'busy' | null

const emptyForm = (productId: number): FormState => ({
  product_id: productId,
  key: '',
  name: '',
  description: '',
  version: '1',
  status: 'draft',
  priority: 'medium',
  category: '',
})

export function RequirementsView() {
  const { notify, notifyAction } = useWorkspace()
  const { t } = useI18n()
  const navigate = useNavigate()
  const [items, setItems] = useState<Requirement[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [q, setQ] = useState('')
  const [productId, setProductId] = useState(0)
  const [status, setStatus] = useState('')
  const [products, setProducts] = useState<Product[]>([])
  const [showForm, setShowForm] = useState(false)
  const [editing, setEditing] = useState<Requirement | null>(null)
  const [aiPanel, setAiPanel] = useState<AiPanelState>(null)
  const [form, setForm] = useState<FormState>(emptyForm(0))
  const [confirmDelete, setConfirmDelete] = useState<Requirement | null>(null)
  const [expandedId, setExpandedId] = useState<number | null>(null)
  const [coverage, setCoverage] = useState<CoverageEntry[]>([])
  const [cases, setCases] = useState<TestCase[]>([])
  const [linkCaseId, setLinkCaseId] = useState(0)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const [res, prods] = await Promise.all([
        api.listRequirements({ q, product_id: productId || undefined, status: status || undefined, limit: 200 }),
        api.listProducts(),
      ])
      setItems(res.items)
      setTotal(res.total)
      setProducts(prods.items)
    } catch {
      notify(t.requirements.loadFailed, 'error')
    } finally {
      setLoading(false)
    }
  }, [q, productId, status, notify, t])

  useEffect(() => {
    void load()
  }, [load])

  const loadCoverage = async (id: number) => {
    try {
      const [cov, tc] = await Promise.all([
        api.requirementCoverage(id),
        api.listTestCases({ product_id: items.find((r) => r.id === id)?.product_id, limit: 500 }),
      ])
      setCoverage(cov.items)
      setCases(tc.items)
      if (!linkCaseId && tc.items.length > 0) setLinkCaseId(tc.items[0].id)
    } catch {
      notify(t.requirements.coverageLoadFailed, 'error')
    }
  }

  const toggleExpand = (id: number) => {
    if (expandedId === id) {
      setExpandedId(null)
      return
    }
    setExpandedId(id)
    void loadCoverage(id)
  }

  const openCreate = () => {
    setEditing(null)
    setForm(emptyForm(productId || (products[0]?.id ?? 0)))
    setShowForm(true)
  }

  const openEdit = (r: Requirement) => {
    setEditing(r)
    setForm({
      product_id: r.product_id,
      key: r.key,
      name: r.name,
      description: r.description ?? '',
      version: r.version,
      status: r.status,
      priority: r.priority,
      category: r.category ?? '',
    })
    setShowForm(true)
  }

  const submit = async () => {
    try {
      const body = { ...form, product_id: Number(form.product_id) }
      if (editing) {
        await api.updateRequirement(editing.id, body as Omit<Requirement, 'id' | 'created_at' | 'updated_at'>)
        notify(t.requirements.updatedToast)
      } else {
        await api.createRequirement(body as Omit<Requirement, 'id' | 'created_at' | 'updated_at'>)
        notify(t.requirements.createdToast)
      }
      setShowForm(false)
      await load()
    } catch {
      notify(t.requirements.saveFailed, 'error')
    }
  }

  const doDelete = async (r: Requirement) => {
    try {
      await api.deleteRequirement(r.id)
      notifyAction(t.requirements.deletedToast, t.requirements.undo, async () => {
        await api.createRequirement({
          product_id: r.product_id,
          key: r.key,
          name: r.name,
          description: r.description,
          version: r.version,
          status: r.status,
          priority: r.priority,
          category: r.category,
        } as Omit<Requirement, 'id' | 'created_at' | 'updated_at'>)
        await load()
      })
      setConfirmDelete(null)
      await load()
    } catch {
      notify(t.requirements.deleteFailed, 'error')
    }
  }

  const doLink = async () => {
    if (!expandedId || !linkCaseId) return
    try {
      await api.linkRequirementCase(expandedId, linkCaseId)
      notify(t.requirements.caseLinked)
      await loadCoverage(expandedId)
    } catch {
      notify(t.requirements.caseLinkFailed, 'error')
    }
  }

  const doUnlink = async (caseId: number) => {
    if (!expandedId) return
    try {
      await api.unlinkRequirementCase(expandedId, caseId)
      notify(t.requirements.caseUnlinked)
      await loadCoverage(expandedId)
    } catch {
      notify(t.requirements.caseUnlinkFailed, 'error')
    }
  }

  return (
    <div className="view">
      <div className="view-header">
        <h1>{t.requirements.title}</h1>
        <button type="button" className="btn btn-primary" onClick={openCreate}>
          <Plus size={16} /> {t.requirements.newRequirement}
        </button>
      </div>

      <div className="filters" style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap', marginBottom: '0.75rem' }}>
        <div style={{ position: 'relative' }}>
          <Search size={14} style={{ position: 'absolute', left: '0.5rem', top: '50%', transform: 'translateY(-50%)', color: 'var(--muted)' }} />
          <input className="input" placeholder={t.requirements.searchPlaceholder} value={q} onChange={(e) => setQ(e.target.value)} style={{ paddingLeft: '1.7rem' }} />
        </div>
        <select className="select" value={productId} onChange={(e) => setProductId(Number(e.target.value))}>
          <option value={0}>{t.shell.allProducts}</option>
          {products.map((p) => (
            <option key={p.id} value={p.id}>{p.name}</option>
          ))}
        </select>
        <select className="select" value={status} onChange={(e) => setStatus(e.target.value)}>
          <option value="">{t.requirements.allStatuses}</option>
          {STATUSES.map((s) => (
            <option key={s} value={s}>{s}</option>
          ))}
        </select>
        <ExportButton
          path="/api/v1/requirements"
          filename="requirements"
          params={{ q: q || undefined, product_id: productId || undefined, status: status || undefined }}
          onError={(msg) => notify(msg, 'error')}
        />
      </div>

      {loading ? (
        <LoadingSkeleton rows={6} />
      ) : items.length === 0 ? (
        <EmptyState
          title={t.requirements.empty}
          description={t.requirements.emptyDescription}
          actionLabel={t.requirements.newRequirement}
          onAction={openCreate}
        />
      ) : (
        <table className="table">
          <thead>
            <tr>
              <th>{t.requirements.key}</th>
              <th>{t.common.name}</th>
              <th>{t.requirements.version}</th>
              <th>{t.common.status}</th>
              <th>{t.common.priority}</th>
              <th>{t.common.category}</th>
              <th style={{ width: '7rem' }}>{t.common.actions}</th>
            </tr>
          </thead>
          <tbody>
            {items.map((r) => (
              <>
                <tr key={r.id}>
                  <td><code>{r.key}</code></td>
                  <td>{r.name}</td>
                  <td>{r.version}</td>
                  <td><span className={`badge badge-${r.status === 'approved' ? 'passed' : r.status === 'obsolete' ? 'failed' : 'idle'}`}>{r.status}</span></td>
                  <td>{r.priority}</td>
                  <td>{r.category || '—'}</td>
                  <td style={{ display: 'flex', gap: '0.25rem' }}>
                    <button type="button" className="btn btn-sm" title={t.requirements.coverage} onClick={() => toggleExpand(r.id)}>
                      {expandedId === r.id ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
                    </button>
                    <button
                      type="button"
                      className="btn btn-sm"
                      title={t.requirements.aiGenerateHint}
                      onClick={async () => {
                        try {
                          const res = await api.aiGenerateTests({ product_id: r.product_id, requirement_id: r.id })
                          notify(t.requirements.aiGenerated(res.scenarios, res.model))
                          if (expandedId === r.id) await loadCoverage(r.id)
                        } catch (err) {
                          notify(err instanceof Error ? err.message : t.requirements.aiGenerateFailed, 'error')
                        }
                      }}
                    >
                      <Bot size={14} />
                    </button>
                    <button
                      type="button"
                      className="btn btn-sm"
                      title={t.requirements.aiQualityHint}
                      onClick={async () => {
                        setAiPanel('busy')
                        try {
                          const data = await api.aiAnalyzeRequirement(r.id)
                          setAiPanel({ kind: 'quality', req: r, data })
                          await load()
                        } catch (err) {
                          setAiPanel(null)
                          notify(err instanceof Error ? err.message : t.requirements.aiAnalysisFailed, 'error')
                        }
                      }}
                    >
                      <ShieldCheck size={14} />
                    </button>
                    <button
                      type="button"
                      className="btn btn-sm"
                      title={t.requirements.aiRegressionHint}
                      onClick={async () => {
                        setAiPanel('busy')
                        try {
                          const data = await api.aiRegressionScope(r.id)
                          setAiPanel({ kind: 'regression', req: r, data })
                        } catch (err) {
                          setAiPanel(null)
                          notify(err instanceof Error ? err.message : t.requirements.aiRegressionFailed, 'error')
                        }
                      }}
                    >
                      <Crosshair size={14} />
                    </button>
                    <button type="button" className="btn btn-sm" title={t.requirements.editAria} onClick={() => openEdit(r)}>
                      <Pencil size={14} />
                    </button>
                    <button type="button" className="btn btn-sm btn-danger" title={t.common.delete} onClick={() => setConfirmDelete(r)}>
                      <Trash2 size={14} />
                    </button>
                  </td>
                </tr>
                {expandedId === r.id && (
                  <tr key={`${r.id}-cov`}>
                    <td colSpan={7}>
                      <div style={{ padding: '0.5rem 0' }}>
                        <div style={{ display: 'flex', gap: '0.4rem', alignItems: 'center', marginBottom: '0.5rem', flexWrap: 'wrap' }}>
                          <Link2 size={14} />
                          <strong>{t.requirements.coverage}</strong>
                          <select className="select" value={linkCaseId} onChange={(e) => setLinkCaseId(Number(e.target.value))} style={{ padding: '0.2rem 0.45rem', fontSize: '0.8rem' }}>
                            {cases.map((c) => (
                              <option key={c.id} value={c.id}>#{c.id} {c.summary}</option>
                            ))}
                          </select>
                          <button type="button" className="btn btn-sm" onClick={doLink}>{t.requirements.link}</button>
                        </div>
                        {coverage.length === 0 ? (
                          <div style={{ color: 'var(--muted)', fontSize: '0.85rem' }}>{t.requirements.noCases}</div>
                        ) : (
                          <table className="table">
                            <thead>
                              <tr><th>{t.requirements.testCase}</th><th>{t.requirements.caseStatus}</th><th>{t.requirements.latestExecution}</th><th>{t.requirements.run}</th><th></th></tr>
                            </thead>
                            <tbody>
                              {coverage.map((c) => (
                                <tr key={c.test_case_id}>
                                  <td>#{c.test_case_id} {c.summary}</td>
                                  <td>{c.status}</td>
                                  <td>{c.latest_execution_status ? <span className={`badge badge-${c.latest_execution_status}`}>{c.latest_execution_status}</span> : '—'}</td>
                                  <td>{c.latest_run_id ? `#${c.latest_run_id}` : '—'}</td>
                                  <td>
                                    <button type="button" className="btn btn-sm btn-danger" onClick={() => doUnlink(c.test_case_id)}>
                                      <X size={14} />
                                    </button>
                                  </td>
                                </tr>
                              ))}
                            </tbody>
                          </table>
                        )}
                      </div>
                    </td>
                  </tr>
                )}
              </>
            ))}
          </tbody>
        </table>
      )}
      <div style={{ color: 'var(--muted)', fontSize: '0.8rem', marginTop: '0.5rem' }}>{t.requirements.countLabel(total)}</div>

      {showForm && (
        <div className="modal-backdrop" onClick={() => setShowForm(false)}>
          <div className="modal" onClick={(e) => e.stopPropagation()} role="dialog" aria-modal="true" aria-label={t.requirements.formAria}>
            <h2>{editing ? t.requirements.editRequirement : t.requirements.newRequirement}</h2>
            <div style={{ display: 'grid', gap: '0.5rem' }}>
              <select className="select" value={form.product_id} onChange={(e) => setForm({ ...form, product_id: Number(e.target.value) })}>
                {products.map((p) => (
                  <option key={p.id} value={p.id}>{p.name}</option>
                ))}
              </select>
              <input className="input" placeholder={t.requirements.keyPlaceholder} value={form.key} onChange={(e) => setForm({ ...form, key: e.target.value })} />
              <input className="input" placeholder={t.common.name} value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
              <textarea className="input" placeholder={t.common.description} rows={3} value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })} />
              <div style={{ display: 'flex', gap: '0.5rem' }}>
                <input className="input" placeholder={t.requirements.version} value={form.version} onChange={(e) => setForm({ ...form, version: e.target.value })} />
                <select className="select" value={form.status} onChange={(e) => setForm({ ...form, status: e.target.value })}>
                  {STATUSES.map((s) => (
                    <option key={s} value={s}>{s}</option>
                  ))}
                </select>
                <select className="select" value={form.priority} onChange={(e) => setForm({ ...form, priority: e.target.value })}>
                  {PRIORITIES.map((p) => (
                    <option key={p} value={p}>{p}</option>
                  ))}
                </select>
                <input className="input" placeholder={t.common.category} value={form.category} onChange={(e) => setForm({ ...form, category: e.target.value })} />
              </div>
            </div>
            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.5rem', marginTop: '0.75rem' }}>
              <button type="button" className="btn" onClick={() => setShowForm(false)}>{t.common.cancel}</button>
              <button type="button" className="btn btn-primary" onClick={submit} disabled={!form.key || !form.name || !form.product_id}>
                {editing ? t.common.save : t.requirements.create}
              </button>
            </div>
          </div>
        </div>
      )}

      {aiPanel === 'busy' && (
        <div className="modal-backdrop">
          <div className="modal" role="dialog" aria-modal="true">
            <p style={{ margin: 0 }}>{t.requirements.aiBusy}</p>
          </div>
        </div>
      )}

      {aiPanel && aiPanel !== 'busy' && (
        <div className="modal-backdrop" onClick={() => setAiPanel(null)}>
          <div className="modal" style={{ maxWidth: 720, maxHeight: '80vh', overflowY: 'auto' }} onClick={(e) => e.stopPropagation()} role="dialog" aria-modal="true" aria-label={t.requirements.aiResultAria}>
            {aiPanel.kind === 'quality' ? (() => {
              const d = aiPanel.data as { quality_score: number; verdict: string; summary: string; issues: { type: string; severity: string; description: string }[]; suggestions: string[]; model: string }
              return (
                <>
                  <h2>{t.requirements.aiQualityTitle(aiPanel.req.key)}</h2>
                  <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'center', margin: '0.5rem 0' }}>
                    <span className={`badge badge-${d.verdict === 'good' ? 'passed' : d.verdict === 'poor' ? 'failed' : 'idle'}`}>{d.verdict}</span>
                    <span className="badge">{d.quality_score}/100</span>
                    <span style={{ color: 'var(--muted)', fontSize: '0.8rem' }}>{d.model}</span>
                  </div>
                  <p style={{ fontSize: '0.9rem' }}>{d.summary}</p>
                  {d.issues.length > 0 && (
                    <div style={{ margin: '0.5rem 0' }}>
                      <strong style={{ fontSize: '0.85rem' }}>{t.requirements.problems}</strong>
                      <ul style={{ margin: '0.3rem 0', paddingLeft: '1.1rem', fontSize: '0.85rem' }}>
                        {d.issues.map((i, idx) => (
                          <li key={idx}><span className={`badge badge-${i.severity === 'high' ? 'failed' : i.severity === 'medium' ? 'idle' : 'passed'}`}>{i.type}/{i.severity}</span> {i.description}</li>
                        ))}
                      </ul>
                    </div>
                  )}
                  {d.suggestions.length > 0 && (
                    <div>
                      <strong style={{ fontSize: '0.85rem' }}>{t.requirements.suggestions}</strong>
                      <ul style={{ margin: '0.3rem 0', paddingLeft: '1.1rem', fontSize: '0.85rem' }}>
                        {d.suggestions.map((s, idx) => (<li key={idx}>{s}</li>))}
                      </ul>
                    </div>
                  )}
                </>
              )
            })() : (() => {
              const d = aiPanel.data as { summary: string; retest: { case_id: number; summary: string; reason: string; priority: string }[]; risks: string[]; gaps: string[]; model: string }
              return (
                <>
                  <h2>{t.requirements.aiRegressionTitle(aiPanel.req.key)}</h2>
                  <p style={{ fontSize: '0.9rem' }}>{d.summary}</p>
                  {d.retest.length > 0 && (
                    <table className="table" style={{ margin: '0.5rem 0' }}>
                      <thead><tr><th>{t.requirements.case}</th><th>{t.common.priority}</th><th>{t.requirements.reason}</th></tr></thead>
                      <tbody>
                        {d.retest.map((rt) => (
                          <tr key={rt.case_id}>
                            <td><button type="button" className="btn btn-sm" onClick={() => { setAiPanel(null); navigate(`/cases/${rt.case_id}`) }}>#{rt.case_id} {rt.summary}</button></td>
                            <td><span className={`badge badge-${rt.priority === 'high' ? 'failed' : rt.priority === 'medium' ? 'idle' : 'passed'}`}>{rt.priority}</span></td>
                            <td style={{ fontSize: '0.82rem' }}>{rt.reason}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  )}
                  {d.risks.length > 0 && (
                    <div><strong style={{ fontSize: '0.85rem' }}>{t.requirements.risks}</strong>
                      <ul style={{ margin: '0.3rem 0', paddingLeft: '1.1rem', fontSize: '0.85rem' }}>{d.risks.map((x, i) => (<li key={i}>{x}</li>))}</ul>
                    </div>
                  )}
                  {d.gaps.length > 0 && (
                    <div><strong style={{ fontSize: '0.85rem' }}>{t.requirements.gaps}</strong>
                      <ul style={{ margin: '0.3rem 0', paddingLeft: '1.1rem', fontSize: '0.85rem' }}>{d.gaps.map((x, i) => (<li key={i}>{x}</li>))}</ul>
                    </div>
                  )}
                  <div style={{ color: 'var(--muted)', fontSize: '0.75rem' }}>{d.model}</div>
                </>
              )
            })()}
            <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: '0.75rem' }}>
              <button type="button" className="btn" onClick={() => setAiPanel(null)}>{t.common.close}</button>
            </div>
          </div>
        </div>
      )}

      {confirmDelete && (
        <ConfirmDialog
          open
          title={t.requirements.confirmDeleteTitle}
          message={t.requirements.confirmDeleteMessage(confirmDelete.key, confirmDelete.name)}
          confirmLabel={t.common.delete}
          onConfirm={() => void doDelete(confirmDelete)}
          onCancel={() => setConfirmDelete(null)}
        />
      )}
    </div>
  )
}
