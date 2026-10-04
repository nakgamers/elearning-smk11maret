'use client'
import { useEffect, useState } from 'react'
import Shell from '../lib/Shell'
import { api } from '../lib/api'

const TABS = [
  { id: 'materi', label: 'Materi', icon: '📚' },
  { id: 'tugas', label: 'Tugas & Nilai', icon: '📝' },
  { id: 'absen', label: 'Absensi', icon: '🗓️' },
  { id: 'ujian', label: 'Ujian', icon: '📋' },
  { id: 'kuis', label: 'Kuis AI', icon: '🧠' },
  { id: 'rpn', label: 'Acak Nama', icon: '🎲' },
  { id: 'umum', label: 'Pengumuman', icon: '📢' },
]

export default function Guru() {
  const [tab, setTab] = useState('materi')
  const [user] = useState(typeof window !== 'undefined' ? (() => { try { return JSON.parse(localStorage.getItem('user')) } catch { return null } })() : null)
  const isWalas = user?.walas?.rombel_id

  const tabs = isWalas
    ? [...TABS, { id: 'walas', label: 'Walas', icon: '👥' }]
    : TABS

  return (
    <Shell tabs={tabs} active={tab} onTab={setTab}
      title={tabs.find((t) => t.id === tab)?.label}
      subtitle="Dashboard Guru">
      {tab === 'materi' && <Materi />}
      {tab === 'tugas' && <Tugas />}
      {tab === 'absen' && <Absen />}
      {tab === 'ujian' && <Ujian />}
      {tab === 'kuis' && <KuisAI />}
      {tab === 'rpn' && <RPN />}
      {tab === 'umum' && <Umum />}
      {tab === 'walas' && isWalas && <Walas user={user} />}
    </Shell>
  )
}

function useRombel() {
  const [rombel, setRombel] = useState([])
  useEffect(() => { api.get('/rombel').then(setRombel).catch(() => {}) }, [])
  return rombel
}

function useList(path) {
  const [data, setData] = useState([])
  const [err, setErr] = useState('')
  const reload = () => api.get(path).then(setData).catch((e) => setErr(e.message))
  useEffect(() => { reload() }, [path])
  return { data, err, reload }
}

function useSubjects() {
  const [subjects, setSubjects] = useState([])
  const [cps, setCps] = useState([])
  useEffect(() => {
    api.get('/subjects').then(setSubjects).catch(() => {})
    api.get('/cps').then(setCps).catch(() => {})
  }, [])
  return { subjects, cps }
}

function Materi() {
  const { data, reload, err } = useList('/materials')
  const { subjects, cps } = useSubjects()
  const rombel = useRombel()
  const [f, setF] = useState({ mapel_id: '', cp_id: '', judul: '', isi: '', kelas: '' })
  const [file, setFile] = useState(null)
  const [msg, setMsg] = useState('')
  const [busy, setBusy] = useState(false)

  async function submit(e) {
    e.preventDefault()
    setBusy(true); setMsg('')
    try {
      const fd = new FormData()
      Object.entries(f).forEach(([k, v]) => fd.append(k, v))
      if (file) fd.append('file', file)
      await api.postForm('/materials', fd)
      setMsg('✅ Materi terbit.')
      setF({ mapel_id: '', cp_id: '', judul: '', isi: '', kelas: '' }); setFile(null)
      reload()
    } catch (e) { setMsg('❌ ' + e.message) } finally { setBusy(false) }
  }

  return (
    <div style={{ display: 'grid', gap: 14 }}>
      <form className="card" style={{ padding: 18, display: 'grid', gap: 12 }} onSubmit={submit}>
        <div style={{ fontWeight: 700 }}>Terbitkan Materi</div>
        <div className="grid-2">
          <div>
            <label className="label">Mata pelajaran</label>
            <select className="input" value={f.mapel_id} onChange={(e) => setF({ ...f, mapel_id: e.target.value })} required>
              <option value="">Pilih…</option>
              {subjects.map((s) => <option key={s.id} value={s.id}>{s.nama}</option>)}
            </select>
          </div>
          <div>
            <label className="label">Capaian Pembelajaran (CP)</label>
            <select className="input" value={f.cp_id} onChange={(e) => setF({ ...f, cp_id: e.target.value })}>
              <option value="">— tanpa CP —</option>
              {cps.filter((c) => c.mapel_id === +f.mapel_id).map((c) =>
                <option key={c.id} value={c.id}>{c.elemen}</option>)}
            </select>
          </div>
        </div>
        <div>
          <label className="label">Judul</label>
          <input className="input" value={f.judul} required placeholder="cth. Pola Bilangan"
            onChange={(e) => setF({ ...f, judul: e.target.value })} />
        </div>
        <div>
          <label className="label">Isi / uraian</label>
          <textarea className="input" rows={4} value={f.isi} placeholder="Penjelasan materi…"
            onChange={(e) => setF({ ...f, isi: e.target.value })} />
        </div>
        <div className="grid-2">
          <div>
            <label className="label">Rombel sasaran (kosong = semua)</label>
            <select className="input" value={f.kelas} onChange={(e) => setF({ ...f, kelas: e.target.value })}>
              <option value="">— semua rombel —</option>
              {rombel.map((r) => <option key={r.id} value={r.nama}>{r.nama}</option>)}
            </select>
          </div>
          <div>
            <label className="label">Lampiran (PDF/DOC/…)</label>
            <input className="input" type="file" onChange={(e) => setFile(e.target.files[0])} />
          </div>
        </div>
        {msg && <div style={{ fontSize: 14 }}>{msg}</div>}
        <button className="btn btn-primary" disabled={busy} style={{ justifySelf: 'start' }}>{busy ? 'Menyimpan…' : 'Terbitkan'}</button>
      </form>

      <div className="card" style={{ padding: 16 }}>
        <div style={{ fontWeight: 700, marginBottom: 10 }}>Daftar Materi</div>
        {err && <div>{err}</div>}
        <table className="tbl">
          <thead><tr><th>Judul</th><th>Mapel</th><th>CP</th><th>Pengunggah</th><th>Kelas</th></tr></thead>
          <tbody>
            {data.map((m) => (
              <tr key={m.id}>
                <td><span style={{ fontWeight: 600 }}>{m.judul}</span></td>
                <td>{m.mapel}</td>
                <td>{m.cp_elemen || '—'}</td>
                <td>{m.guru}</td>
                <td>{m.kelas || 'Semua'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

function Tugas() {
  const { data, reload, err } = useList('/assignments')
  const { subjects, cps } = useSubjects()
  const rombel = useRombel()
  const [f, setF] = useState({ mapel_id: '', cp_id: '', judul: '', deskripsi: '', kelas: '', deadline: '', max_score: '100' })
  const [msg, setMsg] = useState('')
  const [openSub, setOpenSub] = useState(null)
  const [subs, setSubs] = useState([])

  async function submit(e) {
    e.preventDefault()
    setMsg('')
    try {
      const fd = new FormData()
      Object.entries(f).forEach(([k, v]) => fd.append(k, v))
      await api.postForm('/assignments', fd)
      setMsg('✅ Tugas dibuat.')
      setF({ mapel_id: '', cp_id: '', judul: '', deskripsi: '', kelas: '', deadline: '', max_score: '100' })
      reload()
    } catch (e) { setMsg('❌ ' + e.message) }
  }

  async function viewSubs(id) {
    setOpenSub(id)
    setSubs(await api.get(`/assignments/${id}/submissions`))
  }

  async function grade(sid, nilai, feedback) {
    await api.post(`/submissions/${sid}/grade`, { nilai: +nilai, feedback })
    setSubs(await api.get(`/assignments/${openSub}/submissions`))
  }

  return (
    <div style={{ display: 'grid', gap: 14 }}>
      <form className="card" style={{ padding: 18, display: 'grid', gap: 12 }} onSubmit={submit}>
        <div style={{ fontWeight: 700 }}>Buat Tugas</div>
        <div className="grid-2">
          <div>
            <label className="label">Mapel</label>
            <select className="input" value={f.mapel_id} onChange={(e) => setF({ ...f, mapel_id: e.target.value })} required>
              <option value="">Pilih…</option>
              {subjects.map((s) => <option key={s.id} value={s.id}>{s.nama}</option>)}
            </select>
          </div>
          <div>
            <label className="label">CP</label>
            <select className="input" value={f.cp_id} onChange={(e) => setF({ ...f, cp_id: e.target.value })}>
              <option value="">—</option>
              {cps.filter((c) => c.mapel_id === +f.mapel_id).map((c) => <option key={c.id} value={c.id}>{c.elemen}</option>)}
            </select>
          </div>
        </div>
        <div>
          <label className="label">Judul tugas</label>
          <input className="input" required value={f.judul} onChange={(e) => setF({ ...f, judul: e.target.value })} />
        </div>
        <div>
          <label className="label">Deskripsi</label>
          <textarea className="input" rows={2} value={f.deskripsi} onChange={(e) => setF({ ...f, deskripsi: e.target.value })} />
        </div>
        <div className="grid-3">
          <div>
            <label className="label">Rombel</label>
            <select className="input" value={f.kelas} onChange={(e) => setF({ ...f, kelas: e.target.value })}>
              <option value="">— semua rombel —</option>
              {rombel.map((r) => <option key={r.id} value={r.nama}>{r.nama}</option>)}
            </select>
          </div>
          <div>
            <label className="label">Deadline (opsional)</label>
            <input className="input" type="datetime-local" value={f.deadline} onChange={(e) => setF({ ...f, deadline: e.target.value })} />
          </div>
          <div>
            <label className="label">Nilai maks.</label>
            <input className="input" type="number" value={f.max_score} onChange={(e) => setF({ ...f, max_score: e.target.value })} />
          </div>
        </div>
        {msg && <div style={{ fontSize: 14 }}>{msg}</div>}
        <button className="btn btn-primary" style={{ justifySelf: 'start' }}>Buat Tugas</button>
      </form>

      <div className="card" style={{ padding: 16 }}>
        <div style={{ fontWeight: 700, marginBottom: 10 }}>Daftar Tugas</div>
        {err && <div>{err}</div>}
        {data.map((a) => (
          <div key={a.id} style={{ borderBottom: '1px solid #f1f5f9', padding: '10px 0' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', gap: 8, flexWrap: 'wrap' }}>
              <span style={{ fontWeight: 600 }}>{a.judul} <span style={{ color: '#64748b', fontWeight: 400, fontSize: 12 }}>({a.mapel} · {a.kelas || 'Semua'})</span></span>
              <button className="btn btn-ghost" style={{ padding: '4px 10px', fontSize: 12 }} onClick={() => viewSubs(a.id)}>Lihat pengumpulan</button>
            </div>
          </div>
        ))}
      </div>

      {openSub && (
        <div className="card" style={{ padding: 16 }}>
          <div style={{ fontWeight: 700, marginBottom: 10 }}>Pengumpulan</div>
          <table className="tbl">
            <thead><tr><th>NIS</th><th>Nama</th><th>Jawaban</th><th>Nilai</th><th>Feedback</th><th>Aksi</th></tr></thead>
            <tbody>{subs.map((s) => <SubRow key={s.id} s={s} onSave={grade} />)}</tbody>
          </table>
          <button className="btn btn-ghost" style={{ marginTop: 12, fontSize: 12 }} onClick={() => setOpenSub(null)}>Tutup</button>
        </div>
      )}
    </div>
  )
}

const IMG_EXT = /\.(png|jpe?g|gif|webp|bmp|svg)$/i
const PDF_EXT = /\.pdf$/i
const VID_EXT = /\.(mp4|mp3|webm)$/i

function fileKind(url) {
  if (!url) return 'none'
  if (IMG_EXT.test(url)) return 'img'
  if (PDF_EXT.test(url)) return 'pdf'
  if (VID_EXT.test(url)) return 'video'
  return 'doc'
}

function PreviewModal({ url, type, nama, onClose }) {
  return (
    <div onClick={onClose} role="dialog" aria-modal="true" aria-label="Pratinjau file"
      style={{ position: 'fixed', inset: 0, zIndex: 110, background: 'rgba(2,6,23,.92)', display: 'grid', placeItems: 'center', padding: 20, cursor: 'zoom-out' }}>
      <div onClick={(e) => e.stopPropagation()}
        style={{ width: 'min(96vw, 1100px)', height: '90vh', display: 'grid', placeItems: 'center', position: 'relative', cursor: 'default' }}>
        <button onClick={onClose} style={{ position: 'absolute', top: 0, right: 0, zIndex: 3, background: '#fff', color: '#0f172a', border: 'none', borderRadius: 8, padding: '8px 14px', fontWeight: 700, cursor: 'pointer' }}>✕ Tutup</button>
        {type === 'img' && <img src={url} alt={`Kumpulan ${nama}`} style={{ maxWidth: '100%', maxHeight: '100%', objectFit: 'contain', borderRadius: 10, background: '#fff' }} />}
        {type === 'pdf' && <iframe src={url} title={`Kumpulan ${nama}`} style={{ width: '100%', height: '100%', border: 'none', borderRadius: 10, background: '#fff' }} />}
        {type === 'video' && <video src={url} controls autoPlay style={{ maxWidth: '100%', maxHeight: '100%', borderRadius: 10, background: '#000' }} />}
      </div>
    </div>
  )
}

function SubRow({ s, onSave }) {
  const [nilai, setNilai] = useState(s.nilai ?? '')
  const [fb, setFb] = useState(s.feedback ?? '')
  const [preview, setPreview] = useState(null)
  useEffect(() => {
    if (!preview) return
    const h = (e) => { if (e.key === 'Escape') setPreview(null) }
    window.addEventListener('keydown', h)
    return () => window.removeEventListener('keydown', h)
  }, [preview])
  const kind = fileKind(s.file_url)
  return (
    <tr>
      <td>{s.nis}</td>
      <td>{s.nama}</td>
      <td style={{ maxWidth: 280 }}>
        {s.jawaban && <div style={{ whiteSpace: 'pre-wrap', marginBottom: s.file_url ? 6 : 0 }}>{s.jawaban}</div>}
        {kind === 'img' && (
          <div>
            <img
              src={s.file_url}
              alt={`Kumpulan ${s.nama}`}
              title="Klik untuk memperbesar"
              loading="lazy"
              onClick={() => setPreview({ url: s.file_url, type: 'img' })}
              style={{ maxWidth: 160, maxHeight: 120, borderRadius: 8, border: '1px solid #e2e8f0', cursor: 'zoom-in', display: 'block' }}
            />
            <button className="btn btn-ghost" style={{ padding: '2px 8px', fontSize: 11, marginTop: 4 }} onClick={() => setPreview({ url: s.file_url, type: 'img' })}>Lihat</button>
          </div>
        )}
        {kind === 'pdf' && (
          <button className="btn btn-ghost" style={{ padding: '4px 10px', fontSize: 12 }} onClick={() => setPreview({ url: s.file_url, type: 'pdf' })}>📄 Lihat PDF</button>
        )}
        {kind === 'video' && (
          <button className="btn btn-ghost" style={{ padding: '4px 10px', fontSize: 12 }} onClick={() => setPreview({ url: s.file_url, type: 'video' })}>🎬 Lihat video</button>
        )}
        {kind === 'doc' && (
          <a href={s.file_url} target="_blank" rel="noreferrer">📎 Buka dokumen</a>
        )}
        {kind === 'none' && !s.jawaban && '—'}
      </td>
      <td><input className="input" style={{ width: 70, padding: '4px 8px' }} type="number" value={nilai} onChange={(e) => setNilai(e.target.value)} /></td>
      <td><input className="input" style={{ width: 150, padding: '4px 8px' }} value={fb} onChange={(e) => setFb(e.target.value)} /></td>
      <td><button className="btn btn-primary" style={{ padding: '4px 10px', fontSize: 12 }} onClick={() => onSave(s.id, nilai, fb)}>Simpan</button></td>
      {preview && <PreviewModal url={preview.url} type={preview.type} nama={s.nama} onClose={() => setPreview(null)} />}
    </tr>
  )
}

function Absen() {
  const rombel = useRombel()
  const [user] = useState(typeof window !== 'undefined' ? (() => { try { return JSON.parse(localStorage.getItem('user')) } catch { return null } })() : null)
  const mapelID = user?.mapel_id || 0
  const [kelas, setKelas] = useState('')
  const [tanggal, setTanggal] = useState(new Date().toISOString().slice(0, 10))
  const { data, reload } = useList(`/attendance?kelas=${encodeURIComponent(kelas)}&tanggal=${tanggal}&mapel_id=${mapelID}`)
  const [students, setStudents] = useState([])
  const [exporting, setExporting] = useState(false)
  useEffect(() => { if (kelas) api.get(`/students?kelas=${encodeURIComponent(kelas)}`).then(setStudents).catch(() => {}) }, [kelas])
  const [busy, setBusy] = useState(false)

  const summary = students.reduce((acc, s) => {
    const status = data.find((a) => a.nis === s.nis)?.status || 'belum'
    acc[status] = (acc[status] || 0) + 1
    return acc
  }, { hadir: 0, alpa: 0, izin: 0, sakit: 0, belum: 0 })

  async function setAbsen(sid, status) {
    setBusy(true)
    try { await api.post('/attendance', { student_id: sid, tanggal, status, mapel_id: mapelID }); reload() }
    finally { setBusy(false) }
  }

  async function exportPDF() {
    setExporting(true)
    try { await api.download(`/attendance/export.pdf?kelas=${encodeURIComponent(kelas)}&tanggal=${tanggal}&mapel_id=${mapelID}`) }
    finally { setExporting(false) }
  }

  const byNis = Object.fromEntries(data.map((a) => [a.nis, a]))

  return (
    <div className="card" style={{ padding: 16 }}>
      <div style={{ display: 'flex', gap: 10, marginBottom: 14, flexWrap: 'wrap', alignItems: 'center' }}>
        <select className="input" style={{ width: 190 }} value={kelas} onChange={(e) => setKelas(e.target.value)}>
          <option value="">Pilih rombel…</option>
          {rombel.map((r) => <option key={r.id} value={r.nama}>{r.nama}</option>)}
        </select>
        <input className="input" style={{ width: 170 }} type="date" value={tanggal} onChange={(e) => setTanggal(e.target.value)} />
        <button className="btn btn-primary" disabled={!kelas || exporting} onClick={exportPDF}>
          {exporting ? 'Menyiapkan PDF…' : '⬇ Export PDF'}
        </button>
      </div>
      {kelas && <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginBottom: 14 }}>
        {[
          ['hadir', '✅ Hadir', '#dcfce7', '#166534'],
          ['alpa', '❌ Alpa', '#fee2e2', '#991b1b'],
          ['izin', '📝 Izin', '#fef3c7', '#92400e'],
          ['sakit', '🤒 Sakit', '#dbeafe', '#1e40af'],
          ['belum', '⏳ Belum absen', '#f1f5f9', '#475569'],
        ].map(([key, label, bg, color]) => <span key={key} className="badge" style={{ background: bg, color }}><b>{summary[key] || 0}</b> {label}</span>)}
        <span className="badge" style={{ background: '#ede9fe', color: '#5b21b6' }}><b>{students.length}</b> Total</span>
      </div>}
      {!kelas ? <div className="empty"><span className="big">🗓️</span>Pilih rombel untuk mengisi absensi.</div> : (
        <>
          <div style={{ fontWeight: 700, marginBottom: 10 }}>Absensi {kelas} — {tanggal}</div>
          <table className="tbl">
            <thead><tr><th>NIS</th><th>Nama</th><th>Status</th><th>Ubah</th></tr></thead>
            <tbody>
              {students.map((s) => {
                const st = byNis[s.nis]?.status
                return (
                  <tr key={s.id}>
                    <td>{s.nis}</td><td>{s.nama}</td>
                    <td>{st ? { hadir: '✅ Hadir', izin: '📝 Izin', sakit: '🤒 Sakit', alpa: '❌ Alpa' }[st] : <span style={{ color: '#94a3b8' }}>— belum tercatat</span>}</td>
                    <td><div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
                      {[['hadir', 'Hadir'], ['izin', 'Izin'], ['sakit', 'Sakit'], ['alpa', 'Alpa']].map(([v, l]) => <button key={v} className="btn btn-ghost" disabled={busy} style={{ padding: '4px 8px', fontSize: 11.5 }} onClick={() => setAbsen(s.id, v)}>{l}</button>)}
                    </div></td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </>
      )}
    </div>
  )
}

function Ujian() {
  const { data, reload, err } = useList('/exams')
  const { subjects, cps } = useSubjects()
  const rombel = useRombel()
  const [f, setF] = useState({ nama: '', mapel_id: '', cp_id: '', kelas: '', durasi_menit: '60', mulai_at: '', selesai_at: '', acak_soal: false })
  const [msg, setMsg] = useState('')
  const [importFor, setImportFor] = useState(null)
  const [impFile, setImpFile] = useState(null)
  const [impMsg, setImpMsg] = useState('')

  async function submit(e) {
    e.preventDefault()
    setMsg('')
    try {
      await api.post('/exams', { ...f, mapel_id: +f.mapel_id, cp_id: +f.cp_id || 0, durasi_menit: +f.durasi_menit })
      setMsg('✅ Ujian dibuat.')
      setF({ nama: '', mapel_id: '', cp_id: '', kelas: '', durasi_menit: '60', mulai_at: '', selesai_at: '', acak_soal: false })
      reload()
    } catch (e) { setMsg('❌ ' + e.message) }
  }

  async function doImport() {
    setImpMsg('')
    if (!impFile) return setImpMsg('❌ Pilih file xlsx dulu.')
    const fd = new FormData()
    fd.append('file', impFile)
    const r = await api.postForm(`/exams/${importFor}/questions/import`, fd)
    setImpMsg(`✅ ${r.imported} soal diimpor. Sekarang aktifkan ujian agar soal di-cache.`)
    reload()
  }

  async function activate(id) {
    const r = await api.post(`/exams/${id}/activate`, {})
    setMsg(`✅ Ujian aktif, ${r.cached} soal masuk cache.`)
    reload()
  }

  return (
    <div style={{ display: 'grid', gap: 14 }}>
      <form className="card" style={{ padding: 18, display: 'grid', gap: 12 }} onSubmit={submit}>
        <div style={{ fontWeight: 700 }}>Buat Ujian</div>
        <div className="grid-2">
          <div>
            <label className="label">Nama ujian</label>
            <input className="input" required value={f.nama} onChange={(e) => setF({ ...f, nama: e.target.value })} />
          </div>
          <div>
            <label className="label">Mapel</label>
            <select className="input" value={f.mapel_id} onChange={(e) => setF({ ...f, mapel_id: e.target.value })} required>
              <option value="">Pilih…</option>
              {subjects.map((s) => <option key={s.id} value={s.id}>{s.nama}</option>)}
            </select>
          </div>
        </div>
        <div className="grid-3">
          <div>
            <label className="label">Rombel</label>
            <select className="input" value={f.kelas} onChange={(e) => setF({ ...f, kelas: e.target.value })}>
              <option value="">— semua rombel —</option>
              {rombel.map((r) => <option key={r.id} value={r.nama}>{r.nama}</option>)}
            </select>
          </div>
          <div>
            <label className="label">Durasi (menit)</label>
            <input className="input" type="number" value={f.durasi_menit} onChange={(e) => setF({ ...f, durasi_menit: e.target.value })} />
          </div>
          <div>
            <label className="label">CP</label>
            <select className="input" value={f.cp_id} onChange={(e) => setF({ ...f, cp_id: e.target.value })}>
              <option value="">—</option>
              {cps.map((c) => <option key={c.id} value={c.id}>{c.elemen}</option>)}
            </select>
          </div>
        </div>
        {msg && <div style={{ fontSize: 14 }}>{msg}</div>}
        <button className="btn btn-primary" style={{ justifySelf: 'start' }}>Buat Ujian</button>
      </form>

      <div className="card" style={{ padding: 16 }}>
        <div style={{ fontWeight: 700, marginBottom: 10 }}>Daftar Ujian</div>
        {err && <div>{err}</div>}
        <table className="tbl">
          <thead><tr><th>Nama</th><th>Mapel</th><th>Kelas</th><th>Durasi</th><th>Soal</th><th>Status</th><th>Aksi</th></tr></thead>
          <tbody>{data.map((x) => (
            <tr key={x.id}>
              <td style={{ fontWeight: 600 }}>{x.nama}</td>
              <td>{x.mapel}</td>
              <td>{x.kelas || 'Semua'}</td>
              <td>{x.durasi_menit}m</td>
              <td>{x.jumlah_soal}</td>
              <td>{x.aktif ? <span className="badge" style={{ background: '#dcfce7', color: '#15803d' }}>Aktif</span> : <span className="badge" style={{ background: '#f1f5f9', color: '#64748b' }}>Draft</span>}</td>
              <td>
                <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
                  <button className="btn btn-ghost" style={{ padding: '4px 8px', fontSize: 11.5 }} onClick={() => setImportFor(x.id)}>Import soal</button>
                  <button className="btn btn-primary" style={{ padding: '4px 10px', fontSize: 11.5 }} onClick={() => activate(x.id)}>Aktifkan</button>
                </div>
              </td>
            </tr>
          ))}</tbody>
        </table>

        {importFor != null && (
          <div style={{ marginTop: 14, background: '#f8fafc', padding: 14, borderRadius: 12, display: 'grid', gap: 10 }}>
            <div style={{ fontWeight: 600, fontSize: 14 }}>Import soal (xlsx): Soal | OpsiA | OpsiB | OpsiC | OpsiD | OpsiE | Kunci(1-5) | Bobot</div>
            <input className="input" type="file" accept=".xlsx" onChange={(e) => setImpFile(e.target.files[0])} />
            {impMsg && <div style={{ fontSize: 13 }}>{impMsg}</div>}
            <div style={{ display: 'flex', gap: 8 }}>
              <button className="btn btn-primary" style={{ padding: '6px 14px', fontSize: 13 }} onClick={doImport}>Unggah soal</button>
              <button className="btn btn-ghost" style={{ padding: '6px 14px', fontSize: 13 }} onClick={() => setImportFor(null)}>Batal</button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

function Umum() {
  const [f, setF] = useState({ judul: '', isi: '' })
  const [msg, setMsg] = useState('')
  async function submit(e) {
    e.preventDefault()
    setMsg('')
    try { await api.post('/news', f); setMsg('✅ Pengumuman terbit.'); setF({ judul: '', isi: '' }) }
    catch (e) { setMsg('❌ ' + e.message) }
  }
  return (
    <form className="card" style={{ padding: 18, display: 'grid', gap: 12 }} onSubmit={submit}>
      <div style={{ fontWeight: 700 }}>Tulis Pengumuman</div>
      <div>
        <label className="label">Judul</label>
        <input className="input" required value={f.judul} onChange={(e) => setF({ ...f, judul: e.target.value })} />
      </div>
      <div>
        <label className="label">Isi</label>
        <textarea className="input" rows={4} required value={f.isi} onChange={(e) => setF({ ...f, isi: e.target.value })} />
      </div>
      {msg && <div style={{ fontSize: 14 }}>{msg}</div>}
      <button className="btn btn-primary" style={{ justifySelf: 'start' }}>Terbitkan</button>
    </form>
  )
}

// Walas: lihat kehadiran siswa rombel yang dipegang, lintas mapel (diampu guru lain).
const ST_ICON = { hadir: '✅', izin: '📝', sakit: '🤒', alpa: '❌' }
function Walas({ user }) {
  const [tanggal, setTanggal] = useState(new Date().toISOString().slice(0, 10))
  const [data, setData] = useState(null)
  const [err, setErr] = useState('')

  useEffect(() => {
    const rid = user?.walas?.rombel_id
    if (!rid) return
    api.get(`/walas/attendance?rombel_id=${rid}&tanggal=${tanggal}`).then(setData).catch((e) => setErr(e.message))
  }, [tanggal, user])

  return (
    <div style={{ display: 'grid', gap: 14 }}>
      <div className="card" style={{ padding: 16 }}>
        <div style={{ display: 'flex', gap: 10, alignItems: 'center', flexWrap: 'wrap', marginBottom: 12 }}>
          <div style={{ fontWeight: 700, marginRight: 'auto' }}>Kehadiran Siswa — {data?.rombel || user?.walas?.rombel}</div>
          <input className="input" style={{ width: 170 }} type="date" value={tanggal} onChange={(e) => setTanggal(e.target.value)} />
        </div>
        {err && <div>{err}</div>}
        {!data ? <div className="empty">Memuat…</div> : (
          data.siswa.length === 0 ? <div className="empty"><span className="big">👥</span>Belum ada siswa di rombel ini.</div> : (
            <table className="tbl">
              <thead>
                <tr>
                  <th>NIS</th><th>Nama</th>
                  {data.mapel.map((m) => <th key={m.id}>{m.nama}</th>)}
                </tr>
              </thead>
              <tbody>
                {data.siswa.map((s) => (
                  <tr key={s.id}>
                    <td>{s.nis}</td>
                    <td style={{ fontWeight: 600 }}>{s.nama}</td>
                    {data.mapel.map((m) => {
                      const st = s.status?.[m.nama]
                      return <td key={m.id}>{st ? ST_ICON[st] + ' ' + ({ hadir: 'Hadir', izin: 'Izin', sakit: 'Sakit', alpa: 'Alpa' }[st]) : <span style={{ color: '#cbd5e1' }}>—</span>}</td>
                    })}
                  </tr>
                ))}
              </tbody>
            </table>
          )
        )}
      </div>
    </div>
  )
}
// ===== Random Pick Name: pilih siswa maju secara acak (tampil di IFP) =====
function RPN() {
  const rombel = useRombel()
  const [kelas, setKelas] = useState('')
  const [students, setStudents] = useState([])
  const [current, setCurrent] = useState(null)
  const [calledIds, setCalledIds] = useState([])
  const [noRepeat, setNoRepeat] = useState(true)
  const [rolling, setRolling] = useState(false)
  const [msg, setMsg] = useState('')

  useEffect(() => {
    if (!kelas) { setStudents([]); setCalledIds([]); setCurrent(null); return }
    api.get('/students?kelas=' + encodeURIComponent(kelas))
      .then(setStudents).catch(() => {})
    setCalledIds([]); setCurrent(null); setMsg('')
  }, [kelas])

  function pick() {
    if (rolling || !students.length) return
    const pool = noRepeat ? students.filter((s) => !calledIds.includes(s.id)) : students
    if (!pool.length) { setMsg('Semua siswa sudah dipanggil. Tekan Reset untuk mengulang.'); return }
    setMsg('')
    setRolling(true)
    const iv = setInterval(() => setCurrent(pool[Math.floor(Math.random() * pool.length)]), 90)
    setTimeout(() => {
      clearInterval(iv)
      const w = pool[Math.floor(Math.random() * pool.length)]
      setCurrent(w)
      setCalledIds((c) => (c.includes(w.id) ? c : [...c, w.id]))
      setRolling(false)
    }, 1500)
  }

  function reset() { setCalledIds([]); setCurrent(null); setMsg('') }

  const calledNames = students.filter((s) => calledIds.includes(s.id))

  return (
    <div style={{ display: 'grid', gap: 14, maxWidth: 860 }}>
      <div className="card" style={{ padding: 18, display: 'grid', gap: 12 }}>
        <div style={{ fontWeight: 700 }}>🎲 Acak Nama Siswa</div>
        <div className="grid-2">
          <div>
            <label className="label">Rombel</label>
            <select className="input" value={kelas} onChange={(e) => setKelas(e.target.value)}>
              <option value="">Pilih rombel…</option>
              {rombel.map((r) => <option key={r.id} value={r.nama}>{r.nama}</option>)}
            </select>
          </div>
          <div style={{ display: 'flex', alignItems: 'flex-end', gap: 8 }}>
            <label style={{ display: 'flex', gap: 6, alignItems: 'center', fontSize: '.9rem' }}>
              <input type="checkbox" checked={noRepeat} onChange={(e) => setNoRepeat(e.target.checked)} />
              Tanpa pengulangan
            </label>
          </div>
        </div>
        {msg && <div className="alert alert-warn">{msg}</div>}
        <div className="btn-row">
          <button className="btn btn-ok" disabled={!kelas || rolling || !students.length} onClick={pick}>
            {rolling ? 'Mengacak…' : '🎲 Acak!'}
          </button>
          <button className="btn btn-ghost" disabled={rolling} onClick={reset}>Reset</button>
          <span className="dim" style={{ alignSelf: 'center' }}>
            {students.length ? `${calledIds.length}/${students.length} sudah dipanggil` : 'Pilih rombel dulu'}
          </span>
        </div>
      </div>

      <div className="card" style={{ padding: 32, textAlign: 'center', minHeight: 190, display: 'grid', placeItems: 'center' }}>
        {current ? (
          <div className="animate-in" key={current.id + '-' + calledIds.length}>
            <div className="dim">yang maju:</div>
            <div style={{ fontSize: '2.6rem', fontWeight: 800 }}>{current.nama}</div>
            <div className="dim">{current.nis} · {current.kelas}</div>
          </div>
        ) : (
          <div className="empty"><span className="big">🎲</span>Nama yang terpilih tampil di sini</div>
        )}
      </div>

      {calledNames.length > 0 && (
        <div className="card" style={{ padding: 18 }}>
          <div style={{ fontWeight: 700, marginBottom: 8 }}>Sudah dipanggil sesi ini</div>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
            {calledNames.map((s) => <span key={s.id} className="badge badge-default">{s.nama}</span>)}
          </div>
        </div>
      )}
    </div>
  )
}

// ===== Kuis AI: materi -> 10 soal -> tampil di IFP + papan skor manual =====
const QUIZ_SK = 'kuisai-skor'

function KuisAI() {
  const [step, setStep] = useState('setup') // setup | review | tampil
  const [materi, setMateri] = useState('')
  const [file, setFile] = useState(null)
  const [jumlah, setJumlah] = useState(10)
  const [kesulitan, setKesulitan] = useState('sedang')
  const [provider, setProvider] = useState('server') // server | gemini
  const [aiStatus, setAiStatus] = useState({ server: false, gemini: false })
  const [geminiKey, setGeminiKey] = useState('')
  const [showKeyForm, setShowKeyForm] = useState(false)
  const [showAdv, setShowAdv] = useState(false) // mode lanjutan: tempel JSON
  const [raw, setRaw] = useState('')
  const [qs, setQs] = useState([])
  const [busy, setBusy] = useState(false)
  const [msg, setMsg] = useState('')
  const [msgOk, setMsgOk] = useState(false)

  function note(ok, text) { setMsgOk(ok); setMsg(text) }

  useEffect(() => {
    api.get('/ai/status').then(setAiStatus).catch(() => {})
  }, [])

  async function simpanKey() {
    if (geminiKey.trim().length < 10) { note(false, '❌ Kunci terlalu pendek.'); return }
    setBusy(true)
    try {
      await api.post('/ai/key', { provider: 'gemini', key: geminiKey.trim() })
      setAiStatus((s) => ({ ...s, gemini: true }))
      setGeminiKey(''); setShowKeyForm(false)
      note(true, '✅ Kunci Gemini tersimpan terenkripsi.')
    } catch (e) { note(false, '❌ ' + e.message) } finally { setBusy(false) }
  }

  async function hapusKey() {
    if (!confirm('Hapus kunci Gemini tersimpan?')) return
    try {
      await api.del('/ai/key?provider=gemini')
      setAiStatus((s) => ({ ...s, gemini: false }))
      note(true, '✅ Kunci Gemini dihapus.')
    } catch (e) { note(false, '❌ ' + e.message) }
  }

  async function doGenerate() {
    setBusy(true); note(false, '')
    try {
      let jobId
      if (file) {
        const fd = new FormData()
        fd.append('file', file)
        fd.append('provider', provider)
        fd.append('jumlah', String(+jumlah || 10))
        fd.append('kesulitan', kesulitan)
        const r = await api.postForm('/quiz/generate', fd)
        jobId = r.job_id
      } else {
        if (materi.trim().length < 50) { note(false, '❌ Tempel materi dulu (min. 50 karakter) atau upload file.'); setBusy(false); return }
        const r = await api.post('/quiz/generate', { materi, jumlah: +jumlah || 10, kesulitan, provider })
        jobId = r.job_id
      }
      if (!jobId) throw new Error('server tidak mengembalikan job_id')
      // Polling: generate via AI bisa 1-3 menit untuk materi besar.
      // Tiap poll cepat, jadi kebal terhadap timeout proxy.
      const tungguPesan = [
        '⏳ Quineilla sedang membaca materimu...',
        '📖 Menandai konsep-konsep kunci...',
        '✍️ Menyusun opsi pengecoh yang (semoga) mengecoh...',
        '🧠 Meracik pembahasan tiap soal...',
        '☕ Sambil menunggu: siapkan IFP dan atur posisi duduk siswa...',
        '🎲 Fakta: guru yang pakai kuis dadakan bikin siswa 2x lebih waspada.',
        '🔍 Mengecek ulang kunci jawaban...',
        '📝 Hampir jadi — merapikan format soal...',
      ]
      for (let i = 0; i < 200; i++) {
        await new Promise((res) => setTimeout(res, 2500))
        const j = await api.get('/quiz/jobs/' + jobId)
        if (j.status === 'done') {
          setQs(j.questions)
          setStep('review')
          note(true, `✅ ${j.questions.length} soal dibuat. Periksa & edit sebelum ditampilkan.`)
          return
        }
        if (j.status === 'error') throw new Error(j.error || 'gagal membuat soal')
        const detik = Math.round((i + 1) * 2.5)
        note(false, `${tungguPesan[i % tungguPesan.length]} (${detik} dtk)`)
      }
      throw new Error('waktu tunggu habis (5 menit) — coba lagi')
    } catch (e) { note(false, '❌ ' + e.message) } finally { setBusy(false) }
  }

  async function doValidate() {
    if (!raw.trim()) { note(false, '❌ Tempel dulu JSON hasil chat AI.'); return }
    setBusy(true); note(false, '')
    try {
      const r = await api.post('/quiz/validate', { raw })
      setQs(r.questions)
      setStep('review')
      note(true, `✅ ${r.questions.length} soal valid. Periksa & edit sebelum ditampilkan.`)
    } catch (e) { note(false, '❌ ' + e.message) } finally { setBusy(false) }
  }

  function updQ(i, patch) { setQs((list) => list.map((q, j) => (j === i ? { ...q, ...patch } : q))) }
  function updOpsi(i, j, v) {
    setQs((list) => list.map((q, k) => (k === i
      ? { ...q, opsi: q.opsi.map((o, jj) => (jj === j ? v : o)) } : q)))
  }
  function delQ(i) { setQs((list) => list.filter((_, j) => j !== i)) }

  function unduhJSON() {
    const blob = new Blob([JSON.stringify({ questions: qs }, null, 2)], { type: 'application/json' })
    const a = document.createElement('a')
    a.href = URL.createObjectURL(blob)
    a.download = 'kuis.json'
    a.click()
    URL.revokeObjectURL(a.href)
  }

  async function salinJSON() {
    try {
      await navigator.clipboard.writeText(JSON.stringify({ questions: qs }, null, 2))
      note(true, '✅ JSON tersalin.')
    } catch { note(false, '❌ Gagal menyalin.') }
  }

  return (
    <div style={{ display: 'grid', gap: 14 }}>
      {msg && <div className={'alert ' + (msgOk ? 'alert-ok' : 'alert-error')}>{msg}</div>}

      {step === 'setup' && (
        <div className="card" style={{ padding: 18, display: 'grid', gap: 12, maxWidth: 860 }}>
          <div style={{ fontWeight: 700 }}>🧠 Buat Kuis dari Materi</div>

          <div>
            <label className="label">Sumber AI</label>
            <div className="btn-row">
              <button className={'btn btn-sm ' + (provider === 'server' ? 'btn-ok' : 'btn-ghost')}
                onClick={() => setProvider('server')}>
                ✨ Quineilla{!aiStatus.server && ' (belum aktif)'}
              </button>
              <button className={'btn btn-sm ' + (provider === 'gemini' ? 'btn-ok' : 'btn-ghost')}
                onClick={() => setProvider('gemini')}>
                🔑 Gemini saya{aiStatus.gemini && ' ✓'}
              </button>
            </div>
          </div>

          {provider === 'gemini' && !aiStatus.gemini && (
            <div className="alert alert-warn">
              <div style={{ marginBottom: 8 }}>
                Tempel <b>API key Gemini</b> kamu (gratis dari{' '}
                <a href="https://aistudio.google.com/apikey" target="_blank" rel="noreferrer">aistudio.google.com</a>).
                Kunci tersimpan <b>terenkripsi</b> di server dan hanya dipakai untuk akunmu.
              </div>
              {!showKeyForm ? (
                <button className="btn btn-sm btn-ok" onClick={() => setShowKeyForm(true)}>🔑 Simpan API key</button>
              ) : (
                <div style={{ display: 'flex', gap: 8 }}>
                  <input className="input" type="password" value={geminiKey} placeholder="AIza…"
                    onChange={(e) => setGeminiKey(e.target.value)} style={{ fontFamily: 'monospace' }} />
                  <button className="btn btn-sm btn-ok" disabled={busy} onClick={simpanKey}>Simpan</button>
                  <button className="btn btn-sm btn-ghost" onClick={() => { setShowKeyForm(false); setGeminiKey('') }}>Batal</button>
                </div>
              )}
            </div>
          )}
          {provider === 'gemini' && aiStatus.gemini && (
            <div className="dim" style={{ fontSize: '.85rem' }}>
              🔑 Kunci Gemini tersimpan. <button className="btn btn-sm btn-ghost" onClick={hapusKey}>Hapus kunci</button>
            </div>
          )}
          {provider === 'server' && !aiStatus.server && (
            <div className="alert alert-warn">
              <b>✨ Quineilla</b> belum dikonfigurasi admin. Pakai <b>🔑 Gemini saya</b> di atas,
              atau minta admin mengisi AI_BASE_URL / AI_API_KEY / AI_MODEL di server.
            </div>
          )}

          <div>
            <label className="label">Upload materi (PDF / PPTX / DOCX / TXT / MD)</label>
            <input className="input" type="file" accept=".pdf,.pptx,.docx,.txt,.md"
              onChange={(e) => setFile(e.target.files?.[0] || null)} />
            {file && <div className="dim" style={{ fontSize: '.85rem', marginTop: 4 }}>📎 {file.name} — teks akan dibaca otomatis.</div>}
          </div>
          <div>
            <label className="label">…atau tempel teks materi</label>
            <textarea className="input" rows={6} value={materi} placeholder="Tempel isi materi di sini… (kosongkan jika sudah upload file)"
              onChange={(e) => setMateri(e.target.value)} />
          </div>
          <div className="grid-2">
            <div>
              <label className="label">Jumlah soal</label>
              <input className="input" type="number" min={1} max={50} value={jumlah}
                onChange={(e) => setJumlah(e.target.value)} />
            </div>
            <div>
              <label className="label">Kesulitan</label>
              <select className="input" value={kesulitan} onChange={(e) => setKesulitan(e.target.value)}>
                <option value="mudah">Mudah</option>
                <option value="sedang">Sedang</option>
                <option value="sukar">Sukar</option>
              </select>
            </div>
          </div>
          <div>
            <button className="btn btn-ok" disabled={busy} onClick={doGenerate}>
              {busy ? 'Membuat soal…' : '✨ Generate ' + (jumlah || 10) + ' Soal'}
            </button>
          </div>

          <details style={{ marginTop: 4 }}>
            <summary className="dim" style={{ cursor: 'pointer', fontSize: '.85rem' }}>
              Mode lanjutan: tempel JSON soal dari chat AI
            </summary>
            <div style={{ display: 'grid', gap: 8, marginTop: 8 }}>
              <textarea className="input" rows={5} value={raw}
                placeholder={'{"questions":[{"soal":"...","opsi":["A","B","C","D"],"kunci":0,"pembahasan":"..."}]}'}
                onChange={(e) => setRaw(e.target.value)} style={{ fontFamily: 'monospace', fontSize: '.8rem' }} />
              <div>
                <button className="btn btn-sm" disabled={busy} onClick={doValidate}>
                  {busy ? 'Memvalidasi…' : '✅ Validasi & Lanjut'}
                </button>
              </div>
            </div>
          </details>
        </div>
      )}

      {step === 'review' && (
        <div style={{ display: 'grid', gap: 12, maxWidth: 860 }}>
          <div className="btn-row">
            <button className="btn btn-ok" disabled={!qs.length} onClick={() => setStep('tampil')}>▶️ Tampilkan Kuis ({qs.length} soal)</button>
            <button className="btn btn-ghost btn-sm" onClick={unduhJSON}>⬇️ Unduh JSON</button>
            <button className="btn btn-ghost btn-sm" onClick={salinJSON}>📋 Salin</button>
            <button className="btn btn-ghost btn-sm" onClick={() => setStep('setup')}>← Kembali</button>
          </div>
          {qs.map((q, i) => (
            <div key={i} className="card" style={{ padding: 16, display: 'grid', gap: 10 }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <b>Soal {i + 1}</b>
                <button className="btn btn-danger btn-sm" onClick={() => delQ(i)}>Hapus</button>
              </div>
              <textarea className="input" rows={2} value={q.soal} onChange={(e) => updQ(i, { soal: e.target.value })} />
              <div className="grid-2">
                {q.opsi.map((o, j) => (
                  <div key={j} style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                    <input type="radio" name={'kunci-' + i} checked={q.kunci === j}
                      onChange={() => updQ(i, { kunci: j })} title="Jawaban benar" />
                    <b>{'ABCD'[j]}.</b>
                    <input className="input" value={o} onChange={(e) => updOpsi(i, j, e.target.value)} />
                  </div>
                ))}
              </div>
              <input className="input" value={q.pembahasan || ''} placeholder="Pembahasan (opsional)"
                onChange={(e) => updQ(i, { pembahasan: e.target.value })} />
            </div>
          ))}
        </div>
      )}

      {step === 'tampil' && <KuisTampil qs={qs} onEdit={() => setStep('review')} />}
    </div>
  )
}

// Tampilan presentasi kuis untuk IFP + papan skor manual.
function KuisTampil({ qs, onEdit }) {
  const [idx, setIdx] = useState(0)
  const [showAns, setShowAns] = useState(false)
  const [showScore, setShowScore] = useState(false)
  const q = qs[idx]

  useEffect(() => { setShowAns(false) }, [idx])
  useEffect(() => {
    function onKey(e) {
      if (e.key === 'ArrowRight') setIdx((i) => Math.min(qs.length - 1, i + 1))
      if (e.key === 'ArrowLeft') setIdx((i) => Math.max(0, i - 1))
      if (e.key === ' ') { e.preventDefault(); setShowAns((s) => !s) }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [qs.length])

  function fullscreen() {
    if (document.fullscreenElement) document.exitFullscreen()
    else document.documentElement.requestFullscreen?.()
  }

  if (!q) return <div className="empty">Belum ada soal.</div>

  return (
    <div style={{ display: 'grid', gap: 12 }}>
      <div className="btn-row">
        <button className="btn btn-ghost btn-sm" onClick={onEdit}>✏️ Edit soal</button>
        <button className="btn btn-ghost btn-sm" onClick={fullscreen}>⛶ Layar penuh</button>
        <button className={'btn btn-sm ' + (showScore ? 'btn-ok' : 'btn-ghost')} onClick={() => setShowScore((s) => !s)}>🏆 Papan skor</button>
        <span className="dim" style={{ alignSelf: 'center' }}>Soal {idx + 1} / {qs.length} · ← → navigasi · spasi = jawaban</span>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: showScore ? '1fr 320px' : '1fr', gap: 12, alignItems: 'start' }}>
        <div className="card animate-in" key={idx} style={{ padding: 28 }}>
          <div className="badge badge-default" style={{ marginBottom: 12 }}>Soal {idx + 1}</div>
          <div style={{ fontSize: '1.5rem', fontWeight: 700, lineHeight: 1.5, marginBottom: 20 }}>{q.soal}</div>
          <div style={{ display: 'grid', gap: 10 }}>
            {q.opsi.map((o, j) => {
              const benar = showAns && j === q.kunci
              return (
                <div key={j} className="card"
                  style={{
                    padding: '14px 16px', margin: 0, fontSize: '1.1rem',
                    borderWidth: 2,
                    borderColor: benar ? '#16a34a' : undefined,
                    background: benar ? '#f0fdf4' : undefined,
                  }}>
                  <b style={{ marginRight: 10 }}>{'ABCD'[j]}.</b>{o}
                  {benar && <span style={{ marginLeft: 10 }}>✅</span>}
                </div>
              )
            })}
          </div>
          {showAns && q.pembahasan && (
            <div className="alert alert-ok" style={{ marginTop: 16 }}>💡 {q.pembahasan}</div>
          )}
          <div className="btn-row" style={{ marginTop: 20 }}>
            <button className="btn" disabled={idx === 0} onClick={() => setIdx(idx - 1)}>⬅️ Sebelumnya</button>
            <button className="btn btn-ok" onClick={() => setShowAns((s) => !s)}>
              {showAns ? '🙈 Sembunyikan jawaban' : '👁️ Lihat jawaban'}
            </button>
            <button className="btn" disabled={idx === qs.length - 1} onClick={() => setIdx(idx + 1)}>Berikutnya ➡️</button>
          </div>
        </div>
        {showScore && <PapanSkor />}
      </div>
    </div>
  )
}

// Papan skor manual: guru mengetuk nama siswa yang menjawab benar.
// Tersimpan di localStorage per sesi (aman dari refresh).
function PapanSkor() {
  const rombel = useRombel()
  const [kelas, setKelas] = useState('')
  const [students, setStudents] = useState([])
  const [skor, setSkor] = useState(() => {
    try { return JSON.parse(localStorage.getItem(QUIZ_SK) || '{}') } catch { return {} }
  })
  const [poin, setPoin] = useState(10)
  const [msg, setMsg] = useState('')

  useEffect(() => {
    localStorage.setItem(QUIZ_SK, JSON.stringify(skor))
  }, [skor])

  useEffect(() => {
    if (!kelas) { setStudents([]); return }
    api.get('/students?kelas=' + encodeURIComponent(kelas)).then(setStudents).catch(() => {})
  }, [kelas])

  function add(id, v) {
    setSkor((s) => ({ ...s, [id]: Math.max(0, (s[id] || 0) + v) }))
  }

  function reset() {
    if (confirm('Reset semua skor sesi ini?')) { setSkor({}); setMsg('') }
  }

  async function salinRekap() {
    const rows = students
      .map((s) => ({ nama: s.nama, nis: s.nis, skor: skor[s.id] || 0 }))
      .filter((r) => r.skor > 0)
      .sort((a, b) => b.skor - a.skor)
    const teks = rows.length
      ? 'Rekap Kuis (' + new Date().toLocaleDateString('id-ID') + ')\n' +
        rows.map((r, i) => `${i + 1}. ${r.nama} (${r.nis}) — ${r.skor}`).join('\n')
      : 'Belum ada skor.'
    try { await navigator.clipboard.writeText(teks); setMsg('✅ Rekap tersalin.') }
    catch { setMsg('❌ Gagal menyalin.') }
  }

  const terurut = [...students].sort((a, b) => (skor[b.id] || 0) - (skor[a.id] || 0))

  return (
    <div className="card" style={{ padding: 16, display: 'grid', gap: 10 }}>
      <div style={{ fontWeight: 700 }}>🏆 Papan Skor</div>
      <select className="input" value={kelas} onChange={(e) => setKelas(e.target.value)}>
        <option value="">Pilih rombel…</option>
        {rombel.map((r) => <option key={r.id} value={r.nama}>{r.nama}</option>)}
      </select>
      <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
        <label className="label" style={{ margin: 0 }}>Poin per jawaban</label>
        <input className="input" type="number" min={1} value={poin} style={{ width: 80 }}
          onChange={(e) => setPoin(+e.target.value || 1)} />
      </div>
      {msg && <div className="dim" style={{ fontSize: '.85rem' }}>{msg}</div>}
      <div style={{ display: 'grid', gap: 6, maxHeight: 420, overflowY: 'auto' }}>
        {terurut.map((s, i) => (
          <div key={s.id} style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '8px 10px', border: '1px solid #e5e7eb', borderRadius: 10 }}>
            <b style={{ width: 26 }}>{i + 1}</b>
            <div style={{ flex: 1, minWidth: 0 }}>
              <div style={{ fontWeight: 600, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{s.nama}</div>
              <div className="dim" style={{ fontSize: '.75rem' }}>{s.nis}</div>
            </div>
            <b style={{ minWidth: 34, textAlign: 'right' }}>{skor[s.id] || 0}</b>
            <button className="btn btn-sm btn-ok" onClick={() => add(s.id, poin)}>+{poin}</button>
            <button className="btn btn-sm btn-ghost" onClick={() => add(s.id, -poin)}>−</button>
          </div>
        ))}
        {!students.length && <div className="empty">Pilih rombel untuk memuat siswa.</div>}
      </div>
      <div className="btn-row">
        <button className="btn btn-ghost btn-sm" onClick={salinRekap}>📋 Salin rekap</button>
        <button className="btn btn-danger btn-sm" onClick={reset}>Reset</button>
      </div>
    </div>
  )
}
