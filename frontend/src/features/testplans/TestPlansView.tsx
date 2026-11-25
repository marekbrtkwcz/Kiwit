import { useEffect, useState } from 'react'
import { api, PlanCaseItem, Product, TestCase, TestPlan, Version } from '../../api/client'
import { useI18n } from '../../i18n'
import { EntityTagsBar } from '../management/EntityTagsBar'

export function TestPlansView() {
  const { t } = useI18n()
  const [products, setProducts] = useState<Product[]>([])
  const [versions, setVersions] = useState<Version[]>([])
  const [plans, setPlans] = useState<TestPlan[]>([])
  const [cases, setCases] = useState<TestCase[]>([])
  const [attachedCases, setAttachedCases] = useState<PlanCaseItem[]>([])
  const [productId, setProductId] = useState<number>(1)
  const [versionId, setVersionId] = useState<number>(0)
  const [selectedPlanId, setSelectedPlanId] = useState<number>(0)
  const [parentId, setParentId] = useState<number | undefined>(undefined)
  const [selectedCaseIds, setSelectedCaseIds] = useState<number[]>([])
  const [name, setName] = useState('')
  const [error, setError] = useState<string | null>(null)

  const load = async () => {
    try {
      const [pRes, tpRes, tcRes] = await Promise.all([api.listProducts(), api.listTestPlans(), api.listTestCases({ limit: 200 })])
      setProducts(pRes.items)
      setPlans(tpRes.items)
      setCases(tcRes.items)
      if (tpRes.items.length > 0 && !selectedPlanId) {
        setSelectedPlanId(tpRes.items[0].id)
      }
      const activePid = pRes.items.length > 0 ? pRes.items[0].id : 0
      if (activePid && productId !== activePid && !pRes.items.some((p) => p.id === productId)) {
        setProductId(activePid)
      }
      if (activePid) {
        const vRes = await api.listVersions(productId || activePid)
        setVersions(vRes.items)
        if (vRes.items.length > 0) setVersionId(vRes.items[0].id)
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
  }, [productId])

  useEffect(() => {
    if (selectedPlanId > 0) {
      void loadAttached(selectedPlanId)
    } else {
      setAttachedCases([])
    }
  }, [selectedPlanId])

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim() || !versionId) return
    try {
      const created = await api.createTestPlan({ product_id: productId, version_id: versionId, parent_id: parentId, name: name.trim() })
      setName('')
      setParentId(undefined)
      setSelectedPlanId(created.id)
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
      await api.addTestPlanCases(selectedPlanId, selectedCaseIds)
      setSelectedCaseIds([])
      await loadAttached(selectedPlanId)
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errors.createTestPlan)
    }
  }

  const attachedSet = new Set(attachedCases.map((c) => c.test_case_id))

  return (
    <section>
      <h2>{t.testplans.title}</h2>
      {error && <p className="alert-error">{error}</p>}

      <form onSubmit={handleCreate} style={{ display: 'flex', gap: '0.5rem', marginBottom: '1rem', flexWrap: 'wrap' }}>
        <select className="select" value={productId} onChange={(e) => setProductId(Number(e.target.value))}>
          {products.map((p) => (
            <option key={p.id} value={p.id}>{p.name}</option>
          ))}
        </select>
        <select className="select" value={versionId} onChange={(e) => setVersionId(Number(e.target.value))}>
          {versions.map((v) => (
            <option key={v.id} value={v.id}>{v.value}</option>
          ))}
        </select>
        <select className="select" value={parentId ?? ''} onChange={(e) => setParentId(e.target.value ? Number(e.target.value) : undefined)}>
          <option value="">No parent</option>
          {plans.map((p) => (
            <option key={p.id} value={p.id}>Parent: {p.name}</option>
          ))}
        </select>
        <input className="input" placeholder={t.testplans.namePlaceholder} value={name} onChange={(e) => setName(e.target.value)} style={{ flex: 1, minWidth: 200 }} />
        <button type="submit" disabled={!versionId} className="btn btn-primary">{t.testplans.create}</button>
      </form>

      <ul style={{ listStyle: 'none', padding: 0, display: 'grid', gap: '0.5rem', marginBottom: '1.25rem' }}>
        {plans.map((tp) => (
          <li
            key={tp.id}
            className="card"
            onClick={() => setSelectedPlanId(tp.id)}
            style={{
              border: selectedPlanId === tp.id ? '2px solid var(--primary)' : undefined,
              cursor: 'pointer',
            }}
          >
            <strong>{tp.name}</strong>
            <span style={{ marginLeft: '0.5rem', fontSize: '0.85rem', color: 'var(--muted)' }}>
              Product #{tp.product_id} · Version #{tp.version_id}{tp.parent_id ? ` · Parent #${tp.parent_id}` : ''}
            </span>
            <span style={{ float: 'right', display: 'inline-flex', gap: '0.35rem' }}>
              <button
                type="button"
                className="btn btn-sm"
                onClick={async (e) => {
                  e.stopPropagation()
                  try {
                    await api.cloneTestPlan(tp.id, `${tp.name} (copy)`)
                    await load()
                  } catch (err) {
                    setError(err instanceof Error ? err.message : t.errors.createTestPlan)
                  }
                }}
              >
                Clone
              </button>
              <button
                type="button"
                className="btn btn-danger btn-sm"
                onClick={async (e) => {
                  e.stopPropagation()
                  if (!window.confirm(`Delete TestPlan #${tp.id}?`)) return
                  try {
                    await api.deleteTestPlan(tp.id)
                    if (selectedPlanId === tp.id) setSelectedPlanId(0)
                    await load()
                  } catch (err) {
                    setError(err instanceof Error ? err.message : t.errors.loadTestPlans)
                  }
                }}
              >
                ×
              </button>
            </span>
          </li>
        ))}
        {plans.length === 0 && <li style={{ color: 'var(--muted)' }}>{t.testplans.empty}</li>}
      </ul>

      {selectedPlanId > 0 && (
        <div className="card" style={{ display: 'grid', gap: '1rem' }}>
          <div>
            <h3 style={{ margin: '0 0 0.4rem' }}>TestPlan #{selectedPlanId} — Attached Cases ({attachedCases.length})</h3>
            <EntityTagsBar entity="testplans" entityId={selectedPlanId} />
            {attachedCases.length === 0 ? (
              <p style={{ color: 'var(--muted)', margin: '0.5rem 0 0', fontSize: '0.85rem' }}>No test cases attached yet.</p>
            ) : (
              <table className="data-table" style={{ marginTop: '0.5rem' }}>
                <thead>
                  <tr>
                    <th>#</th>
                    <th>Summary</th>
                    <th>Priority</th>
                    <th>Status</th>
                  </tr>
                </thead>
                <tbody>
                  {attachedCases.map((ac) => (
                    <tr key={ac.test_case_id}>
                      <td>#{ac.test_case_id}</td>
                      <td>{ac.summary}</td>
                      <td><span className="badge">{ac.priority}</span></td>
                      <td><span className="badge">{ac.status}</span></td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>

          {cases.length > 0 && (
            <div style={{ borderTop: '1px solid var(--border)', paddingTop: '0.9rem' }}>
              <h4 style={{ margin: '0 0 0.5rem' }}>{t.testplans.attachTitle} #{selectedPlanId}</h4>
              <div style={{ display: 'grid', gap: '0.35rem', marginBottom: '0.75rem' }}>
                {cases.map((tc) => (
                  <label key={tc.id} style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', cursor: 'pointer', fontSize: '0.875rem' }}>
                    <input
                      type="checkbox"
                      checked={selectedCaseIds.includes(tc.id)}
                      disabled={attachedSet.has(tc.id)}
                      onChange={() => toggleCase(tc.id)}
                    />
                    <span style={{ color: attachedSet.has(tc.id) ? 'var(--muted)' : undefined }}>
                      #{tc.id} {tc.summary} {attachedSet.has(tc.id) ? '(attached)' : ''}
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
      )}
    </section>
  )
}
