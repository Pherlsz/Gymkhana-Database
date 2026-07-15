package auth

import "testing"

func TestIdentifierRoundTrip(t *testing.T) {
	identifier, err := NewIdentifier()
	if err != nil {
		t.Fatalf("NewIdentifier() error = %v", err)
	}
	parsed, err := ParseIdentifier(identifier.String())
	if err != nil {
		t.Fatalf("ParseIdentifier() error = %v", err)
	}
	if parsed != identifier {
		t.Fatalf("parsed = %v, want %v", parsed, identifier)
	}
	if _, err := ParseIdentifier("invalid"); err == nil {
		t.Fatal("invalid identifier parsed successfully")
	}
}
