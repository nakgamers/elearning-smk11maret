package main

// Fitur Kuis AI: guru menempel materi -> AI menghasilkan soal pilihan ganda
// dalam format JSON ketat, atau guru menempel JSON hasil chat (mis. dengan
// asisten AI) untuk divalidasi. Tidak ada tabel baru: kuis bersifat sesi
// kelas (ditampilkan di IFP), bukan ujian tersimpan.

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
)

// QuizQuestion adalah kontrak JSON soal kuis. Dipakai di kedua jalur:
// generate otomatis (AI) dan tempel JSON (hasil chat dengan asisten AI).
// Contoh yang valid:
// {"soal":"...","opsi":["A","B","C","D"],"kunci":0,"pembahasan":"..."}
type QuizQuestion struct {
	Soal       string   `json:"soal"`
	Opsi       []string `json:"opsi"`
	Kunci      int      `json:"kunci"`
	Pembahasan string   `json:"pembahasan"`
}

// validateQuizQuestions memastikan JSON kuis sesuai kontrak:
// 1..50 soal, tiap soal punya tepat 4 opsi tak-kosong dan kunci 0..3.
func validateQuizQuestions(raw json.RawMessage) ([]QuizQuestion, error) {
	trimmed := bytes.TrimSpace(raw)
	// Toleransi: buang pembungkus code fence ```json ... ```
	s := string(trimmed)
	if i := strings.Index(s, "```"); i >= 0 {
		s = s[i+3:]
		if j := strings.Index(s, "```"); j >= 0 {
			s = s[:j]
		}
		s = strings.TrimSpace(s)
		if strings.HasPrefix(strings.ToLower(s), "json") {
			s = strings.TrimSpace(s[4:])
		}
	}
	// Toleransi: ambil objek JSON terluar bila ada teks di sekitarnya.
	if i := strings.Index(s, "{"); i > 0 {
		if j := strings.LastIndex(s, "}"); j > i {
			s = s[i : j+1]
		}
	}
var wrap struct {
		Questions []json.RawMessage `json:"questions"`
	}
	dec := json.NewDecoder(strings.NewReader(s))
	// Sengaja TANPA DisallowUnknownFields: model kadang menambah field
	// ekstra; yang penting field wajib ada dan valid.
	if err := dec.Decode(&wrap); err != nil {
		return nil, fmt.Errorf("JSON tidak valid: %v", err)
	}
	if len(wrap.Questions) == 0 {
		return nil, fmt.Errorf("tidak ada soal dalam JSON (butuh field \"questions\")")
	}
	if len(wrap.Questions) > 50 {
		return nil, fmt.Errorf("maksimal 50 soal, dapat %d", len(wrap.Questions))
	}
	qs := make([]QuizQuestion, 0, len(wrap.Questions))
	for i, rawQ := range wrap.Questions {
		q, err := normalizeQuizQuestion(rawQ)
		if err != nil {
			return nil, fmt.Errorf("soal %d: %v", i+1, err)
		}
		qs = append(qs, q)
	}
	for i, q := range qs {
		n := i + 1
		if strings.TrimSpace(q.Soal) == "" {
			return nil, fmt.Errorf("soal %d: teks soal kosong", n)
		}
		if len(q.Opsi) != 4 {
			return nil, fmt.Errorf("soal %d: harus tepat 4 opsi, dapat %d", n, len(q.Opsi))
		}
		for j, o := range q.Opsi {
			if strings.TrimSpace(o) == "" {
				return nil, fmt.Errorf("soal %d: opsi %s kosong", n, string(rune('A'+j)))
			}
		}
		if q.Kunci < 0 || q.Kunci > 3 {
			return nil, fmt.Errorf("soal %d: kunci harus 0..3, dapat %d", n, q.Kunci)
		}
	}
	return qs, nil
}

// normalizeQuizQuestion menoleransi variasi field yang lumrah dikeluarkan
// model: typo ("soul" utk "soal"), sinonim ("pertanyaan", "options",
// "jawaban", "penjelasan"), dan kunci berupa huruf "A"-"D".
func normalizeQuizQuestion(raw json.RawMessage) (QuizQuestion, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return QuizQuestion{}, fmt.Errorf("bukan objek JSON")
	}
	lower := map[string]json.RawMessage{}
	for k, v := range m {
		lower[strings.ToLower(k)] = v
	}
	// Kanonikalisasi kunci umum.
	renames := map[string]string{
		"soul": "soal", "pertanyaan": "soal", "question": "soal",
		"options": "opsi", "pilihan": "opsi", "choices": "opsi",
		"answer": "kunci", "jawaban": "kunci", "kunci_jawaban": "kunci", "correct": "kunci",
		"penjelasan": "pembahasan", "explanation": "pembahasan", "reason": "pembahasan",
	}
	for typo, canon := range renames {
		if v, ok := lower[typo]; ok {
			if _, exists := lower[canon]; !exists {
				lower[canon] = v
			}
		}
	}
	var q QuizQuestion
	getStr := func(key string) string {
		var s string
		if v, ok := lower[key]; ok {
			_ = json.Unmarshal(v, &s)
		}
		return s
	}
	q.Soal = getStr("soal")
	q.Pembahasan = getStr("pembahasan")
	if v, ok := lower["opsi"]; ok {
		_ = json.Unmarshal(v, &q.Opsi)
	}
	// Kunci: angka 0-3, atau huruf "A"-"D" / "a"-"d".
	q.Kunci = -1
	if v, ok := lower["kunci"]; ok {
		var n int
		if err := json.Unmarshal(v, &n); err == nil {
			q.Kunci = n
		} else {
			var s string
			if err := json.Unmarshal(v, &s); err == nil {
				s = strings.ToUpper(strings.TrimSpace(s))
				if len(s) == 1 && s[0] >= 'A' && s[0] <= 'D' {
					q.Kunci = int(s[0] - 'A')
				}
			}
		}
	}
	return q, nil
}

const quizSystemPrompt = `Kamu adalah generator soal kuis untuk guru SMK di Indonesia. ` +
	`Dari materi yang diberikan, buat soal pilihan ganda dalam Bahasa Indonesia yang jelas dan tidak ambigu. ` +
	`Setiap soal tepat 4 opsi (A-D), tepat 1 jawaban benar, dan sertakan pembahasan singkat (1-2 kalimat). ` +
	`Kembalikan HANYA satu objek JSON valid tanpa markdown, tanpa code fence, tanpa teks lain, dengan struktur persis:\n` +
	`{"questions":[{"soal":"...","opsi":["...","...","...","..."],"kunci":0,"pembahasan":"..."}]}\n` +
	`Field "kunci" adalah indeks opsi benar (0=A, 1=B, 2=C, 3=D). Acak posisi kunci jawaban antar soal.`

// callAIChat memanggil endpoint chat-completions yang kompatibel OpenAI.
type aiConf struct {
	baseURL string
	apiKey  string
	model   string
}

func (s *Server) callAIChat(ctx context.Context, ac aiConf, system, user string) (string, error) {
	if ac.baseURL == "" || ac.apiKey == "" || ac.model == "" {
		return "", fmt.Errorf("AI belum dikonfigurasi")
	}
	// Retry untuk error transient (503/429 — mis. model Google overload).
	// Backoff 30 dtk lalu 60 dtk; total dibatasi context dari pemanggil.
	backoffs := []time.Duration{0, 30 * time.Second, 60 * time.Second}
	var lastErr error
	for i, wait := range backoffs {
		if i > 0 {
			t := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				t.Stop()
				return "", fmt.Errorf("gagal menghubungi AI: %v", ctx.Err())
			case <-t.C:
			}
			s.log.Info(fmt.Sprintf("quiz: retry AI (percobaan %d)", i+1))
		}
		content, status, err := s.callAIChatOnce(ctx, ac, system, user)
		if err == nil {
			return content, nil
		}
		lastErr = err
		if status == 503 || status == 429 {
			continue // transient, coba lagi
		}
		return "", err // permanen (401/404/400...), langsung gagal
	}
	if lastErr != nil {
		s.log.Warn("quiz: AI tetap sibuk setelah retry", zapErr(lastErr))
	}
	return "", fmt.Errorf("AI sedang sibuk (server penuh), coba lagi beberapa saat")
}

func (s *Server) callAIChatOnce(ctx context.Context, ac aiConf, system, user string) (string, int, error) {
	body, _ := json.Marshal(map[string]any{
		"model": ac.model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"temperature": 0.7,
	})
	url := strings.TrimRight(ac.baseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ac.apiKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("gagal menghubungi AI: %v", err)
	}
	defer resp.Body.Close()
	rawBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", resp.StatusCode, fmt.Errorf("respons AI tidak terbaca: %v", err)
	}
	content, err := parseChatCompletion(resp.StatusCode, rawBody)
	return content, resp.StatusCode, err
}

// parseChatCompletion membaca respons chat-completions dalam dua bentuk:
// JSON langsung, atau SSE ("data: {...}" + "data: [DONE]") seperti yang
// dikembalikan bridge 9Router untuk request non-streaming.
func parseChatCompletion(status int, rawBody []byte) (string, error) {
	candidates := [][]byte{bytes.TrimSpace(rawBody)}
	// Kumpulkan payload SSE bila ada.
	for _, line := range bytes.Split(rawBody, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if bytes.HasPrefix(line, []byte("data:")) {
			payload := bytes.TrimSpace(line[len("data:"):])
			if len(payload) > 0 && !bytes.Equal(payload, []byte("[DONE]")) {
				candidates = append(candidates, payload)
			}
		}
	}
	// Kasus bridge 9Router: JSON diikuti "data: [DONE]" dalam SATU baris.
	// Ambil objek JSON terluar via pencocokan kurung kurawal.
	if obj := extractJSONObject(rawBody); obj != nil {
		candidates = append(candidates, obj)
	}
	var lastErr error
	for _, cand := range candidates {
		var out struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
			Error json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal(cand, &out); err != nil {
			lastErr = err
			continue
		}
		if len(out.Error) > 0 && string(out.Error) != "null" {
			return "", fmt.Errorf("AI menolak (%d): %s", status, truncate(string(out.Error), 200))
		}
		if status < 200 || status >= 300 {
			return "", fmt.Errorf("AI HTTP %d: %s", status, truncate(string(cand), 200))
		}
		if len(out.Choices) == 0 {
			lastErr = fmt.Errorf("AI tidak mengembalikan jawaban")
			continue
		}
		return out.Choices[0].Message.Content, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("respons AI tidak terbaca (%d): %s", status, truncate(string(rawBody), 200))
	}
	return "", lastErr
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// extractJSONObject mengambil objek JSON terluar dari awal body dengan
// mencocokkan kurung kurawal (menghormati string dan escape).
func extractJSONObject(b []byte) []byte {
	start := bytes.IndexByte(b, '{')
	if start < 0 {
		return nil
	}
	depth := 0
	inStr := false
	esc := false
	for i := start; i < len(b); i++ {
		c := b[i]
		if inStr {
			if esc {
				esc = false
			} else if c == '\\' {
				esc = true
			} else if c == '"' {
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return b[start : i+1]
			}
		}
	}
	return nil
}

// resolveAI menentukan konfigurasi AI berdasarkan provider pilihan guru:
//   - "server" (default): memakai env server (mis. endpoint 9Router sekolah).
//   - "gemini": memakai API key Gemini pribadi guru yang tersimpan terenkripsi.
func (s *Server) resolveAI(c fiber.Ctx, provider string) (aiConf, error) {
	if provider == "" {
		provider = "server"
	}
	switch provider {
	case "server":
		ac := aiConf{baseURL: s.cfg.AIBaseURL, apiKey: s.cfg.AIAPIKey, model: s.cfg.AIModel}
		if ac.baseURL == "" || ac.apiKey == "" || ac.model == "" {
			return ac, fmt.Errorf("Quineilla belum dikonfigurasi. Minta admin mengisi AI_BASE_URL, AI_API_KEY, dan AI_MODEL — atau gunakan Gemini pribadi.")
		}
		return ac, nil
	case "gemini":
		key, err := s.getAIKey(c.RequestCtx(), claimsOf(c).UID, "gemini")
		if err != nil || key == "" {
			return aiConf{}, fmt.Errorf("kunci Gemini belum disimpan. Tempel API key Gemini (gratis dari aistudio.google.com) di pengaturan.")
		}
		return aiConf{
			baseURL: "https://generativelanguage.googleapis.com/v1beta/openai",
			apiKey:  key,
			model:   s.cfg.AIGeminiModel,
		}, nil
	default:
		return aiConf{}, fmt.Errorf("provider %q tidak dikenal", provider)
	}
}

// POST /api/quiz/generate  {materi?, jumlah, kesulitan, provider?} -> 202 {job_id}
// atau multipart/form-data dengan field "file" (pdf/pptx/docx/txt/md).
// 501 bila provider yang dipilih belum dikonfigurasi.
//
// ASYNC: generate soal via AI bisa 1-3 menit (materi besar). Agar kebal
// terhadap timeout di rantai proxy (Next.js rewrite, dsb.), endpoint ini
// langsung mengembalikan job_id; frontend polling GET /api/quiz/jobs/:id.
func (s *Server) postQuizGenerate(c fiber.Ctx) error {
	var materi, provider string
	var jumlah int
	var kesulitan string

	if strings.HasPrefix(c.Get("Content-Type"), "multipart/form-data") {
		fh, err := c.FormFile("file")
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "file tidak ditemukan (field \"file\")")
		}
		f, err := fh.Open()
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "gagal membuka file")
		}
		text, err := extractMateriText(fh.Filename, f)
		f.Close()
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, err.Error())
		}
		materi = text
		provider = c.FormValue("provider")
		fmt.Sscanf(c.FormValue("jumlah"), "%d", &jumlah)
		kesulitan = c.FormValue("kesulitan")
	} else {
		var b struct {
			Materi    string `json:"materi"`
			Jumlah    int    `json:"jumlah"`
			Kesulitan string `json:"kesulitan"`
			Provider  string `json:"provider"`
		}
		if err := c.Bind().JSON(&b); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "body tidak valid")
		}
		materi = strings.TrimSpace(b.Materi)
		provider = b.Provider
		jumlah = b.Jumlah
		kesulitan = b.Kesulitan
	}

	if len(materi) < 50 {
		return fiber.NewError(fiber.StatusBadRequest, "materi terlalu pendek (min. 50 karakter)")
	}
	if len(materi) > maxMateriChars {
		materi = materi[:maxMateriChars]
	}
	if jumlah <= 0 {
		jumlah = 10
	}
	if jumlah > 50 {
		jumlah = 50
	}
	if kesulitan == "" {
		kesulitan = "sedang"
	}
	ac, err := s.resolveAI(c, provider)
	if err != nil {
		return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{"error": err.Error()})
	}

	job := &quizJob{status: "processing", createdAt: time.Now()}
	quizJobs.Store(job.id(), job)
	go s.runQuizJob(job, ac, materi, jumlah, kesulitan)
	// 202: langsung kembali, hasil diambil via polling.
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"job_id": job.id()})
}

// quizJob adalah satu pekerjaan generate soal yang berjalan di background.
type quizJob struct {
	mu        sync.Mutex
	jobID     string
	status    string // "processing" | "done" | "error"
	questions []QuizQuestion
	errMsg    string
	createdAt time.Time
}

func (j *quizJob) id() string {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.jobID == "" {
		var b [16]byte
		rand.Read(b[:])
		j.jobID = hex.EncodeToString(b[:])
	}
	return j.jobID
}

func (j *quizJob) finish(qs []QuizQuestion, errMsg string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if errMsg != "" {
		j.status = "error"
		j.errMsg = errMsg
	} else {
		j.status = "done"
		j.questions = qs
	}
}

func (j *quizJob) snapshot() (status string, qs []QuizQuestion, errMsg string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.status, j.questions, j.errMsg
}

// quizJobs menyimpan job yang aktif. In-memory: cukup untuk 1 instance;
// job kedaluwarsa 30 menit dan dibersihkan saat diakses.
var quizJobs sync.Map // jobID -> *quizJob

func (s *Server) runQuizJob(job *quizJob, ac aiConf, materi string, jumlah int, kesulitan string) {
	userPrompt := fmt.Sprintf(
		"Buat %d soal pilihan ganda dengan tingkat kesulitan \"%s\" dari materi berikut:\n\n%s",
		jumlah, kesulitan, materi)
	// Background: tidak terikat request HTTP (client boleh disconnect).
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.cfg.AITimeoutSec)*time.Second)
	defer cancel()
	raw, err := s.callAIChat(ctx, ac, quizSystemPrompt, userPrompt)
	if err != nil {
		s.log.Warn("quiz job gagal", zapErr(err))
		job.finish(nil, "gagal meminta AI: "+err.Error())
		return
	}
	qs, err := validateQuizQuestions(json.RawMessage(raw))
	if err != nil {
		s.log.Warn("quiz job: JSON AI tidak valid", zapErr(err))
		job.finish(nil, "AI mengembalikan format yang tidak valid, coba lagi")
		return
	}
	job.finish(qs, "")
}

// GET /api/quiz/jobs/:id -> {status} | {status:"done", questions} | {status:"error", error}
func (s *Server) getQuizJob(c fiber.Ctx) error {
	v, ok := quizJobs.Load(c.Params("id"))
	if !ok {
		return fiber.NewError(fiber.StatusNotFound, "job tidak ditemukan / kedaluwarsa")
	}
	job := v.(*quizJob)
	if time.Since(job.createdAt) > 30*time.Minute {
		quizJobs.Delete(c.Params("id"))
		return fiber.NewError(fiber.StatusNotFound, "job tidak ditemukan / kedaluwarsa")
	}
	status, qs, errMsg := job.snapshot()
	switch status {
	case "done":
		return c.JSON(fiber.Map{"status": "done", "questions": qs})
	case "error":
		return c.JSON(fiber.Map{"status": "error", "error": errMsg})
	default:
		return c.JSON(fiber.Map{"status": "processing"})
	}
}

// POST /api/quiz/validate  {raw: "<teks JSON>"} -> {questions}
// Memvalidasi JSON tempelan (mis. hasil chat dengan asisten AI, boleh
// beserta code fence / teks di sekitarnya) ke kontrak kuis.
func (s *Server) postQuizValidate(c fiber.Ctx) error {
	var b struct {
		Raw string `json:"raw"`
	}
	if err := c.Bind().JSON(&b); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "body tidak valid")
	}
	if len(bytes.TrimSpace([]byte(b.Raw))) == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "JSON kosong")
	}
	qs, err := validateQuizQuestions(json.RawMessage(b.Raw))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	return c.JSON(fiber.Map{"questions": qs})
}

// === Kunci AI pribadi guru (Gemini) — tersimpan terenkripsi AES-GCM ===

func (s *Server) aiCipher() (cipher.AEAD, error) {
	sum := sha256.Sum256([]byte("elearning-ai-key:" + s.cfg.Secret))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func (s *Server) encryptAIKey(plain string) (string, error) {
	aead, err := s.aiCipher()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := aead.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(ct), nil
}

func (s *Server) decryptAIKey(enc string) (string, error) {
	aead, err := s.aiCipher()
	if err != nil {
		return "", err
	}
	ct, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return "", err
	}
	nonce := ct[:aead.NonceSize()]
	pt, err := aead.Open(nil, nonce, ct[aead.NonceSize():], nil)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

func (s *Server) getAIKey(ctx context.Context, uid int64, provider string) (string, error) {
	var enc string
	err := s.pool.QueryRow(ctx,
		`SELECT key_enc FROM ai_keys WHERE user_id=$1 AND provider=$2`, uid, provider).Scan(&enc)
	if err != nil {
		return "", err
	}
	return s.decryptAIKey(enc)
}

// GET /api/ai/status -> {server: bool, gemini: bool}
func (s *Server) getAIStatus(c fiber.Ctx) error {
	serverOK := s.cfg.AIBaseURL != "" && s.cfg.AIAPIKey != "" && s.cfg.AIModel != ""
	var n int
	_ = s.pool.QueryRow(c.RequestCtx(),
		`SELECT COUNT(*) FROM ai_keys WHERE user_id=$1 AND provider='gemini'`,
		claimsOf(c).UID).Scan(&n)
	return c.JSON(fiber.Map{"server": serverOK, "gemini": n > 0})
}

// POST /api/ai/key  {provider: "gemini", key} — simpan/rotasi kunci pribadi.
func (s *Server) postAIKey(c fiber.Ctx) error {
	var b struct {
		Provider string `json:"provider"`
		Key      string `json:"key"`
	}
	if err := c.Bind().JSON(&b); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "body tidak valid")
	}
	if b.Provider != "gemini" {
		return fiber.NewError(fiber.StatusBadRequest, "provider tidak didukung")
	}
	key := strings.TrimSpace(b.Key)
	if len(key) < 10 {
		return fiber.NewError(fiber.StatusBadRequest, "kunci terlalu pendek")
	}
	enc, err := s.encryptAIKey(key)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "gagal mengenkripsi kunci")
	}
	_, err = s.pool.Exec(c.RequestCtx(), `
		INSERT INTO ai_keys (user_id, provider, key_enc, updated_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (user_id) DO UPDATE SET provider=$2, key_enc=$3, updated_at=now()`,
		claimsOf(c).UID, "gemini", enc)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "gagal menyimpan kunci")
	}
	return c.JSON(fiber.Map{"ok": true})
}

// DELETE /api/ai/key?provider=gemini — hapus kunci pribadi.
func (s *Server) deleteAIKey(c fiber.Ctx) error {
	if c.Query("provider") != "gemini" {
		return fiber.NewError(fiber.StatusBadRequest, "provider tidak didukung")
	}
	_, err := s.pool.Exec(c.RequestCtx(),
		`DELETE FROM ai_keys WHERE user_id=$1 AND provider='gemini'`, claimsOf(c).UID)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "gagal menghapus kunci")
	}
	return c.JSON(fiber.Map{"ok": true})
}
