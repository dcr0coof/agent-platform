package config

import (
	"os"
	"path/filepath"
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
	t.Setenv("AGENT_LLM_API_KEY", "test-key-123")
	t.Setenv("AGENT_LLM_MODEL", "test-model")

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

func TestInvalidConfig(t *testing.T) {
	for _, input := range []string{
		"memory:\n  max_messages: -1", "agent:\n  max_iterations: 0",
		"llm:\n  temperature: .nan", "llm:\n  temperature: 3", "llm:\n  max_tokens: 0",
		"llm:\n  base_url: nope", "memory:\n  enable_summary: true",
		"llm:\n  modle: typo", "agent:\n  name: ''", "weather:\n  api_key: test",
		"{}\n---\n{}", "llm: [", "llm:\n  base_url: https://user:password@example.com",
	} {
		t.Run(input, func(t *testing.T) {
			for _, key := range []string{"AGENT_LLM_BASE_URL", "AGENT_LLM_MODEL", "AGENT_WEATHER_API_KEY", "AGENT_WEATHER_BASE_URL"} {
				t.Setenv(key, "")
			}
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(input), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(path); err == nil {
				t.Fatal("invalid config accepted")
			}
		})
	}
}

func TestPartialConfigAndWeatherEnvironment(t *testing.T) {
	t.Setenv("AGENT_WEATHER_API_KEY", "test")
	t.Setenv("AGENT_WEATHER_BASE_URL", "https://example.qweatherapi.com/")
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("llm:\n  temperature: 0"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LLM.Temperature != 0 || cfg.Memory.MaxMessages != 20 || cfg.Weather.BaseURL != "https://example.qweatherapi.com" {
		t.Fatal("defaults/overrides lost")
	}
}
