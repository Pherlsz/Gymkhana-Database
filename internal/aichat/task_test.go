package aichat

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	querydomain "github.com/Pherlsz/Gymkhana-Database/internal/queryengine"
)

func TestTaskSheetRunsCountChainAndUnverifiableRules(t *testing.T) {
	actor, _ := chatTestActor(t, "member")
	now := time.Date(2026, time.September, 21, 21, 30, 0, 0, time.UTC)
	query := &fakeToolQuery{
		catalog: querydomain.Catalog{Version: strings.Repeat("a", 64)},
		scan: querydomain.TextScan{Rows: []querydomain.TextScanRow{
			{ID: "1", Values: []string{"Ana Maria Santos", "Rua Rio Branco, 410"}},
			{ID: "2", Values: []string{"Bento Carvalho", "Avenida Brasil, 590"}},
			{ID: "3", Values: []string{"Carmem Machado da Costa", "Rua Sete de Setembro, 683"}},
		}},
	}
	gateway, _ := NewToolGateway(&fakeToolSearch{}, query, &fakeToolReferences{}, func() time.Time { return now })
	arguments := `{
		"stages":[
			{"titulo":"Físicas","tipo":"consulta","root_entity":"bills","projections":["bill.printed_holder_name"],"filter":{"kind":"predicate","field":"bill.medium","operator":"eq","values":["PHYSICAL"]}},
			{"titulo":"Alfabeto","tipo":"cadeia","root_entity":"bills","letter_field":"bill.printed_holder_name","number_field":"bill.printed_address","minimum_length":10,"filter":{"kind":"predicate","field":"bill.medium","operator":"eq","values":["PHYSICAL"]}},
			{"titulo":"Clipe","tipo":"fora_da_base","nota":"As faturas precisam estar presas por clipe."}
		]
	}`
	output, err := gateway.Execute(context.Background(), actor, nil, Identifier{41}, 1,
		ToolCall{ID: "task-call", Name: "tarefa", Arguments: json.RawMessage(arguments)}, now.Add(time.Hour), "request")
	if err != nil {
		t.Fatalf("Execute(tarefa) error = %v", err)
	}
	payload := string(output.Payload)
	if output.Reference != nil || !strings.Contains(payload, "## Físicas") || !strings.Contains(payload, "Há 0 registros") ||
		!strings.Contains(payload, "## Alfabeto") || !strings.Contains(payload, "Não atende o mínimo de 10") ||
		!strings.Contains(payload, "Ana Maria Santos") || !strings.Contains(payload, "## Clipe") ||
		!strings.Contains(payload, "Fora da base") || strings.Contains(payload, "SELECT") {
		t.Fatalf("task payload = %s", payload)
	}
	if query.plan.CatalogVersion != strings.Repeat("a", 64) || query.plan.RootEntity != "bills" {
		t.Fatalf("chain plan = %#v", query.plan)
	}
}

func TestCrossStageRequiresARelation(t *testing.T) {
	actor, _ := chatTestActor(t, "member")
	now := time.Date(2026, time.September, 21, 21, 40, 0, 0, time.UTC)
	query := &fakeToolQuery{catalog: querydomain.Catalog{Version: strings.Repeat("b", 64)}}
	gateway, _ := NewToolGateway(&fakeToolSearch{}, query, &fakeToolReferences{}, func() time.Time { return now })
	arguments := `{"stages":[{"titulo":"Pessoa com conta","tipo":"cruzada","root_entity":"profiles","projections":["profile.full_name"],"filter":{"kind":"predicate","field":"profile.full_name","operator":"contains","values":["Ana"]}}]}`
	_, err := gateway.Execute(context.Background(), actor, nil, Identifier{42}, 1,
		ToolCall{ID: "cross-call", Name: "tarefa", Arguments: json.RawMessage(arguments)}, now.Add(time.Hour), "request")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Execute(cruzada sem relation) error = %v", err)
	}
	withRelation := `{"stages":[{"titulo":"Pessoa com conta","tipo":"cruzada","root_entity":"profiles","projections":["profile.full_name"],"filter":{"kind":"relation","relation":"profile.bills","children":[{"kind":"predicate","field":"bill.medium","operator":"eq","values":["PHYSICAL"]}]}}]}`
	query.matchCount = 2
	query.scan = querydomain.TextScan{Rows: []querydomain.TextScanRow{{ID: "p", Label: "Ana", Values: []string{"Ana"}}}}
	output, err := gateway.Execute(context.Background(), actor, nil, Identifier{43}, 1,
		ToolCall{ID: "cross-ok", Name: "tarefa", Arguments: json.RawMessage(withRelation)}, now.Add(time.Hour), "request")
	if err != nil || !strings.Contains(string(output.Payload), "Há 2 registros") || !strings.Contains(string(output.Payload), "Ana") {
		t.Fatalf("Execute(cruzada) = %s error=%v", output.Payload, err)
	}
	if query.plan.Filter == nil || query.plan.Filter.Kind != querydomain.FilterRelation || query.plan.Filter.Relation != "profile.bills" {
		t.Fatalf("cross plan = %#v", query.plan.Filter)
	}
}

func TestPatternStageUsesTheAdvancedPlan(t *testing.T) {
	actor, _ := chatTestActor(t, "member")
	now := time.Date(2026, time.September, 21, 21, 50, 0, 0, time.UTC)
	text := "1010"
	query := &fakeToolQuery{
		catalog: querydomain.Catalog{Version: strings.Repeat("d", 64)},
		advanced: querydomain.AdvancedResult{Rows: []querydomain.ResultRow{{
			EntityLabel: "RG · 1010",
			Cells:       []querydomain.ResultCell{{TextValue: &text}},
		}}},
	}
	gateway, _ := NewToolGateway(&fakeToolSearch{}, query, &fakeToolReferences{}, func() time.Time { return now })
	arguments := `{"stages":[{"titulo":"Binário","tipo":"padrao","root_entity":"documents","patterns":[{"field":"document.identifier","grammar":"binary_digits","pattern":"10+"}]}]}`
	output, err := gateway.Execute(context.Background(), actor, nil, Identifier{44}, 1,
		ToolCall{ID: "pattern-call", Name: "tarefa", Arguments: json.RawMessage(arguments)}, now.Add(time.Hour), "request")
	if err != nil {
		t.Fatalf("Execute(padrao) error = %v", err)
	}
	payload := string(output.Payload)
	if !strings.Contains(payload, "Atende") || !strings.Contains(payload, "1010") || query.plan.Version != "" && query.plan.Patterns[0].Grammar != querydomain.PatternBinaryDigits {
		t.Fatalf("pattern payload = %s plan=%#v", payload, query.plan)
	}
	if len(query.plan.Patterns) != 1 || query.plan.Patterns[0].Grammar != querydomain.PatternBinaryDigits || query.plan.RootEntity != "documents" {
		t.Fatalf("pattern plan = %#v", query.plan)
	}
}
