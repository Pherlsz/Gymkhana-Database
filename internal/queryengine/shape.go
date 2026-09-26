package queryengine

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// ShapeRow is one scanned record. Fields are catalog keys and derived names.
type ShapeRow struct {
	EntityID   string
	EntityKind string
	Label      string
	Fields     map[string]string
}

type ShapeColumn struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

type ShapeGridRow struct {
	ID         string            `json:"id"`
	EntityKind string            `json:"entity_kind"`
	EntityID   string            `json:"entity_id"`
	Label      string            `json:"entity_label"`
	Cells      map[string]string `json:"cells"`
}

// ShapePage is the in-memory result of derive, match, and sequence.
type ShapePage struct {
	Columns   []ShapeColumn  `json:"columns"`
	Rows      []ShapeGridRow `json:"rows"`
	Total     int            `json:"total"`
	Summary   string         `json:"summary"`
	Truncated bool           `json:"truncated"`
}

func PlanNeedsShape(plan QueryPlan) bool {
	return len(plan.Derive) > 0 || len(plan.Matches) > 0 || plan.Sequence != nil
}

// ApplyShape evaluates derive, match, and sequence over scanned rows.
// A plan without those steps returns the scanned rows and their projections.
func ApplyShape(rows []ShapeRow, plan QueryPlan, truncated bool) ShapePage {
	working := cloneShapeRows(rows)
	for _, step := range plan.Derive {
		for index := range working {
			working[index].Fields[step.As] = deriveValue(working[index].Fields, step)
		}
	}
	showNotes := []string{}
	for _, match := range plan.Matches {
		kept := make([]ShapeRow, 0, len(working))
		for _, row := range working {
			values, total := matchValues(row.Fields, match)
			if !matchKeeps(values, match) {
				continue
			}
			shown, cut := clipShown(values)
			row.Fields[matchColumn(match)] = strings.Join(shown, ", ")
			if cut < total {
				showNotes = append(showNotes, fmt.Sprintf("A coluna mostra %d de %d valores.", len(shown), total))
			}
			kept = append(kept, row)
		}
		working = kept
	}
	summary := ""
	if plan.Sequence != nil {
		working, summary = applySequence(working, *plan.Sequence)
	} else {
		sortShapeRows(working)
	}
	if truncated {
		if summary != "" {
			summary += " "
		}
		summary += "A busca não cobriu o cadastro inteiro, então o resultado pode estar incompleto."
	}
	if len(showNotes) > 0 {
		if summary != "" {
			summary += " "
		}
		summary += showNotes[0]
	}
	page := ShapePage{
		Columns: shapeColumns(plan), Rows: shapeGrid(working, plan), Total: len(working),
		Summary: strings.TrimSpace(summary), Truncated: truncated,
	}
	return page
}

// Page slices a computed result. Sequence and reorder need the full scan first.
func (page ShapePage) Page(limit, offset int) ShapePage {
	if limit < 1 {
		limit = 1
	}
	if offset < 0 {
		offset = 0
	}
	next := page
	if offset >= len(page.Rows) {
		next.Rows = []ShapeGridRow{}
		return next
	}
	end := offset + limit
	if end > len(page.Rows) {
		end = len(page.Rows)
	}
	next.Rows = append([]ShapeGridRow(nil), page.Rows[offset:end]...)
	return next
}

func sortShapeRows(rows []ShapeRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		left, right := foldText(rows[i].Label), foldText(rows[j].Label)
		if left != right {
			return left < right
		}
		return rows[i].EntityID < rows[j].EntityID
	})
}

func cloneShapeRows(rows []ShapeRow) []ShapeRow {
	cloned := make([]ShapeRow, len(rows))
	for index, row := range rows {
		cloned[index] = row
		cloned[index].Fields = map[string]string{}
		for key, value := range row.Fields {
			cloned[index].Fields[key] = value
		}
		if cloned[index].Fields == nil {
			cloned[index].Fields = map[string]string{}
		}
	}
	return cloned
}

func deriveValue(fields map[string]string, step Derivation) string {
	source := fields[step.From]
	switch step.Op {
	case "keep":
		return keepClass(source, step.Class)
	case "token":
		return tokenAt(source, step.Index)
	case "first_letter":
		letter, ok := foldInitial(firstToken(source))
		if !ok {
			return ""
		}
		return string(letter)
	case "fold":
		return foldText(source)
	case "numbers":
		return strings.Join(numberTokens(source), " ")
	case "drop":
		return strings.Join(dropTokens(source, step.Pattern), " ")
	case "pick":
		tokens := numberTokens(source)
		if step.From != "" && fields[step.From] != source && strings.Contains(fields[step.From], " ") && step.Op == "pick" {
			tokens = strings.Fields(fields[step.From])
		}
		if len(tokens) == 0 {
			tokens = strings.Fields(source)
		}
		if step.Which == "first" {
			if len(tokens) == 0 {
				return ""
			}
			return tokens[0]
		}
		if len(tokens) == 0 {
			return ""
		}
		return tokens[len(tokens)-1]
	case "date_part":
		return datePart(source, step.Which)
	case "reverse":
		return reverseRunes(source)
	default:
		return ""
	}
}

func keepClass(value, class string) string {
	var builder strings.Builder
	for _, character := range value {
		switch class {
		case "digits":
			if character >= '0' && character <= '9' {
				builder.WriteRune(character)
			}
		case "letters":
			if folded, ok := foldInitial(string(character)); ok {
				builder.WriteRune(folded)
			}
		default:
			if character >= '0' && character <= '9' {
				builder.WriteRune(character)
				continue
			}
			if folded, ok := foldInitial(string(character)); ok {
				builder.WriteRune(folded)
			}
		}
	}
	return builder.String()
}

func tokenAt(value string, index *int) string {
	tokens := strings.Fields(value)
	if len(tokens) == 0 {
		return ""
	}
	at := 0
	if index != nil {
		at = *index
	}
	if at < 0 {
		at = len(tokens) + at
	}
	if at < 0 || at >= len(tokens) {
		return ""
	}
	return tokens[at]
}

func firstToken(value string) string {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func numberTokens(value string) []string {
	return digitRunShape.FindAllString(value, -1)
}

func dropTokens(value, pattern string) []string {
	kept := []string{}
	for _, token := range strings.Fields(value) {
		if tokenMatches(token, pattern) {
			continue
		}
		kept = append(kept, token)
	}
	if len(kept) > 0 {
		return kept
	}
	for _, token := range numberTokens(value) {
		if tokenMatches(token, pattern) {
			continue
		}
		kept = append(kept, token)
	}
	return kept
}

func tokenMatches(token, pattern string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}
	if hashes := strings.Count(pattern, "#"); hashes == len(pattern) && hashes > 0 {
		if len(token) != hashes {
			return false
		}
		for _, character := range token {
			if character < '0' || character > '9' {
				return false
			}
		}
		return true
	}
	return strings.Contains(strings.ToLower(token), strings.ToLower(pattern))
}

func datePart(value, which string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 10 && value[4] == '-' && value[7] == '-' {
		switch which {
		case "year":
			return value[0:4]
		case "day":
			return value[8:10]
		default:
			return value[5:7]
		}
	}
	if len(value) >= 7 && value[4] == '-' {
		switch which {
		case "year":
			return value[0:4]
		default:
			return value[5:7]
		}
	}
	return ""
}

func reverseRunes(value string) string {
	runes := []rune(value)
	for left, right := 0, len(runes)-1; left < right; left, right = left+1, right-1 {
		runes[left], runes[right] = runes[right], runes[left]
	}
	return string(runes)
}

func matchColumn(match MatchSpec) string {
	if strings.TrimSpace(match.Show) != "" {
		return strings.TrimSpace(match.Show)
	}
	return "found"
}

func matchKeeps(values []string, match MatchSpec) bool {
	if match.Quantifier == "all" {
		return len(match.Equals) > 0 && len(values) == len(match.Equals)
	}
	return len(values) > 0
}

func matchValues(fields map[string]string, match MatchSpec) ([]string, int) {
	source := fields[match.On]
	if match.Alphabet != "" {
		source = keepAlphabet(source, match.Alphabet)
	}
	order := match.Order
	if order == "" {
		order = "keep"
	}
	if order == "any" && match.Interpret == "byte" && match.Width > 0 && len(match.Equals) == 0 {
		return binaryCharacters(source, match.Width)
	}
	if order == "any" && match.Width > 0 && len(match.Equals) == 0 {
		formed := enumerateWidth(source, match.Width)
		return formed, len(formed)
	}
	if len(match.Equals) > 0 {
		found := []string{}
		for _, target := range match.Equals {
			if order == "any" {
				if covers(source, target, match.Width) {
					found = append(found, target)
				}
				continue
			}
			if strings.Contains(foldText(source), foldText(target)) {
				found = append(found, target)
			}
		}
		return found, len(found)
	}
	if order == "any" {
		if match.Pattern != "" && covers(source, match.Pattern, match.Width) {
			return []string{match.Pattern}, 1
		}
		return nil, 0
	}
	span := keepSpan(source, match)
	if span == "" {
		return nil, 0
	}
	return []string{span}, 1
}

func keepAlphabet(value, alphabet string) string {
	allowed := map[rune]bool{}
	for _, character := range alphabet {
		if character == '-' || character == ',' {
			continue
		}
		allowed[character] = true
		if folded, ok := foldInitial(string(character)); ok {
			allowed[folded] = true
			allowed[folded+('a'-'A')] = true
		}
	}
	if strings.Contains(alphabet, "0") && strings.Contains(alphabet, "1") && !strings.ContainsAny(alphabet, "23456789") {
		allowed = map[rune]bool{'0': true, '1': true}
	}
	var builder strings.Builder
	for _, character := range value {
		if allowed[character] {
			builder.WriteRune(character)
		}
	}
	return builder.String()
}

func covers(source, target string, width int) bool {
	if width == 0 {
		return sameCounts(source, target)
	}
	if utf8.RuneCountInString(target) != width {
		return false
	}
	return countsCover(source, target)
}

func countMap(value string) map[rune]int {
	counts := map[rune]int{}
	for _, character := range foldText(value) {
		counts[character]++
	}
	return counts
}

func sameCounts(left, right string) bool {
	return mapsEqual(countMap(left), countMap(right))
}

func countsCover(source, target string) bool {
	have := countMap(source)
	for character, count := range countMap(target) {
		if have[character] < count {
			return false
		}
	}
	return true
}

func mapsEqual(left, right map[rune]int) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

var digitRunShape = regexp.MustCompile(`\d+`)

func foldText(value string) string {
	var builder strings.Builder
	for _, character := range value {
		if letter, ok := foldRune(character); ok {
			builder.WriteRune(letter)
			continue
		}
		builder.WriteRune(character)
	}
	return builder.String()
}

func foldInitial(value string) (rune, bool) {
	for _, character := range value {
		if letter, ok := foldRune(character); ok {
			return letter, true
		}
	}
	return 0, false
}

func foldRune(character rune) (rune, bool) {
	switch character {
	case 'á', 'à', 'â', 'ã', 'ä', 'Á', 'À', 'Â', 'Ã', 'Ä':
		return 'A', true
	case 'é', 'è', 'ê', 'ë', 'É', 'È', 'Ê', 'Ë':
		return 'E', true
	case 'í', 'ì', 'î', 'ï', 'Í', 'Ì', 'Î', 'Ï':
		return 'I', true
	case 'ó', 'ò', 'ô', 'õ', 'ö', 'Ó', 'Ò', 'Ô', 'Õ', 'Ö':
		return 'O', true
	case 'ú', 'ù', 'û', 'ü', 'Ú', 'Ù', 'Û', 'Ü':
		return 'U', true
	case 'ç', 'Ç':
		return 'C', true
	default:
		if character >= 'a' && character <= 'z' {
			return character - ('a' - 'A'), true
		}
		if character >= 'A' && character <= 'Z' {
			return character, true
		}
		return 0, false
	}
}

func keepSpan(source string, match MatchSpec) string {
	if strings.TrimSpace(match.Pattern) == "" {
		return ""
	}
	grammar := match.Grammar
	if grammar == "" {
		grammar = PatternLiteralSequence
	}
	body, err := compilePatternExpression(PatternPredicate{Grammar: grammar, Pattern: match.Pattern, Anchored: match.Anchored})
	if err != nil || body == "" {
		if strings.Contains(foldText(source), foldText(match.Pattern)) {
			return match.Pattern
		}
		return ""
	}
	compiled, err := regexp.Compile(body)
	if err != nil {
		return ""
	}
	return compiled.FindString(source)
}

func binaryCharacters(source string, width int) ([]string, int) {
	zeros, ones := 0, 0
	for _, character := range source {
		switch character {
		case '0':
			zeros++
		case '1':
			ones++
		}
	}
	seen := map[string]struct{}{}
	total := 0
	for used := 0; used <= width; used++ {
		if used > ones || width-used > zeros {
			continue
		}
		for _, bits := range bitPatterns(width, used) {
			value := byteCharacter(bits)
			if value == "" {
				continue
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			total++
		}
	}
	values := make([]string, 0, len(seen))
	for value := range seen {
		values = append(values, value)
	}
	sort.Strings(values)
	return values, total
}

func bitPatterns(width, ones int) []string {
	if ones < 0 || ones > width {
		return nil
	}
	patterns := []string{}
	var walk func(prefix string, left, need int)
	walk = func(prefix string, left, need int) {
		if left == 0 {
			if need == 0 {
				patterns = append(patterns, prefix)
			}
			return
		}
		if need < left {
			walk(prefix+"0", left-1, need)
		}
		if need > 0 {
			walk(prefix+"1", left-1, need-1)
		}
	}
	walk("", width, ones)
	return patterns
}

func byteCharacter(bits string) string {
	value := 0
	for _, character := range bits {
		value <<= 1
		if character == '1' {
			value++
		}
	}
	if value < 32 || value > 126 {
		return ""
	}
	return string(rune(value))
}

func enumerateWidth(source string, width int) []string {
	if width < 1 || width > 8 || utf8.RuneCountInString(source) > 16 {
		return nil
	}
	counts := countMap(source)
	alphabet := make([]rune, 0, len(counts))
	for character := range counts {
		alphabet = append(alphabet, character)
	}
	sort.Slice(alphabet, func(i, j int) bool { return alphabet[i] < alphabet[j] })
	found := []string{}
	var walk func(prefix []rune, used map[rune]int)
	walk = func(prefix []rune, used map[rune]int) {
		if len(found) >= MaximumShownValues {
			return
		}
		if len(prefix) == width {
			found = append(found, string(prefix))
			return
		}
		for _, character := range alphabet {
			if used[character] >= counts[character] {
				continue
			}
			used[character]++
			walk(append(prefix, character), used)
			used[character]--
		}
	}
	walk(nil, map[rune]int{})
	return found
}

func clipShown(values []string) ([]string, int) {
	total := len(values)
	shown := make([]string, 0, len(values))
	runes := 0
	for _, value := range values {
		if len(shown) >= MaximumShownValues {
			break
		}
		next := utf8.RuneCountInString(value) + 2
		if len(shown) > 0 && runes+next > MaximumShownRunes {
			break
		}
		shown = append(shown, value)
		runes += next
	}
	return shown, total
}

func shapeColumns(plan QueryPlan) []ShapeColumn {
	columns := []ShapeColumn{}
	seen := map[string]bool{}
	add := func(key, label string) {
		if key == "" || seen[key] {
			return
		}
		seen[key] = true
		if label == "" {
			label = key
		}
		columns = append(columns, ShapeColumn{Key: key, Label: label})
	}
	for _, key := range plan.Projections {
		add(key, key)
	}
	for _, step := range plan.Derive {
		add(step.As, step.As)
	}
	for _, match := range plan.Matches {
		add(matchColumn(match), matchColumn(match))
	}
	if len(columns) == 0 {
		add("label", "Registro")
	}
	return columns
}

func shapeGrid(rows []ShapeRow, plan QueryPlan) []ShapeGridRow {
	columns := shapeColumns(plan)
	seen := map[string]int{}
	grid := make([]ShapeGridRow, 0, len(rows))
	for _, row := range rows {
		id := row.EntityID
		if id == "" {
			id = row.Label
		}
		seen[id]++
		if seen[id] > 1 {
			id = fmt.Sprintf("%s#%d", row.EntityID, seen[id])
		}
		cells := map[string]string{}
		for _, column := range columns {
			if column.Key == "label" {
				cells[column.Key] = row.Label
				continue
			}
			cells[column.Key] = row.Fields[column.Key]
		}
		grid = append(grid, ShapeGridRow{ID: id, EntityKind: row.EntityKind, EntityID: row.EntityID, Label: row.Label, Cells: cells})
	}
	return grid
}
