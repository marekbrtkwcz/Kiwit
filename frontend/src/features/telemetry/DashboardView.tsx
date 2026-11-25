import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  Activity,
  AlertTriangle,
  FileCheck2,
  FolderKanban,
  Package,
  PieChart as PieIcon,
  PlayCircle,
  RotateCcw,
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
  MatrixRow,
  TelemetryBreakdown,
  TelemetrySummary,
  TrendPoint,
} from '../../api/client'
import { useWorkspace } from '../../context/WorkspaceContext'
import { useI18n } from '../../i18n'

const PIE_COLORS = ['#6366f1', '#94a3b8']

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
    ])
      .then(([s, tr, mx, hl, bd]) => {
        setSummary(s)
        setTrends(tr.items)
        setMatrix(mx.items)
        setHealth(hl.items)
        setBreakdown(bd)
        setError(null)
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : t.errors.authFailed)
      })
  }, [activeProductId, from, to, t.errors.authFailed])

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
            Pass rate: {passRate}%
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
          <span className="badge pill-idle">Idle: {summary.executions.idle}</span>
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
                    Priority
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
                    Categories
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
                    <th>TestCase</th>
                    <th>Failed / Total</th>
                    <th>Fail rate</th>
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
                    <th>TestCase</th>
                    <th>Status</th>
                    <th>Count</th>
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
