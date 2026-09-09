#!/usr/bin/env python3
"""Generator xlsx minimal tanpa dependensi (zipfile + XML stdlib).
Membuat file xlsx berisi siswa/tugas sesuai kebutuhan smoke test."""
import sys, zipfile

def xlsx(path, rows, headers=None):
    if headers:
        rows = [headers] + rows
    # sheet XML
    rows_xml = []
    for r, row in enumerate(rows, 1):
        cells = []
        for c, val in enumerate(row, 1):
            col = chr(ord('A') + c - 1)
            cells.append(f'<c r="{col}{r}" t="inlineStr"><is><t>{val}</t></is></c>')
        rows_xml.append(f'<row r="{r}">{"".join(cells)}</row>')
    sheet = ('<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
             '<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">'
             '<sheetData>' + ''.join(rows_xml) + '</sheetData></worksheet>')
    content_types = ('<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
        '<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">'
        '<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>'
        '<Default Extension="xml" ContentType="application/xml"/>'
        '<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>'
        '<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>'
        '</Types>')
    rels = ('<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
        '<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">'
        '<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>'
        '</Relationships>')
    workbook = ('<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
        '<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" '
        'xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">'
        '<sheets><sheet name="Sheet1" sheetId="1" r:id="rId1"/></sheets></workbook>')
    wbrels = ('<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
        '<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">'
        '<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>'
        '</Relationships>')
    with zipfile.ZipFile(path, 'w', zipfile.ZIP_DEFLATED) as z:
        z.writestr('[Content_Types].xml', content_types)
        z.writestr('_rels/.rels', rels)
        z.writestr('xl/workbook.xml', workbook)
        z.writestr('xl/_rels/workbook.xml.rels', wbrels)
        z.writestr('xl/worksheets/sheet1.xml', sheet)
    print(f'wrote {path} ({len(rows)} rows)')

if __name__ == '__main__':
    out = sys.argv[1]
    mode = 'mksoal' if 'soal' in out else 'mksiswa'
    if mode == 'mksiswa':
        xlsx(out, [
            ['NIS', 'Nama', 'Kelas', 'JK'],
            ['24001', 'Andi Saputra', 'X TKJ 1', 'L'],
            ['24002', 'Bunga Lestari', 'X TKJ 1', 'P'],
            ['24003', 'Citra Wulandari', 'X TKJ 1', 'P'],
        ])
    elif mode == 'mksoal':
        xlsx(out, [
            ['Soal', 'OpsiA', 'OpsiB', 'OpsiC', 'OpsiD', 'OpsiE', 'Kunci', 'Bobot'],
            ['2 + 2 = ?', '3', '4', '5', '6', '', '2', '1'],
            ['Ibukota Indonesia?', 'Surabaya', 'Bandung', 'Jakarta', 'Medan', '', '3', '1'],
        ])
    else:
        raise SystemExit('mode tidak dikenal')