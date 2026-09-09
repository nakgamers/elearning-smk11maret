#!/usr/bin/env bash
# Smoke test end-to-end elearning API. Jalankan: bash smoke.sh
set -e
BASE=http://127.0.0.1:8081/api
pass() { echo "PASS: $1"; }
fail() { echo "FAIL: $1"; exit 1; }
chk() { if [ "$1" = "$2" ]; then pass "$3"; else fail "$3 (dapat: $1, harap: $2)"; fi; }

echo "== 1. health =="
chk "$(curl -s $BASE/health)" '{"ok":true}' "health"

echo "== 2. login admin =="
R=$(curl -s -X POST $BASE/login -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123"}')
echo "$R" | head -c 200; echo
TOK=$(echo "$R" | python -c "import sys,json;print(json.load(sys.stdin)['token'])")
[ -n "$TOK" ] && pass "token admin" || fail "token admin"
AUTH="Authorization: Bearer $TOK"

echo "== 3. subjects & CP =="
curl -s $BASE/subjects -H "$AUTH" | python -c "import sys,json;d=json.load(sys.stdin);print(len(d),'subjects')"
curl -s $BASE/cps -H "$AUTH" | python -c "import sys,json;d=json.load(sys.stdin);print(len(d),'CPs')"

echo "== 4. import siswa (xlsx) =="
mkdir -p tmp
python mksiswa.py ./tmp/siswa.xlsx
R=$(curl -s -X POST $BASE/students/import -H "$AUTH" -F "file=@./tmp/siswa.xlsx")
echo "$R"
echo "$R" | grep -q '"imported":3' && pass "import 3 siswa" || fail "import siswa"

echo "== 5. login siswa =="
R=$(curl -s -X POST $BASE/login -H 'Content-Type: application/json' -d '{"username":"24001","password":"siswa123"}')
STOK=$(echo "$R" | python -c "import sys,json;print(json.load(sys.stdin)['token'])")
[ -n "$STOK" ] && pass "login siswa NIS" || fail "login siswa"
SAUTH="Authorization: Bearer $STOK"

echo "== 6. guru buat materi =="
GTOK=$(curl -s -X POST $BASE/login -H 'Content-Type: application/json' -d '{"username":"guru.mtk","password":"admin123"}' | python -c "import sys,json;print(json.load(sys.stdin)['token'])")
GAUTH="Authorization: Bearer $GTOK"
R=$(curl -s -X POST $BASE/materials -H "$GAUTH" -F "mapel_id=1" -F "cp_id=1" -F "judul=Materi Barisan Bilangan" -F "isi=Pola bilangan..." -F "kelas=X TKJ 1")
echo "$R" | grep -q '"id"' && pass "materi dibuat" || fail "materi"

echo "== 7. siswa lihat materi (filter kelas) =="
R=$(curl -s $BASE/my/materials -H "$SAUTH")
echo "$R" | python -c "import sys,json;d=json.load(sys.stdin);assert len(d)>=1 and d[0]['guru']=='Guru Matematika';print('materi utk siswa:',len(d))"
pass "materi siswa + nama guru tampil"

echo "== 8. guru buat tugas + siswa submit =="
R=$(curl -s -X POST $BASE/assignments -H "$GAUTH" -F "mapel_id=1" -F "cp_id=1" -F "judul=Tugas Deret" -F "kelas=X TKJ 1" -F "max_score=100")
AID=$(echo "$R" | python -c "import sys,json;print(json.load(sys.stdin)['id'])")
R=$(curl -s -X POST $BASE/submissions -H "$SAUTH" -F "assignment_id=$AID" -F "jawaban=Jawaban saya")
echo "$R" | grep -q '"ok":true' && pass "submit tugas" || fail "submit tugas"
# double-submit = idempotent, bukan error
R=$(curl -s -X POST $BASE/submissions -H "$SAUTH" -F "assignment_id=$AID" -F "jawaban=Jawaban revisi")
echo "$R" | grep -q '"ok":true' && pass "resubmit idempotent" || fail "resubmit"

echo "== 9. guru nilai + export =="
R=$(curl -s "$BASE/assignments/$AID/submissions" -H "$GAUTH")
SID=$(echo "$R" | python -c "import sys,json;print(json.load(sys.stdin)[0]['id'])")
curl -s -X POST "$BASE/submissions/$SID/grade" -H "$GAUTH" -H 'Content-Type: application/json' -d '{"nilai":90,"feedback":"Bagus"}' | grep -q ok && pass "grade"
curl -s "$BASE/grades/export" -H "$AUTH" -o ./tmp/nilai.xlsx
python -c "
import zipfile
z=zipfile.ZipFile('./tmp/nilai.xlsx')
assert 'xl/worksheets/sheet1.xml' in z.namelist()
print('export xlsx valid')"
pass "export nilai xlsx"

echo "== 10. absen siswa checkin =="
curl -s -X POST $BASE/attendance/checkin -H "$SAUTH" -H 'Content-Type: application/json' -d '{}' | grep -q ok && pass "absen mandiri"
curl -s "$BASE/attendance?tanggal=$(date +%F)" -H "$AUTH" | grep -q '"nama"' && pass "rekap absen guru"

echo "== 11. news =="
curl -s -X POST $BASE/news -H "$AUTH" -H 'Content-Type: application/json' -d '{"judul":"Ujian pekan depan","isi":"Persiapkan diri."}' | grep -q id && pass "news dibuat"
curl -s $BASE/news | python -c "import sys,json;d=json.load(sys.stdin);assert len(d)>=2;print(len(d),'news publik')"

echo "== 12. ujian: buat, import soal, aktivasi(preload cache), kerjakan =="
R=$(curl -s -X POST $BASE/exams -H "$GAUTH" -H 'Content-Type: application/json' -d '{"nama":"UH Barisan","mapel_id":1,"cp_id":1,"kelas":"X TKJ 1","durasi_menit":60}')
EID=$(echo "$R" | python -c "import sys,json;print(json.load(sys.stdin)['id'])")
python mksiswa.py ./tmp/soal.xlsx
R=$(curl -s -X POST "$BASE/exams/$EID/questions/import" -H "$GAUTH" -F "file=@./tmp/soal.xlsx")
echo "$R" | grep -q '"imported":2' && pass "import 2 soal" || fail "import soal ($R)"
curl -s -X POST "$BASE/exams/$EID/activate" -H "$GAUTH" | grep -q '"cached":2' && pass "aktivasi + preload cache"
R=$(curl -s -X POST "$BASE/exams/$EID/start" -H "$SAUTH")
ATT=$(echo "$R" | python -c "import sys,json;print(json.load(sys.stdin)['attempt_id'])")
Q1=$(echo "$R" | python -c "import sys,json;print(json.load(sys.stdin)['questions'][0]['id'])")
Q2=$(echo "$R" | python -c "import sys,json;print(json.load(sys.stdin)['questions'][1]['id'])")
# sync jawaban batch (Opsi A master plan) — Q1 benar=opsi idx1 ("4"), Q2 benar=idx2 ("Jakarta")
curl -s -X POST "$BASE/attempts/$ATT/answers" -H "$SAUTH" -H 'Content-Type: application/json' -d "{\"answers\":[{\"question_id\":$Q1,\"jawaban\":1},{\"question_id\":$Q2,\"jawaban\":2}]}" | grep -q '"n":2' && pass "sync jawaban batch"
# kirim ulang payload sama = idempotent
curl -s -X POST "$BASE/attempts/$ATT/answers" -H "$SAUTH" -H 'Content-Type: application/json' -d "{\"answers\":[{\"question_id\":$Q1,\"jawaban\":1},{\"question_id\":$Q2,\"jawaban\":2}]}" | grep -q '"n":2' && pass "sync idempotent"
R=$(curl -s -X POST "$BASE/attempts/$ATT/finish" -H "$SAUTH")
SKOR=$(echo "$R" | python -c "import sys,json;print(json.load(sys.stdin)['skor'])")
chk "$SKOR" "100" "skor 100 (2/2 benar)"
# finish kedua = ditolak (status sudah selesai)
CODE=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE/attempts/$ATT/finish" -H "$SAUTH")
chk "$CODE" "403" "finish ulang ditolak"

echo "== 13. cheat signal =="
curl -s -X POST $BASE/cheat-signals -H "$SAUTH" -H 'Content-Type: application/json' -d "{\"exam_id\":$EID,\"jenis\":\"onPause\"}" | grep -q ok && pass "cheat signal"

echo; echo "SEMUA SMOKE TEST PASS ✔"
