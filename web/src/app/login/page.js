'use client'
import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { api, setSession } from '../lib/api'

// Login SISWA — NIS + password. Petugas (guru/admin) punya halaman sendiri.
export default function LoginSiswa() {
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
      if (r.user.role === 'siswa') {
        setSession(r.token, r.user)
        router.replace('/siswa')
      } else {
        setErr('Ini akun petugas (guru/admin). Gunakan menu "Login Guru & Admin".')
      }
    } catch (e) {
      setErr(e.message === 'Unauthorized' ? 'NIS atau password salah.' : e.message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="auth-wrap">
      <div className="auth-card">
        <div className="auth-logo">11</div>
        <h1>SMK 11 MARET</h1>
        <span className="auth-badge">Portal Siswa</span>
        <div className="auth-sub">Masuk dengan NIS untuk akses materi, tugas, absensi & ujian</div>

        <form onSubmit={submit} autoComplete="off">
          <div className="field">
            <label htmlFor="nis">NIS</label>
            <input type="text" id="nis" value={form.username} autoFocus
              placeholder="Nomor Induk Siswa" required
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
          <a href="/login/petugas">👨‍🏫 Login Guru & Admin →</a>
        </div>
      </div>
    </div>
  )
}