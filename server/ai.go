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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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
		Questions []QuizQuestion `json:"questions"`
	}
	dec := json.NewDecoder(strings.NewReader(s))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&wrap); err != nil {
		return nil, fmt.Errorf("JSON tidak valid: %v", err)
	}
	if len(wrap.Questions) == 0 {
		return nil, fmt.Errorf("tidak ada soal dalam JSON (butuh field \"questions\")")
	}
	if len(wrap.Questions) > 50 {
		return nil, fmt.Errorf("maksimal 50 soal, dapat %d", len(wrap.Questions))
	}
	for i, q := range wrap.Questions {
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
	return wrap.Questions, nil
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
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ac.apiKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("gagal menghubungi AI: %v", err)
	}
	defer resp.Body.Close()
	rawBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", fmt.Errorf("respons AI tidak terbaca: %v", err)
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(rawBody, &out); err != nil {
		return "", fmt.Errorf("respons AI tidak terbaca (%d): %s", resp.StatusCode, truncate(string(rawBody), 200))
	}
	if len(out.Error) > 0 && string(out.Error) != "null" {
		return "", fmt.Errorf("AI menolak (%d): %s", resp.StatusCode, truncate(string(out.Error), 200))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("AI HTTP %d: %s", resp.StatusCode, truncate(string(rawBody), 200))
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("AI tidak mengembalikan jawaban")
	}
	return out.Choices[0].Message.Content, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
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

// POST /api/quiz/generate  {materi?, jumlah, kesulitan, provider?} -> {questions}
// atau multipart/form-data dengan field "file" (pdf/pptx/docx/txt/md).
// 501 bila provider yang dipilih belum dikonfigurasi.
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
	userPrompt := fmt.Sprintf(
		"Buat %d soal pilihan ganda dengan tingkat kesulitan \"%s\" dari materi berikut:\n\n%s",
		jumlah, kesulitan, materi)
	ctx, cancel := context.WithTimeout(c.RequestCtx(), time.Duration(s.cfg.AITimeoutSec)*time.Second)
	defer cancel()
	raw, err := s.callAIChat(ctx, ac, quizSystemPrompt, userPrompt)
	if err != nil {
		s.log.Warn("quiz generate gagal", zapErr(err))
		return fiber.NewError(fiber.StatusBadGateway, "gagal meminta AI: "+err.Error())
	}
	qs, err := validateQuizQuestions(json.RawMessage(raw))
	if err != nil {
		s.log.Warn("quiz generate: JSON AI tidak valid", zapErr(err))
		return fiber.NewError(fiber.StatusBadGateway, "AI mengembalikan format yang tidak valid, coba lagi: "+err.Error())
	}
	return c.JSON(fiber.Map{"questions": qs})
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
