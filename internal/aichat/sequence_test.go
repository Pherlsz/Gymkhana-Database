package aichat

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	querydomain "github.com/Pherlsz/Gymkhana-Database/internal/queryengine"
)

func TestLetterChainFollowsTheAlphabetAndOneNumberDirection(t *testing.T) {
	items := []letterItem{
		{id: "a", letter: 'A', number: 410, holder: "Ana Maria Santos"},
		{id: "b", letter: 'B', number: 590, holder: "Bento Carvalho"},
		{id: "c", letter: 'C', number: 683, holder: "Carmem Machado da Costa"},
		{id: "e", letter: 'E', number: 700, holder: "Eduardo"},
		{id: "f", letter: 'F', number: 710, holder: "Fernanda"},
		{id: "g", letter: 'G', number: 720, holder: "Gustavo"},
		{id: "h", letter: 'H', number: 730, holder: "Helena"},
	}
	chain, direction := bestLetterChain(items)
	if direction != "crescentes" || len(chain) != 4 || chain[0].letter != 'E' || chain[3].letter != 'H' {
		t.Fatalf("gap chain = %q %d %c", direction, len(chain), chainLetter(chain, 0))
	}

	decreasing := []letterItem{
		{id: "a", letter: 'A', number: 1892, holder: "Ana"},
		{id: "b", letter: 'B', number: 1420, holder: "Bento"},
		{id: "c", letter: 'C', number: 683, holder: "Carmem"},
	}
	chain, direction = bestLetterChain(decreasing)
	if direction != "decrescentes" || len(chain) != 3 || chain[0].number != 1892 || chain[2].number != 683 {
		t.Fatalf("decreasing chain = %q %#v", direction, chain)
	}

	if letter, ok := firstLetter("Ágata Lima"); !ok || letter != 'A' {
		t.Fatalf("firstLetter(Ágata) = %c %v", letter, ok)
	}
	if number, ok := houseNumber("Rua Rio Branco, 410, CEP 93110-060"); !ok || number != 410 {
		t.Fatalf("houseNumber = %d %v", number, ok)
	}
	if cityFromAddress("Rua Rio Branco, 410") != "" {
		t.Fatal("street without a city was parsed as a city")
	}
	if cityFromAddress("Rua Rio Branco, 410, Butiá - RS") != "Butiá" {
		t.Fatalf("city = %q", cityFromAddress("Rua Rio Branco, 410, Butiá - RS"))
	}
	if number, ok := digitsValue("12.345-6"); !ok || number != 123456 {
		t.Fatalf("digitsValue = %d %v", number, ok)
	}
	if _, ok := digitsValue("sem numero"); ok {
		t.Fatal("digitsValue accepted a field without digits")
	}
}

func TestSequenceToolReturnsTheChainNotASample(t *testing.T) {
	actor, _ := chatTestActor(t, "member")
	now := time.Date(2026, time.September, 21, 18, 0, 0, 0, time.UTC)
	query := &fakeToolQuery{scan: querydomain.TextScan{Rows: []querydomain.TextScanRow{
		{ID: "1", Values: []string{"Ana Maria Santos", "Rua Rio Branco, 410"}},
		{ID: "2", Values: []string{"Bento Carvalho", "Avenida Brasil, 590"}},
		{ID: "3", Values: []string{"Carmem Machado da Costa", "Rua Sete de Setembro, 683"}},
	}}}
	gateway, _ := NewToolGateway(&fakeToolSearch{}, query, &fakeToolReferences{}, func() time.Time { return now })
	arguments := `{"catalog_version":"` + strings.Repeat("a", 64) + `","root_entity":"bills","letter_field":"bill.printed_holder_name","number_field":"bill.printed_address","minimum_length":10,"filter":{"kind":"predicate","field":"bill.medium","operator":"eq","values":["PHYSICAL"]}}`
	output, err := gateway.Execute(context.Background(), actor, nil, Identifier{21}, 1,
		ToolCall{ID: "seq-call", Name: "sequencia", Arguments: json.RawMessage(arguments)}, now.Add(time.Hour), "request")
	if err != nil {
		t.Fatalf("Execute(sequencia) error = %v", err)
	}
	payload := string(output.Payload)
	if output.Kind != ToolQuery || output.Reference != nil || output.RowCount != 3 || query.limit != querydomain.MaximumSequenceScan {
		t.Fatalf("output = %#v limit=%d", output, query.limit)
	}
	if !strings.Contains(payload, `"length":3`) || !strings.Contains(payload, "Não atende o mínimo de 10") ||
		!strings.Contains(payload, "Ana Maria Santos") || strings.Contains(payload, "SELECT") {
		t.Fatalf("payload = %s", payload)
	}
	if query.plan.RootEntity != "bills" || len(query.plan.Projections) != 2 {
		t.Fatalf("scan plan = %#v", query.plan)
	}
}

func TestSequenceToolReadsDigitsFromAnyEntity(t *testing.T) {
	actor, _ := chatTestActor(t, "member")
	now := time.Date(2026, time.September, 21, 21, 0, 0, 0, time.UTC)
	query := &fakeToolQuery{scan: querydomain.TextScan{Rows: []querydomain.TextScanRow{
		{ID: "1", Values: []string{"Ana Maria", "410"}},
		{ID: "2", Values: []string{"Bento", "590"}},
		{ID: "3", Values: []string{"Diego", "700"}},
	}}}
	gateway, _ := NewToolGateway(&fakeToolSearch{}, query, &fakeToolReferences{}, func() time.Time { return now })
	arguments := `{"catalog_version":"` + strings.Repeat("c", 64) + `","root_entity":"profiles","letter_field":"profile.full_name","number_field":"profile.document_number","number_extractor":"digitos","minimum_length":2,"filter":{"kind":"predicate","field":"profile.full_name","operator":"contains","values":["a"]}}`
	output, err := gateway.Execute(context.Background(), actor, nil, Identifier{31}, 1,
		ToolCall{ID: "people-chain", Name: "sequencia", Arguments: json.RawMessage(arguments)}, now.Add(time.Hour), "request")
	if err != nil {
		t.Fatalf("Execute(sequencia people) error = %v", err)
	}
	payload := string(output.Payload)
	if !strings.Contains(payload, `"length":2`) || !strings.Contains(payload, "Ana Maria") || strings.Contains(payload, "Diego") {
		t.Fatalf("people chain payload = %s", payload)
	}
	if query.plan.RootEntity != "profiles" {
		t.Fatalf("root = %s", query.plan.RootEntity)
	}
}

func chainLetter(chain []letterItem, index int) rune {
	if index >= len(chain) {
		return 0
	}
	return chain[index].letter
}
