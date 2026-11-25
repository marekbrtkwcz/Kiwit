import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { api, getAuthToken, request, setAuthToken } from '../api/client'
import { dict } from './index'

function createMemoryStorage(): Storage {
  const store = new Map<string, string>()
  return {
    get length() {
      return store.size
    },
    clear: () => store.clear(),
    getItem: (key: string) => store.get(key) ?? null,
    key: (index: number) => Array.from(store.keys())[index] ?? null,
    removeItem: (key: string) => {
      store.delete(key)
    },
    setItem: (key: string, value: string) => {
      store.set(key, value)
    },
  }
}

function flattenKeys(obj: Record<string, unknown>, prefix = ''): Record<string, string> {
  const out: Record<string, string> = {}
  for (const [k, v] of Object.entries(obj)) {
    const full = prefix ? `${prefix}.${k}` : k
    if (v && typeof v === 'object' && !Array.isArray(v)) {
      Object.assign(out, flattenKeys(v as Record<string, unknown>, full))
    } else {
      out[full] = String(v ?? '')
    }
  }
  return out
}

describe('i18n dictionary parity', () => {
  it('has identical key sets for PL and EN', () => {
    const plKeys = Object.keys(flattenKeys(dict.pl)).sort()
    const enKeys = Object.keys(flattenKeys(dict.en)).sort()
    expect(plKeys).toEqual(enKeys)
    expect(plKeys.length).toBeGreaterThan(100)
  })

  it('has non-empty trimmed translations for every PL and EN key', () => {
    for (const [lang, tree] of Object.entries(dict)) {
      const flat = flattenKeys(tree as Record<string, unknown>)
      for (const [key, val] of Object.entries(flat)) {
        expect(val.trim().length, `${lang}.${key} should not be empty`).toBeGreaterThan(0)
      }
    }
  })

  it('translates key navigation and domain labels differently between PL and EN', () => {
    expect(dict.pl.nav.bugs).toBe('Błędy')
    expect(dict.en.nav.bugs).toBe('Bugs')
    expect(dict.pl.nav.settings).toBe('Ustawienia')
    expect(dict.en.nav.settings).toBe('Settings')
    expect(dict.pl.testcases.newCase).not.toEqual(dict.en.testcases.newCase)
    expect(dict.pl.bugs.report).not.toEqual(dict.en.bugs.report)
  })
})

describe('api client', () => {
  beforeEach(() => {
    vi.stubGlobal('localStorage', createMemoryStorage())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
  })

  it('stores and attaches Bearer token from localStorage', async () => {
    setAuthToken('jwt-test-token')
    expect(getAuthToken()).toBe('jwt-test-token')

    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ items: [] }), { status: 200, headers: { 'Content-Type': 'application/json' } }),
    )
    vi.stubGlobal('fetch', fetchMock)

    const res = await api.listProducts()
    expect(res.items).toEqual([])
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/products',
      expect.objectContaining({
        headers: expect.objectContaining({ Authorization: 'Bearer jwt-test-token' }),
      }),
    )
  })

  it('handles 204 No Content without failing JSON parsing', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }))
    vi.stubGlobal('fetch', fetchMock)

    await expect(api.deleteTestCase(42)).resolves.toBeUndefined()
    expect(fetchMock).toHaveBeenCalledWith('/api/v1/testcases/42', expect.objectContaining({ method: 'DELETE' }))
  })

  it('throws descriptive error on non-OK responses', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response('{"detail":"forbidden"}', { status: 403 }))
    vi.stubGlobal('fetch', fetchMock)

    await expect(request('/api/v1/products')).rejects.toThrow('{"detail":"forbidden"}')
  })
})
