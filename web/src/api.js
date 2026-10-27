const TOKEN_KEY = 'chat_token'

export const token = () => localStorage.getItem(TOKEN_KEY) || ''

export function setToken(t) {
  if (t) localStorage.setItem(TOKEN_KEY, t)
  else localStorage.removeItem(TOKEN_KEY)
}

// File URLs (img/video) need the token as a query param because browser
// media tags can't send Authorization headers.
export function fileUrl(key) {
  if (!key) return ''
  return '/files/' + key + '?token=' + encodeURIComponent(token())
}

async function request(path, options = {}) {
  const opts = {
    credentials: 'include',
    ...options,
    headers: {
      ...(options.body instanceof FormData ? {} : { 'Content-Type': 'application/json' }),
      ...(options.headers || {}),
    },
  }
  if (token()) opts.headers.Authorization = 'Bearer ' + token()
  const res = await fetch(path, opts)
  if (res.status === 204) return null
  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    const err = new Error(data.error || res.statusText || 'Request failed')
    err.status = res.status
    throw err
  }
  return data
}

export const api = {
  me: () => request('/api/me'),
  register: (username, password) =>
    request('/api/register', { method: 'POST', body: JSON.stringify({ username, password }) }),
  login: (username, password) =>
    request('/api/login', { method: 'POST', body: JSON.stringify({ username, password }) }),
  logout: () => request('/api/logout', { method: 'POST' }),
  setStatus: (status) =>
    request('/api/me', { method: 'PATCH', body: JSON.stringify({ status }) }),
  uploadAvatar: (file) => {
    const fd = new FormData()
    fd.append('file', file)
    return request('/api/me/profile-picture', { method: 'POST', body: fd })
  },
  upload: (file) => {
    const fd = new FormData()
    fd.append('file', file)
    return request('/api/upload', { method: 'POST', body: fd })
  },
  searchUsers: (q) => request('/api/users?q=' + encodeURIComponent(q)),
  conversations: () => request('/api/conversations'),
  createConversation: (username) =>
    request('/api/conversations', { method: 'POST', body: JSON.stringify({ username }) }),
  messages: (convId, before = 0) =>
    request('/api/conversations/' + convId + '/messages?limit=50&before=' + before),
}

export function wsUrl() {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  return `${proto}://${location.host}/ws?token=${encodeURIComponent(token())}`
}