package operations

import (
	"context"
	"testing"
)

func TestTargetFromValuesCreateWithoutIDs(t *testing.T) {
	id, version, action, err := targetFromValues(map[string]string{
		"full_name": "Ana",
		"cpf":       "529.982.247-25",
	})
	if err != nil || id != nil || version != 0 || action != ActionCreate {
		t.Fatalf("targetFromValues() = %v %d %q %v", id, version, action, err)
	}
}

func TestResolveTargetCreatesWhenCPFEmpty(t *testing.T) {
	service := &Service{}
	id, version, action, err := service.resolveTarget(context.Background(), ModuleProfiles, map[string]string{
		"full_name": "Ana",
	})
	if err != nil || id != nil || version != 0 || action != ActionCreate {
		t.Fatalf("resolveTarget() = %v %d %q %v", id, version, action, err)
	}
}
