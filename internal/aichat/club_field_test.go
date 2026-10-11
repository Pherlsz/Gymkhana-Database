package aichat

import (
	"strings"
	"testing"

	querydomain "github.com/Pherlsz/Gymkhana-Database/internal/queryengine"
)

func TestRewriteClubMembershipFieldsMovesTeamClubValues(t *testing.T) {
	plan := querydomain.QueryPlan{
		RootEntity:  "profiles",
		Projections: []string{"profile.full_name", "profile.team"},
		Filter: &querydomain.FilterNode{Kind: querydomain.FilterGroup, Conjunction: querydomain.ConjunctionOr, Children: []querydomain.FilterNode{
			{Kind: querydomain.FilterGroup, Conjunction: querydomain.ConjunctionAnd, Children: []querydomain.FilterNode{
				{Kind: querydomain.FilterPredicate, Field: "profile.full_name", Operator: querydomain.OperatorStartsWith, Values: []string{"Alessandro"}},
				{Kind: querydomain.FilterPredicate, Field: "profile.team", Operator: querydomain.OperatorContains, Values: []string{"gremio"}},
			}},
			{Kind: querydomain.FilterGroup, Conjunction: querydomain.ConjunctionAnd, Children: []querydomain.FilterNode{
				{Kind: querydomain.FilterPredicate, Field: "profile.full_name", Operator: querydomain.OperatorStartsWith, Values: []string{"Pablo"}},
				{Kind: querydomain.FilterPredicate, Field: "profile.team", Operator: querydomain.OperatorEqual, Values: []string{"Internacional"}},
			}},
			{Kind: querydomain.FilterPredicate, Field: "profile.team", Operator: querydomain.OperatorEqual, Values: []string{"TNC"}},
		}},
	}
	if !rewriteClubMembershipFields(&plan) {
		t.Fatal("expected club rewrite")
	}
	if plan.Filter.Children[0].Children[1].Field != clubFieldKey || plan.Filter.Children[1].Children[1].Field != clubFieldKey {
		t.Fatalf("club predicates = %#v", plan.Filter)
	}
	if plan.Filter.Children[2].Field != teamFieldKey {
		t.Fatalf("gymkhana team was rewritten: %#v", plan.Filter.Children[2])
	}
	if strings.Join(plan.Projections, ",") != "profile.full_name,profile.club_membership" {
		t.Fatalf("projections = %s", strings.Join(plan.Projections, ","))
	}
}

func TestRewriteClubMembershipFieldsIgnoresGymkhanaTeam(t *testing.T) {
	plan := querydomain.QueryPlan{
		Projections: []string{"profile.full_name", "profile.team"},
		Filter:      &querydomain.FilterNode{Kind: querydomain.FilterPredicate, Field: teamFieldKey, Operator: querydomain.OperatorEqual, Values: []string{"TNC"}},
	}
	if rewriteClubMembershipFields(&plan) {
		t.Fatalf("rewrote gymkhana team: %#v", plan)
	}
	if plan.Filter.Field != teamFieldKey || plan.Projections[1] != teamFieldKey {
		t.Fatalf("plan = %#v", plan)
	}
}

func TestLooksLikeFootballClubFoldsAccent(t *testing.T) {
	for _, value := range []string{"Grêmio", "gremio", "GREMIO", "Internacional", "Chapecoense"} {
		if !looksLikeFootballClub(value) {
			t.Fatalf("%q should look like a football club", value)
		}
	}
	for _, value := range []string{"TNC", "Azzurra", "Força Tarefa", "NERD"} {
		if looksLikeFootballClub(value) {
			t.Fatalf("%q should stay a gymkhana team", value)
		}
	}
}

func TestPairingEmptyHint(t *testing.T) {
	club := querydomain.QueryPlan{Filter: &querydomain.FilterNode{Kind: querydomain.FilterPredicate, Field: clubFieldKey, Operator: querydomain.OperatorEqual, Values: []string{"Grêmio"}}}
	if !pairingEmptyHint(club) {
		t.Fatal("club filter should hint")
	}
	names := querydomain.QueryPlan{Filter: &querydomain.FilterNode{Kind: querydomain.FilterGroup, Conjunction: querydomain.ConjunctionOr, Children: []querydomain.FilterNode{
		{Kind: querydomain.FilterPredicate, Field: nameFieldKey, Operator: querydomain.OperatorStartsWith, Values: []string{"Alessandro"}},
		{Kind: querydomain.FilterPredicate, Field: nameFieldKey, Operator: querydomain.OperatorStartsWith, Values: []string{"Ismael"}},
	}}}
	if !pairingEmptyHint(names) {
		t.Fatal("two names should hint")
	}
	single := querydomain.QueryPlan{Filter: &querydomain.FilterNode{Kind: querydomain.FilterPredicate, Field: nameFieldKey, Operator: querydomain.OperatorContains, Values: []string{"Ana"}}}
	if pairingEmptyHint(single) {
		t.Fatal("single name should not hint")
	}
}
