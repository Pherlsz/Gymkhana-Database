package aichat

import (
	"context"
	"encoding/json"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	querydomain "github.com/Pherlsz/Gymkhana-Database/internal/queryengine"
)

var (
	cepPattern  = regexp.MustCompile(`\d{5}-?\d{3}`)
	digitRun    = regexp.MustCompile(`\d+`)
	stateSuffix = regexp.MustCompile(`(?i)\s*[-/]\s*[a-z]{2}\s*$`)
)

type sequenceToolRequest struct {
	CatalogVersion  string                  `json:"catalog_version"`
	RootEntity      string                  `json:"root_entity"`
	LetterField     string                  `json:"letter_field"`
	NumberField     string                  `json:"number_field"`
	NumberExtractor string                  `json:"number_extractor,omitempty"`
	CityField       string                  `json:"city_field,omitempty"`
	MinimumLength   int                     `json:"minimum_length,omitempty"`
	Filter          *querydomain.FilterNode `json:"filter,omitempty"`
}

type letterItem struct {
	id      string
	letter  rune
	number  int
	city    string
	holder  string
	address string
}

func (gateway *ToolGateway) sequenceData(ctx context.Context, actor auth.Session, raw json.RawMessage, requestID string) (ToolOutput, error) {
	var request sequenceToolRequest
	if err := decodeToolArguments(raw, &request); err != nil {
		return ToolOutput{}, err
	}
	request.LetterField = strings.TrimSpace(request.LetterField)
	request.NumberField = strings.TrimSpace(request.NumberField)
	request.CityField = strings.TrimSpace(request.CityField)
	if request.MinimumLength == 0 {
		request.MinimumLength = 1
	}
	if request.LetterField == "" || request.NumberField == "" || request.MinimumLength < 1 || request.MinimumLength > 26 || !validNumberExtractor(request.NumberExtractor) {
		return ToolOutput{}, normalizeToolError(&querydomain.ValidationError{Fields: []querydomain.FieldError{{Field: "letter_field", Code: "required"}}})
	}
	projections := uniqueFields(request.LetterField, request.NumberField, request.CityField)
	plan := querydomain.QueryPlan{
		Version:        querydomain.PlanVersionV1,
		CatalogVersion: strings.TrimSpace(request.CatalogVersion),
		RootEntity:     strings.TrimSpace(request.RootEntity),
		Projections:    projections,
		Filter:         request.Filter,
		MaximumRows:    1,
	}
	scan, err := gateway.query.ScanFields(ctx, actor, plan, querydomain.MaximumSequenceScan, requestID)
	if err != nil {
		return ToolOutput{}, normalizeToolError(err)
	}
	column := map[string]int{}
	for index, key := range projections {
		column[key] = index
	}
	items := make([]letterItem, 0, len(scan.Rows))
	for _, row := range scan.Rows {
		holder := fieldAt(row.Values, column, request.LetterField)
		address := fieldAt(row.Values, column, request.NumberField)
		letter, ok := firstLetter(holder)
		if !ok {
			continue
		}
		number, ok := orderNumber(address, request.NumberExtractor)
		if !ok {
			continue
		}
		city := ""
		if request.CityField != "" && request.CityField != request.NumberField {
			city = strings.TrimSpace(fieldAt(row.Values, column, request.CityField))
		}
		if city == "" {
			city = cityFromAddress(address)
		}
		items = append(items, letterItem{id: row.ID, letter: letter, number: number, city: city, holder: holder, address: address})
	}
	chain, direction := bestLetterChain(items)
	payload, err := sequencePayload(chain, direction, request.MinimumLength, !scan.Truncated)
	if err != nil {
		return ToolOutput{}, err
	}
	return ToolOutput{Kind: ToolQuery, Payload: payload, RowCount: len(chain), FieldCount: 4, ByteCount: len(payload)}, nil
}

func sequencePayload(chain []letterItem, direction string, minimum int, complete bool) ([]byte, error) {
	rows := make([]sequenceRow, 0, len(chain))
	letters := make([]rune, 0, len(chain))
	for _, item := range chain {
		letters = append(letters, item.letter)
		row := sequenceRow{Letter: string(item.letter), Holder: clipRunes(item.holder, 180), Number: item.number}
		if item.address != "" {
			row.Address = clipRunes(item.address, 180)
		}
		if item.city != "" {
			row.City = clipRunes(item.city, 80)
		}
		rows = append(rows, row)
	}
	start, end := "", ""
	if len(letters) > 0 {
		start, end = string(letters[0]), string(letters[len(letters)-1])
	}
	body := struct {
		Note           string        `json:"note"`
		Summary        string        `json:"resumo"`
		Length         int           `json:"length"`
		Direction      string        `json:"direction"`
		Start          string        `json:"start"`
		End            string        `json:"end"`
		Letters        string        `json:"letters"`
		MeetsMinimum   bool          `json:"meets_minimum"`
		MinimumLength  int           `json:"minimum_length"`
		DistinctCities int           `json:"distinct_cities"`
		CityBonus      bool          `json:"city_bonus"`
		ScanComplete   bool          `json:"scan_complete"`
		Rows           []sequenceRow `json:"rows"`
	}{
		Note:           "length é a maior sequência contínua de letras, com o número na mesma direção o tempo todo. rows são esses registros, não uma amostra.",
		Summary:        sequenceSummary(chain, direction, minimum, complete),
		Length:         len(chain),
		Direction:      direction,
		Start:          start,
		End:            end,
		Letters:        string(letters),
		MeetsMinimum:   len(chain) >= minimum,
		MinimumLength:  minimum,
		DistinctCities: distinctCities(chain),
		CityBonus:      cityBonus(chain),
		ScanComplete:   complete,
		Rows:           rows,
	}
	return marshalBoundedToolPayload(body)
}

type sequenceRow struct {
	Letter  string `json:"letra"`
	Holder  string `json:"titular"`
	Number  int    `json:"numero"`
	Address string `json:"endereco,omitempty"`
	City    string `json:"cidade,omitempty"`
}

func sequenceSummary(chain []letterItem, direction string, minimum int, complete bool) string {
	if len(chain) == 0 {
		text := "Nenhum registro formou sequência. O mínimo pedido é de " + strconv.Itoa(minimum) + "."
		if !complete {
			text += " A busca não cobriu o cadastro inteiro."
		}
		return text
	}
	text := "Maior sequência contínua: " + strconv.Itoa(len(chain)) + " registros, de " + string(chain[0].letter) + " até " + string(chain[len(chain)-1].letter) + ", números " + direction + "."
	if len(chain) >= minimum {
		text += " Atende o mínimo de " + strconv.Itoa(minimum) + "."
	} else {
		text += " Não atende o mínimo de " + strconv.Itoa(minimum) + "."
	}
	if cityBonus(chain) {
		text += " Bonificação de municípios diferentes vale."
	} else if len(chain) == 26 {
		text += " Os 26 registros não são de municípios todos diferentes."
	}
	if !complete {
		text += " A busca não cobriu o cadastro inteiro, então a sequência pode estar incompleta."
	}
	return text
}

func bestLetterChain(items []letterItem) ([]letterItem, string) {
	increasing := chainInDirection(items, 1)
	decreasing := chainInDirection(items, -1)
	if len(decreasing) > len(increasing) || (len(decreasing) == len(increasing) && distinctCities(decreasing) > distinctCities(increasing)) {
		return decreasing, "decrescentes"
	}
	if len(increasing) == 0 {
		return nil, ""
	}
	return increasing, "crescentes"
}

func chainInDirection(items []letterItem, direction int) []letterItem {
	grouped := map[rune][]letterItem{}
	for _, item := range items {
		if item.letter < 'A' || item.letter > 'Z' || item.number <= 0 {
			continue
		}
		grouped[item.letter] = append(grouped[item.letter], item)
	}
	type node struct {
		length int
		prev   int
	}
	nodes := map[rune][]node{}
	for letter := 'A'; letter <= 'Z'; letter++ {
		group := grouped[letter]
		sort.Slice(group, func(i, j int) bool {
			if group[i].number != group[j].number {
				if direction > 0 {
					return group[i].number < group[j].number
				}
				return group[i].number > group[j].number
			}
			return group[i].id < group[j].id
		})
		grouped[letter] = group
		current := make([]node, len(group))
		previous := grouped[letter-1]
		previousNodes := nodes[letter-1]
		cursor := 0
		bestLength := 0
		bestIndex := -1
		for index, item := range group {
			for cursor < len(previous) && extends(direction, previous[cursor].number, item.number) {
				if previousNodes[cursor].length > bestLength {
					bestLength = previousNodes[cursor].length
					bestIndex = cursor
				}
				cursor++
			}
			current[index] = node{length: 1, prev: -1}
			if bestIndex >= 0 {
				current[index] = node{length: bestLength + 1, prev: bestIndex}
			}
		}
		nodes[letter] = current
	}
	bestLetter := rune(0)
	bestIndex := -1
	bestLength := 0
	for letter := 'A'; letter <= 'Z'; letter++ {
		for index, node := range nodes[letter] {
			if node.length > bestLength {
				bestLength = node.length
				bestLetter = letter
				bestIndex = index
			}
		}
	}
	if bestIndex < 0 {
		return nil
	}
	chain := make([]letterItem, 0, bestLength)
	for letter, index := bestLetter, bestIndex; index >= 0; letter, index = letter-1, nodes[letter][index].prev {
		chain = append(chain, grouped[letter][index])
	}
	for left, right := 0, len(chain)-1; left < right; left, right = left+1, right-1 {
		chain[left], chain[right] = chain[right], chain[left]
	}
	return chain
}

func extends(direction, previous, current int) bool {
	if direction > 0 {
		return previous < current
	}
	return previous > current
}

func firstLetter(name string) (rune, bool) {
	fields := strings.Fields(name)
	if len(fields) == 0 {
		return 0, false
	}
	for _, character := range fields[0] {
		if letter, ok := foldInitial(character); ok {
			return letter, true
		}
	}
	return 0, false
}

func foldInitial(character rune) (rune, bool) {
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

func orderNumber(value, extractor string) (int, bool) {
	if extractor == "digitos" {
		return digitsValue(value)
	}
	return houseNumber(value)
}

func validNumberExtractor(extractor string) bool {
	return extractor == "" || extractor == "numero_imovel" || extractor == "digitos"
}

func digitsValue(value string) (int, bool) {
	var builder strings.Builder
	for _, character := range value {
		if character >= '0' && character <= '9' {
			builder.WriteRune(character)
		}
	}
	token := strings.TrimLeft(builder.String(), "0")
	if token == "" || len(token) > 9 {
		return 0, false
	}
	number, err := strconv.Atoi(token)
	if err != nil || number <= 0 {
		return 0, false
	}
	return number, true
}

func houseNumber(address string) (int, bool) {
	cleaned := cepPattern.ReplaceAllString(address, " ")
	matches := digitRun.FindAllString(cleaned, -1)
	for index := len(matches) - 1; index >= 0; index-- {
		token := matches[index]
		if len(token) < 1 || len(token) > 6 {
			continue
		}
		number, err := strconv.Atoi(token)
		if err != nil || number <= 0 {
			continue
		}
		return number, true
	}
	return 0, false
}

func cityFromAddress(address string) string {
	cleaned := cepPattern.ReplaceAllString(address, " ")
	parts := strings.Split(cleaned, ",")
	named := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || !containsLetter(part) || isMostlyNumber(part) {
			continue
		}
		named = append(named, part)
	}
	if len(named) < 2 {
		return ""
	}
	city := strings.TrimSpace(stateSuffix.ReplaceAllString(named[len(named)-1], ""))
	if !containsLetter(city) {
		return ""
	}
	return city
}

func containsLetter(value string) bool {
	for _, character := range value {
		if _, ok := foldInitial(character); ok {
			return true
		}
	}
	return false
}

func isMostlyNumber(value string) bool {
	digits := 0
	letters := 0
	for _, character := range value {
		switch {
		case character >= '0' && character <= '9':
			digits++
		default:
			if _, ok := foldInitial(character); ok {
				letters++
			}
		}
	}
	return digits > 0 && letters == 0
}

func cityKey(city string) string {
	city = strings.TrimSpace(city)
	if city == "" {
		return ""
	}
	var builder strings.Builder
	for _, character := range city {
		if letter, ok := foldInitial(character); ok {
			builder.WriteRune(letter + ('a' - 'A'))
			continue
		}
		builder.WriteRune(character)
	}
	return strings.ToLower(builder.String())
}

func distinctCities(chain []letterItem) int {
	seen := map[string]struct{}{}
	for _, item := range chain {
		key := cityKey(item.city)
		if key == "" {
			continue
		}
		seen[key] = struct{}{}
	}
	return len(seen)
}

func cityBonus(chain []letterItem) bool {
	if len(chain) != 26 {
		return false
	}
	seen := map[string]struct{}{}
	for _, item := range chain {
		key := cityKey(item.city)
		if key == "" {
			return false
		}
		if _, exists := seen[key]; exists {
			return false
		}
		seen[key] = struct{}{}
	}
	return true
}

func uniqueFields(values ...string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func fieldAt(values []string, column map[string]int, key string) string {
	index, ok := column[key]
	if !ok || index < 0 || index >= len(values) {
		return ""
	}
	return values[index]
}

func clipRunes(value string, maximum int) string {
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}
