package operations

import "testing"

func TestCoerceExcelCPFScientificNotation(t *testing.T) {
	got := coerceExcelCPF("8.391234567e+10")
	if got != "83912345670" {
		t.Fatalf("coerceExcelCPF() = %q", got)
	}
	if coerceExcelCPF("529.982.247-25") != "529.982.247-25" {
		t.Fatal("formatted CPF must stay intact")
	}
}

func TestCoerceCivilDateSlashAndSerial(t *testing.T) {
	if got := coerceCivilDate("15/03/1985"); got != "1985-03-15" {
		t.Fatalf("slash date = %q", got)
	}
	if got := coerceCivilDate("1985-03-15"); got != "1985-03-15" {
		t.Fatalf("iso date = %q", got)
	}
	serial, err := excelCivilDate("31141", false)
	if err != nil {
		t.Fatal(err)
	}
	if got := coerceCivilDate("31141"); got != serial {
		t.Fatalf("excel serial = %q want %q", got, serial)
	}
}

func TestParseOptionalBoolPortuguese(t *testing.T) {
	yes, err := parseOptionalBool("Sim")
	if err != nil || yes == nil || !*yes {
		t.Fatalf("sim = %v %v", yes, err)
	}
	no, err := parseOptionalBool("Não")
	if err != nil || no == nil || *no {
		t.Fatalf("nao = %v %v", no, err)
	}
	empty, err := parseOptionalBool("")
	if err != nil || empty != nil {
		t.Fatalf("empty = %v %v", empty, err)
	}
}

func TestProfileValuesFromImportSplitsPackedCells(t *testing.T) {
	values, err := profileValuesFromImport(map[string]string{
		"full_name":      "Fixture Batch Silva",
		"address_street": "Rua Exemplo 100",
		"team":           "TNC / Rua",
		"vehicle_model":  "Uno, Prata, HYP1983, 2014",
	})
	if err != nil {
		t.Fatal(err)
	}
	if values.Address.Street != "Rua Exemplo" || values.Address.Number != "100" {
		t.Fatalf("address = %+v", values.Address)
	}
	if values.Team != "TNC" || values.Sector != "Rua" {
		t.Fatalf("team=%q sector=%q", values.Team, values.Sector)
	}
	if values.VehicleModel != "Uno" || values.VehicleColor != "Prata" || values.VehiclePlate != "HYP1983" {
		t.Fatalf("vehicle = %q %q %q", values.VehicleModel, values.VehicleColor, values.VehiclePlate)
	}
	if values.VehicleYear == nil || *values.VehicleYear != 2014 {
		t.Fatalf("year = %v", values.VehicleYear)
	}
}
