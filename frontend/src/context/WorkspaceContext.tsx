import { createContext, ReactNode, useCallback, useContext, useEffect, useState } from 'react'
import { api, getAuthToken, Product } from '../api/client'

export type Theme = 'light' | 'dark'

export interface ToastMessage {
  id: number
  kind: 'ok' | 'error'
  text: string
}

interface WorkspaceContextValue {
  theme: Theme
  toggleTheme: () => void
  products: Product[]
  activeProductId: number | null // null = All products
  setActiveProductId: (id: number | null) => void
  refreshProducts: () => Promise<void>
  toasts: ToastMessage[]
  notify: (text: string, kind?: 'ok' | 'error') => void
  cmdkOpen: boolean
  setCmdkOpen: (open: boolean) => void
}

const THEME_KEY = 'kiwit_theme'
const PRODUCT_KEY = 'kiwit_active_product'

const WorkspaceContext = createContext<WorkspaceContextValue | null>(null)

export function WorkspaceProvider({ children }: { children: ReactNode }) {
  const [theme, setTheme] = useState<Theme>(() => {
    const saved = localStorage.getItem(THEME_KEY)
    if (saved === 'dark' || saved === 'light') return saved
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
  })

  const [products, setProducts] = useState<Product[]>([])
  const [activeProductId, setActiveProductIdState] = useState<number | null>(() => {
    const saved = localStorage.getItem(PRODUCT_KEY)
    const parsed = saved ? Number(saved) : NaN
    return Number.isFinite(parsed) && parsed > 0 ? parsed : null
  })
  const [toasts, setToasts] = useState<ToastMessage[]>([])
  const [cmdkOpen, setCmdkOpen] = useState(false)

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme)
    localStorage.setItem(THEME_KEY, theme)
  }, [theme])

  const toggleTheme = useCallback(() => {
    setTheme((prev) => (prev === 'dark' ? 'light' : 'dark'))
  }, [])

  const setActiveProductId = useCallback((id: number | null) => {
    setActiveProductIdState(id)
    if (id && id > 0) localStorage.setItem(PRODUCT_KEY, String(id))
    else localStorage.removeItem(PRODUCT_KEY)
  }, [])

  const refreshProducts = useCallback(async () => {
    if (!getAuthToken()) {
      setProducts([])
      return
    }
    try {
      const res = await api.listProducts()
      setProducts(res.items)
      if (activeProductId && !res.items.some((p) => p.id === activeProductId)) {
        setActiveProductId(null)
      }
    } catch {
      // Ignore if token expired
    }
  }, [activeProductId, setActiveProductId])

  useEffect(() => {
    void refreshProducts()
  }, [refreshProducts])

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        setCmdkOpen((v) => !v)
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [])

  const notify = useCallback((text: string, kind: 'ok' | 'error' = 'ok') => {
    const id = Date.now() + Math.floor(Math.random() * 1000)
    setToasts((prev) => [...prev.slice(-3), { id, kind, text }])
    setTimeout(() => {
      setToasts((prev) => prev.filter((t) => t.id !== id))
    }, 3600)
  }, [])

  return (
    <WorkspaceContext.Provider
      value={{
        theme,
        toggleTheme,
        products,
        activeProductId,
        setActiveProductId,
        refreshProducts,
        toasts,
        notify,
        cmdkOpen,
        setCmdkOpen,
      }}
    >
      {children}
    </WorkspaceContext.Provider>
  )
}

export function useWorkspace() {
  const ctx = useContext(WorkspaceContext)
  if (!ctx) throw new Error('useWorkspace must be used inside WorkspaceProvider')
  return ctx
}
