import { useState } from 'react'
import { api, getAuthToken, setAuthToken } from '../../api/client'
import { useI18n } from '../../i18n'

export function AuthView() {
  const { t } = useI18n()
  const [mode, setMode] = useState<'login' | 'register'>('login')
  const [email, setEmail] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [tokenName, setTokenName] = useState('ci-runner')
  const [generatedToken, setGeneratedToken] = useState<string | null>(null)
  const [jwt, setJwt] = useState<string | null>(getAuthToken())
  const [error, setError] = useState<string | null>(null)

  const handleAuth = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
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
    try {
      const res = await api.createApiToken(tokenName.trim() || 'ci-runner')
      setGeneratedToken(res.token)
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errors.createToken)
    }
  }

  const handleLogout = () => {
    setAuthToken(null)
    setJwt(null)
    setGeneratedToken(null)
  }

  return (
    <section style={{ maxWidth: 520 }}>
      <h2>{t.auth.title}</h2>
      {error && <p style={{ color: '#b91c1c' }}>{error}</p>}

      {!jwt ? (
        <form onSubmit={handleAuth} style={{ display: 'grid', gap: '0.6rem', border: '1px solid #e5e7eb', padding: '1rem', borderRadius: 8 }}>
          <div style={{ display: 'flex', gap: '0.5rem' }}>
            <button type="button" onClick={() => setMode('login')} style={{ fontWeight: mode === 'login' ? 700 : 400 }}>{t.auth.login}</button>
            <button type="button" onClick={() => setMode('register')} style={{ fontWeight: mode === 'register' ? 700 : 400 }}>{t.auth.register}</button>
          </div>
          <input type="email" placeholder="Email" value={email} onChange={(e) => setEmail(e.target.value)} required style={{ padding: '0.4rem', borderRadius: 6, border: '1px solid #d1d5db' }} />
          {mode === 'register' && (
            <input placeholder={t.auth.usernamePlaceholder} value={username} onChange={(e) => setUsername(e.target.value)} required style={{ padding: '0.4rem', borderRadius: 6, border: '1px solid #d1d5db' }} />
          )}
          <input type="password" placeholder={t.auth.passwordPlaceholder} value={password} onChange={(e) => setPassword(e.target.value)} required minLength={8} style={{ padding: '0.4rem', borderRadius: 6, border: '1px solid #d1d5db' }} />
          <button type="submit" style={{ padding: '0.45rem', borderRadius: 6, background: '#111827', color: '#fff', border: 'none', cursor: 'pointer' }}>
            {mode === 'login' ? t.auth.submitLogin : t.auth.submitRegister}
          </button>
        </form>
      ) : (
        <div style={{ display: 'grid', gap: '1rem' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '0.75rem', border: '1px solid #e5e7eb', borderRadius: 8 }}>
            <span>{t.auth.loggedIn}</span>
            <button type="button" onClick={handleLogout} style={{ padding: '0.3rem 0.6rem', borderRadius: 6, border: '1px solid #d1d5db', background: '#fff', cursor: 'pointer' }}>{t.auth.logout}</button>
          </div>

          <form onSubmit={handleCreateToken} style={{ display: 'grid', gap: '0.5rem', border: '1px solid #e5e7eb', padding: '1rem', borderRadius: 8 }}>
            <strong>{t.auth.tokenTitle}</strong>
            <div style={{ display: 'flex', gap: '0.5rem' }}>
              <input placeholder={t.auth.tokenPlaceholder} value={tokenName} onChange={(e) => setTokenName(e.target.value)} style={{ flex: 1, padding: '0.4rem', borderRadius: 6, border: '1px solid #d1d5db' }} />
              <button type="submit" style={{ padding: '0.4rem 0.8rem', borderRadius: 6, background: '#111827', color: '#fff', border: 'none', cursor: 'pointer' }}>{t.auth.generate}</button>
            </div>
            {generatedToken && (
              <div style={{ padding: '0.5rem', background: '#f3f4f6', borderRadius: 6, fontFamily: 'monospace', wordBreak: 'break-all' }}>
                {generatedToken}
              </div>
            )}
          </form>
        </div>
      )}
    </section>
  )
}
