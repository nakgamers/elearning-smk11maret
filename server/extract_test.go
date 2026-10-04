package main

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

func TestExtractTXT(t *testing.T) {
	txt := strings.Repeat("Fotosintesis adalah proses tumbuhan hijau. ", 20)
	out, err := extractMateriText("materi.txt", strings.NewReader(txt))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Fotosintesis") {
		t.Fatal("teks txt tidak terbaca")
	}
}

func TestExtractPDF(t *testing.T) {
	data := buildMinimalPDF("Klorofil menangkap cahaya matahari untuk fotosintesis tumbuhan hijau.")
	out, err := extractMateriText("materi.pdf", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Klorofil") {
		t.Fatalf("teks pdf tidak terbaca: %q", out)
	}
}

// buildMinimalPDF membuat PDF satu halaman dgn xref yang benar.
func buildMinimalPDF(text string) []byte {
	stream := []byte("BT /F1 12 Tf 72 720 Td (" + text + ") Tj ET")
	objs := [][]byte{
		[]byte("<< /Type /Catalog /Pages 2 0 R >>"),
		[]byte("<< /Type /Pages /Kids [3 0 R] /Count 1 >>"),
		[]byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>"),
		append(append([]byte("<< /Length "+itoa(len(stream))+" >>\nstream\n"), stream...), []byte("\nendstream")...),
		[]byte("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"),
	}
	out := []byte("%PDF-1.4\n")
	var offsets []int
	for i, body := range objs {
		offsets = append(offsets, len(out))
		out = append(out, []byte(itoa(i+1)+" 0 obj\n")...)
		out = append(out, body...)
		out = append(out, []byte("\nendobj\n")...)
	}
	xref := len(out)
	out = append(out, []byte("xref\n0 "+itoa(len(objs)+1)+"\n0000000000 65535 f \n")...)
	for _, o := range offsets {
		out = append(out, []byte(sprintf("%010d 00000 n \n", o))...)
	}
	out = append(out, []byte("trailer\n<< /Size "+itoa(len(objs)+1)+" /Root 1 0 R >>\nstartxref\n"+itoa(xref)+"\n%%EOF\n")...)
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func sprintf(format string, a ...any) string {
	// format sederhana: hanya %010d
	s := format
	if len(a) == 1 {
		if n, ok := a[0].(int); ok {
			digits := itoa(n)
			for len(digits) < 10 {
				digits = "0" + digits
			}
			s = strings.Replace(s, "%010d", digits, 1)
		}
	}
	return s
}

func zipBuf(files map[string]string) *bytes.Buffer {
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	for name, content := range files {
		w, _ := zw.Create(name)
		w.Write([]byte(content))
	}
	zw.Close()
	return buf
}

func TestExtractPPTX(t *testing.T) {
	buf := zipBuf(map[string]string{
		"[Content_Types].xml": `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>`,
		"ppt/slides/slide1.xml": `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" ` +
			`xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">` +
			`<p:txBody><a:p><a:r><a:t>Sel tumbuhan memiliki dinding sel dan kloroplas untuk fotosintesis yang efisien.</a:t></a:r></a:p></p:txBody></p:sld>`,
	})
	out, err := extractMateriText("materi.pptx", bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "kloroplas") {
		t.Fatalf("teks pptx tidak terbaca: %q", out)
	}
}

func TestExtractDOCX(t *testing.T) {
	buf := zipBuf(map[string]string{
		"[Content_Types].xml": `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
			`<w:body><w:p><w:r><w:t>Energi kimia disimpan dalam bentuk glukosa hasil fotosintesis tumbuhan hijau.</w:t></w:r></w:p></w:body></w:document>`,
	})
	out, err := extractMateriText("bab1.docx", bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "glukosa") {
		t.Fatalf("teks docx tidak terbaca: %q", out)
	}
}

func TestExtractUnsupported(t *testing.T) {
	_, err := extractMateriText("materi.exe", strings.NewReader("data"))
	if err == nil {
		t.Fatal("format exe seharusnya ditolak")
	}
}
