import { useEffect, useState } from 'react'
import Auth from './auth/Auth.jsx'
import Chat from './chat/Chat.jsx'
import { api, setToken } from './api.js'

export default function App() {
  const [me, setMe] = useState(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    api.me()
      .then((d) => {
        setToken(d?.token || localStorage.getItem('chat_token'))
        setMe(d?.user || null)
      })
      .catch(() => setMe(null))
      .finally(() => setLoading(false))
  }, [])

  const handleAuthed = (data) => {
    setToken(data.token)
    setMe(data.user)
  }

  const handleLogout = async () => {
    await api.logout().catch(() => {})
    setToken(null)
    setMe(null)
  }

  if (loading) return <div className="center-screen">Loading…</div>
  return me ? <Chat me={me} onLogout={handleLogout} /> : <Auth onAuthed={handleAuthed} />
}