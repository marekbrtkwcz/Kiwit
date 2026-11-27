import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import {
  Bot,
  Bug,
  CheckCircle2,
  Download,
  ExternalLink,
  FileCode2,
  Keyboard,
  Link2,
  Play,
  Plus,
  RefreshCw,
  Search,
  SkipForward,
  Square,
  Trash2,
  XCircle,
  Ban,
} from 'lucide-react'
import { api, VisualComparison, BugItem, Build, EnvironmentItem, ExecutionLinkItem, IssueTrackerItem, TestExecution, TestPlan, TestRun } from '../../api/client'
import { useWorkspace } from '../../context/WorkspaceContext'
import { ExportButton } from '../../components/ExportButton'
import { useI18n } from '../../i18n'
import { AttachmentsPanel } from '../attachments/AttachmentsPanel'
import { ScreenshotsGallery } from '../attachments/ScreenshotsGallery'
import { CustomFieldsPanel } from '../../components/CustomFieldsPanel'
import { CommentsPanel } from '../comments/CommentsPanel'
import { EntityTagsBar } from '../management/EntityTagsBar'

function ExecutionBugsPanel({ executionId, trackers }: { executionId: number; trackers: IssueTrackerItem[] }) {
  const { t } = useI18n()
  const { notify } = useWorkspace()
  const [bugs, setBugs] = useState<BugItem[]>([])
  const [mode, setMode] = useState<'none' | 'link' | 'report'>('none')
  const [key, setKey] = useState('')
  const [url, setUrl] = useState('')
  const [title, setTitle] = useState('')
  const [trackerId, setTrackerId] = useState<number>(trackers[0]?.id ?? 0)
  const [projectKey, setProjectKey] = useState('')
  const [apiToken, setApiToken] = useState('')
  const [reportBody, setReportBody] = useState('')
  const [err, setErr] = useState<string | null>(null)

  const load = async () => {
    try {
      const res = await api.listExecutionBugs(executionId)
      setBugs(res.items)
    } catch {
      // ignore
    }
  }

  useEffect(() => {
    void load()
  }, [executionId])

  useEffect(() => {
    if (!trackerId && trackers.length > 0) setTrackerId(trackers[0].id)
  }, [trackers, trackerId])

  const handleLink = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!key.trim() || !url.trim()) return
    try {
      await api.linkExecutionBug(executionId, { external_key: key.trim(), url: url.trim(), title: title.trim() })
      setKey('')
      setUrl('')
      setTitle('')
      setMode('none')
      setErr(null)
      notify(t.testruns.bugLinked)
      await load()
    } catch (e) {
      setErr(e instanceof Error ? e.message : t.testruns.linkBugFailed)
    }
  }

  const handleReport = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!trackerId || !title.trim()) return
    try {
      await api.reportExecutionBug(executionId, {
        tracker_id: trackerId,
        project_key: projectKey.trim() || undefined,
        api_token: apiToken.trim() || undefined,
        title: title.trim(),
        body: reportBody.trim() || undefined,
      })
      setTitle('')
      setReportBody('')
      setApiToken('')
      setMode('none')
      setErr(null)
      notify(t.testruns.issueCreated)
      await load()
    } catch (e) {
      setErr(e instanceof Error ? e.message : t.testruns.reportBugFailed)
    }
  }

  return (
    <div style={{ display: 'grid', gap: '0.45rem', fontSize: '0.82rem' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', flexWrap: 'wrap' }}>
        <span style={{ color: 'var(--muted)', display: 'inline-flex', alignItems: 'center', gap: 4 }}>
          <Bug size={14} /> {t.testruns.bugsLabel}
        </span>
        {bugs.map((b) => (
          <span key={b.id} className="badge" style={{ gap: 5 }}>
            <a href={b.url} target="_blank" rel="noreferrer" style={{ textDecoration: 'none', display: 'inline-flex', alignItems: 'center', gap: 3 }}>
              {b.external_key}{b.title ? `: ${b.title}` : ''} [{b.status}]
              <ExternalLink size={11} />
            </a>
            {b.tracker_id && (
              <button
                type="button"
                title={t.testruns.syncStatus}
                onClick={async () => {
                  try {
                    const updated = await api.syncExecutionBug(executionId, b.id)
                    setBugs((prev) => prev.map((x) => (x.id === updated.id ? updated : x)))
                    notify(t.testruns.synced(updated.external_key, updated.status))
                  } catch (e) {
                    setErr(e instanceof Error ? e.message : t.testruns.syncFailed)
                  }
                }}
                style={{ border: 'none', background: 'transparent', cursor: 'pointer', color: 'var(--primary-text)', padding: 0, display: 'inline-flex' }}
              >
                <RefreshCw size={11} />
              </button>
            )}
          </span>
        ))}
        <button type="button" className="btn btn-sm" onClick={() => setMode((m) => (m === 'link' ? 'none' : 'link'))}>
          {mode === 'link' ? t.common.cancel : t.testruns.linkBug}
        </button>
        {trackers.length > 0 && (
          <button type="button" className="btn btn-sm" onClick={() => setMode((m) => (m === 'report' ? 'none' : 'report'))}>
            {mode === 'report' ? t.common.cancel : t.testruns.reportToTracker}
          </button>
        )}
      </div>
      {err && <div className="alert-error" style={{ margin: 0 }}>{err}</div>}
      {mode === 'link' && (
        <form onSubmit={handleLink} style={{ display: 'flex', gap: '0.35rem', flexWrap: 'wrap' }}>
          <input className="input" placeholder={t.testruns.bugKeyPlaceholder} value={key} onChange={(e) => setKey(e.target.value)} required style={{ width: 120 }} />
          <input className="input" placeholder="https://..." value={url} onChange={(e) => setUrl(e.target.value)} required style={{ flex: 1, minWidth: 180 }} />
          <input className="input" placeholder={t.testruns.bugTitleOptionalPlaceholder} value={title} onChange={(e) => setTitle(e.target.value)} style={{ width: 160 }} />
          <button type="submit" className="btn btn-primary btn-sm"><Link2 size={13} /> {t.testruns.link}</button>
        </form>
      )}
      {mode === 'report' && (
        <form onSubmit={handleReport} style={{ display: 'grid', gap: '0.35rem' }}>
          <div style={{ display: 'flex', gap: '0.35rem', flexWrap: 'wrap' }}>
            <select className="select" value={trackerId} onChange={(e) => setTrackerId(Number(e.target.value))}>
              {trackers.map((tr) => (
                <option key={tr.id} value={tr.id}>{tr.name} ({tr.provider})</option>
              ))}
            </select>
            <input className="input" placeholder={t.testruns.issueTitlePlaceholder} value={title} onChange={(e) => setTitle(e.target.value)} required style={{ flex: 1, minWidth: 160 }} />
            <input className="input" placeholder={t.testruns.projectKeyPlaceholder} value={projectKey} onChange={(e) => setProjectKey(e.target.value)} style={{ width: 130 }} />
            <input className="input" type="password" placeholder={t.testruns.apiTokenPlaceholder} value={apiToken} onChange={(e) => setApiToken(e.target.value)} style={{ width: 150 }} />
          </div>
          <div style={{ display: 'flex', gap: '0.35rem' }}>
            <input className="input" placeholder={t.testruns.descOptionalPlaceholder} value={reportBody} onChange={(e) => setReportBody(e.target.value)} style={{ flex: 1 }} />
            <button type="submit" className="btn btn-primary btn-sm"><ExternalLink size={13} /> {t.testruns.createIssue}</button>
          </div>
        </form>
      )}
    </div>
  )
}

function ExecutionLinksPanel({ executionId }: { executionId: number }) {
  const { t } = useI18n()
  const { notify } = useWorkspace()
  const [links, setLinks] = useState<ExecutionLinkItem[]>([])
  const [open, setOpen] = useState(false)
  const [name, setName] = useState('')
  const [url, setUrl] = useState('')

  const load = async () => {
    try {
      const res = await api.listExecutionLinks(executionId)
      setLinks(res.items)
    } catch {
      setLinks([])
    }
  }

  useEffect(() => {
    void load()
  }, [executionId])

  return (
    <div style={{ display: 'grid', gap: '0.4rem', fontSize: '0.82rem' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', flexWrap: 'wrap' }}>
        <span style={{ color: 'var(--muted)', display: 'inline-flex', alignItems: 'center', gap: 4 }}>
          <Link2 size={14} /> {t.testruns.ciLinks}
        </span>
        {links.map((l) => (
          <span key={l.id} className="badge" style={{ gap: 5 }}>
            <a href={l.url} target="_blank" rel="noreferrer" style={{ textDecoration: 'none' }}>{l.name}</a>
            <button
              type="button"
              onClick={async () => {
                await api.deleteExecutionLink(executionId, l.id)
                await load()
              }}
              style={{ border: 'none', background: 'transparent', cursor: 'pointer', color: 'var(--muted)', padding: 0 }}
              title={t.testruns.removeLinkTitle}
            >
              ×
            </button>
          </span>
        ))}
        <button type="button" className="btn btn-sm" onClick={() => setOpen((v) => !v)}>
          {open ? t.common.cancel : t.testruns.addLink}
        </button>
      </div>
      {open && (
        <form
          onSubmit={async (e) => {
            e.preventDefault()
            if (!name.trim() || !url.trim()) return
            await api.addExecutionLink(executionId, { name: name.trim(), url: url.trim() })
            setName('')
            setUrl('')
            setOpen(false)
            notify(t.testruns.linkAttached)
            await load()
          }}
          style={{ display: 'flex', gap: '0.35rem', flexWrap: 'wrap' }}
        >
          <input className="input" placeholder={t.testruns.linkLabelPlaceholder} value={name} onChange={(e) => setName(e.target.value)} required style={{ width: 150 }} />
          <input className="input" placeholder="https://..." value={url} onChange={(e) => setUrl(e.target.value)} required style={{ flex: 1, minWidth: 180 }} />
          <button type="submit" className="btn btn-primary btn-sm">{t.common.add}</button>
        </form>
      )}
    </div>
  )
}

export function TestRunsView() {
  const { t } = useI18n()
  const { activeProductId, notify } = useWorkspace()
  const { id: routeRunId } = useParams<{ id: string }>()
  const navigate = useNavigate()

  const [plans, setPlans] = useState<TestPlan[]>([])
  const [builds, setBuilds] = useState<Build[]>([])
  const [environments, setEnvironments] = useState<EnvironmentItem[]>([])
  const [runs, setRuns] = useState<TestRun[]>([])
  const [trackers, setTrackers] = useState<IssueTrackerItem[]>([])
  const [selectedRunId, setSelectedRunId] = useState<number | null>(routeRunId ? Number(routeRunId) : null)
  const [executions, setExecutions] = useState<TestExecution[]>([])
  const [activeExecId, setActiveExecId] = useState<number | null>(null)
  const [selectedExecIds, setSelectedExecIds] = useState<number[]>([])
  const [visualResults, setVisualResults] = useState<Record<number, VisualComparison[]>>({})
  const [aiVerdicts, setAiVerdicts] = useState<Record<number, { verdict: string; confidence: number; explanation: string } | 'busy'>>({})
  const [bulkStatus, setBulkStatus] = useState('passed')

  const [filterStatus, setFilterStatus] = useState<'' | 'running' | 'stopped'>('')
  const [filterQ, setFilterQ] = useState('')
  const [execFilter, setExecFilter] = useState<string>('')

  const [showCreate, setShowCreate] = useState(false)
  const [showJUnit, setShowJUnit] = useState(false)
  const [planId, setPlanId] = useState<number>(0)
  const [buildId, setBuildId] = useState<number>(0)
  const [environmentId, setEnvironmentId] = useState<number>(0)
  const [summary, setSummary] = useState('')
  const [junitXML, setJunitXML] = useState('')
  const [error, setError] = useState<string | null>(null)

  const loadRuns = async () => {
    try {
      const [tpRes, trRes, itRes, envRes] = await Promise.all([
        api.listTestPlans({ product_id: activeProductId ?? undefined }),
        api.listTestRuns({
          product_id: activeProductId ?? undefined,
          status: filterStatus || undefined,
          q: filterQ || undefined,
        }),
        api.listIssueTrackers().catch(() => ({ items: [] as IssueTrackerItem[] })),
        api.listEnvironments().catch(() => ({ items: [] as EnvironmentItem[] })),
      ])
      setPlans(tpRes.items)
      setTrackers(itRes.items)
      setEnvironments(envRes.items)
      if (tpRes.items.length > 0 && !tpRes.items.some((p) => p.id === planId)) {
        setPlanId(tpRes.items[0].id)
      }
      setRuns(trRes.items)
      if (routeRunId) {
        setSelectedRunId(Number(routeRunId))
      } else if (trRes.items.length > 0 && (!selectedRunId || !trRes.items.some((r) => r.id === selectedRunId))) {
        setSelectedRunId(trRes.items[0].id)
      }
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : t.errors.loadTestRuns)
    }
  }

  useEffect(() => {
    void loadRuns()
  }, [activeProductId, filterStatus, filterQ])

  useEffect(() => {
    if (routeRunId) {
      const num = Number(routeRunId)
      if (Number.isFinite(num) && num > 0) setSelectedRunId(num)
    }
  }, [routeRunId])

  useEffect(() => {
    const plan = plans.find((p) => p.id === planId)
    if (!plan) {
      setBuilds([])
      setBuildId(0)
      return
    }
    void api
      .listBuilds(plan.version_id)
      .then((res) => {
        setBuilds(res.items)
        setBuildId(res.items.length > 0 ? res.items[0].id : 0)
      })
      .catch((e) => setError(e instanceof Error ? e.message : t.errors.loadTestRuns))
  }, [planId, plans])

  useEffect(() => {
    if (selectedRunId === null) {
      setExecutions([])
      setActiveExecId(null)
      return
    }
    void api
      .listExecutions(selectedRunId)
      .then((res) => {
        setExecutions(res.items)
        setActiveExecId(res.items.length > 0 ? res.items[0].id : null)
      })
      .catch((e) => setError(e instanceof Error ? e.message : t.errors.loadTestRuns))
  }, [selectedRunId])

  const filteredExecs = executions.filter((ex) => !execFilter || ex.status === execFilter)
  const activeExec = filteredExecs.find((ex) => ex.id === activeExecId) ?? filteredExecs[0] ?? null

  const handleStatus = async (execId: number, status: TestExecution['status']) => {
    try {
      await api.updateExecution(execId, { status })
      setExecutions((prev) => prev.map((ex) => (ex.id === execId ? { ...ex, status } : ex)))
      notify(t.testruns.statusChanged(execId, status))
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errors.updateStatus)
    }
  }

  // Keyboard shortcuts for Test Runner (P, F, B, S, J, K)
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.ctrlKey || e.metaKey || e.altKey) return
      const tag = (e.target as HTMLElement | null)?.tagName
      if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return
      if (!activeExec || filteredExecs.length === 0) return

      const idx = filteredExecs.findIndex((x) => x.id === activeExec.id)
      const k = e.key.toLowerCase()
      if (k === 'p') {
        e.preventDefault()
        void handleStatus(activeExec.id, 'passed')
      } else if (k === 'f') {
        e.preventDefault()
        void handleStatus(activeExec.id, 'failed')
      } else if (k === 'b') {
        e.preventDefault()
        void handleStatus(activeExec.id, 'blocked')
      } else if (k === 's') {
        e.preventDefault()
        void handleStatus(activeExec.id, 'skipped')
      } else if (k === 'j' && idx < filteredExecs.length - 1) {
        e.preventDefault()
        setActiveExecId(filteredExecs[idx + 1].id)
      } else if (k === 'k' && idx > 0) {
        e.preventDefault()
        setActiveExecId(filteredExecs[idx - 1].id)
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [activeExec, filteredExecs])

  const handleCreateRun = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!summary.trim() || !planId || !buildId) return
    try {
      const created = await api.createTestRun({
        test_plan_id: planId,
        build_id: buildId,
        environment_id: environmentId || undefined,
        summary: summary.trim(),
      })
      setSummary('')
      setShowCreate(false)
      setSelectedRunId(created.id)
      navigate(`/runs/${created.id}`, { replace: true })
      notify(t.testruns.startedToast(created.id))
      await loadRuns()
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errors.createTestRun)
    }
  }

  const handleImportJUnit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!planId || !buildId || !junitXML.trim()) return
    try {
      const res = await api.importJUnitXML({
        test_plan_id: planId,
        build_id: buildId,
        summary: summary.trim() || t.testruns.junitImportName(new Date().toLocaleDateString()),
        xml: junitXML,
      })
      setJunitXML('')
      setShowJUnit(false)
      setSelectedRunId(res.test_run.id)
      navigate(`/runs/${res.test_run.id}`, { replace: true })
      notify(t.testruns.importedToast(res.results.length, res.test_run.id))
      await loadRuns()
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errors.createTestRun)
    }
  }

  const selectedRun = runs.find((r) => r.id === selectedRunId) ?? null
  const counts = executions.reduce(
    (acc, ex) => {
      acc[ex.status] = (acc[ex.status] ?? 0) + 1
      return acc
    },
    {} as Record<string, number>,
  )
  const totalExecs = executions.length || 1

  return (
    <section style={{ display: 'grid', gap: '1rem' }}>
      {error && <p className="alert-error">{error}</p>}

      {/* Top filter & actions */}
      <div className="filter-bar" style={{ marginBottom: 0 }}>
        <div style={{ position: 'relative', flex: 1, minWidth: 180 }}>
          <Search size={14} style={{ position: 'absolute', left: 10, top: '50%', transform: 'translateY(-50%)', color: 'var(--muted)' }} />
          <input
            className="input"
            placeholder={t.testruns.summaryPlaceholder}
            value={filterQ}
            onChange={(e) => setFilterQ(e.target.value)}
            style={{ width: '100%', paddingLeft: 30 }}
          />
        </div>
        <select className="select" value={filterStatus} onChange={(e) => setFilterStatus(e.target.value as '' | 'running' | 'stopped')}>
          <option value="">{t.testruns.allRuns}</option>
          <option value="running">{t.testruns.runningRuns}</option>
          <option value="stopped">{t.testruns.stoppedRuns}</option>
        </select>
        <button type="button" className="btn btn-primary" onClick={() => { setShowCreate((v) => !v); setShowJUnit(false) }}>
          <Plus size={15} /> {t.testruns.create}
        </button>
        <button type="button" className="btn" onClick={() => { setShowJUnit((v) => !v); setShowCreate(false) }}>
          <FileCode2 size={15} /> {t.testruns.importJUnit}
        </button>
        <ExportButton
          path="/api/v1/testruns"
          filename="testruns"
          params={{
            product_id: activeProductId ?? undefined,
            status: filterStatus || undefined,
            q: filterQ || undefined,
          }}
          onError={(msg) => notify(msg, 'error')}
        />
      </div>

      {showCreate && (
        <form onSubmit={handleCreateRun} className="card" style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap', alignItems: 'center' }}>
          <select className="select" value={planId} onChange={(e) => setPlanId(Number(e.target.value))}>
            {plans.map((tp) => (
              <option key={tp.id} value={tp.id}>{tp.name}</option>
            ))}
          </select>
          <select className="select" value={buildId} onChange={(e) => setBuildId(Number(e.target.value))}>
            {builds.length === 0 && <option value={0}>{t.testruns.noBuilds}</option>}
            {builds.map((b) => (
              <option key={b.id} value={b.id}>{b.name} (#{b.id})</option>
            ))}
          </select>
          {environments.length > 0 && (
            <select className="select" value={environmentId} onChange={(e) => setEnvironmentId(Number(e.target.value))}>
              <option value={0}>{t.testruns.noEnv}</option>
              {environments.map((env) => (
                <option key={env.id} value={env.id}>{env.name}</option>
              ))}
            </select>
          )}
          <input className="input" placeholder={t.testruns.summaryPlaceholder} value={summary} onChange={(e) => setSummary(e.target.value)} style={{ flex: 1, minWidth: 200 }} required />
          <button type="submit" disabled={!planId || !buildId} className="btn btn-primary">
            <Play size={14} /> {t.testruns.start}
          </button>
        </form>
      )}

      {showJUnit && (
        <form onSubmit={handleImportJUnit} className="card" style={{ display: 'grid', gap: '0.6rem' }}>
          <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap', alignItems: 'center' }}>
            <select className="select" value={planId} onChange={(e) => setPlanId(Number(e.target.value))}>
              {plans.map((tp) => (
                <option key={tp.id} value={tp.id}>{tp.name}</option>
              ))}
            </select>
            <select className="select" value={buildId} onChange={(e) => setBuildId(Number(e.target.value))}>
              {builds.length === 0 && <option value={0}>{t.testruns.noBuilds}</option>}
              {builds.map((b) => (
                <option key={b.id} value={b.id}>{b.name} (#{b.id})</option>
              ))}
            </select>
            <input
              type="file"
              accept=".xml,text/xml,application/xml"
              onChange={async (e) => {
                const f = e.target.files?.[0]
                if (f) setJunitXML(await f.text())
              }}
            />
          </div>
          <textarea
            className="textarea"
            rows={5}
            placeholder='<testsuite name="suite"><testcase name="Login works"/></testsuite>'
            value={junitXML}
            onChange={(e) => setJunitXML(e.target.value)}
            required
          />
          <div>
            <button type="submit" disabled={!planId || !buildId || !junitXML.trim()} className="btn btn-primary btn-sm">
              {t.testruns.importXmlBtn}
            </button>
          </div>
        </form>
      )}

      {/* Main split: Runs sidebar + Execution Runner */}
      <div className="runs-layout">
        <div className="card" style={{ padding: '0.75rem', display: 'grid', gap: '0.45rem' }}>
          <div style={{ fontSize: '0.75rem', fontWeight: 600, color: 'var(--muted)', textTransform: 'uppercase', padding: '0 0.25rem' }}>
            {t.testruns.runsCount(runs.length)}
          </div>
          {runs.map((r) => (
            <div
              key={r.id}
              onClick={() => {
                setSelectedRunId(r.id)
                navigate(`/runs/${r.id}`, { replace: true })
              }}
              style={{
                padding: '0.65rem 0.75rem',
                borderRadius: 8,
                border: selectedRunId === r.id ? '1px solid var(--primary)' : '1px solid var(--border)',
                background: selectedRunId === r.id ? 'var(--primary-soft)' : 'var(--surface)',
                cursor: 'pointer',
              }}
            >
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: '0.4rem' }}>
                <strong style={{ fontSize: '0.86rem' }}>{r.summary}</strong>
                <span className={`badge ${r.stopped_at ? 'pill-skipped' : 'pill-running'}`}>
                  {r.stopped_at ? t.testruns.stoppedRuns : t.testruns.runningRuns}
                </span>
              </div>
              <div style={{ fontSize: '0.75rem', color: 'var(--muted)', marginTop: 3 }}>
                #{r.id} · Plan #{r.test_plan_id} · Build #{r.build_id}
              </div>
            </div>
          ))}
          {runs.length === 0 && <p style={{ color: 'var(--muted)', margin: '0.5rem', fontSize: '0.84rem' }}>{t.testruns.emptyRuns}</p>}
        </div>

        {/* Right: Active TestRun Execution Runner */}
        {selectedRun ? (
          <div style={{ display: 'grid', gap: '0.9rem' }}>
            <div className="card" style={{ display: 'grid', gap: '0.75rem' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '0.5rem' }}>
                <div>
                  <h3 style={{ margin: 0, fontSize: '1.02rem' }}>
                    #{selectedRun.id} {selectedRun.summary}
                  </h3>
                  <div style={{ fontSize: '0.78rem', color: 'var(--muted)', marginTop: 2 }}>
                    {t.testruns.startedLabel} {new Date(selectedRun.started_at).toLocaleString()}
                    {selectedRun.stopped_at ? ` · ${t.testruns.stoppedLabel} ${new Date(selectedRun.stopped_at).toLocaleString()}` : ''}
                  </div>
                </div>
                <div style={{ display: 'flex', gap: '0.35rem', flexWrap: 'wrap' }}>
                  <button type="button" className="btn btn-sm" onClick={() => void api.downloadRunExport(selectedRun.id, 'csv')}>
                    <Download size={13} /> CSV
                  </button>
                  <button type="button" className="btn btn-sm" onClick={() => void api.downloadRunExport(selectedRun.id, 'html')}>
                    <Download size={13} /> HTML
                  </button>
                  {!selectedRun.stopped_at && (
                    <button
                      type="button"
                      className="btn btn-sm"
                      onClick={async () => {
                        await api.stopTestRun(selectedRun.id)
                        notify(t.testruns.stoppedToast(selectedRun.id))
                        await loadRuns()
                      }}
                    >
                      <Square size={13} /> {t.testruns.stopRun}
                    </button>
                  )}
                  <button
                    type="button"
                    className="btn btn-danger btn-sm"
                    onClick={async () => {
                      if (!window.confirm(t.testruns.confirmDeleteRun(selectedRun.id))) return
                      await api.deleteTestRun(selectedRun.id)
                      setSelectedRunId(null)
                      navigate('/runs', { replace: true })
                      notify(t.testruns.deletedToast(selectedRun.id))
                      await loadRuns()
                    }}
                  >
                    <Trash2 size={13} />
                  </button>
                </div>
              </div>

              <div className="progress-bar">
                <div style={{ width: `${((counts.passed ?? 0) / totalExecs) * 100}%`, background: '#16a34a' }} />
                <div style={{ width: `${((counts.failed ?? 0) / totalExecs) * 100}%`, background: '#dc2626' }} />
                <div style={{ width: `${((counts.blocked ?? 0) / totalExecs) * 100}%`, background: '#d97706' }} />
                <div style={{ width: `${((counts.skipped ?? 0) / totalExecs) * 100}%`, background: '#64748b' }} />
              </div>

              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '0.5rem' }}>
                <div style={{ display: 'flex', gap: '0.35rem', flexWrap: 'wrap' }}>
                  {['', 'passed', 'failed', 'blocked', 'skipped', 'idle'].map((st) => (
                    <button
                      key={st || 'all'}
                      type="button"
                      className={`btn btn-sm ${execFilter === st ? 'btn-primary' : ''}`}
                      onClick={() => setExecFilter(st)}
                    >
                      {st ? `${st} (${counts[st] ?? 0})` : `${t.common.all} (${executions.length})`}
                    </button>
                  ))}
                </div>
                <span style={{ fontSize: '0.75rem', color: 'var(--muted)', display: 'inline-flex', alignItems: 'center', gap: 4 }}>
                  <Keyboard size={13} /> {t.testruns.shortcutsHint}
                </span>
              </div>

              <button
                type="button"
                className="btn btn-sm"
                title={t.testruns.aiSummaryHint}
                onClick={async () => {
                  try {
                    const res = await api.aiSummarizeRun(selectedRun.id)
                    notify(res.summary.length > 220 ? res.summary.slice(0, 220) + '…' : res.summary)
                  } catch (err) {
                    notify(err instanceof Error ? err.message : t.testruns.aiSummaryFailed, 'error')
                  }
                }}
              >
                <Bot size={13} /> {t.testruns.aiSummary}
              </button>
              <EntityTagsBar entity="testruns" entityId={selectedRun.id} />
            </div>

            {filteredExecs.length === 0 ? (
              <div className="card" style={{ color: 'var(--muted)' }}>{t.testruns.emptyExecutions}</div>
            ) : (
              <>
              {selectedExecIds.length > 0 && (
                <div className="card" style={{ display: 'flex', gap: '0.5rem', alignItems: 'center', padding: '0.45rem 0.6rem', marginBottom: '0.5rem' }}>
                  <strong style={{ fontSize: '0.82rem' }}>{selectedExecIds.length} {t.common.selected}</strong>
                  <select className="select" value={bulkStatus} onChange={(e) => setBulkStatus(e.target.value)} style={{ fontSize: '0.8rem' }}>
                    {['passed', 'failed', 'blocked', 'skipped', 'running', 'idle'].map((s) => (
                      <option key={s} value={s}>{s}</option>
                    ))}
                  </select>
                  <button
                    type="button"
                    className="btn btn-sm btn-primary"
                    onClick={async () => {
                      try {
                        const res = await api.bulkUpdateExecutions({ ids: selectedExecIds, status: bulkStatus })
                        notify(t.testruns.bulkUpdatedToast(res.updated))
                        setSelectedExecIds([])
                        await loadRuns()
                      } catch {
                        notify(t.testruns.bulkFailed, 'error')
                      }
                    }}
                  >
                    {t.testruns.applyStatus}
                  </button>
                  <button type="button" className="btn btn-sm" onClick={() => setSelectedExecIds([])}>{t.common.clear}</button>
                </div>
              )}
              <div className="runner-split">
                <div className="card" style={{ padding: '0.6rem', display: 'grid', gap: '0.35rem', maxHeight: 560, overflowY: 'auto' }}>
                  {filteredExecs.map((ex) => (
                    <div
                      key={ex.id}
                      style={{ display: 'flex', alignItems: 'center', gap: '0.35rem', borderRadius: 8, border: activeExec?.id === ex.id ? '1px solid var(--primary)' : '1px solid transparent', background: activeExec?.id === ex.id ? 'var(--primary-soft)' : 'transparent' }}
                    >
                      <input
                        type="checkbox"
                        checked={selectedExecIds.includes(ex.id)}
                        onChange={() => setSelectedExecIds((prev) => prev.includes(ex.id) ? prev.filter((x) => x !== ex.id) : [...prev, ex.id])}
                        aria-label={t.testruns.selectExecAria(ex.id)}
                      />
                    <button
                      type="button"
                      onClick={() => setActiveExecId(ex.id)}
                      style={{
                        textAlign: 'left',
                        padding: '0.55rem 0.65rem',
                        borderRadius: 8,
                        border: activeExec?.id === ex.id ? '1px solid var(--primary)' : '1px solid transparent',
                        background: activeExec?.id === ex.id ? 'var(--primary-soft)' : 'transparent',
                        cursor: 'pointer',
                        display: 'flex',
                        justifyContent: 'space-between',
                        alignItems: 'center',
                        gap: '0.4rem',
                        color: 'var(--text)',
                        fontFamily: 'inherit',
                      }}
                    >
                      <span style={{ fontSize: '0.83rem', fontWeight: 500, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        <span style={{ fontFamily: 'var(--font-mono)', color: 'var(--muted)', marginRight: 5 }}>#{ex.test_case_id}</span>
                        {ex.summary}
                      </span>
                      <span style={{ display: 'inline-flex', gap: '0.3rem', alignItems: 'center' }}>
                        {ex.dataset_name && <span className="badge" title={t.testruns.datasetLabel}>DS: {ex.dataset_name}</span>}
                        <span className={`badge pill-${ex.status}`}>{ex.status}</span>
                      </span>
                    </button>
                    </div>
                  ))}
                </div>

                {activeExec && (
                  <div className="card" style={{ display: 'grid', gap: '0.9rem' }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: '0.5rem' }}>
                      <div>
                        <div style={{ fontSize: '0.76rem', color: 'var(--muted)', fontFamily: 'var(--font-mono)' }}>
                          {t.testruns.execHeader(activeExec.id, activeExec.test_case_id)}
                        </div>
                        <h4 style={{ margin: '0.2rem 0 0', fontSize: '1rem' }}>{activeExec.summary}</h4>
                        {activeExec.dataset_name && (
                          <div style={{ fontSize: '0.75rem', marginTop: '0.2rem' }}>
                            <span className="badge">{t.testruns.datasetLabel}: {activeExec.dataset_name}</span>
                          </div>
                        )}
                      </div>
                      <div style={{ display: 'flex', gap: '0.35rem', alignItems: 'center' }}>
                        {activeExec.latest_case_version && activeExec.case_text_version && activeExec.latest_case_version > activeExec.case_text_version && (
                          <button
                            type="button"
                            className="btn btn-sm"
                            style={{ borderColor: 'var(--warning)', color: 'var(--warning-text)' }}
                            onClick={async () => {
                              const synced = await api.syncExecutionCase(activeExec.id)
                              setExecutions((prev) => prev.map((x) => (x.id === synced.id ? synced : x)))
                              notify(t.testruns.snapshotSynced(synced.case_text_version))
                            }}
                          >
                            <RefreshCw size={12} /> {t.testruns.syncVersion(activeExec.case_text_version, activeExec.latest_case_version)}
                          </button>
                        )}
                        <button type="button" className="btn btn-ghost btn-sm" onClick={() => navigate(`/cases/${activeExec.test_case_id}`)}>
                          {t.testruns.openCase} <ExternalLink size={13} />
                        </button>
                      </div>
                    </div>

                    <div style={{ display: 'flex', gap: '0.45rem', flexWrap: 'wrap' }}>
                      <button type="button" className={`btn btn-sm ${activeExec.status === 'passed' ? 'btn-primary' : ''}`} onClick={() => void handleStatus(activeExec.id, 'passed')}>
                        <CheckCircle2 size={14} color="#16a34a" /> {t.dashboard.passed} <span className="kbd">P</span>
                      </button>
                      <button type="button" className={`btn btn-sm ${activeExec.status === 'failed' ? 'btn-primary' : ''}`} onClick={() => void handleStatus(activeExec.id, 'failed')}>
                        <XCircle size={14} color="#dc2626" /> {t.dashboard.failed} <span className="kbd">F</span>
                      </button>
                      <button type="button" className={`btn btn-sm ${activeExec.status === 'blocked' ? 'btn-primary' : ''}`} onClick={() => void handleStatus(activeExec.id, 'blocked')}>
                        <Ban size={14} color="#d97706" /> {t.dashboard.blocked} <span className="kbd">B</span>
                      </button>
                      <button type="button" className={`btn btn-sm ${activeExec.status === 'skipped' ? 'btn-primary' : ''}`} onClick={() => void handleStatus(activeExec.id, 'skipped')}>
                        <SkipForward size={14} color="#64748b" /> {t.dashboard.skipped} <span className="kbd">S</span>
                      </button>
                    </div>

                    {activeExec.comment && (
                      <div style={{ padding: '0.55rem 0.75rem', borderRadius: 8, background: 'var(--bg-subtle)', fontSize: '0.82rem', fontFamily: 'var(--font-mono)' }}>
                        {activeExec.comment}
                      </div>
                    )}

                    <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
                      <button
                        type="button"
                        className="btn btn-sm"
                        title={t.testruns.visualCompareHint}
                        onClick={async () => {
                          try {
                            const shots = await api.listExecutionScreenshots(activeExec.id)
                            if ((shots.items ?? []).length === 0) {
                              notify(t.testruns.noScreenshots, 'error')
                              return
                            }
                            const res = await api.visualCompare(activeExec.id, shots.items[0].id)
                            notify(t.testruns.visualResultToast(res.verdict, res.diff_percent.toFixed(2)))
                            const list = await api.listVisualComparisons(activeExec.id)
                            setVisualResults((prev) => ({ ...prev, [activeExec.id]: list.items ?? [] }))
                          } catch (err) {
                            notify(err instanceof Error ? err.message : t.testruns.visualFailed, 'error')
                          }
                        }}
                      >
                        {t.testruns.visualCompare}
                      </button>
                      <button
                        type="button"
                        className="btn btn-sm"
                        title={t.testruns.expandDatasetsHint}
                        onClick={async () => {
                          try {
                            const res = await api.expandDatasets(selectedRun.id, activeExec.test_case_id)
                            notify(t.testruns.expandToast(res.created))
                            await loadRuns()
                          } catch {
                            notify(t.testruns.expandFailed, 'error')
                          }
                        }}
                      >
                        {t.testruns.expandDatasets}
                      </button>
                    </div>
                    <div style={{ borderTop: '1px solid var(--border)', paddingTop: '0.75rem' }}>
                      <ExecutionLinksPanel executionId={activeExec.id} />
                    </div>
                    <div style={{ borderTop: '1px solid var(--border)', paddingTop: '0.75rem' }}>
                      <ExecutionBugsPanel executionId={activeExec.id} trackers={trackers} />
                    </div>
                    {visualResults[activeExec.id] && visualResults[activeExec.id].length > 0 && (
                      <div style={{ borderTop: '1px solid var(--border)', paddingTop: '0.75rem', display: 'grid', gap: '0.3rem' }}>
                        <strong style={{ fontSize: '0.85rem' }}>{t.testruns.visualComparisons}</strong>
                        {visualResults[activeExec.id].map((c) => {
                          const av = aiVerdicts[c.id]
                          let stored: { verdict: string; confidence: number; explanation: string } | null = null
                          if (!av && c.ai_verdict) { try { stored = JSON.parse(c.ai_verdict) } catch { stored = null } }
                          const verdict = av === 'busy' ? null : (av ?? stored)
                          return (
                          <div key={c.id} style={{ fontSize: '0.82rem', display: 'flex', gap: '0.5rem', alignItems: 'center', flexWrap: 'wrap' }}>
                            <span className={`badge badge-${c.verdict === 'match' ? 'passed' : c.verdict === 'mismatch' ? 'failed' : 'idle'}`}>{c.verdict}</span>
                            <span>{t.testruns.diffLabel(c.diff_percent.toFixed(2))}</span>
                            <span style={{ color: 'var(--muted)' }}>{new Date(c.created_at).toLocaleTimeString()}</span>
                            {c.verdict === 'mismatch' && (
                              <button
                                type="button"
                                className="btn btn-sm"
                                disabled={av === 'busy'}
                                title={t.testruns.aiVerdictHint}
                                onClick={async () => {
                                  setAiVerdicts((prev) => ({ ...prev, [c.id]: 'busy' }))
                                  try {
                                    const res = await api.aiVisualVerdict(c.id)
                                    setAiVerdicts((prev) => ({ ...prev, [c.id]: { verdict: res.verdict, confidence: res.confidence, explanation: res.explanation } }))
                                    const list = await api.listVisualComparisons(activeExec.id)
                                    setVisualResults((prev) => ({ ...prev, [activeExec.id]: list.items ?? [] }))
                                  } catch (err) {
                                    setAiVerdicts((prev) => { const n = { ...prev }; delete n[c.id]; return n })
                                    notify(err instanceof Error ? err.message : t.testruns.aiVerdictFailed, 'error')
                                  }
                                }}
                              >
                                {av === 'busy' ? t.testruns.aiVerdictBusy : t.testruns.aiVerdictBtn}
                              </button>
                            )}
                            {av === 'busy' && <span style={{ color: 'var(--muted)' }}>{t.testruns.aiAnalyzing}</span>}
                            {verdict && (
                              <span style={{ display: 'inline-flex', gap: '0.4rem', alignItems: 'center' }}>
                                <span className={`badge badge-${verdict.verdict === 'real-defect' ? 'failed' : verdict.verdict === 'acceptable-change' ? 'passed' : 'idle'}`}>AI: {verdict.verdict}</span>
                                <span style={{ color: 'var(--muted)' }}>{verdict.confidence}% — {verdict.explanation}</span>
                              </span>
                            )}
                          </div>
                          )
                        })}
                      </div>
                    )}
                    <div style={{ borderTop: '1px solid var(--border)', paddingTop: '0.75rem' }}>
                      <ScreenshotsGallery executionId={activeExec.id} />
                    </div>
                    <div style={{ borderTop: '1px solid var(--border)', paddingTop: '0.75rem' }}>
                      <CustomFieldsPanel entityType="test_execution" entityId={activeExec.id} />
                    </div>
                    <div style={{ borderTop: '1px solid var(--border)', paddingTop: '0.75rem' }}>
                      <AttachmentsPanel entityType="test_execution" entityId={activeExec.id} />
                    </div>
                    <div style={{ borderTop: '1px solid var(--border)', paddingTop: '0.75rem' }}>
                      <CommentsPanel entityType="test_execution" entityId={activeExec.id} />
                    </div>
                  </div>
                )}
              </div>
              </>
            )}
          </div>
        ) : (
          <div className="card" style={{ color: 'var(--muted)' }}>{t.testruns.emptyRuns}</div>
        )}
      </div>
    </section>
  )
}
