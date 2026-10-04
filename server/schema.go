package main

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

var schema = `
CREATE TABLE IF NOT EXISTS subjects (
  id BIGSERIAL PRIMARY KEY,
  nama TEXT NOT NULL,
  kode TEXT UNIQUE
);

CREATE TABLE IF NOT EXISTS cps (
  id BIGSERIAL PRIMARY KEY,
  mapel_id BIGINT NOT NULL REFERENCES subjects(id) ON DELETE CASCADE,
  fase TEXT NOT NULL DEFAULT 'E',
  elemen TEXT NOT NULL DEFAULT '',
  deskripsi TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS users (
  id BIGSERIAL PRIMARY KEY,
  nama TEXT NOT NULL,
  username TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,
  role TEXT NOT NULL CHECK (role IN ('admin','guru')),
  mapel_id BIGINT REFERENCES subjects(id) ON DELETE SET NULL,
  aktif BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS students (
  id BIGSERIAL PRIMARY KEY,
  nis TEXT UNIQUE NOT NULL,
  nama TEXT NOT NULL,
  kelas TEXT NOT NULL,
  jk TEXT NOT NULL DEFAULT 'L' CHECK (jk IN ('L','P')),
  password_hash TEXT NOT NULL,
  aktif BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_students_kelas ON students(kelas);

-- Rombongan belajar (rombel). walas_id = guru yang menjadi wali kelas.
CREATE TABLE IF NOT EXISTS rombel (
  id BIGSERIAL PRIMARY KEY,
  nama TEXT UNIQUE NOT NULL,
  walas_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS materials (
  id BIGSERIAL PRIMARY KEY,
  mapel_id BIGINT NOT NULL REFERENCES subjects(id) ON DELETE CASCADE,
  cp_id BIGINT REFERENCES cps(id) ON DELETE SET NULL,
  guru_id BIGINT NOT NULL REFERENCES users(id),
  judul TEXT NOT NULL,
  isi TEXT NOT NULL DEFAULT '',
  file_url TEXT NOT NULL DEFAULT '',
  kelas TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS assignments (
  id BIGSERIAL PRIMARY KEY,
  mapel_id BIGINT NOT NULL REFERENCES subjects(id) ON DELETE CASCADE,
  cp_id BIGINT REFERENCES cps(id) ON DELETE SET NULL,
  guru_id BIGINT NOT NULL REFERENCES users(id),
  judul TEXT NOT NULL,
  deskripsi TEXT NOT NULL DEFAULT '',
  file_url TEXT NOT NULL DEFAULT '',
  kelas TEXT NOT NULL DEFAULT '',
  deadline TIMESTAMPTZ,
  max_score INT NOT NULL DEFAULT 100,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- UNIQUE(assignment_id, student_id) = idempotency alami (Aturan #2):
-- submit double-tap → upsert, tak pernah dobel.
CREATE TABLE IF NOT EXISTS submissions (
  id BIGSERIAL PRIMARY KEY,
  assignment_id BIGINT NOT NULL REFERENCES assignments(id) ON DELETE CASCADE,
  student_id BIGINT NOT NULL REFERENCES students(id) ON DELETE CASCADE,
  jawaban TEXT NOT NULL DEFAULT '',
  file_url TEXT NOT NULL DEFAULT '',
  nilai INT,
  feedback TEXT NOT NULL DEFAULT '',
  submitted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(assignment_id, student_id)
);

CREATE TABLE IF NOT EXISTS attendance (
  id BIGSERIAL PRIMARY KEY,
  student_id BIGINT NOT NULL REFERENCES students(id) ON DELETE CASCADE,
  mapel_id BIGINT NOT NULL DEFAULT 0,
  tanggal DATE NOT NULL,
  status TEXT NOT NULL DEFAULT 'hadir' CHECK (status IN ('hadir','izin','sakit','alpa')),
  keterangan TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_attendance_kelas_tanggal ON attendance(tanggal);

CREATE TABLE IF NOT EXISTS news (
  id BIGSERIAL PRIMARY KEY,
  judul TEXT NOT NULL,
  isi TEXT NOT NULL,
  author_id BIGINT NOT NULL REFERENCES users(id),
  pinned BOOLEAN NOT NULL DEFAULT FALSE,
  aktif BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS exams (
  id BIGSERIAL PRIMARY KEY,
  nama TEXT NOT NULL,
  mapel_id BIGINT NOT NULL REFERENCES subjects(id) ON DELETE CASCADE,
  cp_id BIGINT REFERENCES cps(id) ON DELETE SET NULL,
  guru_id BIGINT NOT NULL REFERENCES users(id),
  kelas TEXT NOT NULL DEFAULT '',
  durasi_menit INT NOT NULL DEFAULT 60,
  mulai_at TIMESTAMPTZ,
  selesai_at TIMESTAMPTZ,
  acak_soal BOOLEAN NOT NULL DEFAULT FALSE,
  aktif BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS questions (
  id BIGSERIAL PRIMARY KEY,
  exam_id BIGINT NOT NULL REFERENCES exams(id) ON DELETE CASCADE,
  soal TEXT NOT NULL,
  opsi JSONB NOT NULL DEFAULT '[]'::jsonb,
  kunci INT NOT NULL DEFAULT 0,
  bobot INT NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_questions_exam ON questions(exam_id);

CREATE TABLE IF NOT EXISTS attempts (
  id BIGSERIAL PRIMARY KEY,
  exam_id BIGINT NOT NULL REFERENCES exams(id) ON DELETE CASCADE,
  student_id BIGINT NOT NULL REFERENCES students(id) ON DELETE CASCADE,
  mulai_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  selesai_at TIMESTAMPTZ,
  skor INT,
  status TEXT NOT NULL DEFAULT 'berjalan' CHECK (status IN ('berjalan','selesai')),
  UNIQUE(exam_id, student_id)
);

-- UNIQUE(attempt_id, question_id) = sync jawaban berkala idempotent (Aturan #2).
CREATE TABLE IF NOT EXISTS answers (
  id BIGSERIAL PRIMARY KEY,
  attempt_id BIGINT NOT NULL REFERENCES attempts(id) ON DELETE CASCADE,
  question_id BIGINT NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
  jawaban INT NOT NULL DEFAULT -1,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(attempt_id, question_id)
);

CREATE TABLE IF NOT EXISTS cheat_signals (
  id BIGSERIAL PRIMARY KEY,
  student_id BIGINT NOT NULL REFERENCES students(id) ON DELETE CASCADE,
  exam_id BIGINT REFERENCES exams(id) ON DELETE SET NULL,
  jenis TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Kunci API AI pribadi guru (mis. Gemini) — tersimpan terenkripsi.
CREATE TABLE IF NOT EXISTS ai_keys (
  user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  provider TEXT NOT NULL DEFAULT 'gemini',
  key_enc TEXT NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
`

func Migrate(ctx context.Context, pool *pgxpool.Pool, log *zap.Logger) error {
	if _, err := pool.Exec(ctx, schema); err != nil {
		return err
	}
	// Migrasi idempotent utk DB lama: kolom mapel_id di attendance + unique index.
	// (CREATE TABLE IF NOT EXISTS tidak mengubah tabel yang sudah ada.)
	migrations := []string{
		`ALTER TABLE attendance ADD COLUMN IF NOT EXISTS mapel_id BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE attendance DROP CONSTRAINT IF EXISTS attendance_student_id_tanggal_key`,
		`CREATE UNIQUE INDEX IF NOT EXISTS attendance_student_mapel_tanggal
		   ON attendance(student_id, mapel_id, tanggal)`,
	}
	for _, m := range migrations {
		if _, err := pool.Exec(ctx, m); err != nil {
			return err
		}
	}
	// Rombel seed: jika belum ada, buat dari kelas yang sudah dipakai siswa.
	var nr int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM rombel`).Scan(&nr)
	if nr == 0 {
		if _, err := pool.Exec(ctx,
			`INSERT INTO rombel(nama)
			 SELECT DISTINCT kelas FROM students WHERE kelas<>'' AND aktif
			 ON CONFLICT(nama) DO NOTHING`); err != nil {
			return err
		}
	}
	log.Info("migrasi skema OK")
	return nil
}

func Seed(ctx context.Context, pool *pgxpool.Pool, cfg Config, log *zap.Logger) error {
	var n int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&n)
	if n > 0 {
		return nil
	}
	// Subject + CP contoh (Fase E SMK). Admin menambah yang riil lewat UI.
	subjects := []struct{ nama, kode string }{
		{"Matematika", "MTK"},
		{"Bahasa Indonesia", "BIN"},
		{"Bahasa Inggris", "BIG"},
		{"Dasar Program Keahlian", "DPK"},
	}
	mapelIDs := map[string]int64{}
	for _, s := range subjects {
		var id int64
		if err := pool.QueryRow(ctx,
			`INSERT INTO subjects(nama,kode) VALUES($1,$2) RETURNING id`, s.nama, s.kode).Scan(&id); err != nil {
			return err
		}
		mapelIDs[s.kode] = id
	}
	cps := []struct {
		kode, elemen, desc string
	}{
		{"MTK", "Bilangan", "Peserta didik dapat menggeneralisasi pola bilangan dan menerapkan barisan serta deret."},
		{"BIN", "Menyimak", "Peserta didik mampu memahami dan menganalisis informasi dari tuturan yang didengar."},
		{"BIG", "Reading", "Students can identify specific information and main ideas from various written texts."},
		{"DPK", "Pemrograman Dasar", "Peserta didik mampu menerapkan logika pemrograman dan algoritma dasar."},
	}
	for _, c := range cps {
		if _, err := pool.Exec(ctx,
			`INSERT INTO cps(mapel_id,fase,elemen,deskripsi) VALUES($1,'E',$2,$3)`,
			mapelIDs[c.kode], c.elemen, c.desc); err != nil {
			return err
		}
	}
	pw, err := HashPassword(cfg.BootstrapPw)
	if err != nil {
		return err
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO users(nama,username,password_hash,role) VALUES('Administrator','admin',$1,'admin')`, pw); err != nil {
		return err
	}
	// Guru contoh per mapel — admin mengganti dgn nama guru riil.
	gurus := []struct{ nama, uname, kode string }{
		{"Guru Matematika", "guru.mtk", "MTK"},
		{"Guru B. Indonesia", "guru.bin", "BIN"},
		{"Guru B. Inggris", "guru.big", "BIG"},
		{"Guru DPK", "guru.dpk", "DPK"},
	}
	for _, g := range gurus {
		if _, err := pool.Exec(ctx,
			`INSERT INTO users(nama,username,password_hash,role,mapel_id) VALUES($1,$2,$3,'guru',$4)`,
			g.nama, g.uname, pw, mapelIDs[g.kode]); err != nil {
			return err
		}
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO news(judul,isi,author_id,pinned)
		 SELECT 'Selamat datang di E-Learning','Platform e-learning resmi sekolah. Siswa dapat mengumpulkan tugas, presensi harian, membaca materi sesuai CP, dan mengikuti ujian online.',id,TRUE
		 FROM users WHERE username='admin'`); err != nil {
		return err
	}
	log.Info("seed awal OK (admin/admin123, 4 guru contoh, 4 mapel + CP)")
	return nil
}
