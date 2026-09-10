package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	recoverer "github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type Server struct {
	cfg   Config
	pool  *pgxpool.Pool
	cache Cache
	log   *zap.Logger
}

func NewApp(cfg Config, pool *pgxpool.Pool, cache Cache, log *zap.Logger) *fiber.App {
	s := &Server{cfg: cfg, pool: pool, cache: cache, log: log}
	os.MkdirAll(cfg.UploadDir, 0o755)

	app := fiber.New(fiber.Config{
		BodyLimit:    int(cfg.MaxUploadMB * 1024 * 1024),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
	})
	app.Use(recoverer.New())
	app.Use(cors.New(cors.Config{AllowOrigins: []string{"*"}, AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization"}}))

	// === Publik ===
	pub := app.Group("/api")
	pub.Get("/health", func(c fiber.Ctx) error { return c.JSON(fiber.Map{"ok": true}) })
	pub.Get("/news", func(c fiber.Ctx) error { return s.listNews(c, true) })
	pub.Post("/login", s.login)

	api := app.Group("/api", s.auth)

	// Role guard sebagai middleware inline (dua Group("") akan digabung Fiber).
	adminGuru := s.requireRole("admin", "guru")
	adminOnly := s.requireRole("admin")
	siswa := s.requireRole("siswa")

	// === Guru/Admin ===
	api.Get("/subjects", adminGuru, s.listSubjects)
	api.Post("/subjects", adminGuru, s.createSubject)
	api.Get("/cps", adminGuru, s.listCPs)
	api.Post("/cps", adminGuru, s.createCP)
	api.Put("/cps/:id", adminGuru, s.updateCP)
	api.Get("/teachers", adminGuru, s.listTeachers)
	api.Post("/teachers", adminOnly, s.createTeacher)
	// Siswa: lihat = admin+guru; tambah/edit/import/hapus = admin saja.
	api.Get("/students", adminGuru, s.listStudents)
	api.Post("/students", adminOnly, s.createStudent)
	api.Put("/students/:id", adminOnly, s.updateStudent)
	api.Post("/students/import", adminOnly, s.importStudents)
	api.Delete("/students/:id", adminOnly, s.deleteStudent)
	// Rombel & wali kelas.
	api.Get("/rombel", adminGuru, s.listRombel)
	api.Post("/rombel", adminOnly, s.createRombel)
	api.Put("/rombel/:id", adminOnly, s.updateRombel)
	api.Delete("/rombel/:id", adminOnly, s.deleteRombel)
	api.Get("/walas/attendance", adminGuru, s.walasAttendance)
	api.Get("/materials", adminGuru, s.listMaterials)
	api.Post("/materials", adminGuru, s.createMaterial)
	api.Get("/assignments", adminGuru, s.listAssignments)
	api.Post("/assignments", adminGuru, s.createAssignment)
	api.Get("/assignments/:id/submissions", adminGuru, s.listSubmissions)
	api.Post("/submissions/:id/grade", adminGuru, s.gradeSubmission)
	api.Get("/grades/export", adminGuru, s.exportGrades)
	api.Get("/attendance", adminGuru, s.listAttendance)
	api.Post("/attendance", adminGuru, s.setAttendance)
	api.Post("/news", adminGuru, s.createNews)
	api.Post("/news/:id/toggle", adminGuru, s.toggleNews)
	api.Get("/exams", adminGuru, s.listExams)
	api.Post("/exams", adminGuru, s.createExam)
	api.Post("/exams/:id/questions/import", adminGuru, s.importQuestions)
	api.Post("/exams/:id/activate", adminGuru, s.activateExam) // pre-load ke cache

	// === Siswa ===
	api.Get("/my/dashboard", siswa, s.studentDashboard)
	api.Get("/my/materials", siswa, s.studentMaterials)
	api.Get("/my/assignments", siswa, s.studentAssignments)
	api.Post("/submissions", siswa, s.submitAssignment)
	api.Post("/attendance/checkin", siswa, s.studentCheckin)
	api.Get("/attendance/my/today", siswa, s.studentAttendanceToday)
	api.Get("/my/exams", siswa, s.studentExams)
	api.Post("/exams/:id/start", siswa, s.startExam)
	api.Post("/attempts/:id/answers", siswa, s.syncAnswers) // batch, idempotent
	api.Post("/attempts/:id/finish", siswa, s.finishAttempt)
	api.Get("/my/attempts/:id/hasil", siswa, s.attemptResult)
	api.Post("/cheat-signals", siswa, s.logCheatSignal) // dari perangkat Android siswa

	// File hasil upload (materi/tugas) disajikan statis
	app.Get("/uploads/*", func(c fiber.Ctx) error {
		p := strings.TrimPrefix(c.Path(), "/uploads/")
		return c.SendFile(filepath.Join(s.cfg.UploadDir, filepath.Base(p)))
	})

	return app
}

// ---------- middleware ----------

func (s *Server) auth(c fiber.Ctx) error {
	h := c.Get("Authorization")
	tok := strings.TrimPrefix(h, "Bearer ")
	if tok == "" || tok == h {
		return fiber.ErrUnauthorized
	}
	cl, err := VerifyToken(s.cfg.Secret, tok)
	if err != nil {
		return fiber.ErrUnauthorized
	}
	c.Locals("claims", cl)
	return c.Next()
}

func (s *Server) requireRole(roles ...string) fiber.Handler {
	return func(c fiber.Ctx) error {
		cl, ok := c.Locals("claims").(*Claims)
		if !ok {
			return fiber.ErrUnauthorized
		}
		s.log.Debug("requireRole", zap.String("role", cl.Role), zap.Strings("want", roles), zap.String("path", c.Path()))
		for _, r := range roles {
			if cl.Role == r {
				return c.Next()
			}
		}
		return fiber.ErrForbidden
	}
}

func claimsOf(c fiber.Ctx) *Claims { return c.Locals("claims").(*Claims) }

// ---------- auth ----------

func (s *Server) login(c fiber.Ctx) error {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.Bind().Body(&body); err != nil {
		return fiber.ErrBadRequest
	}
	ctx := c.RequestCtx()
	s.log.Info("login attempt", zap.String("u", body.Username), zap.Int("pwlen", len(body.Password)))

	// Siswa: username = NIS
	var sid int64
	var snama, skelas, shash string
	err := s.pool.QueryRow(ctx,
		`SELECT id,nama,kelas,password_hash FROM students WHERE nis=$1 AND aktif`, body.Username).
		Scan(&sid, &snama, &skelas, &shash)
	if err == nil && CheckPassword(shash, body.Password) {
		tok := SignToken(s.cfg.Secret, sid, "siswa", 0, 12*time.Hour)
		return c.JSON(fiber.Map{"token": tok, "user": fiber.Map{
			"id": sid, "nama": snama, "role": "siswa", "kelas": skelas}})
	}

	// Guru/admin
	var uid, mapelID int64
	var nama, role string
	var hash string
	err = s.pool.QueryRow(ctx,
		`SELECT id,nama,role,password_hash,COALESCE(mapel_id,0) FROM users WHERE username=$1 AND aktif`, body.Username).
		Scan(&uid, &nama, &role, &hash, &mapelID)
	if err != nil || !CheckPassword(hash, body.Password) {
		return fiber.ErrUnauthorized
	}
	tok := SignToken(s.cfg.Secret, uid, role, mapelID, 12*time.Hour)
	resp := fiber.Map{"token": tok, "user": fiber.Map{
		"id": uid, "nama": nama, "role": role, "mapel_id": mapelID}}
	// Info wali kelas (rombel yang diampu) utk guru → tab "Walas".
	if role == "guru" {
		var wid int64
		var wnama string
		if e := s.pool.QueryRow(ctx, `SELECT id,nama FROM rombel WHERE walas_id=$1`, uid).Scan(&wid, &wnama); e == nil && wid > 0 {
			resp["walas"] = fiber.Map{"rombel_id": wid, "rombel": wnama}
		}
	}
	return c.JSON(resp)
}

// ---------- subjects / CP / teachers ----------

func (s *Server) listSubjects(c fiber.Ctx) error {
	rows, err := s.pool.Query(c.RequestCtx(), `SELECT id,nama,COALESCE(kode,'') FROM subjects ORDER BY nama`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []fiber.Map{}
	for rows.Next() {
		var id int64
		var nama, kode string
		rows.Scan(&id, &nama, &kode)
		out = append(out, fiber.Map{"id": id, "nama": nama, "kode": kode})
	}
	return c.JSON(out)
}

func (s *Server) createSubject(c fiber.Ctx) error {
	var b struct {
		Nama string `json:"nama"`
		Kode string `json:"kode"`
	}
	if err := c.Bind().Body(&b); err != nil || b.Nama == "" {
		return fiber.ErrBadRequest
	}
	var id int64
	if err := s.pool.QueryRow(c.RequestCtx(),
		`INSERT INTO subjects(nama,kode) VALUES($1,NULLIF($2,'')) RETURNING id`, b.Nama, b.Kode).Scan(&id); err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": id})
}

func (s *Server) listCPs(c fiber.Ctx) error {
	rows, err := s.pool.Query(c.RequestCtx(),
		`SELECT cp.id,cp.mapel_id,cp.fase,cp.elemen,cp.deskripsi,s.nama
		 FROM cps cp JOIN subjects s ON s.id=cp.mapel_id ORDER BY s.nama,cp.elemen`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []fiber.Map{}
	for rows.Next() {
		var id, mid int64
		var fase, el, desc, mapel string
		rows.Scan(&id, &mid, &fase, &el, &desc, &mapel)
		out = append(out, fiber.Map{"id": id, "mapel_id": mid, "fase": fase,
			"elemen": el, "deskripsi": desc, "mapel": mapel})
	}
	return c.JSON(out)
}

func (s *Server) createCP(c fiber.Ctx) error {
	var b struct {
		MapelID   int64  `json:"mapel_id"`
		Fase      string `json:"fase"`
		Elemen    string `json:"elemen"`
		Deskripsi string `json:"deskripsi"`
	}
	if err := c.Bind().Body(&b); err != nil || b.MapelID == 0 || b.Deskripsi == "" {
		return fiber.ErrBadRequest
	}
	var id int64
	err := s.pool.QueryRow(c.RequestCtx(),
		`INSERT INTO cps(mapel_id,fase,elemen,deskripsi) VALUES($1,$2,$3,$4) RETURNING id`,
		b.MapelID, def(b.Fase, "E"), b.Elemen, b.Deskripsi).Scan(&id)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": id})
}

// Edit CP yang sudah ada (admin & guru).
func (s *Server) updateCP(c fiber.Ctx) error {
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	var b struct {
		MapelID   int64  `json:"mapel_id"`
		Fase      string `json:"fase"`
		Elemen    string `json:"elemen"`
		Deskripsi string `json:"deskripsi"`
	}
	if err := c.Bind().Body(&b); err != nil {
		return fiber.ErrBadRequest
	}
	_, err := s.pool.Exec(c.RequestCtx(),
		`UPDATE cps SET mapel_id=COALESCE(NULLIF($1,0),mapel_id), fase=COALESCE(NULLIF($2,''),fase),
		 elemen=COALESCE(NULLIF($3,''),elemen), deskripsi=COALESCE(NULLIF($4,''),deskripsi) WHERE id=$5`,
		b.MapelID, b.Fase, b.Elemen, b.Deskripsi, id)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (s *Server) listTeachers(c fiber.Ctx) error {
	rows, err := s.pool.Query(c.RequestCtx(),
		`SELECT u.id,u.nama,u.username,u.role,COALESCE(s.nama,'') FROM users u
		 LEFT JOIN subjects s ON s.id=u.mapel_id WHERE u.role='guru' AND u.aktif ORDER BY u.nama`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []fiber.Map{}
	for rows.Next() {
		var id int64
		var nama, uname, role, mapel string
		rows.Scan(&id, &nama, &uname, &role, &mapel)
		out = append(out, fiber.Map{"id": id, "nama": nama, "username": uname, "mapel": mapel})
	}
	return c.JSON(out)
}

func (s *Server) createTeacher(c fiber.Ctx) error {
	var b struct {
		Nama     string `json:"nama"`
		Username string `json:"username"`
		Password string `json:"password"`
		MapelID  int64  `json:"mapel_id"`
	}
	if err := c.Bind().Body(&b); err != nil || b.Nama == "" || b.Username == "" || len(b.Password) < 6 {
		return fiber.ErrBadRequest
	}
	h, err := HashPassword(b.Password)
	if err != nil {
		return err
	}
	var id int64
	err = s.pool.QueryRow(c.RequestCtx(),
		`INSERT INTO users(nama,username,password_hash,role,mapel_id) VALUES($1,$2,$3,'guru',NULLIF($4,0)) RETURNING id`,
		b.Nama, b.Username, h, b.MapelID).Scan(&id)
	if err != nil {
		return fmt.Errorf("username mungkin sudah dipakai: %w", err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": id})
}

// ---------- students ----------

func (s *Server) listStudents(c fiber.Ctx) error {
	onlyActive := c.Query("all") != "1"
	q := `SELECT id,nis,nama,kelas,jk,aktif FROM students`
	args := []any{}
	conds := []string{}
	if onlyActive {
		conds = append(conds, "aktif")
	}
	if k := c.Query("kelas"); k != "" {
		args = append(args, k)
		conds = append(conds, fmt.Sprintf("kelas=$%d", len(args)))
	}
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	q += " ORDER BY kelas,nis"
	rows, err := s.pool.Query(c.RequestCtx(), q, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []fiber.Map{}
	for rows.Next() {
		var id int64
		var nis, nama, kelas, jk string
		var aktif bool
		rows.Scan(&id, &nis, &nama, &kelas, &jk, &aktif)
		out = append(out, fiber.Map{"id": id, "nis": nis, "nama": nama, "kelas": kelas, "jk": jk, "aktif": aktif})
	}
	return c.JSON(out)
}

// Tambah satu siswa (admin only).
func (s *Server) createStudent(c fiber.Ctx) error {
	var b struct {
		NIS      string `json:"nis"`
		Nama     string `json:"nama"`
		Kelas    string `json:"kelas"`
		JK       string `json:"jk"`
		Password string `json:"password"`
	}
	if err := c.Bind().Body(&b); err != nil || b.NIS == "" || b.Nama == "" || b.Kelas == "" {
		return fiber.ErrBadRequest
	}
	pw := def(b.Password, "siswa123")
	h, err := HashPassword(pw)
	if err != nil {
		return err
	}
	var id int64
	err = s.pool.QueryRow(c.RequestCtx(),
		`INSERT INTO students(nis,nama,kelas,jk,password_hash) VALUES($1,$2,$3,$4,$5) RETURNING id`,
		b.NIS, b.Nama, b.Kelas, defJk(b.JK), h).Scan(&id)
	if err != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "NIS sudah terdaftar"})
	}
	// pastikan rombel tersedia
	_, _ = s.pool.Exec(c.RequestCtx(), `INSERT INTO rombel(nama) VALUES($1) ON CONFLICT(nama) DO NOTHING`, b.Kelas)
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": id})
}

// Edit siswa (admin only): nama, kelas, JK, password opsional, aktif.
func (s *Server) updateStudent(c fiber.Ctx) error {
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	var b struct {
		NIS      string `json:"nis"`
		Nama     string `json:"nama"`
		Kelas    string `json:"kelas"`
		JK       string `json:"jk"`
		Password string `json:"password"`
		Aktif    *bool  `json:"aktif"`
	}
	if err := c.Bind().Body(&b); err != nil {
		return fiber.ErrBadRequest
	}
	ctx := c.RequestCtx()
	if b.Nama != "" || b.NIS != "" || b.Kelas != "" || b.JK != "" {
		if _, err := s.pool.Exec(ctx,
			`UPDATE students SET nis=COALESCE(NULLIF($1,''),nis), nama=COALESCE(NULLIF($2,''),nama),
			 kelas=COALESCE(NULLIF($3,''),kelas), jk=COALESCE(NULLIF($4,''),jk) WHERE id=$5`,
			b.NIS, b.Nama, b.Kelas, b.JK, id); err != nil {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "NIS sudah dipakai"})
		}
		if b.Kelas != "" {
			_, _ = s.pool.Exec(ctx, `INSERT INTO rombel(nama) VALUES($1) ON CONFLICT(nama) DO NOTHING`, b.Kelas)
		}
	}
	if b.Password != "" && len(b.Password) >= 6 {
		if h, err := HashPassword(b.Password); err == nil {
			_, _ = s.pool.Exec(ctx, `UPDATE students SET password_hash=$1 WHERE id=$2`, h, id)
		}
	}
	if b.Aktif != nil {
		_, _ = s.pool.Exec(ctx, `UPDATE students SET aktif=$1 WHERE id=$2`, *b.Aktif, id)
	}
	return c.JSON(fiber.Map{"ok": true})
}
func (s *Server) importStudents(c fiber.Ctx) error {
	fh, err := c.FormFile("file")
	if err != nil {
		return fiber.ErrBadRequest
	}
	f, err := fh.Open()
	if err != nil {
		return err
	}
	defer f.Close()

	xl, err := excelizeRead(f)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	pw, _ := HashPassword("siswa123") // password default, siswa ganti lewat admin
	imported, skipped := 0, 0
	ctx := c.RequestCtx()
	for _, r := range xl {
		if len(r) < 3 || r[0] == "" || r[1] == "" {
			continue
		}
		if _, err := s.pool.Exec(ctx,
			`INSERT INTO students(nis,nama,kelas,jk,password_hash) VALUES($1,$2,$3,$4,$5)
			 ON CONFLICT(nis) DO UPDATE SET nama=EXCLUDED.nama,kelas=EXCLUDED.kelas,jk=EXCLUDED.jk,aktif=TRUE`,
			r[0], r[1], r[2], defJk(col(r, 3)), pw); err != nil {
			skipped++
			continue
		}
		imported++
	}
	// Sinkronkan rombel dari kelas yang diimpor.
	if _, err := s.pool.Exec(ctx,
		`INSERT INTO rombel(nama) SELECT DISTINCT kelas FROM students WHERE kelas<>'' AND aktif ON CONFLICT(nama) DO NOTHING`); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"imported": imported, "skipped": skipped})
}

func (s *Server) deleteStudent(c fiber.Ctx) error {
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	_, err := s.pool.Exec(c.RequestCtx(), `UPDATE students SET aktif=FALSE WHERE id=$1`, id)
	return errOr(c, err, fiber.Map{"ok": true})
}

// ---------- rombel & wali kelas ----------

func (s *Server) listRombel(c fiber.Ctx) error {
	rows, err := s.pool.Query(c.RequestCtx(),
		`SELECT r.id,r.nama,COALESCE(u.nama,''),COALESCE(u.id,0),
		        (SELECT count(*) FROM students st WHERE st.kelas=r.nama AND st.aktif)
		 FROM rombel r LEFT JOIN users u ON u.id=r.walas_id
		 ORDER BY r.nama`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []fiber.Map{}
	for rows.Next() {
		var id, walasID, nSiswa int64
		var nama, walasNama string
		rows.Scan(&id, &nama, &walasNama, &walasID, &nSiswa)
		out = append(out, fiber.Map{"id": id, "nama": nama, "walas_id": walasID,
			"walas": walasNama, "jumlah_siswa": nSiswa})
	}
	return c.JSON(out)
}

func (s *Server) createRombel(c fiber.Ctx) error {
	var b struct {
		Nama    string `json:"nama"`
		WalasID int64  `json:"walas_id"`
	}
	if err := c.Bind().Body(&b); err != nil || b.Nama == "" {
		return fiber.ErrBadRequest
	}
	var id int64
	err := s.pool.QueryRow(c.RequestCtx(),
		`INSERT INTO rombel(nama,walas_id) VALUES($1,NULLIF($2,0)) ON CONFLICT(nama) DO NOTHING RETURNING id`,
		b.Nama, b.WalasID).Scan(&id)
	if err != nil {
		// sudah ada → ambil id & tetap set walas kalau diminta
		s.pool.QueryRow(c.RequestCtx(), `SELECT id FROM rombel WHERE nama=$1`, b.Nama).Scan(&id)
		if b.WalasID > 0 {
			s.pool.Exec(c.RequestCtx(), `UPDATE rombel SET walas_id=$1 WHERE id=$2`, b.WalasID, id)
		}
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": id})
}

func (s *Server) updateRombel(c fiber.Ctx) error {
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	var b struct {
		Nama    string `json:"nama"`
		WalasID *int64 `json:"walas_id"` // pointer: boleh clear ke 0
	}
	if err := c.Bind().Body(&b); err != nil {
		return fiber.ErrBadRequest
	}
	ctx := c.RequestCtx()
	if b.Nama != "" {
		var old string
		if err := s.pool.QueryRow(ctx, `SELECT nama FROM rombel WHERE id=$1`, id).Scan(&old); err != nil {
			return fiber.ErrNotFound
		}
		if _, err := s.pool.Exec(ctx, `UPDATE rombel SET nama=$1 WHERE id=$2`, b.Nama, id); err != nil {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Nama rombel sudah ada"})
		}
		// sinkronkan kelas siswa yang mengikuti rombel ini
		_, _ = s.pool.Exec(ctx, `UPDATE students SET kelas=$1 WHERE kelas=$2`, b.Nama, old)
	}
	if b.WalasID != nil {
		if _, err := s.pool.Exec(ctx,
			`UPDATE rombel SET walas_id=CASE WHEN $1=0 THEN NULL ELSE $1 END WHERE id=$2`,
			*b.WalasID, id); err != nil {
			return err
		}
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (s *Server) deleteRombel(c fiber.Ctx) error {
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	_, err := s.pool.Exec(c.RequestCtx(), `DELETE FROM rombel WHERE id=$1`, id)
	return errOr(c, err, fiber.Map{"ok": true})
}

// Kehadiran siswa di rombel wali kelas, per mata pelajaran (diampu guru lain).
func (s *Server) walasAttendance(c fiber.Ctx) error {
	cl := claimsOf(c)
	ctx := c.RequestCtx()
	// admin boleh lihat rombel mana pun via ?rombel_id; guru wajib walas di rombel tsb.
	rid, _ := strconv.ParseInt(c.Query("rombel_id"), 10, 64)
	if rid == 0 {
		s.pool.QueryRow(ctx, `SELECT id FROM rombel WHERE walas_id=$1`, cl.UID).Scan(&rid)
	}
	if rid == 0 {
		return fiber.ErrNotFound
	}
	if cl.Role != "admin" {
		var ok bool
		s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM rombel WHERE id=$1 AND walas_id=$2)`, rid, cl.UID).Scan(&ok)
		if !ok {
			return fiber.ErrForbidden
		}
	}
	var namaRombel string
	s.pool.QueryRow(ctx, `SELECT nama FROM rombel WHERE id=$1`, rid).Scan(&namaRombel)
	tanggal := c.Query("tanggal")
	if tanggal == "" {
		tanggal = time.Now().Format("2006-01-02")
	}
	rows, err := s.pool.Query(ctx,
		`SELECT st.id,st.nis,st.nama,st.jk,a.mapel_id,COALESCE(sb.nama,''),a.status
		 FROM students st
		 LEFT JOIN attendance a ON a.student_id=st.id AND a.tanggal=$2::date
		 LEFT JOIN subjects sb ON sb.id=a.mapel_id
		 WHERE st.kelas=$1 AND st.aktif
		 ORDER BY st.nis`, namaRombel, tanggal)
	if err != nil {
		return err
	}
	defer rows.Close()
	// hasil: daftar mapel (kolom) + per siswa, status per mapel.
	mapelCols := []fiber.Map{}
	mapelIdx := map[int64]int{}
	siswa := []fiber.Map{}
	siswaById := map[int64]int{}
	for rows.Next() {
		var sid, mid int64
		var nis, nama, jk, mapel, status string
		rows.Scan(&sid, &nis, &nama, &jk, &mid, &mapel, &status)
		i, ok := siswaById[sid]
		if !ok {
			siswaById[sid] = len(siswa)
			siswa = append(siswa, fiber.Map{"id": sid, "nis": nis, "nama": nama, "jk": jk, "status": fiber.Map{}})
			i = len(siswa) - 1
		}
		if mid > 0 {
			ci, ok := mapelIdx[mid]
			if !ok {
				mapelIdx[mid] = len(mapelCols)
				mapelCols = append(mapelCols, fiber.Map{"id": mid, "nama": mapel})
				ci = len(mapelCols) - 1
			}
			_ = ci
			st := siswa[i]["status"].(fiber.Map)
			st[mapel] = status
		}
	}
	return c.JSON(fiber.Map{
		"rombel": namaRombel, "tanggal": tanggal,
		"mapel": mapelCols, "siswa": siswa,
	})
}

// ---------- materials ----------

func (s *Server) listMaterials(c fiber.Ctx) error {
	rows, err := s.pool.Query(c.RequestCtx(),
		`SELECT m.id,m.judul,m.isi,m.file_url,m.kelas,m.created_at,u.nama,s.nama,COALESCE(cp.elemen,''),COALESCE(cp.deskripsi,'')
		 FROM materials m JOIN users u ON u.id=m.guru_id JOIN subjects s ON s.id=m.mapel_id
		 LEFT JOIN cps cp ON cp.id=m.cp_id ORDER BY m.created_at DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []fiber.Map{}
	for rows.Next() {
		var id int64
		var judul, isi, furl, kelas, guru, mapel, elemen, cpdesc string
		var createdAt time.Time
		rows.Scan(&id, &judul, &isi, &furl, &kelas, &createdAt, &guru, &mapel, &elemen, &cpdesc)
		out = append(out, fiber.Map{"id": id, "judul": judul, "isi": isi, "file_url": furl,
			"kelas": kelas, "created_at": createdAt, "guru": guru, "mapel": mapel,
			"cp_elemen": elemen, "cp_deskripsi": cpdesc})
	}
	return c.JSON(out)
}

func (s *Server) createMaterial(c fiber.Ctx) error {
	cl := claimsOf(c)
	mapelID, _ := strconv.ParseInt(c.FormValue("mapel_id"), 10, 64)
	cpID, _ := strconv.ParseInt(c.FormValue("cp_id"), 10, 64)
	judul := c.FormValue("judul")
	if mapelID == 0 || judul == "" {
		return fiber.ErrBadRequest
	}
	furl, err := s.saveUpload(c, "file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	var id int64
	err = s.pool.QueryRow(c.RequestCtx(),
		`INSERT INTO materials(mapel_id,cp_id,guru_id,judul,isi,file_url,kelas) VALUES($1,NULLIF($2,0),$3,$4,$5,$6,$7) RETURNING id`,
		mapelID, cpID, cl.UID, judul, c.FormValue("isi"), furl, c.FormValue("kelas")).Scan(&id)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": id})
}

// ---------- assignments ----------

func (s *Server) listAssignments(c fiber.Ctx) error {
	rows, err := s.pool.Query(c.RequestCtx(),
		`SELECT a.id,a.judul,a.deskripsi,a.file_url,a.kelas,a.deadline,a.max_score,a.created_at,u.nama,s.nama,COALESCE(cp.elemen,''),COALESCE(cp.deskripsi,'')
		 FROM assignments a JOIN users u ON u.id=a.guru_id JOIN subjects s ON s.id=a.mapel_id
		 LEFT JOIN cps cp ON cp.id=a.cp_id ORDER BY a.created_at DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []fiber.Map{}
	for rows.Next() {
		var id int64
		var judul, desc, furl, kelas, guru, mapel, elemen, cpdesc string
		var deadline *time.Time
		var maxScore int
		var createdAt time.Time
		rows.Scan(&id, &judul, &desc, &furl, &kelas, &deadline, &maxScore, &createdAt, &guru, &mapel, &elemen, &cpdesc)
		out = append(out, fiber.Map{"id": id, "judul": judul, "deskripsi": desc, "file_url": furl,
			"kelas": kelas, "deadline": deadline, "max_score": maxScore, "created_at": createdAt,
			"guru": guru, "mapel": mapel, "cp_elemen": elemen, "cp_deskripsi": cpdesc})
	}
	return c.JSON(out)
}

func (s *Server) createAssignment(c fiber.Ctx) error {
	cl := claimsOf(c)
	mapelID, _ := strconv.ParseInt(c.FormValue("mapel_id"), 10, 64)
	cpID, _ := strconv.ParseInt(c.FormValue("cp_id"), 10, 64)
	judul := c.FormValue("judul")
	if mapelID == 0 || judul == "" {
		return fiber.ErrBadRequest
	}
	furl, err := s.saveUpload(c, "file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	var dl *time.Time
	if d := c.FormValue("deadline"); d != "" {
		if t, err := time.Parse("2006-01-02T15:04", d); err == nil {
			dl = &t
		}
	}
	maxScore, _ := strconv.Atoi(c.FormValue("max_score"))
	if maxScore <= 0 {
		maxScore = 100
	}
	var id int64
	err = s.pool.QueryRow(c.RequestCtx(),
		`INSERT INTO assignments(mapel_id,cp_id,guru_id,judul,deskripsi,file_url,kelas,deadline,max_score)
		 VALUES($1,NULLIF($2,0),$3,$4,$5,$6,$7,$8,$9) RETURNING id`,
		mapelID, cpID, cl.UID, judul, c.FormValue("deskripsi"), furl, c.FormValue("kelas"), dl, maxScore).Scan(&id)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": id})
}

func (s *Server) listSubmissions(c fiber.Ctx) error {
	aid, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	rows, err := s.pool.Query(c.RequestCtx(),
		`SELECT sub.id,st.nis,st.nama,sub.jawaban,sub.file_url,sub.nilai,sub.feedback,sub.submitted_at
		 FROM submissions sub JOIN students st ON st.id=sub.student_id
		 WHERE sub.assignment_id=$1 ORDER BY st.nis`, aid)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []fiber.Map{}
	for rows.Next() {
		var id int64
		var nis, nama, jawaban, furl, feedback string
		var nilai *int
		var subAt time.Time
		rows.Scan(&id, &nis, &nama, &jawaban, &furl, &nilai, &feedback, &subAt)
		out = append(out, fiber.Map{"id": id, "nis": nis, "nama": nama, "jawaban": jawaban,
			"file_url": furl, "nilai": nilai, "feedback": feedback, "submitted_at": subAt})
	}
	return c.JSON(out)
}

func (s *Server) gradeSubmission(c fiber.Ctx) error {
	sid, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	var b struct {
		Nilai    int    `json:"nilai"`
		Feedback string `json:"feedback"`
	}
	if err := c.Bind().Body(&b); err != nil {
		return fiber.ErrBadRequest
	}
	_, err := s.pool.Exec(c.RequestCtx(),
		`UPDATE submissions SET nilai=$1,feedback=$2,updated_at=now() WHERE id=$3`, b.Nilai, b.Feedback, sid)
	return errOr(c, err, fiber.Map{"ok": true})
}

// ---------- export nilai (xlsx) ----------

func (s *Server) exportGrades(c fiber.Ctx) error {
	kelas := c.Query("kelas")
	q := `SELECT st.nis,st.nama,st.kelas,s.nama,COALESCE(a.judul,''),sub.nilai,sub.submitted_at
	      FROM submissions sub
	      JOIN students st ON st.id=sub.student_id
	      JOIN assignments a ON a.id=sub.assignment_id
	      JOIN subjects s ON s.id=a.mapel_id
	      WHERE sub.nilai IS NOT NULL`
	args := []any{}
	if kelas != "" {
		args = append(args, kelas)
		q += fmt.Sprintf(" AND st.kelas=$%d", len(args))
	}
	q += " ORDER BY st.kelas,st.nis,s.nama"
	rows, err := s.pool.Query(c.RequestCtx(), q, args...)
	if err != nil {
		return err
	}
	var data []GradeRow
	for rows.Next() {
		var r GradeRow
		rows.Scan(&r.NIS, &r.Nama, &r.Kelas, &r.Mapel, &r.Judul, &r.Nilai, &r.T)
		data = append(data, r)
	}
	rows.Close()

	buf, err := gradesXLSX(data)
	if err != nil {
		return err
	}
	name := fmt.Sprintf("nilai-%s.xlsx", time.Now().Format("20060102-1504"))
	c.Set("Content-Disposition", `attachment; filename="`+name+`"`)
	c.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Send(buf)
	return nil
}

// ---------- attendance ----------

func (s *Server) listAttendance(c fiber.Ctx) error {
	q := `SELECT a.id,st.nis,st.nama,st.kelas,a.tanggal,a.status,a.keterangan,a.mapel_id,COALESCE(sb.nama,'')
	      FROM attendance a JOIN students st ON st.id=a.student_id
	      LEFT JOIN subjects sb ON sb.id=a.mapel_id WHERE TRUE`
	args := []any{}
	if k := c.Query("kelas"); k != "" {
		args = append(args, k)
		q += fmt.Sprintf(" AND st.kelas=$%d", len(args))
	}
	if t := c.Query("tanggal"); t != "" {
		args = append(args, t)
		q += fmt.Sprintf(" AND a.tanggal=$%d::date", len(args))
	}
	if m := c.Query("mapel_id"); m != "" {
		args = append(args, m)
		q += fmt.Sprintf(" AND a.mapel_id=$%d", len(args))
	}
	q += " ORDER BY a.tanggal DESC,st.nis LIMIT 500"
	rows, err := s.pool.Query(c.RequestCtx(), q, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []fiber.Map{}
	for rows.Next() {
		var id, mid int64
		var nis, nama, kelas, status, ket, mapel string
		var tgl time.Time
		rows.Scan(&id, &nis, &nama, &kelas, &tgl, &status, &ket, &mid, &mapel)
		out = append(out, fiber.Map{"id": id, "nis": nis, "nama": nama, "kelas": kelas,
			"tanggal": tgl.Format("2006-01-02"), "status": status, "keterangan": ket,
			"mapel_id": mid, "mapel": mapel})
	}
	return c.JSON(out)
}

// Set absen manual oleh guru/admin (bulk per siswa, per mapel).
func (s *Server) setAttendance(c fiber.Ctx) error {
	var b struct {
		StudentID  int64  `json:"student_id"`
		Tanggal    string `json:"tanggal"`
		Status     string `json:"status"`
		Keterangan string `json:"keterangan"`
		MapelID    int64  `json:"mapel_id"`
	}
	if err := c.Bind().Body(&b); err != nil || b.StudentID == 0 || b.Tanggal == "" {
		return fiber.ErrBadRequest
	}
	_, err := s.pool.Exec(c.RequestCtx(),
		`INSERT INTO attendance(student_id,mapel_id,tanggal,status,keterangan) VALUES($1,$2,$3::date,$4,$5)
		 ON CONFLICT(student_id,mapel_id,tanggal) DO UPDATE SET status=EXCLUDED.status,keterangan=EXCLUDED.keterangan`,
		b.StudentID, b.MapelID, b.Tanggal, def(b.Status, "hadir"), b.Keterangan)
	return errOr(c, err, fiber.Map{"ok": true})
}

func (s *Server) studentCheckin(c fiber.Ctx) error {
	cl := claimsOf(c)
	var b struct {
		Keterangan string `json:"keterangan"`
	}
	c.Bind().Body(&b)
	today := time.Now().Format("2006-01-02")
	// Idempotent: baris (student,mapel,tanggal) unik; klik ganda tidak akan menggandakan absen.
	_, err := s.pool.Exec(c.RequestCtx(),
		`INSERT INTO attendance(student_id,mapel_id,tanggal,status,keterangan) VALUES($1,0,$2::date,'hadir',$3)
		 ON CONFLICT(student_id,mapel_id,tanggal) DO NOTHING`, cl.UID, today, b.Keterangan)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"ok": true, "tanggal": today})
}

// Status kehadiran siswa HARI INI → dipakai UI utk men-disable tombol "Hadir sekarang".
func (s *Server) studentAttendanceToday(c fiber.Ctx) error {
	cl := claimsOf(c)
	today := time.Now().Format("2006-01-02")
	var attended bool
	err := s.pool.QueryRow(c.RequestCtx(),
		`SELECT EXISTS(SELECT 1 FROM attendance WHERE student_id=$1 AND tanggal=$2::date)`,
		cl.UID, today).Scan(&attended)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"attended": attended, "tanggal": today})
}

// ---------- news ----------

func (s *Server) listNews(c fiber.Ctx, onlyActive bool) error {
	q := `SELECT n.id,n.judul,n.isi,n.pinned,n.created_at,u.nama
	      FROM news n JOIN users u ON u.id=n.author_id`
	if onlyActive {
		q += ` WHERE n.aktif`
	}
	q += ` ORDER BY n.pinned DESC,n.created_at DESC LIMIT 50`
	rows, err := s.pool.Query(c.RequestCtx(), q)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []fiber.Map{}
	for rows.Next() {
		var id int64
		var judul, isi, author string
		var pinned bool
		var createdAt time.Time
		rows.Scan(&id, &judul, &isi, &pinned, &createdAt, &author)
		out = append(out, fiber.Map{"id": id, "judul": judul, "isi": isi, "pinned": pinned,
			"created_at": createdAt, "author": author})
	}
	return c.JSON(out)
}

func (s *Server) createNews(c fiber.Ctx) error {
	cl := claimsOf(c)
	var b struct {
		Judul string `json:"judul"`
		Isi   string `json:"isi"`
	}
	if err := c.Bind().Body(&b); err != nil || b.Judul == "" || b.Isi == "" {
		return fiber.ErrBadRequest
	}
	var id int64
	err := s.pool.QueryRow(c.RequestCtx(),
		`INSERT INTO news(judul,isi,author_id) VALUES($1,$2,$3) RETURNING id`, b.Judul, b.Isi, cl.UID).Scan(&id)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": id})
}

func (s *Server) toggleNews(c fiber.Ctx) error {
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	_, err := s.pool.Exec(c.RequestCtx(), `UPDATE news SET aktif=NOT aktif WHERE id=$1`, id)
	return errOr(c, err, fiber.Map{"ok": true})
}

// ---------- exams ----------

func (s *Server) listExams(c fiber.Ctx) error {
	rows, err := s.pool.Query(c.RequestCtx(),
		`SELECT e.id,e.nama,e.kelas,e.durasi_menit,e.mulai_at,e.selesai_at,e.aktif,e.acak_soal,u.nama,s.nama,
		        (SELECT count(*) FROM questions q WHERE q.exam_id=e.id)
		 FROM exams e JOIN users u ON u.id=e.guru_id JOIN subjects s ON s.id=e.mapel_id ORDER BY e.created_at DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []fiber.Map{}
	for rows.Next() {
		var id int64
		var nama, kelas, guru, mapel string
		var dur int
		var mulai, selesai *time.Time
		var aktif, acak bool
		var nq int
		rows.Scan(&id, &nama, &kelas, &dur, &mulai, &selesai, &aktif, &acak, &guru, &mapel, &nq)
		out = append(out, fiber.Map{"id": id, "nama": nama, "kelas": kelas, "durasi_menit": dur,
			"mulai_at": mulai, "selesai_at": selesai, "aktif": aktif, "acak_soal": acak,
			"guru": guru, "mapel": mapel, "jumlah_soal": nq})
	}
	return c.JSON(out)
}

func (s *Server) createExam(c fiber.Ctx) error {
	cl := claimsOf(c)
	var b struct {
		Nama    string `json:"nama"`
		MapelID int64  `json:"mapel_id"`
		CpID    int64  `json:"cp_id"`
		Kelas   string `json:"kelas"`
		Durasi  int    `json:"durasi_menit"`
		Mulai   string `json:"mulai_at"`
		Selesai string `json:"selesai_at"`
		Acak    bool   `json:"acak_soal"`
	}
	if err := c.Bind().Body(&b); err != nil || b.Nama == "" || b.MapelID == 0 || b.Durasi <= 0 {
		return fiber.ErrBadRequest
	}
	var mulai, selesai *time.Time
	if t, err := time.Parse(time.RFC3339, b.Mulai); err == nil {
		mulai = &t
	}
	if t, err := time.Parse(time.RFC3339, b.Selesai); err == nil {
		selesai = &t
	}
	var id int64
	err := s.pool.QueryRow(c.RequestCtx(),
		`INSERT INTO exams(nama,mapel_id,cp_id,guru_id,kelas,durasi_menit,mulai_at,selesai_at,acak_soal)
		 VALUES($1,$2,NULLIF($3,0),$4,$5,$6,$7,$8,$9) RETURNING id`,
		b.Nama, b.MapelID, b.CpID, cl.UID, b.Kelas, b.Durasi, mulai, selesai, b.Acak).Scan(&id)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": id})
}

// Import soal xlsx: Soal | OpsiA | OpsiB | OpsiC | OpsiD | OpsiE | Kunci(1-5) | Bobot
func (s *Server) importQuestions(c fiber.Ctx) error {
	eid, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	fh, err := c.FormFile("file")
	if err != nil {
		return fiber.ErrBadRequest
	}
	f, err := fh.Open()
	if err != nil {
		return err
	}
	defer f.Close()
	xl, err := excelizeRead(f)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	ctx := c.RequestCtx()
	imported := 0
	for _, r := range xl {
		if len(r) < 2 || r[0] == "" {
			continue
		}
		opsi := []string{}
		for i := 1; i <= 5 && i < len(r); i++ {
			if r[i] != "" {
				opsi = append(opsi, r[i])
			}
		}
		kunci, _ := strconv.Atoi(col(r, 6))
		bobot, _ := strconv.Atoi(col(r, 7))
		if bobot <= 0 {
			bobot = 1
		}
		if kunci < 1 || kunci > len(opsi) {
			kunci = 1
		}
		oj, _ := json.Marshal(opsi)
		if _, err := s.pool.Exec(ctx,
			`INSERT INTO questions(exam_id,soal,opsi,kunci,bobot) VALUES($1,$2,$3::jsonb,$4,$5)`,
			eid, r[0], string(oj), kunci-1, bobot); err != nil {
			continue
		}
		imported++
	}
	s.cache.Del(ctx, examKey(eid)) // soal berubah → invalidasi cache
	return c.JSON(fiber.Map{"imported": imported})
}

// Pre-load soal ke cache saat paket diaktifkan (master plan §2.1):
// siswa baca dari cache (<1ms), bukan Postgres.
func (s *Server) activateExam(c fiber.Ctx) error {
	eid, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	ctx := c.RequestCtx()
	if _, err := s.pool.Exec(ctx, `UPDATE exams SET aktif=TRUE WHERE id=$1`, eid); err != nil {
		return err
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id,soal,opsi FROM questions WHERE exam_id=$1 ORDER BY id`, eid)
	if err != nil {
		return err
	}
	defer rows.Close()
	type cachedQ struct {
		ID   int64    `json:"id"`
		Soal string   `json:"soal"`
		Opsi []string `json:"opsi"`
	}
	qs := []cachedQ{}
	for rows.Next() {
		var q cachedQ
		var oj []byte
		rows.Scan(&q.ID, &q.Soal, &oj)
		json.Unmarshal(oj, &q.Opsi)
		qs = append(qs, q)
	}
	b, _ := json.Marshal(qs)
	s.cache.Set(ctx, examKey(eid), string(b), 6*time.Hour)
	s.log.Info("soal pre-loaded ke cache", zap.Int64("exam", eid), zap.Int("n", len(qs)))
	return c.JSON(fiber.Map{"ok": true, "cached": len(qs)})
}

func examKey(id int64) string { return fmt.Sprintf("exam:%d:questions", id) }

func (s *Server) logCheatSignal(c fiber.Ctx) error {
	cl := claimsOf(c)
	var b struct {
		ExamID int64  `json:"exam_id"`
		Jenis  string `json:"jenis"` // onPause, screenshot_blocked, dll
	}
	if err := c.Bind().Body(&b); err != nil || b.Jenis == "" {
		return fiber.ErrBadRequest
	}
	_, err := s.pool.Exec(c.RequestCtx(),
		`INSERT INTO cheat_signals(student_id,exam_id,jenis) VALUES($1,NULLIF($2,0),$3)`, cl.UID, b.ExamID, b.Jenis)
	if err != nil {
		return err
	}
	// Aturan #1: pencatatan log non-blocking via goroutine.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.log.Info("cheat signal", zap.Int64("student", cl.UID), zap.Int64("exam", b.ExamID), zap.String("jenis", b.Jenis))
		_ = ctx
	}()
	return c.JSON(fiber.Map{"ok": true})
}

// ---------- sisi siswa ----------

func (s *Server) studentDashboard(c fiber.Ctx) error {
	cl := claimsOf(c)
	ctx := c.RequestCtx()
	var nama, kelas string
	s.pool.QueryRow(ctx, `SELECT nama,kelas FROM students WHERE id=$1`, cl.UID).Scan(&nama, &kelas)
	var tugasBelum, materiN, ujianAktif int
	s.pool.QueryRow(ctx,
		`SELECT count(*) FROM assignments a WHERE (a.kelas='' OR a.kelas=$1)
		 AND NOT EXISTS(SELECT 1 FROM submissions sb WHERE sb.assignment_id=a.id AND sb.student_id=$2)`,
		kelas, cl.UID).Scan(&tugasBelum)
	s.pool.QueryRow(ctx, `SELECT count(*) FROM materials WHERE kelas='' OR kelas=$1`, kelas).Scan(&materiN)
	s.pool.QueryRow(ctx,
		`SELECT count(*) FROM exams WHERE aktif AND (kelas='' OR kelas=$1)`, kelas).Scan(&ujianAktif)
	return c.JSON(fiber.Map{"nama": nama, "kelas": kelas,
		"tugas_belum": tugasBelum, "materi": materiN, "ujian_aktif": ujianAktif})
}

func (s *Server) studentMaterials(c fiber.Ctx) error {
	cl := claimsOf(c)
	var kelas string
	s.pool.QueryRow(c.RequestCtx(), `SELECT kelas FROM students WHERE id=$1`, cl.UID).Scan(&kelas)
	rows, err := s.pool.Query(c.RequestCtx(),
		`SELECT m.id,m.judul,m.isi,m.file_url,m.created_at,u.nama,s.nama,COALESCE(cp.elemen,''),COALESCE(cp.deskripsi,'')
		 FROM materials m JOIN users u ON u.id=m.guru_id JOIN subjects s ON s.id=m.mapel_id
		 LEFT JOIN cps cp ON cp.id=m.cp_id WHERE m.kelas='' OR m.kelas=$1 ORDER BY m.created_at DESC`, kelas)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []fiber.Map{}
	for rows.Next() {
		var id int64
		var judul, isi, furl, guru, mapel, elemen, cpdesc string
		var createdAt time.Time
		rows.Scan(&id, &judul, &isi, &furl, &createdAt, &guru, &mapel, &elemen, &cpdesc)
		out = append(out, fiber.Map{"id": id, "judul": judul, "isi": isi, "file_url": furl,
			"created_at": createdAt, "guru": guru, "mapel": mapel,
			"cp_elemen": elemen, "cp_deskripsi": cpdesc})
	}
	return c.JSON(out)
}

func (s *Server) studentAssignments(c fiber.Ctx) error {
	cl := claimsOf(c)
	var kelas string
	s.pool.QueryRow(c.RequestCtx(), `SELECT kelas FROM students WHERE id=$1`, cl.UID).Scan(&kelas)
	rows, err := s.pool.Query(c.RequestCtx(),
		`SELECT a.id,a.judul,a.deskripsi,a.file_url,a.deadline,a.max_score,u.nama,s.nama,
		        sub.id,sub.nilai,sub.file_url,sub.jawaban,sub.feedback
		 FROM assignments a JOIN users u ON u.id=a.guru_id JOIN subjects s ON s.id=a.mapel_id
		 LEFT JOIN submissions sub ON sub.assignment_id=a.id AND sub.student_id=$1
		 WHERE a.kelas='' OR a.kelas=$2 ORDER BY a.deadline NULLS LAST`, cl.UID, kelas)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []fiber.Map{}
	for rows.Next() {
		var id int64
		var judul, desc, furl, guru, mapel, feedback string
		var deadline *time.Time
		var maxScore int
		var subID *int64
		var nilai *int
		var subFile, subJawaban *string
		rows.Scan(&id, &judul, &desc, &furl, &deadline, &maxScore, &guru, &mapel,
			&subID, &nilai, &subFile, &subJawaban, &feedback)
		item := fiber.Map{"id": id, "judul": judul, "deskripsi": desc, "file_url": furl,
			"deadline": deadline, "max_score": maxScore, "guru": guru, "mapel": mapel,
			"submitted": subID != nil, "nilai": nilai, "feedback": feedback}
		if subJawaban != nil {
			item["jawaban_saya"] = *subJawaban
		}
		if subFile != nil {
			item["file_saya"] = *subFile
		}
		out = append(out, item)
	}
	return c.JSON(out)
}

func (s *Server) submitAssignment(c fiber.Ctx) error {
	cl := claimsOf(c)
	aid, _ := strconv.ParseInt(c.FormValue("assignment_id"), 10, 64)
	if aid == 0 {
		return fiber.ErrBadRequest
	}
	// deadline check
	var dl *time.Time
	var kelas string
	err := s.pool.QueryRow(c.RequestCtx(), `SELECT deadline,kelas FROM assignments WHERE id=$1`, aid).Scan(&dl, &kelas)
	if err != nil {
		return fiber.ErrNotFound
	}
	var skelas string
	s.pool.QueryRow(c.RequestCtx(), `SELECT kelas FROM students WHERE id=$1`, cl.UID).Scan(&skelas)
	if kelas != "" && kelas != skelas {
		return fiber.ErrForbidden
	}
	if dl != nil && time.Now().After(*dl) {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "deadline terlewat"})
	}
	furl, err := s.saveUpload(c, "file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	jawaban := c.FormValue("jawaban")
	// Idempotent upsert (Aturan #2): UNIQUE(assignment_id,student_id).
	_, err = s.pool.Exec(c.RequestCtx(),
		`INSERT INTO submissions(assignment_id,student_id,jawaban,file_url) VALUES($1,$2,$3,$4)
		 ON CONFLICT(assignment_id,student_id) DO UPDATE SET
		   jawaban=EXCLUDED.jawaban,
		   file_url=CASE WHEN EXCLUDED.file_url<>'' THEN EXCLUDED.file_url ELSE submissions.file_url END,
		   updated_at=now()`,
		aid, cl.UID, jawaban, furl)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (s *Server) studentExams(c fiber.Ctx) error {
	cl := claimsOf(c)
	var kelas string
	s.pool.QueryRow(c.RequestCtx(), `SELECT kelas FROM students WHERE id=$1`, cl.UID).Scan(&kelas)
	rows, err := s.pool.Query(c.RequestCtx(),
		`SELECT e.id,e.nama,e.durasi_menit,e.mulai_at,e.selesai_at,s.nama,
		        (SELECT COUNT(*) FROM questions qq WHERE qq.exam_id=e.id) AS jml,
		        att.status,att.skor
		 FROM exams e JOIN subjects s ON s.id=e.mapel_id
		 LEFT JOIN attempts att ON att.exam_id=e.id AND att.student_id=$1
		 WHERE e.aktif AND (e.kelas='' OR e.kelas=$2)`, cl.UID, kelas)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []fiber.Map{}
	for rows.Next() {
		var id int64
		var nama, mapel string
		var dur, jml int
		var mulai, selesai *time.Time
		var status *string
		var skor *int
		rows.Scan(&id, &nama, &dur, &mulai, &selesai, &mapel, &jml, &status, &skor)
		item := fiber.Map{"id": id, "nama": nama, "durasi_menit": dur, "mapel": mapel,
			"jumlah_soal": jml, "mulai_at": mulai, "selesai_at": selesai, "skor": skor}
		if status != nil {
			item["status"] = *status
		}
		out = append(out, item)
	}
	return c.JSON(out)
}

func (s *Server) startExam(c fiber.Ctx) error {
	cl := claimsOf(c)
	eid, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	ctx := c.RequestCtx()

	// Ambil soal dari cache dulu (master plan §2.1), fallback Postgres.
	type cachedQ struct {
		ID   int64    `json:"id"`
		Soal string   `json:"soal"`
		Opsi []string `json:"opsi"`
	}
	var qs []cachedQ
	if v, ok := s.cache.Get(ctx, examKey(eid)); ok {
		json.Unmarshal([]byte(v), &qs)
	}
	if qs == nil {
		rows, err := s.pool.Query(ctx, `SELECT id,soal,opsi FROM questions WHERE exam_id=$1 ORDER BY id`, eid)
		if err != nil {
			return err
		}
		for rows.Next() {
			var q cachedQ
			var oj []byte
			rows.Scan(&q.ID, &q.Soal, &oj)
			json.Unmarshal(oj, &q.Opsi)
			qs = append(qs, q)
		}
		rows.Close()
		b, _ := json.Marshal(qs)
		s.cache.Set(ctx, examKey(eid), string(b), 6*time.Hour)
	}

	// Upsert attempt (idempotent: resume, bukan mulai ulang).
	var attID int64
	var dur int
	err := s.pool.QueryRow(ctx,
		`INSERT INTO attempts(exam_id,student_id) VALUES($1,$2)
		 ON CONFLICT(exam_id,student_id) DO NOTHING RETURNING id`, eid, cl.UID).Scan(&attID)
	if err != nil { // sudah ada → resume
		s.pool.QueryRow(ctx,
			`SELECT id FROM attempts WHERE exam_id=$1 AND student_id=$2`, eid, cl.UID).Scan(&attID)
	}
	// Ujian yang sudah selesai tidak boleh dikerjakan ulang.
	var status string
	s.pool.QueryRow(ctx, `SELECT status FROM attempts WHERE id=$1`, attID).Scan(&status)
	if status == "selesai" {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "Ujian ini sudah dikerjakan dan dinilai.", "attempt_id": attID})
	}
	s.pool.QueryRow(ctx, `SELECT durasi_menit FROM exams WHERE id=$1`, eid).Scan(&dur)

	// mulai_at attempt → frontend hitung sisa waktu dari server (resume-safe).
	var mulaiAt time.Time
	s.pool.QueryRow(ctx, `SELECT mulai_at FROM attempts WHERE id=$1`, attID).Scan(&mulaiAt)

	// Jawaban tersimpan utk resume
	saved := map[int64]int{}
	rows, _ := s.pool.Query(ctx, `SELECT question_id,jawaban FROM answers WHERE attempt_id=$1`, attID)
	if rows != nil {
		for rows.Next() {
			var qid int64
			var jw int
			rows.Scan(&qid, &jw)
			saved[qid] = jw
		}
		rows.Close()
	}
	type outQ struct {
		ID   int64    `json:"id"`
		Soal string   `json:"soal"`
		Opsi []string `json:"opsi"`
		Mine int      `json:"mine"`
	}
	out := make([]outQ, 0, len(qs))
	for _, q := range qs {
		mine := -1 // belum dijawab
		if v, ok := saved[q.ID]; ok {
			mine = v
		}
		out = append(out, outQ{q.ID, q.Soal, q.Opsi, mine})
	}
	return c.JSON(fiber.Map{"attempt_id": attID, "durasi_menit": dur, "mulai_at": mulaiAt, "questions": out})
}

// Sync jawaban berkala (master plan §2.2 Opsi A): payload batch kecil,
// UPSERT per UNIQUE(attempt_id,question_id) → aman double-tap.
func (s *Server) syncAnswers(c fiber.Ctx) error {
	cl := claimsOf(c)
	aid, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	var b struct {
		Answers []struct {
			QuestionID int64 `json:"question_id"`
			Jawaban    int   `json:"jawaban"`
		} `json:"answers"`
	}
	if err := c.Bind().Body(&b); err != nil || len(b.Answers) == 0 {
		return fiber.ErrBadRequest
	}
	ctx := c.RequestCtx()
	// attempt milik siswa ini?
	var owned bool
	s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM attempts WHERE id=$1 AND student_id=$2 AND status='berjalan')`,
		aid, cl.UID).Scan(&owned)
	if !owned {
		return fiber.ErrForbidden
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	batchSQL := `INSERT INTO answers(attempt_id,question_id,jawaban) VALUES($1,$2,$3)
	             ON CONFLICT(attempt_id,question_id) DO UPDATE SET jawaban=EXCLUDED.jawaban,updated_at=now()`
	for _, a := range b.Answers {
		if _, err := tx.Exec(ctx, batchSQL, aid, a.QuestionID, a.Jawaban); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"ok": true, "n": len(b.Answers)})
}

func (s *Server) finishAttempt(c fiber.Ctx) error {
	cl := claimsOf(c)
	aid, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	ctx := c.RequestCtx()
	var owned bool
	s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM attempts WHERE id=$1 AND student_id=$2 AND status='berjalan')`,
		aid, cl.UID).Scan(&owned)
	if !owned {
		return fiber.ErrForbidden
	}
	// Pessimistic lock baris attempt saat kalkulasi (master plan §1):
	// lock attempt dulu (FOR UPDATE), baru hitung agregat terpisah tanpa lock.
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var selesai bool
	err = tx.QueryRow(ctx,
		`SELECT status='selesai' FROM attempts WHERE id=$1 AND student_id=$2 FOR UPDATE`,
		aid, cl.UID).Scan(&selesai)
	if err != nil {
		return err
	}
	if selesai {
		_ = tx.Commit(ctx)
		return c.JSON(fiber.Map{"ok": true, "error": "sudah selesai"})
	}
	var skor, totalBobot int
	err = tx.QueryRow(ctx,
		`SELECT COALESCE(sum(CASE WHEN a.jawaban=q.kunci THEN q.bobot ELSE 0 END),0),
		        COALESCE(sum(q.bobot),0)
		 FROM answers a JOIN questions q ON q.id=a.question_id
		 WHERE a.attempt_id=$1`, aid).Scan(&skor, &totalBobot)
	if err != nil {
		return err
	}
	persen := 0
	if totalBobot > 0 {
		persen = skor * 100 / totalBobot
	}
	if _, err := tx.Exec(ctx,
		`UPDATE attempts SET status='selesai',skor=$1,selesai_at=now() WHERE id=$2`, persen, aid); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	// Aturan #1: kalkulasi/log non-blocking.
	go func() {
		s.log.Info("attempt selesai", zap.Int64("attempt", aid), zap.Int("skor", persen))
	}()
	return c.JSON(fiber.Map{"ok": true, "skor": persen})
}

// Hasil ujian untuk halaman hasil siswa (benar/salah/kosong seperti CBT).
func (s *Server) attemptResult(c fiber.Ctx) error {
	cl := claimsOf(c)
	aid, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	ctx := c.RequestCtx()
	var examNama, mapel, kelas string
	var skor, benar, salah, kosong, total int
	var mulai, selesai *time.Time
	err := s.pool.QueryRow(ctx,
		`SELECT e.nama, s.nama, a.skor, a.mulai_at, a.selesai_at, st.kelas,
		        (SELECT COUNT(*) FROM answers aw JOIN questions qw ON qw.id=aw.question_id
		         WHERE aw.attempt_id=a.id AND aw.jawaban=qw.kunci),
		        (SELECT COUNT(*) FROM answers aw JOIN questions qw ON qw.id=aw.question_id
		         WHERE aw.attempt_id=a.id AND aw.jawaban<>qw.kunci AND aw.jawaban>=0),
		        (SELECT COUNT(*) FROM questions qw WHERE qw.exam_id=e.id)
		 FROM attempts a
		 JOIN exams e ON e.id=a.exam_id
		 JOIN subjects s ON s.id=e.mapel_id
		 JOIN students st ON st.id=a.student_id
		 WHERE a.id=$1 AND a.student_id=$2 AND a.status='selesai'`, aid, cl.UID).
		Scan(&examNama, &mapel, &skor, &mulai, &selesai, &kelas, &benar, &salah, &total)
	if err != nil {
		return fiber.ErrNotFound
	}
	kosong = total - benar - salah
	if kosong < 0 {
		kosong = 0
	}
	var nama, nis string
	s.pool.QueryRow(ctx, `SELECT nama,nis FROM students WHERE id=$1`, cl.UID).Scan(&nama, &nis)
	return c.JSON(fiber.Map{
		"exam": examNama, "mapel": mapel, "skor": skor,
		"benar": benar, "salah": salah, "kosong": kosong, "total": total,
		"mulai_at": mulai, "selesai_at": selesai,
		"siswa": fiber.Map{"nama": nama, "nis": nis, "kelas": kelas},
	})
}

// ---------- util ----------

func (s *Server) saveUpload(c fiber.Ctx, field string) (string, error) {
	fh, err := c.FormFile(field)
	if err != nil {
		return "", nil // file opsional
	}
	if fh.Size > s.cfg.MaxUploadMB*1024*1024 {
		return "", fmt.Errorf("file terlalu besar (maks %dMB)", s.cfg.MaxUploadMB)
	}
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	allowed := map[string]bool{".pdf": true, ".doc": true, ".docx": true, ".ppt": true,
		".pptx": true, ".xls": true, ".xlsx": true, ".png": true, ".jpg": true, ".jpeg": true,
		".zip": true, ".rar": true, ".mp4": true}
	if !allowed[ext] {
		return "", fmt.Errorf("tipe file %s tidak diizinkan", ext)
	}
	name := randHex(12) + ext
	if err := c.SaveFile(fh, filepath.Join(s.cfg.UploadDir, name)); err != nil {
		return "", err
	}
	return "/uploads/" + name, nil
}

func errOr(c fiber.Ctx, err error, ok fiber.Map) error {
	if err != nil {
		return err
	}
	return c.JSON(ok)
}

func def(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

func col(r []string, i int) string {
	if i < len(r) {
		return r[i]
	}
	return ""
}

func defJk(v string) string {
	if strings.EqualFold(v, "p") || strings.EqualFold(v, "perempuan") {
		return "P"
	}
	return "L"
}
