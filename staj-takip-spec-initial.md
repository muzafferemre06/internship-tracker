# Staj Takip Otomasyonu — Spec (v1 / MVP)

## Amaç
Havelsan, Turkcell, İntertech (ve benzeri 3-4 öncelikli şirket) kariyer/staj
sayfalarını periyodik olarak kontrol edip, kullanıcının profiline (2. sınıf
CTIS öğrencisi, network/backend/sysadmin odağı) uygun yeni ilanları tespit
ederek bildirim gönderen sürekli çalışan bir sistem.

## Kapsam (MVP)
- **Kolay tier (kariyer.net üzerinden, statik, login gerekmiyor):**
  Meteksan, ASELSAN, STM, Baykar, Samsung — hepsi aynı ortak
  kariyer.net-scraper mantığıyla taranabilir. Bazılarının (ASELSAN,
  Baykar, Samsung) birden fazla profili var (iştirak/şube başına ayrı
  sayfa) — scraper bir şirket için birden fazla profil URL'si takip
  edebilmeli.
- **Zor tier (özel engelli, v1'de düşük öncelik):** Havelsan
  (savunmakariyer.com, bot-koruması/418 hatası), İntertech
  (career.intertech.com.tr, login duvarı)
- **Belirsiz tier:** Turkcell (bilgi sayfası statik ama gerçek ATS —
  kariyer.turkcell.com.tr — henüz test edilmedi)
- Kapsam dışı (v1): Youthall entegrasyonu, CV/ön yazı üretimi,
  öncelik skorlama, dashboard/CRM

## Mimari — 4 bileşen

### 1. Scraper (her şirket için ayrı modül)
- Her şirketin staj/kariyer sayfasını periyodik çeker (statik ise
  `requests`+`BeautifulSoup`, JS-render gerekiyorsa `Playwright`)
- **Kolay tier için ortak bir "kariyer.net scraper" yeterli**: aynı
  HTML yapısını (firma-profil sayfası, login gerektirmiyor) kullanan
  Meteksan/ASELSAN/STM/Baykar/Samsung tek bir generic modülle
  taranabilir — sadece firma-profil URL'si (birden fazla olabilir)
  parametre olarak değişir
- Zor/belirsiz tier (Havelsan, İntertech, Turkcell) için ayrı,
  şirkete özel modüller gerekecek
- Çıktı: ham ilan listesi (başlık, açıklama/HTML, URL, çekilme zamanı)
- Şirkete özel scraper'lar ortak bir arayüz üzerinden çalışmalı:
  `fetch_listings() -> list[RawListing]`
  (böylece yeni şirket eklemek yeni bir modül yazmaktan ibaret kalır)

### 2. Filtre script'i (ayrı, tek dosya, doğrudan API çağrısı)
- Girdi: `RawListing`
- Anthropic API'ye (Claude) ilan metnini verip şu alanları JSON olarak
  çıkarttırır:
  - `is_relevant` (network/backend/sysadmin ile alakalı mı)
  - `class_year_requirement` (varsa: 2/3/4, yoksa null)
  - `gpa_requirement` (varsa sayı, yoksa null)
  - `eligible_for_user` (2. sınıf şartına göre true/false)
  - `summary` (1-2 cümle özet)
- Not: AI Council altyapısı kullanılmıyor — bağımsız, basit bir script.

### 3. Depolama (dedup)
- SQLite, tek tablo: `listings`
  - `id` (URL hash), `company`, `title`, `url`, `first_seen`,
    `is_relevant`, `eligible_for_user`, `summary`, `notified` (bool)
- Her çalıştırmada: yeni ilan mı diye `id` kontrolü → varsa atla,
  yoksa filtre script'ine gönder → sonucu kaydet → `eligible_for_user=true`
  ve `notified=false` ise bildirim kuyruğuna al

### 4. Bildirim
- Telegram bot (kurulumu basit, push bildirim)
- Sadece `eligible_for_user=true` olan ilanlar gönderilir
- Gönderim sonrası `notified=true` işaretlenir (tekrar gönderilmez)

## Çalışma şekli — 2 faz

### Faz 1 (şimdi): Elle tetiklenen PWA
- Telefonda çalışan bir PWA; kullanıcı uygulamayı açıp taramayı elle başlatır
- Backend mantığı (scraper → filtre → storage) PWA'nın tetiklediği bir
  işlem olarak çalışır, sürekli açık sunucu gerekmez
- Bildirimler bu fazda **push değil, yerel hatırlatıcı**:
  1. "Bugünkü taramayı çalıştır" hatırlatması (zamanlanmış, native bildirim)
  2. Tarama bitince "İşlem tamamlandı, sonuçlara bak" bildirimi
  - Not: Faz 1'de "yeni ilan geldi" bildirimi YOK — sonuçlar sadece
    uygulama açıldığında ekranda görünür

### Faz 2 (sonra): Ev sunucusuna taşıma
- Aynı backend mantığı ev sunucusuna deploy edilir
- Her şirket scraper'ı için ayrı bir zamanlanmış görev (cron), günde 1-2 kez
- Basit bir orchestrator script (`run_all.py`) sırayla:
  scraper → filtre → storage → bildirim adımlarını yürütür
- Bu fazda gerçek push bildirimi eklenir: yeni + uygun bir ilan
  bulunduğunda anında bildirim (PWA'nın service worker'ı üzerinden)

## Veri modeli (özet)
```
CompanySource: {company, profile_urls: [url, ...], tier: kolay/zor/belirsiz}
RawListing: {company, title, url, raw_text, fetched_at}
ProcessedListing: RawListing + {is_relevant, class_year_requirement,
                                 gpa_requirement, eligible_for_user, summary}
```

## Hata durumları
- Scraper bir şirket sayfasına ulaşamazsa: log'la, diğer şirketlerle devam
  et, kullanıcıya günlük özet bildirimde "X şirketi çekilemedi" notu düş
- Filtre API çağrısı başarısız olursa: 1 kez retry, olmazsa ilanı
  `unprocessed` olarak işaretle, bir sonraki çalıştırmada tekrar dene

## Açık Sorular
1. Turkcell'in gerçek ATS alt sistemi (`kariyer.turkcell.com.tr`) statik mi,
   JS-render mi? (henüz test edilmedi)
2. Havelsan ve İntertech (zor tier) için: bot koruması/login duvarını aşmaya
   mı çalışılacak, yoksa bu ikisi için elle takip mi tercih edilecek?
3. Faz 2'de gerçek push bildirimi için Telegram bot mu, yoksa PWA'nın
   kendi native push'u mu tercih edilir?
4. Faz 1 PWA hangi teknoloji ile yazılacak? (ör. basit bir React/vanilla JS
   PWA + backend'i local'de veya küçük bir API olarak çağırma)

## Genişletme noktaları (v2+, şimdilik yapılmayacak)
- Yeni şirket eklemek: sadece yeni bir scraper modülü + ortak arayüz
- Öncelik skoru, başvuru takvimi, CV notu üretimi, dashboard
