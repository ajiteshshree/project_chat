import { fileUrl } from '../api.js'
import { fmtTime } from '../utils.js'

function bubble(mine, m) {
  if (m.type === 'text') return <span>{m.content}</span>
  if (m.type === 'image')
    return (
      <a href={fileUrl(m.file_url)} target="_blank" rel="noreferrer" className="media-link">
        <img className="media" src={fileUrl(m.file_url)} alt="" loading="lazy" />
      </a>
    )
  if (m.type === 'video')
    return (
      <video className="media video" src={fileUrl(m.file_url)} controls preload="metadata" />
    )
  if (m.content) return <span>{m.content}</span>
  return <span className="muted">[file]</span>
}

export default function MessageRow({ m, mine, me, conv }) {
  const showName = conv.kind === 'lobby' && !mine
  return (
    <div className={`msg-row ${mine ? 'mine' : 'theirs'}`}>
      <div className="msg-bubble">
        {showName && <div className="msg-name">{m.sender_username}</div>}
        <div className="msg-body">{bubble(mine, m)}</div>
        <div className="msg-time">{fmtTime(m.created_at)}</div>
      </div>
    </div>
  )
}