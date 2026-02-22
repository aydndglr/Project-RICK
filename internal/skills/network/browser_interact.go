package network

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aydndglr/rick-agent-v3/internal/core/logger"
	"github.com/chromedp/chromedp"
)

func (t *BrowserTool) doInteract(ctx context.Context, args map[string]interface{}) (string, error) {
	targetUrl, _ := args["url"].(string)
	device, _ := args["device"].(string)

	actionsRaw, ok := args["actions"].([]interface{})
	if !ok {
		if singleAction, hasSingle := args["action"].(string); hasSingle {
			actionsRaw = []interface{}{map[string]interface{}{"action": singleAction, "selector": args["selector"], "value": args["value"]}}
		} else {
			return "", fmt.Errorf("HATA: 'interact' modu için 'actions' dizisi gereklidir")
		}
	}

	visible, _ := args["visible"].(bool)
	cCtx, cancel := GetChromeContext(ctx, visible)
	defer cancel()

	var report strings.Builder
	report.WriteString(fmt.Sprintf("🤖 RICK ULTRA-INTERACT RAPORU\n%s\n", strings.Repeat("=", 30)))

	// ========================================================
	// 1. EKRAN BOYUTU VE URL YÖNLENDİRME
	// ========================================================
	var initTasks []chromedp.Action

	if device == "mobile" {
		logger.Action("📱 Mobil Görünüm (375x812) aktif ediliyor.")
		initTasks = append(initTasks, chromedp.EmulateViewport(375, 812))
	} else {
		logger.Action("💻 Masaüstü Görünüm (1920x1080) aktif ediliyor.")
		initTasks = append(initTasks, chromedp.EmulateViewport(1920, 1080))
	}

	if targetUrl != "" {
		logger.Action("🌐 Tarayıcı Yönlendiriliyor: %s", targetUrl)
		initTasks = append(initTasks, chromedp.Navigate(targetUrl))
		report.WriteString(fmt.Sprintf("📍 Hedef: %s\n", targetUrl))
	}

	if len(initTasks) > 0 {
		if err := chromedp.Run(cCtx, initTasks...); err != nil {
			return "", fmt.Errorf("URL/Viewport ayarı başarısız: %v", err)
		}
	}

	// 🚀 RICK'İN GOD-MODE JS MOTORU (YouTube ve Modern Siteler Uyumlu)
	jsHelper := `
		const rickEngine = {
			// Engelleyici (Consent/Çerez) pencerelerini otomatik temizler
			killOverlays: () => {
				const keywords = ['kabul', 'accept', 'onayla', 'agree', 'ok', 'tamam', 'understand', 'i agree', 'all set'];
				const elements = Array.from(document.querySelectorAll('button, [role="button"], a, yt-formatted-string'));
				const consentBtn = elements.find(el => {
					const txt = el.innerText ? el.innerText.toLowerCase() : "";
					return keywords.some(k => txt.includes(k)) && el.offsetHeight > 0;
				});
				if(consentBtn) {
					consentBtn.click();
					return "OVERLAY_KILLED";
				}
				return "CLEAN";
			},

			// Gelişmiş Seçici: Metni bulur ama gerekirse tıklanabilir ebeveyne (parent) çıkar
			find: (sel) => {
				if(!sel) return null;
				let el = null;
				if(sel.startsWith('text=')){
					const target = sel.substring(5).toLowerCase().trim();
					const candidates = Array.from(document.querySelectorAll('a, button, [role="button"], yt-formatted-string, span, div, h1, h2, h3'));
					const matched = candidates.filter(e => e.innerText && e.innerText.toLowerCase().includes(target) && e.offsetHeight > 0);
					if(matched.length > 0) {
						// En spesifik (metni en kısa olan) elementi seç
						el = matched.sort((a,b) => a.innerText.length - b.innerText.length)[0];
					}
				} else {
					el = document.querySelector(sel);
				}

				// recursive check: Eğer bulduğumuz şey sadece metinse (span/div), tıklanabilir parent'ı var mı bak
				if(el && !['BUTTON', 'A', 'INPUT'].includes(el.tagName)) {
					let clickableParent = el.closest('button, a, [role="button"]');
					if(clickableParent) el = clickableParent;
				}
				return el;
			},

			// Gerçek Kullanıcı Simülasyonu (MouseDown + MouseUp + Click)
			superClick: (el) => {
				if(!el) return "NOT_FOUND";
				el.scrollIntoView({block: 'center', behavior: 'instant'});
				const events = ['mousedown', 'mouseup', 'click'];
				events.forEach(evName => {
					const ev = new MouseEvent(evName, {
						view: window,
						bubbles: true,
						cancelable: true,
						buttons: 1
					});
					el.dispatchEvent(ev);
				});
				return "OK";
			}
		};
	`

	// Sayfa yüklendikten sonra engelleri temizlemek için kısa bir nefes al
	chromedp.Run(cCtx, chromedp.Sleep(2*time.Second), chromedp.Evaluate(jsHelper+`rickEngine.killOverlays()`, nil))

	// ========================================================
	// 2. MAKRO ADIMLARI
	// ========================================================
	for i, actRaw := range actionsRaw {
		act, ok := actRaw.(map[string]interface{})
		if !ok { continue }

		action, _ := act["action"].(string)
		selector, _ := act["selector"].(string)
		value, _ := act["value"].(string)

		var stepTasks chromedp.Tasks
		var screenshotBuf []byte
		var resultVar interface{}
		var jsResult string

		// 🛡️ DİNAMİK ZAMAN AŞIMI (TIMEOUT)
		timeoutDuration := 15 * time.Second
		if action == "wait" {
			waitSec := 2
			fmt.Sscanf(value, "%d", &waitSec)
			timeoutDuration = time.Duration(waitSec+5) * time.Second
		}

		switch action {
		case "click":
			logger.Action("🖱️ [%d] Akıllı Tıklama: %s", i+1, selector)
			js := fmt.Sprintf(`(function(){ 
				%s
				let target = rickEngine.find("%s");
				return rickEngine.superClick(target);
			})()`, jsHelper, selector)
			stepTasks = append(stepTasks, chromedp.Evaluate(js, &jsResult))

		case "type":
			logger.Action("⌨️ [%d] Yazılıyor (%s): %s", i+1, selector, value)
			js := fmt.Sprintf(`(function(){ 
				%s
				let el = rickEngine.find("%s"); 
				if(!el) return "NOT_FOUND"; 
				el.scrollIntoView({block: 'center'});
				el.focus(); el.value = "%s"; 
				el.dispatchEvent(new Event('input', {bubbles: true})); 
				el.dispatchEvent(new Event('change', {bubbles: true})); 
				return "OK";
			})()`, jsHelper, selector, value)
			stepTasks = append(stepTasks, chromedp.Evaluate(js, &jsResult))

		case "enter":
			logger.Action("⌨️ [%d] Enter basılıyor: %s", i+1, selector)
			js := fmt.Sprintf(`(function(){ 
				%s
				let el = rickEngine.find("%s"); 
				if(!el) return "NOT_FOUND"; 
				if(el.form) { el.form.submit(); return "OK"; }
				let ev = new KeyboardEvent('keydown', {key: 'Enter', code: 'Enter', keyCode: 13, which: 13, bubbles: true});
				el.dispatchEvent(ev);
				return "OK";
			})()`, jsHelper, selector)
			stepTasks = append(stepTasks, chromedp.Evaluate(js, &jsResult))

		case "keypress":
			logger.Action("🔠 [%d] Tuşa basılıyor: '%s'", i+1, value)
			stepTasks = append(stepTasks, chromedp.KeyEvent(value))

		case "media_play":
			logger.Action("▶️ [%d] ZORUNLU OYNATMA (Muted Fallback)", i+1)
			js := `(function(){ 
				let v = document.querySelector('video'); 
				if(v){ 
					v.muted = false;
					v.play().catch(() => {
						v.muted = true;
						v.play();
					});
					return "OK"; 
				} 
				return "NOT_FOUND"; 
			})()`
			stepTasks = append(stepTasks, chromedp.Evaluate(js, &jsResult))

		case "hover":
			logger.Action("👆 [%d] Hover: %s", i+1, selector)
			js := fmt.Sprintf(`(function(){ 
				%s
				let el = rickEngine.find("%s"); 
				if(el) { 
					el.dispatchEvent(new MouseEvent('mouseover', {bubbles: true})); 
					return "OK";
				}
				return "NOT_FOUND";
			})()`, jsHelper, selector)
			stepTasks = append(stepTasks, chromedp.Evaluate(js, &jsResult))

		case "wait":
			waitSec := 2
			fmt.Sscanf(value, "%d", &waitSec)
			logger.Action("⏳ [%d] Bekleniyor: %d sn", i+1, waitSec)
			stepTasks = append(stepTasks, chromedp.Sleep(time.Duration(waitSec)*time.Second))

		case "get_text":
			logger.Action("📖 [%d] Metin okunuyor: %s", i+1, selector)
			js := fmt.Sprintf(`(function(){ 
				%s
				let el = rickEngine.find("%s"); 
				return el ? el.innerText.trim() : "NOT_FOUND";
			})()`, jsHelper, selector)
			var textVal string
			stepTasks = append(stepTasks, chromedp.Evaluate(js, &textVal))
			resultVar = &textVal

		case "multi_text":
			logger.Action("📚 [%d] Toplu metin çekiliyor: %s", i+1, selector)
			js := fmt.Sprintf(`Array.from(document.querySelectorAll("%s")).map(el => el.innerText.trim())`, selector)
			var listVal []string
			stepTasks = append(stepTasks, chromedp.Evaluate(js, &listVal))
			resultVar = &listVal

		// ========================================================
		// 🚀 YENİ GOD-MODE RADAR ÖZELLİĞİ: CSS TAHMİN ETMEYİ BİTİREN KOD
		// ========================================================
		case "get_links":
			logger.Action("🔗 [%d] Sayfadaki okunabilir başlık ve linkler toplanıyor...", i+1)
			js := `(function(){ 
				const links = Array.from(document.querySelectorAll('a'));
				const results = [];
				links.forEach(a => {
					// Gereksiz boşlukları ve satır atlamalarını temizle
					const txt = a.innerText.trim().replace(/\n/g, ' ');
					
					// Sadece içi dolu, görünür ve belli bir uzunluktaki anlamlı metinleri al (Haber başlıkları vs.)
					if(txt.length > 15 && a.offsetHeight > 0) {
						results.push(txt);
					}
				});
				
				// Benzersiz (Unique) olanları al ve çok fazla şişmemesi için ilk 20 tanesini döndür
				return [...new Set(results)].slice(0, 20);
			})()`
			var linkList []string
			stepTasks = append(stepTasks, chromedp.Evaluate(js, &linkList))
			resultVar = &linkList

		case "screenshot":
			if value == "" { value = fmt.Sprintf("web_snap_%d.png", time.Now().Unix()) }
			// Dosya uzantısı kontrolü (Rick bazen .png yazmayı unutuyor)
			if !strings.HasSuffix(value, ".png") && !strings.HasSuffix(value, ".jpg") {
				value += ".png"
			}
			logger.Action("📸 [%d] SS Alınıyor: %s", i+1, value)
			stepTasks = append(stepTasks, chromedp.CaptureScreenshot(&screenshotBuf))
		}

		// 🛡️ ÇALIŞTIR
		stepCtx, stepCancel := context.WithTimeout(cCtx, timeoutDuration)
		err := chromedp.Run(stepCtx, stepTasks)
		stepCancel() 

		if err != nil {
			errStr := fmt.Errorf("Adım %d (%s) Başarısız: %v", i+1, action, err)
			report.WriteString(fmt.Sprintf("❌ %s\n", errStr.Error()))
			return report.String(), errStr
		}

		if jsResult == "NOT_FOUND" {
			errStr := fmt.Errorf("Adım %d (%s) Başarısız: Hedef bulunamadı", i+1, action)
			report.WriteString(fmt.Sprintf("❌ %s\n", errStr.Error()))
			return report.String(), errStr
		}

		report.WriteString(fmt.Sprintf("✅ Adım %d [%s] Tamamlandı.\n", i+1, action))

		// Dosya işlemleri
		if action == "screenshot" && len(screenshotBuf) > 0 {
			savePath := filepath.Join("logs", value)
			os.MkdirAll("logs", 0755)
			os.WriteFile(savePath, screenshotBuf, 0644)
			report.WriteString(fmt.Sprintf("   📸 SS Kaydedildi: %s\n", savePath))
		}
		if resultVar != nil {
			switch v := resultVar.(type) {
			case *string: report.WriteString(fmt.Sprintf("   📄 Çıktı: %s\n", *v))
			case *[]string:
				report.WriteString(fmt.Sprintf("   📚 Çıktı (%d öğe):\n", len(*v)))
				for idx, item := range *v { report.WriteString(fmt.Sprintf("      %d. %s\n", idx+1, item)) }
			}
		}
	}

	var finalURL, finalTitle string
	chromedp.Run(cCtx, chromedp.Location(&finalURL), chromedp.Title(&finalTitle))
	report.WriteString(strings.Repeat("-", 30) + "\n")
	report.WriteString(fmt.Sprintf("🏁 Son Durak: %s\n📜 Başlık: %s", finalURL, finalTitle))

	return report.String(), nil
}