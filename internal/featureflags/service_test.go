package featureflags

import (
	"context"
	"errors"
	"testing"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

type memoryStore struct {
	flags map[string]bool
}

func (store *memoryStore) List(context.Context) ([]Flag, error) {
	out := make([]Flag, 0, len(Known))
	for _, key := range Known {
		out = append(out, Flag{Key: key, Enabled: store.flags[key]})
	}
	return out, nil
}

func (store *memoryStore) IsEnabled(_ context.Context, key string) (bool, error) {
	key = normalizeKey(key)
	if !IsKnown(key) {
		return false, ErrUnknownFlag
	}
	return store.flags[key], nil
}

func (store *memoryStore) SetEnabled(_ context.Context, key string, enabled bool, _ *auth.Identifier) (Flag, error) {
	key = normalizeKey(key)
	if !IsKnown(key) {
		return Flag{}, ErrUnknownFlag
	}
	if store.flags == nil {
		store.flags = map[string]bool{}
	}
	store.flags[key] = enabled
	return Flag{Key: key, Enabled: enabled}, nil
}

func TestServiceRequiresSuperadmin(t *testing.T) {
	service, err := NewService(&memoryStore{flags: map[string]bool{KeyAIChat: true}})
	if err != nil {
		t.Fatal(err)
	}
	admin := auth.Session{User: auth.User{Role: auth.RoleAdmin, Active: true}}
	if _, err := service.List(context.Background(), admin); !errors.Is(err, ErrForbidden) {
		t.Fatalf("List() error = %v", err)
	}
	super := auth.Session{User: auth.User{Role: auth.RoleSuperadmin, Active: true}}
	flags, err := service.List(context.Background(), super)
	if err != nil || len(flags) != len(Known) {
		t.Fatalf("List() = %#v, %v", flags, err)
	}
	updated, err := service.SetEnabled(context.Background(), super, KeyAIChat, false)
	if err != nil || updated.Enabled {
		t.Fatalf("SetEnabled() = %#v, %v", updated, err)
	}
	ok, err := service.IsEnabled(context.Background(), KeyAIChat)
	if err != nil || ok {
		t.Fatalf("IsEnabled() = %v, %v", ok, err)
	}
}
