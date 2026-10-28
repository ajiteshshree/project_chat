import { useEffect, useRef, useState } from 'react'
import Composer from './Composer.jsx'
import MessageRow from './MessageRow.jsx'
import { fmtDay } from '../utils.js'

export default function Thread({
  conv,
  me,
  connected,
  messages,
  typing,
  hasMore,
  loadingMore,
  onLoadOlder,
  onSend,
  onTyping,
  onBack,
}) {
  const scrollRef = useRef(null)
  const stickBottom = useRef(true)
  const [uploading, setUploading] = useState(false)

  useEffect(() => {
    if (stickBottom.current && scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
    }
  }, [messages])

  const onScroll = () => {
    const el = scrollRef.current
    if (!el) return
    stickBottom.current = el.scrollHeight - el.scrollTop - el.clientHeight < 80
    if (el.scrollTop < 40 && hasMore && !loadingMore) onLoadOlder()
  }

  const typers = Object.keys(typing).filter((k) => typing[k])
  const header = conv.kind === 'lobby' ? `Lobby · ${conv.online_count || 0} online` : conv.members?.[0]?.username || 'Chat'

  let lastDay = ''
  const rows = []
  for (const m of messages) {
    const day = new Date(m.created_at).toDateString()
    if (day !== lastDay) {
      lastDay = day
      rows.push(<div key={'d' + m.id} className="day-divider">{fmtDay(m.created_at)}</div>)
    }
    const mine = m.sender_id === me.id
    rows.push(
      <MessageRow key={m.id} m={m} mine={mine} me={me} conv={conv} />
    )
  }

  return (
    <section className="thread">
      <header className="thread-header">
        <div className="thread-header-left">
          {onBack && (
            <button className="icon-btn mobile-back" onClick={onBack} aria-label="Back">
              ←
            </button>
          )}
          {onToggleMenu && (
            <button className="icon-btn mobile-menu-btn" onClick={onToggleMenu} aria-label="Menu">
              ☰
            </button>
          )}
          <div>
            <div className="thread-title">{header}</div>
            {conv.kind === 'lobby' ? (
              <div className="muted">say hi 👋</div>
            ) : (
              <div className="muted">
                {conv.members?.[0]?.online
                  ? 'online'
                  : conv.members?.[0]?.last_online
                    ? 'last seen ' + new Date(conv.members[0].last_online).toLocaleString([], { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
                    : ''}
              </div>
            )}
          </div>
        </div>
        <span className={`conn ${connected ? 'on' : ''}`}>{connected ? '●' : 'connecting…'}</span>
      </header>

      <div className="scroller" ref={scrollRef} onScroll={onScroll}>
        {loadingMore && <div className="muted loading-more">loading older…</div>}
        {rows}
        {typers.length > 0 && (
          <div className="typing-line">{typers.join(', ')} {typers.length > 1 ? 'are' : 'is'} typing…</div>
        )}
      </div>

      <Composer
        disabled={!connected}
        uploading={uploading}
        setUploading={setUploading}
        onSend={onSend}
        onTyping={onTyping}
      />
    </section>
  )
}