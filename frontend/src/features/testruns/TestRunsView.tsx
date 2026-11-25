import { useEffect, useState } from 'react'
import { api, BugItem, Build, IssueTrackerItem, TestExecution, TestPlan, TestRun } from '../../api/client'
import { useI18n } from '../../i18n'
import { AttachmentsPanel } from '../attachments/AttachmentsPanel'
import { CommentsPanel } from '../comments/CommentsPanel'
import { EntityTagsBar } from '../management/EntityTagsBar'

const STATUSES: TestExecution['status'][] = ['passed', 'failed', 'blocked', 'skipped']

const STATUS_COLORS: Record<string, string> = {
  passed: '#16a34a',
  failed: '#dc2626',
  blocked: '#d97706',
  skipped: '#6b7280',
  idle: '#9ca3af',
  running: '#2563eb',
}

function ExecutionBugsPanel({ executionId, trackers }: { executionId: number; trackers: IssueTrackerItem[] }) {
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
      // Ignore when unauthenticated
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
      await load()
    } catch (e) {
      setErr(e instanceof Error ? e.message : 'Failed to link bug')
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
      await load()
    } catch (e) {
      setErr(e instanceof Error ? e.message : 'Failed to report bug')
    }
  }

  return (
    <div style={{ marginTop: '0.4rem', fontSize: '0.82rem' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: '0.45rem', flexWrap: 'wrap' }}>
        <span style={{ color: 'var(--muted)' }}>Bugs:</span>
        {bugs.map((b) => (
          <a key={b.id} href={b.url} target="_blank" rel="noreferrer" className="badge" style={{ textDecoration: 'none' }}>
            {b.external_key}{b.title ? `: ${b.title}` : ''}
          </a>
        ))}
        <button type="button" className="btn btn-sm" onClick={() => setMode((m) => (m === 'link' ? 'none' : 'link'))}>
          {mode === 'link' ? 'Cancel' : '+ Link bug'}
        </button>
        {trackers.length > 0 && (
          <button type="button" className="btn btn-sm" onClick={() => setMode((m) => (m === 'report' ? 'none' : 'report'))}>
            {mode === 'report' ? 'Cancel' : '+ Report to tracker'}
          </button>
        )}
      </div>
      {err && <div style={{ color: 'var(--danger)' }}>{err}</div>}
      {mode === 'link' && (
        <form onSubmit={handleLink} style={{ display: 'flex', gap: '0.35rem', marginTop: '0.35rem', flexWrap: 'wrap' }}>
          <input className="input" placeholder="Key (e.g. PROJ-12)" value={key} onChange={(e) => setKey(e.target.value)} required style={{ width: 120 }} />
          <input className="input" placeholder="https://..." value={url} onChange={(e) => setUrl(e.target.value)} required style={{ flex: 1, minWidth: 180 }} />
          <input className="input" placeholder="Title (optional)" value={title} onChange={(e) => setTitle(e.target.value)} style={{ width: 160 }} />
          <button type="submit" className="btn btn-primary btn-sm">Link</button>
        </form>
      )}
      {mode === 'report' && (
        <form onSubmit={handleReport} style={{ display: 'grid', gap: '0.35rem', marginTop: '0.35rem' }}>
          <div style={{ display: 'flex', gap: '0.35rem', flexWrap: 'wrap' }}>
            <select className="select" value={trackerId} onChange={(e) => setTrackerId(Number(e.target.value))}>
              {trackers.map((tr) => (
                <option key={tr.id} value={tr.id}>{tr.name} ({tr.provider})</option>
              ))}
            </select>
            <input className="input" placeholder="Title" value={title} onChange={(e) => setTitle(e.target.value)} required style={{ flex: 1, minWidth: 160 }} />
            <input className="input" placeholder="Project key (Jira)" value={projectKey} onChange={(e) => setProjectKey(e.target.value)} style={{ width: 130 }} />
            <input className="input" type="password" placeholder="API token (optional)" value={apiToken} onChange={(e) => setApiToken(e.target.value)} style={{ width: 150 }} />
          </div>
          <div style={{ display: 'flex', gap: '0.35rem' }}>
            <input className="input" placeholder="Description (optional)" value={reportBody} onChange={(e) => setReportBody(e.target.value)} style={{ flex: 1 }} />
            <button type="submit" className="btn btn-primary btn-sm">Create issue</button>
          </div>
        </form>
      )}
    </div>
  )
}

export function TestRunsView() {
  const { t } = useI18n()
  const [plans, setPlans] = useState<TestPlan[]>([])
  const [builds, setBuilds] = useState<Build[]>([])
  const [runs, setRuns] = useState<TestRun[]>([])
  const [trackers, setTrackers] = useState<IssueTrackerItem[]>([])
  const [selectedRunId, setSelectedRunId] = useState<number | null>(null)
  const [executions, setExecutions] = useState<TestExecution[]>([])
  const [planId, setPlanId] = useState<number>(0)
  const [buildId, setBuildId] = useState<number>(0)
  const [summary, setSummary] = useState('')
  const [showJUnit, setShowJUnit] = useState(false)
  const [junitXML, setJunitXML] = useState('')
  const [notice, setNotice] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  const loadRuns = async () => {
    try {
      const [tpRes, trRes, itRes] = await Promise.all([
        api.listTestPlans(),
        api.listTestRuns(),
        api.listIssueTrackers().catch(() => ({ items: [] as IssueTrackerItem[] })),
      ])
      setPlans(tpRes.items)
      setTrackers(itRes.items)
      if (tpRes.items.length > 0 && !planId) setPlanId(tpRes.items[0].id)
      setRuns(trRes.items)
      if (trRes.items.length > 0 && selectedRunId === null) {
        setSelectedRunId(trRes.items[0].id)
      }
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : t.errors.loadTestRuns)
    }
  }

  useEffect(() => {
    void loadRuns()
  }, [])

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
      return
    }
    void api
      .listExecutions(selectedRunId)
      .then((res) => setExecutions(res.items))
      .catch((e) => setError(e instanceof Error ? e.message : t.errors.loadTestRuns))
  }, [selectedRunId])

  const handleCreateRun = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!summary.trim() || !planId || !buildId) return
    try {
      const created = await api.createTestRun({ test_plan_id: planId, build_id: buildId, summary: summary.trim() })
      setSummary('')
      setSelectedRunId(created.id)
      setNotice(null)
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
        summary: summary.trim() || `JUnit Import (${new Date().toLocaleDateString()})`,
        xml: junitXML,
      })
      setJunitXML('')
      setShowJUnit(false)
      setSelectedRunId(res.test_run.id)
      setNotice(`Imported ${res.results.length} JUnit test case result(s) into Run #${res.test_run.id}.`)
      setError(null)
      await loadRuns()
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errors.createTestRun)
    }
  }

  const handleStatus = async (execId: number, status: TestExecution['status']) => {
    try {
      await api.updateExecution(execId, { status })
      setExecutions((prev) => prev.map((ex) => (ex.id === execId ? { ...ex, status } : ex)))
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errors.updateStatus)
    }
  }

  const handleExport = async (format: 'csv' | 'html') => {
    if (selectedRunId === null) return
    try {
      await api.downloadRunExport(selectedRunId, format)
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errors.loadTestRuns)
    }
  }

  return (
    <section>
      <h2>{t.testruns.title}</h2>
      {error && <p className="alert-error">{error}</p>}
      {notice && <p className="alert-ok">{notice}</p>}

      <form onSubmit={handleCreateRun} style={{ display: 'flex', gap: '0.5rem', marginBottom: '0.75rem', flexWrap: 'wrap' }}>
        <select className="select" value={planId} onChange={(e) => setPlanId(Number(e.target.value))}>
          {plans.map((tp) => (
            <option key={tp.id} value={tp.id}>{tp.name}</option>
          ))}
        </select>
        <select className="select" value={buildId} onChange={(e) => setBuildId(Number(e.target.value))}>
          {builds.length === 0 && <option value={0}>No builds</option>}
          {builds.map((b) => (
            <option key={b.id} value={b.id}>{b.name} (#{b.id})</option>
          ))}
        </select>
        <input className="input" placeholder={t.testruns.summaryPlaceholder} value={summary} onChange={(e) => setSummary(e.target.value)} style={{ flex: 1, minWidth: 200 }} />
        <button type="submit" disabled={!planId || !buildId} className="btn btn-primary">{t.testruns.create}</button>
        <button type="button" className="btn" onClick={() => setShowJUnit((v) => !v)}>
          {showJUnit ? 'Hide JUnit' : 'Import JUnit XML'}
        </button>
      </form>

      {showJUnit && (
        <form onSubmit={handleImportJUnit} className="card" style={{ marginBottom: '1rem', display: 'grid', gap: '0.5rem' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <strong>Import JUnit XML</strong>
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
            <button type="submit" className="btn btn-primary btn-sm" disabled={!planId || !buildId || !junitXML.trim()}>
              Run JUnit Import
            </button>
          </div>
        </form>
      )}

      <div className="runs-layout">
        <ul style={{ listStyle: 'none', padding: 0, margin: 0, display: 'grid', gap: '0.4rem', alignContent: 'start' }}>
          {runs.map((r) => (
            <li
              key={r.id}
              className="card"
              onClick={() => setSelectedRunId(r.id)}
              style={{
                padding: '0.65rem',
                border: selectedRunId === r.id ? '2px solid var(--primary)' : undefined,
                cursor: 'pointer',
              }}
            >
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <strong>{r.summary}</strong>
                <button
                  type="button"
                  className="btn btn-danger btn-sm"
                  onClick={async (e) => {
                    e.stopPropagation()
                    if (!window.confirm(`Delete TestRun #${r.id}?`)) return
                    try {
                      await api.deleteTestRun(r.id)
                      if (selectedRunId === r.id) setSelectedRunId(null)
                      await loadRuns()
                    } catch (err) {
                      setError(err instanceof Error ? err.message : t.errors.loadTestRuns)
                    }
                  }}
                >
                  ×
                </button>
              </div>
              <div style={{ fontSize: '0.8rem', color: 'var(--muted)' }}>Plan #{r.test_plan_id} · Build #{r.build_id}</div>
            </li>
          ))}
          {runs.length === 0 && <li style={{ color: 'var(--muted)' }}>{t.testruns.emptyRuns}</li>}
        </ul>

        <div className="card">
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '0.5rem' }}>
            <h3 style={{ margin: 0 }}>{t.testruns.executions}</h3>
            {selectedRunId !== null && (
              <div style={{ display: 'flex', gap: '0.4rem' }}>
                <button
                  type="button"
                  className="btn btn-sm"
                  onClick={async () => {
                    try {
                      await api.stopTestRun(selectedRunId)
                      setNotice(`TestRun #${selectedRunId} stopped.`)
                      setError(null)
                    } catch (err) {
                      setError(err instanceof Error ? err.message : t.errors.updateStatus)
                    }
                  }}
                >
                  Stop
                </button>
                <button type="button" className="btn btn-sm" onClick={() => void handleExport('csv')}>CSV</button>
                <button type="button" className="btn btn-sm" onClick={() => void handleExport('html')}>HTML</button>
              </div>
            )}
          </div>
          {selectedRunId !== null && <EntityTagsBar entity="testruns" entityId={selectedRunId} />}
          {executions.length === 0 && <p style={{ color: 'var(--muted)' }}>{t.testruns.emptyExecutions}</p>}
          <ul style={{ listStyle: 'none', padding: 0, display: 'grid', gap: '0.6rem' }}>
            {executions.map((ex) => (
              <li key={ex.id} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: '0.75rem', padding: '0.6rem', border: '1px solid var(--border)', borderRadius: 6 }}>
                <div style={{ flex: 1 }}>
                  <strong>{ex.summary}</strong>
                  <span style={{ marginLeft: '0.5rem', fontSize: '0.8rem', color: STATUS_COLORS[ex.status] ?? 'var(--text)', fontWeight: 600 }}>
                    {ex.status.toUpperCase()}
                  </span>
                  <ExecutionBugsPanel executionId={ex.id} trackers={trackers} />
                  <AttachmentsPanel entityType="test_execution" entityId={ex.id} />
                  <CommentsPanel entityType="test_execution" entityId={ex.id} />
                </div>
                <div style={{ display: 'flex', gap: '0.25rem', flexShrink: 0 }}>
                  {STATUSES.map((st) => (
                    <button
                      key={st}
                      type="button"
                      onClick={() => void handleStatus(ex.id, st)}
                      style={{
                        padding: '0.25rem 0.5rem',
                        borderRadius: 4,
                        border: '1px solid var(--border-strong)',
                        background: ex.status === st ? STATUS_COLORS[st] : '#fff',
                        color: ex.status === st ? '#fff' : 'var(--text)',
                        cursor: 'pointer',
                        fontSize: '0.8rem',
                      }}
                    >
                      {st}
                    </button>
                  ))}
                </div>
              </li>
            ))}
          </ul>
        </div>
      </div>
    </section>
  )
}
