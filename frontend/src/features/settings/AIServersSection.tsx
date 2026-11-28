import { Bot, Plus, Trash2 } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
import { type AIServer, api } from '../../api/client'
import { useWorkspace } from '../../context/WorkspaceContext'
import { useI18n } from '../../i18n'

const PROVIDERS = ['openai', 'anthropic', 'mistral', 'azure', 'vertex', 'custom']

// AIServersSection manages configured AI servers (admin, SquashTM-style).
export function AIServersSection() {
  const { notify } = useWorkspace()
  const { t } = useI18n()
  const [servers, setServers] = useState<AIServer[]>([])
  const [name, setName] = useState('')
  const [provider, setProvider] = useState('openai')
  const [baseUrl, setBaseUrl] = useState('https://api.openai.com/v1')
  const [model, setModel] = useState('gpt-4o-mini')
  const [apiKey, setApiKey] = useState('')
  const [temperature, setTemperature] = useState(0.3)
  const [maxTokens, setMaxTokens] = useState(2000)
  const [isDefault, setIsDefault] = useState(false)

  const load = useCallback(async () => {
    try {
      const res = await api.listAIServers()
      setServers(res.items ?? [])
    } catch {
      // ignore
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  const add = async () => {
    if (!name.trim() || !baseUrl.trim() || !model.trim()) return
    try {
      await api.createAIServer({
        name: name.trim(),
        provider,
        base_url: baseUrl.trim(),
        model: model.trim(),
        api_key: apiKey,
        temperature,
        max_tokens: maxTokens,
        is_default: isDefault,
      })
      notify(t.aiServers.added)
      setName('')
      setApiKey('')
      await load()
    } catch (err) {
      notify(err instanceof Error ? err.message : t.aiServers.failedAdd, 'error')
    }
  }

  const remove = async (id: number) => {
    try {
      await api.deleteAIServer(id)
      notify(t.aiServers.deleted)
      await load()
    } catch {
      notify(t.aiServers.failedDelete, 'error')
    }
  }

  return (
    <div className="card">
      <div className="card-header">
        <h3 className="card-title">
          <Bot size={15} /> {t.aiServers.title}
        </h3>
      </div>
      <div style={{ display: 'grid', gap: '0.5rem' }}>
        {servers.length === 0 && (
          <p style={{ color: 'var(--muted)', fontSize: '0.85rem', margin: 0 }}>{t.aiServers.none}</p>
        )}
        {servers.map((s) => (
          <div key={s.id} style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', fontSize: '0.85rem' }}>
            <strong>{s.name}</strong>
            {s.is_default && <span className="badge pill-passed">{t.aiServers.default}</span>}
            <span className="badge">{s.provider}</span>
            <code style={{ color: 'var(--muted)', fontSize: '0.75rem' }}>
              {s.base_url} · {s.model}
            </code>
            <button
              type="button"
              className="btn btn-sm btn-danger"
              style={{ marginLeft: 'auto' }}
              onClick={() => remove(s.id)}
              aria-label={`${t.aiServers.deleteAria} ${s.name}`}
            >
              <Trash2 size={13} />
            </button>
          </div>
        ))}
        <div style={{ display: 'grid', gap: '0.4rem', borderTop: '1px solid var(--border)', paddingTop: '0.6rem' }}>
          <div style={{ display: 'flex', gap: '0.4rem', flexWrap: 'wrap' }}>
            <input
              className="input"
              placeholder={t.aiServers.name}
              value={name}
              onChange={(e) => setName(e.target.value)}
              style={{ width: 130, fontSize: '0.8rem' }}
            />
            <select
              className="select"
              value={provider}
              onChange={(e) => setProvider(e.target.value)}
              style={{ fontSize: '0.8rem' }}
            >
              {PROVIDERS.map((p) => (
                <option key={p} value={p}>
                  {p}
                </option>
              ))}
            </select>
            <input
              className="input"
              placeholder={t.aiServers.baseUrl}
              value={baseUrl}
              onChange={(e) => setBaseUrl(e.target.value)}
              style={{ width: 240, fontSize: '0.8rem', fontFamily: 'var(--font-mono)' }}
            />
            <input
              className="input"
              placeholder={t.aiServers.model}
              value={model}
              onChange={(e) => setModel(e.target.value)}
              style={{ width: 150, fontSize: '0.8rem' }}
            />
          </div>
          <div style={{ display: 'flex', gap: '0.4rem', flexWrap: 'wrap', alignItems: 'center' }}>
            <input
              className="input"
              type="password"
              placeholder={t.aiServers.apiKey}
              value={apiKey}
              onChange={(e) => setApiKey(e.target.value)}
              style={{ width: 220, fontSize: '0.8rem' }}
            />
            <label style={{ fontSize: '0.8rem', display: 'inline-flex', gap: '0.25rem', alignItems: 'center' }}>
              {t.aiServers.temp}{' '}
              <input
                className="input"
                type="number"
                step={0.1}
                min={0}
                max={2}
                value={temperature}
                onChange={(e) => setTemperature(Number(e.target.value))}
                style={{ width: 60, fontSize: '0.8rem' }}
              />
            </label>
            <label style={{ fontSize: '0.8rem', display: 'inline-flex', gap: '0.25rem', alignItems: 'center' }}>
              {t.aiServers.maxTokens}{' '}
              <input
                className="input"
                type="number"
                value={maxTokens}
                onChange={(e) => setMaxTokens(Number(e.target.value))}
                style={{ width: 70, fontSize: '0.8rem' }}
              />
            </label>
            <label style={{ fontSize: '0.8rem', display: 'inline-flex', gap: '0.25rem', alignItems: 'center' }}>
              <input type="checkbox" checked={isDefault} onChange={(e) => setIsDefault(e.target.checked)} />{' '}
              {t.aiServers.default}
            </label>
            <button
              type="button"
              className="btn btn-sm"
              onClick={add}
              disabled={!name.trim() || !baseUrl.trim() || !model.trim()}
            >
              <Plus size={13} /> {t.aiServers.add}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
