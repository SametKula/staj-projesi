# STAJ DEFTERİ / GÜNLÜĞÜ

**Öğrenci / Stajyer:** [Ad Soyad]  
**Staj Konusu:** macOS Özelinde Agentic RAG Tabanlı Akıllı Ağ Yönetimi ve Otomatik Arıza Giderme (Troubleshooting) Asistanı Geliştirme  
**Kullanılan Teknolojiler & Araçlar:** Go (Golang), Firebase Genkit SDK, Yerel LLM (Gemma-4-E4B-it QAT GGUF), OpenAI-Uyumlu Local API, macOS Ağ Yığını (networksetup, scutil, ping, dscacheutil), Docker, Git.  
**Geliştirme Metodolojisi:** Araştırma ve Sistem Tasarımı $\rightarrow$ Yapay Zeka Ajanı (AI Agent Pair Programming) ile Kodlama ve Hata Çözümü $\rightarrow$ Doğrulama ve Test.

---

## 1. GÜN: Proje Kapsamının Belirlenmesi, Mimari Tasarım ve Teknoloji Araştırması

### Günlük Çalışma Özeti
Stajımın ilk gününde, macOS işletim sisteminde kullanıcıların ağ yapılandırma ve internet arıza sorunlarını çözebilecek akıllı bir asistan projesi geliştirmeye karar verdim. Sistem sadece statik soru cevaplayan geleneksel bir chatbot olmak yerine; kullanıcının niyetini anlayıp gerekirse işletim sistemi üzerinde güvenli aksiyonlar alabilen ve arıza teşhisi yapabilen bir **Agentic RAG** (Ajanik Bilgi Getirmeli Üretim) mimarisine sahip olmalıydı.

### Yaptığım Araştırmalar ve Aldığım Kararlar
1. **Model ve Altyapı Seçimi:** Bulut tabanlı API'ler yerine veri gizliliğini korumak ve tamamen yerelde çalışabilmek amacıyla yerel ağda çalışan `http://10.2.2.115:8080/v1` adresindeki **Gemma-4 (gemma-4-E4B-it-qat-nvfp4.gguf)** modelini kullanmaya karar verdim.
2. **Yazılım Dili ve Çatı:** Yüksek performans, hafif çalışma alanı ve sistem çağrılarıyla uyumluluk nedeniyle projeyi **Go** dili ve **Firebase Genkit** mimarisi prensipleriyle geliştirmeyi kararlaştırdım.
3. **Temel Senaryolar:** Projenin 3 temel davranış sergilemesini belirledim:
   - *Salt Bilgi Sağlama (Informational):* DHCP ayarları gibi konularda dokümantasyona dayalı yönlendirme.
   - *Eylem Yürütme (Actionable):* Kullanıcı "DNS ayarımı 8.8.8.8 yap" dediğinde sistem ayarını değiştirebilme.
   - *Otomatik Teşhis (Troubleshoot):* İnternet kopukluklarında adım adım teşhis koyup kök nedeni bulma ve otomatik onarma.

### Yapay Zeka Ajanına Verilen Görev & Yapılan Uygulama
- Belirlediğim gereksinimleri AI ajanına aktararak projenin `chatbot-mimari.md` mimari gereksinim dokümanını ve proje klasör yapısını (`cmd/`, `internal/agent`, `internal/rag`, `internal/tools`, `data/knowledge/`, `docs/`) oluşturtmasını sağladım.
- Sistem mimarisini Mermaid formatında görselleştirdik.

### Karşılaşılan Sorun ve Çözüm Yöntemi
* **Sorun:** İlk oluşturulan Mermaid mimari diyagramı yatayda çok geniş olduğu için staj raporuna eklerken ekran görüntüsü kenarlardan taşıyor ve okunaksız hale geliyordu.
* **Çözüm:** AI ajanı ile birlikte Mermaid akışını dikey hiyerarşiye (`flowchart TD`) dönüştürdük ve `browser` render motoru ile rapor kalitesinde (3268x1470 piksel) yüksek çözünürlüklü PNG görseli (`sistem-mimarisi.png`) ürettik.

---

## 2. GÜN: Güvenlik Analizi, Sandbox Ortamı ve macOS Ağ Simülatörü Tasarımı

### Günlük Çalışma Özeti
İkinci günün odağı sistem güvenliğiydi. Asistanın macOS üzerinde `networksetup` ve DNS ayarlarını değiştirmesi gerekiyordu; ancak geliştirme ve test sırasında staj yaptığım bilgisayarın gerçek ağ ayarlarının bozulması veya bağlantının kopması büyük bir riskti. Bu sebeple izole ve gerçekçi bir **macOS Network Sandbox** motoru tasarladım.

### Yaptığım Araştırmalar ve Aldığım Kararlar
1. **Sandbox Mantığı:** Gerçek sistem dosyalarını değiştirmek yerine macOS ağ servislerini (`Wi-Fi`, `Ethernet`, `en0`, `en1`), IP yapılandırmalarını, DNS sunucularını ve DHCP durumlarını bellekte ve JSON durum dosyasında (`data/sandbox_state.json`) simüle eden bir motor tasarladım.
2. **Ping ve DNS Çözümleme Simülasyonu:** `ping` komutunun ağ geçidi (`192.168.1.1`), dış IP (`8.8.8.8`) ve alan adı (`google.com`) için farklı durumlar üretmesini kurguladım. Eğer DNS sunucusu hatalı tanımlanmışsa IP pingi başarılı olurken alan adı çözümlemesinin başarısız olması kuralını getirdim.

### Yapay Zeka Ajanına Verilen Görev & Yapılan Uygulama
- AI ajanına `internal/tools/sandbox.go` dosyasını kodlattım.
- Motor içerisinde `GetDNSServers`, `SetDNSServers`, `GetInfo`, `FlushDNSCache`, `Ping` ve `ResetToDefault` metodları geliştirildi.
- Her işlemin zaman damgası ve başarı durumu ile kaydedildiği bir denetim günlüğü (`LogEntry`) eklendi.

### Karşılaşılan Sorun ve Çözüm Yöntemi
* **Sorun:** Sandbox sıfırlama işlemi (`ResetToDefault`) çağrıldığında Go çalışma zamanında `fatal error: all goroutines are asleep - deadlock!` hatası meydana geldi.
* **Çözüm:** Kod incelendiğinde `ResetToDefault` metodunun `RWMutex.Lock()` aldığı ve ardından kendi içinde tekrar kilit alan `loadOrCreate()` fonksiyonunu çağırdığı tespit edildi. AI ajanı ile fonksiyonu `loadOrCreateLocked()` olarak ayrıştırarak kilit re-entrancy sorununu çözdük.

---

## 3. GÜN: Çift Katmanlı (Dual-Layer) RAG Veriseti ve Arama Motoru Geliştirme

### Günlük Çalışma Özeti
Üçüncü gün, RAG (Retrieval-Augmented Generation) mekanizmasının veri altyapısını ve arama motorunu geliştirdim. Klasik RAG sistemlerinde dokümanlar sadece insanlara yönelik metinler içerir. Ancak bir ajanın bir tool'u ne zaman çağıracağını ve hangi parametreleri kullanacağını bilmesi için dokümanların yapısal meta-veri de içermesi gerekiyordu.

### Yaptığım Araştırmalar ve Aldığım Kararlar
1. **Çift Katmanlı (Dual-Layer) Chunk Standardı:** Her doküman parçasına iki ayrı bölüm ekledim:
   - `user_guide`: Kullanıcıya gösterilecek adım adım anlaşılır rehber metin.
   - `agent_spec`: Modelin hangi tool'u (`supported_tools`), hangi niyetle (`trigger_intents`) ve hangi parametrelerle çağıracağını belirten JSON meta-verisi.
2. **Veri Kapsamı:** macOS DHCP ayarları, DNS yönetimi, Ethernet arıza giderme, Wi-Fi tanılama ve macOS terminal CLI komutları olmak üzere 5 temel kategoride veriseti hazırlamaya karar verdim.

### Yapay Zeka Ajanına Verilen Görev & Yapılan Uygulama
- `data/knowledge/` dizini altında `01_dhcp_configuration.json`, `02_dns_management.json`, `03_ethernet_troubleshooting.json` ve `04_wifi_and_cli_reference.json` dosyaları oluşturuldu.
- `internal/rag/retriever.go` dosyası kodlanarak TF-IDF, anahtar kelime ve niyet eşleştirme (intent scoring) yapan hibrit arama motoru yazıldı.
- `FormatContext` fonksiyonu ile arama sonuçları modele çift katmanlı prompt bağlamı olarak aktarılacak formata getirildi.

### Karşılaşılan Sorun ve Çözüm Yöntemi
* **Sorun:** Kullanıcı "DHCP ayarlarını nereden yapabilirim?" diye sorduğunda genel ağ dokümanları da benzer skorlar alabiliyor ve alakasız chunk'lar prompt'a dahil olabiliyordu.
* **Çözüm:** Retriever içerisine niyet bazlı dinamik ağırlıklandırma (Intent Booster) eklendi; "dhcp", "dns", "internete erişemiyorum" gibi kritik kalıplar tespit edildiğinde ilgili kategoriye ek güven puanı (+5.0/+6.0) verilmesi sağlandı.

---

## 4. GÜN: macOS Ağ Araçları (Tools) ve Otomatik Teşhis Durum Makinesi (FSM)

### Günlük Çalışma Özeti
Dördüncü gün, ajanın kullanacağı araçların (Tools) ve otomatik arıza teşhis algoritmasının (Troubleshooting FSM) geliştirilmesine odaklandım. Kullanıcı "internete erişemiyorum" dediğinde sistemin körü körüne tahmin yürütmesi yerine, bir ağ mühendisi gibi adım adım teşhis koyması gerekiyordu.

### Yaptığım Araştırmalar ve Aldığım Kararlar
1. **4 Adımlı Teşhis Modeli:**
   - *Adım 1 (Arayüz):* Wi-Fi/Ethernet linki aktif mi ve IP alınmış mı?
   - *Adım 2 (Ağ Geçidi):* Varsayılan yönlendiriciye (`192.168.1.1`) ping gidiyor mu?
   - *Adım 3 (Dış IP):* İnternet omurgasına (`8.8.8.8`) ping gidiyor mu? (ISP kesintisi kontrolü)
   - *Adım 4 (DNS Çözümleme):* `google.com` alan adı çözümlenebiliyor mu?
2. **Akıllı Otomatik Onarım (Auto-Remediation):** Eğer Adım 3 başarılı ancak Adım 4 başarısız ise sorunun %100 DNS kaynaklı olduğu anlaşıldığı için ajanın otomatik olarak DNS'i `8.8.8.8` / `1.1.1.1` yapıp, DNS önbelleğini temizlemesi ve testi tekrarlaması kuralını getirdim.

### Yapay Zeka Ajanına Verilen Görev & Yapılan Uygulama
- `internal/tools/manager.go` dosyasında `ToolManager` ve `RunTroubleshootWorkflow` fonksiyonları kodlandı.
- `SetDNSTool`, `GetDNSTool`, `GetInterfaceInfoTool`, `FlushDNSCacheTool`, `PingTool` ve `NetworkTroubleshootTool` araçları OpenAI Function Calling standartlarına uygun JSON şemalarıyla tanımlandı.

### Karşılaşılan Sorun ve Çözüm Yöntemi
* **Sorun:** Çok adımlı teşhis sonucunun modele ham metin olarak verilmesi durumunda model bazen arızanın çözüldüğünü anlamayıp eski hata mesajını tekrarlayabiliyordu.
* **Çözüm:** Teşhis durum makinesi çıktısı yapılandırılmış, teknik ve adım adım başarı göstergeleri içeren bir teşhis raporuna (`TroubleshootStepReport`) dönüştürüldü; böylece modelin 2. tur sentezde onarımı doğru anlaması sağlandı.

---

## 5. GÜN: Yerel LLM Entegrasyonu (Gemma 4) ve Ajan Akış Orkestrasyonu

### Günlük Çalışma Özeti
Beşinci gün, sistemin beyni olan Ajan Akış Orkestratörünü (`Orchestrator`) ve yerel LLM istemcisini geliştirdim. Yerel model sunucusu olan `http://10.2.2.115:8080/v1` adresindeki `gemma-4-E4B-it-qat-nvfp4.gguf` multimodal modeli ile haberleşmeyi kurduk.

### Yaptığım Araştırmalar ve Aldığım Kararlar
1. **İki Aşamalı Model Akışı (Two-Round Inference):**
   - *1. Tur:* Kullanıcı sorusu + RAG bağlamı + Tool şemaları modele verilir $\rightarrow$ Model bir Tool çağrısı (`tool_calls`) üretir veya doğrudan cevap verir.
   - *2. Tur:* Eğer Tool çağrıldıysa, sandbox üzerinde çalıştırılan tool sonucu `role: "tool"` olarak modele iletilir $\rightarrow$ Model sonucu analiz ederek kullanıcıya nihai açıklamayı üretir.
2. **Yedekli ve Güvenli Çalışma (Fallback Strategy):** Yerel model sunucusunun zaman aşımına uğraması veya beklenmeyen formatta yanıt vermesi durumuna karşı kural tabanlı deterministik bir karar motoru yedek mekanizma olarak eklendi.

### Yapay Zeka Ajanına Verilen Görev & Yapılan Uygulama
- `internal/agent/client.go` dosyasında OpenAI uyumlu HTTP istemcisi kodlandı.
- `internal/agent/orchestrator.go` dosyasında RAG arama, sistem promptu enjeksiyonu, tool yönlendirmesi ve konuşma geçmişi (`history`) yönetimini sağlayan `Orchestrator` yapısı yazıldı.

### Karşılaşılan Sorun ve Çözüm Yöntemi
* **Sorun:** Model yerel endpoint üzerinde `..\models\gemma4\gemma-4-E4B-it-qat-nvfp4.gguf` adı altında servis ediliyordu; standart isteklerde model adı uyuşmazlığı yaşanabiliyordu.
* **Çözüm:** `config.go` içerisinde model adı dinamik ortam değişkeninden (`LLM_MODEL`) okunacak şekilde yapılandırıldı ve sunucunun döndürdüğü tam model kimliği ile eşleştirildi.

---

## 6. GÜN: İnteraktif Terminal CLI Arayüzü ve Kullanıcı Deneyimi Geliştirme

### Günlük Çalışma Özeti
Altıncı gün, uygulamanın terminal üzerinden kolayca kullanılabilmesi, durumunun takip edilebilmesi ve tüm RAG/Tool işlemlerinin şeffaf olarak izlenebilmesi için gelişmiş bir interaktif CLI (Komut Satırı Arayüzü) geliştirdim.

### Yaptığım Araştırmalar ve Aldığım Kararlar
1. **Terminal Kullanıcı Deneyimi (TUI / CLI):**
   - Model endpoint'i, aktif model, güvenlik modu (`[GÜVENLİ SANDBOX AKTİF]`) bilgilerini içeren profesyonel bir başlangıç banner'ı.
   - RAG sorgularında hangi chunk'ların hangi güven puanıyla eşleştiğinin renklendirilmiş olarak gösterilmesi.
   - Çalıştırılan tool'ların ve parametrelerinin sarı/yeşil durum göstergeleriyle anlık loglanması.
2. **Kısayol Komutları:** Kullanıcının `/status` ile ağ durumunu görmesi, `/reset` ile sandbox'ı sıfırlaması, `/troubleshoot` ile doğrudan arıza tespitini başlatabilmesi sağlandı.

### Yapay Zeka Ajanına Verilen Görev & Yapılan Uygulama
- `main.go` dosyası terminal renk kodları (ANSI escape sequences), argüman bayrakları (`-query`, `-sandbox`, `-reset-sandbox`) ve komut dinleme döngüsü ile kodlandı.
- Uygulama `go build -o network-assistant main.go` komutuyla bağımsız bir çalıştırılabilir ikili dosyaya (binary) derlendi.

### Karşılaşılan Sorun ve Çözüm Yöntemi
* **Sorun:** Kullanıcı uzun metinler girdiğinde `fmt.Scan` fonksiyonu boşluklardan sonraki kelimeleri kesiyordu.
* **Çözüm:** `bufio.NewScanner(os.Stdin)` yapısına geçilerek satır bazlı (line-buffered) okuma sağlandı ve tüm doğal dil cümleleri eksiksiz işlendi.

---

## 7. GÜN: Uçtan Uca Senaryo Testleri, Doğrulama ve Staj Raporlaması

### Günlük Çalışma Özeti
Stajımın son gününde, geliştirdiğim macOS Agentic RAG Ağ Asistanını tüm temel senaryolar üzerinde uçtan uca test ettim, elde ettiğim sonuçları doğruladım ve staj dokümantasyonunu tamamladım.

### Gerçekleştirilen Testler ve Çıktılar

#### Test 1: Salt Bilgi Getirme Senaryosu (Informational RAG)
* **Girdi:** `"dhcp ayarlarını nerden yapabilirim?"`
* **Gözlem:** RAG motoru 16.5 güven skoruyla `macOS DHCP Yapılandırması ve Ayar Konumu` dokümanını getirdi. Model herhangi bir sistem aracını gereksiz yere çalıştırmadı ve kullanıcıya Sistem Ayarları > Ağ > TCP/IP adımlarını adım adım Türkçe olarak anlattı. (Başarılı)

#### Test 2: Eylem Yürütme Senaryosu (Actionable Tool Calling)
* **Girdi:** `"dns ayarımı 8.8.8.8 yap"`
* **Gözlem:** RAG motoru `macOS DNS Ayarları Değiştirme ve Yönetimi` chunk'ını getirdi. Model `SetDNSTool` aracını `{"dns_servers":["8.8.8.8"],"service":"Wi-Fi"}` parametreleriyle tetikledi. Sandbox üzerinde DNS güvenli bir şekilde `8.8.8.8` yapıldı ve kullanıcıya teyit mesajı iletildi. (Başarılı)

#### Test 3: Otomatik Arıza Giderme Senaryosu (Troubleshoot & Auto-Fix)
* **Girdi:** `"internete erişemiyorum"`
* **Gözlem:** Sandbox bilerek hatalı DNS (`10.255.255.1`) ile başlatıldı. Model `NetworkTroubleshootTool` aracını çalıştırdı.
  1. Arayüz kontrolü $\rightarrow$ Başarılı
  2. Gateway pingi $\rightarrow$ Başarılı
  3. Dış IP (8.8.8.8) pingi $\rightarrow$ Başarılı
  4. Domain pingi (google.com) $\rightarrow$ Başarısız (DNS yanıt vermiyor)
  5. *Otomatik Onarım:* DNS `8.8.8.8` ve `1.1.1.1` yapıldı, önbellek temizlendi ve google.com yeniden test edilerek bağlantı sağlandı.
  6. Model kullanıcıya arızanın DNS kaynaklı olduğunu ve otomatik olarak düzeltildiğini raporladı. (Başarılı)

### Kazanımlar ve Sonuç
Bu staj projesi kapsamında; modern yapay zeka ajan mimarilerini (Agentic RAG), Go dilinde sistem seviyesi programlamayı, yerel LLM entegrasyonunu ve güvenli yazılım geliştirme prensiplerini (Sandbox izolasyonu) uçtan uca deneyimledim ve çalışan bir ürün ortaya koydum.
