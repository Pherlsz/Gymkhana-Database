package googleforms

import (
	"errors"
	"testing"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/operations"
)

func TestSafeReturnPathAllowsOnlyGoogleFormsState(t *testing.T) {
	sourceID, err := NewIdentifier()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		value string
		want  string
	}{
		{value: "", want: "/google-forms"},
		{value: "/google-forms", want: "/google-forms"},
		{value: "/google-forms?tab=history", want: "/google-forms?tab=history"},
		{value: "/google-forms?source=" + sourceID.String() + "&tab=sources", want: "/google-forms?source=" + sourceID.String() + "&tab=sources"},
	}
	for _, test := range tests {
		got, err := safeReturnPath(test.value)
		if err != nil || got != test.want {
			t.Fatalf("safeReturnPath(%q) = %q, %v", test.value, got, err)
		}
	}
	for _, value := range []string{
		"//evil.test/google-forms",
		"https://evil.test/google-forms",
		"/operations",
		"/google-forms#fragment",
		"/google-forms?tab=unknown",
		"/google-forms?tab=",
		"/google-forms?redirect=https://evil.test",
		"/google-forms?source=not-a-uuid",
		"/google-forms?tab=sources&tab=history",
	} {
		if _, err := safeReturnPath(value); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("safeReturnPath(%q) error = %v", value, err)
		}
	}
}

func TestSourceMappingUsesOnlyAllowlistedLogicalFields(t *testing.T) {
	source := Source{Module: operations.ModuleProfiles, Questions: []Question{
		{ID: "name", Position: 0, Supported: true},
		{ID: "files", Position: 1, Supported: false, UnsupportedCode: "file_upload"},
	}}
	catalog := operations.Catalog(auth.RoleAdmin)[0]
	mapping, err := validateSourceMapping(source, catalog, []MappingInput{{QuestionID: "name", TargetField: "full_name"}})
	if err != nil || operations.ValidateMapping(catalog, mapping) != nil {
		t.Fatalf("valid mapping = %#v, %v", mapping, err)
	}
	invalid := [][]MappingInput{
		{{QuestionID: "files", TargetField: "full_name"}},
		{{QuestionID: "name", TargetField: "app_users.github_login"}},
		{{QuestionID: "name", TargetField: "full_name"}, {QuestionID: "name", TargetField: "notes"}},
	}
	for _, candidate := range invalid {
		mapped, mapErr := validateSourceMapping(source, catalog, candidate)
		if mapErr == nil && operations.ValidateMapping(catalog, mapped) == nil {
			t.Fatalf("invalid mapping was accepted: %#v", candidate)
		}
	}
}

func TestSyncCursorKeepsPaginationWindowAndOtherwiseOverlaps(t *testing.T) {
	cursor := time.Date(2026, time.July, 17, 12, 0, 0, 0, time.UTC)
	pageCursor := cursor.Add(-time.Hour)
	withPage := syncCursor(Source{CursorSubmittedAt: &cursor, ResponsePageToken: "next", PageTokenCursor: &pageCursor})
	if withPage == nil || !withPage.Equal(pageCursor) {
		t.Fatalf("syncCursor(page) = %v", withPage)
	}
	withoutPage := syncCursor(Source{CursorSubmittedAt: &cursor})
	if withoutPage == nil || !withoutPage.Equal(cursor.Add(-CursorOverlap)) {
		t.Fatalf("syncCursor(overlap) = %v", withoutPage)
	}
}

func TestGoogleFormsAuthorizationRequiresActiveAdminAndSession(t *testing.T) {
	userID, _ := auth.NewIdentifier()
	sessionID, _ := auth.NewIdentifier()
	service := &Service{enabled: true}
	allowed := auth.Session{ID: sessionID, User: auth.User{ID: userID, Role: auth.RoleAdmin, Active: true}}
	if err := service.authorize(allowed); err != nil {
		t.Fatalf("authorize(admin) error = %v", err)
	}
	for _, denied := range []auth.Session{
		{ID: sessionID, User: auth.User{ID: userID, Role: auth.RoleMember, Active: true}},
		{ID: sessionID, User: auth.User{ID: userID, Role: auth.RoleAdmin, Active: false}},
		{User: auth.User{ID: userID, Role: auth.RoleAdmin, Active: true}},
	} {
		if err := service.authorize(denied); !errors.Is(err, ErrForbidden) {
			t.Fatalf("authorize(%#v) error = %v", denied, err)
		}
	}
	if err := (&Service{}).authorize(allowed); !errors.Is(err, ErrDisabled) {
		t.Fatalf("authorize(disabled) error = %v", err)
	}
}
