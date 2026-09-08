package config

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"strings"

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
	APIKey  string `yaml:"api_key"`
	BaseURL string `yaml:"base_url"`
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
			EnableSummary:  false,
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
			decoder := yaml.NewDecoder(bytes.NewReader(data))
			decoder.KnownFields(true)
			if err := decoder.Decode(cfg); err != nil && err != io.EOF {
				return nil, fmt.Errorf("解析配置失败: %w", err)
			}
			var extra interface{}
			if err := decoder.Decode(&extra); err != io.EOF {
				return nil, fmt.Errorf("配置文件只允许一个 YAML 文档")
			}
		}
	}

	// 环境变量覆盖（优先级最高）
	applyEnvOverrides(cfg)
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
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
	if v := os.Getenv("AGENT_WEATHER_BASE_URL"); v != "" {
		cfg.Weather.BaseURL = v
	}
}

// Validate 在启动阶段拒绝无法工作的配置；密钥是否必需由入口决定。
func (cfg *Config) Validate() error {
	cfg.LLM.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.LLM.BaseURL), "/")
	cfg.Weather.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.Weather.BaseURL), "/")
	if err := validateURL(cfg.LLM.BaseURL); err != nil {
		return fmt.Errorf("llm.base_url: %w", err)
	}
	if strings.TrimSpace(cfg.LLM.Model) == "" {
		return fmt.Errorf("llm.model 不能为空")
	}
	if cfg.LLM.MaxTokens <= 0 {
		return fmt.Errorf("llm.max_tokens 必须大于 0")
	}
	if math.IsNaN(cfg.LLM.Temperature) || math.IsInf(cfg.LLM.Temperature, 0) || cfg.LLM.Temperature < 0 || cfg.LLM.Temperature > 2 {
		return fmt.Errorf("llm.temperature 必须在 0 到 2 之间")
	}
	if cfg.Agent.MaxIterations <= 0 {
		return fmt.Errorf("agent.max_iterations 必须大于 0")
	}
	if strings.TrimSpace(cfg.Agent.Name) == "" {
		return fmt.Errorf("agent.name 不能为空")
	}
	if cfg.Memory.MaxMessages <= 0 {
		return fmt.Errorf("memory.max_messages 必须大于 0")
	}
	if cfg.Memory.EnableSummary {
		return fmt.Errorf("摘要记忆尚未实现，请设置 memory.enable_summary: false")
	}
	if cfg.Weather.BaseURL != "" {
		if err := validateURL(cfg.Weather.BaseURL); err != nil {
			return fmt.Errorf("weather.base_url: %w", err)
		}
	}
	if cfg.Weather.APIKey != "" && cfg.Weather.BaseURL == "" {
		return fmt.Errorf("配置天气密钥时必须同时设置 AGENT_WEATHER_BASE_URL 或 weather.base_url（专属 API Host 的 HTTPS URL）")
	}
	return nil
}

func validateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("必须是有效的 HTTP(S) URL，且不能包含用户信息、查询参数或片段")
	}
	return nil
}
