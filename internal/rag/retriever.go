package rag

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode"

	"staj-projesi/internal/models"
)

// Retriever manages the dual-layer RAG knowledge base and search operations.
type Retriever struct {
	mu           sync.RWMutex
	knowledgeDir string
	chunks       []*models.KnowledgeChunk
}

// NewRetriever initializes and loads all knowledge chunks from the directory.
func NewRetriever(knowledgeDir string) (*Retriever, error) {
	r := &Retriever{
		knowledgeDir: knowledgeDir,
		chunks:       make([]*models.KnowledgeChunk, 0),
	}

	if err := r.Load(); err != nil {
		return nil, fmt.Errorf("failed to load knowledge base: %w", err)
	}

	return r, nil
}

// Load reads all JSON files in the knowledge directory.
func (r *Retriever) Load() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.chunks = make([]*models.KnowledgeChunk, 0)
	files, err := os.ReadDir(r.knowledgeDir)
	if err != nil {
		return fmt.Errorf("read knowledge dir: %w", err)
	}

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		fullPath := filepath.Join(r.knowledgeDir, file.Name())
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return fmt.Errorf("read file %s: %w", file.Name(), err)
		}

		var fileChunks []*models.KnowledgeChunk
		if err := json.Unmarshal(data, &fileChunks); err != nil {
			return fmt.Errorf("unmarshal %s: %w", file.Name(), err)
		}

		r.chunks = append(r.chunks, fileChunks...)
	}

	return nil
}

// Search performs scored retrieval across all knowledge chunks.
func (r *Retriever) Search(query string, topK int) ([]*models.SearchResult, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.chunks) == 0 {
		return nil, nil
	}

	queryTokens := tokenize(query)
	var results []*models.SearchResult

	for _, chunk := range r.chunks {
		score := r.calculateScore(query, queryTokens, chunk)
		if score > 0 {
			results = append(results, &models.SearchResult{
				Chunk: chunk,
				Score: score,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}

	return results, nil
}

func (r *Retriever) calculateScore(query string, tokens []string, chunk *models.KnowledgeChunk) float64 {
	var score float64
	queryLower := strings.ToLower(query)

	// 1. Direct tag matches (High weight)
	for _, tag := range chunk.Tags {
		tagLower := strings.ToLower(tag)
		if strings.Contains(queryLower, tagLower) {
			score += 4.0
		}
		for _, token := range tokens {
			if token == tagLower {
				score += 3.0
			}
		}
	}

	// 2. Intent trigger matches (Very High weight)
	for _, intent := range chunk.AgentSpec.TriggerIntents {
		intentFormatted := strings.ReplaceAll(intent, "_", " ")
		if strings.Contains(queryLower, intentFormatted) {
			score += 6.0
		}
	}

	// 3. Title matching
	titleLower := strings.ToLower(chunk.Title)
	for _, token := range tokens {
		if strings.Contains(titleLower, token) {
			score += 2.5
		}
	}

	// 4. Raw text and User guide matching
	contentLower := strings.ToLower(chunk.RawText + " " + chunk.UserGuide)
	for _, token := range tokens {
		if strings.Contains(contentLower, token) {
			score += 1.0
		}
	}

	// Specific keyword boosters
	if (strings.Contains(queryLower, "dhcp") || strings.Contains(queryLower, "nerden") || strings.Contains(queryLower, "nasıl")) && chunk.Category == "dhcp_configuration" {
		score += 5.0
	}
	if (strings.Contains(queryLower, "dns") || strings.Contains(queryLower, "8.8.8.8") || strings.Contains(queryLower, "değiştir") || strings.Contains(queryLower, "yap")) && chunk.Category == "dns_management" {
		score += 5.0
	}
	if (strings.Contains(queryLower, "internete") || strings.Contains(queryLower, "erişemiyorum") || strings.Contains(queryLower, "bağlantı") || strings.Contains(queryLower, "koptu") || strings.Contains(queryLower, "sorun") || strings.Contains(queryLower, "troubleshoot")) && chunk.Category == "troubleshooting" {
		score += 6.0
	}

	return score
}

// FormatContext converts search results into a clean dual-layer prompt context.
func (r *Retriever) FormatContext(results []*models.SearchResult) string {
	if len(results) == 0 {
		return "İlgili ağ dokümantasyonu bulunamadı."
	}

	var sb strings.Builder
	sb.WriteString("=== RAG BİLGİ VE TOOL REHBERİ (DUAL-LAYER CONTEXT) ===\n\n")

	for i, res := range results {
		c := res.Chunk
		sb.WriteString(fmt.Sprintf("--- [DOKÜMAN %d: %s (Kategori: %s, Sayfa: %d)] ---\n", i+1, c.Title, c.Category, c.Page))
		sb.WriteString("[KULLANICI KILAVUZU (Bilgilendirme için)]:\n")
		sb.WriteString(c.UserGuide + "\n\n")

		sb.WriteString("[AJAN / TOOL METADATA (Eylem ve Teşhis için)]:\n")
		sb.WriteString(fmt.Sprintf("  - Desteklenen Tool'lar: %s\n", strings.Join(c.AgentSpec.SupportedTools, ", ")))
		sb.WriteString(fmt.Sprintf("  - Gerekli Parametreler: %s\n", strings.Join(c.AgentSpec.RequiredParams, ", ")))
		sb.WriteString(fmt.Sprintf("  - Güvenlik Seviyesi: %s\n", c.AgentSpec.SafetyLevel))
		if c.AgentSpec.SuggestedCommand != "" {
			sb.WriteString(fmt.Sprintf("  - Örnek Komut / Aksiyon: %s\n", c.AgentSpec.SuggestedCommand))
		}
		if c.AgentSpec.FallbackStrategy != "" {
			sb.WriteString(fmt.Sprintf("  - Geri Çekilme / Yedek Strateji: %s\n", c.AgentSpec.FallbackStrategy))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func tokenize(text string) []string {
	f := func(c rune) bool {
		return !unicode.IsLetter(c) && !unicode.IsNumber(c)
	}
	words := strings.FieldsFunc(strings.ToLower(text), f)
	var filtered []string
	stopWords := map[string]bool{
		"ve": true, "bir": true, "bu": true, "için": true, "ile": true, "mi": true, "mu": true,
		"ise": true, "gibi": true, "daha": true, "olan": true, "ne": true, "nasıl": true,
	}
	for _, w := range words {
		if len(w) > 1 && !stopWords[w] {
			filtered = append(filtered, w)
		}
	}
	return filtered
}
