import { useEffect, useState } from 'react'
import { Bell, Bug, Layers, Sliders } from 'lucide-react'
import {
  api,
  EmailSettingsItem,
  EnvironmentItem,
  ExecutionStatusItem,
  IssueTrackerItem,
  PriorityItem,
  TemplateItem,
} from '../../api/client'
import { useWorkspace } from '../../context/WorkspaceContext'

function parseProperties(raw: string): Record<string, string> {
  const out: Record<string, string> = {}
  for (const line of raw.split('\n')) {
    const trimmed = line.trim()
    if (!trimmed) continue
    const idx = trimmed.indexOf('=')
    if (idx > 0) out[trimmed.slice(0, idx).trim()] = trimmed.slice(idx + 1).trim()
  }
  return out
}

export function SettingsView() {
  const { notify } = useWorkspace()
  const [tab, setTab] = useState<'trackers' | 'env' | 'dicts' | 'notify'>('trackers')

  const [trackers, setTrackers] = useState<IssueTrackerItem[]>([])
  const [environments, setEnvironments] = useState<EnvironmentItem[]>([])
  const [templates, setTemplates] = useState<TemplateItem[]>([])
  const [priorities, setPriorities] = useState<PriorityItem[]>([])
  const [statuses, setStatuses] = useState<ExecutionStatusItem[]>([])

  const [trackerName, setTrackerName] = useState('')
  const [trackerProvider, setTrackerProvider] = useState<'github' | 'gitlab' | 'jira'>('github')
  const [trackerURL, setTrackerURL] = useState('')

  const [envName, setEnvName] = useState('')
  const [envDesc, setEnvDesc] = useState('')
  const [envProps, setEnvProps] = useState('os=linux\nbrowser=chrome')

  const [tplName, setTplName] = useState('')
  const [tplBody, setTplBody] = useState('')

  const [newPriority, setNewPriority] = useState('')
  const [newStatusName, setNewStatusName] = useState('')
  const [newStatusColor, setNewStatusColor] = useState('#2563eb')
  const [newStatusWeight, setNewStatusWeight] = useState(0)

  const [emailEntityType, setEmailEntityType] = useState<'test_case' | 'test_plan'>('test_case')
  const [emailEntityId, setEmailEntityId] = useState(1)
  const [emailNotify, setEmailNotify] = useState(true)
  const [emailCC, setEmailCC] = useState('')
  const [webhookMsg, setWebhookMsg] = useState('Hello from Kiwit TCMS')
  const [testEmailTo, setTestEmailTo] = useState('')
  const [testEmailSubject, setTestEmailSubject] = useState('Kiwit SMTP Test')
  const [testEmailBody, setTestEmailBody] = useState('SMTP notification test from Kiwit TCMS.')

  const [error, setError] = useState<string | null>(null)

  const loadAll = async () => {
    try {
      const [tr, env, tpl, pri, st] = await Promise.all([
        api.listIssueTrackers(),
        api.listEnvironments(),
        api.listTemplates(),
        api.listPriorities(),
        api.listExecutionStatuses(),
      ])
      setTrackers(tr.items)
      setEnvironments(env.items)
      setTemplates(tpl.items)
      setPriorities(pri.items)
      setStatuses(st.items)
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load settings')
    }
  }

  useEffect(() => {
    void loadAll()
  }, [])

  const runAction = async (fn: () => Promise<void>, okMessage: string) => {
    setError(null)
    try {
      await fn()
      notify(okMessage)
      await loadAll()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Operation failed')
    }
  }

  return (
    <section style={{ display: 'grid', gap: '1rem' }}>
      {error && <p className="alert-error">{error}</p>}

      <div className="tabs-bar" style={{ marginBottom: 0 }}>
        <button type="button" className={`tab-btn ${tab === 'trackers' ? 'active' : ''}`} onClick={() => setTab('trackers')}>
          <Bug size={15} /> Issue Trackers ({trackers.length})
        </button>
        <button type="button" className={`tab-btn ${tab === 'env' ? 'active' : ''}`} onClick={() => setTab('env')}>
          <Layers size={15} /> Environments &amp; Templates
        </button>
        <button type="button" className={`tab-btn ${tab === 'dicts' ? 'active' : ''}`} onClick={() => setTab('dicts')}>
          <Sliders size={15} /> Dictionaries
        </button>
        <button type="button" className={`tab-btn ${tab === 'notify' ? 'active' : ''}`} onClick={() => setTab('notify')}>
          <Bell size={15} /> Notifications &amp; Email
        </button>
      </div>

      {tab === 'trackers' && (
        <div className="card" style={{ display: 'grid', gap: '0.85rem' }}>
          <h3 className="card-title">Issue Trackers (GitHub / GitLab / Jira)</h3>
          <form
            onSubmit={(e) => {
              e.preventDefault()
              if (!trackerName.trim() || !trackerURL.trim()) return
              void runAction(async () => {
                await api.createIssueTracker({ name: trackerName.trim(), provider: trackerProvider, base_url: trackerURL.trim() })
                setTrackerName('')
                setTrackerURL('')
              }, 'Issue tracker created.')
            }}
            style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}
          >
            <input className="input" placeholder="Name (e.g. GitHub Main)" value={trackerName} onChange={(e) => setTrackerName(e.target.value)} required />
            <select className="select" value={trackerProvider} onChange={(e) => setTrackerProvider(e.target.value as 'github' | 'gitlab' | 'jira')}>
              <option value="github">GitHub</option>
              <option value="gitlab">GitLab</option>
              <option value="jira">Jira</option>
            </select>
            <input className="input" placeholder="https://api.github.com/repos/org/repo" value={trackerURL} onChange={(e) => setTrackerURL(e.target.value)} required style={{ flex: 1, minWidth: 240 }} />
            <button type="submit" className="btn btn-primary btn-sm">Add tracker</button>
          </form>
          {trackers.length > 0 && (
            <div className="table-wrap">
              <table className="data-table">
                <thead>
                  <tr>
                    <th>ID</th>
                    <th>Name</th>
                    <th>Provider</th>
                    <th>Base URL</th>
                  </tr>
                </thead>
                <tbody>
                  {trackers.map((t) => (
                    <tr key={t.id}>
                      <td style={{ fontFamily: 'var(--font-mono)', color: 'var(--muted)' }}>#{t.id}</td>
                      <td>{t.name}</td>
                      <td><span className="badge">{t.provider}</span></td>
                      <td style={{ fontFamily: 'var(--font-mono)', fontSize: '0.8rem' }}>{t.base_url}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}

      {tab === 'env' && (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(360px, 1fr))', gap: '1rem' }}>
          <div className="card" style={{ display: 'grid', gap: '0.7rem', alignContent: 'start' }}>
            <h3 className="card-title">Environments</h3>
            <form
              onSubmit={(e) => {
                e.preventDefault()
                if (!envName.trim()) return
                void runAction(async () => {
                  await api.createEnvironment({ name: envName.trim(), description: envDesc.trim(), properties: parseProperties(envProps) })
                  setEnvName('')
                  setEnvDesc('')
                }, 'Environment created.')
              }}
              style={{ display: 'grid', gap: '0.45rem' }}
            >
              <input className="input" placeholder="Name (e.g. Staging Linux)" value={envName} onChange={(e) => setEnvName(e.target.value)} required />
              <input className="input" placeholder="Description (optional)" value={envDesc} onChange={(e) => setEnvDesc(e.target.value)} />
              <textarea className="textarea" rows={3} placeholder="key=value (one per line)" value={envProps} onChange={(e) => setEnvProps(e.target.value)} />
              <div>
                <button type="submit" className="btn btn-primary btn-sm">Add environment</button>
              </div>
            </form>
            {environments.map((env) => (
              <div key={env.id} style={{ padding: '0.6rem', border: '1px solid var(--border)', borderRadius: 8, background: 'var(--bg-subtle)' }}>
                <strong>{env.name}</strong>
                {env.description && <div style={{ fontSize: '0.8rem', color: 'var(--muted)' }}>{env.description}</div>}
                <div style={{ display: 'flex', gap: '0.3rem', flexWrap: 'wrap', marginTop: '0.35rem' }}>
                  {Object.entries(env.properties).map(([k, v]) => (
                    <span key={k} className="badge">{k}={v}</span>
                  ))}
                </div>
              </div>
            ))}
          </div>

          <div className="card" style={{ display: 'grid', gap: '0.7rem', alignContent: 'start' }}>
            <h3 className="card-title">Test Case Templates</h3>
            <form
              onSubmit={(e) => {
                e.preventDefault()
                if (!tplName.trim() || !tplBody.trim()) return
                void runAction(async () => {
                  await api.createTemplate({ name: tplName.trim(), body: tplBody })
                  setTplName('')
                  setTplBody('')
                }, 'Template created.')
              }}
              style={{ display: 'grid', gap: '0.45rem' }}
            >
              <input className="input" placeholder="Template name" value={tplName} onChange={(e) => setTplName(e.target.value)} required />
              <textarea className="textarea" rows={4} placeholder="Template body (Markdown or Gherkin)" value={tplBody} onChange={(e) => setTplBody(e.target.value)} required />
              <div>
                <button type="submit" className="btn btn-primary btn-sm">Add template</button>
              </div>
            </form>
            {templates.map((tpl) => (
              <div key={tpl.id} style={{ padding: '0.6rem', border: '1px solid var(--border)', borderRadius: 8, background: 'var(--bg-subtle)' }}>
                <strong>{tpl.name}</strong>
                <pre style={{ margin: '0.3rem 0 0', fontSize: '0.78rem', whiteSpace: 'pre-wrap', color: 'var(--muted)' }}>{tpl.body}</pre>
              </div>
            ))}
          </div>
        </div>
      )}

      {tab === 'dicts' && (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(340px, 1fr))', gap: '1rem' }}>
          <div className="card" style={{ display: 'grid', gap: '0.65rem', alignContent: 'start' }}>
            <h3 className="card-title">Priorities</h3>
            <form
              onSubmit={(e) => {
                e.preventDefault()
                if (!newPriority.trim()) return
                void runAction(async () => {
                  await api.createPriority(newPriority.trim())
                  setNewPriority('')
                }, 'Priority added.')
              }}
              style={{ display: 'flex', gap: '0.4rem' }}
            >
              <input className="input" placeholder="e.g. P0 / Blocker" value={newPriority} onChange={(e) => setNewPriority(e.target.value)} style={{ flex: 1 }} required />
              <button type="submit" className="btn btn-primary btn-sm">Add</button>
            </form>
            <div style={{ display: 'flex', gap: '0.35rem', flexWrap: 'wrap' }}>
              {priorities.map((p) => (
                <span key={p.id} className="badge">{p.value}</span>
              ))}
            </div>
          </div>

          <div className="card" style={{ display: 'grid', gap: '0.65rem', alignContent: 'start' }}>
            <h3 className="card-title">Execution Statuses</h3>
            <form
              onSubmit={(e) => {
                e.preventDefault()
                if (!newStatusName.trim()) return
                void runAction(async () => {
                  await api.createExecutionStatus({ name: newStatusName.trim(), color: newStatusColor, weight: newStatusWeight })
                  setNewStatusName('')
                }, 'Execution status added.')
              }}
              style={{ display: 'flex', gap: '0.4rem', flexWrap: 'wrap' }}
            >
              <input className="input" placeholder="Status name" value={newStatusName} onChange={(e) => setNewStatusName(e.target.value)} style={{ flex: 1 }} required />
              <input type="color" value={newStatusColor} onChange={(e) => setNewStatusColor(e.target.value)} style={{ width: 38, height: 34, padding: 2, border: '1px solid var(--border-strong)', borderRadius: 6 }} />
              <input className="input" type="number" value={newStatusWeight} onChange={(e) => setNewStatusWeight(Number(e.target.value))} style={{ width: 72 }} placeholder="Weight" />
              <button type="submit" className="btn btn-primary btn-sm">Add</button>
            </form>
            <div style={{ display: 'flex', gap: '0.35rem', flexWrap: 'wrap' }}>
              {statuses.map((s) => (
                <span key={s.id} className="badge" style={{ background: s.color, color: '#fff', borderColor: 'transparent' }}>
                  {s.name} ({s.weight})
                </span>
              ))}
            </div>
          </div>
        </div>
      )}

      {tab === 'notify' && (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(300px, 1fr))', gap: '1rem' }}>
          <form
            onSubmit={(e) => {
              e.preventDefault()
              const cc = emailCC.split(',').map((s) => s.trim()).filter(Boolean)
              void runAction(async () => {
                await api.upsertEmailSettings({
                  entity_type: emailEntityType,
                  entity_id: emailEntityId,
                  notify_on_update: emailNotify,
                  cc_list: cc,
                })
              }, `Saved email settings for ${emailEntityType} #${emailEntityId}.`)
            }}
            className="card"
            style={{ display: 'grid', gap: '0.6rem', alignContent: 'start' }}
          >
            <h3 className="card-title">Entity CC List</h3>
            <div style={{ display: 'flex', gap: '0.4rem' }}>
              <select className="select" value={emailEntityType} onChange={(e) => setEmailEntityType(e.target.value as 'test_case' | 'test_plan')}>
                <option value="test_case">test_case</option>
                <option value="test_plan">test_plan</option>
              </select>
              <input className="input" type="number" min={1} value={emailEntityId} onChange={(e) => setEmailEntityId(Number(e.target.value))} style={{ width: 90 }} />
              <button
                type="button"
                className="btn btn-sm"
                onClick={async () => {
                  try {
                    const res: EmailSettingsItem = await api.getEmailSettings(emailEntityType, emailEntityId)
                    setEmailNotify(res.notify_on_update)
                    setEmailCC(res.cc_list.join(', '))
                    notify(`Loaded settings for ${emailEntityType} #${emailEntityId}`)
                  } catch (err) {
                    setError(err instanceof Error ? err.message : 'Failed to load email settings')
                  }
                }}
              >
                Load
              </button>
            </div>
            <label style={{ display: 'inline-flex', alignItems: 'center', gap: '0.4rem', fontSize: '0.84rem' }}>
              <input type="checkbox" checked={emailNotify} onChange={(e) => setEmailNotify(e.target.checked)} />
              Notify on update
            </label>
            <input className="input" placeholder="qa@example.com, lead@example.com" value={emailCC} onChange={(e) => setEmailCC(e.target.value)} />
            <div>
              <button type="submit" className="btn btn-primary btn-sm">Save email settings</button>
            </div>
          </form>

          <form
            onSubmit={(e) => {
              e.preventDefault()
              void runAction(async () => {
                await api.sendTestWebhook(webhookMsg)
              }, 'Test webhook sent.')
            }}
            className="card"
            style={{ display: 'grid', gap: '0.6rem', alignContent: 'start' }}
          >
            <h3 className="card-title">Test Webhook (Slack / Teams)</h3>
            <input className="input" value={webhookMsg} onChange={(e) => setWebhookMsg(e.target.value)} required />
            <div>
              <button type="submit" className="btn btn-sm">Send test webhook</button>
            </div>
          </form>

          <form
            onSubmit={(e) => {
              e.preventDefault()
              const recipients = testEmailTo.split(',').map((s) => s.trim()).filter(Boolean)
              if (recipients.length === 0) return
              void runAction(async () => {
                await api.sendTestEmail({ to: recipients, subject: testEmailSubject, body: testEmailBody })
              }, 'Test SMTP email sent.')
            }}
            className="card"
            style={{ display: 'grid', gap: '0.6rem', alignContent: 'start' }}
          >
            <h3 className="card-title">Test SMTP Email</h3>
            <input className="input" placeholder="Recipient email(s), comma-separated" value={testEmailTo} onChange={(e) => setTestEmailTo(e.target.value)} required />
            <input className="input" placeholder="Subject" value={testEmailSubject} onChange={(e) => setTestEmailSubject(e.target.value)} required />
            <input className="input" placeholder="Body" value={testEmailBody} onChange={(e) => setTestEmailBody(e.target.value)} required />
            <div>
              <button type="submit" className="btn btn-sm">Send test email</button>
            </div>
          </form>
        </div>
      )}
    </section>
  )
}
