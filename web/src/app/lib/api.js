'use client'

// Klien API minimal utk backend Go. Token disimpan di localStorage,
// state jawaban ujian juga disimpan lokal (master plan: sinkronisasi berkala).
export const API = '/api'

export function getToken() {
  if (typeof window === 'undefined') return ''
  return localStorage.getItem('token') || ''
}

export function getUser() {
  if (typeof window === 'undefined') return null
  const u = localStorage.getItem('user')
  try { return JSON.parse(u) } catch { return null }
}

export function setSession(token, user) {
  localStorage.setItem('token', token)
  localStorage.setItem('user', JSON.stringify(user))
}

export function clearSession() {
  localStorage.removeItem('token')
  localStorage.removeItem('user')
  localStorage.removeItem('exam-answers')
}

async function request(path, { method = 'GET', body, form, noAuth = false } = {}) {
  const headers = {}
  const t = getToken()
  if (t && !noAuth) headers['Authorization'] = `Bearer ${t}`
  let payload
  if (form) {
    payload = form
    // FormData: jangan set Content-Type manual (boundary otomatis)
  } else if (body !== undefined) {
    headers['Content-Type'] = 'application/json'
    payload = JSON.stringify(body)
  }
  const res = await fetch(API + path, { method, headers, body: payload })
  if (res.status === 401) {
    clearSession()
    if (location.pathname !== '/login') location.href = '/login'
    throw new Error('Sesi berakhir, silakan login ulang')
  }
  const ct = res.headers.get('content-type') || ''
  if (!res.ok) {
    const t = await res.text()
    throw new Error(t || `HTTP ${res.status}`)
  }
  if (ct.includes('application/json')) return res.json()
  return res
}

export const api = {
  get: (p) => request(p),
  post: (p, body) => request(p, { method: 'POST', body }),
  postForm: (p, form) => request(p, { method: 'POST', form }),
  del: (p) => request(p, { method: 'DELETE' }),
  login: (username, password) => request('/login', { method: 'POST', body: { username, password }, noAuth: true }),
  news: () => request('/news', { noAuth: true }),
  download: async (path) => {
    // export xlsx butuh auth header
    const headers = {}
    const t = getToken()
    if (t) headers['Authorization'] = `Bearer ${t}`
    const res = await fetch(API + path, { headers })
    if (!res.ok) throw new Error('Gagal mengunduh')
    const blob = await res.blob()
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = ''
    a.click()
    URL.revokeObjectURL(url)
  },
}