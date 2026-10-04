package main

// Fitur Kuis AI: guru menempel materi -> AI menghasilkan soal pilihan ganda
// dalam format JSON ketat, atau guru menempel JSON hasil chat (mis. dengan
// asisten AI) untuk divalidasi. Tidak ada tabel baru: kuis bersifat sesi
// kelas (ditampilkan di IFP), bukan ujian tersimpan.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
// Konfigurasi via env: AI_BASE_URL, AI_API_KEY, AI_MODEL.
func (s *Server) callAIChat(ctx context.Context, system, user string) (string, error) {
	cfg := s.cfg
	if cfg.AIBaseURL == "" || cfg.AIAPIKey == "" || cfg.AIModel == "" {
		return "", fmt.Errorf("AI belum dikonfigurasi")
	}
	body, _ := json.Marshal(map[string]any{
		"model": cfg.AIModel,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"temperature": 0.7,
	})
	url := strings.TrimRight(cfg.AIBaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.AIAPIKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("gagal menghubungi AI: %v", err)
	}
	defer resp.Body.Close()
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("respons AI tidak terbaca: %v", err)
	}
	if out.Error != nil {
		return "", fmt.Errorf("AI menolak: %s", out.Error.Message)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("AI HTTP %d", resp.StatusCode)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("AI tidak mengembalikan jawaban")
	}
	return out.Choices[0].Message.Content, nil
}

// POST /api/quiz/generate  {materi, jumlah, kesulitan} -> {questions}
// 501 bila AI belum dikonfigurasi (guru diminta pakai mode Tempel JSON).
func (s *Server) postQuizGenerate(c fiber.Ctx) error {
	var b struct {
		Materi     string `json:"materi"`
		Jumlah     int    `json:"jumlah"`
		Kesulitan  string `json:"kesulitan"`
	}
	if err := c.Bind().JSON(&b); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "body tidak valid")
	}
	materi := strings.TrimSpace(b.Materi)
	if len(materi) < 50 {
		return fiber.NewError(fiber.StatusBadRequest, "materi terlalu pendek (min. 50 karakter)")
	}
	if len(materi) > 20000 {
		return fiber.NewError(fiber.StatusBadRequest, "materi terlalu panjang (maks. 20000 karakter)")
	}
	jumlah := b.Jumlah
	if jumlah <= 0 {
		jumlah = 10
	}
	if jumlah > 50 {
		jumlah = 50
	}
	kesulitan := b.Kesulitan
	if kesulitan == "" {
		kesulitan = "sedang"
	}
	if s.cfg.AIAPIKey == "" || s.cfg.AIBaseURL == "" || s.cfg.AIModel == "" {
		return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
			"error": "AI belum dikonfigurasi di server. Minta admin mengisi AI_BASE_URL, AI_API_KEY, dan AI_MODEL — atau gunakan mode Tempel JSON.",
		})
	}
	userPrompt := fmt.Sprintf(
		"Buat %d soal pilihan ganda dengan tingkat kesulitan \"%s\" dari materi berikut:\n\n%s",
		jumlah, kesulitan, materi)
	ctx, cancel := context.WithTimeout(c.RequestCtx(), time.Duration(s.cfg.AITimeoutSec)*time.Second)
	defer cancel()
	raw, err := s.callAIChat(ctx, quizSystemPrompt, userPrompt)
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
