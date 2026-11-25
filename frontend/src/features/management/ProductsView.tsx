import { useEffect, useState } from 'react'
import { api, Product } from '../../api/client'
import { useI18n } from '../../i18n'

export function ProductsView() {
  const { t } = useI18n()
  const [products, setProducts] = useState<Product[]>([])
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [error, setError] = useState<string | null>(null)

  const load = async () => {
    try {
      const res = await api.listProducts()
      setProducts(res.items)
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : t.errors.loadProducts)
    }
  }

  useEffect(() => {
    void load()
  }, [])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) return
    try {
      await api.createProduct({ name: name.trim(), description: description.trim() })
      setName('')
      setDescription('')
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errors.createProduct)
    }
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

      <ul style={{ listStyle: 'none', padding: 0, display: 'grid', gap: '0.5rem' }}>
        {products.map((p) => (
          <li key={p.id} style={{ padding: '0.75rem', border: '1px solid #e5e7eb', borderRadius: 8 }}>
            <strong>{p.name}</strong>
            {p.description && <p style={{ margin: '0.25rem 0 0', color: '#4b5563' }}>{p.description}</p>}
          </li>
        ))}
        {products.length === 0 && <li style={{ color: '#6b7280' }}>{t.products.empty}</li>}
      </ul>
    </section>
  )
}
