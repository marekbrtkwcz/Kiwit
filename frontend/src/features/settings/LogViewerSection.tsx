import { useCallback, useEffect, useMemo, useState } from 'react'
import { RefreshCw, ScrollText, Search, X } from 'lucide-react'
import { api } from '../../api/client'

type LogEntry = { time: string; level: string; message: string; attrs?: Record<string, string> }

const LEVELS = ['all', 'info', 'warn', 'error']
const LEVEL_BADGE: Record<string, string> = {
  debug: '',
  info: 'pill-passed',
  warn: 'pill-blocked',
  error: 'pill-failed',
}

type SortKey = 'time' | 'level' | 'message'
const LEVEL_ORDER: Record<string, number> = { error: 0, warn: 1, info: 2, debug: 3 }

// LogViewerSection shows the in-memory server log buffer (admin).
export function LogViewerSection() {
  const [entries, setEntries] = useState<LogEntry[]>([])
  const [level, setLevel] = useState('all')
  const [query, setQuery] = useState('')
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const [sort, setSort] = useState<{ key: SortKey; desc: boolean }>({ key: 'time', desc: true })
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

  // Text search and time range run client-side over the loaded entries;
  // whitespace-separated terms must all match (AND). Sorting afterwards.
  const filtered = useMemo(() => {
    const terms = query.trim().toLowerCase().split(/\s+/).filter(Boolean)
    const fromTs = from ? new Date(from).getTime() : NaN
    const toTs = to ? new Date(to).getTime() + 60_000 : NaN // include the picked minute
    const rows = entries.filter((e) => {
      const ts = new Date(e.time).getTime()
      if (!Number.isNaN(fromTs) && ts < fromTs) return false
      if (!Number.isNaN(toTs) && ts > toTs) return false
      if (terms.length === 0) return true
      const haystack = `${e.message} ${e.level} ${Object.entries(e.attrs ?? {}).map(([k, v]) => `${k}=${v}`).join(' ')}`.toLowerCase()
      return terms.every((t) => haystack.includes(t))
    })
    const dir = sort.desc ? -1 : 1
    return [...rows].sort((a, b) => {
      switch (sort.key) {
        case 'level': {
          const d = (LEVEL_ORDER[a.level] ?? 9) - (LEVEL_ORDER[b.level] ?? 9)
          return d !== 0 ? d * dir : a.time.localeCompare(b.time) * -1
        }
        case 'message':
          return a.message.localeCompare(b.message) * dir
        default:
          return a.time.localeCompare(b.time) * dir
      }
    })
  }, [entries, query, from, to, sort])

  const sortBtn = (key: SortKey, label: string) => (
    <button
      type="button"
      className="btn btn-ghost btn-sm"
      style={{ padding: 0, fontSize: 'inherit', fontWeight: 'inherit', textTransform: 'inherit' }}
      onClick={() => setSort((s) => (s.key === key ? { key, desc: !s.desc } : { key, desc: key !== 'time' }))}
      aria-label={`Sort by ${label}`}
    >
      {label} {sort.key === key ? (sort.desc ? '↓' : '↑') : ''}
    </button>
  )

  return (
    <div className="card">
      <div className="card-header">
        <h3 className="card-title"><ScrollText size={15} /> Server logs</h3>
        <div style={{ display: 'flex', gap: '0.4rem', alignItems: 'center', flexWrap: 'wrap' }}>
          <div style={{ position: 'relative' }}>
            <Search size={13} style={{ position: 'absolute', left: 8, top: 9, color: 'var(--muted)', pointerEvents: 'none' }} />
            <input
              className="input"
              placeholder="Search message, details…"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              aria-label="Search logs"
              style={{ width: 220, fontSize: '0.8rem', paddingLeft: 26 }}
            />
          </div>
          <label style={{ fontSize: '0.8rem', display: 'inline-flex', gap: '0.25rem', alignItems: 'center' }}>
            <input className="input" type="datetime-local" value={from} onChange={(e) => setFrom(e.target.value)} aria-label="From time" style={{ fontSize: '0.8rem' }} />
          </label>
          <label style={{ fontSize: '0.8rem', display: 'inline-flex', gap: '0.25rem', alignItems: 'center' }}>
            <input className="input" type="datetime-local" value={to} onChange={(e) => setTo(e.target.value)} aria-label="To time" style={{ fontSize: '0.8rem' }} />
          </label>
          {(from || to) && (
            <button type="button" className="btn btn-ghost btn-sm" onClick={() => { setFrom(''); setTo('') }} aria-label="Clear time range">
              <X size={13} />
            </button>
          )}
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
      ) : filtered.length === 0 ? (
        <p style={{ color: 'var(--muted)', fontSize: '0.85rem', margin: 0 }}>No entries match the search.</p>
      ) : (
        <div className="table-wrap" style={{ maxHeight: '45vh', overflowY: 'auto' }}>
          <table className="data-table">
            <thead>
              <tr>
                <th>{sortBtn('time', 'Time')}</th>
                <th>{sortBtn('level', 'Level')}</th>
                <th>{sortBtn('message', 'Message')}</th>
                <th>Details</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((e, i) => (
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
        Last 2000 entries in memory{query ? `, showing ${filtered.length} of ${entries.length}` : ''}. Set KIWIT_LOG_FILE / KIWIT_LOG_LEVEL / KIWIT_LOG_FORMAT for persistent output.
      </p>
    </div>
  )
}
