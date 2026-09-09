'use client'
import { useEffect } from 'react'
import { useRouter } from 'next/navigation'

export default function Home() {
  const router = useRouter()
  useEffect(() => {
    const t = localStorage.getItem('token')
    const u = localStorage.getItem('user')
    if (!t) return router.replace('/login')
    try {
      const role = JSON.parse(u).role
      router.replace(role === 'siswa' ? '/siswa' : role === 'guru' ? '/guru' : '/admin')
    } catch {
      router.replace('/login')
    }
  }, [router])
  return <div style={{ minHeight: '100vh', display: 'grid', placeItems: 'center' }}>Memuat…</div>
}