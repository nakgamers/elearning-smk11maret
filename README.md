# E-Learning SMK — Go + PostgreSQL + Redis + Next.js

Sistem e-learning sekolah skala besar (target 700 siswa simultan) dengan fitur
mirip CBT yang sudah ada: pengumpulan tugas, absen hadir, pengumuman, export
nilai, import siswa & soal/materi, dan seluruhnya memakai Capaian Pembelajaran
(CP). Nama guru tampil jelas pada setiap materi/tugas yang diunggah beserta
mata pelajarannya.

## Fitur

- **Login 3 peran**: admin, guru (username+password), siswa (NIS+password).
- **Materi & Tugas**: guru mengunggah materi/tugas (dengan lampiran) terkait
  mapel + CP; nama guru & mapel tercantum otomatis. Siswa mengumpulkan tugas
  (teks + file, idempotent → aman resubmit sebelum deadline).
- **Absensi**: siswa check-in harian + guru/admin rekap & ubah status per kelas.
- **Pengumuman (News)**: admin/guru menulis berita, siswa melihat di beranda.
- **Import siswa** (xlsx) & **import soal ujian** (xlsx) — 1 endpoint.
- **Ujian online**: paket diaktifkan → seluruh soal **pre-loaded ke Redis**
  (akses <1ms); jawaban disimpan lokal HP lalu **sync batch berkala** (idempotent);
  kalkulasi skor memakai **pessimistic lock**; jawaban dobel aman.
- **Kuis AI (sesi kelas/IFP)**: guru menempel materi → AI membuat 10 soal
  pilihan ganda (atau guru menempel JSON hasil chat dengan asisten AI) →
  review & edit → tampil satu-per-satu di layar dengan tombol lihat jawaban,
  navigasi keyboard (← → / spasi), mode layar penuh, dan **papan skor manual**
  (ketuk nama siswa yang menjawab benar, tersimpan di localStorage, bisa salin rekap).
- **Acak Nama (RPN)**: pilih rombel → acak nama siswa yang maju dengan animasi,
  mode tanpa pengulangan + daftar "sudah dipanggil" per sesi.
- **Export nilai** ke xlsx.
- **Anti-cheat Android**: wrapper WebView dengan FLAG_SECURE + deteksi onPause.

## Arsitektur

```
elearning/
├── server/     Go 1.26 + Fiber v3 + pgx(v5) + go-redis + zap + excelize  (:8081)
├── web/        Next.js 16 + Tailwind v4 (SPA, proxy /api → Go)            (:3000)
├── mobile/     Flutter WebView wrapper (FLAG_SECURE + onPause)   [kode inti]
├── Dockerfile + docker-entrypoint.sh + railway.json
└── server/smoke.sh   uji end-to-end
```

## Jalankan lokal

Prasyarat: Go 1.26, PostgreSQL (port 5433 utk dev), Node 22.

```bash
# 1. Postgres lokal (sudah disiapkan: db "elearning", port 5433)
#    (lihat ~/pgsql — portable PostgreSQL 17.5)

# 2. Backend
cd server
DATABASE_URL="postgres://postgres@127.0.0.1:5433/elearning?sslmode=disable" go run .
# → http://127.0.0.1:8081/api/health

# 3. Frontend (terminal terpisah)
cd web
npm install
npm run dev          # → http://localhost:3000  (proxy /api ke :8081)
```

Akun awal (seed otomatis): `admin/admin123`, guru contoh `guru.mtk/admin123`,
siswa contoh NIS `24001/siswa123` (password siswa default saat import = `siswa123`).

## Kuis AI — sumber AI (dua jalur)

Guru memilih salah satu saat membuat kuis:

1. **🏫 AI Sekolah** — via env server (konsep proxy ala 9Router: guru tidak
   perlu kunci sendiri):
   ```bash
   AI_BASE_URL="https://..."   # endpoint OpenAI-compatible (mis. 9Router)
   AI_API_KEY="..."
   AI_MODEL="..."
   AI_TIMEOUT_SEC="120"         # opsional, default 120
   ```
2. **🔑 Gemini pribadi guru** — guru menempel API key Gemini miliknya
   (gratis dari aistudio.google.com) sekali di UI; tersimpan **terenkripsi
   AES-GCM** per akun (`ai_keys`), dipakai via endpoint OpenAI-compatible
   Google (`.../v1beta/openai`, model default `gemini-2.0-flash`,
   bisa diubah via `AI_GEMINI_MODEL`).

Materi bisa ditempel sebagai teks atau **di-upload (PDF/PPTX/DOCX/TXT/MD)** —
teks diekstrak otomatis di server. Mode lanjutan "tempel JSON" tetap ada
untuk hasil chat dengan asisten AI (`POST /api/quiz/validate`).

## Smoke test backend

```bash
cd server
bash smoke.sh   # 13 langkah: auth, import, tugas, absen, news, ujian, skor, dll
```

## Deploy ke Railway

1. Buat project, sambungkan repo.
2. Tambah 3 service via Railway (sesuai master plan: **pisahkan resource agar tak
   rebut RAM**):
   - **PostgreSQL** → otomatis inject `DATABASE_URL`
   - **Redis** → inject `REDIS_URL`
   - **App (web service)** → build dari `Dockerfile` (berisi Go + Next.js)
3. Set env var app: `SECRET_KEY` (string acak panjang). `PORT` diisi Railway.
4. Deploy. Healthcheck `/api/health` diproxy Next → Go.

> Cache/queue Redis otomatis aktif begitu `REDIS_URL` tersedia; tanpa Redis
> (dev lokal) sistem fallback ke cache in-memory tanpa melanggar alur.

## Aturan master plan yang sudah dipenuhi

| # | Aturan | Implementasi |
|---|--------|--------------|
| 1 | Goroutine + defer close + pool 100 | `db.go` (MaxConns=100), `rows.Close()`/`defer` di semua query, log & cheat signal async |
| 2 | Idempotensi jawaban | `UNIQUE(assignment_id,student_id)` + `UNIQUE(attempt_id,question_id)` + upsert `ON CONFLICT` |
| 3 | Keamanan native Android | FLAG_SECURE + deteksi onPause di `mobile/` |
| 4 | Logging terstruktur | `go.uber.org/zap` |
| §2.1 | Pre-load soal ke Redis | `activateExam` → cache `exam:{id}:questions` |
| §2.2 | Batch/sync jawaban | sync berkala 15 dtk dari HP + UPSERT batch |
| §1 | Pessimistic lock | `finishAttempt` → `SELECT ... FOR UPDATE` pada baris attempt |