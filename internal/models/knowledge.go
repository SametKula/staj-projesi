package models

// AgentSpec defines the tool execution criteria, parameters, and intent triggers for a chunk.
type AgentSpec struct {
	SupportedTools    []string `json:"supported_tools"`
	TriggerIntents    []string `json:"trigger_intents"`
	RequiredParams    []string `json:"required_params"`
	SafetyLevel       string   `json:"safety_level"` // safe, medium, critical
	FallbackStrategy  string   `json:"fallback_strategy"`
	SuggestedCommand  string   `json:"suggested_command"`
}

// KnowledgeChunk represents a dual-layer RAG document.
type KnowledgeChunk struct {
	ID         string    `json:"id"`
	Category   string    `json:"category"`
	Title      string    `json:"title"`
	Page       int       `json:"page"`
	UserGuide  string    `json:"user_guide"`
	AgentSpec  AgentSpec `json:"agent_spec"`
	RawText    string    `json:"raw_text"`
	Tags       []string  `json:"tags"`
}

// SearchResult represents a scored RAG retrieval result.
type SearchResult struct {
	Chunk *KnowledgeChunk `json:"chunk"`
	Score float64         `json:"score"`
}
