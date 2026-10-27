export function fmtTime(iso) {
  const d = new Date(iso)
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

export function fmtDay(iso) {
  const d = new Date(iso)
  const today = new Date()
  const sameDay = d.toDateString() === today.toDateString()
  return sameDay ? fmtTime(iso) : d.toLocaleDateString([], { month: 'short', day: 'numeric' })
}

export function fmtLastSeen(iso) {
  if (!iso) return ''
  const d = new Date(iso)
  return 'last seen ' + d.toLocaleString([], { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
}