package tool

import (
	"context"
	"testing"
)

type mockTool struct {
	name string
}

func (m *mockTool) Name() string                       { return m.name }
func (m *mockTool) Description() string                { return "mock" }
func (m *mockTool) Parameters() map[string]interface{} { return nil }
func (m *mockTool) Execute(ctx context.Context, p map[string]interface{}) (string, error) {
	return "ok", nil
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&mockTool{name: "test"})

	tool, err := reg.Get("test")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if tool.Name() != "test" {
		t.Errorf("unexpected name: %s", tool.Name())
	}
}

func TestRegistry_GetNotFound(t *testing.T) {
	reg := NewRegistry()
	_, err := reg.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRegistry_List(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&mockTool{name: "a"})
	reg.Register(&mockTool{name: "b"})

	list := reg.List()
	if len(list) != 2 {
		t.Errorf("expected 2 tools, got %d", len(list))
	}
}

func TestRegistryListIsStable(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&mockTool{name: "z"})
	reg.Register(&mockTool{name: "a"})
	for i := 0; i < 100; i++ {
		list := reg.List()
		if list[0].Name() != "a" || list[1].Name() != "z" {
			t.Fatal("tool schema order is unstable")
		}
	}
}
