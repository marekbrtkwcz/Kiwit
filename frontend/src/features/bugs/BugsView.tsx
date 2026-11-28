import { Bot, Bug, Plus, Save, ScanSearch, Search, Trash2, X } from 'lucide-react'
import { useEffect, useState } from 'react'
import { api, type InternalBug } from '../../api/client'
import { ExportButton } from '../../components/ExportButton'
import { useWorkspace } from '../../context/WorkspaceContext'
import { useI18n } from '../../i18n'

export function BugsView() {
  const { t } = useI18n()
  const { products, activeProductId, notify } = useWorkspace()
  const [bugs, setBugs] = useState<InternalBug[]>([])
  const [filterQ, setFilterQ] = useState('')
  const [filterStatus, setFilterStatus] = useState('')
  const [filterSeverity, setFilterSeverity] = useState('')
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [editing, setEditing] = useState<InternalBug | null>(null)
  const [triage, setTriage] = useState<
    | {
        duplicates: { bug_id: number; confidence: number; reason: string }[]
        suggested_severity: string
        suggested_status: string
        triage_note: string
        model: string
      }
    | 'busy'
    | null
  >(null)

  const [productId, setProductId] = useState<number>(activeProductId ?? 1)
  const [summary, setSummary] = useState('')
  const [description, setDescription] = useState('')
  const [status, setStatus] = useState<InternalBug['status']>('open')
  const [severity, setSeverity] = useState<InternalBug['severity']>('medium')
  const [error, setError] = useState<string | null>(null)

  const load = async () => {
    try {
      const res = await api.listInternalBugs({
        product_id: activeProductId ?? undefined,
        status: filterStatus || undefined,
        severity: filterSeverity || undefined,
        q: filterQ || undefined,
      })
      setBugs(res.items)
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : t.bugs.loadFailed)
    }
  }

  useEffect(() => {
    void load()
  }, [activeProductId, filterQ, filterStatus, filterSeverity])

  const openNew = () => {
    setEditing(null)
    setProductId(activeProductId ?? products[0]?.id ?? 1)
    setSummary('')
    setDescription('')
    setStatus('open')
    setSeverity('medium')
    setDrawerOpen(true)
  }

  const openEdit = (b: InternalBug) => {
    setEditing(b)
    setProductId(b.product_id)
    setSummary(b.summary)
    setDescription(b.description ?? '')
    setStatus(b.status)
    setSeverity(b.severity)
    setDrawerOpen(true)
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!summary.trim()) return
    try {
      if (editing) {
        await api.updateInternalBug(editing.id, {
          summary: summary.trim(),
          description: description.trim(),
          status,
          severity,
        })
        notify(`${t.bugs.updatedToast} #${editing.id}`)
      } else {
        const created = await api.createInternalBug({
          product_id: productId,
          summary: summary.trim(),
          description: description.trim(),
          severity,
        })
        notify(`${t.bugs.createdToast} #${created.id}`)
      }
      setDrawerOpen(false)
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : t.bugs.saveFailed)
    }
  }

  const productMap = new Map(products.map((p) => [p.id, p.name]))

  return (
    <section>
      {error && <p className="alert-error">{error}</p>}

      <div className="filter-bar">
        <div style={{ position: 'relative', flex: 1, minWidth: 200 }}>
          <Search
            size={14}
            style={{ position: 'absolute', left: 10, top: '50%', transform: 'translateY(-50%)', color: 'var(--muted)' }}
          />
          <input
            className="input"
            placeholder={t.bugs.searchPlaceholder}
            value={filterQ}
            onChange={(e) => setFilterQ(e.target.value)}
            style={{ width: '100%', paddingLeft: 30 }}
          />
        </div>
        <select className="select" value={filterStatus} onChange={(e) => setFilterStatus(e.target.value)}>
          <option value="">{t.bugs.allStatuses}</option>
          <option value="open">open</option>
          <option value="in_progress">in_progress</option>
          <option value="resolved">resolved</option>
          <option value="closed">closed</option>
        </select>
        <select className="select" value={filterSeverity} onChange={(e) => setFilterSeverity(e.target.value)}>
          <option value="">{t.bugs.allSeverities}</option>
          <option value="critical">critical</option>
          <option value="high">high</option>
          <option value="medium">medium</option>
          <option value="low">low</option>
        </select>
        <ExportButton
          path="/api/v1/bugs"
          formats={['csv', 'json', 'md', 'xlsx']}
          filename="bugs"
          params={{
            product_id: activeProductId ?? undefined,
            status: filterStatus || undefined,
            severity: filterSeverity || undefined,
            q: filterQ || undefined,
          }}
          onError={(msg) => notify(msg, 'error')}
        />
        <button type="button" className="btn btn-primary" onClick={openNew}>
          <Plus size={15} /> {t.bugs.report}
        </button>
      </div>

      <div className="table-wrap">
        <table className="data-table">
          <thead>
            <tr>
              <th style={{ width: 70 }}>{t.common.id}</th>
              <th>{t.common.summary}</th>
              <th>{t.common.product}</th>
              <th>{t.common.severity}</th>
              <th>{t.common.status}</th>
              <th>{t.common.created}</th>
              <th style={{ textAlign: 'right' }}>{t.common.actions}</th>
            </tr>
          </thead>
          <tbody>
            {bugs.map((b) => (
              <tr key={b.id} className="clickable" onClick={() => openEdit(b)}>
                <td style={{ fontFamily: 'var(--font-mono)', color: 'var(--muted)' }}>#{b.id}</td>
                <td>
                  <div style={{ fontWeight: 500, display: 'flex', alignItems: 'center', gap: 6 }}>
                    <Bug size={14} color="var(--danger)" />
                    {b.summary}
                  </div>
                  {b.test_execution_id && (
                    <div style={{ fontSize: '0.75rem', color: 'var(--muted)' }}>
                      {t.bugs.executionLabel(b.test_execution_id)}
                    </div>
                  )}
                </td>
                <td style={{ fontSize: '0.8rem', color: 'var(--muted)' }}>
                  {productMap.get(b.product_id) ?? `#${b.product_id}`}
                </td>
                <td>
                  <span className={`badge pill-${b.severity}`}>{b.severity}</span>
                </td>
                <td>
                  <span
                    className={`badge ${b.status === 'closed' || b.status === 'resolved' ? 'pill-passed' : b.status === 'in_progress' ? 'pill-running' : 'pill-failed'}`}
                  >
                    {b.status}
                  </span>
                </td>
                <td style={{ fontSize: '0.78rem', color: 'var(--muted)' }}>
                  {new Date(b.created_at).toLocaleDateString()}
                </td>
                <td style={{ textAlign: 'right' }} onClick={(e) => e.stopPropagation()}>
                  <button
                    type="button"
                    className="btn btn-ghost btn-sm"
                    title={t.bugs.aiTriageHint}
                    onClick={async () => {
                      setTriage('busy')
                      try {
                        const res = await api.aiTriageBug(b.id)
                        setTriage(res)
                      } catch (err) {
                        setTriage(null)
                        notify(err instanceof Error ? err.message : t.bugs.aiTriageFailed, 'error')
                      }
                    }}
                  >
                    <ScanSearch size={14} />
                  </button>
                  <button
                    type="button"
                    className="btn btn-ghost btn-sm"
                    style={{ color: 'var(--danger)' }}
                    title={t.common.delete}
                    onClick={async () => {
                      if (!window.confirm(`${t.bugs.confirmDelete} #${b.id}?`)) return
                      await api.deleteInternalBug(b.id)
                      notify(`${t.bugs.deletedToast} #${b.id}`)
                      await load()
                    }}
                  >
                    <Trash2 size={14} />
                  </button>
                </td>
              </tr>
            ))}
            {bugs.length === 0 && (
              <tr>
                <td colSpan={7} style={{ textAlign: 'center', color: 'var(--muted)', padding: '2rem' }}>
                  {t.bugs.empty}
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {triage === 'busy' && (
        <div className="modal-backdrop">
          <div className="modal" role="dialog" aria-modal="true">
            <p style={{ margin: 0 }}>{t.bugs.aiTriageBusy}</p>
          </div>
        </div>
      )}

      {triage && triage !== 'busy' && (
        <div className="modal-backdrop" onClick={() => setTriage(null)}>
          <div
            className="modal"
            style={{ maxWidth: 640, maxHeight: '80vh', overflowY: 'auto' }}
            onClick={(e) => e.stopPropagation()}
            role="dialog"
            aria-modal="true"
            aria-label={t.bugs.aiTriageResultAria}
          >
            <h2 style={{ marginTop: 0 }}>{t.bugs.aiTriageTitle}</h2>
            <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'center', margin: '0.5rem 0', flexWrap: 'wrap' }}>
              <span className="badge">{t.bugs.suggestedSeverity(triage.suggested_severity)}</span>
              <span className="badge">{t.bugs.suggestedStatus(triage.suggested_status)}</span>
              <span style={{ color: 'var(--muted)', fontSize: '0.8rem' }}>{triage.model}</span>
            </div>
            <p style={{ fontSize: '0.9rem' }}>{triage.triage_note}</p>
            {triage.duplicates.length > 0 ? (
              <div>
                <strong style={{ fontSize: '0.85rem' }}>{t.bugs.possibleDuplicates}</strong>
                <table className="table" style={{ margin: '0.4rem 0' }}>
                  <thead>
                    <tr>
                      <th>{t.bugs.duplicatesBug}</th>
                      <th>{t.bugs.duplicatesConfidence}</th>
                      <th>{t.bugs.duplicatesReason}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {triage.duplicates.map((d) => (
                      <tr key={d.bug_id}>
                        <td>
                          <span className="badge">#{d.bug_id}</span>
                        </td>
                        <td>
                          <span
                            className={`badge badge-${d.confidence >= 70 ? 'failed' : d.confidence >= 40 ? 'idle' : 'passed'}`}
                          >
                            {d.confidence}%
                          </span>
                        </td>
                        <td style={{ fontSize: '0.82rem' }}>{d.reason}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : (
              <div style={{ color: 'var(--muted)', fontSize: '0.85rem' }}>{t.bugs.noDuplicates}</div>
            )}
            <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: '0.75rem' }}>
              <button type="button" className="btn" onClick={() => setTriage(null)}>
                {t.common.close}
              </button>
            </div>
          </div>
        </div>
      )}

      {drawerOpen && (
        <div className="drawer-backdrop" onClick={() => setDrawerOpen(false)}>
          <div className="drawer-panel" role="dialog" aria-modal="true" onClick={(e) => e.stopPropagation()}>
            <div className="drawer-header">
              <h3 style={{ margin: 0, fontSize: '1rem' }}>
                {editing ? `${t.bugs.title} #${editing.id}` : t.bugs.report}
              </h3>
              <button
                type="button"
                className="btn btn-ghost btn-icon"
                onClick={() => setDrawerOpen(false)}
                aria-label={t.common.close}
              >
                <X size={17} />
              </button>
            </div>
            <div className="drawer-body">
              <form onSubmit={handleSubmit} style={{ display: 'grid', gap: '0.85rem' }}>
                {!editing && (
                  <select className="select" value={productId} onChange={(e) => setProductId(Number(e.target.value))}>
                    {products.map((p) => (
                      <option key={p.id} value={p.id}>
                        {p.name}
                      </option>
                    ))}
                  </select>
                )}
                <button
                  type="button"
                  className="btn btn-sm"
                  title={t.bugs.aiSuggestHint}
                  onClick={async () => {
                    if (!summary.trim()) return
                    try {
                      const res = await api.aiSuggestBug({ summary, description: '' })
                      setSeverity(res.severity as InternalBug['severity'])
                      if (res.title && !summary.trim()) setSummary(res.title)
                      notify(t.bugs.aiSuggestToast(res.severity))
                    } catch (err) {
                      notify(err instanceof Error ? err.message : t.bugs.aiSuggestFailed, 'error')
                    }
                  }}
                >
                  <Bot size={13} />
                </button>
                <input
                  className="input"
                  placeholder={t.bugs.summaryPlaceholder}
                  value={summary}
                  onChange={(e) => setSummary(e.target.value)}
                  required
                />
                <div style={{ display: 'flex', gap: '0.5rem' }}>
                  <select
                    className="select"
                    value={severity}
                    onChange={(e) => setSeverity(e.target.value as InternalBug['severity'])}
                  >
                    <option value="low">low</option>
                    <option value="medium">medium</option>
                    <option value="high">high</option>
                    <option value="critical">critical</option>
                  </select>
                  {editing && (
                    <select
                      className="select"
                      value={status}
                      onChange={(e) => setStatus(e.target.value as InternalBug['status'])}
                    >
                      <option value="open">open</option>
                      <option value="in_progress">in_progress</option>
                      <option value="resolved">resolved</option>
                      <option value="closed">closed</option>
                    </select>
                  )}
                </div>
                <textarea
                  className="textarea"
                  rows={6}
                  placeholder={t.bugs.descPlaceholder}
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                />
                <div>
                  <button type="submit" className="btn btn-primary">
                    {editing ? (
                      <>
                        <Save size={14} /> {t.common.saveChanges}
                      </>
                    ) : (
                      <>
                        <Plus size={14} /> {t.bugs.create}
                      </>
                    )}
                  </button>
                </div>
              </form>
            </div>
          </div>
        </div>
      )}
    </section>
  )
}
