package queryengine

import "testing"

func TestFoldPortugueseTextAndClubToken(t *testing.T) {
	if got := foldPortugueseText("Grêmio"); got != "gremio" {
		t.Fatalf("foldPortugueseText(Grêmio) = %q", got)
	}
	if got := FoldClubToken("Grêmio FBPA"); got != "gremiofbpa" {
		t.Fatalf("FoldClubToken = %q", got)
	}
	if got := FoldClubToken("Internacional"); got != "internacional" {
		t.Fatalf("FoldClubToken(Internacional) = %q", got)
	}
}
