import { useEffect, useState } from 'react'
import { api, Build, ComponentItem, Product, User, Version } from '../../api/client'
import { useI18n } from '../../i18n'

export function ProductsView() {
  const { t } = useI18n()
  const [products, setProducts] = useState<Product[]>([])
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const [versions, setVersions] = useState<Version[]>([])
  const [builds, setBuilds] = useState<Record<number, Build[]>>({})
  const [components, setComponents] = useState<ComponentItem[]>([])
  const [users, setUsers] = useState<User[]>([])
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [newVersion, setNewVersion] = useState('')
  const [newBuild, setNewBuild] = useState<Record<number, string>>({})
  const [newCompName, setNewCompName] = useState('')
  const [newCompDesc, setNewCompDesc] = useState('')
  const [memberUserId, setMemberUserId] = useState<number>(0)
  const [memberRole, setMemberRole] = useState<'viewer' | 'tester' | 'maintainer'>('tester')
  const [newTag, setNewTag] = useState('')
  const [tags, setTags] = useState<{ id: number; name: string }[]>([])
  const [notice, setNotice] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  const load = async () => {
    try {
      const [res, tRes, uRes] = await Promise.all([
        api.listProducts(),
        api.listTags(),
        api.listUsers().catch(() => ({ items: [] as User[] })),
      ])
      setProducts(res.items)
      setTags(tRes.items)
      setUsers(uRes.items)
      if (!memberUserId && uRes.items.length > 0) setMemberUserId(uRes.items[0].id)
      if (res.items.length > 0 && selectedId === null) {
        setSelectedId(res.items[0].id)
      }
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : t.errors.loadProducts)
    }
  }

  const loadProductDetails = async (pid: number) => {
    try {
      const [vRes, cRes] = await Promise.all([api.listVersions(pid), api.listComponents(pid)])
      setVersions(vRes.items)
      setComponents(cRes.items)
      const map: Record<number, Build[]> = {}
      await Promise.all(
        vRes.items.map(async (v) => {
          const b = await api.listBuilds(v.id)
          map[v.id] = b.items
        }),
      )
      setBuilds(map)
    } catch (e) {
      setError(e instanceof Error ? e.message : t.errors.loadProducts)
    }
  }

  useEffect(() => {
    void load()
  }, [])

  useEffect(() => {
    if (selectedId === null) return
    void loadProductDetails(selectedId)
  }, [selectedId])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) return
    try {
      const created = await api.createProduct({ name: name.trim(), description: description.trim() })
      setName('')
      setDescription('')
      setSelectedId(created.id)
      setNotice(null)
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errors.createProduct)
    }
  }

  const handleAddVersion = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!selectedId || !newVersion.trim()) return
    try {
      await api.createVersion(selectedId, newVersion.trim())
      setNewVersion('')
      await loadProductDetails(selectedId)
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errors.createProduct)
    }
  }

  const handleAddBuild = async (versionId: number) => {
    const val = (newBuild[versionId] ?? '').trim()
    if (!val) return
    try {
      await api.createBuild(versionId, val)
      setNewBuild((prev) => ({ ...prev, [versionId]: '' }))
      const b = await api.listBuilds(versionId)
      setBuilds((prev) => ({ ...prev, [versionId]: b.items }))
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errors.createProduct)
    }
  }

  const handleAddComponent = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!selectedId || !newCompName.trim()) return
    try {
      await api.createComponent(selectedId, newCompName.trim(), newCompDesc.trim())
      setNewCompName('')
      setNewCompDesc('')
      await loadProductDetails(selectedId)
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errors.createProduct)
    }
  }

  const handleAddMember = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!selectedId || !memberUserId) return
    try {
      await api.addProductMember(selectedId, { user_id: memberUserId, role: memberRole })
      setNotice(`Assigned user #${memberUserId} (${memberRole}) to Product #${selectedId}.`)
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errors.createProduct)
    }
  }

  return (
    <section>
      <h2>{t.products.title}</h2>
      {error && <p className="alert-error">{error}</p>}
      {notice && <p className="alert-ok">{notice}</p>}

      <form onSubmit={handleSubmit} style={{ display: 'flex', gap: '0.5rem', marginBottom: '1rem', flexWrap: 'wrap' }}>
        <input className="input" placeholder={t.products.namePlaceholder} value={name} onChange={(e) => setName(e.target.value)} />
        <input className="input" placeholder={t.products.descPlaceholder} value={description} onChange={(e) => setDescription(e.target.value)} style={{ flex: 1, minWidth: 200 }} />
        <button type="submit" className="btn btn-primary">{t.products.add}</button>
      </form>

      <ul style={{ listStyle: 'none', padding: 0, display: 'grid', gap: '0.5rem', marginBottom: '1.25rem' }}>
        {products.map((p) => (
          <li
            key={p.id}
            className="card"
            onClick={() => setSelectedId(p.id)}
            style={{
              border: selectedId === p.id ? '2px solid var(--primary)' : undefined,
              cursor: 'pointer',
            }}
          >
            <strong>{p.name}</strong> <span style={{ color: 'var(--muted)', fontSize: '0.8rem' }}>#{p.id}</span>
            {p.description && <p style={{ margin: '0.25rem 0 0', color: 'var(--muted)' }}>{p.description}</p>}
          </li>
        ))}
        {products.length === 0 && <li style={{ color: 'var(--muted)' }}>{t.products.empty}</li>}
      </ul>

      {selectedId !== null && (
        <div className="card" style={{ display: 'grid', gap: '1.1rem' }}>
          <div>
            <h3 style={{ marginTop: 0 }}>{t.products.versions}</h3>
            <form onSubmit={handleAddVersion} style={{ display: 'flex', gap: '0.5rem', marginBottom: '0.75rem' }}>
              <input className="input" placeholder={t.products.versionPlaceholder} value={newVersion} onChange={(e) => setNewVersion(e.target.value)} />
              <button type="submit" className="btn btn-sm">{t.products.addVersion}</button>
            </form>
            <div style={{ display: 'grid', gap: '0.5rem' }}>
              {versions.map((v) => (
                <div key={v.id} style={{ padding: '0.5rem', background: '#f8fafc', borderRadius: 6, border: '1px solid var(--border)' }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: '0.5rem', flexWrap: 'wrap' }}>
                    <strong>{v.value}</strong>
                    <div style={{ display: 'flex', gap: '0.35rem' }}>
                      <input
                        className="input"
                        placeholder={t.products.buildPlaceholder}
                        value={newBuild[v.id] ?? ''}
                        onChange={(e) => setNewBuild((prev) => ({ ...prev, [v.id]: e.target.value }))}
                        style={{ padding: '0.25rem 0.5rem', fontSize: '0.82rem' }}
                      />
                      <button type="button" className="btn btn-sm" onClick={() => void handleAddBuild(v.id)}>
                        {t.products.addBuild}
                      </button>
                    </div>
                  </div>
                  {(builds[v.id] ?? []).length > 0 && (
                    <div style={{ marginTop: '0.35rem', fontSize: '0.8rem', color: 'var(--muted)' }}>
                      Builds: {(builds[v.id] ?? []).map((b) => `${b.name} (#${b.id})`).join(', ')}
                    </div>
                  )}
                </div>
              ))}
            </div>
          </div>

          <div style={{ borderTop: '1px solid var(--border)', paddingTop: '0.9rem' }}>
            <h4 style={{ margin: '0 0 0.5rem' }}>Components ({components.length})</h4>
            <form onSubmit={handleAddComponent} style={{ display: 'flex', gap: '0.4rem', marginBottom: '0.5rem', flexWrap: 'wrap' }}>
              <input className="input" placeholder="Component name (e.g. Auth API)" value={newCompName} onChange={(e) => setNewCompName(e.target.value)} required />
              <input className="input" placeholder="Description (optional)" value={newCompDesc} onChange={(e) => setNewCompDesc(e.target.value)} style={{ flex: 1, minWidth: 180 }} />
              <button type="submit" className="btn btn-sm">+ Component</button>
            </form>
            <div style={{ display: 'flex', gap: '0.35rem', flexWrap: 'wrap' }}>
              {components.map((c) => (
                <span key={c.id} className="badge" title={c.description}>{c.name}</span>
              ))}
              {components.length === 0 && <span style={{ fontSize: '0.82rem', color: 'var(--muted)' }}>No components yet.</span>}
            </div>
          </div>

          {users.length > 0 && (
            <div style={{ borderTop: '1px solid var(--border)', paddingTop: '0.9rem' }}>
              <h4 style={{ margin: '0 0 0.5rem' }}>Product Access (Admin)</h4>
              <form onSubmit={handleAddMember} style={{ display: 'flex', gap: '0.4rem', flexWrap: 'wrap', alignItems: 'center' }}>
                <select className="select" value={memberUserId} onChange={(e) => setMemberUserId(Number(e.target.value))}>
                  {users.map((u) => (
                    <option key={u.id} value={u.id}>{u.username} ({u.email})</option>
                  ))}
                </select>
                <select className="select" value={memberRole} onChange={(e) => setMemberRole(e.target.value as 'viewer' | 'tester' | 'maintainer')}>
                  <option value="viewer">viewer</option>
                  <option value="tester">tester</option>
                  <option value="maintainer">maintainer</option>
                </select>
                <button type="submit" className="btn btn-primary btn-sm">Assign member</button>
              </form>
            </div>
          )}

          <div style={{ borderTop: '1px solid var(--border)', paddingTop: '0.9rem' }}>
            <strong>Tags:</strong>{' '}
            <span style={{ fontSize: '0.85rem', color: 'var(--muted)' }}>{tags.map((tg) => tg.name).join(', ') || '-'}</span>
            <div style={{ display: 'flex', gap: '0.35rem', marginTop: '0.4rem' }}>
              <input className="input" placeholder="Tag" value={newTag} onChange={(e) => setNewTag(e.target.value)} style={{ padding: '0.25rem 0.5rem', fontSize: '0.85rem' }} />
              <button
                type="button"
                className="btn btn-sm"
                onClick={async () => {
                  if (!newTag.trim()) return
                  try {
                    await api.createTag(newTag.trim())
                    setNewTag('')
                    await load()
                  } catch (err) {
                    setError(err instanceof Error ? err.message : t.errors.createProduct)
                  }
                }}
              >
                + Tag
              </button>
            </div>
          </div>
        </div>
      )}
    </section>
  )
}
