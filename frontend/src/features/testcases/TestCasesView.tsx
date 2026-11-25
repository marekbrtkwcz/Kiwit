import { useEffect, useState } from 'react'
import { api, Category, ComponentItem, Product, RevisionItem, TestCase, TestStep } from '../../api/client'
import { useI18n } from '../../i18n'
import { AttachmentsPanel } from '../attachments/AttachmentsPanel'
import { CommentsPanel } from '../comments/CommentsPanel'
import { EntityTagsBar } from '../management/EntityTagsBar'

function TestCaseComponentsBar({ caseId, productId }: { caseId: number; productId: number }) {
  const [attached, setAttached] = useState<ComponentItem[]>([])
  const [allComps, setAllComps] = useState<ComponentItem[]>([])
  const [compId, setCompId] = useState<number>(0)

  const load = async () => {
    try {
      const [att, all] = await Promise.all([api.listTestCaseComponents(caseId), api.listComponents(productId)])
      setAttached(att.items)
      setAllComps(all.items)
      if (!compId && all.items.length > 0) setCompId(all.items[0].id)
    } catch {
      // Ignore when unauthenticated
    }
  }

  useEffect(() => {
    void load()
  }, [caseId, productId])

  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', flexWrap: 'wrap', marginTop: '0.45rem', fontSize: '0.82rem' }}>
      <span style={{ color: 'var(--muted)' }}>Components:</span>
      {attached.map((c) => (
        <span key={c.id} className="badge">{c.name}</span>
      ))}
      {attached.length === 0 && <span style={{ color: 'var(--muted)' }}>—</span>}
      {allComps.length > 0 && (
        <>
          <select className="select" value={compId} onChange={(e) => setCompId(Number(e.target.value))} style={{ padding: '0.18rem 0.4rem', fontSize: '0.8rem' }}>
            {allComps.map((c) => (
              <option key={c.id} value={c.id}>{c.name}</option>
            ))}
          </select>
          <button
            type="button"
            className="btn btn-sm"
            onClick={async () => {
              if (!compId) return
              await api.attachTestCaseComponent(caseId, compId)
              await load()
            }}
          >
            + Component
          </button>
        </>
      )}
    </div>
  )
}

const PAGE_SIZE = 20

export function TestCasesView() {
  const { t } = useI18n()
  const [products, setProducts] = useState<Product[]>([])
  const [categories, setCategories] = useState<Category[]>([])
  const [cases, setCases] = useState<TestCase[]>([])
  const [total, setTotal] = useState(0)
  const [offset, setOffset] = useState(0)
  const [productId, setProductId] = useState<number>(1)
  const [categoryId, setCategoryId] = useState<number | undefined>(undefined)
  const [newCategory, setNewCategory] = useState('')
  const [editingId, setEditingId] = useState<number | null>(null)
  const [revisions, setRevisions] = useState<RevisionItem[]>([])
  const [filterQ, setFilterQ] = useState('')
  const [filterPriority, setFilterPriority] = useState('')
  const [summary, setSummary] = useState('')
  const [preconditions, setPreconditions] = useState('')
  const [format, setFormat] = useState<'steps' | 'gherkin'>('steps')
  const [gherkinBody, setGherkinBody] = useState('')
  const [priority, setPriority] = useState<TestCase['priority']>('medium')
  const [status, setStatus] = useState<TestCase['status']>('confirmed')
  const [isAutomated, setIsAutomated] = useState(false)
  const [script, setScript] = useState('')
  const [scriptArgs, setScriptArgs] = useState('')
  const [steps, setSteps] = useState<TestStep[]>([{ position: 1, action: '', expected_result: '' }])
  const [selectedIds, setSelectedIds] = useState<number[]>([])
  const [bulkStatus, setBulkStatus] = useState('')
  const [bulkPriority, setBulkPriority] = useState('')
  const [error, setError] = useState<string | null>(null)

  const load = async () => {
    try {
      const [pRes, cRes, tcRes] = await Promise.all([
        api.listProducts(),
        api.listCategories(),
        api.listTestCases({ q: filterQ, priority: filterPriority, limit: PAGE_SIZE, offset }),
      ])
      setProducts(pRes.items)
      setCategories(cRes.items)
      if (cRes.items.length > 0 && categoryId === undefined) {
        setCategoryId(cRes.items[0].id)
      }
      if (pRes.items.length > 0 && !pRes.items.some((p) => p.id === productId)) {
        setProductId(pRes.items[0].id)
      }
      setCases(tcRes.items)
      setTotal(tcRes.total ?? tcRes.items.length)
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : t.errors.loadTestCases)
    }
  }

  useEffect(() => {
    void load()
  }, [filterQ, filterPriority, offset])

  const resetForm = () => {
    setEditingId(null)
    setSummary('')
    setPreconditions('')
    setFormat('steps')
    setGherkinBody('')
    setPriority('medium')
    setStatus('confirmed')
    setIsAutomated(false)
    setScript('')
    setScriptArgs('')
    setSteps([{ position: 1, action: '', expected_result: '' }])
  }

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
    const validSteps =
      format === 'steps'
        ? steps.filter((s) => s.action.trim() !== '').map((s, i) => ({ ...s, position: i + 1 }))
        : []
    try {
      const payload = {
        product_id: productId,
        category_id: categoryId,
        summary: summary.trim(),
        preconditions: preconditions.trim(),
        format,
        gherkin_body: format === 'gherkin' ? gherkinBody : '',
        priority,
        status,
        is_automated: isAutomated,
        script: isAutomated ? script.trim() : '',
        arguments: isAutomated ? scriptArgs.trim() : '',
        steps: validSteps,
      }
      if (editingId !== null) {
        await api.updateTestCase(editingId, payload)
      } else {
        await api.createTestCase(payload)
      }
      resetForm()
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errors.saveTestCase)
    }
  }

  const toggleSelect = (id: number) => {
    setSelectedIds((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]))
  }

  const handleBulkUpdate = async () => {
    if (selectedIds.length === 0 || (!bulkStatus && !bulkPriority)) return
    try {
      await api.bulkUpdateTestCases({
        ids: selectedIds,
        status: bulkStatus || undefined,
        priority: bulkPriority || undefined,
      })
      setSelectedIds([])
      setBulkStatus('')
      setBulkPriority('')
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errors.saveTestCase)
    }
  }

  return (
    <section>
      <h2>{t.testcases.title}</h2>
      {error && <p className="alert-error">{error}</p>}

      <form onSubmit={handleSubmit} className="card" style={{ marginBottom: '1.25rem', display: 'grid', gap: '0.75rem' }}>
        <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}>
          <select className="select" value={productId} onChange={(e) => setProductId(Number(e.target.value))}>
            {products.map((p) => (
              <option key={p.id} value={p.id}>{p.name}</option>
            ))}
          </select>
          <input
            className="input"
            placeholder={t.testcases.summaryPlaceholder}
            value={summary}
            onChange={(e) => setSummary(e.target.value)}
            style={{ flex: 1, minWidth: 220 }}
          />
          <select className="select" value={categoryId ?? ''} onChange={(e) => setCategoryId(e.target.value ? Number(e.target.value) : undefined)}>
            {categories.map((c) => (
              <option key={c.id} value={c.id}>{c.name}</option>
            ))}
          </select>
          <select className="select" value={priority} onChange={(e) => setPriority(e.target.value as TestCase['priority'])}>
            <option value="low">low</option>
            <option value="medium">medium</option>
            <option value="high">high</option>
            <option value="critical">critical</option>
          </select>
          <select className="select" value={status} onChange={(e) => setStatus(e.target.value as TestCase['status'])}>
            <option value="draft">draft</option>
            <option value="confirmed">confirmed</option>
            <option value="disabled">disabled</option>
          </select>
          <select className="select" value={format} onChange={(e) => setFormat(e.target.value as 'steps' | 'gherkin')}>
            <option value="steps">Steps</option>
            <option value="gherkin">Gherkin</option>
          </select>
        </div>

        <input
          className="input"
          placeholder="Preconditions (optional)"
          value={preconditions}
          onChange={(e) => setPreconditions(e.target.value)}
        />

        <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'center', flexWrap: 'wrap' }}>
          <label style={{ display: 'inline-flex', alignItems: 'center', gap: '0.35rem', fontSize: '0.85rem' }}>
            <input type="checkbox" checked={isAutomated} onChange={(e) => setIsAutomated(e.target.checked)} />
            Automated
          </label>
          {isAutomated && (
            <>
              <input className="input" placeholder="Script path (e.g. tests/login.spec.ts)" value={script} onChange={(e) => setScript(e.target.value)} style={{ flex: 1, minWidth: 200 }} />
              <input className="input" placeholder="Arguments (optional)" value={scriptArgs} onChange={(e) => setScriptArgs(e.target.value)} style={{ flex: 1, minWidth: 160 }} />
            </>
          )}
          <div style={{ marginLeft: 'auto', display: 'flex', gap: '0.35rem' }}>
            <input className="input" placeholder={t.testcases.newCategoryPlaceholder} value={newCategory} onChange={(e) => setNewCategory(e.target.value)} style={{ fontSize: '0.8rem' }} />
            <button
              type="button"
              className="btn btn-sm"
              onClick={async () => {
                if (!newCategory.trim()) return
                try {
                  const created = await api.createCategory(newCategory.trim())
                  setNewCategory('')
                  setCategoryId(created.id)
                  await load()
                } catch (err) {
                  setError(err instanceof Error ? err.message : t.errors.saveTestCase)
                }
              }}
            >
              {t.testcases.addCategory}
            </button>
          </div>
        </div>

        {format === 'gherkin' ? (
          <textarea
            className="textarea"
            rows={5}
            placeholder={"Feature: Login\n  Scenario: Valid credentials\n    Given user is on /login\n    When user submits valid credentials\n    Then user sees dashboard"}
            value={gherkinBody}
            onChange={(e) => setGherkinBody(e.target.value)}
          />
        ) : (
          <div style={{ display: 'grid', gap: '0.4rem' }}>
            <strong>{t.testcases.stepsLabel}</strong>
            {steps.map((s, idx) => (
              <div key={idx} style={{ display: 'flex', gap: '0.4rem', alignItems: 'center' }}>
                <span style={{ width: 24, color: 'var(--muted)' }}>{idx + 1}.</span>
                <input className="input" placeholder={t.testcases.actionPlaceholder} value={s.action} onChange={(e) => updateStep(idx, 'action', e.target.value)} style={{ flex: 1 }} />
                <input className="input" placeholder={t.testcases.expectedPlaceholder} value={s.expected_result ?? ''} onChange={(e) => updateStep(idx, 'expected_result', e.target.value)} style={{ flex: 1 }} />
                <button type="button" className="btn btn-sm" onClick={() => moveStep(idx, -1)} disabled={idx === 0}>↑</button>
                <button type="button" className="btn btn-sm" onClick={() => moveStep(idx, 1)} disabled={idx === steps.length - 1}>↓</button>
              </div>
            ))}
            <div>
              <button type="button" className="btn btn-sm" onClick={addStep}>{t.testcases.addStep}</button>
            </div>
          </div>
        )}

        <div style={{ display: 'flex', gap: '0.5rem' }}>
          <button type="submit" className="btn btn-primary">{editingId !== null ? `Update #${editingId}` : t.testcases.save}</button>
          {editingId !== null && (
            <button type="button" className="btn" onClick={resetForm}>Cancel</button>
          )}
        </div>
      </form>

      <div style={{ display: 'flex', gap: '0.5rem', marginBottom: '0.75rem', flexWrap: 'wrap', alignItems: 'center' }}>
        <input
          className="input"
          placeholder="Search..."
          value={filterQ}
          onChange={(e) => {
            setOffset(0)
            setFilterQ(e.target.value)
          }}
          style={{ width: 240 }}
        />
        <select
          className="select"
          value={filterPriority}
          onChange={(e) => {
            setOffset(0)
            setFilterPriority(e.target.value)
          }}
        >
          <option value="">All priorities</option>
          <option value="low">low</option>
          <option value="medium">medium</option>
          <option value="high">high</option>
          <option value="critical">critical</option>
        </select>

        {selectedIds.length > 0 && (
          <div style={{ marginLeft: 'auto', display: 'flex', gap: '0.4rem', alignItems: 'center' }}>
            <span className="badge">{selectedIds.length} selected</span>
            <select className="select" value={bulkStatus} onChange={(e) => setBulkStatus(e.target.value)}>
              <option value="">Status...</option>
              <option value="draft">draft</option>
              <option value="confirmed">confirmed</option>
              <option value="disabled">disabled</option>
            </select>
            <select className="select" value={bulkPriority} onChange={(e) => setBulkPriority(e.target.value)}>
              <option value="">Priority...</option>
              <option value="low">low</option>
              <option value="medium">medium</option>
              <option value="high">high</option>
              <option value="critical">critical</option>
            </select>
            <button type="button" className="btn btn-primary btn-sm" onClick={() => void handleBulkUpdate()} disabled={!bulkStatus && !bulkPriority}>
              Apply bulk
            </button>
          </div>
        )}
      </div>

      <ul style={{ listStyle: 'none', padding: 0, display: 'grid', gap: '0.5rem' }}>
        {cases.map((tc) => (
          <li
            key={tc.id}
            className="card"
            onClick={() => {
              setEditingId(tc.id)
              setProductId(tc.product_id)
              setCategoryId(tc.category_id)
              setSummary(tc.summary)
              setPreconditions(tc.preconditions ?? '')
              setFormat(tc.format)
              setGherkinBody(tc.gherkin_body ?? '')
              setPriority(tc.priority)
              setStatus(tc.status)
              setIsAutomated(Boolean(tc.is_automated))
              setScript(tc.script ?? '')
              setScriptArgs(tc.arguments ?? '')
              setSteps(tc.steps.length > 0 ? tc.steps : [{ position: 1, action: '', expected_result: '' }])
              void api.listTestCaseRevisions(tc.id).then((r) => setRevisions(r.items)).catch(() => setRevisions([]))
            }}
            style={{
              border: editingId === tc.id ? '2px solid var(--primary)' : undefined,
              cursor: 'pointer',
            }}
          >
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: '0.5rem' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                <input
                  type="checkbox"
                  checked={selectedIds.includes(tc.id)}
                  onClick={(e) => e.stopPropagation()}
                  onChange={() => toggleSelect(tc.id)}
                />
                <strong>{tc.summary}</strong>
                <span className="badge">{tc.format}</span>
                {tc.is_automated && <span className="badge">auto</span>}
              </div>
              <div style={{ display: 'flex', gap: '0.4rem', alignItems: 'center' }}>
                <span style={{ fontSize: '0.8rem', color: 'var(--muted)' }}>
                  {tc.priority} · {tc.status} · {tc.steps.length} {t.testcases.stepsCount}
                </span>
                <button
                  type="button"
                  className="btn btn-sm"
                  onClick={async (e) => {
                    e.stopPropagation()
                    try {
                      await api.cloneTestCase(tc.id)
                      await load()
                    } catch (err) {
                      setError(err instanceof Error ? err.message : t.errors.saveTestCase)
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
                    if (!window.confirm(`Delete TestCase #${tc.id}?`)) return
                    try {
                      await api.deleteTestCase(tc.id)
                      if (editingId === tc.id) resetForm()
                      await load()
                    } catch (err) {
                      setError(err instanceof Error ? err.message : t.errors.saveTestCase)
                    }
                  }}
                >
                  ×
                </button>
              </div>
            </div>
          </li>
        ))}
        {cases.length === 0 && <li style={{ color: 'var(--muted)' }}>{t.testcases.empty}</li>}
      </ul>

      {total > PAGE_SIZE && (
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginTop: '0.75rem', fontSize: '0.85rem' }}>
          <span style={{ color: 'var(--muted)' }}>
            Showing {offset + 1}–{Math.min(offset + cases.length, total)} of {total}
          </span>
          <div style={{ display: 'flex', gap: '0.4rem' }}>
            <button type="button" className="btn btn-sm" disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}>
              Prev
            </button>
            <button type="button" className="btn btn-sm" disabled={offset + PAGE_SIZE >= total} onClick={() => setOffset(offset + PAGE_SIZE)}>
              Next
            </button>
          </div>
        </div>
      )}

      {editingId !== null && (
        <div className="card" style={{ marginTop: '1rem' }}>
          <h4 style={{ margin: '0 0 0.5rem' }}>TestCase #{editingId} — Revisions ({revisions.length})</h4>
          <ul style={{ listStyle: 'none', padding: 0, margin: '0 0 0.75rem', fontSize: '0.85rem', color: 'var(--muted)' }}>
            {revisions.map((r) => (
              <li key={r.id}>v{r.version}: {r.summary} ({r.priority}, {r.status})</li>
            ))}
          </ul>
          <EntityTagsBar entity="testcases" entityId={editingId} />
          <TestCaseComponentsBar caseId={editingId} productId={productId} />
          <AttachmentsPanel entityType="test_case" entityId={editingId} />
          <CommentsPanel entityType="test_case" entityId={editingId} />
        </div>
      )}
    </section>
  )
}
