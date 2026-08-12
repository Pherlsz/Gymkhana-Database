package httpserver

import (
	"encoding/json"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

// The current public API contract still exposes the authenticated identifier as
// `login`. Authentication itself is email/subject based; these marshalers keep
// the existing v1 wire contract stable until the OpenAPI contract is versioned
// to rename `login` to `email` without breaking generated clients.
func (user authUserResponse) MarshalJSON() ([]byte, error) {
	type wireUser struct {
		Login       string `json:"login"`
		DisplayName string `json:"display_name"`
		AvatarURL   string `json:"avatar_url,omitempty"`
		Role        string `json:"role"`
	}
	return json.Marshal(wireUser{
		Login:       user.Email,
		DisplayName: user.DisplayName,
		AvatarURL:   user.AvatarURL,
		Role:        string(user.Role),
	})
}

func (user *authUserResponse) UnmarshalJSON(data []byte) error {
	var wire struct {
		Login       string    `json:"login"`
		DisplayName string    `json:"display_name"`
		AvatarURL   string    `json:"avatar_url"`
		Role        auth.Role `json:"role"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	user.Email = wire.Login
	user.DisplayName = wire.DisplayName
	user.AvatarURL = wire.AvatarURL
	user.Role = wire.Role
	return nil
}

func (user adminUserResponse) MarshalJSON() ([]byte, error) {
	type wireUser struct {
		ID          string `json:"id"`
		Login       string `json:"login"`
		DisplayName string `json:"display_name"`
		AvatarURL   string `json:"avatar_url,omitempty"`
		Role        string `json:"role"`
		Active      bool   `json:"active"`
		Version     int64  `json:"version"`
	}
	return json.Marshal(wireUser{
		ID:          user.ID,
		Login:       user.Email,
		DisplayName: user.DisplayName,
		AvatarURL:   user.AvatarURL,
		Role:        string(user.Role),
		Active:      user.Active,
		Version:     user.Version,
	})
}

func (user *adminUserResponse) UnmarshalJSON(data []byte) error {
	var wire struct {
		ID          string    `json:"id"`
		Login       string    `json:"login"`
		DisplayName string    `json:"display_name"`
		AvatarURL   string    `json:"avatar_url"`
		Role        auth.Role `json:"role"`
		Active      bool      `json:"active"`
		Version     int64     `json:"version"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	user.ID = wire.ID
	user.Email = wire.Login
	user.DisplayName = wire.DisplayName
	user.AvatarURL = wire.AvatarURL
	user.Role = wire.Role
	user.Active = wire.Active
	user.Version = wire.Version
	return nil
}
