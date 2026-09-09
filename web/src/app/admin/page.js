'use client'
import { useEffect, useState } from 'react'
import Shell from '../lib/Shell'
import { api } from '../lib/api'

const TABS = [
  { id: 'mapel', label: 'Mapel & CP', icon: '🗂️' },
  { id: 'guru', label: 'Guru', icon: '👨‍🏫' },
  { id: 'siswa', label: 'Siswa', icon: '🎓' },
  { id: 'nilai', label: 'Export Nilai', icon: '📊' },
  { id: 'umum', label: 'Pengumuman', icon: '📢' },
]

export default function Admin() {
  const [tab, setTab] = useState('mapel')
  return (
    <Shell tabs={TABS} active={tab} onTab={setTab}
      title={TABS.find((t) => t.id === tab)?.label}
      subtitle="Dashboard Admin">
      {tab === 'mapel' && <Mapel />}
      {tab === 'guru' && <Guru />}
      {tab === 'siswa' && <Siswa />}
      {tab === 'nilai' && <Nilai />}
      {tab === 'umum' && <Umum />}
    </Shell>
  )
}

function useList(path) {
  const [data, setData] = useState([])
  const [err, setErr] = useState('')
  const reload = () => api.get(path).then(setData).catch((e) => setErr(e.message))
  useEffect(() => { reload() }, [path])
  return { data, err, reload }
}

function Mapel() {
  const { data: subjects, reload: rSubj } = useList('/subjects')
  const { data: cps, reload: rCps } = useList('/cps')
  const [f, setF] = useState({ nama: '', kode: '' })
  const [cp, setCp] = useState({ mapel_id: '', fase: 'E', elemen: '', deskripsi: '' })
  const [msg, setMsg] = useState('')

  async function addSubject(e) {
    e.preventDefault(); setMsg('')
    try { await api.post('/subjects', f); setF({ nama: '', kode: '' }); rSubj() }
    catch (e) { setMsg('❌ ' + e.message) }
  }
  async function addCP(e) {
    e.preventDefault(); setMsg('')
    try { await api.post('/cps', { ...cp, mapel_id: +cp.mapel_id }); setCp({ mapel_id: '', fase: 'E', elemen: '', deskripsi: '' }); rCps() }
    catch (e) { setMsg('❌ ' + e.message) }
  }

  return (
    <div style={{ display: 'grid', gap: 14 }}>
      <div className="grid-2" style={{ alignItems: 'start' }}>
        <form className="card" style={{ padding: 18, display: 'grid', gap: 10 }} onSubmit={addSubject}>
          <div style={{ fontWeight: 700 }}>Tambah Mata Pelajaran</div>
          <div><label className="label">Nama</label><input className="input" required value={f.nama} onChange={(e) => setF({ ...f, nama: e.target.value })} /></div>
          <div><label className="label">Kode (opsional)</label><input className="input" value={f.kode} onChange={(e) => setF({ ...f, kode: e.target.value })} /></div>
          <button className="btn btn-primary" style={{ justifySelf: 'start' }}>Tambah</button>
        </form>

        <form className="card" style={{ padding: 18, display: 'grid', gap: 10 }} onSubmit={addCP}>
          <div style={{ fontWeight: 700 }}>Tambah Capaian Pembelajaran (CP)</div>
          <div><label className="label">Mapel</label>
            <select className="input" required value={cp.mapel_id} onChange={(e) => setCp({ ...cp, mapel_id: e.target.value })}>
              <option value="">Pilih…</option>
              {subjects.map((s) => <option key={s.id} value={s.id}>{s.nama}</option>)}
            </select>
          </div>
          <div><label className="label">Elemen</label><input className="input" value={cp.elemen} placeholder="cth. Bilangan" onChange={(e) => setCp({ ...cp, elemen: e.target.value })} /></div>
          <div><label className="label">Deskripsi CP</label><textarea className="input" rows={2} required value={cp.deskripsi} onChange={(e) => setCp({ ...cp, deskripsi: e.target.value })} /></div>
          <button className="btn btn-primary" style={{ justifySelf: 'start' }}>Tambah CP</button>
        </form>
      </div>
      {msg && <div className="card" style={{ padding: 12, fontSize: 14 }}>{msg}</div>}

      <div className="card" style={{ padding: 16 }}>
        <div style={{ fontWeight: 700, marginBottom: 10 }}>Capaian Pembelajaran (per mapel)</div>
        <table className="tbl">
          <thead><tr><th>Mapel</th><th>Fase</th><th>Elemen</th><th>CP</th></tr></thead>
          <tbody>{cps.map((c) => (
            <tr key={c.id}><td>{c.mapel}</td><td>{c.fase}</td><td>{c.elemen}</td><td>{c.deskripsi}</td></tr>
          ))}</tbody>
        </table>
      </div>
    </div>
  )
}

function Guru() {
  const { data, reload, err } = useList('/teachers')
  const { data: subjects } = useList('/subjects')
  const [f, setF] = useState({ nama: '', username: '', password: '', mapel_id: '' })
  const [msg, setMsg] = useState('')

  async function submit(e) {
    e.preventDefault(); setMsg('')
    try {
      await api.post('/teachers', { ...f, mapel_id: +f.mapel_id || 0 })
      setMsg('✅ Guru ditambahkan.')
      setF({ nama: '', username: '', password: '', mapel_id: '' })
      reload()
    } catch (e) { setMsg('❌ ' + e.message) }
  }

  return (
    <div style={{ display: 'grid', gap: 14 }}>
      <form className="card" style={{ padding: 18, display: 'grid', gap: 12 }} onSubmit={submit}>
        <div style={{ fontWeight: 700 }}>Tambah Guru</div>
        <div className="grid-2">
          <div><label className="label">Nama lengkap</label><input className="input" required value={f.nama} onChange={(e) => setF({ ...f, nama: e.target.value })} /></div>
          <div><label className="label">Mapel diampu</label>
            <select className="input" value={f.mapel_id} onChange={(e) => setF({ ...f, mapel_id: e.target.value })}>
              <option value="">— umum —</option>
              {subjects.map((s) => <option key={s.id} value={s.id}>{s.nama}</option>)}
            </select>
          </div>
          <div><label className="label">Username</label><input className="input" required value={f.username} onChange={(e) => setF({ ...f, username: e.target.value })} /></div>
          <div><label className="label">Password (min 6)</label><input className="input" required type="password" value={f.password} onChange={(e) => setF({ ...f, password: e.target.value })} /></div>
        </div>
        {msg && <div style={{ fontSize: 14 }}>{msg}</div>}
        <button className="btn btn-primary" style={{ justifySelf: 'start' }}>Tambah Guru</button>
      </form>

      <div className="card" style={{ padding: 16 }}>
        <div style={{ fontWeight: 700, marginBottom: 10 }}>Daftar Guru</div>
        {err && <div>{err}</div>}
        <table className="tbl">
          <thead><tr><th>Nama</th><th>Username</th><th>Mapel diampu</th></tr></thead>
          <tbody>{data.map((g) => (
            <tr key={g.id}><td style={{ fontWeight: 600 }}>{g.nama}</td><td>{g.username}</td><td>{g.mapel || 'Umum'}</td></tr>
          ))}</tbody>
        </table>
      </div>
    </div>
  )
}

function Siswa() {
  const [kelas, setKelas] = useState('')
  const [shown, setShown] = useState([])
  const [err, setErr] = useState('')
  const [file, setFile] = useState(null)
  const [msg, setMsg] = useState('')

  const load = () => api.get(`/students${kelas ? '?kelas=' + encodeURIComponent(kelas) : ''}`).then(setShown).catch((e) => setErr(e.message))
  useEffect(() => { load() }, [kelas])

  async function doImport() {
    setMsg('')
    if (!file) return setMsg('❌ Pilih file xlsx dulu.')
    const fd = new FormData()
    fd.append('file', file)
    const r = await api.postForm('/students/import', fd)
    setMsg(`✅ ${r.imported} siswa diimpor${r.skipped ? `, ${r.skipped} dilewati` : ''}.`)
    load()
    setFile(null)
  }

  async function remove(id) {
    await api.del(`/students/${id}`)
    load()
  }

  return (
    <div style={{ display: 'grid', gap: 14 }}>
      <div className="card" style={{ padding: 18, display: 'grid', gap: 10 }}>
        <div style={{ fontWeight: 700 }}>Import Siswa</div>
        <p style={{ fontSize: 13, color: '#64748b', margin: 0 }}>
          Format xlsx: baris 1 header <b>NIS | Nama | Kelas | JK</b>. Password awal: <b>siswa123</b>. Import ulang NIS yang sama = update data.
        </p>
        <input className="input" type="file" accept=".xlsx" onChange={(e) => setFile(e.target.files[0])} />
        {msg && <div style={{ fontSize: 14 }}>{msg}</div>}
        <div style={{ display: 'flex', gap: 8 }}>
          <button className="btn btn-primary" style={{ padding: '8px 16px' }} onClick={doImport}>Import xlsx</button>
        </div>
      </div>

      <div className="card" style={{ padding: 16 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 10, alignItems: 'center', flexWrap: 'wrap', gap: 8 }}>
          <div style={{ fontWeight: 700 }}>Data Siswa</div>
          <input className="input" style={{ width: 180 }} placeholder="Filter kelas…" value={kelas} onChange={(e) => setKelas(e.target.value)} />
        </div>
        {err && <div>{err}</div>}
        <table className="tbl">
          <thead><tr><th>NIS</th><th>Nama</th><th>Kelas</th><th>JK</th><th></th></tr></thead>
          <tbody>{shown.map((s) => (
            <tr key={s.id}>
              <td>{s.nis}</td><td>{s.nama}</td><td>{s.kelas}</td><td>{s.jk}</td>
              <td><button className="btn btn-danger" style={{ padding: '3px 8px', fontSize: 11.5 }} onClick={() => remove(s.id)}>Nonaktif</button></td>
            </tr>
          ))}</tbody>
        </table>
      </div>
    </div>
  )
}

function Nilai() {
  const [kelas, setKelas] = useState('')
  const [msg, setMsg] = useState('')
  async function exportCsv() {
    setMsg('')
    try {
      await api.download(`/grades/export${kelas ? '?kelas=' + encodeURIComponent(kelas) : ''}`)
      setMsg('✅ File nilai diunduh.')
    } catch (e) { setMsg('❌ ' + e.message) }
  }
  return (
    <div className="card" style={{ padding: 20 }}>
      <div style={{ fontWeight: 700, marginBottom: 10 }}>Export Nilai (xlsx)</div>
      <p style={{ fontSize: 13, color: '#64748b' }}>Berisi rekap nilai tugas yang sudah dinilai, kolom: NIS, Nama, Kelas, Mapel, Tugas, Nilai, Waktu.</p>
      <input className="input" style={{ width: 200, marginBottom: 12 }} placeholder="Filter kelas (opsional)" value={kelas} onChange={(e) => setKelas(e.target.value)} />
      <div><button className="btn btn-primary" onClick={exportCsv}>⬇️ Unduh xlsx</button></div>
      {msg && <div style={{ fontSize: 14, marginTop: 10 }}>{msg}</div>}
    </div>
  )
}

function Umum() {
  const { data, reload } = useList('/news')
  const [f, setF] = useState({ judul: '', isi: '' })
  const [msg, setMsg] = useState('')
  async function submit(e) {
    e.preventDefault(); setMsg('')
    try { await api.post('/news', f); setMsg('✅ Terbit.'); setF({ judul: '', isi: '' }); reload() }
    catch (e) { setMsg('❌ ' + e.message) }
  }
  async function toggle(id) { await api.post(`/news/${id}/toggle`, {}); reload() }

  return (
    <div style={{ display: 'grid', gap: 14 }}>
      <form className="card" style={{ padding: 18, display: 'grid', gap: 12 }} onSubmit={submit}>
        <div style={{ fontWeight: 700 }}>Tulis Pengumuman</div>
        <div><label className="label">Judul</label><input className="input" required value={f.judul} onChange={(e) => setF({ ...f, judul: e.target.value })} /></div>
        <div><label className="label">Isi</label><textarea className="input" rows={4} required value={f.isi} onChange={(e) => setF({ ...f, isi: e.target.value })} /></div>
        {msg && <div style={{ fontSize: 14 }}>{msg}</div>}
        <button className="btn btn-primary" style={{ justifySelf: 'start' }}>Terbitkan</button>
      </form>

      <div className="card" style={{ padding: 16 }}>
        <div style={{ fontWeight: 700, marginBottom: 10 }}>Semua Pengumuman</div>
        <table className="tbl">
          <thead><tr><th>Judul</th><th>Penulis</th><th>Status</th><th></th></tr></thead>
          <tbody>{data.map((n) => (
            <tr key={n.id}>
              <td style={{ fontWeight: 600 }}>{n.judul}</td>
              <td>{n.author}</td>
              <td>{n.aktif !== false ? <span className="badge" style={{ background: '#dcfce7', color: '#15803d' }}>Aktif</span> : <span className="badge" style={{ background: '#f1f5f9', color: '#64748b' }}>Nonaktif</span>}</td>
              <td><button className="btn btn-ghost" style={{ padding: '3px 10px', fontSize: 11.5 }} onClick={() => toggle(n.id)}>{n.aktif !== false ? 'Sembunyikan' : 'Tampilkan'}</button></td>
            </tr>
          ))}</tbody>
        </table>
      </div>
    </div>
  )
}