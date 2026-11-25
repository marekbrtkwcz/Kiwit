import { createContext, ReactNode, useContext, useState } from 'react'

export type Lang = 'pl' | 'en'

const LANG_KEY = 'kiwit_lang'

const dict = {
  pl: {
    nav: {
      dashboard: 'Dashboard',
      products: 'Produkty',
      testcases: 'Scenariusze',
      testplans: 'Plany testowe',
      testruns: 'Uruchomienia',
      settings: 'Ustawienia',
      auth: 'Konto',
    },
    products: {
      title: 'Produkty',
      namePlaceholder: 'Nazwa produktu',
      descPlaceholder: 'Opis (opcjonalnie)',
      add: 'Dodaj produkt',
      empty: 'Brak produktów.',
      versions: 'Wersje',
      versionPlaceholder: 'Wersja (np. 1.0.0)',
      addVersion: '+ Wersja',
      buildPlaceholder: 'Build (np. b100)',
      addBuild: '+ Build',
    },
    testcases: {
      title: 'Scenariusze testowe (TestCases)',
      summaryPlaceholder: 'Tytuł scenariusza',
      stepsLabel: 'Kroki:',
      actionPlaceholder: 'Akcja',
      expectedPlaceholder: 'Oczekiwany rezultat',
      addStep: '+ Dodaj krok',
      save: 'Zapisz scenariusz',
      empty: 'Brak scenariuszy.',
      stepsCount: 'kroków',
      newCategoryPlaceholder: 'Nowy rodzaj (np. UX)',
      addCategory: '+ Rodzaj',
    },
    testplans: {
      title: 'Plany testowe (TestPlans)',
      namePlaceholder: 'Nazwa planu testowego',
      create: 'Utwórz plan',
      empty: 'Brak planów testowych.',
      attachTitle: 'Podepnij scenariusze do planu',
      attachBtn: 'Podepnij zaznaczone',
    },
    testruns: {
      title: 'Uruchomienia testów (TestRuns)',
      summaryPlaceholder: 'Nazwa uruchomienia (np. Smoke v1.0)',
      create: 'Uruchom TestRun',
      executions: 'Wykonania',
      emptyRuns: 'Brak uruchomień.',
      emptyExecutions: 'Brak wykonań dla wybranego TestRun.',
    },
    auth: {
      title: 'Konto i tokeny CI/CD',
      login: 'Logowanie',
      register: 'Rejestracja',
      usernamePlaceholder: 'Nazwa użytkownika',
      passwordPlaceholder: 'Hasło (min. 8 znaków)',
      submitLogin: 'Zaloguj',
      submitRegister: 'Zarejestruj i zaloguj',
      loggedIn: 'Zalogowano (JWT zapisany w localStorage)',
      logout: 'Wyloguj',
      tokenTitle: 'Generator tokena CI/CD',
      tokenPlaceholder: 'Nazwa tokena',
      generate: 'Generuj',
    },
    errors: {
      loadProducts: 'Błąd ładowania produktów',
      createProduct: 'Nie udało się dodać produktu',
      loadTestCases: 'Błąd ładowania scenariuszy',
      saveTestCase: 'Nie udało się zapisać scenariusza',
      loadTestPlans: 'Błąd ładowania planów',
      createTestPlan: 'Nie udało się utworzyć planu',
      loadTestRuns: 'Błąd ładowania TestRuns',
      createTestRun: 'Nie udało się utworzyć TestRun',
      updateStatus: 'Nie udało się zaktualizować statusu',
      authFailed: 'Błąd autentykacji',
      createToken: 'Nie udało się utworzyć tokena',
      uploadAttachment: 'Nie udało się wgrać załącznika',
    },
    attachments: {
      title: 'Załączniki',
      upload: 'Dodaj plik',
      empty: 'Brak załączników',
    },
    dashboard: {
      loading: 'Ładowanie...',
      products: 'Produkty',
      testCases: 'Scenariusze',
      testPlans: 'Plany testowe',
      testRuns: 'Uruchomienia',
      passFail: 'Pass / Fail',
      passed: 'Passed',
      failed: 'Failed',
      blocked: 'Blocked',
      skipped: 'Skipped',
    },
  },
  en: {
    nav: {
      dashboard: 'Dashboard',
      products: 'Products',
      testcases: 'TestCases',
      testplans: 'TestPlans',
      testruns: 'TestRuns',
      settings: 'Settings',
      auth: 'Account',
    },
    products: {
      title: 'Products',
      namePlaceholder: 'Product name',
      descPlaceholder: 'Description (optional)',
      add: 'Add product',
      empty: 'No products yet.',
      versions: 'Versions',
      versionPlaceholder: 'Version (e.g. 1.0.0)',
      addVersion: '+ Version',
      buildPlaceholder: 'Build (e.g. b100)',
      addBuild: '+ Build',
    },
    testcases: {
      title: 'Test Cases',
      summaryPlaceholder: 'Test case summary',
      stepsLabel: 'Steps:',
      actionPlaceholder: 'Action',
      expectedPlaceholder: 'Expected result',
      addStep: '+ Add step',
      save: 'Save test case',
      empty: 'No test cases yet.',
      stepsCount: 'steps',
      newCategoryPlaceholder: 'New type (e.g. UX)',
      addCategory: '+ Type',
    },
    testplans: {
      title: 'Test Plans',
      namePlaceholder: 'Test plan name',
      create: 'Create plan',
      empty: 'No test plans yet.',
      attachTitle: 'Attach test cases to plan',
      attachBtn: 'Attach selected',
    },
    testruns: {
      title: 'Test Runs',
      summaryPlaceholder: 'Run summary (e.g. Smoke v1.0)',
      create: 'Start TestRun',
      executions: 'Executions',
      emptyRuns: 'No test runs yet.',
      emptyExecutions: 'No executions for selected TestRun.',
    },
    auth: {
      title: 'Account & CI/CD Tokens',
      login: 'Login',
      register: 'Register',
      usernamePlaceholder: 'Username',
      passwordPlaceholder: 'Password (min. 8 chars)',
      submitLogin: 'Sign in',
      submitRegister: 'Register & sign in',
      loggedIn: 'Signed in (JWT stored in localStorage)',
      logout: 'Sign out',
      tokenTitle: 'CI/CD Token Generator',
      tokenPlaceholder: 'Token name',
      generate: 'Generate',
    },
    errors: {
      loadProducts: 'Failed to load products',
      createProduct: 'Failed to create product',
      loadTestCases: 'Failed to load test cases',
      saveTestCase: 'Failed to save test case',
      loadTestPlans: 'Failed to load test plans',
      createTestPlan: 'Failed to create test plan',
      loadTestRuns: 'Failed to load test runs',
      createTestRun: 'Failed to create test run',
      updateStatus: 'Failed to update status',
      authFailed: 'Authentication failed',
      createToken: 'Failed to create token',
      uploadAttachment: 'Failed to upload attachment',
    },
    attachments: {
      title: 'Attachments',
      upload: 'Upload file',
      empty: 'No attachments',
    },
    dashboard: {
      loading: 'Loading...',
      products: 'Products',
      testCases: 'TestCases',
      testPlans: 'TestPlans',
      testRuns: 'TestRuns',
      passFail: 'Pass / Fail',
      passed: 'Passed',
      failed: 'Failed',
      blocked: 'Blocked',
      skipped: 'Skipped',
    },
  },
} as const

type Dict = (typeof dict)['pl']

interface I18nContextValue {
  lang: Lang
  setLang: (lang: Lang) => void
  t: Dict
}

const I18nContext = createContext<I18nContextValue | null>(null)

export function I18nProvider({ children }: { children: ReactNode }) {
  const [lang, setLangState] = useState<Lang>(() => {
    const saved = localStorage.getItem(LANG_KEY)
    return saved === 'en' || saved === 'pl' ? saved : 'pl'
  })

  const setLang = (next: Lang) => {
    localStorage.setItem(LANG_KEY, next)
    setLangState(next)
  }

  return (
    <I18nContext.Provider value={{ lang, setLang, t: dict[lang] as Dict }}>
      {children}
    </I18nContext.Provider>
  )
}

export function useI18n() {
  const ctx = useContext(I18nContext)
  if (!ctx) throw new Error('useI18n must be used inside I18nProvider')
  return ctx
}
