package main

// Ekstraksi teks materi dari file upload (PDF/PPTX/DOCX/TXT/MD)
// untuk fitur Kuis AI. Guru cukup upload file — tidak perlu tahu JSON.

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/ledongthuc/pdf"
)

const maxMateriChars = 20000

// extractMateriText membaca teks dari file upload berdasarkan ekstensi.
// Mengembalikan teks yang sudah dipangkas ke maxMateriChars.
func extractMateriText(filename string, r io.Reader) (string, error) {
	data, err := io.ReadAll(io.LimitReader(r, 25<<20)) // maks 25MB
	if err != nil {
		return "", err
	}
	if len(data) == 0 {
		return "", fmt.Errorf("file kosong")
	}
	var text string
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".pdf":
		text, err = extractPDF(data)
	case ".pptx":
		text, err = extractOfficeXML(data, "ppt/slides/", "a:t")
	case ".docx":
		text, err = extractOfficeXML(data, "word/", "w:t")
	case ".txt", ".md", ".markdown":
		text = string(data)
	default:
		return "", fmt.Errorf("format %q tidak didukung (pakai PDF, PPTX, DOCX, TXT, atau MD)", filepath.Ext(filename))
	}
	if err != nil {
		return "", err
	}
	text = strings.TrimSpace(collapseSpace(text))
	if len(text) < 50 {
		return "", fmt.Errorf("teks yang terbaca terlalu sedikit (%d karakter) — pastikan file berisi teks, bukan hasil scan gambar", len(text))
	}
	if len(text) > maxMateriChars {
		text = text[:maxMateriChars]
	}
	return text, nil
}

func extractPDF(data []byte) (string, error) {
	rd, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("gagal baca PDF: %v", err)
	}
	var sb strings.Builder
	n := rd.NumPage()
	if n > 60 {
		n = 60 // batasi 60 halaman pertama
	}
	for i := 1; i <= n; i++ {
		p := rd.Page(i)
		if p.V.IsNull() {
			continue
		}
		t, err := p.GetTextByRow()
		if err != nil {
			continue
		}
		for _, row := range t {
			for _, w := range row.Content {
				sb.WriteString(w.S)
				sb.WriteString(" ")
			}
			sb.WriteString("\n")
		}
	}
	return sb.String(), nil
}

// extractOfficeXML membuka arsip zip Office (pptx/docx) dan menggabungkan
// isi semua node teks <tag> dari file XML di bawah prefix (slide/dokumen).
func extractOfficeXML(data []byte, prefix, tag string) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("gagal baca arsip office: %v", err)
	}
	var sb strings.Builder
	for _, f := range zr.File {
		if !strings.HasPrefix(f.Name, prefix) || !strings.HasSuffix(f.Name, ".xml") {
			continue
		}
		if strings.HasSuffix(f.Name, ".rels") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		collectTextNodes(rc, tag, &sb)
		rc.Close()
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

func collectTextNodes(r io.Reader, tag string, sb *strings.Builder) {
	dec := xml.NewDecoder(r)
	for {
		tok, err := dec.Token()
		if err != nil {
			return
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		name := se.Name.Local
		if se.Name.Space != "" {
			name = se.Name.Space + ":" + se.Name.Local
		}
		// cocokkan "a:t"/"w:t" maupun lokal "t"
		if name != tag && se.Name.Local != "t" {
			continue
		}
		var s string
		if err := dec.DecodeElement(&s, &se); err == nil {
			s = strings.TrimSpace(s)
			if s != "" {
				sb.WriteString(s)
				sb.WriteString(" ")
			}
		}
	}
}

func collapseSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
