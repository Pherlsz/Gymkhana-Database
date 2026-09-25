package aichat

import (
	"errors"
	"testing"
)

func TestCompileTableRecorteMapsGridFiltersAndHiddenColumns(t *testing.T) {
	logical := []byte(`{"plan":{"version":"v1","catalog_version":"` + "a" + `","root_entity":"profiles","projections":["profile.full_name","profile.father_name"],"filter":{"kind":"group","conjunction":"AND","children":[{"kind":"predicate","field":"profile.full_name","operator":"contains","values":["Pedro"]},{"kind":"predicate","field":"profile.address_state","operator":"eq","values":["rs"]}]},"sort":[{"field":"profile.full_name","direction":"desc"}]}}`)
	got, err := CompileTableRecorte(ResultReferenceQuery, logical)
	if err != nil || got.Table != "people" || got.Filters["full_name"] != "Pedro" || got.Filters["state"] != "RS" ||
		len(got.Columns) != 2 || got.Columns[1] != "father_name" || got.Sort != "full_name" || got.Order != "desc" {
		t.Fatalf("recorte = %#v, err = %v", got, err)
	}
}

func TestCompileTableRecorteRejectsWhatTheGridCannotExpress(t *testing.T) {
	cases := [][]byte{
		[]byte(`{"plan":{"version":"v1","root_entity":"profiles","projections":["profile.full_name"],"filter":{"kind":"relation","relation":"profile.bills","children":[{"kind":"predicate","field":"bill.medium","operator":"eq","values":["PHYSICAL"]}]}}}`),
		[]byte(`{"plan":{"version":"v1","root_entity":"profiles","projections":["profile.full_name"],"filter":{"kind":"predicate","field":"profile.full_name","operator":"eq","values":["Ana"]}}}`),
		[]byte(`{"plan":{"version":"v2","root_entity":"profiles","projections":["profile.full_name"],"group_by":["profile.address_city"],"filter":{"kind":"predicate","field":"profile.full_name","operator":"contains","values":["Ana"]}}}`),
		[]byte(`{"plan":{"version":"v1","root_entity":"bills","projections":["bill.reference"],"filter":{"kind":"predicate","field":"bill.medium","operator":"eq","values":["PAPER"]}}}`),
	}
	for _, logical := range cases {
		if _, err := CompileTableRecorte(ResultReferenceQuery, logical); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("logical %s err = %v", logical, err)
		}
	}
	if _, err := CompileTableRecorte(ResultReferenceSearch, []byte(`{"q":"Ana"}`)); !errors.Is(err, ErrInvalidInput) {
		t.Fatal("search reference compiled")
	}
}

func TestCompileTableRecorteMapsBillMedium(t *testing.T) {
	logical := []byte(`{"plan":{"version":"v1","root_entity":"bills","projections":["bill.printed_holder_name"],"filter":{"kind":"predicate","field":"bill.medium","operator":"eq","values":["physical"]}}}`)
	got, err := CompileTableRecorte(ResultReferenceQuery, logical)
	if err != nil || got.Table != "bills" || got.Filters["bill_medium"] != "PHYSICAL" || got.Columns[0] != "printed_holder_name" {
		t.Fatalf("recorte = %#v, err = %v", got, err)
	}
}
