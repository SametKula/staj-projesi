package tools

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ToolManager provides high-level tool execution and diagnostic workflows.
type ToolManager struct {
	sandbox *NetworkSandbox
}

// NewToolManager initializes the tool manager with the network sandbox.
func NewToolManager(sandbox *NetworkSandbox) *ToolManager {
	return &ToolManager{
		sandbox: sandbox,
	}
}

// ToolDefinition defines the JSON schema for LLM function calling.
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// GetToolDefinitions returns OpenAI-compatible function calling schemas.
func (tm *ToolManager) GetToolDefinitions() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "SetDNSTool",
				"description": "macOS üzerinde belirtilen ağ arayüzü (Wi-Fi/Ethernet) için DNS sunucu adreslerini ayarlar.",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"service": map[string]interface{}{
							"type":        "string",
							"description": "Ağ servisi adı (Örn: 'Wi-Fi' veya 'Ethernet')",
						},
						"dns_servers": map[string]interface{}{
							"type":        "array",
							"items":       map[string]interface{}{"type": "string"},
							"description": "Atanacak DNS sunucu IP adresleri listesi (Örn: ['8.8.8.8', '8.8.4.4'])",
						},
					},
					"required": []string{"dns_servers"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "GetDNSTool",
				"description": "macOS ağ servisine atanmış mevcut DNS sunucularını listeler.",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"service": map[string]interface{}{
							"type":        "string",
							"description": "Ağ servisi adı (Örn: 'Wi-Fi')",
						},
					},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "GetInterfaceInfoTool",
				"description": "Ağ arayüzünün IP, Alt Ağ Maskesi, Ağ Geçidi (Router) ve DHCP durumunu getirir.",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"service": map[string]interface{}{
							"type":        "string",
							"description": "Ağ servisi adı (Örn: 'Wi-Fi')",
						},
					},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "FlushDNSCacheTool",
				"description": "macOS sistem DNS önbelleğini temizler (dscacheutil ve mDNSResponder).",
				"parameters": map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "PingTool",
				"description": "Belirtilen hedef IP adresine veya alan adına ping atarak erişilebilirliği test eder.",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"target": map[string]interface{}{
							"type":        "string",
							"description": "Ping atılacak hedef (Örn: '8.8.8.8', 'google.com', '192.168.1.1')",
						},
						"count": map[string]interface{}{
							"type":        "integer",
							"description": "Gönderilecek paket sayısı (varsayılan: 3)",
						},
					},
					"required": []string{"target"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "NetworkTroubleshootTool",
				"description": "macOS üzerinde çok adımlı otomatik internet/ağ arıza teşhisi (Troubleshoot FSM) yürütür ve gerekirse otomatik DNS düzeltmesi uygular.",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"service": map[string]interface{}{
							"type":        "string",
							"description": "Test edilecek ağ arayüzü (varsayılan: 'Wi-Fi')",
						},
						"auto_remediate": map[string]interface{}{
							"type":        "boolean",
							"description": "DNS kaynaklı arızalarda otomatik olarak 8.8.8.8 DNS'i atayıp tekrar denensin mi? (varsayılan: true)",
						},
					},
				},
			},
		},
	}
}

// ExecuteTool runs the appropriate tool with given arguments.
func (tm *ToolManager) ExecuteTool(name string, argsJSON string) (string, error) {
	var args map[string]interface{}
	if argsJSON != "" {
		_ = json.Unmarshal([]byte(argsJSON), &args)
	}
	if args == nil {
		args = make(map[string]interface{})
	}

	service, _ := args["service"].(string)
	if service == "" {
		service = "Wi-Fi"
	}

	switch name {
	case "SetDNSTool":
		var dnsList []string
		if rawServers, ok := args["dns_servers"].([]interface{}); ok {
			for _, s := range rawServers {
				if str, ok := s.(string); ok {
					dnsList = append(dnsList, str)
				}
			}
		} else if rawStr, ok := args["dns_servers"].(string); ok {
			dnsList = strings.Fields(rawStr)
		}

		if len(dnsList) == 0 {
			return "", fmt.Errorf("SetDNSTool requires at least one DNS server IP")
		}
		return tm.sandbox.SetDNSServers(service, dnsList)

	case "GetDNSTool":
		out, _, err := tm.sandbox.GetDNSServers(service)
		return out, err

	case "GetInterfaceInfoTool":
		out, _, err := tm.sandbox.GetInfo(service)
		return out, err

	case "FlushDNSCacheTool":
		return tm.sandbox.FlushDNSCache()

	case "PingTool":
		target, ok := args["target"].(string)
		if !ok || target == "" {
			return "", fmt.Errorf("PingTool requires a target")
		}
		count := 3
		if c, ok := args["count"].(float64); ok && c > 0 {
			count = int(c)
		}
		res, err := tm.sandbox.Ping(target, count)
		if err != nil {
			return "", err
		}
		return res.Output, nil

	case "NetworkTroubleshootTool":
		autoFix := true
		if af, ok := args["auto_remediate"].(bool); ok {
			autoFix = af
		}
		return tm.RunTroubleshootWorkflow(service, autoFix)

	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

// TroubleshootStepReport contains diagnostic findings for one phase.
type TroubleshootStepReport struct {
	StepNumber  int    `json:"step_number"`
	Name        string `json:"name"`
	Target      string `json:"target"`
	Passed      bool   `json:"passed"`
	Description string `json:"description"`
	Details     string `json:"details"`
}

// TroubleshootFinalReport encapsulates the complete multi-step diagnosis.
type TroubleshootFinalReport struct {
	Timestamp          time.Time                 `json:"timestamp"`
	InterfaceTested    string                    `json:"interface_tested"`
	Steps              []*TroubleshootStepReport `json:"steps"`
	RootCauseIdentified string                   `json:"root_cause_identified"`
	AutoFixApplied     bool                      `json:"auto_fix_applied"`
	FixDetails         string                    `json:"fix_details"`
	FinalStatus        string                    `json:"final_status"`
}

// RunTroubleshootWorkflow executes the 4-step diagnostic FSM.
func (tm *ToolManager) RunTroubleshootWorkflow(service string, autoRemediate bool) (string, error) {
	report := &TroubleshootFinalReport{
		Timestamp:       time.Now(),
		InterfaceTested: service,
		Steps:           make([]*TroubleshootStepReport, 0),
		FinalStatus:     "FAILED",
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== 🔍 macOS Ağ Teşhis & Troubleshooting Raporu (%s) ===\n\n", service))

	// Step 1: Interface & Link State
	infoText, iface, err := tm.sandbox.GetInfo(service)
	step1 := &TroubleshootStepReport{
		StepNumber: 1,
		Name:       "Arayüz ve Donanım Bağlantı Kontrolü (Link State)",
		Target:     service,
		Passed:     err == nil && iface != nil && iface.Active && iface.LinkUp,
	}
	if !step1.Passed {
		step1.Description = "Ağ arayüzü kapalı veya fiziksel kablo/Wi-Fi bağlantısı yok."
		step1.Details = "Interface is DOWN."
		report.Steps = append(report.Steps, step1)
		report.RootCauseIdentified = "Fiziksel Arayüz Kapalı / Bağlantı Yok"
		sb.WriteString("❌ ADIM 1 [BAŞARISIZ]: Fiziksel ağ arayüzü bağlı değil veya kapalı.\n\n")
		return sb.String(), nil
	}
	step1.Description = fmt.Sprintf("Arayüz aktif. IP: %s, Ağ Geçidi (Router): %s", iface.IPv4Address, iface.Router)
	step1.Details = infoText
	report.Steps = append(report.Steps, step1)
	sb.WriteString(fmt.Sprintf("✅ ADIM 1 [BAŞARILI]: Arayüz aktif (IP: %s, Router: %s)\n", iface.IPv4Address, iface.Router))

	// Step 2: Gateway Ping
	router := iface.Router
	if router == "" {
		router = "192.168.1.1"
	}
	resGW, _ := tm.sandbox.Ping(router, 2)
	step2 := &TroubleshootStepReport{
		StepNumber: 2,
		Name:       "Varsayılan Ağ Geçidi (Gateway/Modem) Erişimi",
		Target:     router,
		Passed:     resGW != nil && resGW.Success,
	}
	if !step2.Passed {
		step2.Description = fmt.Sprintf("Ağ geçidine (%s) ulaşılamıyor. Modem veya yerel yönlendirici kapalı olabilir.", router)
		step2.Details = resGW.Output
		report.Steps = append(report.Steps, step2)
		report.RootCauseIdentified = "Yerel Ağ Geçidi / Modem Erişilemez"
		sb.WriteString(fmt.Sprintf("❌ ADIM 2 [BAŞARISIZ]: Ağ geçidine (%s) ping atılamadı.\n\n", router))
		return sb.String(), nil
	}
	step2.Description = fmt.Sprintf("Ağ geçidi (%s) yanıt veriyor. Gecikme: %.2f ms", router, resGW.AvgLatencyMs)
	report.Steps = append(report.Steps, step2)
	sb.WriteString(fmt.Sprintf("✅ ADIM 2 [BAŞARILI]: Ağ geçidi (%s) erişilebilir (Gecikme: %.2f ms)\n", router, resGW.AvgLatencyMs))

	// Step 3: External IP Ping (8.8.8.8)
	resExt, _ := tm.sandbox.Ping("8.8.8.8", 2)
	step3 := &TroubleshootStepReport{
		StepNumber: 3,
		Name:       "Dış Ağ IP Düzeyi İnternet Çıkışı (8.8.8.8)",
		Target:     "8.8.8.8",
		Passed:     resExt != nil && resExt.Success,
	}
	if !step3.Passed {
		step3.Description = "Modem erişilebilir ancak internet sağlayıcısı (ISP) veya uplink hattında genel kesinti var."
		step3.Details = resExt.Output
		report.Steps = append(report.Steps, step3)
		report.RootCauseIdentified = "Genel İnternet Kesintisi (ISP Uplink Down)"
		sb.WriteString("❌ ADIM 3 [BAŞARISIZ]: Dış IP'ye (8.8.8.8) ulaşılamıyor. İnternet servis sağlayıcı kesintisi.\n\n")
		return sb.String(), nil
	}
	step3.Description = fmt.Sprintf("İnternet IP çıkışı aktif. 8.8.8.8 yanıt verdi (Gecikme: %.2f ms)", resExt.AvgLatencyMs)
	report.Steps = append(report.Steps, step3)
	sb.WriteString(fmt.Sprintf("✅ ADIM 3 [BAŞARILI]: Dış internet IP çıkışı aktif (8.8.8.8, Gecikme: %.2f ms)\n", resExt.AvgLatencyMs))

	// Step 4: Domain Name & DNS Resolution Test (google.com)
	resDNS, _ := tm.sandbox.Ping("google.com", 2)
	step4 := &TroubleshootStepReport{
		StepNumber: 4,
		Name:       "Alan Adı Çözümleme ve DNS Testi (google.com)",
		Target:     "google.com",
		Passed:     resDNS != nil && resDNS.Success,
	}

	if !step4.Passed {
		step4.Description = "Dış IP erişimi çalışıyor ancak alan adları çözümlenemiyor (DNS Sunucu Hatası)."
		step4.Details = resDNS.Output
		report.Steps = append(report.Steps, step4)
		report.RootCauseIdentified = "Hatalı / Yanıt Vermeyen DNS Sunucusu"
		sb.WriteString("❌ ADIM 4 [BAŞARISIZ]: IP pingi başarılı ancak alan adı çözümlenemiyor ('google.com' çözülemedi).\n")
		sb.WriteString("📌 KÖK NEDEN TESPİTİ: Mevcut DNS sunucusu (" + strings.Join(iface.DNSServers, ", ") + ") yanıt vermiyor.\n\n")

		// Auto Remediation Loop
		if autoRemediate {
			sb.WriteString("⚡ [OTOMATİK DÜZELTME BAŞLATILIYOR]:\n")
			newDNS := []string{"8.8.8.8", "1.1.1.1"}
			setMsg, err := tm.sandbox.SetDNSServers(service, newDNS)
			if err == nil {
				sb.WriteString(fmt.Sprintf("  1. %s\n", setMsg))
				flushMsg, _ := tm.sandbox.FlushDNSCache()
				sb.WriteString(fmt.Sprintf("  2. %s\n", flushMsg))

				// Re-test DNS Ping
				retryRes, _ := tm.sandbox.Ping("google.com", 2)
				if retryRes != nil && retryRes.Success {
					report.AutoFixApplied = true
					report.FixDetails = "DNS sunucuları 8.8.8.8 ve 1.1.1.1 olarak güncellendi, önbellek temizlendi ve google.com başarıyla çözümlendi."
					report.FinalStatus = "RECOVERED"
					sb.WriteString(fmt.Sprintf("  3. 🔁 [YENİDEN TEST]: google.com başarıyla çözümlendi ve yanıt verdi (Gecikme: %.2f ms).\n\n", retryRes.AvgLatencyMs))
					sb.WriteString("🎉 SONUÇ: İnternet bağlantısı otomatik düzeltme (DNS güncellemesi) ile tamamen onarıldı!\n")
					return sb.String(), nil
				}
			}
		}

		return sb.String(), nil
	}

	step4.Description = fmt.Sprintf("DNS çözümleme ve alan adı erişimi sorunsuz. google.com yanıt verdi (Gecikme: %.2f ms)", resDNS.AvgLatencyMs)
	report.Steps = append(report.Steps, step4)
	report.FinalStatus = "HEALTHY"
	sb.WriteString(fmt.Sprintf("✅ ADIM 4 [BAŞARILI]: DNS çözümleme ve internet erişimi sağlıklı (google.com, Gecikme: %.2f ms)\n\n", resDNS.AvgLatencyMs))
	sb.WriteString("🎉 SONUÇ: Ağ bağlantısında herhangi bir sorun tespit edilmedi. Tüm katmanlar sağlıklı.\n")

	return sb.String(), nil
}
