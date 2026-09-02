package main

import (
	"staj-projesi/internal/agent"
	"staj-projesi/internal/config"
	"staj-projesi/internal/rag"
	"staj-projesi/internal/tools"
)

// Main entrypoint in cmd/cli package
func main() {
	// Reuses main logic
	_ = config.Load()
	_ = agent.ChatMessage{}
	_ = rag.Retriever{}
	_ = tools.NetworkSandbox{}
}
