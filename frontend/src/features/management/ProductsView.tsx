import { useEffect, useState } from 'react'
import { api, Build, Product, Version } from '../../api/client'
import { useI18n } from '../../i18n'

export function ProductsView() {
  const { t } = useI18n()
  const [products, setProducts] = useState<Product[]>([])
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const [versions, setVersions] = useState<Version[]>([])
  const [builds, setBuilds] = useState<Record<number, Build[]>>({})
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [newVersion, setNewVersion] = useState('')
  const [newBuild, setNewBuild] = useState<Record<number, string>>({})
  const [error, setError] = useState<string | null>(null)

  const load = async () => {
    try {
      const res = await api.listProducts()
      setProducts(res.items)
      if (res.items.length > 0 && selectedId === null) {
        setSelectedId(res.items[0].id)
      }
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : t.errors.loadProducts)
    }
  }

  useEffect(() => {
    void load()
  }, [])

  useEffect(() => {
    if (selectedId === null) return
    void api.listVersions(selectedId).then(async (res) => {
      setVersions(res.items)
      const map: Record<number, Build[]> = {}
      await Promise.all(
        res.items.map(async (v) => {
          const b = await api.listBuilds(v.id)
          map[v.id] = b.items
        }),
      )
      setBuilds(map)
    }).catch(() => {})
  }, [selectedId])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) return
    try {
      const created = await api.createProduct({ name: name.trim(), description: description.trim() })
      setName('')
      setDescription('')
      setSelectedId(created.id)
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errors.createProduct)
    }
  }

  const handleAddVersion = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!selectedId || !newVersion.trim()) return
    await api.createVersion(selectedId, newVersion.trim())
    setNewVersion('')
    const res = await api.listVersions(selectedId)
    setVersions(res.items)
  }

  const handleAddBuild = async (versionId: number) => {
    const val = (newBuild[versionId] ?? '').trim()
    if (!val) return
    await api.createBuild(versionId, val)
    setNewBuild((prev) => ({ ...prev, [versionId]: '' }))
    const b = await api.listBuilds(versionId)
    setBuilds((prev) => ({ ...prev, [versionId]: b.items }))
  }

  return (
    <section>
      <h2>{t.products.title}</h2>
      {error && <p style={{ color: '#b91c1c' }}>{error}</p>}

      <form onSubmit={handleSubmit} style={{ display: 'flex', gap: '0.5rem', marginBottom: '1rem' }}>
        <input
          placeholder={t.products.namePlaceholder}
          value={name}
          onChange={(e) => setName(e.target.value)}
          style={{ padding: '0.4rem 0.6rem', borderRadius: 6, border: '1px solid #d1d5db' }}
        />
        <input
          placeholder={t.products.descPlaceholder}
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          style={{ padding: '0.4rem 0.6rem', borderRadius: 6, border: '1px solid #d1d5db', flex: 1 }}
        />
        <button type="submit" style={{ padding: '0.4rem 0.9rem', borderRadius: 6, background: '#111827', color: '#fff', border: 'none', cursor: 'pointer' }}>
          {t.products.add}
        </button>
      </form>

      <ul style={{ listStyle: 'none', padding: 0, display: 'grid', gap: '0.5rem', marginBottom: '1.5rem' }}>
        {products.map((p) => (
          <li
            key={p.id}
            onClick={() => setSelectedId(p.id)}
            style={{
              padding: '0.75rem',
              border: selectedId === p.id ? '2px solid #111827' : '1px solid #e5e7eb',
              borderRadius: 8,
              cursor: 'pointer',
            }}
          >
            <strong>{p.name}</strong>
            {p.description && <p style={{ margin: '0.25rem 0 0', color: '#4b5563' }}>{p.description}</p>}
          </li>
        ))}
        {products.length === 0 && <li style={{ color: '#6b7280' }}>{t.products.empty}</li>}
      </ul>

      {selectedId !== null && (
        <div style={{ border: '1px solid #e5e7eb', borderRadius: 8, padding: '1rem' }}>
          <h3 style={{ marginTop: 0 }}>{t.products.versions}</h3>
          <form onSubmit={handleAddVersion} style={{ display: 'flex', gap: '0.5rem', marginBottom: '0.75rem' }}>
            <input
              placeholder={t.products.versionPlaceholder}
              value={newVersion}
              onChange={(e) => setNewVersion(e.target.value)}
              style={{ padding: '0.35rem 0.6rem', borderRadius: 6, border: '1px solid #d1d5db' }}
            />
            <button type="submit" style={{ padding: '0.35rem 0.8rem', borderRadius: 6, border: '1px solid #d1d5db', background: '#fff', cursor: 'pointer' }}>
              {t.products.addVersion}
            </button>
          </form>
          <div style={{ display: 'grid', gap: '0.5rem' }}>
            {versions.map((v) => (
              <div key={v.id} style={{ padding: '0.5rem', background: '#f9fafb', borderRadius: 6 }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <strong>{v.value}</strong>
                  <div style={{ display: 'flex', gap: '0.35rem' }}>
                    <input
                      placeholder={t.products.buildPlaceholder}
                      value={newBuild[v.id] ?? ''}
                      onChange={(e) => setNewBuild((prev) => ({ ...prev, [v.id]: e.target.value }))}
                      style={{ padding: '0.25rem 0.5rem', borderRadius: 4, border: '1px solid #d1d5db', fontSize: '0.85rem' }}
                    />
                    <button type="button" onClick={() => void handleAddBuild(v.id)} style={{ padding: '0.25rem 0.6rem', borderRadius: 4, border: '1px solid #d1d5db', background: '#fff', cursor: 'pointer', fontSize: '0.85rem' }}>
                      {t.products.addBuild}
                    </button>
                  </div>
                </div>
                {(builds[v.id] ?? []).length > 0 && (
                  <div style={{ marginTop: '0.35rem', fontSize: '0.8rem', color: '#4b5563' }}>
                    Builds: {(builds[v.id] ?? []).map((b) => `${b.name} (#${b.id})`).join(', ')}
                  </div>
                )}
              </div>
            ))}
          </div>
        </div>
      )}
    </section>
  )
}
