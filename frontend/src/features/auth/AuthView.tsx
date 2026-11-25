import { useEffect, useState } from 'react'
import { api, ApiTokenInfo, getAuthToken, setAuthToken, User } from '../../api/client'
import { useI18n } from '../../i18n'

export function AuthView() {
  const { t } = useI18n()
  const [mode, setMode] = useState<'login' | 'register'>('login')
  const [email, setEmail] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [tokenName, setTokenName] = useState('ci-runner')
  const [generatedToken, setGeneratedToken] = useState<string | null>(null)
  const [tokens, setTokens] = useState<ApiTokenInfo[]>([])
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [users, setUsers] = useState<User[]>([])
  const [isAdmin, setIsAdmin] = useState(false)
  const [jwt, setJwt] = useState<string | null>(getAuthToken())
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)

  const loadAccountData = async () => {
    try {
      const tokRes = await api.listApiTokens()
      setTokens(tokRes.items)
    } catch {
      setTokens([])
    }
    try {
      const uRes = await api.listUsers()
      setUsers(uRes.items)
      setIsAdmin(true)
    } catch {
      setUsers([])
      setIsAdmin(false)
    }
  }

  useEffect(() => {
    if (jwt) {
      void loadAccountData()
    }
  }, [jwt])

  const handleAuth = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setNotice(null)
    try {
      if (mode === 'register') {
        await api.register({ email, username, password })
      }
      const res = await api.login({ email, password })
      setAuthToken(res.access_token)
      setJwt(res.access_token)
      setPassword('')
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errors.authFailed)
    }
  }

  const handleCreateToken = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setNotice(null)
    try {
      const res = await api.createApiToken(tokenName.trim() || 'ci-runner')
      setGeneratedToken(res.token)
      await loadAccountData()
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errors.createToken)
    }
  }

  const handleRevokeToken = async (id: number) => {
    if (!window.confirm('Revoke this API token?')) return
    setError(null)
    try {
      await api.revokeApiToken(id)
      await loadAccountData()
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errors.createToken)
    }
  }

  const handleChangePassword = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setNotice(null)
    try {
      await api.changePassword({ current_password: currentPassword, new_password: newPassword })
      setCurrentPassword('')
      setNewPassword('')
      setNotice('Password updated.')
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errors.authFailed)
    }
  }

  const handleUpdateUser = async (u: User, role: 'viewer' | 'tester' | 'admin', isActive: boolean) => {
    setError(null)
    setNotice(null)
    try {
      await api.updateUser(u.id, { role, is_active: isActive })
      await loadAccountData()
      setNotice(`Updated user ${u.email}.`)
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errors.authFailed)
    }
  }

  const handleLogout = () => {
    setAuthToken(null)
    setJwt(null)
    setGeneratedToken(null)
    setTokens([])
    setUsers([])
    setIsAdmin(false)
  }

  return (
    <section style={{ maxWidth: 820, display: 'grid', gap: '1.25rem' }}>
      <h2 style={{ margin: 0 }}>{t.auth.title}</h2>
      {error && <p className="alert-error">{error}</p>}
      {notice && <p className="alert-ok">{notice}</p>}

      {!jwt ? (
        <form onSubmit={handleAuth} className="card" style={{ maxWidth: 460, display: 'grid', gap: '0.65rem' }}>
          <div style={{ display: 'flex', gap: '0.5rem' }}>
            <button type="button" onClick={() => setMode('login')} className={mode === 'login' ? 'btn btn-primary btn-sm' : 'btn btn-sm'}>
              {t.auth.login}
            </button>
            <button type="button" onClick={() => setMode('register')} className={mode === 'register' ? 'btn btn-primary btn-sm' : 'btn btn-sm'}>
              {t.auth.register}
            </button>
          </div>
          <label htmlFor="auth-email" style={{ display: 'grid', gap: '0.25rem', fontSize: '0.85rem' }}>
            <span>Email</span>
            <input
              id="auth-email"
              name="email"
              className="input"
              type="email"
              autoComplete="username"
              spellCheck={false}
              enterKeyHint="next"
              placeholder="Email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
            />
          </label>
          {mode === 'register' && (
            <label htmlFor="auth-username" style={{ display: 'grid', gap: '0.25rem', fontSize: '0.85rem' }}>
              <span>{t.auth.usernamePlaceholder}</span>
              <input
                id="auth-username"
                name="username"
                className="input"
                autoComplete="nickname"
                spellCheck={false}
                autoCapitalize="none"
                {...({ autocorrect: 'off' } as Record<string, string>)}
                enterKeyHint="next"
                placeholder={t.auth.usernamePlaceholder}
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                required
              />
            </label>
          )}
          <label htmlFor="auth-password" style={{ display: 'grid', gap: '0.25rem', fontSize: '0.85rem' }}>
            <span>{t.auth.passwordPlaceholder}</span>
            <div style={{ display: 'flex', gap: '0.4rem' }}>
              <input
                id="auth-password"
                name="password"
                className="input"
                type={showPassword ? 'text' : 'password'}
                autoComplete={mode === 'login' ? 'current-password' : 'new-password'}
                enterKeyHint="done"
                placeholder={t.auth.passwordPlaceholder}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                minLength={8}
                style={{ flex: 1 }}
              />
              <button
                type="button"
                className="btn btn-sm"
                onClick={() => setShowPassword((v) => !v)}
                aria-pressed={showPassword}
              >
                {showPassword ? 'Hide' : 'Show'}
              </button>
            </div>
          </label>
          <button type="submit" className="btn btn-primary">
            {mode === 'login' ? t.auth.submitLogin : t.auth.submitRegister}
          </button>
        </form>
      ) : (
        <div style={{ display: 'grid', gap: '1rem' }}>
          <div className="card" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <span>{t.auth.loggedIn}</span>
            <button type="button" onClick={handleLogout} className="btn btn-sm">{t.auth.logout}</button>
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(320px, 1fr))', gap: '1rem' }}>
            <form onSubmit={handleCreateToken} className="card" style={{ display: 'grid', gap: '0.6rem', alignContent: 'start' }}>
              <strong>{t.auth.tokenTitle}</strong>
              <div style={{ display: 'flex', gap: '0.5rem' }}>
                <input className="input" placeholder={t.auth.tokenPlaceholder} value={tokenName} onChange={(e) => setTokenName(e.target.value)} style={{ flex: 1 }} />
                <button type="submit" className="btn btn-primary btn-sm">{t.auth.generate}</button>
              </div>
              {generatedToken && (
                <div style={{ padding: '0.5rem', background: '#f1f5f9', borderRadius: 6, fontFamily: 'monospace', fontSize: '0.8rem', wordBreak: 'break-all' }}>
                  {generatedToken}
                </div>
              )}
              {tokens.length > 0 && (
                <table className="data-table">
                  <thead>
                    <tr>
                      <th>Name</th>
                      <th>Last used</th>
                      <th />
                    </tr>
                  </thead>
                  <tbody>
                    {tokens.map((tok) => (
                      <tr key={tok.id}>
                        <td>{tok.name}</td>
                        <td style={{ color: 'var(--muted)' }}>{tok.last_used_at ? new Date(tok.last_used_at).toLocaleDateString() : 'Never'}</td>
                        <td style={{ textAlign: 'right' }}>
                          <button type="button" className="btn btn-danger btn-sm" onClick={() => void handleRevokeToken(tok.id)}>
                            Revoke
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </form>

            <form onSubmit={handleChangePassword} className="card" style={{ display: 'grid', gap: '0.6rem', alignContent: 'start' }}>
              <strong>Change password</strong>
              <label htmlFor="current-password" style={{ display: 'grid', gap: '0.25rem', fontSize: '0.85rem' }}>
                <span>Current password</span>
                <input
                  id="current-password"
                  name="current-password"
                  className="input"
                  type="password"
                  autoComplete="current-password"
                  enterKeyHint="next"
                  placeholder="Current password"
                  value={currentPassword}
                  onChange={(e) => setCurrentPassword(e.target.value)}
                  required
                  minLength={8}
                />
              </label>
              <label htmlFor="new-password" style={{ display: 'grid', gap: '0.25rem', fontSize: '0.85rem' }}>
                <span>New password</span>
                <input
                  id="new-password"
                  name="new-password"
                  className="input"
                  type="password"
                  autoComplete="new-password"
                  enterKeyHint="done"
                  placeholder="New password (min. 8 chars)"
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  required
                  minLength={8}
                />
              </label>
              <button type="submit" className="btn btn-primary btn-sm">Update password</button>
            </form>
          </div>

          {isAdmin && (
            <div className="card" style={{ display: 'grid', gap: '0.6rem' }}>
              <strong>Admin — Users ({users.length})</strong>
              <table className="data-table">
                <thead>
                  <tr>
                    <th>ID</th>
                    <th>Email</th>
                    <th>Username</th>
                    <th>Role</th>
                    <th>Active</th>
                  </tr>
                </thead>
                <tbody>
                  {users.map((u) => (
                    <tr key={u.id}>
                      <td>#{u.id}</td>
                      <td>{u.email}</td>
                      <td>{u.username}</td>
                      <td>
                        <select
                          className="select"
                          value={u.role}
                          onChange={(e) => void handleUpdateUser(u, e.target.value as 'viewer' | 'tester' | 'admin', u.is_active ?? true)}
                        >
                          <option value="viewer">viewer</option>
                          <option value="tester">tester</option>
                          <option value="admin">admin</option>
                        </select>
                      </td>
                      <td>
                        <button
                          type="button"
                          className={u.is_active ? 'btn btn-sm' : 'btn btn-danger btn-sm'}
                          onClick={() => void handleUpdateUser(u, u.role as 'viewer' | 'tester' | 'admin', !(u.is_active ?? true))}
                        >
                          {u.is_active ? 'Active' : 'Disabled'}
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
