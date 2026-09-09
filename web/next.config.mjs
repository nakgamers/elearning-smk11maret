/** @type {import('next').NextConfig} */
const nextConfig = {
  async rewrites() {
    const backend = process.env.BACKEND_URL || 'http://127.0.0.1:8081'
    return [
      // Proxy API calls to Go backend
      { source: '/api/:path*', destination: `${backend}/api/:path*` },
      // Proxy uploaded files (materi/tugas/submission)
      { source: '/uploads/:path*', destination: `${backend}/uploads/:path*` },
    ]
  },
}

export default nextConfig