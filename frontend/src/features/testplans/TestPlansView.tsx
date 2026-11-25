import { useEffect, useState } from 'react'
import { api, Product, TestCase, TestPlan, Version } from '../../api/client'
import { useI18n } from '../../i18n'

export function TestPlansView() {
  const { t } = useI18n()
  const [products, setProducts] = useState<Product[]>([])
  const [versions, setVersions] = useState<Version[]>([])
  const [plans, setPlans] = useState<TestPlan[]>([])
  const [cases, setCases] = useState<TestCase[]>([])
  const [productId, setProductId] = useState<number>(1)
  const [versionId, setVersionId] = useState<number>(0)
  const [selectedPlanId, setSelectedPlanId] = useState<number>(0)
  const [selectedCaseIds, setSelectedCaseIds] = useState<number[]>([])
  const [name, setName] = useState('')
  const [error, setError] = useState<string | null>(null)

  const load = async () => {
    try {
      const [pRes, tpRes, tcRes] = await Promise.all([api.listProducts(), api.listTestPlans(), api.listTestCases()])
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

  useEffect(() => {
    void load()
  }, [productId])

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim() || !versionId) return
    try {
      const created = await api.createTestPlan({ product_id: productId, version_id: versionId, name: name.trim() })
      setName('')
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
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errors.createTestPlan)
    }
  }

  return (
    <section>
      <h2>{t.testplans.title}</h2>
      {error && <p style={{ color: '#b91c1c' }}>{error}</p>}

      <form onSubmit={handleCreate} style={{ display: 'flex', gap: '0.5rem', marginBottom: '1rem' }}>
        <select value={productId} onChange={(e) => setProductId(Number(e.target.value))} style={{ padding: '0.4rem', borderRadius: 6, border: '1px solid #d1d5db' }}>
          {products.map((p) => (
            <option key={p.id} value={p.id}>{p.name}</option>
          ))}
        </select>
        <select value={versionId} onChange={(e) => setVersionId(Number(e.target.value))} style={{ padding: '0.4rem', borderRadius: 6, border: '1px solid #d1d5db' }}>
          {versions.map((v) => (
            <option key={v.id} value={v.id}>{v.value}</option>
          ))}
        </select>
        <input
          placeholder={t.testplans.namePlaceholder}
          value={name}
          onChange={(e) => setName(e.target.value)}
          style={{ flex: 1, padding: '0.4rem 0.6rem', borderRadius: 6, border: '1px solid #d1d5db' }}
        />
        <button type="submit" disabled={!versionId} style={{ padding: '0.4rem 0.9rem', borderRadius: 6, background: '#111827', color: '#fff', border: 'none', cursor: 'pointer' }}>
          {t.testplans.create}
        </button>
      </form>

      <ul style={{ listStyle: 'none', padding: 0, display: 'grid', gap: '0.5rem', marginBottom: '1.5rem' }}>
        {plans.map((tp) => (
          <li
            key={tp.id}
            onClick={() => setSelectedPlanId(tp.id)}
            style={{
              padding: '0.75rem',
              border: selectedPlanId === tp.id ? '2px solid #111827' : '1px solid #e5e7eb',
              borderRadius: 8,
              cursor: 'pointer',
            }}
          >
            <strong>{tp.name}</strong>
            <span style={{ marginLeft: '0.5rem', fontSize: '0.85rem', color: '#6b7280' }}>
              Product #{tp.product_id} · Version #{tp.version_id}
            </span>
            <button
              type="button"
              onClick={async (e) => {
                e.stopPropagation()
                await api.cloneTestPlan(tp.id, `${tp.name} (copy)`)
                await load()
              }}
              style={{ float: 'right', padding: '0.2rem 0.5rem', borderRadius: 4, border: '1px solid #d1d5db', background: '#fff', cursor: 'pointer', fontSize: '0.8rem' }}
            >
              Clone
            </button>
          </li>
        ))}
        {plans.length === 0 && <li style={{ color: '#6b7280' }}>{t.testplans.empty}</li>}
      </ul>

      {selectedPlanId > 0 && cases.length > 0 && (
        <div style={{ border: '1px solid #e5e7eb', borderRadius: 8, padding: '1rem' }}>
          <h3 style={{ marginTop: 0 }}>{t.testplans.attachTitle} #{selectedPlanId}</h3>
          <div style={{ display: 'grid', gap: '0.35rem', marginBottom: '0.75rem' }}>
            {cases.map((tc) => (
              <label key={tc.id} style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', cursor: 'pointer' }}>
                <input
                  type="checkbox"
                  checked={selectedCaseIds.includes(tc.id)}
                  onChange={() => toggleCase(tc.id)}
                />
                <span>{tc.summary}</span>
              </label>
            ))}
          </div>
          <button
            type="button"
            onClick={() => void handleAttach()}
            disabled={selectedCaseIds.length === 0}
            style={{ padding: '0.4rem 0.9rem', borderRadius: 6, background: '#111827', color: '#fff', border: 'none', cursor: 'pointer' }}
          >
            {t.testplans.attachBtn}
          </button>
        </div>
      )}
    </section>
  )
}
