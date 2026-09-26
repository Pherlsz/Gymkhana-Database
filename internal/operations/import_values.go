package operations

import (
	"strconv"
	"strings"
	"time"

	"github.com/Pherlsz/Gymkhana-Core/normalize"
	"github.com/Pherlsz/Gymkhana-Database/internal/catalog"
	"github.com/Pherlsz/Gymkhana-Database/internal/importcatalog"
	"github.com/Pherlsz/Gymkhana-Database/internal/profile"
)

func profileValuesFromImport(effective map[string]string) (profile.Values, error) {
	fields := cloneStringMap(effective)
	importcatalog.SplitPackedValues(fields)
	bloodDonor, err := parseOptionalBool(fields["blood_donor"])
	if err != nil {
		return profile.Values{}, err
	}
	organDonor, err := parseOptionalBool(fields["organ_donor"])
	if err != nil {
		return profile.Values{}, err
	}
	vehicleYear, err := parseOptionalYear(fields["vehicle_year"])
	if err != nil {
		return profile.Values{}, err
	}
	return profile.Values{
		FullName:      fields["full_name"],
		SocialName:    fields["social_name"],
		CPF:           coerceExcelCPF(fields["cpf"]),
		Email:         fields["email"],
		MobilePhone:   fields["mobile_phone"],
		LandlinePhone: fields["landline_phone"],
		Address: profile.Address{
			Street:       fields["address_street"],
			Number:       fields["address_number"],
			Complement:   fields["address_complement"],
			Neighborhood: fields["address_neighborhood"],
			City:         fields["address_city"],
			State:        fields["address_state"],
			PostalCode:   fields["address_postal_code"],
		},
		Notes:           fields["notes"],
		BirthDate:       coerceCivilDate(fields["birth_date"]),
		Gender:          firstNonEmpty(normalize.FormatGender(fields["gender"]), fields["gender"]),
		BloodType:       firstNonEmpty(normalize.FormatBloodType(fields["blood_type"]), fields["blood_type"]),
		Nationality:     fields["nationality"],
		BirthCity:       fields["birth_city"],
		MaritalStatus:   firstNonEmpty(normalize.FormatMaritalStatus(fields["marital_status"]), fields["marital_status"]),
		WeddingDate:     coerceCivilDate(fields["wedding_date"]),
		FatherName:      fields["father_name"],
		FatherBirthDate: coerceCivilDate(fields["father_birth_date"]),
		MotherName:      fields["mother_name"],
		MotherBirthDate: coerceCivilDate(fields["mother_birth_date"]),
		HealthPlan:      firstNonEmpty(catalog.FormatHealthPlan(fields["health_plan"]), fields["health_plan"]),
		BloodDonor:      bloodDonor,
		OrganDonor:      organDonor,
		Team:            firstNonEmpty(catalog.FormatTeam(fields["team"]), fields["team"]),
		Sector:          firstNonEmpty(catalog.FormatSector(fields["sector"]), fields["sector"]),
		Collections:     fields["collections"],
		VehicleModel:    fields["vehicle_model"],
		VehicleColor:    fields["vehicle_color"],
		VehiclePlate:    fields["vehicle_plate"],
		VehicleYear:     vehicleYear,
		ClubMembership:  formatClubMembership(fields["club_membership"]),
		MembershipType:  firstNonEmpty(catalog.FormatMembershipType(fields["membership_type"]), fields["membership_type"]),
		PlaceOfOrigin:   fields["place_of_origin"],
		BirthCountry:    fields["birth_country"],
		ParentsWedding:  coerceCivilDate(fields["parents_wedding_date"]),
		SupermarketClub: fields["supermarket_club"],
		Pet:             fields["pet"],
		TravelCountries: fields["travel_countries"],
		CardBrand:       fields["card_brand"],
		CardBank:        fields["card_bank"],
	}, nil
}

func cloneStringMap(values map[string]string) map[string]string {
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func formatClubMembership(raw string) string {
	club := catalog.FormatClub(raw)
	if club.Club != "" {
		return club.Club
	}
	return raw
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func coerceExcelCPF(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	folded := strings.ToLower(trimmed)
	if looksLikeScientific(folded) {
		parsed, err := strconv.ParseFloat(trimmed, 64)
		if err == nil && parsed > 0 {
			return strconv.FormatInt(int64(parsed+0.5), 10)
		}
	}
	return trimmed
}

func looksLikeScientific(value string) bool {
	hasDigit := false
	hasE := false
	for _, character := range value {
		switch {
		case character >= '0' && character <= '9':
			hasDigit = true
		case character == 'e':
			hasE = true
		case character == '+' || character == '-' || character == '.':
			continue
		default:
			return false
		}
	}
	return hasDigit && hasE
}

func coerceCivilDate(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if _, err := time.Parse("2006-01-02", trimmed); err == nil {
		return trimmed
	}
	if parsed, err := time.Parse("02/01/2006", trimmed); err == nil {
		return parsed.Format("2006-01-02")
	}
	if parsed, err := time.Parse("2/1/2006", trimmed); err == nil {
		return parsed.Format("2006-01-02")
	}
	if _, err := strconv.ParseFloat(trimmed, 64); err == nil && !strings.ContainsAny(trimmed, "/-") {
		if parsed, err := strconv.ParseFloat(trimmed, 64); err == nil && parsed >= 20000 && parsed < 80000 {
			civil, err := excelCivilDate(trimmed, false)
			if err == nil {
				return civil
			}
		}
	}
	return trimmed
}

func parseOptionalBool(raw string) (*bool, error) {
	key := normalize.SearchText(raw)
	if key == "" {
		return nil, nil
	}
	switch key {
	case "sim", "s", "true", "1", "yes", "y", "verdadeiro":
		value := true
		return &value, nil
	case "nao", "n", "false", "0", "no", "falso":
		value := false
		return &value, nil
	default:
		if fields := strings.Fields(key); len(fields) > 0 {
			switch fields[0] {
			case "sim", "s", "true", "yes", "y", "verdadeiro":
				value := true
				return &value, nil
			case "nao", "n", "false", "no", "falso":
				value := false
				return &value, nil
			}
		}
		return nil, ErrInvalidInput
	}
}

func parseOptionalYear(raw string) (*int32, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return nil, ErrInvalidInput
	}
	year := int32(parsed)
	return &year, nil
}

func nullableBool(value *bool) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableInt32(value *int32) any {
	if value == nil {
		return nil
	}
	return *value
}

func profileSQLArgs(values *profile.Values) []any {
	return []any{
		values.FullName,
		nullableString(values.SocialName),
		nullableString(values.Email),
		nullableString(values.MobilePhone),
		nullableString(values.LandlinePhone),
		nullableString(values.Address.Street),
		nullableString(values.Address.Number),
		nullableString(values.Address.Complement),
		nullableString(values.Address.Neighborhood),
		nullableString(values.Address.City),
		nullableString(values.Address.State),
		nullableString(values.Address.PostalCode),
		nullableString(values.Notes),
		nullableDate(values.BirthDate),
		nullableString(values.Gender),
		nullableString(values.BloodType),
		nullableString(values.Nationality),
		nullableString(values.BirthCity),
		nullableString(values.MaritalStatus),
		nullableDate(values.WeddingDate),
		nullableString(values.FatherName),
		nullableDate(values.FatherBirthDate),
		nullableString(values.MotherName),
		nullableDate(values.MotherBirthDate),
		nullableString(values.HealthPlan),
		nullableBool(values.BloodDonor),
		nullableBool(values.OrganDonor),
		nullableString(values.Team),
		nullableString(values.Sector),
		nullableString(values.Collections),
		nullableString(values.VehicleModel),
		nullableString(values.VehicleColor),
		nullableString(values.VehiclePlate),
		nullableInt32(values.VehicleYear),
		nullableString(values.ClubMembership),
		nullableString(values.MembershipType),
		nullableString(values.PlaceOfOrigin),
		nullableString(values.BirthCountry),
		nullableDate(values.ParentsWedding),
		nullableString(values.SupermarketClub),
		nullableString(values.Pet),
		nullableString(values.TravelCountries),
		nullableString(values.CardBrand),
		nullableString(values.CardBank),
	}
}
