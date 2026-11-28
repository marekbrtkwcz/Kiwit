import { ChevronDown, ChevronRight, FolderTree, Plus, Trash2 } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
import { api, type Campaign, type CampaignTree, type Product, type TestPlan } from '../../api/client'
import { ConfirmDialog, EmptyState, LoadingSkeleton } from '../../components/UiPrimitives'
import { useWorkspace } from '../../context/WorkspaceContext'
import { useI18n } from '../../i18n'

export function CampaignsView() {
  const { t } = useI18n()
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
      notify(t.campaigns.loadFailed, 'error')
    } finally {
      setLoading(false)
    }
  }, [productId, notify, t])

  useEffect(() => {
    void load()
  }, [load])

  const loadTree = async (id: number) => {
    try {
      const tree = await api.campaignTree(id)
      setTrees((prev) => ({ ...prev, [id]: tree }))
    } catch {
      notify(t.campaigns.treeLoadFailed, 'error')
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
      notify(t.campaigns.createdToast)
      setShowForm(false)
      setName('')
      setDescription('')
      await load()
    } catch {
      notify(t.campaigns.createFailed, 'error')
    }
  }

  const addIteration = async (campaignId: number) => {
    const n = (newIter[campaignId] ?? '').trim()
    if (!n) return
    try {
      await api.createIteration(campaignId, { name: n })
      setNewIter((prev) => ({ ...prev, [campaignId]: '' }))
      notify(t.campaigns.iterationAdded)
      await loadTree(campaignId)
    } catch {
      notify(t.campaigns.iterationAddFailed, 'error')
    }
  }

  const assignPlan = async (campaignId: number, planId: number) => {
    try {
      await api.assignPlanToCampaign(campaignId, planId)
      notify(t.campaigns.planAssigned)
      await loadTree(campaignId)
    } catch {
      notify(t.campaigns.planAssignFailed, 'error')
    }
  }

  return (
    <div className="view">
      <div className="view-header">
        <h1>{t.campaigns.title}</h1>
        <button type="button" className="btn btn-primary" onClick={() => setShowForm(true)}>
          <Plus size={16} /> {t.campaigns.newCampaign}
        </button>
      </div>

      <div style={{ marginBottom: '0.75rem' }}>
        <select className="select" value={productId} onChange={(e) => setProductId(Number(e.target.value))}>
          <option value={0}>{t.shell.allProducts}</option>
          {products.map((p) => (
            <option key={p.id} value={p.id}>
              {p.name}
            </option>
          ))}
        </select>
      </div>

      {loading ? (
        <LoadingSkeleton rows={4} />
      ) : campaigns.length === 0 ? (
        <EmptyState
          icon={<FolderTree size={28} />}
          title={t.campaigns.empty}
          description={t.campaigns.emptyDescription}
          actionLabel={t.campaigns.newCampaign}
          onAction={() => setShowForm(true)}
        />
      ) : (
        <div style={{ display: 'grid', gap: '0.6rem' }}>
          {campaigns.map((c) => (
            <div key={c.id} className="card" style={{ display: 'grid', gap: '0.4rem' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                <button
                  type="button"
                  className="btn btn-sm"
                  onClick={() => toggle(c.id)}
                  aria-label={t.campaigns.toggleAria}
                >
                  {expanded[c.id] ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
                </button>
                <FolderTree size={15} />
                <strong>{c.name}</strong>
                <span
                  className={`badge badge-${c.status === 'active' ? 'running' : c.status === 'completed' ? 'passed' : 'idle'}`}
                >
                  {c.status}
                </span>
                <span style={{ marginLeft: 'auto' }}>
                  <button
                    type="button"
                    className="btn btn-sm btn-danger"
                    onClick={() => setConfirmDelete(c)}
                    aria-label={t.campaigns.deleteAria}
                  >
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
                        {t.campaigns.iterationLabel(it.name)} <span className="badge">{it.status}</span>
                      </div>
                      <ul style={{ margin: '0.2rem 0 0.4rem', paddingLeft: '1.2rem', fontSize: '0.82rem' }}>
                        {it.plans.map((p) => (
                          <li key={p.id}>
                            #{p.id} {p.name}
                          </li>
                        ))}
                        {it.plans.length === 0 && <li style={{ color: 'var(--muted)' }}>{t.campaigns.noPlans}</li>}
                      </ul>
                    </div>
                  ))}
                  {trees[c.id].plans.length > 0 && (
                    <div style={{ fontSize: '0.85rem' }}>
                      <strong>{t.campaigns.plansNoIteration}</strong>
                      <ul style={{ margin: '0.2rem 0', paddingLeft: '1.2rem', fontSize: '0.82rem' }}>
                        {trees[c.id].plans.map((p) => (
                          <li key={p.id}>
                            #{p.id} {p.name}
                          </li>
                        ))}
                      </ul>
                    </div>
                  )}
                  <div style={{ display: 'flex', gap: '0.35rem', alignItems: 'center', flexWrap: 'wrap' }}>
                    <input
                      className="input"
                      placeholder={t.campaigns.iterationNamePlaceholder}
                      value={newIter[c.id] ?? ''}
                      onChange={(e) => setNewIter((prev) => ({ ...prev, [c.id]: e.target.value }))}
                      style={{ width: 160, fontSize: '0.8rem' }}
                    />
                    <button type="button" className="btn btn-sm" onClick={() => addIteration(c.id)}>
                      {t.campaigns.addIteration}
                    </button>
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
                      <option value="">{t.campaigns.assignPlanPlaceholder}</option>
                      {plans.map((p) => (
                        <option key={p.id} value={p.id}>
                          #{p.id} {p.name}
                        </option>
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
          <div
            className="modal"
            onClick={(e) => e.stopPropagation()}
            role="dialog"
            aria-modal="true"
            aria-label={t.campaigns.formAria}
          >
            <h2>{t.campaigns.newCampaign}</h2>
            <div style={{ display: 'grid', gap: '0.5rem' }}>
              <select
                className="select"
                value={productId || (products[0]?.id ?? 0)}
                onChange={(e) => setProductId(Number(e.target.value))}
              >
                {products.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
                  </option>
                ))}
              </select>
              <input
                className="input"
                placeholder={t.campaigns.namePlaceholder}
                value={name}
                onChange={(e) => setName(e.target.value)}
              />
              <textarea
                className="input"
                rows={3}
                placeholder={t.common.description}
                value={description}
                onChange={(e) => setDescription(e.target.value)}
              />
            </div>
            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.5rem', marginTop: '0.75rem' }}>
              <button type="button" className="btn" onClick={() => setShowForm(false)}>
                {t.common.cancel}
              </button>
              <button type="button" className="btn btn-primary" onClick={create} disabled={!name.trim()}>
                {t.campaigns.create}
              </button>
            </div>
          </div>
        </div>
      )}

      {confirmDelete && (
        <ConfirmDialog
          open
          title={t.campaigns.confirmDeleteTitle}
          message={t.campaigns.confirmDeleteMessage(confirmDelete.name)}
          confirmLabel={t.common.delete}
          onConfirm={async () => {
            try {
              await api.deleteCampaign(confirmDelete.id)
              setConfirmDelete(null)
              notify(t.campaigns.deletedToast)
              await load()
            } catch {
              notify(t.campaigns.deleteFailed, 'error')
            }
          }}
          onCancel={() => setConfirmDelete(null)}
        />
      )}
    </div>
  )
}
