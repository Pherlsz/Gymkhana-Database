package archiveimport

import (
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Pherlsz/Gymkhana-Database/internal/profile"
)

const requiredDevProjectID = "damp-fog-66470386"

type Presence struct {
	TypeKey    string
	Claim      string
	Identifier string
}

type Record struct {
	Values     profile.Values
	Presences  []Presence
	Skip       bool
	SkipReason string
}

func MapRow(header []string, cells []string) Record {
	get := func(name string) string {
		for i, h := range header {
			if h == name && i < len(cells) {
				return strings.TrimSpace(cells[i])
			}
		}
		return ""
	}

	name := get("nome")
	if name == "" {
		return Record{Skip: true, SkipReason: "missing_name"}
	}

	landline := get("residencial")
	if landline == "" {
		landline = get("fone_comercial")
	}

	club := get("socio_clube")
	if strings.EqualFold(club, "Outro") {
		if extra := get("socio_clube_outro"); extra != "" {
			club = extra
		} else {
			club = "Outros"
		}
	}

	values := profile.Values{
		FullName:        name,
		Email:           get("email"),
		MobilePhone:     get("celular"),
		LandlinePhone:   landline,
		BirthDate:       civilDate(get("data_nascimento")),
		Gender:          get("sexo"),
		BloodType:       get("tipo_sanguineo"),
		Nationality:     get("nacionalidade"),
		BirthCity:       get("cidade_nascimento"),
		MaritalStatus:   get("estado_civil"),
		WeddingDate:     civilDate(get("casamento")),
		FatherName:      get("pai"),
		FatherBirthDate: civilDate(get("nasc_pai")),
		MotherName:      get("mae"),
		MotherBirthDate: civilDate(get("nasc_mae")),
		HealthPlan:      get("plano_saude"),
		BloodDonor:      yesNo(get("doador_sangue")),
		OrganDonor:      yesNo(get("doador_orgaos")),
		Team:            get("equipe"),
		Sector:          get("setor"),
		Collections:     get("colecao"),
		VehicleModel:    get("veiculo_modelo"),
		VehicleColor:    get("veiculo_cor"),
		VehiclePlate:    get("veiculo_placa"),
		VehicleYear:     year(get("veiculo_ano")),
		ClubMembership:  club,
		MembershipType:  get("membership_type"),
		PlaceOfOrigin:   get("naturalidade"),
		BirthCountry:    get("pais_nascimento"),
		ParentsWedding:  civilDate(get("casamento_pais")),
		SupermarketClub: get("clube_supermercado"),
		Pet:             get("animal"),
		TravelCountries: get("viagem"),
		CardBrand:       get("cartao_bandeira"),
		CardBank:        get("cartao_banco"),
		Address: profile.Address{
			Street:       get("endereco"),
			Number:       get("numero"),
			Complement:   composeComplement(get("bloco"), get("apto"), get("predio"), get("complemento")),
			Neighborhood: get("bairro"),
			City:         get("cidade_reside"),
			State:        get("uf"),
			PostalCode:   get("cep"),
		},
	}

	presences := collectPresences(get)
	if claim, number := get("cpf_presenca"), get("cpf"); claim == "informed_number" && number != "" {
		values.CPF = number
	}

	return Record{Values: values, Presences: presences}
}

func collectPresences(get func(string) string) []Presence {
	specs := []struct {
		key, claimCol, numberCol string
	}{
		{"cpf", "cpf_presenca", "cpf"},
		{"rg", "rg_presenca", "rg"},
		{"cnh", "cnh_presenca", "cnh"},
		{"voter_id", "titulo_presenca", "titulo"},
		{"sus_card", "sus_presenca", "sus"},
		{"ctps", "ctps_presenca", "ctps"},
		{"citizen_card", "cartao_cidadao_presenca", "cartao_cidadao"},
		{"passport", "passaporte_presenca", "passaporte"},
		{"tri", "tri_presenca", "tri"},
		{"teu", "teu_presenca", "teu"},
	}
	var out []Presence
	for _, spec := range specs {
		claim := get(spec.claimCol)
		if claim != "absence" && claim != "indication" && claim != "informed_number" {
			continue
		}
		identifier := ""
		if claim == "informed_number" {
			identifier = get(spec.numberCol)
			if identifier == "" {
				continue
			}
		}
		out = append(out, Presence{TypeKey: spec.key, Claim: claim, Identifier: identifier})
	}
	formacaoClaim := get("formacao_presenca")
	if formacaoClaim == "absence" || formacaoClaim == "indication" || formacaoClaim == "informed_number" {
		key := formationType(get("formacao_tipo"))
		if key != "" {
			identifier := ""
			if formacaoClaim == "informed_number" {
				identifier = get("formacao_numero")
				if identifier == "" {
					return out
				}
			}
			out = append(out, Presence{TypeKey: key, Claim: formacaoClaim, Identifier: identifier})
		}
	}
	return out
}

func formationType(tipo string) string {
	switch strings.ToLower(strings.TrimSpace(tipo)) {
	case "oab":
		return "oab"
	case "crea":
		return "crea"
	case "coren":
		return "coren"
	case "crm":
		return "crm"
	case "cro":
		return "cro"
	case "cref":
		return "cref"
	case "drt":
		return ""
	case "estudante":
		return "student_id"
	default:
		return ""
	}
}

func NormalizeRecord(record Record) (Record, []string) {
	if record.Skip {
		return record, nil
	}
	normalized, err := profile.Normalize(record.Values)
	if err == nil {
		record.Values = normalized
		syncNormalizedIdentifiers(&record)
		return record, nil
	}
	validation, ok := err.(*profile.ValidationError)
	if !ok {
		record.Skip = true
		record.SkipReason = "normalize"
		return record, nil
	}
	dropped := make([]string, 0, len(validation.Fields))
	for _, field := range validation.Fields {
		dropped = append(dropped, field.Field+":"+field.Code)
		clearField(&record.Values, field.Field)
	}
	normalized, err = profile.Normalize(record.Values)
	if err != nil {
		record.Skip = true
		record.SkipReason = "normalize"
		return record, dropped
	}
	record.Values = normalized
	syncNormalizedIdentifiers(&record)
	return record, dropped
}

func syncNormalizedIdentifiers(record *Record) {
	filtered := record.Presences[:0]
	for _, item := range record.Presences {
		if item.TypeKey == "cpf" && item.Claim == "informed_number" {
			if record.Values.CPF == "" {
				continue
			}
			item.Identifier = record.Values.CPF
		}
		filtered = append(filtered, item)
	}
	record.Presences = filtered
}

func clearField(values *profile.Values, field string) {
	switch field {
	case "full_name":
		values.FullName = ""
	case "email":
		values.Email = ""
	case "mobile_phone":
		values.MobilePhone = ""
	case "landline_phone":
		values.LandlinePhone = ""
	case "cpf":
		values.CPF = ""
	case "address.street":
		values.Address.Street = ""
	case "address.number":
		values.Address.Number = ""
	case "address.complement":
		values.Address.Complement = ""
	case "address.neighborhood":
		values.Address.Neighborhood = ""
	case "address.city":
		values.Address.City = ""
	case "address.state":
		values.Address.State = ""
	case "address.postal_code":
		values.Address.PostalCode = ""
	case "birth_date":
		values.BirthDate = ""
	case "wedding_date":
		values.WeddingDate = ""
	case "father_birth_date":
		values.FatherBirthDate = ""
	case "mother_birth_date":
		values.MotherBirthDate = ""
	case "gender":
		values.Gender = ""
	case "father_name":
		values.FatherName = ""
	case "mother_name":
		values.MotherName = ""
	case "place_of_origin":
		values.PlaceOfOrigin = ""
	case "birth_country":
		values.BirthCountry = ""
	case "parents_wedding_date":
		values.ParentsWedding = ""
	case "supermarket_club":
		values.SupermarketClub = ""
	case "pet":
		values.Pet = ""
	case "travel_countries":
		values.TravelCountries = ""
	case "card_brand":
		values.CardBrand = ""
	case "card_bank":
		values.CardBank = ""
	}
}

func civilDate(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if parsed, err := time.Parse("2006-01-02", value); err == nil {
		return parsed.Format("2006-01-02")
	}
	if parsed, err := time.Parse("02/01/2006", value); err == nil {
		return parsed.Format("2006-01-02")
	}
	return ""
}

func yesNo(value string) *bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "sim":
		v := true
		return &v
	case "não", "nao":
		v := false
		return &v
	default:
		return nil
	}
}

func year(value string) *int32 {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return nil
	}
	year := int32(parsed)
	return &year
}

func composeComplement(bloco, apto, predio, complemento string) string {
	parts := make([]string, 0, 4)
	for _, part := range []string{bloco, apto, predio, complemento} {
		part = strings.TrimSpace(part)
		if part != "" {
			parts = append(parts, part)
		}
	}
	joined := strings.Join(parts, ", ")
	if utf8.RuneCountInString(joined) <= profile.MaxComplementLength {
		return joined
	}
	parts = []string{}
	for _, part := range []string{bloco, apto, complemento} {
		part = strings.TrimSpace(part)
		if part != "" {
			parts = append(parts, part)
		}
	}
	joined = strings.Join(parts, ", ")
	runes := []rune(joined)
	if len(runes) > profile.MaxComplementLength {
		return string(runes[:profile.MaxComplementLength])
	}
	return joined
}
