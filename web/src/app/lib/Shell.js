'use client'
import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import { getUser, clearSession } from '../lib/api'

// Shell dashboard dua varian:
//  - guru/admin : sidebar gelap navy + topbar (mirip CBT layout admin)
//  - siswa      : top bar gradient + tab pills (mirip CBT layout student)
export default function Shell({ tabs, active, onTab, title, subtitle, children }) {
  const router = useRouter()
  const [user, setUser] = useState(null)
  useEffect(() => {
    const u = getUser()
    if (!u) router.replace('/login')
    else setUser(u)
  }, [router])

  function logout() {
    clearSession()
    router.replace('/login')
  }

  if (!user) return <div style={{ minHeight: '100vh', display: 'grid', placeItems: 'center' }}>Memuat…</div>

  const roleLabel = { admin: 'Administrator', guru: 'Guru', siswa: 'Siswa' }
  const current = tabs.find((t) => t.id === active) || tabs[0]

  if (user.role === 'siswa') {
    return (
      <div style={{ background: 'var(--bg)', minHeight: '100vh' }}>
        <div className="stu-top">
          <div className="logo">11</div>
          <div style={{ fontWeight: 800, fontSize: '.95rem', letterSpacing: '.3px' }}>SMK 11 MARET</div>
          <div className="me">
            <b>{user.nama}</b>
            <span>{user.kelas || 'Siswa'}</span>
          </div>
          <button className="out" onClick={logout}>Keluar</button>
        </div>
        <div className="stu-wrap">
          <div className="tab-row">
            {tabs.map((t) => (
              <button key={t.id} className={'tab-btn' + (active === t.id ? ' active' : '')}
                onClick={() => onTab(t.id)}>
                {t.icon && <span style={{ marginRight: 5 }}>{t.icon}</span>}{t.label}
              </button>
            ))}
          </div>
          <div className="animate-in" key={active}>{children}</div>
        </div>
      </div>
    )
  }

  return (
    <div className="shell">
      <aside className="sidebar">
        <div className="brand">
          <div className="logo">11</div>
          <div>
            <b>SMK 11 MARET</b>
            <span>{roleLabel[user.role]}</span>
          </div>
        </div>
        <nav className="nav">
          {tabs.map((t) => (
            <button key={t.id} className={active === t.id ? 'on' : ''} onClick={() => onTab(t.id)}>
              {t.icon && <span style={{ fontSize: 17 }}>{t.icon}</span>}{t.label}
            </button>
          ))}
        </nav>
        <div className="foot">
          <div className="who">
            <b>{user.nama}</b>
            <span>{roleLabel[user.role]}{user.mapel_id ? ` · Mapel` : ''}</span>
          </div>
          <button className="logout" onClick={logout}><span>Keluar akun</span></button>
        </div>
      </aside>
      <div className="main">
        <div className="topbar">
          <div className="title">{title || current?.label || ''}</div>
          <div className="who"><b>{user.nama}</b><span>{roleLabel[user.role]}</span></div>
        </div>
        <div className="content">
          <div className="animate-in" key={active}>{children}</div>
        </div>
      </div>
    </div>
  )
}