'use client'
import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { api, setSession } from '../../lib/api'

// Login PETUGAS — username + password (guru & admin digabung).
export default function LoginPetugas() {
  const router = useRouter()
  const [form, setForm] = useState({ username: '', password: '' })
  const [err, setErr] = useState('')
  const [loading, setLoading] = useState(false)

  async function submit(e) {
    e.preventDefault()
    setErr('')
    setLoading(true)
    try {
      const r = await api.login(form.username, form.password)
      if (r.user.role === 'guru') {
        setSession(r.token, r.user)
        router.replace('/guru')
      } else if (r.user.role === 'admin') {
        setSession(r.token, r.user)
        router.replace('/admin')
      } else {
        setErr('Ini akun siswa. Gunakan portal siswa.')
      }
    } catch (e) {
      setErr(e.message === 'Unauthorized' ? 'Username atau password salah.' : e.message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="auth-wrap">
      <div className="auth-card">
        <div className="auth-logo">11</div>
        <h1>SMK 11 MARET</h1>
        <span className="auth-badge">Guru &amp; Admin</span>
        <div className="auth-sub">Kelola materi, tugas, nilai, absensi & ujian</div>

        <form onSubmit={submit} autoComplete="off">
          <div className="field">
            <label htmlFor="username">Username</label>
            <input type="text" id="username" value={form.username} autoFocus
              placeholder="mis. guru.mtk atau admin" required
              onChange={(e) => setForm({ ...form, username: e.target.value })} />
          </div>
          <div className="field">
            <label htmlFor="pw">Password</label>
            <input type="password" id="pw" value={form.password} required
              placeholder="••••••••"
              onChange={(e) => setForm({ ...form, password: e.target.value })} />
          </div>
          {err && <div className="alert alert-error">{err}</div>}
          <button className="btn btn-block" type="submit" disabled={loading}>
            {loading ? 'Memeriksa…' : 'Masuk'}
          </button>
        </form>

        <div className="auth-foot">
          <a href="/login">🎓 Login Siswa →</a>
        </div>
      </div>
    </div>
  )
}