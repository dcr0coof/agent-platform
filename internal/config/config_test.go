package config

import (
	"os"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.LLM.Model != "deepseek-chat" {
		t.Errorf("expected deepseek-chat, got %s", cfg.LLM.Model)
	}
	if cfg.Agent.MaxIterations != 10 {
		t.Errorf("expected 10, got %d", cfg.Agent.MaxIterations)
	}
}

func TestLoadWithDefaults(t *testing.T) {
	cfg, err := Load("nonexistent.yaml")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.LLM.BaseURL != "https://api.deepseek.com/v1" {
		t.Errorf("unexpected base URL: %s", cfg.LLM.BaseURL)
	}
}

func TestEnvOverride(t *testing.T) {
	os.Setenv("AGENT_LLM_API_KEY", "test-key-123")
	os.Setenv("AGENT_LLM_MODEL", "test-model")
	defer os.Unsetenv("AGENT_LLM_API_KEY")
	defer os.Unsetenv("AGENT_LLM_MODEL")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.LLM.APIKey != "test-key-123" {
		t.Errorf("APIKey not overridden: %s", cfg.LLM.APIKey)
	}
	if cfg.LLM.Model != "test-model" {
		t.Errorf("Model not overridden: %s", cfg.LLM.Model)
	}
}
