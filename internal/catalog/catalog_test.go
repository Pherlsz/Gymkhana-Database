package catalog

import "testing"

func TestFormatCatalogs(t *testing.T) {
	if got := FormatTeam("Tamo nessa por cerveja"); got != "TNC" {
		t.Fatalf("FormatTeam = %q", got)
	}
	if got := FormatSector("maquiagem"); got != "Artística" {
		t.Fatalf("FormatSector = %q", got)
	}
	club := FormatClub("internacioal")
	if club.Club != "Internacional" {
		t.Fatalf("FormatClub = %#v", club)
	}
	if got := FormatClub("tiradentes"); got != (Club{}) {
		t.Fatalf("tiradentes must be empty, got %#v", got)
	}
	if got := FormatHealthPlan("Sim Unimed 12345"); got != "Unimed" {
		t.Fatalf("FormatHealthPlan = %q", got)
	}
	if got := FormatCollection("nenhuma"); got != "" {
		t.Fatalf("FormatCollection nenhum = %q", got)
	}
	if got := FormatAnimal("sim 1 poodle"); got != "Cachorro" {
		t.Fatalf("FormatAnimal = %q", got)
	}
	if got := FormatMembershipType("só cadastro"); got != "cadastro" {
		t.Fatalf("FormatMembershipType = %q", got)
	}
}

func TestFormatVehicle(t *testing.T) {
	if got := FormatVehiclePlate("UNO2010"); got != "" {
		t.Fatalf("UNO2010 must not be a plate, got %q", got)
	}
	if got := FormatVehicleModel("UNO2010"); got != "Uno" {
		t.Fatalf("FormatVehicleModel = %q", got)
	}
	if got := FormatVehicleYear("ANO2002"); got != "2002" {
		t.Fatalf("FormatVehicleYear = %q", got)
	}
	if got := FormatVehiclePlate("HYP1983"); got != "HYP1983" {
		t.Fatalf("real plate = %q", got)
	}
}
