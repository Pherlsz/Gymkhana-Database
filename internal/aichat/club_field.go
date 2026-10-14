package aichat

import (
	"strings"

	querydomain "github.com/Pherlsz/Gymkhana-Database/internal/queryengine"
)

const (
	teamFieldKey     = "profile.team"
	clubFieldKey     = "profile.club_membership"
	nameFieldKey     = "profile.full_name"
	emptyPairingNote = "Nenhuma linha com este plano. Não percorra os nomes com search. Sócio de time é profile.club_membership, não profile.team. Corrija o plano ou diga que não há."
)

// Football club names that never belong on Equipe (profile.team).
var footballClubTokens = []string{
	"gremio", "gremiofbpa", "internacional", "chapecoense", "juventude", "cacequifc",
}

func rewriteClubMembershipFields(plan *querydomain.QueryPlan) bool {
	if plan == nil || plan.Filter == nil {
		return false
	}
	if !rewriteClubFilter(plan.Filter) {
		return false
	}
	replaced := false
	for index, key := range plan.Projections {
		if key == teamFieldKey {
			plan.Projections[index] = clubFieldKey
			replaced = true
		}
	}
	if !replaced && len(plan.Projections) > 0 && len(plan.Projections) < querydomain.MaximumProjections &&
		plan.Version != querydomain.PlanVersionV2 && len(plan.Aggregates) == 0 && len(plan.GroupBy) == 0 {
		plan.Projections = append(plan.Projections, clubFieldKey)
	}
	return true
}

func rewriteClubFilter(node *querydomain.FilterNode) bool {
	if node == nil {
		return false
	}
	changed := false
	if node.Field == teamFieldKey && valuesLookLikeFootballClub(node.Values) {
		node.Field = clubFieldKey
		changed = true
	}
	for index := range node.Children {
		if rewriteClubFilter(&node.Children[index]) {
			changed = true
		}
	}
	return changed
}

func valuesLookLikeFootballClub(values []string) bool {
	for _, value := range values {
		if looksLikeFootballClub(value) {
			return true
		}
	}
	return false
}

func looksLikeFootballClub(value string) bool {
	token := querydomain.FoldClubToken(value)
	if token == "" {
		return false
	}
	for _, club := range footballClubTokens {
		if token == club || (len(club) >= 6 && strings.Contains(token, club)) {
			return true
		}
	}
	return false
}

func pairingEmptyHint(plan querydomain.QueryPlan) bool {
	stats := collectFilterFieldStats(plan.Filter)
	return stats.club || stats.team || stats.names >= 2
}

type filterFieldStats struct {
	club  bool
	team  bool
	names int
}

func collectFilterFieldStats(node *querydomain.FilterNode) filterFieldStats {
	if node == nil {
		return filterFieldStats{}
	}
	stats := filterFieldStats{}
	switch node.Field {
	case clubFieldKey:
		stats.club = true
	case teamFieldKey:
		stats.team = true
	case nameFieldKey, "profile.social_name":
		stats.names++
	}
	for index := range node.Children {
		child := collectFilterFieldStats(&node.Children[index])
		stats.club = stats.club || child.club
		stats.team = stats.team || child.team
		stats.names += child.names
	}
	return stats
}
