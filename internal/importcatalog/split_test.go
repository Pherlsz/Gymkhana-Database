package importcatalog

import "testing"

func TestSplitPackedAddressPeelsHouseNumber(t *testing.T) {
	values := map[string]string{"address_street": "Rua Exemplo 100"}
	SplitPackedValues(values)
	if values["address_street"] != "Rua Exemplo" || values["address_number"] != "100" {
		t.Fatalf("packed address = %#v", values)
	}
}

func TestSplitPackedAddressLeavesNumberedStreetAloneWhenNumberFilled(t *testing.T) {
	values := map[string]string{"address_street": "Rua Exemplo 100", "address_number": "12"}
	SplitPackedValues(values)
	if values["address_street"] != "Rua Exemplo 100" || values["address_number"] != "12" {
		t.Fatalf("must not overwrite number: %#v", values)
	}
}

func TestSplitPackedTeamSector(t *testing.T) {
	values := map[string]string{"team": "TNC / Rua"}
	SplitPackedValues(values)
	if values["team"] != "TNC" || values["sector"] != "Rua" {
		t.Fatalf("team/sector = %#v", values)
	}
}

func TestSplitPackedVehicle(t *testing.T) {
	values := map[string]string{"vehicle_model": "Uno, Prata, HYP1983, 2014"}
	SplitPackedValues(values)
	if values["vehicle_model"] != "Uno" {
		t.Fatalf("model = %q", values["vehicle_model"])
	}
	if values["vehicle_color"] != "Prata" {
		t.Fatalf("color = %q", values["vehicle_color"])
	}
	if values["vehicle_plate"] != "HYP1983" {
		t.Fatalf("plate = %q", values["vehicle_plate"])
	}
	if values["vehicle_year"] != "2014" {
		t.Fatalf("year = %q", values["vehicle_year"])
	}
}

func TestSplitIdentifierDate(t *testing.T) {
	id, date := SplitIdentifierDate("12345678900 12/03/2010")
	if id != "12345678900" || date != "12/03/2010" {
		t.Fatalf("got %q %q", id, date)
	}
	plain, empty := SplitIdentifierDate("12345678900")
	if plain != "12345678900" || empty != "" {
		t.Fatalf("plain = %q %q", plain, empty)
	}
}

func TestCollectDocumentSidecarsPeelsCNHDate(t *testing.T) {
	got := CollectDocumentSidecars(map[string]string{
		DocumentFieldPrefix + "cnh": "12345678900, 12/03/2010",
	})
	if len(got) != 1 || got[0].TypeKey != "cnh" || got[0].Identifier != "12345678900" || got[0].Date != "12/03/2010" {
		t.Fatalf("sidecar = %#v", got)
	}
}
