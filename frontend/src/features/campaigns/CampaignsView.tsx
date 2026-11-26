import { useCallback, useEffect, useState } from 'react'
import { ChevronDown, ChevronRight, FolderTree, Plus, Trash2 } from 'lucide-react'
import { api, Campaign, CampaignTree, Product, TestPlan } from '../../api/client'
import { ConfirmDialog, EmptyState, LoadingSkeleton } from '../../components/UiPrimitives'
import { useWorkspace } from '../../context/WorkspaceContext'

export function CampaignsView() {
  const { notify } = useWorkspace()
  const [campaigns, setCampaigns] = useState<Campaign[]>([])
  const [products, setProducts] = useState<Product[]>([])
  const [productId, setProductId] = useState(0)
  const [loading, setLoading] = useState(true)
  const [trees, setTrees] = useState<Record<number, CampaignTree>>({})
  const [expanded, setExpanded] = useState<Record<number, boolean>>({})
  const [showForm, setShowForm] = useState(false)
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [confirmDelete, setConfirmDelete] = useState<Campaign | null>(null)
  const [newIter, setNewIter] = useState<Record<number, string>>({})
  const [plans, setPlans] = useState<TestPlan[]>([])

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const [res, prods, pl] = await Promise.all([
        api.listCampaigns(productId || undefined),
        api.listProducts(),
        api.listTestPlans({}),
      ])
      setCampaigns(res.items ?? [])
      setProducts(prods.items ?? [])
      setPlans(pl.items ?? [])
    } catch {
      notify('Failed to load campaigns', 'error')
    } finally {
      setLoading(false)
    }
  }, [productId, notify])

  useEffect(() => {
    void load()
  }, [load])

  const loadTree = async (id: number) => {
    try {
      const tree = await api.campaignTree(id)
      setTrees((prev) => ({ ...prev, [id]: tree }))
    } catch {
      notify('Failed to load campaign tree', 'error')
    }
  }

  const toggle = (id: number) => {
    setExpanded((prev) => ({ ...prev, [id]: !prev[id] }))
    if (!trees[id]) void loadTree(id)
  }

  const create = async () => {
    if (!name.trim()) return
    try {
      await api.createCampaign({ product_id: productId || (products[0]?.id ?? 0), name: name.trim(), description })
      notify('Campaign created')
      setShowForm(false)
      setName('')
      setDescription('')
      await load()
    } catch {
      notify('Failed to create campaign', 'error')
    }
  }

  const addIteration = async (campaignId: number) => {
    const n = (newIter[campaignId] ?? '').trim()
    if (!n) return
    try {
      await api.createIteration(campaignId, { name: n })
      setNewIter((prev) => ({ ...prev, [campaignId]: '' }))
      notify('Iteration added')
      await loadTree(campaignId)
    } catch {
      notify('Failed to add iteration', 'error')
    }
  }

  const assignPlan = async (campaignId: number, planId: number) => {
    try {
      await api.assignPlanToCampaign(campaignId, planId)
      notify('Plan assigned')
      await loadTree(campaignId)
    } catch {
      notify('Failed to assign plan', 'error')
    }
  }

  return (
    <div className="view">
      <div className="view-header">
        <h1>Campaigns</h1>
        <button type="button" className="btn btn-primary" onClick={() => setShowForm(true)}>
          <Plus size={16} /> New Campaign
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
      ) : campaigns.length === 0 ? (
        <EmptyState
          icon={<FolderTree size={28} />}
          title="No campaigns yet"
          description="Campaigns group iterations, which group test plans (suites) — the SquashTM-style hierarchy."
          actionLabel="New Campaign"
          onAction={() => setShowForm(true)}
        />
      ) : (
        <div style={{ display: 'grid', gap: '0.6rem' }}>
          {campaigns.map((c) => (
            <div key={c.id} className="card" style={{ display: 'grid', gap: '0.4rem' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                <button type="button" className="btn btn-sm" onClick={() => toggle(c.id)} aria-label="Toggle campaign">
                  {expanded[c.id] ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
                </button>
                <FolderTree size={15} />
                <strong>{c.name}</strong>
                <span className={`badge badge-${c.status === 'active' ? 'running' : c.status === 'completed' ? 'passed' : 'idle'}`}>{c.status}</span>
                <span style={{ marginLeft: 'auto' }}>
                  <button type="button" className="btn btn-sm btn-danger" onClick={() => setConfirmDelete(c)} aria-label="Delete campaign">
                    <Trash2 size={13} />
                  </button>
                </span>
              </div>
              {c.description && <div style={{ fontSize: '0.82rem', color: 'var(--muted)' }}>{c.description}</div>}
              {expanded[c.id] && trees[c.id] && (
                <div style={{ paddingLeft: '1.4rem', display: 'grid', gap: '0.4rem' }}>
                  {trees[c.id].iterations.map((it) => (
                    <div key={it.id}>
                      <div style={{ fontSize: '0.85rem', fontWeight: 600 }}>
                        Iteration: {it.name} <span className="badge">{it.status}</span>
                      </div>
                      <ul style={{ margin: '0.2rem 0 0.4rem', paddingLeft: '1.2rem', fontSize: '0.82rem' }}>
                        {it.plans.map((p) => (
                          <li key={p.id}>#{p.id} {p.name}</li>
                        ))}
                        {it.plans.length === 0 && <li style={{ color: 'var(--muted)' }}>no plans</li>}
                      </ul>
                    </div>
                  ))}
                  {trees[c.id].plans.length > 0 && (
                    <div style={{ fontSize: '0.85rem' }}>
                      <strong>Plans (no iteration):</strong>
                      <ul style={{ margin: '0.2rem 0', paddingLeft: '1.2rem', fontSize: '0.82rem' }}>
                        {trees[c.id].plans.map((p) => (
                          <li key={p.id}>#{p.id} {p.name}</li>
                        ))}
                      </ul>
                    </div>
                  )}
                  <div style={{ display: 'flex', gap: '0.35rem', alignItems: 'center', flexWrap: 'wrap' }}>
                    <input
                      className="input"
                      placeholder="New iteration name"
                      value={newIter[c.id] ?? ''}
                      onChange={(e) => setNewIter((prev) => ({ ...prev, [c.id]: e.target.value }))}
                      style={{ width: 160, fontSize: '0.8rem' }}
                    />
                    <button type="button" className="btn btn-sm" onClick={() => addIteration(c.id)}>Add iteration</button>
                    <select
                      className="select"
                      defaultValue=""
                      style={{ fontSize: '0.8rem' }}
                      onChange={(e) => {
                        if (e.target.value) {
                          void assignPlan(c.id, Number(e.target.value))
                          e.target.value = ''
                        }
                      }}
                    >
                      <option value="">Assign test plan…</option>
                      {plans.map((p) => (
                        <option key={p.id} value={p.id}>#{p.id} {p.name}</option>
                      ))}
                    </select>
                  </div>
                </div>
              )}
            </div>
          ))}
        </div>
      )}

      {showForm && (
        <div className="modal-backdrop" onClick={() => setShowForm(false)}>
          <div className="modal" onClick={(e) => e.stopPropagation()} role="dialog" aria-modal="true" aria-label="Campaign form">
            <h2>New Campaign</h2>
            <div style={{ display: 'grid', gap: '0.5rem' }}>
              <select className="select" value={productId || (products[0]?.id ?? 0)} onChange={(e) => setProductId(Number(e.target.value))}>
                {products.map((p) => (
                  <option key={p.id} value={p.id}>{p.name}</option>
                ))}
              </select>
              <input className="input" placeholder="Name (e.g. Q1 Regression)" value={name} onChange={(e) => setName(e.target.value)} />
              <textarea className="input" rows={3} placeholder="Description" value={description} onChange={(e) => setDescription(e.target.value)} />
            </div>
            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.5rem', marginTop: '0.75rem' }}>
              <button type="button" className="btn" onClick={() => setShowForm(false)}>Cancel</button>
              <button type="button" className="btn btn-primary" onClick={create} disabled={!name.trim()}>Create</button>
            </div>
          </div>
        </div>
      )}

      {confirmDelete && (
        <ConfirmDialog
          open
          title="Delete campaign?"
          message={`"${confirmDelete.name}" and its iterations will be removed; test plans are kept.`}
          confirmLabel="Delete"
          onConfirm={async () => {
            try {
              await api.deleteCampaign(confirmDelete.id)
              setConfirmDelete(null)
              notify('Campaign deleted')
              await load()
            } catch {
              notify('Failed to delete campaign', 'error')
            }
          }}
          onCancel={() => setConfirmDelete(null)}
        />
      )}
    </div>
  )
}
