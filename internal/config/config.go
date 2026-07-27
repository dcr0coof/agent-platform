package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// LLMConfig LLM 客户端配置
type LLMConfig struct {
	APIKey      string  `yaml:"api_key"`
	BaseURL     string  `yaml:"base_url"`
	Model       string  `yaml:"model"`
	MaxTokens   int     `yaml:"max_tokens"`
	Temperature float64 `yaml:"temperature"`
}

// AgentConfig Agent 运行时配置
type AgentConfig struct {
	MaxIterations int    `yaml:"max_iterations"`
	Name          string `yaml:"name"`
}

// MemoryConfig 记忆配置
type MemoryConfig struct {
	MaxMessages    int  `yaml:"max_messages"`
	EnableSummary  bool `yaml:"enable_summary"`
	SummaryTrigger int  `yaml:"summary_trigger"`
}

// WeatherConfig 天气 API 配置
type WeatherConfig struct {
	APIKey string `yaml:"api_key"`
}

// Config 顶层配置
type Config struct {
	LLM     LLMConfig     `yaml:"llm"`
	Agent   AgentConfig   `yaml:"agent"`
	Memory  MemoryConfig  `yaml:"memory"`
	Weather WeatherConfig `yaml:"weather"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		LLM: LLMConfig{
			BaseURL:     "https://api.deepseek.com/v1",
			Model:       "deepseek-chat",
			MaxTokens:   2000,
			Temperature: 0.7,
		},
		Agent: AgentConfig{
			MaxIterations: 10,
			Name:          "default",
		},
		Memory: MemoryConfig{
			MaxMessages:    20,
			EnableSummary:  true,
			SummaryTrigger: 30,
		},
	}
}

// Load 从 YAML 文件加载配置，再用环境变量覆盖
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			// 配置文件不存在时用默认值，不报错
			if !os.IsNotExist(err) {
				return nil, err
			}
		} else {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, err
			}
		}
	}

	// 环境变量覆盖（优先级最高）
	applyEnvOverrides(cfg)

	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("AGENT_LLM_API_KEY"); v != "" {
		cfg.LLM.APIKey = v
	}
	if v := os.Getenv("AGENT_LLM_BASE_URL"); v != "" {
		cfg.LLM.BaseURL = v
	}
	if v := os.Getenv("AGENT_LLM_MODEL"); v != "" {
		cfg.LLM.Model = v
	}
	if v := os.Getenv("AGENT_WEATHER_API_KEY"); v != "" {
		cfg.Weather.APIKey = v
	}
}
