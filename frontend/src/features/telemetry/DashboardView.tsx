import { useEffect, useState } from 'react'
import { api, TelemetrySummary } from '../../api/client'
import { useI18n } from '../../i18n'

export function DashboardView() {
  const { t } = useI18n()
  const [summary, setSummary] = useState<TelemetrySummary | null>(null)

  useEffect(() => {
    void api.getTelemetrySummary().then(setSummary).catch(() => {})
  }, [])

  if (!summary) {
    return <p style={{ color: '#6b7280' }}>{t.dashboard.loading}</p>
  }

  const total = summary.executions.total || 1
  const pct = (n: number) => `${Math.round((n / total) * 100)}%`

  return (
    <section style={{ display: 'grid', gap: '1.25rem' }}>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: '0.75rem' }}>
        {[
          { label: t.dashboard.products, value: summary.products },
          { label: t.dashboard.testCases, value: summary.test_cases },
          { label: t.dashboard.testPlans, value: summary.test_plans },
          { label: t.dashboard.testRuns, value: summary.test_runs },
        ].map((card) => (
          <div key={card.label} style={{ padding: '1rem', border: '1px solid #e5e7eb', borderRadius: 8 }}>
            <div style={{ fontSize: '0.85rem', color: '#6b7280' }}>{card.label}</div>
            <div style={{ fontSize: '1.5rem', fontWeight: 700 }}>{card.value}</div>
          </div>
        ))}
      </div>

      <div style={{ padding: '1rem', border: '1px solid #e5e7eb', borderRadius: 8 }}>
        <h3 style={{ marginTop: 0 }}>{t.dashboard.passFail} ({summary.executions.total})</h3>
        <div style={{ display: 'flex', height: 22, borderRadius: 6, overflow: 'hidden', background: '#f3f4f6' }}>
          <div style={{ width: pct(summary.executions.passed), background: '#16a34a' }} title={`${t.dashboard.passed}: ${summary.executions.passed}`} />
          <div style={{ width: pct(summary.executions.failed), background: '#dc2626' }} title={`${t.dashboard.failed}: ${summary.executions.failed}`} />
          <div style={{ width: pct(summary.executions.blocked), background: '#d97706' }} title={`${t.dashboard.blocked}: ${summary.executions.blocked}`} />
          <div style={{ width: pct(summary.executions.skipped), background: '#6b7280' }} title={`${t.dashboard.skipped}: ${summary.executions.skipped}`} />
        </div>
        <div style={{ display: 'flex', gap: '1rem', marginTop: '0.5rem', fontSize: '0.85rem' }}>
          <span style={{ color: '#16a34a' }}>{t.dashboard.passed}: {summary.executions.passed}</span>
          <span style={{ color: '#dc2626' }}>{t.dashboard.failed}: {summary.executions.failed}</span>
          <span style={{ color: '#d97706' }}>{t.dashboard.blocked}: {summary.executions.blocked}</span>
          <span style={{ color: '#6b7280' }}>{t.dashboard.skipped}: {summary.executions.skipped}</span>
        </div>
      </div>
    </section>
  )
}
