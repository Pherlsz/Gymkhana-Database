package archiveimport

import (
	"strings"
	"testing"

	"github.com/Pherlsz/Gymkhana-Database/internal/profile"
)

func TestMapRowSkipsMissingName(t *testing.T) {
	record := MapRow([]string{"nome", "cpf"}, []string{"", "39053344705"})
	if !record.Skip || record.SkipReason != "missing_name" {
		t.Fatalf("skip = %#v", record)
	}
}

func TestMapRowConvertsCivilDateAndComplement(t *testing.T) {
	header := []string{
		"nome", "data_nascimento", "bloco", "apto", "complemento",
		"cpf", "cpf_presenca", "rg", "rg_presenca", "cnh", "cnh_presenca",
		"socio_clube", "doador_sangue", "residencial", "fone_comercial",
		"naturalidade", "pais_nascimento", "casamento_pais", "clube_supermercado",
		"animal", "viagem", "cartao_bandeira", "cartao_banco",
	}
	cells := []string{
		"Maria da Silva", "15/04/1980", "bl. A", "apt. 202", "fundos",
		"39053344705", "informed_number", "", "discard", "", "indication",
		"Outro", "Sim", "", "+55 (51) 3333-4444",
		"Pelotas", "Brasil", "01/05/1960", "Nacional",
		"Cachorro", "Argentina", "Visa", "BB",
	}
	record := MapRow(header, cells)
	if record.Skip {
		t.Fatalf("skipped: %s", record.SkipReason)
	}
	if record.Values.BirthDate != "1980-04-15" {
		t.Fatalf("birth date = %q", record.Values.BirthDate)
	}
	if record.Values.Address.Complement != "bl. A, apt. 202, fundos" {
		t.Fatalf("complement = %q", record.Values.Address.Complement)
	}
	if record.Values.ClubMembership != "Outros" {
		t.Fatalf("club = %q", record.Values.ClubMembership)
	}
	if record.Values.PlaceOfOrigin != "Pelotas" {
		t.Fatalf("place of origin = %q", record.Values.PlaceOfOrigin)
	}
	if record.Values.BirthCountry != "Brasil" {
		t.Fatalf("birth country = %q", record.Values.BirthCountry)
	}
	if record.Values.ParentsWedding != "1960-05-01" {
		t.Fatalf("parents wedding = %q", record.Values.ParentsWedding)
	}
	if record.Values.SupermarketClub != "Nacional" {
		t.Fatalf("supermarket = %q", record.Values.SupermarketClub)
	}
	if record.Values.Pet != "Cachorro" {
		t.Fatalf("pet = %q", record.Values.Pet)
	}
	if record.Values.TravelCountries != "Argentina" {
		t.Fatalf("travel = %q", record.Values.TravelCountries)
	}
	if record.Values.CardBrand != "Visa" {
		t.Fatalf("card brand = %q", record.Values.CardBrand)
	}
	if record.Values.CardBank != "BB" {
		t.Fatalf("card bank = %q", record.Values.CardBank)
	}
	if record.Values.LandlinePhone != "+55 (51) 3333-4444" {
		t.Fatalf("landline = %q", record.Values.LandlinePhone)
	}
	if record.Values.BloodDonor == nil || !*record.Values.BloodDonor {
		t.Fatal("blood donor")
	}
	if record.Values.CPF != "39053344705" {
		t.Fatalf("cpf = %q", record.Values.CPF)
	}
	var claims []string
	for _, presence := range record.Presences {
		claims = append(claims, presence.TypeKey+":"+presence.Claim)
	}
	joined := strings.Join(claims, ",")
	if !strings.Contains(joined, "cpf:informed_number") || !strings.Contains(joined, "cnh:indication") {
		t.Fatalf("presences = %v", claims)
	}
	if strings.Contains(joined, "rg:") {
		t.Fatalf("discarded rg leaked: %v", claims)
	}
}

func TestNormalizeRecordDropsInvalidOptionalPhone(t *testing.T) {
	record := Record{Values: profile.Values{FullName: "Ana Teste", MobilePhone: "not-a-phone"}}
	got, dropped := NormalizeRecord(record)
	if got.Skip {
		t.Fatalf("skipped after drop: %s", got.SkipReason)
	}
	if got.Values.MobilePhone != "" {
		t.Fatalf("mobile = %q", got.Values.MobilePhone)
	}
	if len(dropped) == 0 {
		t.Fatal("expected dropped field")
	}
}

func TestFormationType(t *testing.T) {
	if formationType("OAB") != "oab" {
		t.Fatalf("oab = %q", formationType("OAB"))
	}
	if formationType("estudante") != "student_id" {
		t.Fatalf("student = %q", formationType("estudante"))
	}
	if formationType("formacao") != "" {
		t.Fatalf("formacao = %q", formationType("formacao"))
	}
}
