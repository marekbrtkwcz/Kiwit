import { useCallback, useEffect, useState } from 'react'
import {
  CheckCircle2,
  ChevronDown,
  ChevronUp,
  Flag,
  Pencil,
  Plus,
  Trash2,
  XCircle,
} from 'lucide-react'
import {
  api,
  Milestone,
  MilestoneProgress,
  Product,
  TestRun,
  Version,
} from '../../api/client'
import { ConfirmDialog, EmptyState, LoadingSkeleton } from '../../components/UiPrimitives'
import { useWorkspace } from '../../context/WorkspaceContext'

const STATUSES = ['planned', 'in_progress', 'done'] as const

export function MilestonesView() {
  const { notify } = useWorkspace()
  const [items, setItems] = useState<Milestone[]>([])
  const [loading, setLoading] = useState(true)
  const [products, setProducts] = useState<Product[]>([])
  const [productId, setProductId] = useState(0)
  const [versions, setVersions] = useState<Version[]>([])
  const [versionId, setVersionId] = useState(0)
  const [showForm, setShowForm] = useState(false)
  const [editing, setEditing] = useState<Milestone | null>(null)
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [dueDate, setDueDate] = useState('')
  const [status, setStatus] = useState('planned')
  const [confirmDelete, setConfirmDelete] = useState<Milestone | null>(null)
  const [expandedId, setExpandedId] = useState<number | null>(null)
  const [progress, setProgress] = useState<Record<number, MilestoneProgress>>({})
  const [linkedRuns, setLinkedRuns] = useState<number[]>([])
  const [runs, setRuns] = useState<TestRun[]>([])
  const [runId, setRunId] = useState(0)
  const [signoffNote, setSignoffNote] = useState('')

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const [res, prods] = await Promise.all([
        api.listMilestones(versionId || undefined),
        api.listProducts(),
      ])
      setItems(res.items)
      setProducts(prods.items)
    } catch {
      notify('Failed to load milestones', 'error')
    } finally {
      setLoading(false)
    }
  }, [versionId, notify])

  useEffect(() => {
    void load()
  }, [load])

  useEffect(() => {
    if (!productId && products.length > 0) setProductId(products[0].id)
  }, [products, productId])

  useEffect(() => {
    if (!productId) return
    api.listVersions(productId)
      .then((res) => setVersions(res.items))
      .catch(() => setVersions([]))
  }, [productId])

  const loadDetail = async (id: number) => {
    try {
      const [prog, runIds, allRuns] = await Promise.all([
        api.milestoneProgress(id),
        api.milestoneRuns(id),
        api.listTestRuns({ limit: 500 }),
      ])
      setProgress((prev) => ({ ...prev, [id]: prog }))
      setLinkedRuns(runIds.items)
      setRuns(allRuns.items)
      if (!runId && allRuns.items.length > 0) setRunId(allRuns.items[0].id)
    } catch {
      notify('Failed to load milestone detail', 'error')
    }
  }

  const toggleExpand = (id: number) => {
    if (expandedId === id) {
      setExpandedId(null)
      return
    }
    setExpandedId(id)
    void loadDetail(id)
  }

  const openCreate = () => {
    setEditing(null)
    setName('')
    setDescription('')
    setDueDate('')
    setStatus('planned')
    setShowForm(true)
  }

  const openEdit = (m: Milestone) => {
    setEditing(m)
    setName(m.name)
    setDescription(m.description ?? '')
    setDueDate(m.due_date ?? '')
    setStatus(m.status)
    setShowForm(true)
  }

  const submit = async () => {
    const vid = editing ? editing.version_id : versionId
    if (!vid || !name) return
    try {
      const body = { version_id: vid, name, description, due_date: dueDate || undefined, status }
      if (editing) {
        await api.updateMilestone(editing.id, body)
        notify('Milestone updated')
      } else {
        await api.createMilestone(body)
        notify('Milestone created')
      }
      setShowForm(false)
      await load()
    } catch {
      notify('Failed to save milestone', 'error')
    }
  }

  const doDelete = async (m: Milestone) => {
    try {
      await api.deleteMilestone(m.id)
      setConfirmDelete(null)
      notify('Milestone deleted')
      await load()
    } catch {
      notify('Failed to delete milestone', 'error')
    }
  }

  const doSignoff = async (m: Milestone, decision: 'go' | 'no-go') => {
    try {
      await api.milestoneSignoff(m.id, { decision, note: signoffNote })
      setSignoffNote('')
      notify(`Signoff recorded: ${decision.toUpperCase()}`)
      await load()
      if (expandedId === m.id) await loadDetail(m.id)
    } catch {
      notify('Failed to record signoff (admin only)', 'error')
    }
  }

  const doLink = async () => {
    if (!expandedId || !runId) return
    try {
      await api.milestoneLinkRun(expandedId, runId)
      notify('Run linked')
      await loadDetail(expandedId)
    } catch {
      notify('Failed to link run', 'error')
    }
  }

  const doUnlink = async (id: number, rid: number) => {
    try {
      await api.milestoneUnlinkRun(id, rid)
      notify('Run unlinked')
      await loadDetail(id)
    } catch {
      notify('Failed to unlink run', 'error')
    }
  }

  return (
    <div className="view">
      <div className="view-header">
        <h1>Milestones &amp; Releases</h1>
        <button type="button" className="btn btn-primary" onClick={openCreate}>
          <Plus size={16} /> New Milestone
        </button>
      </div>

      <div style={{ display: 'flex', gap: '0.5rem', marginBottom: '0.75rem', flexWrap: 'wrap' }}>
        <select className="select" value={productId} onChange={(e) => { setProductId(Number(e.target.value)); setVersionId(0) }}>
          {products.map((p) => (
            <option key={p.id} value={p.id}>{p.name}</option>
          ))}
        </select>
        <select className="select" value={versionId} onChange={(e) => setVersionId(Number(e.target.value))}>
          <option value={0}>All versions</option>
          {versions.map((v) => (
            <option key={v.id} value={v.id}>{v.value}</option>
          ))}
        </select>
      </div>

      {loading ? (
        <LoadingSkeleton rows={5} />
      ) : items.length === 0 ? (
        <EmptyState
          title="No milestones yet"
          description="Milestones group test runs under a release and carry the go/no-go signoff."
          actionLabel="New Milestone"
          onAction={openCreate}
        />
      ) : (
        <table className="table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Due</th>
              <th>Status</th>
              <th>Progress</th>
              <th>Signoff</th>
              <th style={{ width: '8rem' }}>Actions</th>
            </tr>
          </thead>
          <tbody>
            {items.map((m) => (
              <>
                <tr key={m.id}>
                  <td>
                    <button type="button" className="btn btn-sm" onClick={() => toggleExpand(m.id)} style={{ marginRight: '0.25rem' }}>
                      {expandedId === m.id ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
                    </button>
                    {m.name}
                  </td>
                  <td>{m.due_date || '—'}</td>
                  <td><span className={`badge badge-${m.status === 'done' ? 'passed' : m.status === 'in_progress' ? 'running' : 'idle'}`}>{m.status}</span></td>
                  <td>{progress[m.id] ? progress[m.id].percent_done : '—'}</td>
                  <td>
                    {m.signoff_decision ? (
                      <span className={`badge badge-${m.signoff_decision === 'go' ? 'passed' : 'failed'}`}>
                        {m.signoff_decision === 'go' ? <CheckCircle2 size={12} /> : <XCircle size={12} />} {m.signoff_decision.toUpperCase()}
                      </span>
                    ) : '—'}
                  </td>
                  <td style={{ display: 'flex', gap: '0.25rem' }}>
                    <button type="button" className="btn btn-sm" title="Edit" onClick={() => openEdit(m)}>
                      <Pencil size={14} />
                    </button>
                    <button type="button" className="btn btn-sm btn-danger" title="Delete" onClick={() => setConfirmDelete(m)}>
                      <Trash2 size={14} />
                    </button>
                  </td>
                </tr>
                {expandedId === m.id && (
                  <tr key={`${m.id}-detail`}>
                    <td colSpan={6}>
                      <div style={{ padding: '0.5rem 0', display: 'grid', gap: '0.5rem' }}>
                        {progress[m.id] && (
                          <div style={{ display: 'flex', gap: '0.75rem', flexWrap: 'wrap', fontSize: '0.85rem' }}>
                            <span>Total: <strong>{progress[m.id].total}</strong></span>
                            <span className="badge badge-passed">passed {progress[m.id].passed}</span>
                            <span className="badge badge-failed">failed {progress[m.id].failed}</span>
                            <span className="badge badge-blocked">blocked {progress[m.id].blocked}</span>
                            <span className="badge badge-skipped">skipped {progress[m.id].skipped}</span>
                            <span className="badge badge-idle">idle {progress[m.id].idle}</span>
                            <strong>{progress[m.id].percent_done}</strong>
                          </div>
                        )}
                        <div style={{ display: 'flex', gap: '0.4rem', alignItems: 'center', flexWrap: 'wrap' }}>
                          <Flag size={14} />
                          <strong>Linked runs:</strong>
                          {linkedRuns.length === 0 && <span style={{ color: 'var(--muted)' }}>none</span>}
                          {linkedRuns.map((rid) => (
                            <span key={rid} className="badge" style={{ display: 'inline-flex', gap: '0.25rem', alignItems: 'center' }}>
                              Run #{rid}
                              <button type="button" className="btn btn-sm" onClick={() => doUnlink(m.id, rid)} aria-label={`Unlink run ${rid}`}>
                                <XCircle size={12} />
                              </button>
                            </span>
                          ))}
                          <select className="select" value={runId} onChange={(e) => setRunId(Number(e.target.value))} style={{ padding: '0.2rem 0.45rem', fontSize: '0.8rem' }}>
                            {runs.map((r) => (
                              <option key={r.id} value={r.id}>#{r.id} {r.summary}</option>
                            ))}
                          </select>
                          <button type="button" className="btn btn-sm" onClick={doLink}>Link run</button>
                        </div>
                        <div style={{ display: 'flex', gap: '0.4rem', alignItems: 'center', flexWrap: 'wrap' }}>
                          <strong>Go/No-Go:</strong>
                          <input className="input" placeholder="Note (optional)" value={signoffNote} onChange={(e) => setSignoffNote(e.target.value)} style={{ maxWidth: '16rem' }} />
                          <button type="button" className="btn btn-sm btn-primary" onClick={() => doSignoff(m, 'go')}>
                            <CheckCircle2 size={14} /> GO
                          </button>
                          <button type="button" className="btn btn-sm btn-danger" onClick={() => doSignoff(m, 'no-go')}>
                            <XCircle size={14} /> NO-GO
                          </button>
                        </div>
                      </div>
                    </td>
                  </tr>
                )}
              </>
            ))}
          </tbody>
        </table>
      )}

      {showForm && (
        <div className="modal-backdrop" onClick={() => setShowForm(false)}>
          <div className="modal" onClick={(e) => e.stopPropagation()} role="dialog" aria-modal="true" aria-label="Milestone form">
            <h2>{editing ? 'Edit Milestone' : 'New Milestone'}</h2>
            <div style={{ display: 'grid', gap: '0.5rem' }}>
              {!editing && (
                <select className="select" value={versionId} onChange={(e) => setVersionId(Number(e.target.value))}>
                  {versions.map((v) => (
                    <option key={v.id} value={v.id}>{v.value}</option>
                  ))}
                </select>
              )}
              <input className="input" placeholder="Name (e.g. Release 1.2)" value={name} onChange={(e) => setName(e.target.value)} />
              <textarea className="input" placeholder="Description" rows={3} value={description} onChange={(e) => setDescription(e.target.value)} />
              <div style={{ display: 'flex', gap: '0.5rem' }}>
                <input className="input" type="date" value={dueDate} onChange={(e) => setDueDate(e.target.value)} />
                <select className="select" value={status} onChange={(e) => setStatus(e.target.value)}>
                  {STATUSES.map((s) => (
                    <option key={s} value={s}>{s}</option>
                  ))}
                </select>
              </div>
            </div>
            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.5rem', marginTop: '0.75rem' }}>
              <button type="button" className="btn" onClick={() => setShowForm(false)}>Cancel</button>
              <button type="button" className="btn btn-primary" onClick={submit} disabled={!name || (!editing && !versionId)}>
                {editing ? 'Save' : 'Create'}
              </button>
            </div>
          </div>
        </div>
      )}

      {confirmDelete && (
        <ConfirmDialog
          open
          title="Delete milestone?"
          message={`"${confirmDelete.name}" will be removed.`}
          confirmLabel="Delete"
          onConfirm={() => void doDelete(confirmDelete)}
          onCancel={() => setConfirmDelete(null)}
        />
      )}
    </div>
  )
}
