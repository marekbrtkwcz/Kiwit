import { useEffect, useState } from 'react'
import { Boxes, GitBranch, Package, Plus, Shield, Tag } from 'lucide-react'
import { api, Build, ComponentItem, Product, User, Version } from '../../api/client'
import { useWorkspace } from '../../context/WorkspaceContext'
import { useI18n } from '../../i18n'

export function ProductsView() {
  const { t } = useI18n()
  const { refreshProducts, notify } = useWorkspace()
  const [products, setProducts] = useState<Product[]>([])
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const [tab, setTab] = useState<'versions' | 'components' | 'members' | 'tags'>('versions')
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
      if (res.items.length > 0 && selectedId === null) setSelectedId(res.items[0].id)
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
    if (selectedId !== null) void loadProductDetails(selectedId)
  }, [selectedId])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) return
    try {
      const created = await api.createProduct({ name: name.trim(), description: description.trim() })
      setName('')
      setDescription('')
      setSelectedId(created.id)
      notify(`Product "${created.name}" created`)
      await Promise.all([load(), refreshProducts()])
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errors.createProduct)
    }
  }

  const selectedProduct = products.find((p) => p.id === selectedId) ?? null

  return (
    <section style={{ display: 'grid', gap: '1rem' }}>
      {error && <p className="alert-error">{error}</p>}

      <form onSubmit={handleSubmit} className="filter-bar" style={{ marginBottom: 0 }}>
        <input className="input" placeholder={t.products.namePlaceholder} value={name} onChange={(e) => setName(e.target.value)} required />
        <input className="input" placeholder={t.products.descPlaceholder} value={description} onChange={(e) => setDescription(e.target.value)} style={{ flex: 1, minWidth: 200 }} />
        <button type="submit" className="btn btn-primary">
          <Plus size={15} /> {t.products.add}
        </button>
      </form>

      <div className="split-layout">
        <div className="card" style={{ padding: '0.75rem', display: 'grid', gap: '0.45rem' }}>
          {products.map((p) => (
            <div
              key={p.id}
              onClick={() => setSelectedId(p.id)}
              style={{
                padding: '0.65rem 0.75rem',
                borderRadius: 8,
                border: selectedId === p.id ? '1px solid var(--primary)' : '1px solid var(--border)',
                background: selectedId === p.id ? 'var(--primary-soft)' : 'var(--surface)',
                cursor: 'pointer',
              }}
            >
              <div style={{ fontWeight: 600, display: 'flex', alignItems: 'center', gap: 6 }}>
                <Package size={15} /> {p.name}
                <span style={{ fontFamily: 'var(--font-mono)', fontSize: '0.74rem', color: 'var(--muted)' }}>#{p.id}</span>
              </div>
              {p.description && <div style={{ fontSize: '0.78rem', color: 'var(--muted)', marginTop: 2 }}>{p.description}</div>}
            </div>
          ))}
          {products.length === 0 && <p style={{ color: 'var(--muted)', margin: '0.5rem' }}>{t.products.empty}</p>}
        </div>

        {selectedProduct && (
          <div className="card">
            <h3 style={{ margin: '0 0 0.75rem' }}>{selectedProduct.name}</h3>
            <div className="tabs-bar">
              <button type="button" className={`tab-btn ${tab === 'versions' ? 'active' : ''}`} onClick={() => setTab('versions')}>
                <GitBranch size={14} /> {t.products.versions} ({versions.length})
              </button>
              <button type="button" className={`tab-btn ${tab === 'components' ? 'active' : ''}`} onClick={() => setTab('components')}>
                <Boxes size={14} /> Components ({components.length})
              </button>
              {users.length > 0 && (
                <button type="button" className={`tab-btn ${tab === 'members' ? 'active' : ''}`} onClick={() => setTab('members')}>
                  <Shield size={14} /> Product Access
                </button>
              )}
              <button type="button" className={`tab-btn ${tab === 'tags' ? 'active' : ''}`} onClick={() => setTab('tags')}>
                <Tag size={14} /> Tags ({tags.length})
              </button>
            </div>

            {tab === 'versions' && (
              <div style={{ display: 'grid', gap: '0.75rem' }}>
                <form
                  onSubmit={async (e) => {
                    e.preventDefault()
                    if (!newVersion.trim()) return
                    try {
                      await api.createVersion(selectedProduct.id, newVersion.trim())
                      setNewVersion('')
                      notify('Version added')
                      await loadProductDetails(selectedProduct.id)
                    } catch (err) {
                      setError(err instanceof Error ? err.message : t.errors.createProduct)
                    }
                  }}
                  style={{ display: 'flex', gap: '0.45rem' }}
                >
                  <input className="input" placeholder={t.products.versionPlaceholder} value={newVersion} onChange={(e) => setNewVersion(e.target.value)} />
                  <button type="submit" className="btn btn-primary btn-sm">{t.products.addVersion}</button>
                </form>

                {versions.map((v) => (
                  <div key={v.id} style={{ padding: '0.7rem', background: 'var(--bg-subtle)', borderRadius: 8, border: '1px solid var(--border)' }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: '0.5rem', flexWrap: 'wrap' }}>
                      <strong>{v.value}</strong>
                      <div style={{ display: 'flex', gap: '0.35rem' }}>
                        <input
                          className="input"
                          placeholder={t.products.buildPlaceholder}
                          value={newBuild[v.id] ?? ''}
                          onChange={(e) => setNewBuild((prev) => ({ ...prev, [v.id]: e.target.value }))}
                          style={{ padding: '0.28rem 0.55rem', fontSize: '0.8rem' }}
                        />
                        <button
                          type="button"
                          className="btn btn-sm"
                          onClick={async () => {
                            const val = (newBuild[v.id] ?? '').trim()
                            if (!val) return
                            await api.createBuild(v.id, val)
                            setNewBuild((prev) => ({ ...prev, [v.id]: '' }))
                            const b = await api.listBuilds(v.id)
                            setBuilds((prev) => ({ ...prev, [v.id]: b.items }))
                            notify(`Build "${val}" added`)
                          }}
                        >
                          {t.products.addBuild}
                        </button>
                      </div>
                    </div>
                    <div style={{ display: 'flex', gap: '0.35rem', flexWrap: 'wrap', marginTop: '0.45rem' }}>
                      {(builds[v.id] ?? []).map((b) => (
                        <span key={b.id} className="badge">{b.name} (#{b.id})</span>
                      ))}
                      {(builds[v.id] ?? []).length === 0 && <span style={{ fontSize: '0.78rem', color: 'var(--muted)' }}>No builds yet</span>}
                    </div>
                  </div>
                ))}
              </div>
            )}

            {tab === 'components' && (
              <div style={{ display: 'grid', gap: '0.75rem' }}>
                <form
                  onSubmit={async (e) => {
                    e.preventDefault()
                    if (!newCompName.trim()) return
                    await api.createComponent(selectedProduct.id, newCompName.trim(), newCompDesc.trim())
                    setNewCompName('')
                    setNewCompDesc('')
                    notify('Component added')
                    await loadProductDetails(selectedProduct.id)
                  }}
                  style={{ display: 'flex', gap: '0.45rem', flexWrap: 'wrap' }}
                >
                  <input className="input" placeholder="Component name (e.g. Auth API)" value={newCompName} onChange={(e) => setNewCompName(e.target.value)} required />
                  <input className="input" placeholder="Description (optional)" value={newCompDesc} onChange={(e) => setNewCompDesc(e.target.value)} style={{ flex: 1, minWidth: 180 }} />
                  <button type="submit" className="btn btn-primary btn-sm">+ Component</button>
                </form>
                <div style={{ display: 'flex', gap: '0.4rem', flexWrap: 'wrap' }}>
                  {components.map((c) => (
                    <span key={c.id} className="badge" title={c.description}>{c.name}</span>
                  ))}
                  {components.length === 0 && <span style={{ color: 'var(--muted)', fontSize: '0.84rem' }}>No components yet.</span>}
                </div>
              </div>
            )}

            {tab === 'members' && users.length > 0 && (
              <form
                onSubmit={async (e) => {
                  e.preventDefault()
                  if (!memberUserId) return
                  await api.addProductMember(selectedProduct.id, { user_id: memberUserId, role: memberRole })
                  notify(`Assigned user #${memberUserId} (${memberRole})`)
                }}
                style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap', alignItems: 'center' }}
              >
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
            )}

            {tab === 'tags' && (
              <div style={{ display: 'grid', gap: '0.75rem' }}>
                <div style={{ display: 'flex', gap: '0.45rem' }}>
                  <input className="input" placeholder="New tag" value={newTag} onChange={(e) => setNewTag(e.target.value)} />
                  <button
                    type="button"
                    className="btn btn-primary btn-sm"
                    onClick={async () => {
                      if (!newTag.trim()) return
                      await api.createTag(newTag.trim())
                      setNewTag('')
                      notify('Tag created')
                      await load()
                    }}
                  >
                    + Tag
                  </button>
                </div>
                <div style={{ display: 'flex', gap: '0.35rem', flexWrap: 'wrap' }}>
                  {tags.map((tg) => (
                    <span key={tg.id} className="badge">{tg.name}</span>
                  ))}
                </div>
              </div>
            )}
          </div>
        )}
      </div>
    </section>
  )
}
