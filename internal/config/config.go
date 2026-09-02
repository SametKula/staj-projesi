package config

import (
	"os"
)

// Config holds all configuration settings for the Agentic RAG Network Assistant.
type Config struct {
	APIBaseURL       string
	APIKey           string
	ModelName        string
	SandboxMode      bool
	SandboxStateFile string
	KnowledgeDir     string
	HTTPPort         int
}

// Load loads configuration with environment variable overrides and sensible defaults.
func Load() *Config {
	cfg := &Config{
		APIBaseURL:       getEnv("LLM_API_BASE", "http://10.2.2.115:8080/v1"),
		APIKey:           getEnv("LLM_API_KEY", ""),
		ModelName:        getEnv("LLM_MODEL", `..\models\gemma4\gemma-4-E4B-it-qat-nvfp4.gguf`),
		SandboxMode:      getEnvBool("SANDBOX_MODE", true),
		SandboxStateFile: getEnv("SANDBOX_STATE_FILE", "data/sandbox_state.json"),
		KnowledgeDir:     getEnv("KNOWLEDGE_DIR", "data/knowledge"),
		HTTPPort:         8081,
	}
	return cfg
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if val, ok := os.LookupEnv(key); ok {
		return val == "true" || val == "1" || val == "yes"
	}
	return defaultVal
}
