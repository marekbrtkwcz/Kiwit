import { useEffect, useState } from 'react'
import { api, CaseHealth, MatrixRow, TelemetrySummary, TrendPoint } from '../../api/client'
import { useI18n } from '../../i18n'

export function DashboardView() {
  const { t } = useI18n()
  const [summary, setSummary] = useState<TelemetrySummary | null>(null)
  const [trends, setTrends] = useState<TrendPoint[]>([])
  const [matrix, setMatrix] = useState<MatrixRow[]>([])
  const [health, setHealth] = useState<CaseHealth[]>([])
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    void Promise.all([
      api.getTelemetrySummary(),
      api.getExecutionTrends(),
      api.getStatusMatrix(),
      api.getTestCaseHealth(),
    ])
      .then(([s, tr, mx, hl]) => {
        setSummary(s)
        setTrends(tr.items)
        setMatrix(mx.items)
        setHealth(hl.items)
        setError(null)
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : t.errors.authFailed)
      })
  }, [])

  if (error) {
    return <p className="alert-error">{error}</p>
  }

  if (!summary) {
    return <p style={{ color: 'var(--muted)' }}>{t.dashboard.loading}</p>
  }

  const total = summary.executions.total || 1
  const pct = (n: number) => `${Math.round((n / total) * 100)}%`

  return (
    <section style={{ display: 'grid', gap: '1.25rem' }}>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: '0.75rem' }}>
        {[
          { label: t.dashboard.products, value: summary.products },
          { label: t.dashboard.testCases, value: summary.test_cases },
          { label: t.dashboard.testPlans, value: summary.test_plans },
          { label: t.dashboard.testRuns, value: summary.test_runs },
        ].map((card) => (
          <div key={card.label} className="card">
            <div style={{ fontSize: '0.85rem', color: 'var(--muted)' }}>{card.label}</div>
            <div style={{ fontSize: '1.5rem', fontWeight: 700 }}>{card.value}</div>
          </div>
        ))}
      </div>

      <div className="card">
        <h3 style={{ marginTop: 0 }}>{t.dashboard.passFail} ({summary.executions.total})</h3>
        <div style={{ display: 'flex', height: 22, borderRadius: 6, overflow: 'hidden', background: '#f1f5f9' }}>
          <div style={{ width: pct(summary.executions.passed), background: '#16a34a' }} title={`${t.dashboard.passed}: ${summary.executions.passed}`} />
          <div style={{ width: pct(summary.executions.failed), background: '#dc2626' }} title={`${t.dashboard.failed}: ${summary.executions.failed}`} />
          <div style={{ width: pct(summary.executions.blocked), background: '#d97706' }} title={`${t.dashboard.blocked}: ${summary.executions.blocked}`} />
          <div style={{ width: pct(summary.executions.skipped), background: '#6b7280' }} title={`${t.dashboard.skipped}: ${summary.executions.skipped}`} />
        </div>
        <div style={{ display: 'flex', gap: '1rem', marginTop: '0.5rem', fontSize: '0.85rem', flexWrap: 'wrap' }}>
          <span style={{ color: '#16a34a' }}>{t.dashboard.passed}: {summary.executions.passed}</span>
          <span style={{ color: '#dc2626' }}>{t.dashboard.failed}: {summary.executions.failed}</span>
          <span style={{ color: '#d97706' }}>{t.dashboard.blocked}: {summary.executions.blocked}</span>
          <span style={{ color: '#6b7280' }}>{t.dashboard.skipped}: {summary.executions.skipped}</span>
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(340px, 1fr))', gap: '1rem' }}>
        <div className="card">
          <h3 style={{ marginTop: 0 }}>Test Case Health (Flaky / Failing)</h3>
          {health.length === 0 ? (
            <p style={{ color: 'var(--muted)', margin: 0 }}>No execution history yet.</p>
          ) : (
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
                  <tr key={h.test_case_id}>
                    <td>#{h.test_case_id} {h.summary}</td>
                    <td>{h.failed} / {h.total}</td>
                    <td style={{ color: h.fail_rate > 0 ? '#dc2626' : '#16a34a', fontWeight: 600 }}>
                      {Math.round(h.fail_rate * 100)}%
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>

        <div className="card">
          <h3 style={{ marginTop: 0 }}>Execution Trends</h3>
          {trends.length === 0 ? (
            <p style={{ color: 'var(--muted)', margin: 0 }}>No trend data yet.</p>
          ) : (
            <table className="data-table">
              <thead>
                <tr>
                  <th>Date</th>
                  <th>Status</th>
                  <th>Count</th>
                </tr>
              </thead>
              <tbody>
                {trends.slice(-12).map((pt, i) => (
                  <tr key={`${pt.date}-${pt.status}-${i}`}>
                    <td>{pt.date}</td>
                    <td><span className="badge">{pt.status}</span></td>
                    <td>{pt.count}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>

      {matrix.length > 0 && (
        <div className="card">
          <h3 style={{ marginTop: 0 }}>Status Matrix</h3>
          <table className="data-table">
            <thead>
              <tr>
                <th>TestCase</th>
                <th>Status</th>
                <th>Count</th>
              </tr>
            </thead>
            <tbody>
              {matrix.slice(0, 20).map((m, i) => (
                <tr key={`${m.test_case_id}-${m.status}-${i}`}>
                  <td>#{m.test_case_id} {m.summary}</td>
                  <td><span className="badge">{m.status}</span></td>
                  <td>{m.count}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  )
}
