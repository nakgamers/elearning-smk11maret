'use client'
import { useEffect, useState } from 'react'

const ACCEPT = '.pdf,.doc,.docx,.ppt,.pptx,.xls,.xlsx,.png,.jpg,.jpeg,.zip,.rar,.mp4,.html,.htm'
const MAX_MB = 20

export default function Kumpul() {
  const [kelasList, setKelasList] = useState([])
  const [nama, setNama] = useState('')
  const [kelas, setKelas] = useState('')
  const [tugas, setTugas] = useState('')
  const [catatan, setCatatan] = useState('')
  const [file, setFile] = useState(null)
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')
  const [done, setDone] = useState(null)

  useEffect(() => {
    fetch('/api/kumpul/kelas')
      .then((r) => (r.ok ? r.json() : { kelas: [] }))
      .then((d) => setKelasList(d.kelas || []))
      .catch(() => setKelasList([]))
  }, [])

  const submit = async (e) => {
    e.preventDefault()
    setErr('')
    if (!nama.trim() || !kelas || !tugas.trim()) return setErr('Nama, kelas, dan judul tugas wajib diisi.')
    if (!file) return setErr('File tugas wajib dilampirkan.')
    if (file.size > MAX_MB * 1024 * 1024) return setErr(`File terlalu besar (maks ${MAX_MB}MB).`)
    const form = new FormData()
    form.append('nama', nama.trim())
    form.append('kelas', kelas)
    form.append('tugas', tugas.trim())
    form.append('catatan', catatan.trim())
    form.append('file', file)
    setBusy(true)
    try {
      const res = await fetch('/api/kumpul', { method: 'POST', body: form })
      const d = await res.json().catch(() => ({}))
      if (!res.ok) throw new Error(d.error || 'Gagal mengirim, coba lagi.')
      setDone({ nama: nama.trim(), kelas, tugas: tugas.trim(), file: file.name })
    } catch (e2) {
      setErr(e2.message)
    }
    setBusy(false)
  }

  const reset = () => {
    setDone(null)
    setFile(null)
    setTugas('')
    setCatatan('')
    setErr('')
  }

  return (
    <div style={{ minHeight: '100vh', background: '#f1f5f9', padding: '28px 14px 44px' }}>
      <div style={{ maxWidth: 560, margin: '0 auto' }}>
        <div style={{ textAlign: 'center', marginBottom: 18 }}>
          <div style={{ fontSize: 22, fontWeight: 800, color: '#1e293b' }}>Pengumpulan Tugas</div>
          <div style={{ color: '#64748b', fontSize: 13, marginTop: 4 }}>SEMART E-Learning · SMK 11 Maret</div>
        </div>

        {done ? (
          <div className="card" style={{ padding: 24, textAlign: 'center' }}>
            <div style={{
              width: 52, height: 52, borderRadius: '50%', background: '#dcfce7', color: '#16a34a',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              fontSize: 26, fontWeight: 800, margin: '0 auto 12px',
            }}>✓</div>
            <div style={{ fontWeight: 700, fontSize: 16, marginBottom: 6 }}>Tugas terkirim</div>
            <div style={{ fontSize: 13, color: '#475569', lineHeight: 1.6 }}>
              {done.nama} · {done.kelas}<br />
              {done.tugas}<br />
              File: {done.file}
            </div>
            <button className="btn btn-ghost" style={{ marginTop: 16 }} onClick={reset}>Kirim tugas lain</button>
          </div>
        ) : (
          <form className="card" style={{ padding: 20 }} onSubmit={submit}>
            <label style={{ display: 'block', fontSize: 13, fontWeight: 600, marginBottom: 6 }}>Nama lengkap</label>
            <input className="input" style={{ width: '100%', marginBottom: 14 }} placeholder="Nama sesuai daftar hadir"
              value={nama} onChange={(e) => setNama(e.target.value)} maxLength={100} />

            <label style={{ display: 'block', fontSize: 13, fontWeight: 600, marginBottom: 6 }}>Kelas</label>
            {kelasList.length ? (
              <select className="input" style={{ width: '100%', marginBottom: 14 }} value={kelas} onChange={(e) => setKelas(e.target.value)}>
                <option value="">— Pilih kelas —</option>
                {kelasList.map((k) => <option key={k} value={k}>{k}</option>)}
              </select>
            ) : (
              <input className="input" style={{ width: '100%', marginBottom: 14 }} placeholder="Contoh: X TKJ 4"
                value={kelas} onChange={(e) => setKelas(e.target.value)} maxLength={60} />
            )}

            <label style={{ display: 'block', fontSize: 13, fontWeight: 600, marginBottom: 6 }}>Judul tugas</label>
            <input className="input" style={{ width: '100%', marginBottom: 14 }} placeholder="Sesuai yang diberikan guru"
              value={tugas} onChange={(e) => setTugas(e.target.value)} maxLength={160} />

            <label style={{ display: 'block', fontSize: 13, fontWeight: 600, marginBottom: 6 }}>File tugas</label>
            <input type="file" accept={ACCEPT} style={{ width: '100%', marginBottom: 6 }}
              onChange={(e) => setFile(e.target.files?.[0] || null)} />
            <div style={{ fontSize: 12, color: '#64748b', marginBottom: 14 }}>
              {file ? `${file.name} · ${(file.size / 1024 / 1024).toFixed(2)} MB` : `PDF, Word, PowerPoint, Excel, gambar, ZIP/RAR, MP4, atau HTML — maks ${MAX_MB}MB.`}
            </div>

            <label style={{ display: 'block', fontSize: 13, fontWeight: 600, marginBottom: 6 }}>Catatan (opsional)</label>
            <textarea className="input" style={{ width: '100%', marginBottom: 16 }} rows={3} placeholder="Pesan untuk guru, bila ada"
              value={catatan} onChange={(e) => setCatatan(e.target.value)} maxLength={500} />

            {err && <div style={{ color: '#dc2626', fontSize: 13, marginBottom: 12 }}>{err}</div>}
            <button className="btn btn-block" type="submit" disabled={busy}>{busy ? 'Mengirim…' : 'Kirim Tugas'}</button>
            <div style={{ fontSize: 12, color: '#94a3b8', marginTop: 12, textAlign: 'center' }}>
              Tanpa perlu login. Periksa kembali nama dan kelas — itulah yang dilihat guru.
            </div>
          </form>
        )}
      </div>
    </div>
  )
}
