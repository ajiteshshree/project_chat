import { useState } from 'react'
import { api, fileUrl } from '../api.js'

function Avatar({ user, size = 40 }) {
  return (
    <div className="avatar" style={{ width: size, height: size }}>
      {user.profile_pic ? (
        <img src={fileUrl(user.profile_pic)} alt="" loading="lazy" />
      ) : (
        <span>{user.username?.[0]?.toUpperCase() || '?'}</span>
      )}
    </div>
  )
}

export default function Sidebar({
  me,
  conversations,
  active,
  onSelect,
  onOpenProfile,
  onLogout,
  onConversationCreated,
}) {
  const [searching, setSearching] = useState(false)
  const [query, setQuery] = useState('')
  const [results, setResults] = useState([])
  const [error, setError] = useState('')

  const search = async (q) => {
    setQuery(q)
    if (!q.trim()) {
      setResults([])
      setSearching(false)
      return
    }
    setSearching(true)
    setResults(await api.searchUsers(q.trim()).catch(() => []))
  }

  const startDm = async (username) => {
    try {
      const conv = await api.createConversation(username)
      setQuery('')
      setResults([])
      setSearching(false)
      onConversationCreated(conv)
    } catch (e) {
      setError(e.message)
    }
  }

  const preview = (c) => {
    const m = c.last_message
    if (!m) return 'No messages yet'
    const prefix = m.sender_username === me.username ? 'You: ' : m.type === 'text' ? m.sender_username + ': ' : ''
    if (m.type === 'image') return prefix || 'Image'
    if (m.type === 'video') return prefix || 'Video'
    return prefix + m.content
  }

  const dmOther = (c) => c.members?.[0] || null

  return (
    <aside className="sidebar">
      <div className="me-row" onClick={onOpenProfile} role="button">
        <Avatar user={me} />
        <div className="me-info">
          <div className="me-name">{me.username}</div>
          {me.status ? <div className="me-status">{me.status}</div> : <div className="muted">set a status</div>}
        </div>
        <button className="icon-btn" title="Log out" onClick={(e) => { e.stopPropagation(); onLogout() }}>
          ⇥
        </button>
      </div>

      <div className="search-box">
        <input
          value={query}
          onChange={(e) => search(e.target.value)}
          placeholder="Find a user…"
          autoCapitalize="none"
        />
      </div>
      {searching && (
        <div className="search-results">
          {results.length === 0 && <div className="muted">No users found</div>}
          {results.map((u) => (
            <button key={u.id} className="result-row" onClick={() => startDm(u.username)}>
              <Avatar user={u} size={32} />
              <div className="result-name">
                <div>{u.username}</div>
                <span className={u.online ? 'dot on' : 'dot'} />
              </div>
            </button>
          ))}
          {error && <div className="form-error">{error}</div>}
        </div>
      )}

      <nav className="conv-list">
        {conversations.map((c) => {
          const other = c.kind === 'dm' ? dmOther(c) : null
          const label =
            c.kind === 'lobby' ? `Lobby · ${c.online_count || 0} online` : other ? other.username : c.name || '?'
          const sub = c.kind === 'lobby' ? 'Everyone' : other?.status || 'Group'
          const online = c.kind === 'dm' ? other?.online : true
          return (
            <button
              key={c.id}
              className={`conv-row ${active === c.id ? 'active' : ''}`}
              onClick={() => onSelect(c)}
            >
              {c.kind === 'lobby' ? (
                <div className="avatar lobby-avatar">#</div>
              ) : (
                <Avatar user={other || { username: '?' }} size={40} />
              )}
              <div className="conv-info">
                <div className="conv-name">{label}</div>
                <div className="conv-preview">{preview(c)}</div>
              </div>
              {c.kind === 'dm' && <span className={online ? 'dot on' : 'dot'} />}
            </button>
          )
        })}
      </nav>
    </aside>
  )
}