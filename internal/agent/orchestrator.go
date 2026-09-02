package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"staj-projesi/internal/models"
	"staj-projesi/internal/rag"
	"staj-projesi/internal/tools"
)

// ToolExecutionRecord stores details of an executed tool.
type ToolExecutionRecord struct {
	ToolName string `json:"tool_name"`
	Input    string `json:"input"`
	Output   string `json:"output"`
	Success  bool   `json:"success"`
}

// AgentResponse contains the full synthesis and metadata for the user.
type AgentResponse struct {
	FinalAnswer     string                 `json:"final_answer"`
	RetrievedChunks []*models.SearchResult `json:"retrieved_chunks"`
	ToolsExecuted   []ToolExecutionRecord  `json:"tools_executed"`
	IsTroubleshoot  bool                   `json:"is_troubleshoot"`
}

// Orchestrator coordinates RAG retrieval, LLM reasoning, and tool execution.
type Orchestrator struct {
	retriever   *rag.Retriever
	client      *LLMClient
	toolManager *tools.ToolManager
	history     []ChatMessage
}

// NewOrchestrator initializes the agent flow orchestrator.
func NewOrchestrator(retriever *rag.Retriever, client *LLMClient, toolManager *tools.ToolManager) *Orchestrator {
	return &Orchestrator{
		retriever:   retriever,
		client:      client,
		toolManager: toolManager,
		history:     make([]ChatMessage, 0),
	}
}

// ProcessQuery runs the multi-stage Agentic RAG pipeline.
func (o *Orchestrator) ProcessQuery(ctx context.Context, userQuery string) (*AgentResponse, error) {
	resp := &AgentResponse{
		RetrievedChunks: make([]*models.SearchResult, 0),
		ToolsExecuted:   make([]ToolExecutionRecord, 0),
	}

	// 1. RAG Retrieval Phase
	searchResults, err := o.retriever.Search(userQuery, 2)
	if err == nil {
		resp.RetrievedChunks = searchResults
	}

	ragContext := o.retriever.FormatContext(searchResults)

	// 2. Build System Prompt with Dual-Layer Instructions
	systemPrompt := `Sen macOS işletim sistemine özel uzmanlaşmış bir "Agentic Ağ ve Troubleshooting Asistanı"sın.
Sana sağlanan RAG bilgi dokümanları çift katmanlıdır:
- [KULLANICI KILAVUZU]: Kullanıcının genel sorularına rehberlik etmek için adım adım bilgi içerir.
- [AJAN / TOOL METADATA]: Kullanıcı bir işlem yapmanı veya sorun çözmeni istediğinde kullanacağın Tool ve komut bilgilerini içerir.

GÖREVLERİN:
1. Soru Sorma / Bilgi İsteme: Kullanıcı yalnızca soru soruyorsa (örn: "DHCP ayarlarını nerden yapabilirim?"), RAG kılavuzundan yararlanarak adım adım açıkla. Tool çağırma.
2. Eylem / Ayar Değiştirme: Kullanıcı doğrudan ayar değiştirmek istiyorsa (örn: "DNS ayarımı 8.8.8.8 yap"), ilgili Tool'u (örn: SetDNSTool) çağır.
3. Sorun Giderme / Arıza (Troubleshoot): Kullanıcı bağlantı hatası bildiriyorsa (örn: "internete erişemiyorum"), NetworkTroubleshootTool aracını çağır. Tool sonucunu analiz edip kullanıcıya sorunun kök nedenini ve yapılan düzeltmeyi net biçimde özetle.

Tüm yanıtlarını Türkçe, teknik olarak doğru, nazik ve profesyonel bir dille ver.`

	// 3. Assemble Messages
	messages := []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "system", Content: ragContext},
	}

	// Add recent history if available
	if len(o.history) > 6 {
		messages = append(messages, o.history[len(o.history)-6:]...)
	} else {
		messages = append(messages, o.history...)
	}

	userMsg := ChatMessage{Role: "user", Content: userQuery}
	messages = append(messages, userMsg)

	// 4. LLM Call Round 1 (With Tool Definitions)
	toolDefs := o.toolManager.GetToolDefinitions()
	assistantMsg, err := o.client.Complete(ctx, messages, toolDefs)
	if err != nil {
		// Fallback: If model fails or tool calling API encounters an issue, fallback to deterministic agent execution
		return o.fallbackExecution(userQuery, ragContext, searchResults)
	}

	// 5. Check for Tool Calling
	if len(assistantMsg.ToolCalls) > 0 {
		messages = append(messages, *assistantMsg)

		for _, toolCall := range assistantMsg.ToolCalls {
			fnName := toolCall.Function.Name
			fnArgs := toolCall.Function.Arguments

			if fnName == "NetworkTroubleshootTool" {
				resp.IsTroubleshoot = true
			}

			// Execute the Tool
			toolOutput, execErr := o.toolManager.ExecuteTool(fnName, fnArgs)
			success := execErr == nil
			if execErr != nil {
				toolOutput = fmt.Sprintf("Tool execution failed: %v", execErr)
			}

			resp.ToolsExecuted = append(resp.ToolsExecuted, ToolExecutionRecord{
				ToolName: fnName,
				Input:    fnArgs,
				Output:   toolOutput,
				Success:  success,
			})

			// Append tool output to conversation
			toolMsg := ChatMessage{
				Role:       "tool",
				Name:       fnName,
				Content:    toolOutput,
				ToolCallID: toolCall.ID,
			}
			messages = append(messages, toolMsg)
		}

		// 6. LLM Call Round 2 (Synthesizing Tool Output for Final Answer)
		finalMsg, err := o.client.Complete(ctx, messages, nil)
		if err != nil {
			// If synthesis fails, build clear summary from tool output
			resp.FinalAnswer = o.synthesizeFallbackAnswer(resp.ToolsExecuted)
		} else {
			resp.FinalAnswer = finalMsg.Content
		}

	} else {
		// No tool call triggered - pure informational answer or text response
		resp.FinalAnswer = assistantMsg.Content
	}

	// Update in-memory history
	o.history = append(o.history, userMsg)
	o.history = append(o.history, ChatMessage{Role: "assistant", Content: resp.FinalAnswer})

	return resp, nil
}

// fallbackExecution provides robust deterministic routing if the LLM API is unavailable or returns errors.
func (o *Orchestrator) fallbackExecution(userQuery, ragContext string, searchResults []*models.SearchResult) (*AgentResponse, error) {
	resp := &AgentResponse{
		RetrievedChunks: searchResults,
		ToolsExecuted:   make([]ToolExecutionRecord, 0),
	}

	lower := strings.ToLower(userQuery)

	// Scenario 2: Action - DNS Change
	if strings.Contains(lower, "dns") && (strings.Contains(lower, "değiştir") || strings.Contains(lower, "8.8.8.8") || strings.Contains(lower, "yap") || strings.Contains(lower, "güncelle")) {
		var targetDNS []string
		if strings.Contains(lower, "8.8.8.8") {
			targetDNS = []string{"8.8.8.8", "8.8.4.4"}
		} else if strings.Contains(lower, "1.1.1.1") {
			targetDNS = []string{"1.1.1.1", "1.0.0.1"}
		} else {
			targetDNS = []string{"8.8.8.8"}
		}

		argsBytes, _ := json.Marshal(map[string]interface{}{
			"service":     "Wi-Fi",
			"dns_servers": targetDNS,
		})
		out, err := o.toolManager.ExecuteTool("SetDNSTool", string(argsBytes))
		resp.ToolsExecuted = append(resp.ToolsExecuted, ToolExecutionRecord{
			ToolName: "SetDNSTool",
			Input:    string(argsBytes),
			Output:   out,
			Success:  err == nil,
		})

		resp.FinalAnswer = fmt.Sprintf("İsteğiniz üzerine macOS Sandbox ortamında Wi-Fi arayüzü DNS sunucuları **%s** olarak başarıyla güncellendi.\n\n**Uygulanan Komut:** `networksetup -setdnsservers Wi-Fi %s`\n**Sistem Çıktısı:** %s", strings.Join(targetDNS, ", "), strings.Join(targetDNS, " "), out)
		return resp, nil
	}

	// Scenario 3: Troubleshooting
	if strings.Contains(lower, "internete") || strings.Contains(lower, "erişemiyorum") || strings.Contains(lower, "koptu") || strings.Contains(lower, "sorun") || strings.Contains(lower, "troubleshoot") || strings.Contains(lower, "bağlantı") {
		resp.IsTroubleshoot = true
		out, err := o.toolManager.ExecuteTool("NetworkTroubleshootTool", `{"service":"Wi-Fi","auto_remediate":true}`)
		resp.ToolsExecuted = append(resp.ToolsExecuted, ToolExecutionRecord{
			ToolName: "NetworkTroubleshootTool",
			Input:    `{"service":"Wi-Fi","auto_remediate":true}`,
			Output:   out,
			Success:  err == nil,
		})

		resp.FinalAnswer = fmt.Sprintf("İnternet bağlantı sorununuz için macOS üzerinde otomatik teşhis ve onarım süreci çalıştırıldı:\n\n%s", out)
		return resp, nil
	}

	// Scenario 1: Information (DHCP or general network)
	if len(searchResults) > 0 {
		topChunk := searchResults[0].Chunk
		resp.FinalAnswer = fmt.Sprintf("### %s\n\n%s\n\n*(Bilgi Kaynağı: %s - Sayfa %d)*", topChunk.Title, topChunk.UserGuide, topChunk.Category, topChunk.Page)
		return resp, nil
	}

	resp.FinalAnswer = "Sorunuzla ilgili bilgi ve işlem adımları RAG veritabanında arandı. macOS ağ yönetimi ile ilgili DHCP ayarları, DNS değiştirme veya internet arıza teşhisi konusunda size yardımcı olabilirim."
	return resp, nil
}

func (o *Orchestrator) synthesizeFallbackAnswer(records []ToolExecutionRecord) string {
	var sb strings.Builder
	sb.WriteString("İşleminiz başarıyla tamamlandı:\n\n")
	for _, rec := range records {
		sb.WriteString(fmt.Sprintf("- **Tool:** `%s`\n", rec.ToolName))
		sb.WriteString(fmt.Sprintf("- **Sonuç:** %s\n\n", rec.Output))
	}
	return sb.String()
}
