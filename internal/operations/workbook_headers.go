package operations

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/Pherlsz/Gymkhana-Database/internal/importcatalog"
)

type sheetLayout struct {
	Headers     []string
	ColumnCount int
	HeaderRow   int // 0 means headerless: every Excel row is data.
}

func workbookHeaders(sheet WorkbookSheet) ([]string, int, error) {
	layout, err := workbookLayout(sheet)
	if err != nil {
		return nil, 0, err
	}
	return layout.Headers, layout.ColumnCount, nil
}

func workbookLayout(sheet WorkbookSheet) (sheetLayout, error) {
	headerRow := detectHeaderRow(sheet)
	columnCount := usedColumnCount(sheet)
	if columnCount < 1 || columnCount > MaximumColumns {
		return sheetLayout{}, ErrUnsupportedWorkbook
	}
	headers := make([]string, columnCount)
	if headerRow > 0 {
		row := rowByNumber(sheet, headerRow)
		if row == nil {
			return sheetLayout{}, ErrUnsupportedWorkbook
		}
		for _, cell := range row.Cells {
			if cell.Column < 0 || cell.Column >= columnCount {
				continue
			}
			if cell.FormulaPresent {
				return sheetLayout{}, ErrUnsupportedWorkbook
			}
			headers[cell.Column] = strings.TrimSpace(cell.Value)
		}
	}
	for index, header := range headers {
		if header == "" {
			headers[index] = fmt.Sprintf("Coluna %d", index+1)
		}
		if len(headers[index]) > 500 {
			return sheetLayout{}, ErrUnsupportedWorkbook
		}
	}
	uniquifyHeaders(headers)
	return sheetLayout{Headers: headers, ColumnCount: columnCount, HeaderRow: headerRow}, nil
}

func detectHeaderRow(sheet WorkbookSheet) int {
	bestHits, bestRow := 0, 0
	for _, row := range sheet.Rows {
		if row.Number < 1 || row.Number > headerRowSlack {
			continue
		}
		hits := catalogHeaderHits(row)
		if hits >= 3 && hits > bestHits {
			bestHits, bestRow = hits, row.Number
		}
	}
	if bestRow > 0 {
		return bestRow
	}
	first, second, hasFirst, hasSecond := firstPopulatedRows(sheet, headerRowSlack)
	if !hasFirst {
		return 0
	}
	var secondValues []string
	if hasSecond {
		secondValues = rowValues(second)
	}
	if detectHasHeader(rowValues(first), secondValues) {
		return first.Number
	}
	return 0
}

func catalogHeaderHits(row WorkbookRow) int {
	hits := 0
	for _, cell := range row.Cells {
		suggestion := importcatalog.SuggestColumn(importcatalog.ModuleProfiles, cell.Value)
		if suggestion.TargetField != "" {
			hits++
		}
	}
	return hits
}

func firstPopulatedRows(sheet WorkbookSheet, limit int) (first, second WorkbookRow, hasFirst, hasSecond bool) {
	for _, row := range sheet.Rows {
		if row.Number < 1 || row.Number > limit {
			continue
		}
		if !rowHasText(row) {
			continue
		}
		if !hasFirst {
			first, hasFirst = row, true
			continue
		}
		second, hasSecond = row, true
		return
	}
	return
}

func rowByNumber(sheet WorkbookSheet, number int) *WorkbookRow {
	for index := range sheet.Rows {
		if sheet.Rows[index].Number == number {
			return &sheet.Rows[index]
		}
	}
	return nil
}

func rowHasText(row WorkbookRow) bool {
	for _, cell := range row.Cells {
		if strings.TrimSpace(cell.Value) != "" {
			return true
		}
	}
	return false
}

func rowValues(row WorkbookRow) []string {
	if len(row.Cells) == 0 {
		return nil
	}
	maximum := row.Cells[len(row.Cells)-1].Column
	values := make([]string, maximum+1)
	for _, cell := range row.Cells {
		if cell.Column >= 0 && cell.Column <= maximum {
			values[cell.Column] = cell.Value
		}
	}
	return values
}

func usedColumnCount(sheet WorkbookSheet) int {
	maximum := -1
	for _, row := range sheet.Rows {
		for _, cell := range row.Cells {
			if strings.TrimSpace(cell.Value) == "" && !cell.FormulaPresent {
				continue
			}
			if cell.Column > maximum {
				maximum = cell.Column
			}
		}
	}
	return maximum + 1
}

func uniquifyHeaders(headers []string) {
	seen := make(map[string]int, len(headers))
	for index, header := range headers {
		folded := strings.ToLower(header)
		count := seen[folded]
		seen[folded] = count + 1
		if count > 0 {
			headers[index] = fmt.Sprintf("%s_%d", header, count+1)
		}
	}
}

func isDataRow(number, headerRow int) bool {
	if headerRow > 0 {
		return number > headerRow
	}
	return number > 0
}

var brazilianUFs = map[string]struct{}{
	"AC": {}, "AL": {}, "AP": {}, "AM": {}, "BA": {}, "CE": {}, "DF": {}, "ES": {},
	"GO": {}, "MA": {}, "MT": {}, "MS": {}, "MG": {}, "PA": {}, "PB": {}, "PR": {},
	"PE": {}, "PI": {}, "RJ": {}, "RN": {}, "RS": {}, "RO": {}, "RR": {}, "SC": {},
	"SP": {}, "SE": {}, "TO": {},
}

func detectHasHeader(firstRow, secondRow []string) bool {
	nonEmpty := nonemptyCells(firstRow)
	if len(nonEmpty) == 0 {
		return true
	}
	dataLike := float64(countMatching(nonEmpty, cellLooksLikeData)) / float64(len(nonEmpty))
	headerLike := float64(countMatching(nonEmpty, cellLooksLikeHeader)) / float64(len(nonEmpty))
	secondNonEmpty := nonemptyCells(secondRow)
	if len(secondNonEmpty) > 0 {
		secondDataLike := float64(countMatching(secondNonEmpty, cellLooksLikeData)) / float64(len(secondNonEmpty))
		if dataLike >= 0.45 && secondDataLike >= dataLike {
			return false
		}
	}
	if headerLike >= 0.55 && dataLike < 0.35 {
		return true
	}
	return dataLike < 0.4
}

func nonemptyCells(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			result = append(result, value)
		}
	}
	return result
}

func countMatching(values []string, match func(string) bool) int {
	count := 0
	for _, value := range values {
		if match(value) {
			count++
		}
	}
	return count
}

func cellLooksLikeData(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	if looksLikeEmail(trimmed) {
		return true
	}
	digits := digitsOnly(trimmed)
	if len(digits) == 11 {
		return true
	}
	if len(digits) == 8 && trimmed[0] >= '0' && trimmed[0] <= '9' {
		return true
	}
	if looksLikeSlashDate(trimmed) {
		return true
	}
	if len(trimmed) == 1 {
		upper := strings.ToUpper(trimmed)
		if upper == "M" || upper == "F" {
			return true
		}
	}
	if _, ok := brazilianUFs[strings.ToUpper(trimmed)]; ok {
		return true
	}
	if len(strings.Fields(trimmed)) >= 2 && len(trimmed) >= 8 && hasLatinLetter(trimmed) {
		return true
	}
	return false
}

func cellLooksLikeHeader(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || len(trimmed) > 48 {
		return false
	}
	if cellLooksLikeData(trimmed) {
		return false
	}
	if looksLikePlainHeader(trimmed) && digitsOnly(trimmed) != trimmed {
		return true
	}
	return false
}

func looksLikePlainHeader(value string) bool {
	for _, character := range value {
		if unicode.IsLetter(character) || unicode.IsDigit(character) || character == '_' || character == '-' || character == ' ' {
			continue
		}
		return false
	}
	return true
}

func looksLikeEmail(value string) bool {
	at := strings.IndexByte(value, '@')
	if at <= 0 || at == len(value)-1 {
		return false
	}
	return strings.Contains(value[at+1:], ".") && !strings.ContainsAny(value, " ")
}

func looksLikeSlashDate(value string) bool {
	parts := strings.Split(value, "/")
	if len(parts) != 3 {
		return false
	}
	if len(parts[0]) < 1 || len(parts[0]) > 2 || len(parts[1]) < 1 || len(parts[1]) > 2 || len(parts[2]) != 4 {
		return false
	}
	return digitsOnly(parts[0]) == parts[0] && digitsOnly(parts[1]) == parts[1] && digitsOnly(parts[2]) == parts[2]
}

func hasLatinLetter(value string) bool {
	for _, character := range strings.ToLower(value) {
		if character >= 'a' && character <= 'z' {
			return true
		}
		switch character {
		case 'á', 'à', 'â', 'ã', 'é', 'ê', 'í', 'ó', 'ô', 'õ', 'ú', 'ç':
			return true
		}
	}
	return false
}

func digitsOnly(value string) string {
	var builder strings.Builder
	for _, character := range value {
		if character >= '0' && character <= '9' {
			builder.WriteRune(character)
		}
	}
	return builder.String()
}
