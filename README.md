# 🧪 Project RICK

> *"Wubba Lubba Dub Dub!"* 

**Project RICK**, Go (Golang) tabanlı, yerel LLM modelleri (Ollama) ile çalışan, kendi kendine karar verebilen, kod yazabilen ve sistem yönetimi yapabilen otonom bir yapay zeka ajanıdır. 

Sıradan chatbotların aksine Rick; işletim sistemi üzerinde tam yetkiye sahiptir, araç (tool) kullanabilir ve WhatsApp üzerinden gerçek zamanlı iletişim kurabilir.

---

## 🚀 Temel Yetenekler (v3.0)

Rick, **"Re-Act" (Reasoning + Acting)** mimarisi ile çalışır. Bir görevi önce düşünür, planlar, uygun aracı seçer ve uygular.

* **🧠 Beyin & Hafıza:**
    * **LLM Bağımsız:** Ollama (Llama 3, Mistral) veya Claude API ile çalışabilir.
    * **Vektör Hafıza (RAG):** Geçmiş konuşmaları ve deneyimleri hatırlar (`LocalVectorDB`).
    * **Dinamik Sistem Analizi:** Çalıştığı işletim sistemini, kullanıcıyı ve ortamı analiz eder.

* **🛠️ Araç Seti (Toolbox):**
    * **Sistem Kontrolü:** Terminal komutları çalıştırma, uygulama açma/kapama, virüs taraması (Windows Defender).
    * **Ofis Otomasyonu:** Outlook mail okuma/gönderme/düzenleme, Excel ve Word dosya işlemleri (Python Bridge ile).
    * **Dosya Yönetimi:** Dosya oluşturma, okuma, taşıma, silme ve proje ağacı analizi.
    * **Web Yetenekleri:** DuckDuckGo ile internet araması, sayfa içeriği okuma (Scraping).
    * **Kodlama & Yama:** Kendi kodunu analiz edip (`lint`), hatalı blokları tespit edip "Smart Patcher" ile onarabilir.

* **📱 İletişim:**
    * **WhatsApp Entegrasyonu:** `whatsmeow` kütüphanesi ile QR kod üzerinden bağlanır, sahibinden gelen mesajları dinler ve yanıtlar.

---

## 🏗️ Mimari & Teknoloji Yığını

Proje hibrit bir yapıda geliştirilmiştir:

* **Core:** Golang (Yüksek performans, concurrency ve sistem erişimi).
* **Scripting:** Python (Veri analizi, otomasyon ve karmaşık kütüphane gereksinimleri için Go içinden çağrılır).
* **Database:** JSON tabanlı yerel Vektör Veritabanı (Harici kurulum gerektirmez).

### Dizin Yapısı
```text
/cmd/rick        # Ana giriş noktası (Main)
/internal
  /agent         # Ajan döngüsü ve karar mekanizması
  /brain         # LLM entegrasyonu (Ollama/Claude)
  /commands      # Rick'in kullanabildiği tüm araçlar (Tools)
  /memory        # Vektör veritabanı ve RAG motoru
  /communication # WhatsApp ve diğer iletişim kanalları
```

## Yol Haritası (Roadmap to v4.0)
Şu an üzerinde çalışılan özellikler:

[ ] Asenkron Çoklu Görev (Non-blocking Async): Uzun süren işlemlerin (örn. virüs taraması) arka planda çalıştırılması ve ana akışın bloklanmaması.

[ ] Canlı Geri Bildirim (Streaming): İşlem sürerken WhatsApp üzerinden anlık durum (progress) bildirimleri.

[ ] Görev Yöneticisi (Task Manager): Başlatılan arka plan görevlerinin sorgulanması ve iptal edilebilmesi.

## Lisans ve Kullanım
BU PROJE AÇIK KAYNAK DEĞİLDİR.

Tüm hakları saklıdır © 2026.
Bu depodaki kodlar sadece portföy sunumu ve inceleme amacıyla paylaşılmıştır. İzinsiz kopyalanması, dağıtılması, ticari veya bireysel projelerde kullanılması kesinlikle yasaktır.

Detaylar için LICENSE dosyasına bakınız.

---
