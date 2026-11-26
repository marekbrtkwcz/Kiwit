import { useCallback, useEffect, useState } from 'react'
import {
  ChevronDown,
  ChevronUp,
  Link2,
  Pencil,
  Plus,
  Search,
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
import { useWorkspace } from '../../context/WorkspaceContext'

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
  const [items, setItems] = useState<Requirement[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [q, setQ] = useState('')
  const [productId, setProductId] = useState(0)
  const [status, setStatus] = useState('')
  const [products, setProducts] = useState<Product[]>([])
  const [showForm, setShowForm] = useState(false)
  const [editing, setEditing] = useState<Requirement | null>(null)
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
      notify('Failed to load requirements', 'error')
    } finally {
      setLoading(false)
    }
  }, [q, productId, status, notify])

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
      notify('Failed to load coverage', 'error')
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
        notify('Requirement updated')
      } else {
        await api.createRequirement(body as Omit<Requirement, 'id' | 'created_at' | 'updated_at'>)
        notify('Requirement created')
      }
      setShowForm(false)
      await load()
    } catch {
      notify('Failed to save requirement', 'error')
    }
  }

  const doDelete = async (r: Requirement) => {
    try {
      await api.deleteRequirement(r.id)
      notifyAction('Requirement deleted', 'Undo', async () => {
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
      notify('Failed to delete requirement', 'error')
    }
  }

  const doLink = async () => {
    if (!expandedId || !linkCaseId) return
    try {
      await api.linkRequirementCase(expandedId, linkCaseId)
      notify('Test case linked')
      await loadCoverage(expandedId)
    } catch {
      notify('Failed to link test case', 'error')
    }
  }

  const doUnlink = async (caseId: number) => {
    if (!expandedId) return
    try {
      await api.unlinkRequirementCase(expandedId, caseId)
      notify('Test case unlinked')
      await loadCoverage(expandedId)
    } catch {
      notify('Failed to unlink test case', 'error')
    }
  }

  return (
    <div className="view">
      <div className="view-header">
        <h1>Requirements</h1>
        <button type="button" className="btn btn-primary" onClick={openCreate}>
          <Plus size={16} /> New Requirement
        </button>
      </div>

      <div className="filters" style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap', marginBottom: '0.75rem' }}>
        <div style={{ position: 'relative' }}>
          <Search size={14} style={{ position: 'absolute', left: '0.5rem', top: '50%', transform: 'translateY(-50%)', color: 'var(--muted)' }} />
          <input className="input" placeholder="Search key, name, description…" value={q} onChange={(e) => setQ(e.target.value)} style={{ paddingLeft: '1.7rem' }} />
        </div>
        <select className="select" value={productId} onChange={(e) => setProductId(Number(e.target.value))}>
          <option value={0}>All products</option>
          {products.map((p) => (
            <option key={p.id} value={p.id}>{p.name}</option>
          ))}
        </select>
        <select className="select" value={status} onChange={(e) => setStatus(e.target.value)}>
          <option value="">All statuses</option>
          {STATUSES.map((s) => (
            <option key={s} value={s}>{s}</option>
          ))}
        </select>
      </div>

      {loading ? (
        <LoadingSkeleton rows={6} />
      ) : items.length === 0 ? (
        <EmptyState
          title="No requirements yet"
          description="Requirements let you trace which test cases cover which business needs."
          actionLabel="New Requirement"
          onAction={openCreate}
        />
      ) : (
        <table className="table">
          <thead>
            <tr>
              <th>Key</th>
              <th>Name</th>
              <th>Version</th>
              <th>Status</th>
              <th>Priority</th>
              <th>Category</th>
              <th style={{ width: '7rem' }}>Actions</th>
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
                    <button type="button" className="btn btn-sm" title="Coverage" onClick={() => toggleExpand(r.id)}>
                      {expandedId === r.id ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
                    </button>
                    <button type="button" className="btn btn-sm" title="Edit" onClick={() => openEdit(r)}>
                      <Pencil size={14} />
                    </button>
                    <button type="button" className="btn btn-sm btn-danger" title="Delete" onClick={() => setConfirmDelete(r)}>
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
                          <strong>Coverage</strong>
                          <select className="select" value={linkCaseId} onChange={(e) => setLinkCaseId(Number(e.target.value))} style={{ padding: '0.2rem 0.45rem', fontSize: '0.8rem' }}>
                            {cases.map((c) => (
                              <option key={c.id} value={c.id}>#{c.id} {c.summary}</option>
                            ))}
                          </select>
                          <button type="button" className="btn btn-sm" onClick={doLink}>Link</button>
                        </div>
                        {coverage.length === 0 ? (
                          <div style={{ color: 'var(--muted)', fontSize: '0.85rem' }}>No test cases linked yet.</div>
                        ) : (
                          <table className="table">
                            <thead>
                              <tr><th>Test case</th><th>Case status</th><th>Latest execution</th><th>Run</th><th></th></tr>
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
      <div style={{ color: 'var(--muted)', fontSize: '0.8rem', marginTop: '0.5rem' }}>{total} requirement(s)</div>

      {showForm && (
        <div className="modal-backdrop" onClick={() => setShowForm(false)}>
          <div className="modal" onClick={(e) => e.stopPropagation()} role="dialog" aria-modal="true" aria-label="Requirement form">
            <h2>{editing ? 'Edit Requirement' : 'New Requirement'}</h2>
            <div style={{ display: 'grid', gap: '0.5rem' }}>
              <select className="select" value={form.product_id} onChange={(e) => setForm({ ...form, product_id: Number(e.target.value) })}>
                {products.map((p) => (
                  <option key={p.id} value={p.id}>{p.name}</option>
                ))}
              </select>
              <input className="input" placeholder="Key (e.g. REQ-001)" value={form.key} onChange={(e) => setForm({ ...form, key: e.target.value })} />
              <input className="input" placeholder="Name" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
              <textarea className="input" placeholder="Description" rows={3} value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })} />
              <div style={{ display: 'flex', gap: '0.5rem' }}>
                <input className="input" placeholder="Version" value={form.version} onChange={(e) => setForm({ ...form, version: e.target.value })} />
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
                <input className="input" placeholder="Category" value={form.category} onChange={(e) => setForm({ ...form, category: e.target.value })} />
              </div>
            </div>
            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.5rem', marginTop: '0.75rem' }}>
              <button type="button" className="btn" onClick={() => setShowForm(false)}>Cancel</button>
              <button type="button" className="btn btn-primary" onClick={submit} disabled={!form.key || !form.name || !form.product_id}>
                {editing ? 'Save' : 'Create'}
              </button>
            </div>
          </div>
        </div>
      )}

      {confirmDelete && (
        <ConfirmDialog
          open
          title="Delete requirement?"
          message={`"${confirmDelete.key} — ${confirmDelete.name}" and its coverage links will be removed.`}
          confirmLabel="Delete"
          onConfirm={() => void doDelete(confirmDelete)}
          onCancel={() => setConfirmDelete(null)}
        />
      )}
    </div>
  )
}
