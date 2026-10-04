package main

import (
	"encoding/json"
	"testing"
)

func TestNormalizeTypoSoul(t *testing.T) {
	raw := json.RawMessage(`{"questions":[
		{"soul":"Apa itu NAT?","opsi":["A","B","C","D"],"kunci":"B","pembahasan":"ok","extra_field":123},
		{"soal":"Apa itu MCP?","opsi":["A","B","C","D"],"kunci":2}
	]}`)
	qs, err := validateQuizQuestions(raw)
	if err != nil {
		t.Fatalf("seharusnya toleran, dapat error: %v", err)
	}
	if len(qs) != 2 {
		t.Fatalf("dapat %d soal, mau 2", len(qs))
	}
	if qs[0].Soal != "Apa itu NAT?" || qs[0].Kunci != 1 {
		t.Fatalf("normalisasi soal 1 gagal: %+v", qs[0])
	}
	if qs[1].Kunci != 2 {
		t.Fatalf("kunci soal 2 berubah: %+v", qs[1])
	}
}
