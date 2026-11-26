import { useEffect, useState } from 'react'
import { Bell, Bug, Layers, Sliders, Webhook as WebhookIcon } from 'lucide-react'
import {
  api,
  EmailSettingsItem,
  EnvironmentItem,
  ExecutionStatusItem,
  IssueTrackerItem,
  NotificationEventPreference,
  PriorityItem,
  TemplateItem,
  WebhookItem,
} from '../../api/client'
import { useWorkspace } from '../../context/WorkspaceContext'
import { useI18n } from '../../i18n'

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
  const { t } = useI18n()
  const { notify } = useWorkspace()
  const [tab, setTab] = useState<'trackers' | 'env' | 'dicts' | 'notify' | 'webhooks'>('trackers')

  const [trackers, setTrackers] = useState<IssueTrackerItem[]>([])
  const [environments, setEnvironments] = useState<EnvironmentItem[]>([])
  const [templates, setTemplates] = useState<TemplateItem[]>([])
  const [priorities, setPriorities] = useState<PriorityItem[]>([])
  const [statuses, setStatuses] = useState<ExecutionStatusItem[]>([])

  const [trackerName, setTrackerName] = useState('')
  const [trackerProvider, setTrackerProvider] = useState<'github' | 'gitlab' | 'jira'>('github')
  const [trackerURL, setTrackerURL] = useState('')
  const [trackerUsername, setTrackerUsername] = useState('')

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
  const [notifPrefs, setNotifPrefs] = useState<NotificationEventPreference[]>([])

  const [webhooks, setWebhooks] = useState<WebhookItem[]>([])
  const [hookName, setHookName] = useState('')
  const [hookURL, setHookURL] = useState('')
  const [hookSecret, setHookSecret] = useState('')
  const [hookEvents, setHookEvents] = useState<string[]>(['run.stopped'])
  const [hookActive, setHookActive] = useState(true)

  const [error, setError] = useState<string | null>(null)

  const loadAll = async () => {
    try {
      const [tr, env, tpl, pri, st, prefs, hooks] = await Promise.all([
        api.listIssueTrackers(),
        api.listEnvironments(),
        api.listTemplates(),
        api.listPriorities(),
        api.listExecutionStatuses(),
        api.getMyNotificationSettings(),
        api.listWebhooks().catch(() => ({ items: [] as WebhookItem[] })),
      ])
      setTrackers(tr.items)
      setEnvironments(env.items)
      setTemplates(tpl.items)
      setPriorities(pri.items)
      setStatuses(st.items)
      setNotifPrefs(prefs.events)
      setWebhooks(hooks.items)
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : t.settings.loadFailed)
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
      setError(err instanceof Error ? err.message : t.settings.opFailed)
    }
  }

  const eventLabels: Record<string, string> = {
    'run.stopped': t.notifications.eventRunStopped,
    'execution.failed': t.notifications.eventExecutionFailed,
    'execution.assigned': t.notifications.eventExecutionAssigned,
  }
  const updateNotifPref = (eventType: string, channel: 'in_app' | 'email', value: boolean) => {
    setNotifPrefs((prev) => prev.map((p) => (p.event_type === eventType ? { ...p, [channel]: value } : p)))
  }

  return (
    <section style={{ display: 'grid', gap: '1rem' }}>
      {error && <p className="alert-error">{error}</p>}

      <div className="tabs-bar" style={{ marginBottom: 0 }}>
        <button type="button" className={`tab-btn ${tab === 'trackers' ? 'active' : ''}`} onClick={() => setTab('trackers')}>
          <Bug size={15} /> {t.settings.tabTrackers} ({trackers.length})
        </button>
        <button type="button" className={`tab-btn ${tab === 'env' ? 'active' : ''}`} onClick={() => setTab('env')}>
          <Layers size={15} /> {t.settings.tabEnv}
        </button>
        <button type="button" className={`tab-btn ${tab === 'dicts' ? 'active' : ''}`} onClick={() => setTab('dicts')}>
          <Sliders size={15} /> {t.settings.tabDicts}
        </button>
        <button type="button" className={`tab-btn ${tab === 'notify' ? 'active' : ''}`} onClick={() => setTab('notify')}>
          <Bell size={15} /> {t.settings.tabNotify}
        </button>
        <button type="button" className={`tab-btn ${tab === 'webhooks' ? 'active' : ''}`} onClick={() => setTab('webhooks')}>
          <WebhookIcon size={15} /> {t.settings.tabWebhooks} ({webhooks.length})
        </button>
      </div>

      {tab === 'trackers' && (
        <div className="card" style={{ display: 'grid', gap: '0.85rem' }}>
          <h3 className="card-title">{t.settings.trackersTitle}</h3>
          <form
            onSubmit={(e) => {
              e.preventDefault()
              if (!trackerName.trim() || !trackerURL.trim()) return
              void runAction(async () => {
                await api.createIssueTracker({
                  name: trackerName.trim(),
                  provider: trackerProvider,
                  base_url: trackerURL.trim(),
                  api_username: trackerUsername.trim() || undefined,
                })
                setTrackerName('')
                setTrackerURL('')
                setTrackerUsername('')
              }, `${t.settings.addTracker}: OK`)
            }}
            style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}
          >
            <input className="input" placeholder={t.settings.trackerNamePlaceholder} value={trackerName} onChange={(e) => setTrackerName(e.target.value)} required />
            <select className="select" value={trackerProvider} onChange={(e) => setTrackerProvider(e.target.value as 'github' | 'gitlab' | 'jira')}>
              <option value="github">GitHub</option>
              <option value="gitlab">GitLab</option>
              <option value="jira">Jira</option>
            </select>
            <input className="input" placeholder="https://api.github.com/repos/org/repo" value={trackerURL} onChange={(e) => setTrackerURL(e.target.value)} required style={{ flex: 1, minWidth: 220 }} />
            <input className="input" placeholder={t.settings.trackerUserPlaceholder} value={trackerUsername} onChange={(e) => setTrackerUsername(e.target.value)} style={{ minWidth: 200 }} />
            <button type="submit" className="btn btn-primary btn-sm">{t.settings.addTracker}</button>
          </form>
          {trackers.length > 0 && (
            <div className="table-wrap">
              <table className="data-table">
                <thead>
                  <tr>
                    <th>{t.common.id}</th>
                    <th>{t.common.name}</th>
                    <th>{t.settings.provider}</th>
                    <th>{t.settings.baseUrl}</th>
                  </tr>
                </thead>
                <tbody>
                  {trackers.map((tr) => (
                    <tr key={tr.id}>
                      <td style={{ fontFamily: 'var(--font-mono)', color: 'var(--muted)' }}>#{tr.id}</td>
                      <td>{tr.name}</td>
                      <td><span className="badge">{tr.provider}</span></td>
                      <td style={{ fontFamily: 'var(--font-mono)', fontSize: '0.8rem' }}>{tr.base_url}</td>
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
            <h3 className="card-title">{t.settings.environmentsTitle}</h3>
            <form
              onSubmit={(e) => {
                e.preventDefault()
                if (!envName.trim()) return
                void runAction(async () => {
                  await api.createEnvironment({ name: envName.trim(), description: envDesc.trim(), properties: parseProperties(envProps) })
                  setEnvName('')
                  setEnvDesc('')
                }, `${t.settings.addEnvironment}: OK`)
              }}
              style={{ display: 'grid', gap: '0.45rem' }}
            >
              <input className="input" placeholder={t.settings.envNamePlaceholder} value={envName} onChange={(e) => setEnvName(e.target.value)} required />
              <input className="input" placeholder={t.settings.envDescPlaceholder} value={envDesc} onChange={(e) => setEnvDesc(e.target.value)} />
              <textarea className="textarea" rows={3} placeholder={t.settings.envPropsPlaceholder} value={envProps} onChange={(e) => setEnvProps(e.target.value)} />
              <div>
                <button type="submit" className="btn btn-primary btn-sm">{t.settings.addEnvironment}</button>
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
            <h3 className="card-title">{t.settings.templatesTitle}</h3>
            <form
              onSubmit={(e) => {
                e.preventDefault()
                if (!tplName.trim() || !tplBody.trim()) return
                void runAction(async () => {
                  await api.createTemplate({ name: tplName.trim(), body: tplBody })
                  setTplName('')
                  setTplBody('')
                }, `${t.settings.addTemplate}: OK`)
              }}
              style={{ display: 'grid', gap: '0.45rem' }}
            >
              <input className="input" placeholder={t.settings.tplNamePlaceholder} value={tplName} onChange={(e) => setTplName(e.target.value)} required />
              <textarea className="textarea" rows={4} placeholder={t.settings.tplBodyPlaceholder} value={tplBody} onChange={(e) => setTplBody(e.target.value)} required />
              <div>
                <button type="submit" className="btn btn-primary btn-sm">{t.settings.addTemplate}</button>
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
            <h3 className="card-title">{t.settings.prioritiesTitle}</h3>
            <form
              onSubmit={(e) => {
                e.preventDefault()
                if (!newPriority.trim()) return
                void runAction(async () => {
                  await api.createPriority(newPriority.trim())
                  setNewPriority('')
                }, `${t.settings.prioritiesTitle}: OK`)
              }}
              style={{ display: 'flex', gap: '0.4rem' }}
            >
              <input className="input" placeholder={t.settings.priorityPlaceholder} value={newPriority} onChange={(e) => setNewPriority(e.target.value)} style={{ flex: 1 }} required />
              <button type="submit" className="btn btn-primary btn-sm">{t.common.add}</button>
            </form>
            <div style={{ display: 'flex', gap: '0.35rem', flexWrap: 'wrap' }}>
              {priorities.map((p) => (
                <span key={p.id} className="badge">{p.value}</span>
              ))}
            </div>
          </div>

          <div className="card" style={{ display: 'grid', gap: '0.65rem', alignContent: 'start' }}>
            <h3 className="card-title">{t.settings.statusesTitle}</h3>
            <form
              onSubmit={(e) => {
                e.preventDefault()
                if (!newStatusName.trim()) return
                void runAction(async () => {
                  await api.createExecutionStatus({ name: newStatusName.trim(), color: newStatusColor, weight: newStatusWeight })
                  setNewStatusName('')
                }, `${t.settings.statusesTitle}: OK`)
              }}
              style={{ display: 'flex', gap: '0.4rem', flexWrap: 'wrap' }}
            >
              <input className="input" placeholder={t.settings.statusPlaceholder} value={newStatusName} onChange={(e) => setNewStatusName(e.target.value)} style={{ flex: 1 }} required />
              <input type="color" value={newStatusColor} onChange={(e) => setNewStatusColor(e.target.value)} style={{ width: 38, height: 34, padding: 2, border: '1px solid var(--border-strong)', borderRadius: 6 }} />
              <input className="input" type="number" value={newStatusWeight} onChange={(e) => setNewStatusWeight(Number(e.target.value))} style={{ width: 72 }} placeholder={t.settings.weightPlaceholder} />
              <button type="submit" className="btn btn-primary btn-sm">{t.common.add}</button>
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
              }, `${t.settings.saveEmailSettings}: OK`)
            }}
            className="card"
            style={{ display: 'grid', gap: '0.6rem', alignContent: 'start' }}
          >
            <h3 className="card-title">{t.settings.ccListTitle}</h3>
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
                    notify(`${t.common.load}: #${emailEntityId}`)
                  } catch (err) {
                    setError(err instanceof Error ? err.message : t.settings.loadFailed)
                  }
                }}
              >
                {t.common.load}
              </button>
            </div>
            <label style={{ display: 'inline-flex', alignItems: 'center', gap: '0.4rem', fontSize: '0.84rem' }}>
              <input type="checkbox" checked={emailNotify} onChange={(e) => setEmailNotify(e.target.checked)} />
              {t.settings.notifyOnUpdate}
            </label>
            <input className="input" placeholder="qa@example.com, lead@example.com" value={emailCC} onChange={(e) => setEmailCC(e.target.value)} />
            <div>
              <button type="submit" className="btn btn-primary btn-sm">{t.settings.saveEmailSettings}</button>
            </div>
          </form>

          <form
            onSubmit={(e) => {
              e.preventDefault()
              void runAction(async () => {
                await api.sendTestWebhook(webhookMsg)
              }, `${t.settings.sendWebhook}: OK`)
            }}
            className="card"
            style={{ display: 'grid', gap: '0.6rem', alignContent: 'start' }}
          >
            <h3 className="card-title">{t.settings.webhookTitle}</h3>
            <input className="input" value={webhookMsg} onChange={(e) => setWebhookMsg(e.target.value)} required />
            <div>
              <button type="submit" className="btn btn-sm">{t.settings.sendWebhook}</button>
            </div>
          </form>

          <form
            onSubmit={(e) => {
              e.preventDefault()
              const recipients = testEmailTo.split(',').map((s) => s.trim()).filter(Boolean)
              if (recipients.length === 0) return
              void runAction(async () => {
                await api.sendTestEmail({ to: recipients, subject: testEmailSubject, body: testEmailBody })
              }, `${t.settings.sendEmail}: OK`)
            }}
            className="card"
            style={{ display: 'grid', gap: '0.6rem', alignContent: 'start' }}
          >
            <h3 className="card-title">{t.settings.smtpTitle}</h3>
            <input className="input" placeholder={t.settings.smtpRecipientsPlaceholder} value={testEmailTo} onChange={(e) => setTestEmailTo(e.target.value)} required />
            <input className="input" placeholder={t.settings.smtpSubjectPlaceholder} value={testEmailSubject} onChange={(e) => setTestEmailSubject(e.target.value)} required />
            <input className="input" placeholder={t.settings.smtpBodyPlaceholder} value={testEmailBody} onChange={(e) => setTestEmailBody(e.target.value)} required />
            <div>
              <button type="submit" className="btn btn-sm">{t.settings.sendEmail}</button>
            </div>
          </form>

          <form
            onSubmit={(e) => {
              e.preventDefault()
              void runAction(async () => {
                await api.putMyNotificationSettings(notifPrefs)
              }, t.notifications.prefsSaved)
            }}
            className="card"
            style={{ display: 'grid', gap: '0.6rem', alignContent: 'start' }}
          >
            <h3 className="card-title">{t.notifications.eventsTitle}</h3>
            <p style={{ color: 'var(--muted)', fontSize: '0.8rem', margin: 0 }}>{t.notifications.eventsHint}</p>
            <div className="table-wrap">
              <table className="data-table">
                <thead>
                  <tr>
                    <th>{t.notifications.eventColumn}</th>
                    <th>{t.notifications.channelInApp}</th>
                    <th>{t.notifications.channelEmail}</th>
                  </tr>
                </thead>
                <tbody>
                  {notifPrefs.map((p) => (
                    <tr key={p.event_type}>
                      <td>{eventLabels[p.event_type] ?? p.event_type}</td>
                      <td>
                        <input
                          type="checkbox"
                          checked={p.in_app}
                          onChange={(e) => updateNotifPref(p.event_type, 'in_app', e.target.checked)}
                          aria-label={`${eventLabels[p.event_type] ?? p.event_type} — ${t.notifications.channelInApp}`}
                        />
                      </td>
                      <td>
                        <input
                          type="checkbox"
                          checked={p.email}
                          onChange={(e) => updateNotifPref(p.event_type, 'email', e.target.checked)}
                          aria-label={`${eventLabels[p.event_type] ?? p.event_type} — ${t.notifications.channelEmail}`}
                        />
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <div>
              <button type="submit" className="btn btn-primary btn-sm">{t.notifications.savePrefs}</button>
            </div>
          </form>
        </div>
      )}

      {tab === 'webhooks' && (
        <div className="card" style={{ display: 'grid', gap: '0.85rem' }}>
          <h3 className="card-title">{t.settings.webhooksTitle}</h3>
          <p style={{ color: 'var(--muted)', fontSize: '0.8rem', margin: 0 }}>{t.settings.webhooksHint}</p>

          <form
            onSubmit={(e) => {
              e.preventDefault()
              if (!hookName.trim() || !hookURL.trim() || hookEvents.length === 0) return
              void runAction(async () => {
                await api.createWebhook({
                  name: hookName.trim(),
                  url: hookURL.trim(),
                  secret: hookSecret.trim() || undefined,
                  events: [...hookEvents],
                  is_active: hookActive,
                })
                setHookName('')
                setHookURL('')
                setHookSecret('')
                setHookEvents(['run.stopped'])
                setHookActive(true)
              }, `${t.settings.addWebhook}: OK`)
            }}
            style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap', alignItems: 'center' }}
          >
            <input className="input" placeholder={t.settings.webhookNamePlaceholder} value={hookName} onChange={(e) => setHookName(e.target.value)} required style={{ minWidth: 140 }} />
            <input className="input" placeholder="https://hooks.example.com/..." value={hookURL} onChange={(e) => setHookURL(e.target.value)} required style={{ flex: 1, minWidth: 240 }} />
            <input className="input" placeholder={t.settings.webhookSecretPlaceholder} value={hookSecret} onChange={(e) => setHookSecret(e.target.value)} style={{ minWidth: 140 }} />
            <div style={{ display: 'flex', gap: '0.6rem', alignItems: 'center', flexWrap: 'wrap' }}>
              {Object.keys(eventLabels).map((ev) => (
                <label key={ev} style={{ display: 'inline-flex', alignItems: 'center', gap: '0.3rem', fontSize: '0.8rem' }}>
                  <input
                    type="checkbox"
                    checked={hookEvents.includes(ev)}
                    onChange={(e) => setHookEvents((prev) => (e.target.checked ? [...prev, ev] : prev.filter((x) => x !== ev)))}
                  />
                  {eventLabels[ev]}
                </label>
              ))}
              <label style={{ display: 'inline-flex', alignItems: 'center', gap: '0.3rem', fontSize: '0.8rem' }}>
                <input type="checkbox" checked={hookActive} onChange={(e) => setHookActive(e.target.checked)} />
                {t.settings.webhookActive}
              </label>
            </div>
            <button type="submit" className="btn btn-primary btn-sm">{t.settings.addWebhook}</button>
          </form>

          {webhooks.length > 0 && (
            <div className="table-wrap">
              <table className="data-table">
                <thead>
                  <tr>
                    <th>{t.common.id}</th>
                    <th>{t.common.name}</th>
                    <th>URL</th>
                    <th>{t.settings.webhookEvents}</th>
                    <th>{t.common.status}</th>
                    <th>{t.common.actions}</th>
                  </tr>
                </thead>
                <tbody>
                  {webhooks.map((h) => (
                    <tr key={h.id}>
                      <td style={{ fontFamily: 'var(--font-mono)', color: 'var(--muted)' }}>#{h.id}</td>
                      <td>{h.name}</td>
                      <td style={{ fontFamily: 'var(--font-mono)', fontSize: '0.8rem' }}>{h.url}</td>
                      <td>
                        <div style={{ display: 'flex', gap: '0.3rem', flexWrap: 'wrap' }}>
                          {h.events.map((ev) => (
                            <span key={ev} className="badge">{eventLabels[ev] ?? ev}</span>
                          ))}
                        </div>
                      </td>
                      <td>
                        <span className={`badge ${h.is_active ? 'pill-passed' : 'pill-idle'}`}>
                          {h.is_active ? t.settings.webhookActive : t.settings.webhookInactive}
                        </span>
                      </td>
                      <td>
                        <button
                          type="button"
                          className="btn btn-ghost btn-sm"
                          onClick={async () => {
                            setError(null)
                            try {
                              await api.testWebhook(h.id)
                              notify(`${h.name}: ${t.settings.webhookTestOk}`)
                            } catch (err) {
                              setError(err instanceof Error ? err.message : t.settings.opFailed)
                            }
                          }}
                        >
                          {t.settings.testWebhook}
                        </button>{' '}
                        <button
                          type="button"
                          className="btn btn-ghost btn-sm"
                          onClick={() => {
                            if (window.confirm(`${t.common.delete}: ${h.name}?`)) {
                              void runAction(async () => {
                                await api.deleteWebhook(h.id)
                              }, `${t.common.delete}: OK`)
                            }
                          }}
                        >
                          {t.common.delete}
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}
    </section>
  )
}
