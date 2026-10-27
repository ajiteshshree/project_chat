import { useRef, useState } from 'react'
import { api, fileUrl } from '../api.js'

export default function ProfileModal({ me, onClose, onSaved }) {
  const [status, setStatus] = useState(me.status || '')
  const [uploading, setUploading] = useState(false)
  const [error, setError] = useState('')
  const fileRef = useRef(null)

  const save = async () => {
    setError('')
    try {
      await api.setStatus(status.trim())
      onSaved()
    } catch (e) {
      setError(e.message)
    }
  }

  const upload = async (file) => {
    if (!file) return
    if (file.size > 25 * 1024 * 1024) {
      setError('image too large (max 25 MB)')
      return
    }
    setUploading(true)
    try {
      await api.uploadAvatar(file)
      onSaved()
    } catch (e) {
      setError(e.message)
    } finally {
      setUploading(false)
    }
  }

  return (
    <div className="modal-wrap">
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <button className="modal-x" onClick={onClose}>×</button>
        <h2>Profile</h2>
        <div className="profile-avatar">
          {me.profile_pic ? (
            <img src={fileUrl(me.profile_pic)} alt="" />
          ) : (
            <span className="profile-initial">{me.username?.[0]?.toUpperCase() || '?'}</span>
          )}
        </div>
        <button className="link-btn" onClick={() => fileRef.current?.click()} disabled={uploading}>
          {uploading ? 'Uploading…' : me.profile_pic ? 'Change picture' : 'Add picture'}
        </button>
        <input
          ref={fileRef}
          type="file"
          hidden
          accept="image/*"
          onChange={(e) => upload(e.target.files?.[0])}
        />
        <label className="field-label">Status</label>
        <input value={status} maxLength={200} onChange={(e) => setStatus(e.target.value)} placeholder="e.g. busy 📵" />
        {error && <div className="form-error">{error}</div>}
        <button className="send-btn" onClick={save}>Save</button>
      </div>
    </div>
  )
}