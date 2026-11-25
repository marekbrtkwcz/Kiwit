import { useState } from 'react'
import { AuthView } from './features/auth/AuthView'
import { ProductsView } from './features/management/ProductsView'
import { DashboardView } from './features/telemetry/DashboardView'
import { TestCasesView } from './features/testcases/TestCasesView'
import { TestPlansView } from './features/testplans/TestPlansView'
import { TestRunsView } from './features/testruns/TestRunsView'
import { I18nProvider, Lang, useI18n } from './i18n'

type Tab = 'dashboard' | 'products' | 'testcases' | 'testplans' | 'testruns' | 'auth'

function AppShell() {
  const [tab, setTab] = useState<Tab>('dashboard')
  const { lang, setLang, t } = useI18n()

  return (
    <div style={{ fontFamily: 'system-ui, sans-serif', maxWidth: 1100, margin: '0 auto', padding: '1.5rem' }}>
      <header style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', borderBottom: '1px solid #e5e7eb', paddingBottom: '1rem', marginBottom: '1.5rem', gap: '1rem' }}>
        <h1 style={{ margin: 0, fontSize: '1.25rem' }}>Kiwit TCMS</h1>
        <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
          <nav style={{ display: 'flex', gap: '0.5rem' }}>
            {(['dashboard', 'products', 'testcases', 'testplans', 'testruns', 'auth'] as Tab[]).map((item) => (
              <button
                key={item}
                onClick={() => setTab(item)}
                style={{
                  padding: '0.4rem 0.8rem',
                  borderRadius: 6,
                  border: '1px solid #d1d5db',
                  background: tab === item ? '#111827' : '#ffffff',
                  color: tab === item ? '#ffffff' : '#111827',
                  cursor: 'pointer',
                }}
              >
                {t.nav[item]}
              </button>
            ))}
          </nav>
          <div style={{ display: 'flex', border: '1px solid #d1d5db', borderRadius: 6, overflow: 'hidden' }}>
            {(['pl', 'en'] as Lang[]).map((l) => (
              <button
                key={l}
                type="button"
                onClick={() => setLang(l)}
                style={{
                  padding: '0.35rem 0.6rem',
                  border: 'none',
                  background: lang === l ? '#111827' : '#ffffff',
                  color: lang === l ? '#ffffff' : '#111827',
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
