import './globals.css'

export const metadata = {
  title: 'SMK 11 MARET',
  description: 'E-Learning SMK 11 MARET: materi, tugas, absensi, ujian online',
}

export const viewport = {
  width: 'device-width',
  initialScale: 1,
  viewportFit: 'cover',
}

export default function RootLayout({ children }) {
  return (
    <html lang="id">
      <head>
        <meta name="theme-color" content="#4f46e5" />
      </head>
      <body>{children}</body>
    </html>
  )
}