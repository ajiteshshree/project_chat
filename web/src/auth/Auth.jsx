import { useState } from 'react'
import { api } from '../api.js'

export default function Auth({ onAuthed }) {
  const [mode, setMode] = useState('login')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async (e) => {
    e.preventDefault()
    setError('')
    setBusy(true)
    try {
      const data =
        mode === 'login'
          ? await api.login(username.trim(), password)
          : await api.register(username.trim(), password)
      onAuthed(data)
    } catch (err) {
      setError(err.message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="auth-wrap">
      <div className="auth-card">
        <h1>Chat</h1>
        <p className="auth-sub">
          {mode === 'login' ? 'Welcome back' : 'Create your account'}
        </p>
        <form onSubmit={submit}>
          <input
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            placeholder="Username"
            minLength={3}
            maxLength={32}
            required
            autoCapitalize="none"
            autoComplete="username"
          />
          <input
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            type="password"
            placeholder="Password"
            minLength={8}
            required
            autoComplete={mode === 'login' ? 'current-password' : 'new-password'}
          />
          {error && <div className="form-error">{error}</div>}
          <button type="submit" disabled={busy}>
            {busy ? '…' : mode === 'login' ? 'Log in' : 'Create account'}
          </button>
        </form>
        <button
          className="link-btn"
          onClick={() => {
            setMode(mode === 'login' ? 'register' : 'login')
            setError('')
          }}
        >
          {mode === 'login' ? 'No account? Create one' : 'Have an account? Log in'}
        </button>
      </div>
    </div>
  )
}