package operations

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestWorkbookRoundTripPreservesLiteralValuesAndNeverWritesFormulas(t *testing.T) {
	rows := [][]string{{"Ada", "00123456789", "=2+2"}, {"Bia", "0001", "+cmd"}, {"Caio", "0002", "-cmd"}, {"Dani", "0003", "@cmd"}}
	data, err := WriteWorkbook("Pessoas", []string{"full_name", "cpf", "note"}, rows)
	if err != nil {
		t.Fatalf("WriteWorkbook() error = %v", err)
	}
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip.NewReader() error = %v", err)
	}
	for _, file := range archive.File {
		if file.Name != "xl/worksheets/sheet1.xml" {
			continue
		}
		opened, openErr := file.Open()
		if openErr != nil {
			t.Fatal(openErr)
		}
		content, readErr := io.ReadAll(opened)
		_ = opened.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		if bytes.Contains(content, []byte("<f")) {
			t.Fatalf("worksheet contains an executable formula or lost literal text: %s", content)
		}
		for _, marker := range []string{"=2+2", "+cmd", "-cmd", "@cmd"} {
			if !bytes.Contains(content, []byte(marker)) {
				t.Fatalf("worksheet lost formula-like literal %q: %s", marker, content)
			}
		}
	}
	workbook, _, err := ReadWorkbook(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("ReadWorkbook() error = %v", err)
	}
	if len(workbook.Sheets) != 1 || len(workbook.Sheets[0].Rows) != len(rows)+1 {
		t.Fatalf("workbook = %#v", workbook)
	}
	if got := workbook.Sheets[0].Rows[1].Cells[1].Value; got != "00123456789" {
		t.Fatalf("leading zeros = %q", got)
	}
	repeated, err := WriteWorkbook("Pessoas", []string{"full_name", "cpf", "note"}, rows)
	if err != nil || !bytes.Equal(data, repeated) {
		t.Fatalf("workbook output is not deterministic: error=%v", err)
	}
}

func TestWorkbookWriterAllowsLargeFullTableExportsBeyondImportLimit(t *testing.T) {
	// Fixed size: must not scale with MaximumRows (100k would write ~100k XML rows).
	rows := make([][]string, 10_001)
	for index := range rows {
		rows[index] = []string{"row"}
	}
	data, err := WriteWorkbook("Pessoas", []string{"full_name"}, rows)
	if err != nil || len(data) == 0 {
		t.Fatalf("WriteWorkbook(large export) bytes=%d, error=%v", len(data), err)
	}
}

func TestWorkbookRejectsExternalRelationshipsAndLimits(t *testing.T) {
	data, err := WriteWorkbook("Safe", []string{"name"}, [][]string{{"Ada"}})
	if err != nil {
		t.Fatal(err)
	}
	unsafe := replaceZipEntry(t, data, "xl/_rels/workbook.xml.rels", strings.Replace(workbookRelationshipsXML, "</Relationships>", `<Relationship Id="external" Type="x" Target="https://invalid.example" TargetMode="External"/></Relationships>`, 1))
	if _, _, err := ReadWorkbook(bytes.NewReader(unsafe), int64(len(unsafe))); err != ErrUnsupportedWorkbook {
		t.Fatalf("external relationship error = %v", err)
	}
	if _, _, err := ReadWorkbook(bytes.NewReader(data), MaximumFileSize+1); err != ErrWorkbookLimit {
		t.Fatalf("oversized declaration error = %v", err)
	}
	bomb := replaceZipEntry(t, data, "xl/worksheets/sheet1.xml", strings.Repeat(" ", 2<<20))
	if _, _, err := ReadWorkbook(bytes.NewReader(bomb), int64(len(bomb))); err != ErrWorkbookLimit {
		t.Fatalf("compressed-ratio limit error = %v", err)
	}
}

func TestWorkbookPreservesCivilDatesAndDecimalText(t *testing.T) {
	data, err := WriteWorkbook("Contas", []string{"competence", "amount"}, [][]string{{"2026-07", "0012.30"}})
	if err != nil {
		t.Fatal(err)
	}
	styles := `<?xml version="1.0" encoding="UTF-8"?><styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><numFmts count="1"><numFmt numFmtId="164" formatCode="yyyy-mm-dd"/></numFmts><cellXfs count="2"><xf numFmtId="0"/><xf numFmtId="164"/></cellXfs></styleSheet>`
	data = replaceZipEntry(t, data, "xl/styles.xml", styles)
	sheet := `<?xml version="1.0" encoding="UTF-8"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>document_date</t></is></c><c r="B1" t="inlineStr"><is><t>amount</t></is></c></row><row r="2"><c r="A2" s="1"><v>45292</v></c><c r="B2" t="inlineStr"><is><t>0012.30</t></is></c></row></sheetData></worksheet>`
	data = replaceZipEntry(t, data, "xl/worksheets/sheet1.xml", sheet)
	workbook, _, err := ReadWorkbook(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("ReadWorkbook() error = %v", err)
	}
	date := workbook.Sheets[0].Rows[1].Cells[0]
	if date.Kind != ValueDate || date.Value != "2024-01-01" {
		t.Fatalf("date cell = %#v", date)
	}
	if amount := workbook.Sheets[0].Rows[1].Cells[1].Value; amount != "0012.30" {
		t.Fatalf("decimal text = %q", amount)
	}
}

func TestWorkbookRejectsMacroContentAndInvalidFormulaDate(t *testing.T) {
	data, err := WriteWorkbook("Safe", []string{"name"}, [][]string{{"Ada"}})
	if err != nil {
		t.Fatal(err)
	}
	macro := replaceZipEntry(t, data, "[Content_Types].xml", strings.Replace(contentTypesXML, "spreadsheetml.sheet.main+xml", "ms-excel.sheet.macroEnabled.main+xml", 1))
	if _, _, err := ReadWorkbook(bytes.NewReader(macro), int64(len(macro))); err != ErrUnsupportedWorkbook {
		t.Fatalf("macro content error = %v", err)
	}
	styles := `<?xml version="1.0"?><styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><cellXfs count="2"><xf numFmtId="0"/><xf numFmtId="14"/></cellXfs></styleSheet>`
	data = replaceZipEntry(t, data, "xl/styles.xml", styles)
	sheet := `<?xml version="1.0"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>date</t></is></c></row><row r="2"><c r="A2" s="1"><f>TODAY()</f><v>60</v></c></row></sheetData></worksheet>`
	data = replaceZipEntry(t, data, "xl/worksheets/sheet1.xml", sheet)
	if _, _, err := ReadWorkbook(bytes.NewReader(data), int64(len(data))); err != ErrUnsupportedWorkbook {
		t.Fatalf("invalid Excel leap date error = %v", err)
	}
}

func TestWorkbookRejectsMalformedXML(t *testing.T) {
	data, err := WriteWorkbook("Safe", []string{"name"}, [][]string{{"Ada"}})
	if err != nil {
		t.Fatal(err)
	}
	malformedSheet := replaceZipEntry(t, data, "xl/worksheets/sheet1.xml", `<worksheet><sheetData><row></worksheet>`)
	if _, _, err := ReadWorkbook(bytes.NewReader(malformedSheet), int64(len(malformedSheet))); err != ErrUnsupportedWorkbook {
		t.Fatalf("malformed worksheet error = %v", err)
	}
	malformedRelationships := replaceZipEntry(t, data, "xl/_rels/workbook.xml.rels", `<Relationships><Relationship></Relationships>`)
	if _, _, err := ReadWorkbook(bytes.NewReader(malformedRelationships), int64(len(malformedRelationships))); err != ErrUnsupportedWorkbook {
		t.Fatalf("malformed relationships error = %v", err)
	}
}

func TestWorkbookRejectsDuplicateAndEncryptedEntries(t *testing.T) {
	data, err := WriteWorkbook("Safe", []string{"name"}, [][]string{{"Ada"}})
	if err != nil {
		t.Fatal(err)
	}
	duplicate := appendZipEntry(t, data, "xl/workbook.xml", workbookDocumentXML("Other"))
	if _, _, err := ReadWorkbook(bytes.NewReader(duplicate), int64(len(duplicate))); err != ErrUnsupportedWorkbook {
		t.Fatalf("duplicate entry error = %v", err)
	}
	encrypted := append([]byte(nil), data...)
	centralHeader := bytes.Index(encrypted, []byte{'P', 'K', 1, 2})
	if centralHeader < 0 {
		t.Fatal("central ZIP header not found")
	}
	encrypted[centralHeader+8] |= 1
	if _, _, err := ReadWorkbook(bytes.NewReader(encrypted), int64(len(encrypted))); err != ErrUnsupportedWorkbook {
		t.Fatalf("encrypted entry error = %v", err)
	}
}

func replaceZipEntry(t *testing.T, input []byte, name, replacement string) []byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(input), int64(len(input)))
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for _, file := range reader.File {
		entry, createErr := writer.Create(file.Name)
		if createErr != nil {
			t.Fatal(createErr)
		}
		if file.Name == name {
			_, err = io.WriteString(entry, replacement)
		} else {
			opened, openErr := file.Open()
			if openErr != nil {
				t.Fatal(openErr)
			}
			_, err = io.Copy(entry, opened)
			_ = opened.Close()
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func appendZipEntry(t *testing.T, input []byte, name, content string) []byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(input), int64(len(input)))
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for _, file := range reader.File {
		entry, createErr := writer.Create(file.Name)
		if createErr != nil {
			t.Fatal(createErr)
		}
		opened, openErr := file.Open()
		if openErr != nil {
			t.Fatal(openErr)
		}
		_, err = io.Copy(entry, opened)
		_ = opened.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	entry, err := writer.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(entry, content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}
