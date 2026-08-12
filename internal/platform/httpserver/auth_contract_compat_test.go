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

func TestAuthWireContractDecodesLoginField(t *testing.T) {
	var user authUserResponse
	if err := json.Unmarshal([]byte(`{"login":"member@example.test","display_name":"Member","avatar_url":"https://example.test/avatar","role":"EXTERNAL"}`), &user); err != nil {
		t.Fatalf("unmarshal auth user: %v", err)
	}
	if user.Email != "member@example.test" || user.DisplayName != "Member" || user.AvatarURL != "https://example.test/avatar" || user.Role != auth.RoleExternal {
		t.Fatalf("auth user = %#v", user)
	}
}

func TestAdminWireContractDecodesLoginField(t *testing.T) {
	var user adminUserResponse
	if err := json.Unmarshal([]byte(`{"id":"user-id","login":"admin@example.test","display_name":"Admin","avatar_url":"https://example.test/avatar","role":"ADMIN","active":true,"version":2}`), &user); err != nil {
		t.Fatalf("unmarshal admin user: %v", err)
	}
	if user.ID != "user-id" || user.Email != "admin@example.test" || user.DisplayName != "Admin" || user.AvatarURL != "https://example.test/avatar" || user.Role != auth.RoleAdmin || !user.Active || user.Version != 2 {
		t.Fatalf("admin user = %#v", user)
	}
}
