import { useCallback, useEffect, useRef, useState } from 'react'
import { api, wsUrl } from '../api.js'
import Sidebar from './Sidebar.jsx'
import Thread from './Thread.jsx'
import ProfileModal from './ProfileModal.jsx'

export default function Chat({ me, onLogout }) {
  const [conversations, setConversations] = useState([])
  const [active, setActive] = useState(null)
  const [messages, setMessages] = useState([])
  const [hasMore, setHasMore] = useState(true)
  const [loadingMore, setLoadingMore] = useState(false)
  const [typing, setTyping] = useState({})
  const [showProfile, setShowProfile] = useState(false)
  const [connected, setConnected] = useState(false)
  const [showSidebar, setShowSidebar] = useState(true)
  const wsRef = useRef(null)
  const activeRef = useRef(null)
  const messagesRef = useRef(null)
  const typingTimers = useRef({})

  const refreshConversations = useCallback(() => {
    api.conversations().then((res) => setConversations(Array.isArray(res) ? res : [])).catch(() => setConversations([]))
  }, [])

  const openConversation = useCallback((conv) => {
    setActive(conv.id)
    activeRef.current = conv.id
    setMessages([])
    messagesRef.current = []
    setHasMore(true)
    setTyping({})
    setShowSidebar(false)
    api
      .messages(conv.id)
      .then((msgs) => {
        if (activeRef.current !== conv.id) return
        const arr = Array.isArray(msgs) ? msgs : []
        setMessages(arr)
        messagesRef.current = arr
        setHasMore(arr.length === 50)
      })
      .catch(() => {})
    const ws = wsRef.current
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'join', conversation_id: conv.id }))
    }
  }, [])

  useEffect(() => {
    refreshConversations()
  }, [refreshConversations])

  useEffect(() => {
    let closed = false
    let retryTimer = null
    const connect = () => {
      const ws = new WebSocket(wsUrl())
      wsRef.current = ws
      ws.onopen = () => {
        setConnected(true)
        if (activeRef.current) {
          ws.send(JSON.stringify({ type: 'join', conversation_id: activeRef.current }))
        }
      }
      ws.onclose = () => {
        setConnected(false)
        if (!closed && document.visibilityState === 'visible') {
          retryTimer = setTimeout(connect, 3000)
        }
      }
      ws.onmessage = (ev) => {
        const data = JSON.parse(ev.data)
        if (data.type === 'message') {
          const m = data.message
          if (m.conversation_id === activeRef.current) {
            setMessages((prev) => {
              const next = [...prev, m]
              messagesRef.current = next
              return next
            })
          } else {
            refreshConversations()
          }
        } else if (data.type === 'presence') {
          refreshConversations()
        } else if (data.type === 'typing') {
          if (data.conversation_id === activeRef.current) {
            const key = data.sender_username
            setTyping((prev) => ({ ...prev, [key]: data.is_typing }))
            clearTimeout(typingTimers.current[key])
            if (data.is_typing) {
              typingTimers.current[key] = setTimeout(() => {
                setTyping((prev) => ({ ...prev, [key]: false }))
              }, 3500)
            }
          }
        }
      }
    }
    const onVisible = () => {
      const ws = wsRef.current
      if (document.visibilityState === 'visible' && (!ws || ws.readyState !== WebSocket.OPEN) && !closed) {
        connect()
      }
    }
    document.addEventListener('visibilitychange', onVisible)
    connect()
    return () => {
      closed = true
      clearTimeout(retryTimer)
      document.removeEventListener('visibilitychange', onVisible)
      wsRef.current?.close()
    }
  }, [refreshConversations])

  useEffect(() => {
    // default to the lobby once loaded
    if (active === null && conversations.length > 0) {
      const lobby = conversations.find((c) => c.kind === 'lobby')
      openConversation(lobby || conversations[0])
    }
  }, [conversations, active, openConversation])

  const loadOlder = useCallback(() => {
    if (!activeRef.current || loadingMore || !hasMore) return
    setLoadingMore(true)
    const convId = activeRef.current
    const before = messagesRef.current[0]?.id || 0
    api
      .messages(convId, before)
      .then((older) => {
        if (activeRef.current !== convId) return
        const arr = Array.isArray(older) ? older : []
        setMessages((prev) => {
          const merged = [...arr, ...prev]
          messagesRef.current = merged
          return merged
        })
        setHasMore(arr.length === 50)
      })
      .catch(() => {})
      .finally(() => setLoadingMore(false))
  }, [loadingMore, hasMore])

  const sendMessage = useCallback((opts) => {
    const ws = wsRef.current
    if (!ws || ws.readyState !== WebSocket.OPEN) return
    ws.send(JSON.stringify({ type: 'message', conversation_id: activeRef.current, ...opts }))
  }, [])

  const sendTyping = useCallback((isTyping) => {
    const ws = wsRef.current
    if (!ws || ws.readyState !== WebSocket.OPEN) return
    ws.send(JSON.stringify({ type: 'typing', conversation_id: activeRef.current, is_typing: isTyping }))
  }, [])

  const activeConv = conversations.find((c) => c.id === active) || null
  const goBack = useCallback(() => {
    setShowSidebar(true)
    setActive(null)
    activeRef.current = null
  }, [])

  return (
    <div className={`layout ${showSidebar ? 'show-sidebar' : 'show-thread'}`}>
      <Sidebar
        me={me}
        conversations={conversations}
        active={active}
        onSelect={openConversation}
        onOpenProfile={() => setShowProfile(true)}
        onLogout={onLogout}
        onConversationCreated={(conv) => {
          refreshConversations()
          openConversation(conv)
        }}
      />
      {activeConv ? (
        <Thread
          conv={activeConv}
          me={me}
          connected={connected}
          messages={messages}
          typing={typing}
          hasMore={hasMore}
          loadingMore={loadingMore}
          onLoadOlder={loadOlder}
          onSend={({ content, fileUrl, type }) =>
            sendMessage({ content: content || '', file_url: fileUrl || '', msg_type: type || 'text' })
          }
          onTyping={sendTyping}
          onBack={goBack}
        />
      ) : (
        <div className="thread empty-thread">
          <div className="center-screen empty">Pick a conversation</div>
        </div>
      )}
      {showProfile && (
        <ProfileModal
          me={me}
          onClose={() => setShowProfile(false)}
          onSaved={(updated) => {
            refreshConversations()
            setShowProfile(false)
          }}
        />
      )}
    </div>
  )
}