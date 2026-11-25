import { useEffect, useState } from 'react'
import { api, Product, TestCase, TestStep } from '../../api/client'
import { useI18n } from '../../i18n'

export function TestCasesView() {
  const { t } = useI18n()
  const [products, setProducts] = useState<Product[]>([])
  const [cases, setCases] = useState<TestCase[]>([])
  const [productId, setProductId] = useState<number>(1)
  const [summary, setSummary] = useState('')
  const [priority, setPriority] = useState<TestCase['priority']>('medium')
  const [steps, setSteps] = useState<TestStep[]>([{ position: 1, action: '', expected_result: '' }])
  const [error, setError] = useState<string | null>(null)

  const load = async () => {
    try {
      const [pRes, tcRes] = await Promise.all([api.listProducts(), api.listTestCases()])
      setProducts(pRes.items)
      if (pRes.items.length > 0 && !pRes.items.some((p) => p.id === productId)) {
        setProductId(pRes.items[0].id)
      }
      setCases(tcRes.items)
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : t.errors.loadTestCases)
    }
  }

  useEffect(() => {
    void load()
  }, [])

  const updateStep = (idx: number, field: keyof TestStep, value: string) => {
    setSteps((prev) => prev.map((s, i) => (i === idx ? { ...s, [field]: value } : s)))
  }

  const addStep = () => {
    setSteps((prev) => [...prev, { position: prev.length + 1, action: '', expected_result: '' }])
  }

  const moveStep = (idx: number, dir: -1 | 1) => {
    const target = idx + dir
    if (target < 0 || target >= steps.length) return
    const copy = [...steps]
    ;[copy[idx], copy[target]] = [copy[target], copy[idx]]
    setSteps(copy.map((s, i) => ({ ...s, position: i + 1 })))
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!summary.trim()) return
    const validSteps = steps
      .filter((s) => s.action.trim() !== '')
      .map((s, i) => ({ ...s, position: i + 1 }))
    try {
      await api.createTestCase({
        product_id: productId,
        summary: summary.trim(),
        format: 'steps',
        priority,
        status: 'confirmed',
        steps: validSteps,
      })
      setSummary('')
      setSteps([{ position: 1, action: '', expected_result: '' }])
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errors.saveTestCase)
    }
  }

  return (
    <section>
      <h2>{t.testcases.title}</h2>
      {error && <p style={{ color: '#b91c1c' }}>{error}</p>}

      <form onSubmit={handleSubmit} style={{ border: '1px solid #e5e7eb', borderRadius: 8, padding: '1rem', marginBottom: '1.5rem', display: 'grid', gap: '0.75rem' }}>
        <div style={{ display: 'flex', gap: '0.5rem' }}>
          <select value={productId} onChange={(e) => setProductId(Number(e.target.value))} style={{ padding: '0.4rem', borderRadius: 6, border: '1px solid #d1d5db' }}>
            {products.map((p) => (
              <option key={p.id} value={p.id}>{p.name}</option>
            ))}
          </select>
          <input
            placeholder={t.testcases.summaryPlaceholder}
            value={summary}
            onChange={(e) => setSummary(e.target.value)}
            style={{ flex: 1, padding: '0.4rem 0.6rem', borderRadius: 6, border: '1px solid #d1d5db' }}
          />
          <select value={priority} onChange={(e) => setPriority(e.target.value as TestCase['priority'])} style={{ padding: '0.4rem', borderRadius: 6, border: '1px solid #d1d5db' }}>
            <option value="low">low</option>
            <option value="medium">medium</option>
            <option value="high">high</option>
            <option value="critical">critical</option>
          </select>
        </div>

        <div style={{ display: 'grid', gap: '0.4rem' }}>
          <strong>{t.testcases.stepsLabel}</strong>
          {steps.map((s, idx) => (
            <div key={idx} style={{ display: 'flex', gap: '0.4rem', alignItems: 'center' }}>
              <span style={{ width: 24, color: '#6b7280' }}>{idx + 1}.</span>
              <input
                placeholder={t.testcases.actionPlaceholder}
                value={s.action}
                onChange={(e) => updateStep(idx, 'action', e.target.value)}
                style={{ flex: 1, padding: '0.35rem 0.5rem', borderRadius: 6, border: '1px solid #d1d5db' }}
              />
              <input
                placeholder={t.testcases.expectedPlaceholder}
                value={s.expected_result ?? ''}
                onChange={(e) => updateStep(idx, 'expected_result', e.target.value)}
                style={{ flex: 1, padding: '0.35rem 0.5rem', borderRadius: 6, border: '1px solid #d1d5db' }}
              />
              <button type="button" onClick={() => moveStep(idx, -1)} disabled={idx === 0}>↑</button>
              <button type="button" onClick={() => moveStep(idx, 1)} disabled={idx === steps.length - 1}>↓</button>
            </div>
          ))}
          <div>
            <button type="button" onClick={addStep} style={{ padding: '0.3rem 0.6rem', borderRadius: 6, border: '1px solid #d1d5db', background: '#fff', cursor: 'pointer' }}>
              {t.testcases.addStep}
            </button>
          </div>
        </div>

        <div>
          <button type="submit" style={{ padding: '0.45rem 1rem', borderRadius: 6, background: '#111827', color: '#fff', border: 'none', cursor: 'pointer' }}>
            {t.testcases.save}
          </button>
        </div>
      </form>

      <ul style={{ listStyle: 'none', padding: 0, display: 'grid', gap: '0.5rem' }}>
        {cases.map((tc) => (
          <li key={tc.id} style={{ padding: '0.75rem', border: '1px solid #e5e7eb', borderRadius: 8 }}>
            <div style={{ display: 'flex', justifyContent: 'space-between' }}>
              <strong>{tc.summary}</strong>
              <span style={{ fontSize: '0.85rem', color: '#6b7280' }}>{tc.priority} · {tc.steps.length} {t.testcases.stepsCount}</span>
            </div>
          </li>
        ))}
        {cases.length === 0 && <li style={{ color: '#6b7280' }}>{t.testcases.empty}</li>}
      </ul>
    </section>
  )
}
