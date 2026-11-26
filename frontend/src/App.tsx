import { useState } from 'react'
import { BrowserRouter, Navigate, NavLink, Route, Routes, useLocation } from 'react-router-dom'
import {
  Bug,
  CheckCircle2,
  FileCheck2,
  FolderKanban,
  LayoutDashboard,
  Menu,
  Moon,
  Package,
  PanelLeftClose,
  PanelLeftOpen,
  PlayCircle,
  Search,
  Settings,
  Sun,
  UserCircle2,
  X,
} from 'lucide-react'
import { getAuthToken } from './api/client'
import { CommandPalette } from './components/CommandPalette'
import { useWorkspace, WorkspaceProvider } from './context/WorkspaceContext'
import { AuthView } from './features/auth/AuthView'
import { BugsView } from './features/bugs/BugsView'
import { ProductsView } from './features/management/ProductsView'
import { NotificationBell } from './features/notifications/NotificationBell'
import { SettingsView } from './features/settings/SettingsView'
import { DashboardView } from './features/telemetry/DashboardView'
import { TestCasesView } from './features/testcases/TestCasesView'
import { TestPlansView } from './features/testplans/TestPlansView'
import { TestRunsView } from './features/testruns/TestRunsView'
import { I18nProvider, Lang, useI18n } from './i18n'

function AppShell() {
  const { lang, setLang, t } = useI18n()
  const { theme, toggleTheme, products, activeProductId, setActiveProductId, setCmdkOpen, toasts } = useWorkspace()
  const [collapsed, setCollapsed] = useState(false)
  const [mobileOpen, setMobileOpen] = useState(false)
  const location = useLocation()

  const navLinks = [
    { to: '/dashboard', label: t.nav.dashboard, icon: LayoutDashboard },
    { to: '/cases', label: t.nav.testcases, icon: FileCheck2 },
    { to: '/plans', label: t.nav.testplans, icon: FolderKanban },
    { to: '/runs', label: t.nav.testruns, icon: PlayCircle },
    { to: '/bugs', label: t.nav.bugs, icon: Bug },
    { to: '/products', label: t.nav.products, icon: Package },
    { to: '/settings', label: t.nav.settings, icon: Settings },
    { to: '/auth', label: t.nav.auth, icon: UserCircle2 },
  ]

  const currentNav = navLinks.find((n) => location.pathname.startsWith(n.to))
  const pageTitle = currentNav?.label ?? 'Kiwit TCMS'

  return (
    <div className="layout-root">
      {mobileOpen && (
        <div
          className="drawer-backdrop"
          style={{ justifyContent: 'flex-start' }}
          onClick={() => setMobileOpen(false)}
        />
      )}

      <aside className={`sidebar ${collapsed ? 'collapsed' : ''} ${mobileOpen ? 'mobile-open' : ''}`}>
        <div className="sidebar-brand">
          <NavLink to="/dashboard" className="brand-mark" onClick={() => setMobileOpen(false)}>
            <span className="brand-logo">
              <CheckCircle2 size={17} />
            </span>
            {!collapsed && <span>Kiwit TCMS</span>}
          </NavLink>
          <button
            type="button"
            className="btn btn-ghost btn-icon"
            onClick={() => {
              if (mobileOpen) setMobileOpen(false)
              else setCollapsed((c) => !c)
            }}
            title={collapsed ? t.shell.expandSidebar : t.shell.collapseSidebar}
            aria-label={collapsed ? t.shell.expandSidebar : t.shell.collapseSidebar}
          >
            {mobileOpen ? <X size={16} /> : collapsed ? <PanelLeftOpen size={16} /> : <PanelLeftClose size={16} />}
          </button>
        </div>

        {!collapsed && products.length > 0 && (
          <div className="sidebar-product">
            <label htmlFor="global-product-select">{t.shell.productScope}</label>
            <select
              id="global-product-select"
              className="select"
              style={{ width: '100%' }}
              value={activeProductId ?? ''}
              onChange={(e) => setActiveProductId(e.target.value ? Number(e.target.value) : null)}
            >
              <option value="">{t.shell.allProducts}</option>
              {products.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name}
                </option>
              ))}
            </select>
          </div>
        )}

        <nav className="sidebar-nav" aria-label="Main navigation">
          {navLinks.map((item) => {
            const Icon = item.icon
            return (
              <NavLink
                key={item.to}
                to={item.to}
                className={({ isActive }) => `nav-item ${isActive ? 'active' : ''}`}
                onClick={() => setMobileOpen(false)}
                title={collapsed ? item.label : undefined}
              >
                <Icon size={17} style={{ flexShrink: 0 }} />
                {!collapsed && <span>{item.label}</span>}
              </NavLink>
            )
          })}
        </nav>

        <div className="sidebar-footer">
          {!collapsed && (
            <div style={{ fontSize: '0.73rem', color: 'var(--muted)', padding: '0.2rem 0.45rem' }}>
              SQLite &amp; PostgreSQL · OpenAPI
            </div>
          )}
        </div>
      </aside>

      <div className="main-column">
        <header className="topbar">
          <div className="topbar-left">
            <button
              type="button"
              className="btn btn-ghost btn-icon"
              style={{ display: 'inline-flex' }}
              onClick={() => setMobileOpen((v) => !v)}
              aria-label="Toggle menu"
            >
              <Menu size={18} />
            </button>
            <h1 className="topbar-title">{pageTitle}</h1>
            {activeProductId && (
              <span className="badge" style={{ background: 'var(--primary-soft)', color: 'var(--primary-text)', borderColor: 'transparent' }}>
                {products.find((p) => p.id === activeProductId)?.name ?? `#${activeProductId}`}
              </span>
            )}
          </div>

          <div className="topbar-actions">
            <button type="button" className="cmdk-trigger" onClick={() => setCmdkOpen(true)}>
              <Search size={14} />
              <span>{t.shell.searchPlaceholder}</span>
              <span className="kbd">Ctrl K</span>
            </button>

            <button
              type="button"
              className="btn btn-ghost btn-icon"
              onClick={toggleTheme}
              title={theme === 'dark' ? t.shell.lightMode : t.shell.darkMode}
              aria-label={theme === 'dark' ? t.shell.lightMode : t.shell.darkMode}
            >
              {theme === 'dark' ? <Sun size={16} /> : <Moon size={16} />}
            </button>

            <NotificationBell />

            <div style={{ display: 'inline-flex', border: '1px solid var(--border-strong)', borderRadius: 8, overflow: 'hidden' }}>
              {(['pl', 'en'] as Lang[]).map((l) => (
                <button
                  key={l}
                  type="button"
                  onClick={() => setLang(l)}
                  style={{
                    padding: '0.3rem 0.55rem',
                    border: 'none',
                    background: lang === l ? 'var(--primary)' : 'var(--surface)',
                    color: lang === l ? '#ffffff' : 'var(--text-secondary)',
                    cursor: 'pointer',
                    fontSize: '0.75rem',
                    fontWeight: 600,
                  }}
                >
                  {l.toUpperCase()}
                </button>
              ))}
            </div>
          </div>
        </header>

        <main className="workspace-content">
          <Routes>
            <Route path="/" element={<Navigate to={getAuthToken() ? '/dashboard' : '/auth'} replace />} />
            <Route path="/dashboard" element={<DashboardView />} />
            <Route path="/products" element={<ProductsView />} />
            <Route path="/cases" element={<TestCasesView />} />
            <Route path="/cases/:id" element={<TestCasesView />} />
            <Route path="/plans" element={<TestPlansView />} />
            <Route path="/plans/:id" element={<TestPlansView />} />
            <Route path="/runs" element={<TestRunsView />} />
            <Route path="/runs/:id" element={<TestRunsView />} />
            <Route path="/bugs" element={<BugsView />} />
            <Route path="/settings" element={<SettingsView />} />
            <Route path="/auth" element={<AuthView />} />
            <Route path="*" element={<Navigate to="/dashboard" replace />} />
          </Routes>
        </main>
      </div>

      <CommandPalette />

      {toasts.length > 0 && (
        <div className="toast-stack" role="status" aria-live="polite">
          {toasts.map((toast) => (
            <div key={toast.id} className={`toast ${toast.kind}`}>
              {toast.text}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

export default function App() {
  return (
    <I18nProvider>
      <WorkspaceProvider>
        <BrowserRouter>
          <AppShell />
        </BrowserRouter>
      </WorkspaceProvider>
    </I18nProvider>
  )
}
