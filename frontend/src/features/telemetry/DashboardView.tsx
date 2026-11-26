import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  Activity,
  AlertTriangle,
  FileCheck2,
  FolderKanban,
  GanttChartSquare,
  ListChecks,
  Package,
  PieChart as PieIcon,
  PlayCircle,
  RotateCcw,
  Users,
} from 'lucide-react'
import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  Legend,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import {
  api,
  CaseHealth,
  DashboardSnapshot,
  ExecutionDashboard,
  MatrixRow,
  TelemetryBreakdown,
  TelemetrySummary,
  TrendPoint,
} from '../../api/client'
import { useWorkspace } from '../../context/WorkspaceContext'
import { useI18n } from '../../i18n'

const PIE_COLORS = ['#6366f1', '#94a3b8']

const monoStyle: React.CSSProperties = {
  fontFamily: 'var(--font-mono)',
  color: 'var(--muted)',
  marginRight: 6,
}

export function DashboardView() {
  const { t } = useI18n()
  const { activeProductId } = useWorkspace()
  const navigate = useNavigate()

  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const [summary, setSummary] = useState<TelemetrySummary | null>(null)
  const [trends, setTrends] = useState<TrendPoint[]>([])
  const [matrix, setMatrix] = useState<MatrixRow[]>([])
  const [health, setHealth] = useState<CaseHealth[]>([])
  const [breakdown, setBreakdown] = useState<TelemetryBreakdown | null>(null)
  const [execDash, setExecDash] = useState<ExecutionDashboard | null>(null)
  const [dash, setDash] = useState<DashboardSnapshot | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const params = {
      product_id: activeProductId ?? undefined,
      from: from || undefined,
      to: to || undefined,
    }
    void Promise.all([
      api.getTelemetrySummary(params),
      api.getExecutionTrends(params),
      api.getStatusMatrix(params),
      api.getTestCaseHealth(params),
      api.getTelemetryBreakdown(params),
      api.getExecutionDashboard(params),
    ])
      .then(([s, tr, mx, hl, bd, ed]) => {
        setSummary(s)
        setTrends(tr.items)
        setMatrix(mx.items)
        setHealth(hl.items)
        setBreakdown(bd)
        setExecDash(ed)
        setError(null)
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : t.errors.authFailed)
      })
  }, [activeProductId, from, to, t.errors.authFailed])

  useEffect(() => {
    void api.getDashboard().then(setDash).catch(() => setDash(null))
  }, [])

  const trendSeries = useMemo(() => {
    const map = new Map<string, { date: string; passed: number; failed: number; blocked: number; skipped: number }>()
    for (const pt of trends) {
      const row = map.get(pt.date) ?? { date: pt.date, passed: 0, failed: 0, blocked: 0, skipped: 0 }
      if (pt.status === 'passed') row.passed += pt.count
      else if (pt.status === 'failed') row.failed += pt.count
      else if (pt.status === 'blocked') row.blocked += pt.count
      else if (pt.status === 'skipped') row.skipped += pt.count
      map.set(pt.date, row)
    }
    return Array.from(map.values()).slice(-14)
  }, [trends])

  const autoPieData = useMemo(() => {
    if (!breakdown) return []
    return [
      { name: 'Automated', value: breakdown.automated },
      { name: 'Manual', value: breakdown.manual },
    ]
  }, [breakdown])

  if (error) {
    return <p className="alert-error">{error}</p>
  }

  if (!summary) {
    return <p style={{ color: 'var(--muted)' }}>{t.dashboard.loading}</p>
  }

  const total = summary.executions.total || 1
  const pct = (n: number) => `${Math.round((n / total) * 100)}%`
  const passRate = summary.executions.total > 0 ? Math.round((summary.executions.passed / summary.executions.total) * 100) : 0

  return (
    <section style={{ display: 'grid', gap: '1.15rem' }}>
      {/* Personal widgets: my runs + recent activity */}
      {dash && (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(320px, 1fr))', gap: '1.15rem' }}>
          <div className="card">
            <div className="card-header">
              <h3 className="card-title">
                <PlayCircle size={16} />
                {t.dashboard.myRuns}
              </h3>
            </div>
            {dash.my_runs.length === 0 ? (
              <p style={{ color: 'var(--muted)', fontSize: '0.85rem' }}>{t.dashboard.noRuns}</p>
            ) : (
              <ul style={{ listStyle: 'none', margin: 0, padding: 0, display: 'grid', gap: '0.4rem' }}>
                {dash.my_runs.map((r) => (
                  <li key={r.id}>
                    <a
                      href={`/runs/${r.id}`}
                      onClick={(e) => {
                        e.preventDefault()
                        navigate(`/runs/${r.id}`)
                      }}
                      style={{ display: 'flex', justifyContent: 'space-between', gap: '0.5rem', fontSize: '0.87rem' }}
                    >
                      <span>
                        #{r.id} {r.summary}
                      </span>
                      <span className={`badge ${r.stopped_at ? 'pill-idle' : 'pill-running'}`}>
                        {r.stopped_at ? t.dashboard.stateStopped : t.dashboard.stateRunning}
                      </span>
                    </a>
                  </li>
                ))}
              </ul>
            )}
          </div>
          <div className="card">
            <div className="card-header">
              <h3 className="card-title">
                <Activity size={16} />
                {t.dashboard.recentActivity}
              </h3>
            </div>
            {dash.recent_activity.length === 0 ? (
              <p style={{ color: 'var(--muted)', fontSize: '0.85rem' }}>{t.dashboard.noActivity}</p>
            ) : (
              <ul style={{ listStyle: 'none', margin: 0, padding: 0, display: 'grid', gap: '0.4rem', fontSize: '0.85rem' }}>
                {dash.recent_activity.map((a) => (
                  <li key={a.id} style={{ display: 'flex', justifyContent: 'space-between', gap: '0.5rem' }}>
                    <span>
                      <strong>{a.actor_name}</strong> {a.action} {a.entity_type} #{a.entity_id}
                    </span>
                    <span style={monoStyle}>{new Date(a.created_at).toLocaleString()}</span>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </div>
      )}

      {/* Filter bar */}
      <div className="filter-bar" style={{ marginBottom: 0 }}>
        <span style={{ fontSize: '0.8rem', fontWeight: 600, color: 'var(--muted)' }}>{t.dashboard.from}:</span>
        <input className="input" type="date" value={from} onChange={(e) => setFrom(e.target.value)} />
        <span style={{ fontSize: '0.8rem', fontWeight: 600, color: 'var(--muted)' }}>{t.dashboard.to}:</span>
        <input className="input" type="date" value={to} onChange={(e) => setTo(e.target.value)} />
        {(from || to) && (
          <button
            type="button"
            className="btn btn-ghost btn-sm"
            onClick={() => {
              setFrom('')
              setTo('')
            }}
          >
            <RotateCcw size={14} />
            {t.dashboard.resetFilters}
          </button>
        )}
      </div>

      {/* KPI cards */}
      <div className="stat-grid">
        {[
          { label: t.dashboard.products, value: summary.products, icon: Package, to: '/products' },
          { label: t.dashboard.testCases, value: summary.test_cases, icon: FileCheck2, to: '/cases' },
          { label: t.dashboard.testPlans, value: summary.test_plans, icon: FolderKanban, to: '/plans' },
          { label: t.dashboard.testRuns, value: summary.test_runs, icon: PlayCircle, to: '/runs' },
        ].map((card) => {
          const Icon = card.icon
          return (
            <div key={card.label} className="stat-card" style={{ cursor: 'pointer' }} onClick={() => navigate(card.to)}>
              <div>
                <div className="stat-label">{card.label}</div>
                <div className="stat-value">{card.value}</div>
              </div>
              <div className="stat-icon">
                <Icon size={19} />
              </div>
            </div>
          )
        })}
      </div>

      {/* Pass / Fail progress */}
      <div className="card">
        <div className="card-header">
          <h3 className="card-title">
            <Activity size={16} />
            {t.dashboard.passFail} ({summary.executions.total})
          </h3>
          <span className={`badge ${passRate >= 80 ? 'pill-passed' : passRate >= 50 ? 'pill-blocked' : 'pill-failed'}`}>
            {t.dashboard.passRate}: {passRate}%
          </span>
        </div>
        <div className="progress-bar" style={{ height: 14 }}>
          <div style={{ width: pct(summary.executions.passed), background: '#16a34a' }} title={`${t.dashboard.passed}: ${summary.executions.passed}`} />
          <div style={{ width: pct(summary.executions.failed), background: '#dc2626' }} title={`${t.dashboard.failed}: ${summary.executions.failed}`} />
          <div style={{ width: pct(summary.executions.blocked), background: '#d97706' }} title={`${t.dashboard.blocked}: ${summary.executions.blocked}`} />
          <div style={{ width: pct(summary.executions.skipped), background: '#64748b' }} title={`${t.dashboard.skipped}: ${summary.executions.skipped}`} />
        </div>
        <div style={{ display: 'flex', gap: '1.25rem', marginTop: '0.65rem', fontSize: '0.82rem', flexWrap: 'wrap' }}>
          <span className="badge pill-passed">{t.dashboard.passed}: {summary.executions.passed}</span>
          <span className="badge pill-failed">{t.dashboard.failed}: {summary.executions.failed}</span>
          <span className="badge pill-blocked">{t.dashboard.blocked}: {summary.executions.blocked}</span>
          <span className="badge pill-skipped">{t.dashboard.skipped}: {summary.executions.skipped}</span>
          <span className="badge pill-idle">{t.dashboard.idle}: {summary.executions.idle}</span>
        </div>
      </div>

      {/* Execution board */}
      <div className="card">
        <div className="card-header">
          <h3 className="card-title">
            <GanttChartSquare size={16} />
            {t.dashboard.execBoard}
          </h3>
        </div>
        {!execDash || execDash.runs.length === 0 ? (
          <p style={{ color: 'var(--muted)', margin: 0 }}>{t.dashboard.noData}</p>
        ) : (
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>{t.dashboard.recentRuns}</th>
                  <th>{t.nav.testplans}</th>
                  <th>{t.dashboard.progress}</th>
                  <th>{t.dashboard.totalExecs}</th>
                  <th>{t.dashboard.started}</th>
                  <th>{t.common.status}</th>
                </tr>
              </thead>
              <tbody>
                {execDash.runs.map((r) => (
                  <tr key={r.test_run_id} className="clickable" onClick={() => navigate(`/runs/${r.test_run_id}`)}>
                    <td>
                      <span style={monoStyle}>#{r.test_run_id}</span>
                      {r.summary}
                    </td>
                    <td>{r.plan_name}</td>
                    <td style={{ minWidth: 150 }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                        <div className="progress-bar" style={{ flex: 1, height: 8, minWidth: 80 }}>
                          <div style={{ width: `${Math.round(r.progress * 100)}%`, background: '#6366f1' }} />
                        </div>
                        <span style={{ fontSize: '0.78rem', color: 'var(--muted)', whiteSpace: 'nowrap' }}>
                          {Math.round(r.progress * 100)}%
                        </span>
                      </div>
                    </td>
                    <td style={{ whiteSpace: 'nowrap' }}>
                      <span className="badge pill-passed" title={`${t.dashboard.passed}: ${r.passed}`}>{r.passed}</span>{' '}
                      <span className="badge pill-failed" title={`${t.dashboard.failed}: ${r.failed}`}>{r.failed}</span>{' '}
                      <span className="badge pill-blocked" title={`${t.dashboard.blocked}: ${r.blocked}`}>{r.blocked}</span>{' '}
                      <span className="badge pill-skipped" title={`${t.dashboard.skipped}: ${r.skipped}`}>{r.skipped}</span>{' '}
                      <span className="badge pill-idle" title={`${t.dashboard.idle}: ${r.idle}`}>{r.idle}</span>
                    </td>
                    <td>{new Date(r.started_at).toLocaleDateString()}</td>
                    <td>
                      <span className={`badge ${r.stopped ? 'pill-skipped' : 'pill-blocked'}`}>
                        {r.stopped ? t.dashboard.stateStopped : t.dashboard.stateRunning}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(360px, 1fr))', gap: '1rem' }}>
        <div className="card">
          <div className="card-header">
            <h3 className="card-title">
              <ListChecks size={16} />
              {t.dashboard.recentExecs}
            </h3>
          </div>
          {!execDash || execDash.recent_executions.length === 0 ? (
            <p style={{ color: 'var(--muted)', margin: 0 }}>{t.dashboard.noData}</p>
          ) : (
            <div className="table-wrap">
              <table className="data-table">
                <thead>
                  <tr>
                    <th>{t.dashboard.testCases}</th>
                    <th>{t.dashboard.recentRuns}</th>
                    <th>{t.common.status}</th>
                    <th>{t.dashboard.tester}</th>
                    <th>{t.dashboard.when}</th>
                  </tr>
                </thead>
                <tbody>
                  {execDash.recent_executions.map((e) => (
                    <tr key={e.execution_id} className="clickable" onClick={() => navigate(`/cases/${e.test_case_id}`)}>
                      <td>
                        <span style={monoStyle}>#{e.test_case_id}</span>
                        {e.case_summary}
                      </td>
                      <td>
                        <span style={monoStyle}>#{e.test_run_id}</span>
                        {e.run_summary}
                      </td>
                      <td>
                        <span className={`badge pill-${e.status}`}>{e.status}</span>
                      </td>
                      <td>{e.tester || '—'}</td>
                      <td>{new Date(e.updated_at).toLocaleString()}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>

        <div className="card">
          <div className="card-header">
            <h3 className="card-title">
              <Users size={16} />
              {t.dashboard.testerLoad}
            </h3>
          </div>
          {!execDash || execDash.testers.length === 0 ? (
            <p style={{ color: 'var(--muted)', margin: 0 }}>{t.dashboard.noData}</p>
          ) : (
            <div className="table-wrap">
              <table className="data-table">
                <thead>
                  <tr>
                    <th>{t.dashboard.tester}</th>
                    <th>{t.dashboard.totalExecs}</th>
                    <th>{t.dashboard.passed}</th>
                    <th>{t.dashboard.failed}</th>
                    <th>{t.dashboard.passRate}</th>
                  </tr>
                </thead>
                <tbody>
                  {execDash.testers.map((w) => (
                    <tr key={w.user_id}>
                      <td>{w.username}</td>
                      <td>{w.total}</td>
                      <td>{w.passed}</td>
                      <td>{w.failed}</td>
                      <td>
                        <span className={`badge ${w.pass_rate >= 0.8 ? 'pill-passed' : w.pass_rate >= 0.5 ? 'pill-blocked' : 'pill-failed'}`}>
                          {Math.round(w.pass_rate * 100)}%
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </div>

      {/* Charts row */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(360px, 1fr))', gap: '1rem' }}>
        <div className="card">
          <div className="card-header">
            <h3 className="card-title">
              <Activity size={16} />
              {t.dashboard.trends}
            </h3>
          </div>
          {trendSeries.length === 0 ? (
            <p style={{ color: 'var(--muted)', margin: 0 }}>{t.dashboard.noData}</p>
          ) : (
            <div style={{ width: '100%', height: 240 }}>
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={trendSeries}>
                  <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" />
                  <XAxis dataKey="date" tick={{ fontSize: 11, fill: 'var(--muted)' }} />
                  <YAxis allowDecimals={false} tick={{ fontSize: 11, fill: 'var(--muted)' }} />
                  <Tooltip
                    contentStyle={{
                      background: 'var(--surface)',
                      border: '1px solid var(--border-strong)',
                      borderRadius: 8,
                      fontSize: 12,
                    }}
                  />
                  <Legend wrapperStyle={{ fontSize: 12 }} />
                  <Bar dataKey="passed" stackId="a" fill="#16a34a" name="Passed" radius={[0, 0, 0, 0]} />
                  <Bar dataKey="failed" stackId="a" fill="#dc2626" name="Failed" />
                  <Bar dataKey="blocked" stackId="a" fill="#d97706" name="Blocked" />
                  <Bar dataKey="skipped" stackId="a" fill="#64748b" name="Skipped" radius={[4, 4, 0, 0]} />
                </BarChart>
              </ResponsiveContainer>
            </div>
          )}
        </div>

        <div className="card">
          <div className="card-header">
            <h3 className="card-title">
              <PieIcon size={16} />
              {t.dashboard.breakdown} — {t.dashboard.automation}
            </h3>
          </div>
          {!breakdown || (breakdown.automated === 0 && breakdown.manual === 0) ? (
            <p style={{ color: 'var(--muted)', margin: 0 }}>{t.dashboard.noData}</p>
          ) : (
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.75rem', alignItems: 'center' }}>
              <div style={{ width: '100%', height: 210 }}>
                <ResponsiveContainer width="100%" height="100%">
                  <PieChart>
                    <Pie data={autoPieData} dataKey="value" nameKey="name" innerRadius={48} outerRadius={78} paddingAngle={3}>
                      {autoPieData.map((_, idx) => (
                        <Cell key={idx} fill={PIE_COLORS[idx % PIE_COLORS.length]} />
                      ))}
                    </Pie>
                    <Tooltip
                      contentStyle={{
                        background: 'var(--surface)',
                        border: '1px solid var(--border-strong)',
                        borderRadius: 8,
                        fontSize: 12,
                      }}
                    />
                    <Legend wrapperStyle={{ fontSize: 12 }} />
                  </PieChart>
                </ResponsiveContainer>
              </div>
              <div style={{ display: 'grid', gap: '0.65rem' }}>
                <div>
                  <div style={{ fontSize: '0.74rem', fontWeight: 600, color: 'var(--muted)', marginBottom: '0.3rem', textTransform: 'uppercase' }}>
                    {t.common.priority}
                  </div>
                  <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.35rem' }}>
                    {breakdown.by_priority.map((p) => (
                      <span key={p.name} className={`badge pill-${p.name}`}>
                        {p.name}: {p.count}
                      </span>
                    ))}
                  </div>
                </div>
                <div>
                  <div style={{ fontSize: '0.74rem', fontWeight: 600, color: 'var(--muted)', marginBottom: '0.3rem', textTransform: 'uppercase' }}>
                    {t.dashboard.categories}
                  </div>
                  <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.35rem' }}>
                    {breakdown.by_category.map((c) => (
                      <span key={c.name} className="badge">
                        {c.name}: {c.count}
                      </span>
                    ))}
                  </div>
                </div>
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Health & Matrix row */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(360px, 1fr))', gap: '1rem' }}>
        <div className="card">
          <div className="card-header">
            <h3 className="card-title">
              <AlertTriangle size={16} />
              {t.dashboard.flaky}
            </h3>
          </div>
          {health.length === 0 ? (
            <p style={{ color: 'var(--muted)', margin: 0 }}>{t.dashboard.noData}</p>
          ) : (
            <div className="table-wrap">
              <table className="data-table">
                <thead>
                  <tr>
                    <th>{t.dashboard.testCases}</th>
                    <th>{t.dashboard.failedTotal}</th>
                    <th>{t.dashboard.failRate}</th>
                  </tr>
                </thead>
                <tbody>
                  {health.slice(0, 10).map((h) => (
                    <tr key={h.test_case_id} className="clickable" onClick={() => navigate(`/cases/${h.test_case_id}`)}>
                      <td>
                        <span style={{ fontFamily: 'var(--font-mono)', color: 'var(--muted)', marginRight: 6 }}>#{h.test_case_id}</span>
                        {h.summary}
                      </td>
                      <td>{h.failed} / {h.total}</td>
                      <td>
                        <span className={`badge ${h.fail_rate > 0 ? 'pill-failed' : 'pill-passed'}`}>
                          {Math.round(h.fail_rate * 100)}%
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>

        <div className="card">
          <div className="card-header">
            <h3 className="card-title">{t.dashboard.matrix}</h3>
          </div>
          {matrix.length === 0 ? (
            <p style={{ color: 'var(--muted)', margin: 0 }}>{t.dashboard.noData}</p>
          ) : (
            <div className="table-wrap">
              <table className="data-table">
                <thead>
                  <tr>
                    <th>{t.dashboard.testCases}</th>
                    <th>{t.common.status}</th>
                    <th>{t.dashboard.count}</th>
                  </tr>
                </thead>
                <tbody>
                  {matrix.slice(0, 12).map((m, i) => (
                    <tr key={`${m.test_case_id}-${m.status}-${i}`} className="clickable" onClick={() => navigate(`/cases/${m.test_case_id}`)}>
                      <td>
                        <span style={{ fontFamily: 'var(--font-mono)', color: 'var(--muted)', marginRight: 6 }}>#{m.test_case_id}</span>
                        {m.summary}
                      </td>
                      <td>
                        <span className={`badge pill-${m.status}`}>{m.status}</span>
                      </td>
                      <td>{m.count}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </div>
    </section>
  )
}
