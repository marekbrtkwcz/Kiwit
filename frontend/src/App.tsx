import { useEffect, useState } from 'react'
import { getAuthToken } from './api/client'
import { AuthView } from './features/auth/AuthView'
import { ProductsView } from './features/management/ProductsView'
import { SettingsView } from './features/settings/SettingsView'
import { DashboardView } from './features/telemetry/DashboardView'
import { TestCasesView } from './features/testcases/TestCasesView'
import { TestPlansView } from './features/testplans/TestPlansView'
import { TestRunsView } from './features/testruns/TestRunsView'
import { I18nProvider, Lang, useI18n } from './i18n'

type Tab = 'dashboard' | 'products' | 'testcases' | 'testplans' | 'testruns' | 'settings' | 'auth'
const VALID_TABS: Tab[] = ['dashboard', 'products', 'testcases', 'testplans', 'testruns', 'settings', 'auth']

function tabFromHash(): Tab {
  const raw = window.location.hash.replace(/^#\/?/, '') as Tab
  if (VALID_TABS.includes(raw)) return raw
  return getAuthToken() ? 'dashboard' : 'auth'
}

function AppShell() {
  const [tab, setTabState] = useState<Tab>(tabFromHash)
  const { lang, setLang, t } = useI18n()

  const setTab = (next: Tab) => {
    window.location.hash = next
    setTabState(next)
  }

  useEffect(() => {
    const onHashChange = () => setTabState(tabFromHash())
    window.addEventListener('hashchange', onHashChange)
    return () => window.removeEventListener('hashchange', onHashChange)
  }, [])

  return (
    <div className="app-shell">
      <header className="app-header">
        <h1 style={{ margin: 0, fontSize: '1.25rem' }}>Kiwit TCMS</h1>
        <div style={{ display: 'flex', alignItems: 'center', gap: '1rem', flexWrap: 'wrap' }}>
          <nav style={{ display: 'flex', gap: '0.45rem', flexWrap: 'wrap' }}>
            {(['dashboard', 'products', 'testcases', 'testplans', 'testruns', 'settings', 'auth'] as Tab[]).map((item) => (
              <button
                key={item}
                onClick={() => setTab(item)}
                className={tab === item ? 'btn btn-primary' : 'btn'}
              >
                {t.nav[item]}
              </button>
            ))}
          </nav>
          <div style={{ display: 'flex', border: '1px solid var(--border-strong)', borderRadius: 6, overflow: 'hidden' }}>
            {(['pl', 'en'] as Lang[]).map((l) => (
              <button
                key={l}
                type="button"
                onClick={() => setLang(l)}
                style={{
                  padding: '0.35rem 0.6rem',
                  border: 'none',
                  background: lang === l ? 'var(--primary)' : 'var(--surface)',
                  color: lang === l ? '#ffffff' : 'var(--text)',
                  cursor: 'pointer',
                  fontSize: '0.8rem',
                  fontWeight: 600,
                }}
              >
                {l.toUpperCase()}
              </button>
            ))}
          </div>
        </div>
      </header>

      <main>
        {tab === 'dashboard' && <DashboardView />}
        {tab === 'products' && <ProductsView />}
        {tab === 'testcases' && <TestCasesView />}
        {tab === 'testplans' && <TestPlansView />}
        {tab === 'testruns' && <TestRunsView />}
        {tab === 'settings' && <SettingsView />}
        {tab === 'auth' && <AuthView />}
      </main>
    </div>
  )
}

export default function App() {
  return (
    <I18nProvider>
      <AppShell />
    </I18nProvider>
  )
}
