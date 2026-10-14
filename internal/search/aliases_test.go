package search

import "testing"

func TestLookupFieldAliasBindsSocioToClubMembership(t *testing.T) {
	for _, token := range []string{"socio", "sócio", "clube", "gremio", "Grêmio", "internacional"} {
		alias, ok := lookupFieldAlias(token)
		if !ok || len(alias.keys) != 1 || alias.keys[0] != "profile.club_membership" {
			t.Fatalf("lookupFieldAlias(%q) = %#v ok=%v", token, alias, ok)
		}
	}
	alias, ok := lookupFieldAlias("equipe")
	if !ok || alias.keys[0] != "profile.team" {
		t.Fatalf("equipe alias = %#v ok=%v", alias, ok)
	}
}
