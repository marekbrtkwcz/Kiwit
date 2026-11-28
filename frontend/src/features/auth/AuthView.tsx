import { useEffect, useState } from 'react'
import { KeyRound, LogIn, LogOut, Mail, Plus } from 'lucide-react'
import { useLocation, useNavigate, useSearchParams } from 'react-router-dom'
import { api, ApiTokenInfo, getAuthToken, setAuthToken, setUserRole, User, UserGroup } from '../../api/client'
import { useWorkspace } from '../../context/WorkspaceContext'
import { useI18n } from '../../i18n'
import { generatePassword } from '../../utils/password'

type AuthMode = 'login' | 'register' | 'reset-request' | 'reset-confirm'

export function AuthView() {
  const { t } = useI18n()
  const { refreshProducts, notify } = useWorkspace()
  const [searchParams] = useSearchParams()
  const location = useLocation()
  const navigate = useNavigate()
  const [mode, setMode] = useState<AuthMode>(() => (searchParams.get('token') ? 'reset-confirm' : 'login'))
  const [email, setEmail] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [showConfirmPassword, setShowConfirmPassword] = useState(false)
  const [resetPassword, setResetPassword] = useState('')
  const [resetPasswordRepeat, setResetPasswordRepeat] = useState('')
  const [tokenName, setTokenName] = useState('ci-runner')
  const [generatedToken, setGeneratedToken] = useState<string | null>(null)
  const [tokens, setTokens] = useState<ApiTokenInfo[]>([])
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [users, setUsers] = useState<User[]>([])
  const [isAdmin, setIsAdmin] = useState(false)
  const [groups, setGroups] = useState<UserGroup[]>([])
  const [newGroupName, setNewGroupName] = useState('')
  const [jwt, setJwt] = useState<string | null>(getAuthToken())
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const [ssoEnabled, setSsoEnabled] = useState(false)
  const [ssoProvider, setSsoProvider] = useState('SSO')
  const [ldapEnabled, setLdapEnabled] = useState(false)
  const [ldapUsername, setLdapUsername] = useState('')
  const [ldapPassword, setLdapPassword] = useState('')
  // Public instance settings (allow_registration, captcha_enabled, …)
  const [pubSettings, setPubSettings] = useState<Record<string, string> | null>(null)
  const [captcha, setCaptcha] = useState<{ captcha_id: string; question: string } | null>(null)
  const [captchaAnswer, setCaptchaAnswer] = useState('')
  const [website, setWebsite] = useState('') // honeypot — must stay empty

  // Handle SSO callback: backend redirects to /auth/callback#token=...
  // Only the pathname triggers this — a ?token= on /reset-password is a
  // password-reset token and must NOT be stored as a session JWT.
  useEffect(() => {
    if (location.pathname === '/auth/callback') {
      const tok = new URLSearchParams(location.hash.replace(/^#\/?/, '?')).get('token') || searchParams.get('token')
      if (tok) {
        setAuthToken(tok)
        setJwt(tok)
        navigate('/auth', { replace: true })
      }
    }
  }, [location, searchParams, navigate])

  useEffect(() => {
    fetch('/api/v1/auth/sso/config')
      .then(r => (r.ok ? r.json() : null))
      .then(d => {
        if (d?.enabled) {
          setSsoEnabled(true)
          setSsoProvider(d.provider || 'SSO')
        }
      })
      .catch(() => {})
    api.authMethods()
      .then(d => setLdapEnabled(!!d.ldap))
      .catch(() => {})
    api.getPublicSettings()
      .then(setPubSettings)
      .catch(() => setPubSettings(null))
  }, [])

  const registrationAllowed = pubSettings?.allow_registration !== 'false'
  const captchaRequired = pubSettings?.captcha_enabled === 'true'

  // When registration is disabled, never stay on the register form.
  useEffect(() => {
    if (mode === 'register' && pubSettings && !registrationAllowed) setMode('login')
  }, [mode, pubSettings, registrationAllowed])

  // Fetch a fresh captcha challenge when entering the register form.
  useEffect(() => {
    if (mode === 'register' && captchaRequired && !captcha) {
      api.getCaptcha()
        .then(setCaptcha)
        .catch(() => setCaptcha(null))
    }
  }, [mode, captchaRequired, captcha])

  const handleLdapLogin = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    try {
      const res = await api.ldapLogin({ username: ldapUsername, password: ldapPassword })
      setAuthToken(res.access_token)
      setUserRole(res.user.role)
      setJwt(res.access_token)
      notify(t.auth.signedInAs(res.user.username))
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errors.authFailed)
    }
  }

  // Reset views ignore any stored JWT: the main reset scenario is a user
  // whose session expired clicking the e-mail link, so a stored (possibly
  // stale) token must never hide the reset form behind the account panel.
  const inResetContext = location.pathname === '/reset-password' || !!searchParams.get('token')

  const loadAccountData = async () => {
    try {
      const tokRes = await api.listApiTokens()
      setTokens(tokRes.items ?? [])
    } catch {
      setTokens([])
    }
    try {
      const uRes = await api.listUsers()
      setUsers(uRes.items ?? [])
      setIsAdmin(true)
    } catch {
      setUsers([])
      setIsAdmin(false)
    }
    try {
      const gRes = await api.listGroups()
      setGroups(gRes.items ?? [])
    } catch {
      setGroups([])
    }
  }

  const reloadGroups = async () => {
    try {
      const gRes = await api.listGroups()
      setGroups(gRes.items ?? [])
    } catch { /* ignore */ }
  }

  const handleCreateGroup = async (e: React.FormEvent) => {
    e.preventDefault()
    const name = newGroupName.trim()
    if (!name) return
    try {
      await api.createGroup(name)
      setNewGroupName('')
      await reloadGroups()
    } catch { setError(t.settings.opFailed) }
  }

  const handleDeleteGroup = async (id: number) => {
    try { await api.deleteGroup(id); await reloadGroups() } catch { setError(t.settings.opFailed) }
  }

  const handleAddMember = async (groupId: number, userId: number) => {
    try { await api.addGroupMember(groupId, userId); await reloadGroups() } catch { setError(t.settings.opFailed) }
  }

  const handleRemoveMember = async (groupId: number, userId: number) => {
    try { await api.removeGroupMember(groupId, userId); await reloadGroups() } catch { setError(t.settings.opFailed) }
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
        await api.register({
          email,
          username,
          password,
          website, // honeypot
          ...(captchaRequired ? { captcha_id: captcha?.captcha_id, captcha_answer: captchaAnswer } : {}),
        })
      }
      const res = await api.login({ email, password })
      setAuthToken(res.access_token)
      setUserRole(res.user.role)
      setJwt(res.access_token)
      setPassword('')
      notify(t.auth.loggedIn)
      // Leave any reset context (e.g. the user remembered their password
      // mid-reset and logged in from it) so the account panel renders.
      navigate('/auth', { replace: true })
      await refreshProducts()
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errors.authFailed)
      // A failed captcha is single-use — clearing it triggers the effect
      // above to fetch a fresh challenge.
      if (mode === 'register' && captchaRequired) {
        setCaptcha(null)
        setCaptchaAnswer('')
      }
    }
  }

  // Extracts the huma error "detail" (invalid token, weak password, …) from
  // the raw error body thrown by request(); falls back to the raw message.
  const errorDetail = (err: unknown, fallback: string): string => {
    if (err instanceof Error) {
      try {
        const parsed = JSON.parse(err.message) as { detail?: string }
        if (parsed && typeof parsed.detail === 'string' && parsed.detail.trim() !== '') {
          return parsed.detail
        }
      } catch {
        // not a huma error body — fall through to the raw message
      }
      if (err.message) return err.message
    }
    return fallback
  }

  const handleResetRequest = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setNotice(null)
    try {
      // The API always responds 200 (also for unknown emails), so the UI
      // shows the same optimistic message and never confirms which
      // addresses exist (no account enumeration).
      await api.requestPasswordReset({ email })
      setNotice(t.auth.resetRequestSent)
      setMode('login')
    } catch (err) {
      setError(errorDetail(err, t.errors.authFailed))
    }
  }

  const handleResetConfirm = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setNotice(null)
    if (resetPassword !== resetPasswordRepeat) {
      setError(t.auth.resetPasswordsDontMatch)
      return
    }
    const token = searchParams.get('token')
    if (!token) {
      setError(t.auth.resetTokenInvalid)
      return
    }
    try {
      await api.confirmPasswordReset({ token, new_password: resetPassword })
      setNotice(t.auth.resetDone)
      setMode('login')
      navigate('/auth', { replace: true })
    } catch (err) {
      setError(errorDetail(err, t.auth.resetFailed))
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
    if (!window.confirm(t.auth.confirmRevoke)) return
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
      setNotice(t.auth.passwordUpdated)
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
      setNotice(t.auth.userUpdatedToast(u.email))
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errors.authFailed)
    }
  }

  const handleLogout = () => {
    setAuthToken(null)
    setUserRole(null)
    setJwt(null)
    setGeneratedToken(null)
    setTokens([])
    setUsers([])
    setIsAdmin(false)
    void refreshProducts()
  }

  return (
    <section style={{ maxWidth: 820, display: 'grid', gap: '1.25rem' }}>
      <h2 style={{ margin: 0 }}>{t.auth.title}</h2>
      {error && <p className="alert-error">{error}</p>}
      {notice && <p className="alert-ok">{notice}</p>}

      {!jwt || inResetContext ? (
        mode === 'reset-request' ? (
          <form onSubmit={handleResetRequest} className="card" style={{ maxWidth: 460, display: 'grid', gap: '0.65rem' }}>
            <div style={{ display: 'flex', gap: '0.5rem' }}>
              <h3 style={{ margin: 0 }}>{t.auth.resetRequestTitle}</h3>
            </div>
            <p style={{ color: 'var(--muted)', fontSize: '0.85rem', margin: 0 }}>{t.auth.resetRequestDescription}</p>
            <label htmlFor="reset-email" style={{ display: 'grid', gap: '0.25rem', fontSize: '0.85rem' }}>
              <span>{t.auth.resetEmailPlaceholder}</span>
              <input
                id="reset-email"
                name="email"
                className="input"
                type="email"
                autoComplete="username"
                spellCheck={false}
                enterKeyHint="next"
                placeholder={t.auth.resetEmailPlaceholder}
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
              />
            </label>
            <button type="submit" className="btn btn-primary"><Mail size={14} /> {t.auth.resetRequestSubmit}</button>
            <button
              type="button"
              className="btn btn-sm"
              onClick={() => {
                setMode('login')
                setError(null)
                setNotice(null)
              }}
            >
              {t.auth.resetBackToLogin}
            </button>
          </form>
        ) : mode === 'reset-confirm' ? (
          searchParams.get('token') ? (
            <form onSubmit={handleResetConfirm} className="card" style={{ maxWidth: 460, display: 'grid', gap: '0.65rem' }}>
              <div style={{ display: 'flex', gap: '0.5rem' }}>
                <h3 style={{ margin: 0 }}>{t.auth.resetConfirmTitle}</h3>
              </div>
              <p style={{ color: 'var(--muted)', fontSize: '0.85rem', margin: 0 }}>{t.auth.resetConfirmDescription}</p>
              <label htmlFor="reset-new-password" style={{ display: 'grid', gap: '0.25rem', fontSize: '0.85rem' }}>
                <span>{t.auth.resetNewPassword}</span>
                <input
                  id="reset-new-password"
                  name="new-password"
                  className="input"
                  type={showConfirmPassword ? 'text' : 'password'}
                  autoComplete="new-password"
                  enterKeyHint="next"
                  placeholder={t.auth.passwordPlaceholder}
                  value={resetPassword}
                  onChange={(e) => setResetPassword(e.target.value)}
                  required
                  minLength={8}
                />
                <button
                  type="button"
                  className="btn btn-sm"
                  style={{ marginTop: '0.25rem' }}
                  onClick={() => { const p = generatePassword(); setResetPassword(p); setResetPasswordRepeat(p); setShowConfirmPassword(true) }}
                >
                  {t.auth.generatePassword}
                </button>
              </label>
              <label htmlFor="reset-new-password-repeat" style={{ display: 'grid', gap: '0.25rem', fontSize: '0.85rem' }}>
                <span>{t.auth.resetNewPasswordRepeat}</span>
                <input
                  id="reset-new-password-repeat"
                  name="new-password-repeat"
                  className="input"
                  type={showConfirmPassword ? 'text' : 'password'}
                  autoComplete="new-password"
                  enterKeyHint="done"
                  placeholder={t.auth.passwordPlaceholder}
                  value={resetPasswordRepeat}
                  onChange={(e) => setResetPasswordRepeat(e.target.value)}
                  required
                  minLength={8}
                />
              </label>
              <button
                type="button"
                className="btn btn-sm"
                onClick={() => setShowConfirmPassword((v) => !v)}
                aria-pressed={showConfirmPassword}
              >
                {showConfirmPassword ? t.common.hide : t.common.show}
              </button>
              <button type="submit" className="btn btn-primary"><KeyRound size={14} /> {t.auth.resetConfirmSubmit}</button>
              <button
                type="button"
                className="btn btn-sm"
                onClick={() => {
                  setMode('login')
                  setError(null)
                  setNotice(null)
                  navigate('/auth', { replace: true })
                }}
              >
                {t.auth.resetBackToLogin}
              </button>
            </form>
          ) : (
            <div className="card" style={{ maxWidth: 460, display: 'grid', gap: '0.65rem' }}>
              <p className="alert-error" style={{ margin: 0 }}>{t.auth.resetTokenInvalid}</p>
              <button
                type="button"
                className="btn btn-sm"
                onClick={() => {
                  setMode('login')
                  setError(null)
                  setNotice(null)
                  navigate('/auth', { replace: true })
                }}
              >
                {t.auth.resetBackToLogin}
              </button>
            </div>
          )
        ) : (
          <form onSubmit={handleAuth} className="card" style={{ maxWidth: 460, display: 'grid', gap: '0.65rem' }}>
          <div style={{ display: 'flex', gap: '0.5rem' }}>
            <button type="button" onClick={() => setMode('login')} className={mode === 'login' ? 'btn btn-primary btn-sm' : 'btn btn-sm'}>
              {t.auth.login}
            </button>
            {registrationAllowed && (
              <button type="button" onClick={() => setMode('register')} className={mode === 'register' ? 'btn btn-primary btn-sm' : 'btn btn-sm'}>
                {t.auth.register}
              </button>
            )}
          </div>
          {mode === 'register' && !registrationAllowed && (
            <p className="alert-error" style={{ margin: 0 }}>{t.auth.registrationDisabled}</p>
          )}
          <label htmlFor="auth-email" style={{ display: 'grid', gap: '0.25rem', fontSize: '0.85rem' }}>
            <span>{t.common.email}</span>
            <input
              id="auth-email"
              name="email"
              className="input"
              type="email"
              autoComplete="username"
              spellCheck={false}
              enterKeyHint="next"
              placeholder={t.common.email}
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
          {mode === 'register' && captchaRequired && captcha && (
            <label htmlFor="auth-captcha" style={{ display: 'grid', gap: '0.25rem', fontSize: '0.85rem' }}>
              <span>{t.auth.captchaLabel}: {captcha.question}</span>
              <input
                id="auth-captcha"
                name="captcha_answer"
                className="input"
                inputMode="numeric"
                autoComplete="off"
                placeholder={t.auth.captchaPlaceholder}
                title={t.auth.captchaHint}
                value={captchaAnswer}
                onChange={(e) => setCaptchaAnswer(e.target.value)}
                required
              />
            </label>
          )}
          {mode === 'register' && (
            <input
              type="text"
              name="website"
              autoComplete="off"
              tabIndex={-1}
              aria-hidden="true"
              style={{ position: 'absolute', left: -9999, width: 1, height: 1, opacity: 0 }}
              value={website}
              onChange={(e) => setWebsite(e.target.value)}
            />
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
              {mode !== 'login' && (
                <button
                  type="button"
                  className="btn btn-sm"
                  title={t.auth.generatePassword}
                  onClick={() => { const p = generatePassword(); setPassword(p); setShowPassword(true) }}
                >
                  {t.auth.generatePassword}
                </button>
              )}
              <button
                type="button"
                className="btn btn-sm"
                onClick={() => setShowPassword((v) => !v)}
                aria-pressed={showPassword}
              >
                {showPassword ? t.common.hide : t.common.show}
              </button>
            </div>
          </label>
          <button type="submit" className="btn btn-primary">
            {mode === 'login' ? t.auth.submitLogin : t.auth.submitRegister}
          </button>
          <button
            type="button"
            className="btn btn-ghost btn-sm"
            onClick={() => {
              setMode('reset-request')
              setError(null)
              setNotice(null)
            }}
          >
            {t.auth.forgotPassword}
          </button>
          {ssoEnabled && mode === 'login' && (
            <a href="/api/v1/auth/sso/login" className="btn btn-sm" style={{ textAlign: 'center' }}>
              {t.auth.ssoLogin} {ssoProvider}
            </a>
          )}
          {ldapEnabled && mode === 'login' && (
            <form onSubmit={handleLdapLogin} style={{ display: 'grid', gap: '0.5rem', borderTop: '1px solid var(--border)', paddingTop: '0.75rem' }}>
              <strong>{t.auth.ldapLogin}</strong>
              <input className="input" placeholder={t.auth.usernamePlaceholder} value={ldapUsername} onChange={(e) => setLdapUsername(e.target.value)} required />
              <input className="input" type="password" placeholder={t.auth.passwordPlaceholder} value={ldapPassword} onChange={(e) => setLdapPassword(e.target.value)} required />
              <button type="submit" className="btn btn-primary btn-sm"><LogIn size={13} /> {t.auth.submitLogin}</button>
            </form>
          )}
          </form>
        )
      ) : (
        <div style={{ display: 'grid', gap: '1rem' }}>
          <div className="card" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <span>{t.auth.loggedIn}</span>
            <button type="button" onClick={handleLogout} className="btn btn-sm"><LogOut size={13} /> {t.auth.logout}</button>
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(320px, 1fr))', gap: '1rem' }}>
            <form onSubmit={handleCreateToken} className="card" style={{ display: 'grid', gap: '0.6rem', alignContent: 'start' }}>
              <strong>{t.auth.tokenTitle}</strong>
              <div style={{ display: 'flex', gap: '0.5rem' }}>
                <input className="input" placeholder={t.auth.tokenPlaceholder} value={tokenName} onChange={(e) => setTokenName(e.target.value)} style={{ flex: 1 }} />
                <button type="submit" className="btn btn-primary btn-sm"><KeyRound size={13} /> {t.auth.generate}</button>
              </div>
              {generatedToken && (
                <div style={{ padding: '0.55rem', background: 'var(--bg-subtle)', border: '1px solid var(--border)', borderRadius: 8, fontFamily: 'var(--font-mono)', fontSize: '0.8rem', wordBreak: 'break-all' }}>
                  {generatedToken}
                </div>
              )}
              {tokens.length > 0 && (
                <table className="data-table">
                  <thead>
                    <tr>
                      <th>{t.common.name}</th>
                      <th>{t.auth.lastUsed}</th>
                      <th />
                    </tr>
                  </thead>
                  <tbody>
                    {tokens.map((tok) => (
                      <tr key={tok.id}>
                        <td>{tok.name}</td>
                        <td style={{ color: 'var(--muted)' }}>{tok.last_used_at ? new Date(tok.last_used_at).toLocaleDateString() : t.auth.never}</td>
                        <td style={{ textAlign: 'right' }}>
                          <button type="button" className="btn btn-danger btn-sm" onClick={() => void handleRevokeToken(tok.id)}>
                            {t.auth.revoke}
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </form>

            <form onSubmit={handleChangePassword} className="card" style={{ display: 'grid', gap: '0.6rem', alignContent: 'start' }}>
              <strong>{t.auth.changePassword}</strong>
              <label htmlFor="current-password" style={{ display: 'grid', gap: '0.25rem', fontSize: '0.85rem' }}>
                <span>{t.auth.currentPassword}</span>
                <input
                  id="current-password"
                  name="current-password"
                  className="input"
                  type="password"
                  autoComplete="current-password"
                  enterKeyHint="next"
                  placeholder={t.auth.currentPassword}
                  value={currentPassword}
                  onChange={(e) => setCurrentPassword(e.target.value)}
                  required
                  minLength={8}
                />
              </label>
              <label htmlFor="new-password" style={{ display: 'grid', gap: '0.25rem', fontSize: '0.85rem' }}>
                <span>{t.auth.newPassword}</span>
                <input
                  id="new-password"
                  name="new-password"
                  className="input"
                  type="password"
                  autoComplete="new-password"
                  enterKeyHint="done"
                  placeholder={t.auth.passwordPlaceholder}
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  required
                  minLength={8}
                />
                <button
                  type="button"
                  className="btn btn-sm"
                  style={{ marginTop: '0.25rem' }}
                  onClick={() => setNewPassword(generatePassword())}
                >
                  {t.auth.generatePassword}
                </button>
              </label>
              <button type="submit" className="btn btn-primary btn-sm"><KeyRound size={13} /> {t.auth.updatePassword}</button>
            </form>
          </div>

          {isAdmin && (
            <div className="card" style={{ display: 'grid', gap: '0.6rem' }}>
              <strong>{t.auth.adminUsers} ({users.length})</strong>
              <table className="data-table">
                <thead>
                  <tr>
                    <th>{t.common.id}</th>
                    <th>{t.common.email}</th>
                    <th>{t.auth.usernamePlaceholder}</th>
                    <th>{t.auth.role}</th>
                    <th>{t.auth.active}</th>
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
                          {u.is_active ? t.auth.active : t.auth.disabled}
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          {isAdmin && (
            <div className="card" style={{ display: 'grid', gap: '0.6rem' }}>
              <strong>{t.auth.adminGroups} ({groups.length})</strong>
              <form onSubmit={handleCreateGroup} style={{ display: 'flex', gap: '0.5rem' }}>
                <input className="input" placeholder={t.auth.groupNamePlaceholder} value={newGroupName} onChange={(e) => setNewGroupName(e.target.value)} style={{ flex: 1 }} />
                <button type="submit" className="btn btn-primary btn-sm"><Plus size={13} /> {t.auth.addGroup}</button>
              </form>
              {groups.map((g) => (
                <div key={g.id} style={{ display: 'grid', gap: '0.4rem', borderTop: '1px solid var(--border)', paddingTop: '0.5rem' }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <strong>{g.name}</strong>
                    <button type="button" className="btn btn-danger btn-sm" onClick={() => void handleDeleteGroup(g.id)}>{t.common.delete}</button>
                  </div>
                  <div style={{ display: 'flex', gap: '0.4rem', flexWrap: 'wrap' }}>
                    {(g.members ?? []).map((uid) => {
                      const u = users.find((x) => x.id === uid)
                      return (
                        <span key={uid} className="badge" style={{ display: 'inline-flex', gap: '0.3rem', alignItems: 'center' }}>
                          {u?.username ?? `#${uid}`}
                          <button type="button" className="btn btn-ghost btn-sm" onClick={() => void handleRemoveMember(g.id, uid)}>×</button>
                        </span>
                      )
                    })}
                  </div>
                  <select
                    className="select"
                    value=""
                    onChange={(e) => { const uid = Number(e.target.value); if (uid) void handleAddMember(g.id, uid) }}
                  >
                    <option value="">{t.products.assignMember}</option>
                    {users.filter((u) => !(g.members ?? []).includes(u.id)).map((u) => (
                      <option key={u.id} value={u.id}>{u.username}</option>
                    ))}
                  </select>
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </section>
  )
}
