import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { api, getAuthToken, request, setAuthToken } from '../api/client'

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
