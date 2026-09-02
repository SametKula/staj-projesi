package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"staj-projesi/internal/agent"
	"staj-projesi/internal/config"
	"staj-projesi/internal/rag"
	"staj-projesi/internal/tools"
)

// ANSI Color definitions
const (
	ColorReset  = "\033[0m"
	ColorBold   = "\033[1m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[36m"
	ColorWhite  = "\033[37m"
	ColorGray   = "\033[90m"
)

func main() {
	queryFlag := flag.String("query", "", "Tek seferlik soru veya komut çalıştırma")
	sandboxFlag := flag.Bool("sandbox", true, "macOS Sandbox Güvenlik Modu (varsayılan: true)")
	resetSandboxFlag := flag.Bool("reset-sandbox", false, "Sandbox durumunu başlangıç değerlerine sıfırlar")
	flag.Parse()

	cfg := config.Load()
	if !*sandboxFlag {
		cfg.SandboxMode = false
	}

	// 1. Initialize Sandbox
	sandbox, err := tools.NewNetworkSandbox(cfg.SandboxStateFile)
	if err != nil {
		fmt.Printf("%s[HATA] Sandbox başlatılamadı: %v%s\n", ColorRed, err, ColorReset)
		os.Exit(1)
	}

	if *resetSandboxFlag {
		sandbox.ResetToDefault()
		fmt.Printf("%s[BİLGİ] Sandbox ortamı başarıyla varsayılan duruma sıfırlandı.%s\n", ColorGreen, ColorReset)
	}

	// 2. Initialize RAG Retriever
	retriever, err := rag.NewRetriever(cfg.KnowledgeDir)
	if err != nil {
		fmt.Printf("%s[HATA] RAG Veritabanı yüklenemedi: %v%s\n", ColorRed, err, ColorReset)
		os.Exit(1)
	}

	// 3. Initialize Tool Manager
	toolManager := tools.NewToolManager(sandbox)

	// 4. Initialize LLM Client & Orchestrator
	llmClient := agent.NewLLMClient(cfg.APIBaseURL, cfg.APIKey, cfg.ModelName)
	orchestrator := agent.NewOrchestrator(retriever, llmClient, toolManager)

	// If single query mode is passed via CLI flag
	if *queryFlag != "" {
		executeAndPrint(orchestrator, sandbox, *queryFlag)
		return
	}

	// Interactive Mode Banner
	printBanner(cfg)

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Printf("\n%s%sKullanici > %s", ColorBold, ColorCyan, ColorReset)
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		if strings.EqualFold(input, "exit") || strings.EqualFold(input, "quit") || input == "/q" {
			fmt.Printf("%s\nİyi günler! Ağ Asistanı kapatılıyor...%s\n", ColorGreen, ColorReset)
			break
		}

		if input == "/help" || input == "help" {
			printHelp()
			continue
		}

		if input == "/status" || input == "status" {
			printStatus(sandbox)
			continue
		}

		if input == "/reset" {
			sandbox.ResetToDefault()
			fmt.Printf("%s[BİLGİ] Sandbox ağ durumu başlangıç değerlerine sıfırlandı.%s\n", ColorGreen, ColorReset)
			continue
		}

		if input == "/troubleshoot" {
			input = "internete erişemiyorum"
		}

		executeAndPrint(orchestrator, sandbox, input)
	}
}

func executeAndPrint(orchestrator *agent.Orchestrator, sandbox *tools.NetworkSandbox, userQuery string) {
	fmt.Printf("\n%s[BILGI] Ag dokumanlari taraniyor ve analiz ediliyor...%s\n", ColorGray, ColorReset)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := orchestrator.ProcessQuery(ctx, userQuery)
	if err != nil {
		fmt.Printf("%s[HATA]: %v%s\n", ColorRed, err, ColorReset)
		return
	}

	// Print RAG Information
	if len(resp.RetrievedChunks) > 0 {
		fmt.Printf("\n%s[RAG Eslenen Bilgi Dokumanlari]:%s\n", ColorPurple, ColorReset)
		for _, match := range resp.RetrievedChunks {
			fmt.Printf("   %s- %s (Kategori: %s, Guven Skoru: %.1f)%s\n", ColorGray, match.Chunk.Title, match.Chunk.Category, match.Score, ColorReset)
		}
	}

	// Print Executed Tools
	if len(resp.ToolsExecuted) > 0 {
		fmt.Printf("\n%s[Yurutulen Sistem / Sandbox Araclari]:%s\n", ColorYellow, ColorReset)
		for _, t := range resp.ToolsExecuted {
			statusColor := ColorGreen
			if !t.Success {
				statusColor = ColorRed
			}
			fmt.Printf("   %s- Arac: %s%s%s (Parametreler: %s)\n", ColorBold, ColorYellow, t.ToolName, ColorReset, t.Input)
			fmt.Printf("     %sSonuc: %s%s\n", statusColor, t.Output, ColorReset)
		}
	}

	// Print Final Answer
	fmt.Printf("\n%s%sAg Asistani:%s\n\n", ColorBold, ColorGreen, ColorReset)
	fmt.Println(resp.FinalAnswer)
}

func printBanner(cfg *config.Config) {
	fmt.Printf("%s=========================================================================%s\n", ColorBlue, ColorReset)
	fmt.Printf("%s%s        macOS Ag Yonetim ve Troubleshooting Asistani (Agentic RAG)       %s\n", ColorBold, ColorGreen, ColorReset)
	fmt.Printf("%s=========================================================================%s\n", ColorBlue, ColorReset)
	fmt.Printf(" %s- Model Endpoint :%s %s\n", ColorWhite, ColorReset, cfg.APIBaseURL)
	fmt.Printf(" %s- Aktif Model    :%s %s\n", ColorWhite, ColorReset, cfg.ModelName)
	fmt.Printf(" %s- Guvenlik Modu  :%s %s%s[GUVENLI SANDBOX AKTIF - macOS Ayarlari Korunuyor]%s\n", ColorWhite, ColorReset, ColorBold, ColorGreen, ColorReset)
	fmt.Printf("%s-------------------------------------------------------------------------%s\n", ColorGray, ColorReset)
	fmt.Printf(" %sKullanilabilir Komutlar: /status (Ag Durumu), /reset (Sifirla), /help, exit%s\n", ColorGray, ColorReset)
	fmt.Printf("%s-------------------------------------------------------------------------%s\n", ColorGray, ColorReset)
}

func printHelp() {
	fmt.Printf("\n%s=== Yardım ve Örnek Komutlar ===%s\n", ColorBold, ColorReset)
	fmt.Println("1. Bilgi İsteme   : 'DHCP ayarlarını nerden yapabilirim?'")
	fmt.Println("2. Eylem / Değişim: 'DNS ayarımı 8.8.8.8 yap'")
	fmt.Println("3. Troubleshooting: 'İnternete erişemiyorum' veya '/troubleshoot'")
	fmt.Println("4. Durum Kontrolü : '/status'")
	fmt.Println("5. Çıkış          : 'exit' veya '/q'")
}

func printStatus(sb *tools.NetworkSandbox) {
	info, _, err := sb.GetInfo("Wi-Fi")
	if err != nil {
		fmt.Printf("%s[HATA] Ağ durumu alınamadı: %v%s\n", ColorRed, err, ColorReset)
		return
	}
	fmt.Printf("\n%s=== macOS Sandbox Ağ Arayüzü Durumu (Wi-Fi) ===%s\n", ColorBold, ColorReset)
	fmt.Println(info)
}
