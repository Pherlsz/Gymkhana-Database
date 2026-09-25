package importcatalog

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/Pherlsz/Gymkhana-Core/normalize"
)

const inferenceThreshold = 0.55

var brazilianUFs = map[string]struct{}{
	"AC": {}, "AL": {}, "AP": {}, "AM": {}, "BA": {}, "CE": {}, "DF": {}, "ES": {},
	"GO": {}, "MA": {}, "MT": {}, "MS": {}, "MG": {}, "PA": {}, "PB": {}, "PR": {},
	"PE": {}, "PI": {}, "RJ": {}, "RN": {}, "RS": {}, "RO": {}, "RR": {}, "SC": {},
	"SP": {}, "SE": {}, "TO": {},
}

// legado scorer keys → rebuild field IDs. rg becomes a sidecar document.
var inferenceTargets = map[string]string{
	"cpf": "cpf", "email": "email", "phone": "mobile_phone", "landline": "landline_phone",
	"birth_date": "birth_date", "gender": "gender", "cep": "address_postal_code",
	"state": "address_state", "name": "full_name", "street": "address_street",
	"neighborhood": "address_neighborhood", "city": "address_city",
	"nationality": "nationality", "rg": DocumentFieldPrefix + "rg",
}

var inferencePriority = map[string]int{
	"cpf": 100, "email": 98, "name": 96, "cep": 92, "rg": 88,
	"phone": 82, "landline": 80, "birth_date": 75, "nationality": 70,
	"gender": 65, "state": 60, "neighborhood": 59, "city": 58, "street": 54,
}

// FillUnmapped runs sample inference only on columns the header catalog left empty.
func FillUnmapped(current []string, samples [][]string) []string {
	out := append([]string(nil), current...)
	inferred := InferFromSamples(samples, len(current))
	used := map[string]struct{}{}
	for _, target := range out {
		if target != "" && target != DiscardSentinel {
			used[target] = struct{}{}
		}
	}
	for index, target := range inferred {
		if index >= len(out) || out[index] != "" || target == "" {
			continue
		}
		if _, taken := used[target]; taken {
			continue
		}
		out[index] = target
		used[target] = struct{}{}
	}
	for index, target := range out {
		if target != "" {
			continue
		}
		if looksLikeRowIndex(columnValues(samples, index)) {
			out[index] = DiscardSentinel
		}
	}
	return out
}

// InferFromSamples ports legado inferColumnMappingFromSamples onto rebuild field IDs.
func InferFromSamples(sampleRows [][]string, columnCount int) []string {
	result := make([]string, columnCount)
	if columnCount < 1 || len(sampleRows) == 0 {
		return result
	}
	address := detectAddressBlock(sampleRows, columnCount)
	used := map[string]struct{}{}
	for index, key := range address {
		if key == "" {
			continue
		}
		result[index] = inferenceTargets[key]
		used[key] = struct{}{}
	}
	type candidate struct {
		index int
		key   string
		score float64
	}
	var candidates []candidate
	for index := 0; index < columnCount; index++ {
		if result[index] != "" {
			continue
		}
		values := columnValues(sampleRows, index)
		bestKey, bestScore := "", 0.0
		bestPriority := -1
		for key, scorer := range fieldScorers {
			if _, taken := used[key]; taken {
				continue
			}
			score := scorer(values)
			if score < inferenceThreshold {
				continue
			}
			priority := inferencePriority[key]
			if bestKey == "" || score > bestScore || (score == bestScore && priority > bestPriority) {
				bestKey, bestScore, bestPriority = key, score, priority
			}
		}
		if bestKey != "" {
			candidates = append(candidates, candidate{index: index, key: bestKey, score: bestScore})
		}
	}
	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].score > candidates[i].score {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}
	for _, item := range candidates {
		if result[item.index] != "" {
			continue
		}
		if _, taken := used[item.key]; taken {
			continue
		}
		result[item.index] = inferenceTargets[item.key]
		used[item.key] = struct{}{}
	}
	return result
}

var fieldScorers = map[string]func([]string) float64{
	"cpf":          scoreCPF,
	"email":        scoreEmail,
	"phone":        scoreMobile,
	"landline":     scoreLandline,
	"birth_date":   scoreDate,
	"gender":       scoreGender,
	"cep":          scoreCEP,
	"state":        scoreState,
	"name":         scoreName,
	"street":       scoreStreet,
	"neighborhood": scoreNeighborhood,
	"city":         scoreCity,
	"nationality":  scoreNationality,
	"rg":           scoreRG,
}

func detectAddressBlock(sampleRows [][]string, columnCount int) []string {
	assigned := make([]string, columnCount)
	used := map[string]struct{}{}
	assign := func(index int, key string) {
		if index < 0 || index >= columnCount || assigned[index] != "" {
			return
		}
		if _, taken := used[key]; taken {
			return
		}
		assigned[index] = key
		used[key] = struct{}{}
	}
	for i := 0; i < columnCount; i++ {
		values := columnValues(sampleRows, i)
		if scoreState(values) < inferenceThreshold {
			continue
		}
		assign(i, "state")
		if i-1 >= 0 && isPlaceLike(columnValues(sampleRows, i-1)) {
			if i-2 >= 0 && isPlaceLike(columnValues(sampleRows, i-2)) {
				assign(i-2, "neighborhood")
				assign(i-1, "city")
			} else {
				assign(i-1, "city")
			}
		}
		if i-3 >= 0 && scoreStreet(columnValues(sampleRows, i-3)) >= inferenceThreshold {
			assign(i-3, "street")
		}
	}
	for i := 0; i < columnCount; i++ {
		values := columnValues(sampleRows, i)
		if scoreCEP(values) < inferenceThreshold {
			continue
		}
		assign(i, "cep")
		if i-1 >= 0 && scoreState(columnValues(sampleRows, i-1)) >= inferenceThreshold {
			assign(i-1, "state")
		}
		if i-2 >= 0 && isPlaceLike(columnValues(sampleRows, i-2)) {
			if i-3 >= 0 && isPlaceLike(columnValues(sampleRows, i-3)) {
				assign(i-3, "neighborhood")
				assign(i-2, "city")
			} else {
				assign(i-2, "city")
			}
		}
		if i-4 >= 0 && scoreStreet(columnValues(sampleRows, i-4)) >= inferenceThreshold {
			assign(i-4, "street")
		}
	}
	return assigned
}

func isPlaceLike(values []string) bool {
	if scoreState(values) >= inferenceThreshold || scoreStreet(values) >= inferenceThreshold || scoreCEP(values) >= inferenceThreshold {
		return false
	}
	return maxFloat(scoreNeighborhood(values), scoreCity(values)) >= inferenceThreshold
}

func columnValues(rows [][]string, index int) []string {
	limit := len(rows)
	if limit > 50 {
		limit = 50
	}
	values := make([]string, 0, limit)
	for _, row := range rows[:limit] {
		if index < len(row) {
			values = append(values, row[index])
		} else {
			values = append(values, "")
		}
	}
	return values
}

func ratio(values []string, match func(string) bool) float64 {
	n := 0
	hits := 0
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		n++
		if match(trimmed) {
			hits++
		}
	}
	if n == 0 {
		return 0
	}
	return float64(hits) / float64(n)
}

func scoreCPF(values []string) float64 {
	return ratio(values, func(value string) bool {
		_, err := normalize.CanonicalCPF(value)
		return err == nil
	})
}

func scoreEmail(values []string) float64 {
	return ratio(values, func(value string) bool {
		_, err := normalize.CanonicalEmail(value)
		return err == nil
	})
}

func scoreMobile(values []string) float64 {
	return ratio(values, func(value string) bool {
		phone, err := normalize.CanonicalBrazilPhone(value)
		return err == nil && len(phone) == 14
	})
}

func scoreLandline(values []string) float64 {
	return ratio(values, func(value string) bool {
		phone, err := normalize.CanonicalBrazilPhone(value)
		return err == nil && len(phone) == 13
	})
}

func scoreDate(values []string) float64 {
	return ratio(values, func(value string) bool {
		if looksLikeSlashDate(value) {
			return true
		}
		if len(value) >= 10 && value[4] == '-' && value[7] == '-' {
			return true
		}
		return false
	})
}

func scoreGender(values []string) float64 {
	return ratio(values, func(value string) bool {
		if len(value) == 1 {
			upper := strings.ToUpper(value)
			return upper == "M" || upper == "F" || upper == "O"
		}
		switch normalize.SearchText(value) {
		case "masculino", "feminino", "outro":
			return true
		}
		return false
	})
}

func scoreCEP(values []string) float64 {
	return ratio(values, func(value string) bool {
		if strings.ContainsAny(value, "/-") {
			return false
		}
		return len(normalize.Digits(value)) == 8
	})
}

func scoreState(values []string) float64 {
	return ratio(values, func(value string) bool {
		_, ok := brazilianUFs[strings.ToUpper(strings.TrimSpace(value))]
		return ok
	})
}

func scoreName(values []string) float64 {
	return ratio(values, func(value string) bool {
		lower := strings.ToLower(value)
		if strings.Contains(lower, "rua") || strings.Contains(lower, "avenida") || strings.Contains(lower, "travessa") {
			return false
		}
		if strings.Contains(value, "@") || len(normalize.Digits(value)) > 3 {
			return false
		}
		return len(strings.Fields(value)) >= 2 && len(value) >= 6 && hasLetter(value)
	})
}

func scoreStreet(values []string) float64 {
	return ratio(values, func(value string) bool {
		folded := normalize.SearchText(value)
		if strings.Contains(folded, "rua") || strings.Contains(folded, "avenida") || strings.Contains(folded, "travessa") || strings.Contains(folded, "alameda") {
			return true
		}
		return len(strings.TrimSpace(value)) >= 12 && normalize.Digits(value) != ""
	})
}

func scoreNeighborhood(values []string) float64 {
	return ratio(values, func(value string) bool {
		if len(value) == 3 && isAllLetters(value) {
			return false
		}
		n := len(strings.TrimSpace(value))
		return n >= 3 && n <= 40 && !strings.Contains(value, "@") && len(normalize.Digits(value)) <= 2
	})
}

func scoreCity(values []string) float64 {
	return ratio(values, func(value string) bool {
		if _, ok := brazilianUFs[strings.ToUpper(value)]; ok {
			return false
		}
		if len(value) == 3 && isAllLetters(value) {
			return false
		}
		n := len(strings.TrimSpace(value))
		return n >= 3 && n <= 40 && !strings.Contains(value, "@") && len(normalize.Digits(value)) <= 2
	})
}

func scoreNationality(values []string) float64 {
	iso3 := ratio(values, func(value string) bool {
		return len(value) == 3 && isAllLetters(value)
	})
	named := ratio(values, func(value string) bool {
		switch normalize.SearchText(value) {
		case "bra", "brasil", "brazil", "arg", "argentina", "ury", "uruguai", "par", "paraguai", "chl", "chile":
			return true
		}
		return false
	})
	return maxFloat(iso3, named)
}

func scoreRG(values []string) float64 {
	return ratio(values, func(value string) bool {
		if _, err := normalize.CanonicalBrazilPhone(value); err == nil {
			return false
		}
		digits := normalize.Digits(value)
		if len(digits) == 8 || len(digits) == 11 {
			return false
		}
		if len(digits) < 7 || len(digits) > 12 {
			return false
		}
		_, err := normalize.CanonicalDocument(normalize.DocumentRG, value)
		return err == nil
	})
}

func looksLikeSlashDate(value string) bool {
	parts := strings.Split(value, "/")
	if len(parts) != 3 || len(parts[2]) != 4 {
		return false
	}
	return digitsOnly(parts[0]) == parts[0] && digitsOnly(parts[1]) == parts[1] && digitsOnly(parts[2]) == parts[2]
}

func digitsOnly(value string) string {
	return normalize.Digits(value)
}

func hasLetter(value string) bool {
	for _, character := range strings.ToLower(value) {
		if character >= 'a' && character <= 'z' {
			return true
		}
	}
	return false
}

func isAllLetters(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if !unicode.IsLetter(character) {
			return false
		}
	}
	return true
}

func maxFloat(left, right float64) float64 {
	if left > right {
		return left
	}
	return right
}

func looksLikeRowIndex(values []string) bool {
	n := 0
	previous := -1
	sequential := 0
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		parsed, err := strconv.Atoi(trimmed)
		if err != nil || parsed < 1 || parsed > 1_000_000 {
			return false
		}
		n++
		if previous >= 0 && parsed == previous+1 {
			sequential++
		}
		previous = parsed
	}
	return n >= 3 && sequential >= n-1
}
