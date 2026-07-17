package operations

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maximumZIPEntries       = 2_000
	maximumUncompressedSize = int64(128 << 20)
	maximumSharedStringSize = int64(32 << 20)
)

type Workbook struct {
	Sheets []WorkbookSheet
}

type WorkbookSheet struct {
	Index int
	Name  string
	Rows  []WorkbookRow
}

type WorkbookRow struct {
	Number int
	Cells  []WorkbookCell
}

type WorkbookCell struct {
	Column         int
	Value          string
	Kind           ValueKind
	FormulaPresent bool
}

type workbookXML struct {
	Properties workbookPropertiesXML `xml:"workbookPr"`
	Sheets     []workbookSheetXML    `xml:"sheets>sheet"`
}

type workbookPropertiesXML struct {
	Date1904 bool `xml:"date1904,attr"`
}

type workbookSheetXML struct {
	Name string `xml:"name,attr"`
	RID  string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
}

type relationshipsXML struct {
	Relationships []relationshipXML `xml:"Relationship"`
}

type relationshipXML struct {
	ID         string `xml:"Id,attr"`
	Target     string `xml:"Target,attr"`
	TargetMode string `xml:"TargetMode,attr"`
}

type sharedStringsXML struct {
	Items []sharedStringXML `xml:"si"`
}

type sharedStringXML struct {
	Text string               `xml:"t"`
	Runs []sharedStringRunXML `xml:"r"`
}

type sharedStringRunXML struct {
	Text string `xml:"t"`
}

type worksheetXML struct {
	Rows []worksheetRowXML `xml:"sheetData>row"`
}

type worksheetRowXML struct {
	Number int                `xml:"r,attr"`
	Cells  []worksheetCellXML `xml:"c"`
}

type worksheetCellXML struct {
	Reference string             `xml:"r,attr"`
	Type      string             `xml:"t,attr"`
	Style     int                `xml:"s,attr"`
	Value     string             `xml:"v"`
	Formula   *string            `xml:"f"`
	Inline    worksheetInlineXML `xml:"is"`
}

type stylesXMLDocument struct {
	NumberFormats []numberFormatXML `xml:"numFmts>numFmt"`
	CellFormats   []cellFormatXML   `xml:"cellXfs>xf"`
}

type numberFormatXML struct {
	ID   int    `xml:"numFmtId,attr"`
	Code string `xml:"formatCode,attr"`
}

type cellFormatXML struct {
	NumberFormatID int `xml:"numFmtId,attr"`
}

type worksheetInlineXML struct {
	Text string               `xml:"t"`
	Runs []sharedStringRunXML `xml:"r"`
}

func ReadWorkbook(reader io.Reader, declaredSize int64) (Workbook, []byte, error) {
	if declaredSize <= 0 || declaredSize > MaximumFileSize {
		return Workbook{}, nil, ErrWorkbookLimit
	}
	data, err := io.ReadAll(io.LimitReader(reader, MaximumFileSize+1))
	if err != nil {
		return Workbook{}, nil, fmt.Errorf("read XLSX object: %w", err)
	}
	if int64(len(data)) != declaredSize || int64(len(data)) > MaximumFileSize {
		return Workbook{}, nil, ErrWorkbookLimit
	}
	workbook, err := parseWorkbook(data)
	if err != nil {
		return Workbook{}, nil, err
	}
	return workbook, data, nil
}

func parseWorkbook(data []byte) (Workbook, error) {
	if len(data) < 4 || data[0] != 'P' || data[1] != 'K' {
		return Workbook{}, ErrUnsupportedWorkbook
	}
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return Workbook{}, ErrUnsupportedWorkbook
	}
	if len(archive.File) == 0 || len(archive.File) > maximumZIPEntries {
		return Workbook{}, ErrWorkbookLimit
	}
	files := make(map[string][]byte, len(archive.File))
	seenEntries := make(map[string]struct{}, len(archive.File))
	var uncompressed int64
	var loaded int64
	for _, file := range archive.File {
		name := strings.TrimPrefix(path.Clean("/"+file.Name), "/")
		if name != file.Name || strings.Contains(name, "\\") || file.Flags&1 != 0 {
			return Workbook{}, ErrUnsupportedWorkbook
		}
		foldedName := strings.ToLower(name)
		if _, duplicate := seenEntries[foldedName]; duplicate {
			return Workbook{}, ErrUnsupportedWorkbook
		}
		seenEntries[foldedName] = struct{}{}
		uncompressed += int64(file.UncompressedSize64)
		if uncompressed > maximumUncompressedSize || file.UncompressedSize64 > uint64(maximumUncompressedSize) {
			return Workbook{}, ErrWorkbookLimit
		}
		if file.CompressedSize64 > 0 && file.UncompressedSize64/file.CompressedSize64 > 200 {
			return Workbook{}, ErrWorkbookLimit
		}
		lowerName := foldedName
		if strings.Contains(lowerName, "vbaproject") || strings.HasPrefix(lowerName, "xl/externallinks/") {
			return Workbook{}, ErrUnsupportedWorkbook
		}
		if !neededWorkbookFile(name) {
			continue
		}
		opened, openErr := file.Open()
		if openErr != nil {
			return Workbook{}, ErrUnsupportedWorkbook
		}
		content, readErr := io.ReadAll(io.LimitReader(opened, maximumUncompressedSize+1))
		closeErr := opened.Close()
		if readErr != nil || closeErr != nil || int64(len(content)) > maximumUncompressedSize {
			return Workbook{}, ErrUnsupportedWorkbook
		}
		if name == "xl/sharedStrings.xml" && int64(len(content)) > maximumSharedStringSize {
			return Workbook{}, ErrWorkbookLimit
		}
		loaded += int64(len(content))
		if loaded > maximumUncompressedSize {
			return Workbook{}, ErrWorkbookLimit
		}
		files[name] = content
	}
	if contentTypes := strings.ToLower(string(files["[Content_Types].xml"])); strings.Contains(contentTypes, "macroenabled") || strings.Contains(contentTypes, "vbaproject") {
		return Workbook{}, ErrUnsupportedWorkbook
	}
	for name, content := range files {
		if strings.HasSuffix(strings.ToLower(name), ".rels") {
			var relationships relationshipsXML
			if err := decodeXML(content, &relationships); err != nil {
				return Workbook{}, ErrUnsupportedWorkbook
			}
			for _, relationship := range relationships.Relationships {
				if strings.EqualFold(relationship.TargetMode, "External") {
					return Workbook{}, ErrUnsupportedWorkbook
				}
			}
		}
	}
	return workbookFromFiles(files)
}

func neededWorkbookFile(name string) bool {
	lower := strings.ToLower(name)
	return name == "[Content_Types].xml" || name == "xl/workbook.xml" || name == "xl/_rels/workbook.xml.rels" || name == "xl/sharedStrings.xml" || name == "xl/styles.xml" || strings.HasPrefix(lower, "xl/worksheets/") || strings.HasSuffix(lower, ".rels")
}

func workbookFromFiles(files map[string][]byte) (Workbook, error) {
	var book workbookXML
	if err := decodeXML(files["xl/workbook.xml"], &book); err != nil || len(book.Sheets) == 0 || len(book.Sheets) > 100 {
		return Workbook{}, ErrUnsupportedWorkbook
	}
	var relationships relationshipsXML
	if err := decodeXML(files["xl/_rels/workbook.xml.rels"], &relationships); err != nil {
		return Workbook{}, ErrUnsupportedWorkbook
	}
	targets := make(map[string]string, len(relationships.Relationships))
	for _, relationship := range relationships.Relationships {
		target := strings.TrimPrefix(path.Clean("/xl/"+relationship.Target), "/")
		if !strings.HasPrefix(target, "xl/worksheets/") {
			continue
		}
		targets[relationship.ID] = target
	}
	shared, err := parseSharedStrings(files["xl/sharedStrings.xml"])
	if err != nil {
		return Workbook{}, err
	}
	dateStyles, err := parseDateStyles(files["xl/styles.xml"])
	if err != nil {
		return Workbook{}, err
	}
	workbook := Workbook{Sheets: make([]WorkbookSheet, 0, len(book.Sheets))}
	seenNames := make(map[string]struct{}, len(book.Sheets))
	totalRows, totalCells := 0, 0
	for index, sheet := range book.Sheets {
		name := strings.TrimSpace(strings.ToValidUTF8(sheet.Name, ""))
		if name == "" || utf8.RuneCountInString(name) > 120 {
			return Workbook{}, ErrUnsupportedWorkbook
		}
		folded := strings.ToLower(name)
		if _, exists := seenNames[folded]; exists {
			return Workbook{}, ErrUnsupportedWorkbook
		}
		seenNames[folded] = struct{}{}
		target, ok := targets[sheet.RID]
		if !ok {
			return Workbook{}, ErrUnsupportedWorkbook
		}
		parsed, parseErr := parseWorksheet(index, name, files[target], shared, dateStyles, book.Properties.Date1904, &totalRows, &totalCells)
		if parseErr != nil {
			return Workbook{}, parseErr
		}
		workbook.Sheets = append(workbook.Sheets, parsed)
	}
	return workbook, nil
}

func parseDateStyles(content []byte) ([]bool, error) {
	if len(content) == 0 {
		return []bool{false}, nil
	}
	var styles stylesXMLDocument
	if err := decodeXML(content, &styles); err != nil || len(styles.CellFormats) > 65_536 || len(styles.NumberFormats) > 65_536 {
		return nil, ErrUnsupportedWorkbook
	}
	custom := make(map[int]string, len(styles.NumberFormats))
	for _, format := range styles.NumberFormats {
		if format.ID < 0 || format.ID > 65_535 || len(format.Code) > 4_096 {
			return nil, ErrWorkbookLimit
		}
		custom[format.ID] = format.Code
	}
	result := make([]bool, len(styles.CellFormats))
	for index, format := range styles.CellFormats {
		result[index] = builtInDateFormat(format.NumberFormatID) || customDateFormat(custom[format.NumberFormatID])
	}
	if len(result) == 0 {
		return []bool{false}, nil
	}
	return result, nil
}

func builtInDateFormat(id int) bool {
	return (id >= 14 && id <= 22) || (id >= 27 && id <= 36) || (id >= 45 && id <= 47) || (id >= 50 && id <= 58)
}

func customDateFormat(code string) bool {
	code = strings.ToLower(code)
	var normalized strings.Builder
	inQuote, inBracket, escaped := false, false, false
	for _, character := range code {
		if escaped {
			escaped = false
			continue
		}
		if character == '\\' || character == '_' || character == '*' {
			escaped = true
			continue
		}
		if character == '"' && !inBracket {
			inQuote = !inQuote
			continue
		}
		if character == '[' && !inQuote {
			inBracket = true
			continue
		}
		if character == ']' && inBracket {
			inBracket = false
			continue
		}
		if !inQuote && !inBracket {
			normalized.WriteRune(character)
		}
	}
	value := normalized.String()
	return strings.ContainsRune(value, 'y') || strings.ContainsRune(value, 'd')
}

func parseSharedStrings(content []byte) ([]string, error) {
	if len(content) == 0 {
		return nil, nil
	}
	var values sharedStringsXML
	if err := decodeXML(content, &values); err != nil {
		return nil, ErrUnsupportedWorkbook
	}
	if len(values.Items) > MaximumCells {
		return nil, ErrWorkbookLimit
	}
	result := make([]string, 0, len(values.Items))
	for _, item := range values.Items {
		value := item.Text
		for _, run := range item.Runs {
			value += run.Text
		}
		if len(value) > MaximumCellBytes || !utf8.ValidString(value) {
			return nil, ErrWorkbookLimit
		}
		result = append(result, value)
	}
	return result, nil
}

func parseWorksheet(index int, name string, content []byte, shared []string, dateStyles []bool, date1904 bool, totalRows, totalCells *int) (WorkbookSheet, error) {
	var worksheet worksheetXML
	if err := decodeXML(content, &worksheet); err != nil {
		return WorkbookSheet{}, ErrUnsupportedWorkbook
	}
	if len(worksheet.Rows) > MaximumRows+1 {
		return WorkbookSheet{}, ErrWorkbookLimit
	}
	result := WorkbookSheet{Index: index, Name: name, Rows: make([]WorkbookRow, 0, len(worksheet.Rows))}
	lastRow := 0
	for rowOffset, row := range worksheet.Rows {
		rowNumber := row.Number
		if rowNumber == 0 {
			rowNumber = rowOffset + 1
		}
		if rowNumber <= lastRow || rowNumber > MaximumRows+1 || len(row.Cells) > MaximumColumns {
			return WorkbookSheet{}, ErrWorkbookLimit
		}
		lastRow = rowNumber
		if rowNumber > 1 {
			*totalRows = *totalRows + 1
			if *totalRows > MaximumRows {
				return WorkbookSheet{}, ErrWorkbookLimit
			}
		}
		mapped := WorkbookRow{Number: rowNumber, Cells: make([]WorkbookCell, 0, len(row.Cells))}
		seenColumns := make(map[int]struct{}, len(row.Cells))
		for cellOffset, cell := range row.Cells {
			column, refRow, ok := parseCellReference(cell.Reference)
			if cell.Reference == "" {
				column, refRow, ok = cellOffset, rowNumber, true
			}
			if !ok || refRow != rowNumber || column >= MaximumColumns {
				return WorkbookSheet{}, ErrWorkbookLimit
			}
			if _, exists := seenColumns[column]; exists {
				return WorkbookSheet{}, ErrUnsupportedWorkbook
			}
			seenColumns[column] = struct{}{}
			value, kind, valueErr := workbookCellValue(cell, shared, dateStyles, date1904)
			if valueErr != nil {
				return WorkbookSheet{}, valueErr
			}
			mapped.Cells = append(mapped.Cells, WorkbookCell{Column: column, Value: value, Kind: kind, FormulaPresent: cell.Formula != nil})
			*totalCells = *totalCells + 1
			if *totalCells > MaximumCells {
				return WorkbookSheet{}, ErrWorkbookLimit
			}
		}
		sort.Slice(mapped.Cells, func(left, right int) bool { return mapped.Cells[left].Column < mapped.Cells[right].Column })
		result.Rows = append(result.Rows, mapped)
	}
	return result, nil
}

func workbookCellValue(cell worksheetCellXML, shared []string, dateStyles []bool, date1904 bool) (string, ValueKind, error) {
	value := cell.Value
	kind := ValueNumber
	switch cell.Type {
	case "", "n":
		if value == "" {
			kind = ValueEmpty
		} else if cell.Style < 0 || cell.Style >= len(dateStyles) {
			return "", "", ErrUnsupportedWorkbook
		} else if dateStyles[cell.Style] {
			civil, err := excelCivilDate(value, date1904)
			if err != nil {
				return "", "", err
			}
			value, kind = civil, ValueDate
		}
	case "s":
		index, err := strconv.Atoi(value)
		if err != nil || index < 0 || index >= len(shared) {
			return "", "", ErrUnsupportedWorkbook
		}
		value, kind = shared[index], ValueText
	case "inlineStr":
		value = cell.Inline.Text
		for _, run := range cell.Inline.Runs {
			value += run.Text
		}
		kind = ValueText
	case "str", "e":
		kind = ValueText
	case "b":
		if value == "1" {
			value = "true"
		} else if value == "0" {
			value = "false"
		} else {
			return "", "", ErrUnsupportedWorkbook
		}
		kind = ValueBoolean
	case "d":
		kind = ValueDate
	default:
		return "", "", ErrUnsupportedWorkbook
	}
	value = strings.ToValidUTF8(value, "")
	if len(value) > MaximumCellBytes {
		return "", "", ErrWorkbookLimit
	}
	return value, kind, nil
}

func excelCivilDate(value string, date1904 bool) (string, error) {
	serial, err := strconv.ParseFloat(value, 64)
	if err != nil || serial < 0 || serial > 2_958_465 {
		return "", ErrUnsupportedWorkbook
	}
	days := int64(serial)
	var date time.Time
	if date1904 {
		date = time.Date(1904, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, int(days))
	} else {
		if days == 60 {
			return "", ErrUnsupportedWorkbook
		}
		if days > 60 {
			days--
		}
		date = time.Date(1899, 12, 31, 0, 0, 0, 0, time.UTC).AddDate(0, 0, int(days))
	}
	return date.Format("2006-01-02"), nil
}

func parseCellReference(reference string) (int, int, bool) {
	lettersEnd := 0
	for lettersEnd < len(reference) && reference[lettersEnd] >= 'A' && reference[lettersEnd] <= 'Z' {
		lettersEnd++
	}
	if lettersEnd == 0 || lettersEnd == len(reference) {
		return 0, 0, false
	}
	column := 0
	for _, character := range reference[:lettersEnd] {
		column = column*26 + int(character-'A'+1)
	}
	row, err := strconv.Atoi(reference[lettersEnd:])
	return column - 1, row, err == nil && row > 0
}

func decodeXML(content []byte, target any) error {
	if len(content) == 0 {
		return ErrUnsupportedWorkbook
	}
	decoder := xml.NewDecoder(bytes.NewReader(content))
	decoder.Strict = true
	return decoder.Decode(target)
}

func WriteWorkbook(sheetName string, headers []string, rows [][]string) ([]byte, error) {
	if strings.TrimSpace(sheetName) == "" || len(headers) == 0 || len(headers) > MaximumColumns || len(rows) > MaximumExportRows {
		return nil, ErrInvalidInput
	}
	seenHeaders := make(map[string]struct{}, len(headers))
	for _, header := range headers {
		normalized := strings.ToLower(strings.TrimSpace(header))
		if normalized == "" || len(header) > MaximumCellBytes {
			return nil, ErrInvalidInput
		}
		if _, duplicate := seenHeaders[normalized]; duplicate {
			return nil, ErrInvalidInput
		}
		seenHeaders[normalized] = struct{}{}
	}
	for _, row := range rows {
		if len(row) != len(headers) {
			return nil, ErrInvalidInput
		}
		for _, value := range row {
			if len(value) > MaximumCellBytes {
				return nil, ErrInvalidInput
			}
		}
	}
	var output bytes.Buffer
	archive := zip.NewWriter(&output)
	files := []struct {
		name    string
		content string
	}{
		{"[Content_Types].xml", contentTypesXML},
		{"_rels/.rels", rootRelationshipsXML},
		{"xl/workbook.xml", workbookDocumentXML(sheetName)},
		{"xl/_rels/workbook.xml.rels", workbookRelationshipsXML},
		{"xl/styles.xml", stylesXML},
		{"xl/worksheets/sheet1.xml", worksheetDocumentXML(headers, rows)},
	}
	for _, file := range files {
		header := &zip.FileHeader{
			Name: file.name, Method: zip.Deflate,
			Modified: time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC),
		}
		writer, err := archive.CreateHeader(header)
		if err != nil {
			return nil, fmt.Errorf("create XLSX entry: %w", err)
		}
		if _, err := io.WriteString(writer, file.content); err != nil {
			return nil, fmt.Errorf("write XLSX entry: %w", err)
		}
	}
	if err := archive.Close(); err != nil {
		return nil, fmt.Errorf("close XLSX archive: %w", err)
	}
	return output.Bytes(), nil
}

func workbookDocumentXML(sheetName string) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="` + escapeXML(sheetName) + `" sheetId="1" r:id="rId1"/></sheets></workbook>`
}

func worksheetDocumentXML(headers []string, rows [][]string) string {
	var content strings.Builder
	content.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	writeWorksheetRow(&content, 1, headers)
	for index, row := range rows {
		writeWorksheetRow(&content, index+2, row)
	}
	content.WriteString(`</sheetData></worksheet>`)
	return content.String()
}

func writeWorksheetRow(content *strings.Builder, rowNumber int, values []string) {
	content.WriteString(`<row r="`)
	content.WriteString(strconv.Itoa(rowNumber))
	content.WriteString(`">`)
	for column, value := range values {
		content.WriteString(`<c r="`)
		content.WriteString(columnName(column))
		content.WriteString(strconv.Itoa(rowNumber))
		content.WriteString(`" t="inlineStr"><is><t xml:space="preserve">`)
		content.WriteString(escapeXML(strings.ToValidUTF8(value, "")))
		content.WriteString(`</t></is></c>`)
	}
	content.WriteString(`</row>`)
}

func columnName(column int) string {
	column++
	result := ""
	for column > 0 {
		column--
		result = string(rune('A'+column%26)) + result
		column /= 26
	}
	return result
}

func escapeXML(value string) string {
	var escaped bytes.Buffer
	_ = xml.EscapeText(&escaped, []byte(value))
	return escaped.String()
}

const contentTypesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/><Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/></Types>`
const rootRelationshipsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`
const workbookRelationshipsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`
const stylesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><fonts count="1"><font><sz val="11"/><name val="Calibri"/></font></fonts><fills count="1"><fill><patternFill patternType="none"/></fill></fills><borders count="1"><border/></borders><cellStyleXfs count="1"><xf/></cellStyleXfs><cellXfs count="1"><xf xfId="0"/></cellXfs></styleSheet>`
