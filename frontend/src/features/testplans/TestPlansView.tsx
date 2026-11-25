import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { ChevronDown, ChevronUp, Copy, ExternalLink, FolderKanban, Plus, Search, Trash2, Unlink } from 'lucide-react'
import { api, PlanCaseItem, TestCase, TestPlan, Version } from '../../api/client'
import { useWorkspace } from '../../context/WorkspaceContext'
import { useI18n } from '../../i18n'
import { EntityTagsBar } from '../management/EntityTagsBar'

export function TestPlansView() {
  const { t } = useI18n()
  const { products, activeProductId, notify } = useWorkspace()
  const { id: routePlanId } = useParams<{ id: string }>()
  const navigate = useNavigate()

  const [versions, setVersions] = useState<Version[]>([])
  const [plans, setPlans] = useState<TestPlan[]>([])
  const [cases, setCases] = useState<TestCase[]>([])
  const [attachedCases, setAttachedCases] = useState<PlanCaseItem[]>([])
  const [productId, setProductId] = useState<number>(activeProductId ?? 1)
  const [versionId, setVersionId] = useState<number>(0)
  const [selectedPlanId, setSelectedPlanId] = useState<number>(routePlanId ? Number(routePlanId) : 0)
  const [parentId, setParentId] = useState<number | undefined>(undefined)
  const [selectedCaseIds, setSelectedCaseIds] = useState<number[]>([])
  const [name, setName] = useState('')
  const [filterQ, setFilterQ] = useState('')
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (activeProductId) setProductId(activeProductId)
    else if (products.length > 0 && !products.some((p) => p.id === productId)) setProductId(products[0].id)
  }, [activeProductId, products])

  const load = async () => {
    try {
      const [tpRes, tcRes] = await Promise.all([
        api.listTestPlans({ product_id: activeProductId ?? undefined, q: filterQ || undefined }),
        api.listTestCases({ product_id: activeProductId ?? undefined, limit: 200 }),
      ])
      setPlans(tpRes.items)
      setCases(tcRes.items)
      if (routePlanId) {
        setSelectedPlanId(Number(routePlanId))
      } else if (tpRes.items.length > 0 && (!selectedPlanId || !tpRes.items.some((p) => p.id === selectedPlanId))) {
        setSelectedPlanId(tpRes.items[0].id)
      }
      if (productId > 0) {
        const vRes = await api.listVersions(productId)
        setVersions(vRes.items)
        if (vRes.items.length > 0 && !vRes.items.some((v) => v.id === versionId)) {
          setVersionId(vRes.items[0].id)
        }
      }
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : t.errors.loadTestPlans)
    }
  }

  const loadAttached = async (pid: number) => {
    try {
      const res = await api.listTestPlanCases(pid)
      setAttachedCases(res.items)
    } catch {
      setAttachedCases([])
    }
  }

  useEffect(() => {
    void load()
  }, [activeProductId, productId, filterQ])

  useEffect(() => {
    if (selectedPlanId > 0) void loadAttached(selectedPlanId)
    else setAttachedCases([])
  }, [selectedPlanId])

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim() || !versionId) return
    try {
      const created = await api.createTestPlan({ product_id: productId, version_id: versionId, parent_id: parentId, name: name.trim() })
      setName('')
      setParentId(undefined)
      setSelectedPlanId(created.id)
      navigate(`/plans/${created.id}`, { replace: true })
      notify(`Created TestPlan #${created.id}`)
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errors.createTestPlan)
    }
  }

  const toggleCase = (id: number) => {
    setSelectedCaseIds((prev) => (prev.includes(id) ? prev.filter((c) => c !== id) : [...prev, id]))
  }

  const handleAttach = async () => {
    if (!selectedPlanId || selectedCaseIds.length === 0) return
    try {
      const res = await api.addTestPlanCases(selectedPlanId, selectedCaseIds)
      setSelectedCaseIds([])
      notify(`Attached ${res.added} case(s) to Plan #${selectedPlanId}`)
      await loadAttached(selectedPlanId)
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errors.createTestPlan)
    }
  }

  const attachedSet = new Set(attachedCases.map((c) => c.test_case_id))
  const selectedPlan = plans.find((p) => p.id === selectedPlanId) ?? null

  return (
    <section style={{ display: 'grid', gap: '1rem' }}>
      {error && <p className="alert-error">{error}</p>}

      <form onSubmit={handleCreate} className="filter-bar" style={{ marginBottom: 0 }}>
        <select className="select" value={productId} onChange={(e) => setProductId(Number(e.target.value))}>
          {products.map((p) => (
            <option key={p.id} value={p.id}>{p.name}</option>
          ))}
        </select>
        <select className="select" value={versionId} onChange={(e) => setVersionId(Number(e.target.value))}>
          {versions.length === 0 && <option value={0}>{t.testplans.noVersions}</option>}
          {versions.map((v) => (
            <option key={v.id} value={v.id}>{v.value}</option>
          ))}
        </select>
        <select className="select" value={parentId ?? ''} onChange={(e) => setParentId(e.target.value ? Number(e.target.value) : undefined)}>
          <option value="">{t.testplans.rootPlan}</option>
          {plans.map((p) => (
            <option key={p.id} value={p.id}>Parent: {p.name}</option>
          ))}
        </select>
        <input className="input" placeholder={t.testplans.namePlaceholder} value={name} onChange={(e) => setName(e.target.value)} style={{ flex: 1, minWidth: 200 }} />
        <button type="submit" disabled={!versionId} className="btn btn-primary">
          <Plus size={15} /> {t.testplans.create}
        </button>
      </form>

      <div className="split-layout">
        <div className="card" style={{ padding: '0.75rem', display: 'grid', gap: '0.45rem' }}>
          <div style={{ position: 'relative' }}>
            <Search size={14} style={{ position: 'absolute', left: 10, top: '50%', transform: 'translateY(-50%)', color: 'var(--muted)' }} />
            <input className="input" placeholder={t.testplans.filterPlaceholder} value={filterQ} onChange={(e) => setFilterQ(e.target.value)} style={{ width: '100%', paddingLeft: 30 }} />
          </div>
          {plans.map((tp) => (
            <div
              key={tp.id}
              onClick={() => {
                setSelectedPlanId(tp.id)
                navigate(`/plans/${tp.id}`, { replace: true })
              }}
              style={{
                padding: '0.6rem 0.7rem',
                marginLeft: tp.parent_id ? 14 : 0,
                borderRadius: 8,
                border: selectedPlanId === tp.id ? '1px solid var(--primary)' : '1px solid var(--border)',
                background: selectedPlanId === tp.id ? 'var(--primary-soft)' : 'var(--surface)',
                cursor: 'pointer',
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
                gap: '0.4rem',
              }}
            >
              <div style={{ minWidth: 0 }}>
                <div style={{ fontWeight: 600, fontSize: '0.86rem', display: 'flex', alignItems: 'center', gap: 5 }}>
                  <FolderKanban size={14} /> {tp.name}
                </div>
                <div style={{ fontSize: '0.74rem', color: 'var(--muted)' }}>
                  #{tp.id} · v#{tp.version_id}{tp.parent_id ? ` · parent #${tp.parent_id}` : ''}
                </div>
              </div>
              <div style={{ display: 'inline-flex', gap: '0.2rem' }} onClick={(e) => e.stopPropagation()}>
                <button
                  type="button"
                  className="btn btn-ghost btn-sm"
                  title="Clone"
                  onClick={async () => {
                    await api.cloneTestPlan(tp.id, `${tp.name} (copy)`)
                    notify(`Cloned plan #${tp.id}`)
                    await load()
                  }}
                >
                  <Copy size={13} />
                </button>
                <button
                  type="button"
                  className="btn btn-ghost btn-sm"
                  style={{ color: 'var(--danger)' }}
                  title="Delete"
                  onClick={async () => {
                    if (!window.confirm(`Delete TestPlan #${tp.id}?`)) return
                    await api.deleteTestPlan(tp.id)
                    if (selectedPlanId === tp.id) setSelectedPlanId(0)
                    notify(`Deleted plan #${tp.id}`)
                    await load()
                  }}
                >
                  <Trash2 size={13} />
                </button>
              </div>
            </div>
          ))}
          {plans.length === 0 && <p style={{ color: 'var(--muted)', margin: '0.5rem', fontSize: '0.84rem' }}>{t.testplans.empty}</p>}
        </div>

        {selectedPlan ? (
          <div className="card" style={{ display: 'grid', gap: '1rem' }}>
            <div>
              <h3 style={{ margin: '0 0 0.45rem' }}>
                #{selectedPlan.id} {selectedPlan.name} — {t.testplans.attachedCases} ({attachedCases.length})
              </h3>
              <EntityTagsBar entity="testplans" entityId={selectedPlan.id} />
              {attachedCases.length === 0 ? (
                <p style={{ color: 'var(--muted)', margin: '0.6rem 0 0', fontSize: '0.84rem' }}>{t.testplans.noCasesAttached}</p>
              ) : (
                <div className="table-wrap" style={{ marginTop: '0.65rem' }}>
                  <table className="data-table">
                    <thead>
                      <tr>
                        <th>{t.common.id}</th>
                        <th>{t.common.summary}</th>
                        <th>{t.common.priority}</th>
                        <th>{t.common.status}</th>
                        <th />
                      </tr>
                    </thead>
                    <tbody>
                      {attachedCases.map((ac, idx) => (
                        <tr key={ac.test_case_id} className="clickable" onClick={() => navigate(`/cases/${ac.test_case_id}`)}>
                          <td style={{ fontFamily: 'var(--font-mono)', color: 'var(--muted)' }}>#{ac.test_case_id}</td>
                          <td>{ac.summary}</td>
                          <td><span className={`badge pill-${ac.priority}`}>{ac.priority}</span></td>
                          <td><span className={`badge pill-${ac.status}`}>{ac.status}</span></td>
                          <td style={{ textAlign: 'right' }} onClick={(e) => e.stopPropagation()}>
                            <div style={{ display: 'inline-flex', gap: '0.2rem', alignItems: 'center' }}>
                              <button
                                type="button"
                                className="btn btn-ghost btn-sm"
                                disabled={idx === 0}
                                title="Move up"
                                onClick={async () => {
                                  const next = [...attachedCases]
                                  ;[next[idx - 1], next[idx]] = [next[idx], next[idx - 1]]
                                  setAttachedCases(next)
                                  await api.reorderTestPlanCases(selectedPlan.id, next.map((x) => x.test_case_id))
                                }}
                              >
                                <ChevronUp size={13} />
                              </button>
                              <button
                                type="button"
                                className="btn btn-ghost btn-sm"
                                disabled={idx === attachedCases.length - 1}
                                title="Move down"
                                onClick={async () => {
                                  const next = [...attachedCases]
                                  ;[next[idx + 1], next[idx]] = [next[idx], next[idx + 1]]
                                  setAttachedCases(next)
                                  await api.reorderTestPlanCases(selectedPlan.id, next.map((x) => x.test_case_id))
                                }}
                              >
                                <ChevronDown size={13} />
                              </button>
                              <button
                                type="button"
                                className="btn btn-ghost btn-sm"
                                style={{ color: 'var(--danger)' }}
                                title="Detach from plan"
                                onClick={async () => {
                                  await api.removeTestPlanCase(selectedPlan.id, ac.test_case_id)
                                  notify(`Detached #${ac.test_case_id}`)
                                  await loadAttached(selectedPlan.id)
                                }}
                              >
                                <Unlink size={13} />
                              </button>
                              <ExternalLink size={13} color="var(--muted)" />
                            </div>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>

            {cases.length > 0 && (
              <div style={{ borderTop: '1px solid var(--border)', paddingTop: '0.9rem' }}>
                <h4 style={{ margin: '0 0 0.55rem' }}>{t.testplans.attachTitle} #{selectedPlan.id}</h4>
                <div style={{ display: 'grid', gap: '0.35rem', maxHeight: 240, overflowY: 'auto', marginBottom: '0.75rem' }}>
                  {cases.map((tc) => (
                    <label key={tc.id} style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', cursor: 'pointer', fontSize: '0.84rem' }}>
                      <input type="checkbox" checked={selectedCaseIds.includes(tc.id)} disabled={attachedSet.has(tc.id)} onChange={() => toggleCase(tc.id)} />
                      <span style={{ color: attachedSet.has(tc.id) ? 'var(--muted)' : undefined }}>
                        <span style={{ fontFamily: 'var(--font-mono)', marginRight: 5 }}>#{tc.id}</span>
                        {tc.summary} {attachedSet.has(tc.id) ? '(attached)' : ''}
                      </span>
                    </label>
                  ))}
                </div>
                <button type="button" onClick={() => void handleAttach()} disabled={selectedCaseIds.length === 0} className="btn btn-primary btn-sm">
                  {t.testplans.attachBtn}
                </button>
              </div>
            )}
          </div>
        ) : (
          <div className="card" style={{ color: 'var(--muted)' }}>{t.testplans.empty}</div>
        )}
      </div>
    </section>
  )
}
