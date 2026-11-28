import {
  Bug,
  FileCheck2,
  FolderKanban,
  LayoutDashboard,
  Package,
  PlayCircle,
  Search,
  Settings,
  Sparkles,
  UserCircle2,
} from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, getAuthToken, type TestCase } from '../api/client'
import { useWorkspace } from '../context/WorkspaceContext'
import { useI18n } from '../i18n'

type AIHit = { type: string; title: string; row: Record<string, unknown> }

export function CommandPalette() {
  const { cmdkOpen, setCmdkOpen, activeProductId } = useWorkspace()
  const { t } = useI18n()
  const navigate = useNavigate()
  const [q, setQ] = useState('')
  const [cases, setCases] = useState<TestCase[]>([])
  const [aiBusy, setAiBusy] = useState(false)
  const [aiErr, setAiErr] = useState('')
  const [aiRes, setAiRes] = useState<{ explanation: string; model: string; items: AIHit[] } | null>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (!cmdkOpen) return
    setQ('')
    setAiRes(null)
    setAiErr('')
    setTimeout(() => inputRef.current?.focus(), 20)
    if (getAuthToken()) {
      void api
        .listTestCases({ product_id: activeProductId ?? undefined, limit: 30 })
        .then((r) => setCases(r.items))
        .catch(() => setCases([]))
    }
  }, [cmdkOpen, activeProductId])

  useEffect(() => {
    if (!cmdkOpen) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setCmdkOpen(false)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [cmdkOpen, setCmdkOpen])

  if (!cmdkOpen) return null

  const navItems = [
    { label: t.nav.dashboard, path: '/dashboard', icon: LayoutDashboard },
    { label: t.nav.testcases, path: '/cases', icon: FileCheck2 },
    { label: t.nav.testplans, path: '/plans', icon: FolderKanban },
    { label: t.nav.testruns, path: '/runs', icon: PlayCircle },
    { label: t.nav.bugs, path: '/bugs', icon: Bug },
    { label: t.nav.products, path: '/products', icon: Package },
    { label: t.nav.settings, path: '/settings', icon: Settings },
    { label: t.nav.auth, path: '/auth', icon: UserCircle2 },
  ]

  const lower = q.trim().toLowerCase()
  const filteredNav = navItems.filter((i) => !lower || i.label.toLowerCase().includes(lower))
  const filteredCases = cases
    .filter((c) => !lower || c.summary.toLowerCase().includes(lower) || `#${c.id}`.includes(lower))
    .slice(0, 8)

  const go = (path: string) => {
    setCmdkOpen(false)
    navigate(path)
  }

  const runAI = async () => {
    const query = q.trim()
    if (query.length < 3 || aiBusy) return
    setAiBusy(true)
    setAiErr('')
    setAiRes(null)
    try {
      const r = await api.aiSearch(query)
      setAiRes({ explanation: r.explanation, model: r.model, items: r.items })
    } catch {
      setAiErr(t.shell.cmdkAIErr)
    } finally {
      setAiBusy(false)
    }
  }

  const hitPath = (hit: AIHit): string | null => {
    const id = Number(hit.row.id)
    switch (hit.type) {
      case 'test_case':
        return `/cases/${id}`
      case 'test_run':
        return `/runs/${id}`
      case 'test_plan':
        return `/plans/${id}`
      case 'product':
        return '/products'
      default:
        return null
    }
  }

  const canAskAI = q.trim().length >= 3

  return (
    <div className="cmdk-backdrop" onClick={() => setCmdkOpen(false)}>
      <div className="cmdk-dialog" role="dialog" aria-modal="true" onClick={(e) => e.stopPropagation()}>
        <div className="cmdk-input-row">
          <Search size={16} color="var(--muted)" />
          <input
            ref={inputRef}
            className="cmdk-input"
            placeholder={t.shell.cmdkPlaceholder}
            value={q}
            onChange={(e) => {
              setQ(e.target.value)
              setAiRes(null)
              setAiErr('')
            }}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && canAskAI && e.ctrlKey) void runAI()
            }}
          />
          <span className="kbd">ESC</span>
        </div>
        <div className="cmdk-list">
          {canAskAI && (
            <div style={{ marginBottom: '0.4rem' }}>
              <button type="button" className="cmdk-item" onClick={() => void runAI()} disabled={aiBusy}>
                <span style={{ display: 'inline-flex', alignItems: 'center', gap: '0.55rem' }}>
                  <Sparkles size={15} color="var(--accent)" />
                  {aiBusy ? '…' : `${t.shell.cmdkAISearch}: ${q.trim()}`}
                </span>
                <span style={{ fontSize: '0.7rem', color: 'var(--muted)' }}>Ctrl+Enter</span>
              </button>
              <div style={{ fontSize: '0.7rem', color: 'var(--muted)', padding: '0.15rem 0.7rem 0.35rem' }}>
                {t.shell.cmdkAISearchHint}
              </div>
            </div>
          )}

          {aiErr && (
            <div style={{ padding: '0.6rem 0.7rem', color: 'var(--danger, #b00)', fontSize: '0.8rem' }}>{aiErr}</div>
          )}

          {aiRes && (
            <div style={{ marginBottom: '0.4rem' }}>
              <div
                style={{
                  fontSize: '0.7rem',
                  fontWeight: 600,
                  color: 'var(--muted)',
                  padding: '0.35rem 0.7rem',
                  textTransform: 'uppercase',
                }}
              >
                {t.shell.cmdkAIResults} ({aiRes.model})
              </div>
              <div style={{ fontSize: '0.75rem', color: 'var(--muted)', padding: '0 0.7rem 0.35rem' }}>
                {aiRes.explanation}
              </div>
              {aiRes.items.length === 0 && (
                <div style={{ padding: '0.5rem 0.7rem', color: 'var(--muted)', fontSize: '0.8rem' }}>
                  {t.shell.cmdkEmpty}
                </div>
              )}
              {aiRes.items.slice(0, 12).map((hit) => {
                const path = hitPath(hit)
                const content = (
                  <span
                    key="hit-content"
                    style={{
                      display: 'inline-flex',
                      alignItems: 'center',
                      gap: '0.5rem',
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                    }}
                  >
                    <span className="badge" style={{ fontFamily: 'var(--font-mono)' }}>
                      {hit.type} #{String(hit.row.id ?? '?')}
                    </span>
                    <span>{hit.title}</span>
                  </span>
                )
                return path ? (
                  <button
                    key={`${hit.type}-${String(hit.row.id ?? hit.title)}`}
                    type="button"
                    className="cmdk-item"
                    onClick={() => go(path)}
                  >
                    {content}
                    <span style={{ fontSize: '0.75rem', color: 'var(--muted)' }}>{t.shell.cmdkAIOpen}</span>
                  </button>
                ) : (
                  <div
                    key={`${hit.type}-${String(hit.row.id ?? hit.title)}-static`}
                    className="cmdk-item"
                    style={{ cursor: 'default' }}
                  >
                    {content}
                  </div>
                )
              })}
            </div>
          )}

          {filteredNav.length > 0 && (
            <div style={{ marginBottom: '0.4rem' }}>
              <div
                style={{
                  fontSize: '0.7rem',
                  fontWeight: 600,
                  color: 'var(--muted)',
                  padding: '0.35rem 0.7rem',
                  textTransform: 'uppercase',
                }}
              >
                {t.shell.cmdkNavigation}
              </div>
              {filteredNav.map((item) => {
                const Icon = item.icon
                return (
                  <button key={item.path} type="button" className="cmdk-item" onClick={() => go(item.path)}>
                    <span style={{ display: 'inline-flex', alignItems: 'center', gap: '0.55rem' }}>
                      <Icon size={15} />
                      {item.label}
                    </span>
                    <span style={{ fontSize: '0.75rem', color: 'var(--muted)' }}>{item.path}</span>
                  </button>
                )
              })}
            </div>
          )}

          {filteredCases.length > 0 && (
            <div>
              <div
                style={{
                  fontSize: '0.7rem',
                  fontWeight: 600,
                  color: 'var(--muted)',
                  padding: '0.35rem 0.7rem',
                  textTransform: 'uppercase',
                }}
              >
                {t.shell.cmdkTestCases}
              </div>
              {filteredCases.map((tc) => (
                <button key={tc.id} type="button" className="cmdk-item" onClick={() => go(`/cases/${tc.id}`)}>
                  <span
                    style={{
                      display: 'inline-flex',
                      alignItems: 'center',
                      gap: '0.5rem',
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                    }}
                  >
                    <span className="badge" style={{ fontFamily: 'var(--font-mono)' }}>
                      #{tc.id}
                    </span>
                    <span>{tc.summary}</span>
                  </span>
                  <span className={`badge pill-${tc.priority}`}>{tc.priority}</span>
                </button>
              ))}
            </div>
          )}

          {filteredNav.length === 0 && filteredCases.length === 0 && !canAskAI && (
            <div style={{ padding: '1.25rem', textAlign: 'center', color: 'var(--muted)', fontSize: '0.85rem' }}>
              {t.shell.cmdkEmpty}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
