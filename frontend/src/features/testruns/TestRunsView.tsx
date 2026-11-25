import { useEffect, useState } from 'react'
import { api, TestExecution, TestPlan, TestRun } from '../../api/client'
import { useI18n } from '../../i18n'
import { AttachmentsPanel } from '../attachments/AttachmentsPanel'

const STATUSES: TestExecution['status'][] = ['passed', 'failed', 'blocked', 'skipped']

const STATUS_COLORS: Record<string, string> = {
  passed: '#16a34a',
  failed: '#dc2626',
  blocked: '#d97706',
  skipped: '#6b7280',
  idle: '#9ca3af',
  running: '#2563eb',
}

export function TestRunsView() {
  const { t } = useI18n()
  const [plans, setPlans] = useState<TestPlan[]>([])
  const [runs, setRuns] = useState<TestRun[]>([])
  const [selectedRunId, setSelectedRunId] = useState<number | null>(null)
  const [executions, setExecutions] = useState<TestExecution[]>([])
  const [planId, setPlanId] = useState<number>(0)
  const [buildId, setBuildId] = useState<number>(1)
  const [summary, setSummary] = useState('')
  const [error, setError] = useState<string | null>(null)

  const loadRuns = async () => {
    try {
      const [tpRes, trRes] = await Promise.all([api.listTestPlans(), api.listTestRuns()])
      setPlans(tpRes.items)
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
    if (selectedRunId === null) return
    void api.listExecutions(selectedRunId).then((res) => setExecutions(res.items)).catch(() => {})
  }, [selectedRunId])

  const handleCreateRun = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!summary.trim() || !planId) return
    try {
      const created = await api.createTestRun({ test_plan_id: planId, build_id: buildId, summary: summary.trim() })
      setSummary('')
      setSelectedRunId(created.id)
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

  return (
    <section>
      <h2>{t.testruns.title}</h2>
      {error && <p style={{ color: '#b91c1c' }}>{error}</p>}

      <form onSubmit={handleCreateRun} style={{ display: 'flex', gap: '0.5rem', marginBottom: '1rem' }}>
        <select value={planId} onChange={(e) => setPlanId(Number(e.target.value))} style={{ padding: '0.4rem', borderRadius: 6, border: '1px solid #d1d5db' }}>
          {plans.map((tp) => (
            <option key={tp.id} value={tp.id}>{tp.name}</option>
          ))}
        </select>
        <input
          type="number"
          min={1}
          value={buildId}
          onChange={(e) => setBuildId(Number(e.target.value))}
          placeholder="Build ID"
          style={{ width: 90, padding: '0.4rem', borderRadius: 6, border: '1px solid #d1d5db' }}
        />
        <input
          placeholder={t.testruns.summaryPlaceholder}
          value={summary}
          onChange={(e) => setSummary(e.target.value)}
          style={{ flex: 1, padding: '0.4rem 0.6rem', borderRadius: 6, border: '1px solid #d1d5db' }}
        />
        <button type="submit" disabled={!planId} style={{ padding: '0.4rem 0.9rem', borderRadius: 6, background: '#111827', color: '#fff', border: 'none', cursor: 'pointer' }}>
          {t.testruns.create}
        </button>
      </form>

      <div style={{ display: 'grid', gridTemplateColumns: '280px 1fr', gap: '1rem' }}>
        <ul style={{ listStyle: 'none', padding: 0, margin: 0, display: 'grid', gap: '0.4rem', alignContent: 'start' }}>
          {runs.map((r) => (
            <li
              key={r.id}
              onClick={() => setSelectedRunId(r.id)}
              style={{
                padding: '0.6rem',
                borderRadius: 8,
                border: selectedRunId === r.id ? '2px solid #111827' : '1px solid #e5e7eb',
                cursor: 'pointer',
              }}
            >
              <strong>{r.summary}</strong>
              <div style={{ fontSize: '0.8rem', color: '#6b7280' }}>Plan #{r.test_plan_id} · Build #{r.build_id}</div>
            </li>
          ))}
          {runs.length === 0 && <li style={{ color: '#6b7280' }}>{t.testruns.emptyRuns}</li>}
        </ul>

        <div style={{ border: '1px solid #e5e7eb', borderRadius: 8, padding: '0.75rem' }}>
          <h3 style={{ marginTop: 0 }}>{t.testruns.executions}</h3>
          {executions.length === 0 && <p style={{ color: '#6b7280' }}>{t.testruns.emptyExecutions}</p>}
          <ul style={{ listStyle: 'none', padding: 0, display: 'grid', gap: '0.5rem' }}>
            {executions.map((ex) => (
              <li key={ex.id} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '0.5rem', border: '1px solid #f3f4f6', borderRadius: 6 }}>
                <div>
                  <strong>{ex.summary}</strong>
                  <span style={{ marginLeft: '0.5rem', fontSize: '0.8rem', color: STATUS_COLORS[ex.status] ?? '#111827', fontWeight: 600 }}>
                    {ex.status.toUpperCase()}
                  </span>
                  <AttachmentsPanel entityType="test_execution" entityId={ex.id} />
                </div>
                <div style={{ display: 'flex', gap: '0.25rem' }}>
                  {STATUSES.map((st) => (
                    <button
                      key={st}
                      type="button"
                      onClick={() => void handleStatus(ex.id, st)}
                      style={{
                        padding: '0.25rem 0.5rem',
                        borderRadius: 4,
                        border: '1px solid #d1d5db',
                        background: ex.status === st ? STATUS_COLORS[st] : '#fff',
                        color: ex.status === st ? '#fff' : '#111827',
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
