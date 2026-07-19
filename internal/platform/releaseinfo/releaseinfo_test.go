package releaseinfo

import (
	"testing"
	"time"
)

func TestValidateAllowsDevelopmentIdentityOutsideProduction(t *testing.T) {
	info := Info{Version: "dev", Commit: "unknown"}
	if err := info.Validate(false); err != nil {
		t.Fatalf("Validate(false) error = %v", err)
	}
}

func TestValidateRequiresImmutableProductionIdentity(t *testing.T) {
	validTime := time.Date(2026, time.July, 19, 1, 2, 3, 0, time.UTC)
	tests := []struct {
		name string
		info Info
	}{
		{name: "development version", info: Info{Version: "dev", Commit: "304b760a3fff934cefaf4bd36cb71bcebac060a2", BuiltAt: validTime}},
		{name: "missing commit", info: Info{Version: "v1.0.0", Commit: "unknown", BuiltAt: validTime}},
		{name: "short commit", info: Info{Version: "v1.0.0", Commit: "304b760a3fff", BuiltAt: validTime}},
		{name: "missing timestamp", info: Info{Version: "v1.0.0", Commit: "304b760a3fff934cefaf4bd36cb71bcebac060a2"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.info.Validate(true); err == nil {
				t.Fatal("Validate(true) error = nil, want validation error")
			}
		})
	}
}

func TestValidateAcceptsImmutableProductionIdentity(t *testing.T) {
	info := Info{
		Version: "sha-304b760a3fff",
		Commit:  "304b760a3fff934cefaf4bd36cb71bcebac060a2",
		BuiltAt: time.Date(2026, time.July, 19, 1, 2, 3, 0, time.UTC),
	}
	if err := info.Validate(true); err != nil {
		t.Fatalf("Validate(true) error = %v", err)
	}
	public := info.Public()
	if public.Version != info.Version || public.Revision != "304b760a3fff" {
		t.Fatalf("Public() = %#v", public)
	}
}

func TestCurrentNormalizesInjectedValues(t *testing.T) {
	originalVersion, originalCommit, originalBuiltAt := Version, Commit, BuiltAt
	t.Cleanup(func() {
		Version, Commit, BuiltAt = originalVersion, originalCommit, originalBuiltAt
	})
	Version = " v1.2.3 "
	Commit = " 304B760A3FFF934CEFAF4BD36CB71BCEBAC060A2 "
	BuiltAt = "2026-07-19T01:02:03-03:00"

	info := Current()
	if info.Version != "v1.2.3" || info.Commit != "304b760a3fff934cefaf4bd36cb71bcebac060a2" {
		t.Fatalf("Current() = %#v", info)
	}
	if info.BuiltAt.Format(time.RFC3339) != "2026-07-19T04:02:03Z" {
		t.Fatalf("BuiltAt = %s", info.BuiltAt.Format(time.RFC3339))
	}
}
