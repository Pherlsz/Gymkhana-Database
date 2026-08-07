package httpserver

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

func TestAuthWireContractKeepsLoginField(t *testing.T) {
	payload, err := json.Marshal(authUserResponse{
		Email:       "member@example.test",
		DisplayName: "Member",
		Role:        auth.RoleExternal,
	})
	if err != nil {
		t.Fatalf("marshal auth user: %v", err)
	}
	body := string(payload)
	if !strings.Contains(body, `"login":"member@example.test"`) {
		t.Fatalf("auth wire contract missing login: %s", body)
	}
	if strings.Contains(body, `"email"`) {
		t.Fatalf("auth wire contract exposed unversioned email field: %s", body)
	}
}

func TestAdminWireContractKeepsLoginField(t *testing.T) {
	payload, err := json.Marshal(adminUserResponse{
		ID:          "user-id",
		Email:       "admin@example.test",
		DisplayName: "Admin",
		Role:        auth.RoleAdmin,
		Active:      true,
		Version:     2,
	})
	if err != nil {
		t.Fatalf("marshal admin user: %v", err)
	}
	body := string(payload)
	if !strings.Contains(body, `"login":"admin@example.test"`) {
		t.Fatalf("admin wire contract missing login: %s", body)
	}
	if strings.Contains(body, `"email"`) {
		t.Fatalf("admin wire contract exposed unversioned email field: %s", body)
	}
}
