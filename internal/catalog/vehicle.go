package catalog

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/Pherlsz/Gymkhana-Core/normalize"
)

var platePattern = regexp.MustCompile(`(?i)^[A-Z]{3}\d{4}$|^[A-Z]{3}\d[A-Z]\d{2}$`)

var vehicleColors = map[string]string{
	"branco": "Branco", "branca": "Branco", "preto": "Preto", "preta": "Preto",
	"prata": "Prata", "cinza": "Cinza", "vermelho": "Vermelho", "vermelha": "Vermelho",
	"azul": "Azul", "verde": "Verde", "amarelo": "Amarelo", "amarela": "Amarelo",
	"grafite": "Grafite", "bordo": "Bordô", "vinho": "Vinho", "bege": "Bege",
}

var carModels = []labeledRule{
	{"Uno", []string{"uno"}},
	{"Gol", []string{"gol"}},
	{"Fox", []string{"fox"}},
	{"Palio", []string{"palio"}},
	{"Celta", []string{"celta"}},
	{"Onix", []string{"onix"}},
	{"HB20", []string{"hb20"}},
	{"Ka", []string{"ka", "ford ka"}},
	{"Civic", []string{"civic"}},
	{"Corolla", []string{"corolla"}},
	{"Sandero", []string{"sandero"}},
	{"Saveiro", []string{"saveiro"}},
	{"Voyage", []string{"voyage"}},
	{"Strada", []string{"strada"}},
	{"Kwid", []string{"kwid"}},
	{"Fusca", []string{"fusca"}},
}

// FormatVehiclePlate returns a Brazilian plate, or empty. Model+year (UNO2010)
// is not a plate.
func FormatVehiclePlate(value string) string {
	if isAbsent(value) {
		return ""
	}
	compact := normalize.Alphanumeric(value)
	if yearToken.MatchString(compact) {
		return ""
	}
	if glued := gluedModelYear.FindStringSubmatch(strings.ToLower(compact)); len(glued) == 3 {
		if carModelToken(glued[1]) != "" || glued[1] == "ano" {
			return ""
		}
	}
	if platePattern.MatchString(compact) {
		if len(compact) == 7 && compact[3] >= '0' && compact[3] <= '9' && compact[4] >= '0' && compact[4] <= '9' {
			letters, nums := compact[:3], compact[3:]
			if yearRange(nums) && carModelToken(strings.ToLower(letters)) != "" {
				return ""
			}
		}
		return compact
	}
	return ""
}

var yearToken = regexp.MustCompile(`^(ANO)?(19|20)\d{2}$`)
var gluedModelYear = regexp.MustCompile(`^([a-z]{2,16})((?:19|20)\d{2})$`)

// FormatVehicleModel returns a catalog car model.
func FormatVehicleModel(value string) string {
	if isAbsent(value) {
		return ""
	}
	compact := strings.ToLower(normalize.Alphanumeric(value))
	if glued := gluedModelYear.FindStringSubmatch(compact); len(glued) == 3 {
		if model := carModelToken(glued[1]); model != "" {
			return model
		}
	}
	return joinLabeled(normalize.SearchText(value), carModels)
}

// FormatVehicleColor returns a canonical color.
func FormatVehicleColor(value string) string {
	if isAbsent(value) {
		return ""
	}
	return vehicleColors[normalize.SearchText(value)]
}

// FormatVehicleYear returns a 4-digit year between 1950 and 2026.
func FormatVehicleYear(value string) string {
	if isAbsent(value) {
		return ""
	}
	compact := strings.ToUpper(normalize.Alphanumeric(value))
	if glued := gluedModelYear.FindStringSubmatch(strings.ToLower(compact)); len(glued) == 3 {
		if yearRange(glued[2]) && (glued[1] == "ano" || carModelToken(glued[1]) != "") {
			return glued[2]
		}
	}
	digits := normalize.Digits(value)
	if yearRange(digits) {
		return digits
	}
	return ""
}

func carModelToken(token string) string {
	for _, rule := range carModels {
		name := strings.ToLower(normalize.Alphanumeric(rule.label))
		if token == name {
			return rule.label
		}
		for _, needle := range rule.needles {
			if token == strings.ReplaceAll(needle, " ", "") {
				return rule.label
			}
		}
	}
	return ""
}

func yearRange(value string) bool {
	if len(value) != 4 {
		return false
	}
	year, err := strconv.Atoi(value)
	if err != nil {
		return false
	}
	return year >= 1950 && year <= 2026
}
