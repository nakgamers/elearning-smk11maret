'use client'
import { useEffect, useState, useRef, useCallback } from 'react'
import { useRouter, useParams } from 'next/navigation'
import { api, getUser } from '../../lib/api'

// Halaman ujian siswa — tampilan pengerjaan meniru CBT (kerjakan.php):
// satu soal per layar, navigasi grid di samping, timer dari server,
// autosave per jawaban + sync berkala, tanda ragu, keyboard 1-5 & panah.

const OPSI_LABEL = ['A', 'B', 'C', 'D', 'E']

export default function UjianPage() {
  const { id } = useParams()
  const router = useRouter()
  const [phase, setPhase] = useState('info') // info | kerja
  const [info, setInfo] = useState(null)     // data ujian utk layar mulai
  const [exam, setExam] = useState(null)     // hasil POST /start
  const [err, setErr] = useState('')
  const user = getUser()

  useEffect(() => {
    if (!id) return
    api.get('/my/exams')
      .then((l) => setInfo((l || []).find((e) => String(e.id) === String(id)) || null))
      .catch(() => setInfo(null))
  }, [id])

  async function mulai() {
    setErr('')
    try {
      const e = await api.post(`/exams/${id}/start`, {})
      setExam(e)
      setPhase('kerja')
    } catch (e) {
      // ujian sudah dikerjakan → arahkan langsung ke hasil
      const m = (e.message || '').trim()
      const j = m.startsWith('{') ? (() => { try { return JSON.parse(m) } catch { return null } })() : null
      if (j && j.attempt_id) {
        router.replace(`/hasil/${j.attempt_id}`)
        return
      }
      setErr(e.message)
    }
  }

  // ---------- layar mulai (meniru CBT mulai.php) ----------
  if (phase === 'info') {
    return (
      <div style={{ background: 'var(--bg)', minHeight: '100vh', display: 'grid', placeItems: 'center', padding: 16 }}>
        <div className="card animate-in" style={{ maxWidth: 480, width: '100%', padding: '1.6rem' }}>
          <div className="auth-badge" style={{ marginBottom: '.6rem' }}>Ujian Online</div>
          <h1 style={{ margin: '0 0 .2rem', fontSize: '1.35rem' }}>{info?.nama || 'Memuat…'}</h1>
          <div className="muted" style={{ marginBottom: '1rem' }}>{info?.mapel || ''}</div>
          <table className="tbl" style={{ marginBottom: '1.1rem' }}>
            <tbody>
              <tr><td style={{ color: 'var(--muted)' }}>Durasi</td><td><b>{info ? info.durasi_menit + ' menit' : '—'}</b></td></tr>
              <tr><td style={{ color: 'var(--muted)' }}>Jumlah soal</td><td><b>{info?.jumlah_soal ?? '—'}</b></td></tr>
              <tr><td style={{ color: 'var(--muted)' }}>Sifat</td><td>Waktu berjalan dari server, jawaban tersimpan otomatis</td></tr>
            </tbody>
          </table>
          <div className="alert" style={{ marginBottom: '1rem' }}>
            ⚠️ Jangan keluar dari halaman ini selama ujian berlangsung. Jawaban yang sudah dipilih akan tersimpan otomatis.
          </div>
          {err && <div className="alert alert-error">{err}</div>}
          <button className="btn btn-block" onClick={mulai} disabled={!info}>Mulai Ujian →</button>
          <button className="btn btn-ghost btn-block" style={{ marginTop: '.5rem' }} onClick={() => router.push('/siswa')}>Kembali</button>
        </div>
      </div>
    )
  }

  // ---------- layar kerja ----------
  return <Kerjakan exam={exam} user={user} onError={(m) => setErr(m)} onDone={(att, skor) => router.replace(`/hasil/${att}?skor=${skor}`)} />
}

function Kerjakan({ exam, user, onDone }) {
  const [cur, setCur] = useState(0)
  const [answers, setAnswers] = useState({})   // qid -> index opsi
  const [ragu, setRagu] = useState({})         // qid -> bool (lokal saja)
  const [saveState, setSaveState] = useState({ txt: 'Tersimpan otomatis', cls: '' })
  const [timeLeft, setTimeLeft] = useState(null)
  const [finishing, setFinishing] = useState(false)
  const answersRef = useRef({})
  answersRef.current = answers
  const examRef = useRef(exam)
  examRef.current = exam

  // restore jawaban lokal + jawaban dari server (mine)
  useEffect(() => {
    const saved = {}
    exam.questions.forEach((q) => { if (q.mine >= 0) saved[q.id] = q.mine })
    try {
      const local = JSON.parse(localStorage.getItem('exam-answers') || '{}')
      if (local[exam.attempt_id]) Object.assign(saved, local[exam.attempt_id])
    } catch {}
    setAnswers(saved)
  }, [exam])

  // timer dihitung dari mulai_at server (resume-safe), bukan jam lokal
  useEffect(() => {
    if (!exam.mulai_at) { setTimeLeft(exam.durasi_menit * 60); return }
    const mulai = new Date(exam.mulai_at).getTime()
    const total = exam.durasi_menit * 60
    const calc = () => Math.max(0, total - Math.floor((Date.now() - mulai) / 1000))
    setTimeLeft(calc())
    const t = setInterval(() => setTimeLeft(calc()), 1000)
    return () => clearInterval(t)
  }, [exam])

  const kirimBatch = useCallback(async (list) => {
    const att = examRef.current?.attempt_id
    if (!att || !list.length) return false
    setSaveState({ txt: 'menyimpan…', cls: '' })
    try {
      await api.post(`/attempts/${att}/answers`, { answers: list })
      setSaveState({ txt: 'tersimpan ✓', cls: 'ok' })
      return true
    } catch {
      setSaveState({ txt: 'koneksi terputus — jawaban belum tersimpan', cls: 'bad' })
      return false
    }
  }, [])

  const kirimSatu = useCallback((qid, jawaban) => kirimBatch([{ question_id: +qid, jawaban }]), [kirimBatch])

  // sync berkala 15 detik (jaring pengaman, master plan §2.2)
  useEffect(() => {
    const t = setInterval(() => {
      const list = Object.entries(answersRef.current).map(([qid, jawaban]) => ({ question_id: +qid, jawaban }))
      if (list.length) kirimBatch(list)
    }, 15000)
    return () => clearInterval(t)
  }, [kirimBatch])

  function pick(qid, idx) {
    const next = { ...answersRef.current, [qid]: idx }
    setAnswers(next)
    simpanLokal(next)
    kirimSatu(qid, idx)
  }

  function kosongkan(qid) {
    const next = { ...answersRef.current }
    delete next[qid]
    setAnswers(next)
    simpanLokal(next)
    kirimSatu(qid, -1) // -1 = dikosongkan
  }

  function simpanLokal(next) {
    try {
      const local = JSON.parse(localStorage.getItem('exam-answers') || '{}')
      local[exam.attempt_id] = next
      localStorage.setItem('exam-answers', JSON.stringify(local))
    } catch {}
  }

  function toggleRagu(qid) {
    setRagu((r) => ({ ...r, [qid]: !r[qid] }))
  }

  // keyboard: panah pindah soal, 1-5 pilih opsi (seperti CBT)
  useEffect(() => {
    function onKey(e) {
      if (e.target.matches('input,textarea')) return
      if (e.key === 'ArrowRight') setCur((c) => Math.min(c + 1, exam.questions.length - 1))
      if (e.key === 'ArrowLeft') setCur((c) => Math.max(c - 1, 0))
      const idx = ['1', '2', '3', '4', '5'].indexOf(e.key)
      if (idx > -1) {
        const q = examRef.current?.questions[curRef.current]
        if (q && q.opsi[idx] !== undefined) pick(q.id, idx)
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])
  const curRef = useRef(cur)
  curRef.current = cur

  // sebelum tutup tab: peringatkan (CBT beforeunload)
  useEffect(() => {
    function bu(e) { e.preventDefault(); e.returnValue = '' }
    window.addEventListener('beforeunload', bu)
    return () => window.removeEventListener('beforeunload', bu)
  }, [])

  async function kumpul(auto = false) {
    if (finishing) return
    const total = exam.questions.length
    const sudah = Object.keys(answersRef.current).length
    if (!auto) {
      const msg = sudah < total
        ? `Masih ada ${total - sudah} soal belum dijawab. Tetap kumpulkan sekarang?`
        : 'Kumpulkan jawaban dan akhiri ujian?'
      if (!window.confirm(msg)) return
    }
    setFinishing(true)
    const list = Object.entries(answersRef.current).map(([qid, jawaban]) => ({ question_id: +qid, jawaban }))
    if (list.length) await kirimBatch(list)
    try {
      const r = await api.post(`/attempts/${exam.attempt_id}/finish`, {})
      try { localStorage.removeItem('exam-answers') } catch {}
      onDone(exam.attempt_id, r.skor)
    } catch (e) {
      setSaveState({ txt: e.message, cls: 'bad' })
      setFinishing(false)
    }
  }

  // auto-kumpul saat waktu habis (seperti CBT)
  useEffect(() => {
    if (timeLeft === 0) kumpul(true)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [timeLeft])

  if (timeLeft == null) return <Center>Menyiapkan ujian…</Center>

  const total = exam.questions.length
  const q = exam.questions[cur]
  const sudah = Object.keys(answers).length
  const h = Math.floor(timeLeft / 3600), m = Math.floor((timeLeft % 3600) / 60), s = timeLeft % 60
  const p = (n) => String(n).padStart(2, '0')

  return (
    <div style={{ background: 'var(--bg)', minHeight: '100vh' }}>
      <div className="exam-bar no-print">
        <div className="nm">
          <b>{exam.nama}</b>
          <small>{user?.nama || ''}{user?.kelas ? ` · ${user.kelas}` : ''}</small>
        </div>
        <span className={'save-pill' + (saveState.cls ? ' ' + saveState.cls : '')}>{saveState.txt}</span>
        <div id="timer" className={timeLeft <= 60 ? 'crit' : timeLeft <= 300 ? 'warn' : ''}>
          {(h > 0 ? p(h) + ':' : '') + p(m) + ':' + p(s)}
        </div>
        <button className="btn btn-ghost btn-sm" type="button" onClick={() => kumpul()} disabled={finishing}>Kumpulkan</button>
      </div>

      <div className="exam-layout">
        <div>
          <div className="q-card animate-in" key={q.id}>
            <div className="q-head">
              <div className="no">{cur + 1}</div>
              <div className="of">Soal {cur + 1} dari {total}</div>
              <label className="check ragu-lbl">
                <input type="checkbox" checked={!!ragu[q.id]} onChange={() => toggleRagu(q.id)} />
                <span className="small">Tandai ragu</span>
              </label>
            </div>
            <div className="q-body">
              <div className="q-text">{q.soal}</div>
              <div role="radiogroup">
                {q.opsi.map((o, oi) => (
                  <label key={oi} className={'opt' + (answers[q.id] === oi ? ' sel' : '')}>
                    <input type="radio" name={`q${q.id}`} checked={answers[q.id] === oi} onChange={() => pick(q.id, oi)} />
                    <span className="k">{OPSI_LABEL[oi]}</span>
                    <span>{o}</span>
                  </label>
                ))}
              </div>
            </div>
            <div className="q-foot">
              <button className="btn btn-ghost btn-sm" type="button" onClick={() => setCur(cur - 1)} disabled={cur === 0}>← Sebelumnya</button>
              <span className="sp"></span>
              {cur + 1 < total ? (
                <button className="btn btn-sm" type="button" onClick={() => setCur(cur + 1)}>Berikutnya →</button>
              ) : (
                <button className="btn btn-sm btn-ok" type="button" onClick={() => kumpul()} disabled={finishing}>Kumpulkan Jawaban</button>
              )}
              <button className="btn btn-ghost btn-sm ksg" type="button" onClick={() => kosongkan(q.id)} disabled={answers[q.id] === undefined}>Kosongkan pilihan</button>
            </div>
          </div>
        </div>

        <div className="side no-print">
          <div className="h">Navigasi Soal</div>
          <div className="nav-grid">
            {exam.questions.map((qq, i) => {
              const cls = [
                answers[qq.id] !== undefined ? 'done' : '',
                ragu[qq.id] ? 'ragu' : '',
                i === cur ? 'now' : '',
              ].join(' ')
              return (
                <button key={qq.id} type="button" className={cls.trim()}
                  aria-label={`Soal ${i + 1}${answers[qq.id] !== undefined ? ', sudah dijawab' : ', belum dijawab'}`}
                  onClick={() => { setCur(i); window.scrollTo({ top: 0, behavior: 'smooth' }) }}>
                  {i + 1}
                </button>
              )
            })}
          </div>
          <div className="legend">
            <div><i style={{ background: '#d1fae5', borderColor: '#6ee7b7' }}></i> <b>✓</b> Sudah dijawab</div>
            <div><i style={{ background: '#fef3c7', borderColor: '#fcd34d' }}></i> <b>?</b> Ditandai ragu</div>
            <div><i style={{ background: '#fff' }}></i> Belum dijawab</div>
            <div><i style={{ background: '#fff', outline: '2px solid var(--brand)', outlineOffset: 1 }}></i> Soal yang dibuka</div>
          </div>
          <div className="foot">
            <div className="small muted" style={{ marginBottom: '.4rem' }}>Terjawab <b>{sudah}</b> / {total}</div>
            <div className="bar-wrap" role="progressbar" aria-valuemin={0} aria-valuemax={total} aria-valuenow={sudah}>
              <div className="bar" style={{ width: total ? (sudah / total * 100) + '%' : 0 }}></div>
            </div>
            <button className="btn btn-danger btn-block" type="button" onClick={() => kumpul()} disabled={finishing}>
              {finishing ? 'Mengumpulkan…' : 'Kumpulkan Jawaban'}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}

function Center({ children }) {
  return <div style={{ minHeight: '100vh', display: 'grid', placeItems: 'center', padding: 16 }}>{children}</div>
}
