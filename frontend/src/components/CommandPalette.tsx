import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  Bug,
  FileCheck2,
  FolderKanban,
  LayoutDashboard,
  Package,
  PlayCircle,
  Search,
  Settings,
  UserCircle2,
} from 'lucide-react'
import { api, getAuthToken, TestCase } from '../api/client'
import { useWorkspace } from '../context/WorkspaceContext'
import { useI18n } from '../i18n'

export function CommandPalette() {
  const { cmdkOpen, setCmdkOpen, activeProductId } = useWorkspace()
  const { t } = useI18n()
  const navigate = useNavigate()
  const [q, setQ] = useState('')
  const [cases, setCases] = useState<TestCase[]>([])
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (!cmdkOpen) return
    setQ('')
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
            onChange={(e) => setQ(e.target.value)}
          />
          <span className="kbd">ESC</span>
        </div>
        <div className="cmdk-list">
          {filteredNav.length > 0 && (
            <div style={{ marginBottom: '0.4rem' }}>
              <div style={{ fontSize: '0.7rem', fontWeight: 600, color: 'var(--muted)', padding: '0.35rem 0.7rem', textTransform: 'uppercase' }}>
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
              <div style={{ fontSize: '0.7rem', fontWeight: 600, color: 'var(--muted)', padding: '0.35rem 0.7rem', textTransform: 'uppercase' }}>
                {t.shell.cmdkTestCases}
              </div>
              {filteredCases.map((tc) => (
                <button key={tc.id} type="button" className="cmdk-item" onClick={() => go(`/cases/${tc.id}`)}>
                  <span style={{ display: 'inline-flex', alignItems: 'center', gap: '0.5rem', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                    <span className="badge" style={{ fontFamily: 'var(--font-mono)' }}>#{tc.id}</span>
                    <span>{tc.summary}</span>
                  </span>
                  <span className={`badge pill-${tc.priority}`}>{tc.priority}</span>
                </button>
              ))}
            </div>
          )}

          {filteredNav.length === 0 && filteredCases.length === 0 && (
            <div style={{ padding: '1.25rem', textAlign: 'center', color: 'var(--muted)', fontSize: '0.85rem' }}>
              {t.shell.cmdkEmpty}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
