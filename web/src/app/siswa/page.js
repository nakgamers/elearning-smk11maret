'use client'
import { useEffect, useState, useCallback } from 'react'
import { useRouter } from 'next/navigation'
import Shell from '../lib/Shell'
import { api } from '../lib/api'

const TABS = [
  { id: 'beranda', label: 'Beranda', icon: '🏠' },
  { id: 'materi', label: 'Materi', icon: '📚' },
  { id: 'tugas', label: 'Tugas', icon: '📝' },
  { id: 'ujian', label: 'Ujian', icon: '📋' },
  { id: 'absen', label: 'Absensi', icon: '🗓️' },
]

export default function Siswa() {
  const router = useRouter()
  const [tab, setTab] = useState('beranda')

  return (
    <Shell tabs={TABS} active={tab} onTab={setTab}>
      {tab === 'beranda' && <Beranda />}
      {tab === 'materi' && <Materi />}
      {tab === 'tugas' && <Tugas />}
      {tab === 'ujian' && <Ujian goExam={(id) => router.push(`/ujian/${id}`)} />}
      {tab === 'absen' && <Absen />}
    </Shell>
  )
}

function useFetch(path, deps = []) {
  const [data, setData] = useState(null)
  const [err, setErr] = useState('')
  const [loading, setLoading] = useState(true)
  useEffect(() => {
    let alive = true
    api.get(path)
      .then((d) => alive && setData(d))
      .catch((e) => alive && setErr(e.message))
      .finally(() => alive && setLoading(false))
    return () => { alive = false }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [path, ...deps])
  return { data, err, loading, reload: () => setLoading(true) }
}

function Beranda() {
  const dash = useFetch('/my/dashboard')
  const news = useFetch('/news')
  return (
    <div>
      <div className="stats">
        <div className="stat">
          <div className="ic">📝</div>
          <div><b>{dash.data?.tugas_belum ?? '—'}</b><span>Tugas belum dikerjakan</span></div>
        </div>
        <div className="stat">
          <div className="ic">📚</div>
          <div><b>{dash.data?.materi ?? '—'}</b><span>Materi tersedia</span></div>
        </div>
        <div className="stat">
          <div className="ic">📋</div>
          <div><b>{dash.data?.ujian_aktif ?? '—'}</b><span>Ujian aktif</span></div>
        </div>
      </div>

      <h3 style={{ fontSize: 15, fontWeight: 700, margin: '6px 0 10px' }}>Pengumuman</h3>
      <div style={{ display: 'grid', gap: 0 }}>
        {news.data?.map((n) => (
          <div key={n.id} className="card">
            <div className="card-body">
              <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
                <span style={{ fontWeight: 700 }}>{n.judul}</span>
                {n.pinned && <span className="badge badge-warn">📌 Disemat</span>}
              </div>
              <p style={{ margin: '0 0 6px', color: 'var(--ink-2)', fontSize: 14, whiteSpace: 'pre-wrap' }}>{n.isi}</p>
              <div style={{ fontSize: 12, color: 'var(--ink-3)' }}>{n.author} · {new Date(n.created_at).toLocaleString('id-ID')}</div>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

function Materi() {
  const { data, loading, err } = useFetch('/my/materials')
  if (loading) return <Div>Memuat…</Div>
  if (err) return <Div>{err}</Div>
  return (
    <div style={{ display: 'grid', gap: 10 }}>
      {data.length === 0 && <Empty>Belum ada materi.</Empty>}
      {data.map((m) => (
        <div key={m.id} className="card" style={{ padding: 16 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', gap: 8, flexWrap: 'wrap' }}>
            <span style={{ fontWeight: 700 }}>{m.judul}</span>
            <span className="badge" style={{ background: '#e0e7ff', color: '#4f46e5' }}>{m.mapel}</span>
          </div>
          {m.cp_elemen && (
            <p style={{ fontSize: 12, color: '#6366f1', margin: '4px 0 0', fontWeight: 600 }}>
              CP: {m.cp_elemen}
            </p>
          )}
          <p style={{ margin: '8px 0', color: '#475569', fontSize: 14, whiteSpace: 'pre-wrap' }}>{m.isi}</p>
          <div style={{ fontSize: 12, color: '#94a3b8' }}>
            Diunggah {m.guru} · {new Date(m.created_at).toLocaleDateString('id-ID')}
          </div>
          {m.file_url && (
            <a className="btn btn-ghost" style={{ marginTop: 10, padding: '6px 12px', fontSize: 12.5 }}
              href={m.file_url} target="_blank" rel="noreferrer">📎 Unduh lampiran</a>
          )}
        </div>
      ))}
    </div>
  )
}

function Tugas() {
  const { data, loading, err } = useFetch('/my/assignments')
  const [openId, setOpenId] = useState(null)
  const [jawaban, setJawaban] = useState('')
  const [file, setFile] = useState(null)
  const [msg, setMsg] = useState('')
  const [busy, setBusy] = useState(false)

  if (loading) return <Div>Memuat…</Div>
  if (err) return <Div>{err}</Div>

  async function submit(aid) {
    setBusy(true)
    setMsg('')
    try {
      const fd = new FormData()
      fd.append('assignment_id', aid)
      fd.append('jawaban', jawaban)
      if (file) fd.append('file', file)
      await api.postForm('/submissions', fd)
      setMsg('✅ Tugas terkirim (bisa dikirim ulang sebelum deadline).')
      setOpenId(null)
      setJawaban('')
      setFile(null)
    } catch (e) {
      setMsg('❌ ' + e.message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div style={{ display: 'grid', gap: 12 }}>
      {data.length === 0 && <Empty>Belum ada tugas.</Empty>}
      {data.map((a) => (
        <div key={a.id} className="card" style={{ padding: 16 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', gap: 8, flexWrap: 'wrap', alignItems: 'center' }}>
            <div>
              <div style={{ fontWeight: 700 }}>{a.judul}</div>
              <div style={{ fontSize: 12, color: '#64748b' }}>
                {a.mapel} · {a.guru}
                {a.deadline ? ` · deadline ${new Date(a.deadline).toLocaleString('id-ID')}` : ''}
              </div>
            </div>
            {a.submitted
              ? <span className="badge" style={{ background: a.nilai != null ? '#dcfce7' : '#e0e7ff', color: a.nilai != null ? '#15803d' : '#4f46e5' }}>
                  {a.nilai != null ? `Nilai ${a.nilai}` : 'Terkumpul'}
                </span>
              : <span className="badge" style={{ background: '#fff1f2', color: '#e11d48' }}>Belum</span>}
          </div>
          {a.deskripsi && <p style={{ margin: '8px 0', color: '#475569', fontSize: 14 }}>{a.deskripsi}</p>}
          {a.feedback && (
            <p style={{ margin: '6px 0', fontSize: 13, background: '#f8fafc', padding: '8px 10px', borderRadius: 8, color: '#334155' }}>
              💬 {a.feedback}
            </p>
          )}
          {a.file_saya && <div style={{ fontSize: 12, color: '#64748b' }}>Kiriman saya: <a href={a.file_saya} target="_blank">lihat</a></div>}
          {openId === a.id ? (
            <div style={{ marginTop: 12, display: 'grid', gap: 10 }}>
              <textarea className="input" rows={4} placeholder="Tulis jawaban…" value={jawaban}
                onChange={(e) => setJawaban(e.target.value)} />
              <input className="input" type="file" onChange={(e) => setFile(e.target.files[0])} />
              <div style={{ display: 'flex', gap: 8 }}>
                <button className="btn btn-primary" disabled={busy} onClick={() => submit(a.id)}>Kirim</button>
                <button className="btn btn-ghost" onClick={() => setOpenId(null)}>Batal</button>
              </div>
            </div>
          ) : (
            <button className="btn btn-ghost" style={{ marginTop: 10, padding: '6px 12px', fontSize: 12.5 }}
              onClick={() => { setOpenId(a.id); setMsg('') }}>
              {a.submitted ? 'Kirim ulang' : 'Kumpulkan tugas'}
            </button>
          )}
        </div>
      ))}
      {msg && <div className="card" style={{ padding: 12, fontSize: 14 }}>{msg}</div>}
    </div>
  )
}

function Ujian({ goExam }) {
  const { data, loading, err } = useFetch('/my/exams')
  if (loading) return <Div>Memuat…</Div>
  if (err) return <Div>{err}</Div>
  return (
    <div style={{ display: 'grid', gap: 12 }}>
      {data.length === 0 && <Empty>Belum ada ujian aktif.</Empty>}
      {data.map((x) => (
        <div key={x.id} className="card" style={{ padding: 16 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
            <div>
              <div style={{ fontWeight: 700 }}>{x.nama}</div>
              <div style={{ fontSize: 12, color: '#64748b' }}>{x.mapel} · {x.durasi_menit} menit</div>
            </div>
            {x.status === 'selesai'
              ? <span className="badge" style={{ background: '#dcfce7', color: '#15803d' }}>Skor {x.skor}</span>
              : <button className="btn btn-primary" onClick={() => goExam(x.id)}
                  style={{ padding: '6px 14px', fontSize: 13 }}>
                  {x.status === 'berjalan' ? 'Lanjutkan' : 'Mulai'}
                </button>}
          </div>
        </div>
      ))}
    </div>
  )
}

function Absen() {
  const [state, setState] = useState({ msg: '', ok: false, busy: false })
  const today = new Date().toLocaleDateString('id-ID', { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' })
  // Cek status kehadiran hari ini → kalau sudah absen, tombol langsung nonaktif (tidak bisa ulang).
  useEffect(() => {
    let on = true
    api.get('/attendance/my/today')
      .then((r) => {
        if (on && r && r.attended) {
          setState((s) => ({ ...s, ok: true, msg: '✅ Anda sudah tercatat hadir hari ini.' }))
        }
      })
      .catch(() => {})
    return () => { on = false }
  }, [])
  async function checkin() {
    setState({ ...state, busy: true, msg: '' })
    try {
      await api.post('/attendance/checkin', {})
      setState({ ok: true, busy: false, msg: '✅ Anda tercatat hadir hari ini.' })
    } catch (e) {
      setState({ ok: false, busy: false, msg: '❌ ' + e.message })
    }
  }
  return (
    <div className="card" style={{ padding: 24, textAlign: 'center' }}>
      <div style={{ fontSize: 44 }}>🗓️</div>
      <div style={{ fontSize: 16, fontWeight: 700, marginTop: 6 }}>{today}</div>
      <p style={{ color: '#64748b', fontSize: 14 }}>Presensi kehadiran harian</p>
      <button className="btn btn-primary" style={{ marginTop: 12 }} disabled={state.busy || state.ok} onClick={checkin}>
        {state.ok ? 'Sudah hadir' : state.busy ? 'Menyimpan…' : 'Hadir sekarang'}
      </button>
      {state.msg && <p style={{ marginTop: 12, fontSize: 14 }}>{state.msg}</p>}
    </div>
  )
}

function Div({ children }) { return <div className="card" style={{ padding: 16 }}>{children}</div> }
function Empty({ children }) {
  return <div className="card" style={{ padding: 32, textAlign: 'center', color: '#94a3b8' }}>{children}</div>
}