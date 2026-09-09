package main

import (
	"bytes"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

// Baca xlsx pertama → [][]string, baris 1 (header) dilewati, sel dibersihkan.
func excelizeRead(r io.Reader) ([][]string, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	f, err := excelize.OpenReader(bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sheet := f.GetSheetName(0)
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, err
	}
	out := [][]string{}
	for i, row := range rows {
		if i == 0 {
			continue // header
		}
		clean := make([]string, len(row))
		empty := true
		for j, c := range row {
			clean[j] = strings.TrimSpace(c)
			if clean[j] != "" {
				empty = false
			}
		}
		if !empty {
			out = append(out, clean)
		}
	}
	return out, nil
}

type GradeRow struct {
	NIS, Nama, Kelas, Mapel, Judul string
	Nilai                          int
	T                              time.Time
}

func gradesXLSX(data []GradeRow) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	sheet := "Nilai"
	f.SetSheetRow(sheet, "A1", &[]any{"NIS", "Nama", "Kelas", "Mapel", "Tugas", "Nilai", "Dikumpulkan"})
	for i, r := range data {
		f.SetSheetRow(sheet, cellRow(i+2), &[]any{r.NIS, r.Nama, r.Kelas, r.Mapel, r.Judul, r.Nilai, r.T.Format("2006-01-02 15:04")})
	}
	f.SetColWidth(sheet, "A", "G", 16)
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func cellRow(n int) string { return "A" + strconv.Itoa(n) }
