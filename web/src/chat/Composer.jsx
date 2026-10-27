import { useEffect, useRef, useState } from 'react'
import { api } from '../api.js'
import EmojiButton from './EmojiButton.jsx'

export default function Composer({ disabled, uploading, setUploading, onSend, onTyping }) {
  const [text, setText] = useState('')
  const [shook, setShook] = useState(false)
  const inputRef = useRef(null)
  const fileRef = useRef(null)
  const typingSent = useRef(false)

  useEffect(() => {
    if (shook) {
      const t = setTimeout(() => setShook(false), 3000)
      return () => clearTimeout(t)
    }
  }, [shook])

  const flagTyping = (nextText) => {
    if (!typingSent.current && nextText.length > 0) {
      typingSent.current = true
      onTyping(true)
    }
    if (nextText.length === 0 && typingSent.current) {
      typingSent.current = false
      onTyping(false)
    }
  }

  const submit = () => {
    const content = text.trim()
    if (!content) return
    typingSent.current = false
    onTyping(false)
    setText('')
    onSend({ content })
    inputRef.current?.focus()
  }

  const onKeyDown = (e) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      submit()
    }
  }

  const attach = async (file) => {
    if (!file) return
    if (file.size > 25 * 1024 * 1024) {
      setShook(true)
      return
    }
    const isImage = file.type.startsWith('image/')
    const isVideo = file.type.startsWith('video/')
    if (!isImage && !isVideo) {
      setShook(true)
      return
    }
    setUploading(true)
    try {
      const { key } = await api.upload(file)
      onSend({ fileUrl: key, type: isImage ? 'image' : 'video' })
    } catch {
      setShook(true)
    } finally {
      setUploading(false)
      if (fileRef.current) fileRef.current.value = ''
    }
  }

  return (
    <footer className={`composer ${shook ? 'shook' : ''}`}>
      <input
        ref={fileRef}
        type="file"
        hidden
        accept="image/*,video/*"
        onChange={(e) => attach(e.target.files?.[0])}
      />
      <button className="icon-btn" title="Send image or video" onClick={() => fileRef.current?.click()} disabled={disabled || uploading}>
        {uploading ? '…' : '📎'}
      </button>
      <EmojiButton onPick={(emoji) => setText((t) => t + emoji)} />
      <textarea
        ref={inputRef}
        value={text}
        rows={1}
        placeholder={disabled ? 'Connecting…' : 'Message'}
        onChange={(e) => {
          setText(e.target.value)
          flagTyping(e.target.value)
        }}
        onKeyDown={onKeyDown}
        disabled={disabled}
      />
      <button className="send-btn" onClick={submit} disabled={disabled || !text.trim()}>
        Send
      </button>
    </footer>
  )
}