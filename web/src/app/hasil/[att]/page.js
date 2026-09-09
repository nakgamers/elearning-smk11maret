'use client'
import { useEffect, useState } from 'react'
import { useParams } from 'next/navigation'
import { api } from '../../lib/api'

// Halaman hasil ujian — meniru hasil.php CBT: score-hero + pill benar/salah/kosong.
export default function Hasil() {
  const { att } = useParams()
  const [r, setR] = useState(null)
  const [err, setErr] = useState('')

  useEffect(() => {
    if (!att) return
    api.get(`/my/attempts/${att}/hasil`).then(setR).catch((e) => {
      // fallback: skor dari querystring kalau endpoint gagal
      const p = new URLSearchParams(window.location.search)
      const s = p.get('skor')
      if (s !== null) setR({ skor: +s, exam: 'Ujian' })
      else setErr(e.message)
    })
  }, [att])

  function fmtDate(s) {
    if (!s) return '—'
    try {
      return new Date(s).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' })
    } catch { return s }
  }

  if (err) {
    return (
      <div style={{ minHeight: '100vh', display: 'grid', placeItems: 'center', padding: 16 }}>
        <div className="card" style={{ padding: 24, maxWidth: 420 }}>{err}</div>
      </div>
    )
  }
  if (!r) {
    return <div style={{ minHeight: '100vh', display: 'grid', placeItems: 'center' }}>Memuat hasil…</div>
  }

  return (
    <div style={{ background: 'var(--bg)', minHeight: '100vh', padding: '1.25rem' }}>
      <div className="card animate-in" style={{ maxWidth: 720, margin: '0 auto' }}>
        <div className="card-head">
          <h2>Hasil Ujian</h2>
          <span className="badge badge-selesai">Selesai</span>
        </div>
        <div className="card-body">
          <div className="score-hero">
            <div className="small muted">{r.exam}{r.mapel ? ` · ${r.mapel}` : ''}</div>
            <div className="n">{r.skor}</div>
            <div className="small muted">Nilai akhir (skala 100)</div>
            {r.benar !== undefined && (
              <div className="score-pills">
                <span className="pill ok">Benar {r.benar}</span>
                <span className="pill bad">Salah {r.salah}</span>
                <span className="pill">Kosong {r.kosong}</span>
              </div>
            )}
          </div>

          {r.siswa && (
            <table className="tbl tbl-kv small">
              <tbody>
                <tr><th style={{ width: '40%' }}>Nama</th><td>{r.siswa.nama} ({r.siswa.nis})</td></tr>
                <tr><th>Kelas</th><td>{r.siswa.kelas}</td></tr>
                <tr><th>Mulai</th><td>{fmtDate(r.mulai_at)}</td></tr>
                <tr><th>Dikumpulkan</th><td>{fmtDate(r.selesai_at)}</td></tr>
              </tbody>
            </table>
          )}

          <div className="btn-row mt">
            <a className="btn btn-ghost" href="/siswa">← Daftar Ujian</a>
            <button className="btn btn-ghost no-print" onClick={() => window.print()}>Cetak</button>
          </div>
        </div>
      </div>
    </div>
  )
}
