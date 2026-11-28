import {
  Bot,
  ChevronDown,
  ChevronUp,
  Copy,
  FileDown,
  History,
  Link2,
  MessageSquare,
  Paperclip,
  Plus,
  RotateCcw,
  Search,
  Trash2,
  Upload,
  X,
} from 'lucide-react'
import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import {
  api,
  type Category,
  type ComponentItem,
  downloadExport,
  type ImportResult,
  type RevisionItem,
  type TemplateItem,
  type TestCase,
  type TestStep,
} from '../../api/client'
import { CustomFieldsPanel } from '../../components/CustomFieldsPanel'
import { ExportButton } from '../../components/ExportButton'
import { ConfirmDialog, EmptyState, LoadingSkeleton } from '../../components/UiPrimitives'
import { useWorkspace } from '../../context/WorkspaceContext'
import { useI18n } from '../../i18n'
import { AttachmentsPanel } from '../attachments/AttachmentsPanel'
import { CommentsPanel } from '../comments/CommentsPanel'
import { EntityTagsBar } from '../management/EntityTagsBar'
import { CaseDatasetsPanel } from './CaseDatasetsPanel'
import { VisualBaselinesPanel } from './VisualBaselinesPanel'

function TestCaseComponentsBar({ caseId, productId }: { caseId: number; productId: number }) {
  const { t } = useI18n()
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
      // ignore
    }
  }

  useEffect(() => {
    void load()
  }, [caseId, productId])

  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', flexWrap: 'wrap', fontSize: '0.82rem' }}>
      <span style={{ color: 'var(--muted)' }}>{t.common.components}:</span>
      {attached.map((c) => (
        <span key={c.id} className="badge">
          {c.name}
        </span>
      ))}
      {attached.length === 0 && <span style={{ color: 'var(--muted)' }}>—</span>}
      {allComps.length > 0 && (
        <>
          <select
            className="select"
            value={compId}
            onChange={(e) => setCompId(Number(e.target.value))}
            style={{ padding: '0.2rem 0.45rem', fontSize: '0.78rem' }}
          >
            {allComps.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
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
            {t.products.addComponent}
          </button>
        </>
      )}
    </div>
  )
}

const PAGE_SIZE = 20

export function TestCasesView() {
  const { t } = useI18n()
  const { products, activeProductId, notify, notifyAction } = useWorkspace()
  const [loading, setLoading] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<{ id: number; summary: string } | null>(null)
  const { id: routeCaseId } = useParams<{ id: string }>()
  const navigate = useNavigate()

  const [categories, setCategories] = useState<Category[]>([])
  const [templates, setTemplates] = useState<TemplateItem[]>([])
  const [cases, setCases] = useState<TestCase[]>([])
  const [total, setTotal] = useState(0)
  const [offset, setOffset] = useState(0)

  // Filters
  const [filterQ, setFilterQ] = useState('')
  const [filterPriority, setFilterPriority] = useState('')
  const [filterStatus, setFilterStatus] = useState('')
  const [filterCategory, setFilterCategory] = useState<number>(0)
  const [filterAutomated, setFilterAutomated] = useState<'' | 'true' | 'false'>('')

  // Drawer state
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [drawerTab, setDrawerTab] = useState<'details' | 'comments' | 'attachments' | 'revisions'>('details')
  const [editingId, setEditingId] = useState<number | null>(null)
  const [revisions, setRevisions] = useState<RevisionItem[]>([])

  // Form fields
  const [productId, setProductId] = useState<number>(activeProductId ?? 1)
  const [categoryId, setCategoryId] = useState<number | undefined>(undefined)
  const [newCategory, setNewCategory] = useState('')
  const [summary, setSummary] = useState('')
  const [preconditions, setPreconditions] = useState('')
  const [format, setFormat] = useState<'steps' | 'gherkin' | 'markdown' | 'checklist'>('steps')
  const [gherkinBody, setGherkinBody] = useState('')
  const [mdPreview, setMdPreview] = useState('')
  const [priority, setPriority] = useState<TestCase['priority']>('medium')
  const [status, setStatus] = useState<TestCase['status']>('confirmed')
  const [isAutomated, setIsAutomated] = useState(false)
  const [script, setScript] = useState('')
  const [scriptArgs, setScriptArgs] = useState('')
  const [steps, setSteps] = useState<TestStep[]>([{ position: 1, action: '', expected_result: '' }])

  // Bulk selection
  const [selectedIds, setSelectedIds] = useState<number[]>([])
  const [bulkStatus, setBulkStatus] = useState('')
  const [bulkPriority, setBulkPriority] = useState('')

  // Import (CSV/JSON)
  const [showImport, setShowImport] = useState(false)
  const [importFile, setImportFile] = useState<File | null>(null)
  const [importBusy, setImportBusy] = useState(false)
  const [importResult, setImportResult] = useState<ImportResult | null>(null)

  // Trash (soft-deleted cases)
  const [showTrash, setShowTrash] = useState(false)
  const [trashItems, setTrashItems] = useState<TestCase[]>([])
  const [trashTotal, setTrashTotal] = useState(0)
  const [purgeTarget, setPurgeTarget] = useState<{ id: number; summary: string } | null>(null)
  const [error, setError] = useState<string | null>(null)

  const populateForm = (tc: TestCase) => {
    setEditingId(tc.id)
    setProductId(tc.product_id)
    setCategoryId(tc.category_id)
    setSummary(tc.summary)
    setPreconditions(tc.preconditions ?? '')
    setFormat(tc.format)
    setGherkinBody(tc.gherkin_body ?? '')
    setMdPreview('')
    setPriority(tc.priority)
    setStatus(tc.status)
    setIsAutomated(Boolean(tc.is_automated))
    setScript(tc.script ?? '')
    setScriptArgs(tc.arguments ?? '')
    setSteps(tc.steps.length > 0 ? tc.steps : [{ position: 1, action: '', expected_result: '' }])
    setDrawerTab('details')
    setDrawerOpen(true)
    void api
      .listTestCaseRevisions(tc.id)
      .then((r) => setRevisions(r.items))
      .catch(() => setRevisions([]))
  }

  const openNewDrawer = () => {
    setEditingId(null)
    setProductId(activeProductId ?? products[0]?.id ?? 1)
    setCategoryId(categories[0]?.id)
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
    setRevisions([])
    setDrawerTab('details')
    setDrawerOpen(true)
    if (routeCaseId) navigate('/cases', { replace: true })
  }

  const closeDrawer = () => {
    setDrawerOpen(false)
    if (routeCaseId) navigate('/cases', { replace: true })
  }

  const load = async () => {
    try {
      const [cRes, tplRes, tcRes] = await Promise.all([
        api.listCategories(),
        api.listTemplates().catch(() => ({ items: [] as TemplateItem[] })),
        api.listTestCases({
          q: filterQ || undefined,
          product_id: activeProductId ?? undefined,
          category_id: filterCategory || undefined,
          priority: filterPriority || undefined,
          status: filterStatus || undefined,
          is_automated: filterAutomated || undefined,
          limit: PAGE_SIZE,
          offset,
        }),
      ])
      setCategories(cRes.items)
      setTemplates(tplRes.items)
      setCases(tcRes.items)
      setTotal(tcRes.total ?? tcRes.items.length)
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : t.errors.loadTestCases)
    }
  }

  useEffect(() => {
    setOffset(0)
  }, [activeProductId, filterQ, filterPriority, filterStatus, filterCategory, filterAutomated])

  useEffect(() => {
    void load()
  }, [activeProductId, filterQ, filterPriority, filterStatus, filterCategory, filterAutomated, offset])

  // Deep-link support: /cases/:id
  useEffect(() => {
    if (!routeCaseId) return
    const numId = Number(routeCaseId)
    if (!Number.isFinite(numId) || numId <= 0) return
    const existing = cases.find((c) => c.id === numId)
    if (existing) {
      if (editingId !== existing.id || !drawerOpen) populateForm(existing)
      return
    }
    void api
      .getTestCase(numId)
      .then((tc) => populateForm(tc))
      .catch((e) => setError(e instanceof Error ? e.message : t.errors.loadTestCases))
  }, [routeCaseId])

  const updateStep = (idx: number, field: keyof TestStep, value: string) => {
    setSteps((prev) => prev.map((s, i) => (i === idx ? { ...s, [field]: value } : s)))
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
      format === 'steps' ? steps.filter((s) => s.action.trim() !== '').map((s, i) => ({ ...s, position: i + 1 })) : []
    try {
      const payload = {
        product_id: productId,
        category_id: categoryId,
        summary: summary.trim(),
        preconditions: preconditions.trim(),
        format,
        gherkin_body: format === 'steps' ? '' : gherkinBody,
        priority,
        status,
        is_automated: isAutomated,
        script: isAutomated ? script.trim() : '',
        arguments: isAutomated ? scriptArgs.trim() : '',
        steps: validSteps,
      }
      if (editingId !== null) {
        const updated = await api.updateTestCase(editingId, payload)
        notify(t.testcases.savedToast(updated.id))
        void api
          .listTestCaseRevisions(updated.id)
          .then((r) => setRevisions(r.items))
          .catch(() => {})
      } else {
        const created = await api.createTestCase(payload)
        notify(t.testcases.createdToast(created.id))
        closeDrawer()
      }
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errors.saveTestCase)
    }
  }

  const handleBulkUpdate = async () => {
    if (selectedIds.length === 0 || (!bulkStatus && !bulkPriority)) return
    try {
      const res = await api.bulkUpdateTestCases({
        ids: selectedIds,
        status: bulkStatus || undefined,
        priority: bulkPriority || undefined,
      })
      notify(t.testcases.bulkUpdatedToast(res.updated))
      setSelectedIds([])
      setBulkStatus('')
      setBulkPriority('')
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errors.saveTestCase)
    }
  }

  const handleImport = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!importFile || !activeProductId || importBusy) return
    setImportBusy(true)
    try {
      const res = await api.importTestCases(importFile, activeProductId)
      setImportResult(res)
      if (res.failed > 0) {
        notify(t.testcases.importPartial(res.imported, res.failed), 'error')
      } else {
        notify(t.testcases.importToast(res.imported))
      }
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : t.testcases.importFailed)
    } finally {
      setImportBusy(false)
    }
  }

  const loadTrash = async () => {
    try {
      const res = await api.listTrashCases({ product_id: activeProductId ?? undefined, limit: 50 })
      setTrashItems(res.items)
      setTrashTotal(res.total)
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errors.loadTestCases)
    }
  }

  const handleRestore = async (id: number) => {
    try {
      await api.restoreTestCase(id)
      notify(t.testcases.restoredToast(id))
      await Promise.all([load(), loadTrash()])
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errors.saveTestCase)
    }
  }

  const handlePurge = async () => {
    if (!purgeTarget) return
    const target = purgeTarget
    setPurgeTarget(null)
    try {
      await api.purgeTestCase(target.id)
      notify(t.testcases.purgedToast(target.id))
      await loadTrash()
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errors.saveTestCase)
    }
  }

  const applyTemplate = (tplId: number) => {
    const tpl = templates.find((x) => x.id === tplId)
    if (!tpl) return
    if (tpl.body.includes('Scenario:') || tpl.body.includes('Feature:') || tpl.body.includes('Given ')) {
      setFormat('gherkin')
      setGherkinBody(tpl.body)
    } else {
      setPreconditions(tpl.body)
    }
  }

  const toggleSelect = (id: number) => {
    setSelectedIds((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]))
  }

  const productMap = new Map(products.map((p) => [p.id, p.name]))
  const categoryMap = new Map(categories.map((c) => [c.id, c.name]))

  return (
    <>
      <section>
        {error && <p className="alert-error">{error}</p>}

        {/* Filter & action bar */}
        <div className="filter-bar">
          <div style={{ position: 'relative', flex: 1, minWidth: 200 }}>
            <Search
              size={14}
              style={{
                position: 'absolute',
                left: 10,
                top: '50%',
                transform: 'translateY(-50%)',
                color: 'var(--muted)',
              }}
            />
            <input
              className="input"
              placeholder={t.testcases.summaryPlaceholder}
              value={filterQ}
              onChange={(e) => setFilterQ(e.target.value)}
              style={{ width: '100%', paddingLeft: 30 }}
            />
          </div>

          <select className="select" value={filterCategory} onChange={(e) => setFilterCategory(Number(e.target.value))}>
            <option value={0}>{t.testcases.allCategories}</option>
            {categories.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </select>

          <select className="select" value={filterPriority} onChange={(e) => setFilterPriority(e.target.value)}>
            <option value="">{t.testcases.allPriorities}</option>
            <option value="critical">critical</option>
            <option value="high">high</option>
            <option value="medium">medium</option>
            <option value="low">low</option>
          </select>

          <select className="select" value={filterStatus} onChange={(e) => setFilterStatus(e.target.value)}>
            <option value="">{t.testcases.allStatuses}</option>
            <option value="confirmed">confirmed</option>
            <option value="draft">draft</option>
            <option value="disabled">disabled</option>
          </select>

          <select
            className="select"
            value={filterAutomated}
            onChange={(e) => setFilterAutomated(e.target.value as '' | 'true' | 'false')}
          >
            <option value="">{t.testcases.allTypes}</option>
            <option value="true">{t.testcases.automatedOnly}</option>
            <option value="false">{t.testcases.manualOnly}</option>
          </select>

          <ExportButton
            path="/api/v1/testcases"
            filename="testcases"
            params={{
              q: filterQ || undefined,
              product_id: activeProductId ?? undefined,
              category_id: filterCategory || undefined,
              priority: filterPriority || undefined,
              status: filterStatus || undefined,
              is_automated: filterAutomated || undefined,
            }}
            formats={['csv', 'json', 'md', 'xlsx', 'feature']}
            onError={(msg) => notify(msg, 'error')}
          />
          <button
            type="button"
            className={`btn ${showImport ? 'btn-primary' : 'btn-ghost'}`}
            onClick={() => {
              setShowImport((v) => !v)
              setImportResult(null)
            }}
            aria-expanded={showImport}
          >
            <Upload size={15} />
            {t.testcases.importCases}
          </button>
          <button
            type="button"
            className={`btn ${showTrash ? 'btn-primary' : 'btn-ghost'}`}
            onClick={() => {
              setShowTrash((v) => !v)
              if (!showTrash) void loadTrash()
            }}
            aria-expanded={showTrash}
          >
            <Trash2 size={15} />
            {t.testcases.trash}
          </button>
          <button type="button" className="btn btn-primary" onClick={openNewDrawer}>
            <Plus size={15} />
            {t.testcases.newCase}
          </button>
        </div>

        {/* Import form */}
        {showImport && (
          <form onSubmit={handleImport} className="card" style={{ display: 'grid', gap: '0.6rem' }}>
            <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap', alignItems: 'center' }}>
              <input
                type="file"
                accept=".csv,.json,text/csv,application/json"
                required
                onChange={(e) => setImportFile(e.target.files?.[0] ?? null)}
              />
              <button type="submit" className="btn btn-primary" disabled={!importFile || importBusy}>
                <Upload size={15} />
                {t.testcases.importCases}
              </button>
            </div>
            <p className="muted" style={{ margin: 0 }}>
              {t.testcases.importHint}
            </p>
            {importResult && importResult.errors && importResult.errors.length > 0 && (
              <ul style={{ margin: 0, paddingLeft: '1.2rem', color: 'var(--danger, #b00)' }}>
                {importResult.errors.map((rowErr) => (
                  <li key={rowErr.row}>{t.testcases.importRowError(rowErr.row, rowErr.error)}</li>
                ))}
              </ul>
            )}
          </form>
        )}

        {/* Trash (soft-deleted cases) */}
        {showTrash && (
          <div className="card" style={{ display: 'grid', gap: '0.6rem' }}>
            <strong>{t.testcases.trashTitle(trashTotal)}</strong>
            {trashItems.length === 0 ? (
              <p className="muted" style={{ margin: 0 }}>
                {t.testcases.trashEmpty}
              </p>
            ) : (
              <table className="table">
                <thead>
                  <tr>
                    <th>#</th>
                    <th>{t.common.summary}</th>
                    <th>{t.common.status}</th>
                    <th></th>
                  </tr>
                </thead>
                <tbody>
                  {trashItems.map((tc) => (
                    <tr key={tc.id}>
                      <td>#{tc.id}</td>
                      <td>{tc.summary}</td>
                      <td>{tc.status}</td>
                      <td style={{ display: 'flex', gap: '0.4rem', justifyContent: 'flex-end' }}>
                        <button type="button" className="btn btn-ghost" onClick={() => void handleRestore(tc.id)}>
                          <RotateCcw size={14} />
                          {t.testcases.restore}
                        </button>
                        <button
                          type="button"
                          className="btn btn-ghost"
                          style={{ color: 'var(--danger, #b00)' }}
                          onClick={() => setPurgeTarget({ id: tc.id, summary: tc.summary })}
                        >
                          <Trash2 size={14} />
                          {t.testcases.purge}
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        )}

        {/* Bulk action bar */}
        {selectedIds.length > 0 && (
          <div className="filter-bar" style={{ background: 'var(--primary-soft)', borderColor: 'var(--primary)' }}>
            <strong>
              {selectedIds.length} {t.common.selected}
            </strong>
            <select className="select" value={bulkStatus} onChange={(e) => setBulkStatus(e.target.value)}>
              <option value="">{t.common.status}…</option>
              <option value="draft">draft</option>
              <option value="confirmed">confirmed</option>
              <option value="disabled">disabled</option>
            </select>
            <select className="select" value={bulkPriority} onChange={(e) => setBulkPriority(e.target.value)}>
              <option value="">{t.common.priority}…</option>
              <option value="low">low</option>
              <option value="medium">medium</option>
              <option value="high">high</option>
              <option value="critical">critical</option>
            </select>
            <button
              type="button"
              className="btn btn-primary btn-sm"
              onClick={() => void handleBulkUpdate()}
              disabled={!bulkStatus && !bulkPriority}
            >
              {t.testcases.applyBulk}
            </button>
            <ExportButton
              path="/api/v1/testcases"
              filename="testcases"
              ids={selectedIds}
              formats={['csv', 'json', 'md', 'xlsx', 'feature']}
              small
              onError={(msg) => notify(msg, 'error')}
            />
            <button type="button" className="btn btn-ghost btn-sm" onClick={() => setSelectedIds([])}>
              {t.common.clear}
            </button>
          </div>
        )}

        {/* Main table */}
        <div className="table-wrap">
          <table className="data-table">
            <thead>
              <tr>
                <th style={{ width: 36 }}>
                  <input
                    type="checkbox"
                    checked={cases.length > 0 && selectedIds.length === cases.length}
                    onChange={(e) => setSelectedIds(e.target.checked ? cases.map((c) => c.id) : [])}
                  />
                </th>
                <th style={{ width: 70 }}>{t.common.id}</th>
                <th>{t.common.summary}</th>
                <th>{t.testcases.productCategory}</th>
                <th>{t.common.priority}</th>
                <th>{t.common.status}</th>
                <th>{t.common.type}</th>
                <th style={{ textAlign: 'right' }}>{t.common.actions}</th>
              </tr>
            </thead>
            <tbody>
              {cases.map((tc) => (
                <tr
                  key={tc.id}
                  className={`clickable ${editingId === tc.id && drawerOpen ? 'selected' : ''}`}
                  onClick={() => {
                    populateForm(tc)
                    navigate(`/cases/${tc.id}`, { replace: true })
                  }}
                >
                  <td onClick={(e) => e.stopPropagation()}>
                    <input type="checkbox" checked={selectedIds.includes(tc.id)} onChange={() => toggleSelect(tc.id)} />
                  </td>
                  <td style={{ fontFamily: 'var(--font-mono)', color: 'var(--muted)' }}>#{tc.id}</td>
                  <td>
                    <div style={{ fontWeight: 500 }}>{tc.summary}</div>
                    <div style={{ fontSize: '0.75rem', color: 'var(--muted)' }}>
                      {tc.format === 'steps' ? `${tc.steps.length} ${t.testcases.stepsCount}` : tc.format}
                    </div>
                  </td>
                  <td style={{ fontSize: '0.8rem', color: 'var(--muted)' }}>
                    {productMap.get(tc.product_id) ?? `#${tc.product_id}`}
                    {tc.category_id ? ` · ${categoryMap.get(tc.category_id) ?? `#${tc.category_id}`}` : ''}
                  </td>
                  <td>
                    <span className={`badge pill-${tc.priority}`}>{tc.priority}</span>
                  </td>
                  <td>
                    <span className={`badge pill-${tc.status}`}>{tc.status}</span>
                  </td>
                  <td>
                    {tc.is_automated ? (
                      <span className="badge pill-running">
                        <Bot size={12} /> {t.common.automated}
                      </span>
                    ) : (
                      <span className="badge">{t.common.manual}</span>
                    )}
                  </td>
                  <td style={{ textAlign: 'right' }} onClick={(e) => e.stopPropagation()}>
                    <div style={{ display: 'inline-flex', gap: '0.25rem' }}>
                      <button
                        type="button"
                        className="btn btn-ghost btn-sm"
                        title={t.common.clone}
                        onClick={async () => {
                          try {
                            await api.cloneTestCase(tc.id)
                            notify(t.testcases.clonedToast(tc.id))
                            await load()
                          } catch (err) {
                            setError(err instanceof Error ? err.message : t.errors.saveTestCase)
                          }
                        }}
                      >
                        <Copy size={14} />
                      </button>
                      <button
                        type="button"
                        className="btn btn-ghost btn-sm"
                        style={{ color: 'var(--danger)' }}
                        title={t.common.delete}
                        onClick={() => setDeleteTarget({ id: tc.id, summary: tc.summary })}
                      >
                        <Trash2 size={14} />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
              {cases.length === 0 && !loading && (
                <tr>
                  <td colSpan={8} style={{ padding: 0, border: 'none' }}>
                    <EmptyState
                      title={t.testcases.empty}
                      description={t.testcases.emptyHint}
                      actionLabel={t.testcases.newCaseAction}
                      onAction={openNewDrawer}
                    />
                  </td>
                </tr>
              )}
              {loading && cases.length === 0 && (
                <tr>
                  <td colSpan={8}>
                    <LoadingSkeleton rows={5} />
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>

        {/* Pagination */}
        {total > PAGE_SIZE && (
          <div
            style={{
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
              marginTop: '0.85rem',
              fontSize: '0.82rem',
              color: 'var(--muted)',
            }}
          >
            <span>{t.testcases.showingRange(offset + 1, Math.min(offset + PAGE_SIZE, total), total)}</span>
            <div style={{ display: 'flex', gap: '0.4rem' }}>
              <button
                type="button"
                className="btn btn-sm"
                disabled={offset === 0}
                onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
              >
                {t.common.prev}
              </button>
              <button
                type="button"
                className="btn btn-sm"
                disabled={offset + PAGE_SIZE >= total}
                onClick={() => setOffset(offset + PAGE_SIZE)}
              >
                {t.common.next}
              </button>
            </div>
          </div>
        )}

        {/* Slide-over Drawer */}
        {drawerOpen && (
          <div className="drawer-backdrop" onClick={closeDrawer}>
            <div className="drawer-panel" role="dialog" aria-modal="true" onClick={(e) => e.stopPropagation()}>
              <div className="drawer-header">
                <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                  <h3 style={{ margin: 0, fontSize: '1rem' }}>
                    {editingId !== null ? `${t.testcases.editCase} #${editingId}` : t.testcases.newCase}
                  </h3>
                  {editingId !== null && (
                    <button
                      type="button"
                      className="btn btn-ghost btn-sm"
                      title={t.testcases.copyLink}
                      onClick={() => {
                        void navigator.clipboard?.writeText(`${window.location.origin}/cases/${editingId}`)
                        notify(t.testcases.linkCopied)
                      }}
                    >
                      <Link2 size={14} />
                    </button>
                  )}
                  {editingId !== null && (
                    <button
                      type="button"
                      className="btn btn-ghost btn-sm"
                      title={t.common.exportMd}
                      onClick={async () => {
                        try {
                          await downloadExport(
                            `/api/v1/testcases/${editingId}/export`,
                            'md',
                            `testcase-${editingId}.md`,
                          )
                        } catch (err) {
                          notify(err instanceof Error ? err.message : t.common.exportFailed, 'error')
                        }
                      }}
                    >
                      <FileDown size={14} />
                    </button>
                  )}
                </div>
                <button
                  type="button"
                  className="btn btn-ghost btn-icon"
                  onClick={closeDrawer}
                  aria-label={t.common.close}
                >
                  <X size={17} />
                </button>
              </div>

              {editingId !== null && (
                <div className="tabs-bar" style={{ padding: '0 1.25rem', marginBottom: 0 }}>
                  <button
                    type="button"
                    className={`tab-btn ${drawerTab === 'details' ? 'active' : ''}`}
                    onClick={() => setDrawerTab('details')}
                  >
                    {t.common.details}
                  </button>
                  <button
                    type="button"
                    className={`tab-btn ${drawerTab === 'comments' ? 'active' : ''}`}
                    onClick={() => setDrawerTab('comments')}
                  >
                    <MessageSquare size={14} /> {t.common.comments}
                  </button>
                  <button
                    type="button"
                    className={`tab-btn ${drawerTab === 'attachments' ? 'active' : ''}`}
                    onClick={() => setDrawerTab('attachments')}
                  >
                    <Paperclip size={14} /> {t.attachments.title}
                  </button>
                  <button
                    type="button"
                    className={`tab-btn ${drawerTab === 'revisions' ? 'active' : ''}`}
                    onClick={() => setDrawerTab('revisions')}
                  >
                    <History size={14} /> {t.testcases.revisions} ({revisions.length})
                  </button>
                </div>
              )}

              <div className="drawer-body">
                {drawerTab === 'details' && (
                  <form onSubmit={handleSubmit} style={{ display: 'grid', gap: '0.85rem' }}>
                    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.5rem' }}>
                      <select
                        className="select"
                        value={productId}
                        onChange={(e) => setProductId(Number(e.target.value))}
                      >
                        {products.map((p) => (
                          <option key={p.id} value={p.id}>
                            {p.name}
                          </option>
                        ))}
                      </select>
                      <select
                        className="select"
                        value={categoryId ?? ''}
                        onChange={(e) => setCategoryId(e.target.value ? Number(e.target.value) : undefined)}
                      >
                        <option value="">{t.testcases.noCategory}</option>
                        {categories.map((c) => (
                          <option key={c.id} value={c.id}>
                            {c.name}
                          </option>
                        ))}
                      </select>
                    </div>

                    <input
                      className="input"
                      placeholder={t.testcases.summaryPlaceholder}
                      value={summary}
                      onChange={(e) => setSummary(e.target.value)}
                      required
                    />

                    <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}>
                      <select
                        className="select"
                        value={priority}
                        onChange={(e) => setPriority(e.target.value as TestCase['priority'])}
                      >
                        <option value="low">low</option>
                        <option value="medium">medium</option>
                        <option value="high">high</option>
                        <option value="critical">critical</option>
                      </select>
                      <select
                        className="select"
                        value={status}
                        onChange={(e) => setStatus(e.target.value as TestCase['status'])}
                      >
                        <option value="draft">draft</option>
                        <option value="confirmed">confirmed</option>
                        <option value="disabled">disabled</option>
                      </select>
                      <select
                        className="select"
                        value={format}
                        onChange={(e) => setFormat(e.target.value as 'steps' | 'gherkin' | 'markdown' | 'checklist')}
                      >
                        <option value="steps">{t.testcases.formatSteps}</option>
                        <option value="gherkin">{t.testcases.formatGherkin}</option>
                        <option value="markdown">{t.testcases.formatMarkdown}</option>
                        <option value="checklist">{t.testcases.formatChecklist}</option>
                      </select>
                      {templates.length > 0 && (
                        <select
                          className="select"
                          defaultValue=""
                          onChange={(e) => {
                            if (e.target.value) applyTemplate(Number(e.target.value))
                            e.target.value = ''
                          }}
                        >
                          <option value="">{t.testcases.applyTemplate}</option>
                          {templates.map((tpl) => (
                            <option key={tpl.id} value={tpl.id}>
                              {tpl.name}
                            </option>
                          ))}
                        </select>
                      )}
                    </div>

                    <textarea
                      className="textarea"
                      rows={2}
                      placeholder={t.testcases.preconditionsPlaceholder}
                      value={preconditions}
                      onChange={(e) => setPreconditions(e.target.value)}
                    />

                    <div style={{ display: 'grid', gap: '0.45rem' }}>
                      <label
                        style={{ display: 'inline-flex', alignItems: 'center', gap: '0.4rem', fontSize: '0.84rem' }}
                      >
                        <input
                          type="checkbox"
                          checked={isAutomated}
                          onChange={(e) => setIsAutomated(e.target.checked)}
                        />
                        {t.testcases.automatedTest}
                      </label>
                      {isAutomated && (
                        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.45rem' }}>
                          <input
                            className="input"
                            placeholder={t.testcases.scriptPlaceholder}
                            value={script}
                            onChange={(e) => setScript(e.target.value)}
                          />
                          <input
                            className="input"
                            placeholder={t.testcases.argsPlaceholder}
                            value={scriptArgs}
                            onChange={(e) => setScriptArgs(e.target.value)}
                          />
                        </div>
                      )}
                    </div>

                    {format === 'markdown' ? (
                      <div style={{ display: 'grid', gap: '0.4rem' }}>
                        <textarea
                          className="textarea"
                          rows={7}
                          placeholder={t.testcases.mdBodyPlaceholder}
                          value={gherkinBody}
                          onChange={(e) => setGherkinBody(e.target.value)}
                        />
                        <div>
                          <button
                            type="button"
                            className="btn btn-sm"
                            onClick={async () => {
                              try {
                                const res = await api.renderMarkdown(gherkinBody)
                                setMdPreview(res.html)
                              } catch {
                                setMdPreview('')
                              }
                            }}
                          >
                            {t.testcases.preview}
                          </button>
                        </div>
                        {mdPreview && (
                          <div
                            className="markdown-body"
                            style={{ border: '1px solid var(--border)', borderRadius: 6, padding: '0.6rem' }}
                            dangerouslySetInnerHTML={{ __html: mdPreview }}
                          />
                        )}
                      </div>
                    ) : format === 'checklist' ? (
                      <textarea
                        className="textarea"
                        rows={7}
                        placeholder={t.testcases.checklistPlaceholder}
                        value={gherkinBody}
                        onChange={(e) => setGherkinBody(e.target.value)}
                      />
                    ) : format === 'gherkin' ? (
                      <textarea
                        className="textarea"
                        rows={7}
                        placeholder={t.testcases.gherkinPlaceholder}
                        value={gherkinBody}
                        onChange={(e) => setGherkinBody(e.target.value)}
                      />
                    ) : (
                      <div style={{ display: 'grid', gap: '0.45rem' }}>
                        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                          <strong style={{ fontSize: '0.85rem' }}>{t.testcases.stepsLabel}</strong>
                          <button
                            type="button"
                            className="btn btn-sm"
                            onClick={() =>
                              setSteps((prev) => [
                                ...prev,
                                { position: prev.length + 1, action: '', expected_result: '' },
                              ])
                            }
                          >
                            <Plus size={13} /> {t.testcases.addStep}
                          </button>
                        </div>
                        {steps.map((s, idx) => (
                          <div key={idx} style={{ display: 'flex', gap: '0.35rem', alignItems: 'center' }}>
                            <span
                              style={{
                                width: 22,
                                color: 'var(--muted)',
                                fontSize: '0.8rem',
                                fontFamily: 'var(--font-mono)',
                              }}
                            >
                              {idx + 1}.
                            </span>
                            <input
                              className="input"
                              placeholder={t.testcases.actionPlaceholder}
                              value={s.action}
                              onChange={(e) => updateStep(idx, 'action', e.target.value)}
                              style={{ flex: 1 }}
                            />
                            <input
                              className="input"
                              placeholder={t.testcases.expectedPlaceholder}
                              value={s.expected_result ?? ''}
                              onChange={(e) => updateStep(idx, 'expected_result', e.target.value)}
                              style={{ flex: 1 }}
                            />
                            <button
                              type="button"
                              className="btn btn-ghost btn-sm"
                              onClick={() => moveStep(idx, -1)}
                              disabled={idx === 0}
                            >
                              <ChevronUp size={14} />
                            </button>
                            <button
                              type="button"
                              className="btn btn-ghost btn-sm"
                              onClick={() => moveStep(idx, 1)}
                              disabled={idx === steps.length - 1}
                            >
                              <ChevronDown size={14} />
                            </button>
                          </div>
                        ))}
                      </div>
                    )}

                    <div
                      style={{
                        display: 'flex',
                        gap: '0.5rem',
                        alignItems: 'center',
                        paddingTop: '0.4rem',
                        borderTop: '1px solid var(--border)',
                      }}
                    >
                      <button type="submit" className="btn btn-primary">
                        {t.testcases.save}
                      </button>
                      <div style={{ marginLeft: 'auto', display: 'flex', gap: '0.35rem' }}>
                        <input
                          className="input"
                          placeholder={t.testcases.newCategoryPlaceholder}
                          value={newCategory}
                          onChange={(e) => setNewCategory(e.target.value)}
                          style={{ width: 170, fontSize: '0.8rem' }}
                        />
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

                    {editingId !== null && (
                      <div
                        style={{
                          display: 'grid',
                          gap: '0.5rem',
                          paddingTop: '0.75rem',
                          borderTop: '1px solid var(--border)',
                        }}
                      >
                        <EntityTagsBar entity="testcases" entityId={editingId} />
                        <TestCaseComponentsBar caseId={editingId} productId={productId} />
                      </div>
                    )}
                  </form>
                )}

                {editingId !== null && drawerTab === 'comments' && (
                  <CommentsPanel entityType="test_case" entityId={editingId} />
                )}

                {editingId !== null && drawerTab === 'attachments' && (
                  <>
                    <AttachmentsPanel entityType="test_case" entityId={editingId} />
                    <div style={{ borderTop: '1px solid var(--border)', paddingTop: '0.75rem' }}>
                      <CaseDatasetsPanel caseId={editingId} />
                    </div>
                    <div style={{ borderTop: '1px solid var(--border)', paddingTop: '0.75rem' }}>
                      <CustomFieldsPanel entityType="test_case" entityId={editingId} />
                    </div>
                    <div style={{ borderTop: '1px solid var(--border)', paddingTop: '0.75rem' }}>
                      <VisualBaselinesPanel caseId={editingId} />
                    </div>
                  </>
                )}

                {editingId !== null && drawerTab === 'revisions' && (
                  <div style={{ display: 'grid', gap: '0.6rem' }}>
                    {revisions.length === 0 ? (
                      <p style={{ color: 'var(--muted)', margin: 0 }}>{t.testcases.noRevisions}</p>
                    ) : (
                      revisions.map((rev, idx) => {
                        const prev = revisions[idx + 1]
                        const changedSummary = prev && prev.summary !== rev.summary
                        const changedPriority = prev && prev.priority !== rev.priority
                        const changedStatus = prev && prev.status !== rev.status
                        return (
                          <div key={rev.id} className="card" style={{ padding: '0.8rem' }}>
                            <div
                              style={{
                                display: 'flex',
                                justifyContent: 'space-between',
                                alignItems: 'center',
                                marginBottom: '0.35rem',
                              }}
                            >
                              <strong>v{rev.version}</strong>
                              <span style={{ fontSize: '0.75rem', color: 'var(--muted)' }}>
                                {new Date(rev.created_at).toLocaleString()}
                              </span>
                            </div>
                            <div style={{ fontSize: '0.85rem', fontWeight: 500 }}>{rev.summary}</div>
                            <div style={{ display: 'flex', gap: '0.4rem', marginTop: '0.4rem', flexWrap: 'wrap' }}>
                              <span className={`badge pill-${rev.priority}`}>{rev.priority}</span>
                              <span className={`badge pill-${rev.status}`}>{rev.status}</span>
                              {changedSummary && (
                                <span className="badge pill-blocked">{t.testcases.summaryChanged}</span>
                              )}
                              {changedPriority && (
                                <span className="badge pill-blocked">
                                  {t.testcases.priorityChanged(prev.priority, rev.priority)}
                                </span>
                              )}
                              {changedStatus && (
                                <span className="badge pill-blocked">
                                  {t.testcases.statusChanged(prev.status, rev.status)}
                                </span>
                              )}
                            </div>
                          </div>
                        )
                      })
                    )}
                  </div>
                )}
              </div>
            </div>
          </div>
        )}
      </section>

      <ConfirmDialog
        open={deleteTarget !== null}
        title={t.testcases.confirmDelete}
        message={deleteTarget ? t.testcases.confirmDeleteMessage(deleteTarget.id, deleteTarget.summary) : ''}
        confirmLabel={t.common.delete}
        onCancel={() => setDeleteTarget(null)}
        onConfirm={async () => {
          if (!deleteTarget) return
          const target = deleteTarget
          setDeleteTarget(null)
          try {
            await api.deleteTestCase(target.id)
            notifyAction(t.testcases.deletedToast(target.id), t.common.undo, async () => {
              try {
                await api.restoreTestCase(target.id)
                notify(t.testcases.restoredToast(target.id))
                await Promise.all([load(), showTrash ? loadTrash() : Promise.resolve()])
              } catch (err) {
                setError(err instanceof Error ? err.message : t.errors.saveTestCase)
              }
            })
            if (editingId === target.id) closeDrawer()
            await load()
          } catch (err) {
            setError(err instanceof Error ? err.message : t.errors.saveTestCase)
          }
        }}
      />

      <ConfirmDialog
        open={purgeTarget !== null}
        title={t.testcases.purgeConfirmTitle}
        message={purgeTarget ? t.testcases.purgeConfirmMessage(purgeTarget.id, purgeTarget.summary) : ''}
        confirmLabel={t.testcases.purge}
        onCancel={() => setPurgeTarget(null)}
        onConfirm={() => void handlePurge()}
      />
    </>
  )
}
