package main

import "testing"

// Perbandingan kelas harus toleran terhadap perbedaan spasi & kapital antara
// nama rombel (dari tabel rombel) dan kelas siswa (dari impor/input) — akar
// bug "tugas per-rombel tidak muncul di siswa".
func TestNormalizeKelas(t *testing.T) {
	cases := []struct{ in, want string }{
		{"X TKJ 4", "x tkj 4"},
		{"  X TKJ 4  ", "x tkj 4"},
		{"X  TKJ   4", "x tkj 4"},
		{"x tkj 4", "x tkj 4"},
		{"", ""},
		{"   ", ""},
	}
	for _, c := range cases {
		if got := normalizeKelas(c.in); got != c.want {
			t.Errorf("normalizeKelas(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	// Varian penulisan dari dua sumber berbeda harus dianggap sama.
	if normalizeKelas(" X  TKJ 4 ") != normalizeKelas("x tkj 4") {
		t.Error("varian spasi/kapital harus setara setelah normalizeKelas")
	}
}

func TestRapikanKelas(t *testing.T) {
	if got := rapikanKelas("  X  TKJ   4 "); got != "X TKJ 4" {
		t.Errorf("rapikanKelas = %q, want %q (kapital dipertahankan)", got, "X TKJ 4")
	}
	if got := rapikanKelas(""); got != "" {
		t.Errorf("rapikanKelas(\"\") = %q, want \"\"", got)
	}
}
