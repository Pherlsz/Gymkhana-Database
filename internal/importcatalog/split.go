package importcatalog

import (
	"strings"
	"unicode"

	"github.com/Pherlsz/Gymkhana-Core/normalize"
	"github.com/Pherlsz/Gymkhana-Database/internal/catalog"
)

// SplitPackedValues peels mold-specific compound cells into sibling fields.
// Unrecognized leftovers stay on the original field (notes/unmapped stay as-is).
func SplitPackedValues(values map[string]string) {
	if values == nil {
		return
	}
	splitPackedAddress(values)
	splitPackedTeamSector(values)
	splitPackedVehicle(values)
}

func SplitIdentifierDate(raw string) (identifier, date string) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", ""
	}
	parts := strings.FieldsFunc(trimmed, func(r rune) bool {
		return unicode.IsSpace(r) || r == ',' || r == ';'
	})
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if date == "" && (looksLikeSlashDate(part) || looksLikeISODate(part)) {
			date = part
			continue
		}
		kept = append(kept, part)
	}
	if date == "" {
		return trimmed, ""
	}
	return strings.TrimSpace(strings.Join(kept, " ")), date
}

func splitPackedAddress(values map[string]string) {
	if strings.TrimSpace(values["address_number"]) != "" {
		return
	}
	street := strings.TrimSpace(values["address_street"])
	fields := strings.Fields(street)
	if len(fields) < 2 {
		return
	}
	last := fields[len(fields)-1]
	digits := normalize.Digits(last)
	if digits == "" || len(digits) >= 7 {
		return
	}
	number := normalize.FormatHouseNumber(last)
	if number == "" {
		return
	}
	values["address_street"] = strings.Join(fields[:len(fields)-1], " ")
	values["address_number"] = number
}

func splitPackedTeamSector(values map[string]string) {
	raw := strings.TrimSpace(values["team"])
	if raw == "" {
		return
	}
	team, sector := peelTeamSector(raw)
	if team != "" {
		values["team"] = team
	}
	if sector != "" && strings.TrimSpace(values["sector"]) == "" {
		values["sector"] = sector
	}
}

func peelTeamSector(raw string) (team, sector string) {
	team = catalog.FormatTeam(raw)
	sector = catalog.FormatSector(raw)
	if team != "" && sector != "" {
		return team, sector
	}
	parts := splitCompound(raw)
	if len(parts) < 2 {
		parts = strings.Fields(raw)
	}
	for _, part := range parts {
		if team == "" {
			if got := catalog.FormatTeam(part); got != "" {
				team = got
				continue
			}
		}
		if sector == "" {
			if got := catalog.FormatSector(part); got != "" {
				sector = got
			}
		}
	}
	return team, sector
}

func splitPackedVehicle(values map[string]string) {
	packed := strings.TrimSpace(values["vehicle_model"])
	if packed == "" {
		return
	}
	parts := splitCompound(packed)
	if len(parts) < 2 {
		parts = strings.Fields(packed)
	}
	if len(parts) < 2 {
		return
	}
	if model := catalog.FormatVehicleModel(packed); model != "" {
		values["vehicle_model"] = model
	}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.TrimSpace(values["vehicle_plate"]) == "" {
			if got := catalog.FormatVehiclePlate(part); got != "" {
				values["vehicle_plate"] = got
				continue
			}
		}
		if strings.TrimSpace(values["vehicle_color"]) == "" {
			if got := catalog.FormatVehicleColor(part); got != "" {
				values["vehicle_color"] = got
				continue
			}
		}
		if strings.TrimSpace(values["vehicle_year"]) == "" {
			if got := catalog.FormatVehicleYear(part); got != "" {
				values["vehicle_year"] = got
			}
		}
	}
}

func splitCompound(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '/' || r == '|'
	})
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		if trimmed := strings.TrimSpace(field); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func looksLikeISODate(value string) bool {
	if len(value) < 10 || value[4] != '-' || value[7] != '-' {
		return false
	}
	for _, r := range value[:10] {
		if r != '-' && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
