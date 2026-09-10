'use client'
import { useEffect, useState } from 'react'
import Shell from '../lib/Shell'
import { api } from '../lib/api'

const TABS = [
  { id: 'materi', label: 'Materi', icon: '📚' },
  { id: 'tugas', label: 'Tugas & Nilai', icon: '📝' },
  { id: 'absen', label: 'Absensi', icon: '🗓️' },
  { id: 'ujian', label: 'Ujian', icon: '📋' },
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

function SubRow({ s, onSave }) {
  const [nilai, setNilai] = useState(s.nilai ?? '')
  const [fb, setFb] = useState(s.feedback ?? '')
  return (
    <tr>
      <td>{s.nis}</td>
      <td>{s.nama}</td>
      <td style={{ maxWidth: 260 }}>{s.jawaban || (s.file_url ? <a href={s.file_url} target="_blank" rel="noreferrer">📎 file</a> : '—')}</td>
      <td><input className="input" style={{ width: 70, padding: '4px 8px' }} type="number" value={nilai} onChange={(e) => setNilai(e.target.value)} /></td>
      <td><input className="input" style={{ width: 150, padding: '4px 8px' }} value={fb} onChange={(e) => setFb(e.target.value)} /></td>
      <td><button className="btn btn-primary" style={{ padding: '4px 10px', fontSize: 12 }} onClick={() => onSave(s.id, nilai, fb)}>Simpan</button></td>
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
  useEffect(() => { if (kelas) api.get(`/students?kelas=${encodeURIComponent(kelas)}`).then(setStudents).catch(() => {}) }, [kelas])
  const [busy, setBusy] = useState(false)

  async function setAbsen(sid, status) {
    setBusy(true)
    try {
      await api.post('/attendance', { student_id: sid, tanggal, status, mapel_id: mapelID })
      reload()
    } finally { setBusy(false) }
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
      </div>
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
                    <td>{s.nis}</td>
                    <td>{s.nama}</td>
                    <td>
                      {st ? { hadir: '✅ Hadir', izin: '📝 Izin', sakit: '🤒 Sakit', alpa: '❌ Alpa' }[st] : <span style={{ color: '#94a3b8' }}>— belum tercatat</span>}
                    </td>
                    <td>
                      <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
                        {[['hadir', 'Hadir'], ['izin', 'Izin'], ['sakit', 'Sakit'], ['alpa', 'Alpa']].map(([v, l]) => (
                          <button key={v} className="btn btn-ghost" disabled={busy} style={{ padding: '4px 8px', fontSize: 11.5 }} onClick={() => setAbsen(s.id, v)}>{l}</button>
                        ))}
                      </div>
                    </td>
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