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
    shell: {
      allProducts: 'Wszystkie produkty',
      productScope: 'Aktywny produkt',
      searchPlaceholder: 'Szukaj lub przejdź…',
      lightMode: 'Tryb jasny',
      darkMode: 'Tryb ciemny',
      collapseSidebar: 'Zwiń pasek boczny',
      expandSidebar: 'Rozwiń pasek boczny',
      cmdkPlaceholder: 'Szukaj scenariuszy, ekranów lub akcji…',
      cmdkNavigation: 'Nawigacja',
      cmdkTestCases: 'Scenariusze testowe',
      cmdkEmpty: 'Brak wyników.',
    },
    products: {
      title: 'Produkty',
      namePlaceholder: 'Nazwa produktu',
      descPlaceholder: 'Opis (opcjonalnie)',
      add: 'Dodaj produkt',
      empty: 'Brak produktów.',
      versions: 'Wersje i buildy',
      versionPlaceholder: 'Wersja (np. 1.0.0)',
      addVersion: 'Dodaj wersję',
      buildPlaceholder: 'Build (np. b100)',
      addBuild: 'Dodaj build',
    },
    testcases: {
      title: 'Scenariusze testowe',
      newCase: 'Nowy scenariusz',
      editCase: 'Edytuj scenariusz',
      summaryPlaceholder: 'Tytuł scenariusza',
      stepsLabel: 'Kroki testowe',
      actionPlaceholder: 'Akcja',
      expectedPlaceholder: 'Oczekiwany rezultat',
      addStep: 'Dodaj krok',
      save: 'Zapisz scenariusz',
      empty: 'Brak scenariuszy spełniających kryteria.',
      stepsCount: 'kroków',
      newCategoryPlaceholder: 'Nowa kategoria (np. UX)',
      addCategory: 'Dodaj kategorię',
      allPriorities: 'Wszystkie priorytety',
      allStatuses: 'Wszystkie statusy',
      allCategories: 'Wszystkie kategorie',
      allTypes: 'Manual + Auto',
      automatedOnly: 'Tylko automatyczne',
      manualOnly: 'Tylko manualne',
      applyTemplate: 'Wczytaj szablon…',
      revisions: 'Historia rewizji',
      copyLink: 'Kopiuj link',
      linkCopied: 'Skopiowano link do schowka',
    },
    testplans: {
      title: 'Plany testowe',
      namePlaceholder: 'Nazwa planu testowego',
      create: 'Utwórz plan',
      empty: 'Brak planów testowych.',
      attachTitle: 'Podepnij scenariusze do planu',
      attachBtn: 'Podepnij zaznaczone',
      attachedCases: 'Podpięte scenariusze',
      noCasesAttached: 'Brak podpiętych scenariuszy.',
    },
    testruns: {
      title: 'Uruchomienia testów',
      summaryPlaceholder: 'Nazwa uruchomienia (np. Smoke v1.0)',
      create: 'Uruchom TestRun',
      executions: 'Wykonania',
      emptyRuns: 'Brak uruchomień.',
      emptyExecutions: 'Brak wykonań dla wybranego uruchomienia.',
      shortcutsHint: 'Skróty: P Pass · F Fail · B Block · S Skip · J/K Następny/Poprzedni',
      allRuns: 'Wszystkie',
      runningRuns: 'W toku',
      stoppedRuns: 'Zakończone',
      stopRun: 'Zakończ',
      importJUnit: 'Import JUnit XML',
    },
    auth: {
      title: 'Konto, Tokeny CI/CD i Użytkownicy',
      login: 'Logowanie',
      register: 'Rejestracja',
      usernamePlaceholder: 'Nazwa użytkownika',
      passwordPlaceholder: 'Hasło (min. 8 znaków)',
      submitLogin: 'Zaloguj się',
      submitRegister: 'Zarejestruj i zaloguj',
      loggedIn: 'Sesja aktywna (JWT)',
      logout: 'Wyloguj',
      tokenTitle: 'Tokeny API CI/CD',
      tokenPlaceholder: 'Nazwa tokena (np. github-actions)',
      generate: 'Generuj token',
    },
    errors: {
      loadProducts: 'Błąd ładowania produktów',
      createProduct: 'Nie udało się dodać produktu',
      loadTestCases: 'Błąd ładowania scenariuszy',
      saveTestCase: 'Nie udało się zapisać scenariusza',
      loadTestPlans: 'Błąd ładowania planów',
      createTestPlan: 'Nie udało się utworzyć planu',
      loadTestRuns: 'Błąd ładowania uruchomień',
      createTestRun: 'Nie udało się utworzyć uruchomienia',
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
      loading: 'Ładowanie telemetrii…',
      products: 'Produkty',
      testCases: 'Scenariusze',
      testPlans: 'Plany testowe',
      testRuns: 'Uruchomienia',
      passFail: 'Wyniki wykonań',
      passed: 'Passed',
      failed: 'Failed',
      blocked: 'Blocked',
      skipped: 'Skipped',
      trends: 'Trendy dzienne wykonań',
      breakdown: 'Podział scenariuszy',
      automation: 'Automatyzacja',
      flaky: 'Kondycja scenariuszy (Flaky / Failing)',
      matrix: 'Macierz statusów',
      from: 'Od',
      to: 'Do',
      resetFilters: 'Wyczyść filtry',
      noData: 'Brak danych dla wybranych filtrów.',
    },
  },
  en: {
    nav: {
      dashboard: 'Dashboard',
      products: 'Products',
      testcases: 'Test Cases',
      testplans: 'Test Plans',
      testruns: 'Test Runs',
      settings: 'Settings',
      auth: 'Account',
    },
    shell: {
      allProducts: 'All products',
      productScope: 'Active product',
      searchPlaceholder: 'Search or jump…',
      lightMode: 'Light mode',
      darkMode: 'Dark mode',
      collapseSidebar: 'Collapse sidebar',
      expandSidebar: 'Expand sidebar',
      cmdkPlaceholder: 'Search test cases, pages or actions…',
      cmdkNavigation: 'Navigation',
      cmdkTestCases: 'Test Cases',
      cmdkEmpty: 'No results.',
    },
    products: {
      title: 'Products',
      namePlaceholder: 'Product name',
      descPlaceholder: 'Description (optional)',
      add: 'Add product',
      empty: 'No products yet.',
      versions: 'Versions & Builds',
      versionPlaceholder: 'Version (e.g. 1.0.0)',
      addVersion: 'Add version',
      buildPlaceholder: 'Build (e.g. b100)',
      addBuild: 'Add build',
    },
    testcases: {
      title: 'Test Cases',
      newCase: 'New test case',
      editCase: 'Edit test case',
      summaryPlaceholder: 'Test case summary',
      stepsLabel: 'Test steps',
      actionPlaceholder: 'Action',
      expectedPlaceholder: 'Expected result',
      addStep: 'Add step',
      save: 'Save test case',
      empty: 'No test cases match current filters.',
      stepsCount: 'steps',
      newCategoryPlaceholder: 'New category (e.g. UX)',
      addCategory: 'Add category',
      allPriorities: 'All priorities',
      allStatuses: 'All statuses',
      allCategories: 'All categories',
      allTypes: 'Manual + Auto',
      automatedOnly: 'Automated only',
      manualOnly: 'Manual only',
      applyTemplate: 'Apply template…',
      revisions: 'Revision history',
      copyLink: 'Copy link',
      linkCopied: 'Link copied to clipboard',
    },
    testplans: {
      title: 'Test Plans',
      namePlaceholder: 'Test plan name',
      create: 'Create plan',
      empty: 'No test plans yet.',
      attachTitle: 'Attach test cases to plan',
      attachBtn: 'Attach selected',
      attachedCases: 'Attached test cases',
      noCasesAttached: 'No test cases attached yet.',
    },
    testruns: {
      title: 'Test Runs',
      summaryPlaceholder: 'Run summary (e.g. Smoke v1.0)',
      create: 'Start TestRun',
      executions: 'Executions',
      emptyRuns: 'No test runs yet.',
      emptyExecutions: 'No executions for selected TestRun.',
      shortcutsHint: 'Shortcuts: P Pass · F Fail · B Block · S Skip · J/K Next/Prev',
      allRuns: 'All',
      runningRuns: 'Running',
      stoppedRuns: 'Stopped',
      stopRun: 'Stop run',
      importJUnit: 'Import JUnit XML',
    },
    auth: {
      title: 'Account, CI/CD Tokens & Users',
      login: 'Sign in',
      register: 'Register',
      usernamePlaceholder: 'Username',
      passwordPlaceholder: 'Password (min. 8 chars)',
      submitLogin: 'Sign in',
      submitRegister: 'Register & sign in',
      loggedIn: 'Session active (JWT)',
      logout: 'Sign out',
      tokenTitle: 'CI/CD API Tokens',
      tokenPlaceholder: 'Token name (e.g. github-actions)',
      generate: 'Generate token',
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
      loading: 'Loading telemetry…',
      products: 'Products',
      testCases: 'Test Cases',
      testPlans: 'Test Plans',
      testRuns: 'Test Runs',
      passFail: 'Execution Results',
      passed: 'Passed',
      failed: 'Failed',
      blocked: 'Blocked',
      skipped: 'Skipped',
      trends: 'Daily Execution Trends',
      breakdown: 'Test Case Breakdown',
      automation: 'Automation',
      flaky: 'Test Case Health (Flaky / Failing)',
      matrix: 'Status Matrix',
      from: 'From',
      to: 'To',
      resetFilters: 'Reset filters',
      noData: 'No data for selected filters.',
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
