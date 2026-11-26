import { useCallback, useEffect, useState } from 'react'
import { RefreshCw, ScrollText } from 'lucide-react'
import { api } from '../../api/client'

type LogEntry = { time: string; level: string; message: string; attrs?: Record<string, string> }

const LEVELS = ['all', 'info', 'warn', 'error']
const LEVEL_BADGE: Record<string, string> = {
  debug: '',
  info: 'pill-passed',
  warn: 'pill-blocked',
  error: 'pill-failed',
}

// LogViewerSection shows the in-memory server log buffer (admin).
export function LogViewerSection() {
  const [entries, setEntries] = useState<LogEntry[]>([])
  const [level, setLevel] = useState('all')
  const [busy, setBusy] = useState(false)

  const load = useCallback(async () => {
    setBusy(true)
    try {
      const res = await api.getLogs({ level: level === 'all' ? undefined : level, limit: 200 })
      setEntries(res.items ?? [])
    } catch {
      setEntries([])
    } finally {
      setBusy(false)
    }
  }, [level])

  useEffect(() => {
    void load()
  }, [load])

  return (
    <div className="card">
      <div className="card-header">
        <h3 className="card-title"><ScrollText size={15} /> Server logs</h3>
        <div style={{ display: 'flex', gap: '0.4rem', alignItems: 'center' }}>
          <select className="select" value={level} onChange={(e) => setLevel(e.target.value)} style={{ fontSize: '0.8rem' }} aria-label="Log level filter">
            {LEVELS.map((l) => (
              <option key={l} value={l}>{l}</option>
            ))}
          </select>
          <button type="button" className="btn btn-sm" onClick={load} disabled={busy} aria-label="Refresh logs">
            <RefreshCw size={13} /> {busy ? 'Loading…' : 'Refresh'}
          </button>
        </div>
      </div>
      {entries.length === 0 ? (
        <p style={{ color: 'var(--muted)', fontSize: '0.85rem', margin: 0 }}>No log entries for the selected level.</p>
      ) : (
        <div className="table-wrap" style={{ maxHeight: '45vh', overflowY: 'auto' }}>
          <table className="data-table">
            <thead>
              <tr>
                <th>Time</th>
                <th>Level</th>
                <th>Message</th>
                <th>Details</th>
              </tr>
            </thead>
            <tbody>
              {entries.map((e, i) => (
                <tr key={i}>
                  <td style={{ whiteSpace: 'nowrap', fontFamily: 'var(--font-mono)', fontSize: '0.75rem', color: 'var(--muted)' }}>
                    {new Date(e.time).toLocaleString()}
                  </td>
                  <td><span className={`badge ${LEVEL_BADGE[e.level] ?? ''}`}>{e.level}</span></td>
                  <td style={{ fontSize: '0.82rem' }}>{e.message}</td>
                  <td style={{ fontFamily: 'var(--font-mono)', fontSize: '0.72rem', color: 'var(--muted)' }}>
                    {e.attrs ? Object.entries(e.attrs).map(([k, v]) => `${k}=${v}`).join(' ') : ''}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <p style={{ color: 'var(--muted)', fontSize: '0.75rem', margin: 0 }}>
        Last 2000 entries in memory. Set KIWIT_LOG_FILE / KIWIT_LOG_LEVEL / KIWIT_LOG_FORMAT for persistent output.
      </p>
    </div>
  )
}
