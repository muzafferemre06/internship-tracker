# Uygulama durumu

## Aktif faz

Faz 12 tamamlandı. Öğrenilmiş selector reçeteleri kaynak bazında versionlanarak
SQLite'ta tutuluyor; olağan taramalar reçeteyi AI'sız çalıştırıyor, kimlik/şema
guard'ı veya tarihsel ilan sayısının sıfıra düşmesi reçeteyi yeniden üretip
sürümünü artırıyor. Sıradaki geliştirme fazı kaynaklar arası dedup ve kanonik
fırsat modeli olan Faz 13'tür.

## Tamamlananlar

- Ürün ve teknik kararları içeren v2 spec
- Go API ve React/Vite PWA iskeleti
- Aday profili ve şirket kaynak yapılandırması için doğrulanan yükleyiciler
- Transaction kullanan, tekrar uygulanmayan SQLite migration çalıştırıcısı
- Meteksan profilinden ilan bağlantılarını normalize eden kariyer.net adapter'ı
- ASELSAN/ASELSANNET çoklu profili ile STM, Baykar ve Samsung kaynak ayarları
- Canonical URL, kararlı kimlik ve duplicate kontrolü yapan SQLite repository
- Profil alanlarıyla temel uygunluk üreten deterministik ilan analizcisi
- Gerçek bağımlılıklarla çalışan manuel scan API'si ve SQLite dashboard sorgusu
- Kalıcı `completed`/`partial`/`failed` scan-run raporu ve kaynak sağlık durumu
- Kalıcı domain erişim bütçesi, 24 saat alt sınır ve 403/429 challenge devre kesicisi
- PWA'da son taramanın başarılı ve başarısız kaynak sayıları
- Kilit dosyasıyla tekrarlanabilir frontend kurulumu ve sıfır npm audit bulgusu
- Test, CI, Docker ve secret yönetimi başlangıç dosyaları
- Ayarlardan seçilen deterministik/OpenRouter/Google analiz sağlayıcısı ve model
- Minimize aday profili, strict JSON Schema ve backend çıktı doğrulaması
- Sınırlı model retry'sı ile pending analizlerin kaynak bağımsız yeniden işlenmesi
- Provider/model, token kullanımı ve ayarlanabilir oranlarla tahmini maliyet kalıcılığı
- İlan detayını ve başvuru takibini sunan HTTP/repository sözleşmesi
- Durum, manuel tarih, mülakat zamanı ve notlar için doğrulanan SQLite upsert akışı
- Kaynak hatalarından üretilen manuel kontrol listesi
- Telefonda tek kolona inen dashboard ve tam genişlik ilan detay paneli
- İlan uygunluk özeti, eşleşen alanlar ve resmî ilan bağlantısı
- Dashboard'da aktif başvuru, yaklaşan tarih ve manuel kontrol bölümleri
- Production runtime temeli: Go 1.26, Alpine 3.24, Node 24 LTS ve nginx 1.30
  container image'ları
- CI'da `go vet`, yüksek önem seviyesinde npm bağımlılık denetimi ve yayınlamayan
  backend/frontend image build kapıları
- `SCAN_SCHEDULE` ve `SCAN_TIMEZONE` ile startup'ta doğrulanan, graceful shutdown
  context'ine bağlı process-içi zamanlanmış tarama
- Manuel ve zamanlanmış taramaların ortak kilidi; çakışan manuel tarama için
  HTTP `409` davranışı
- Production'da açıkça etkinleşen günlük SQLite `VACUUM INTO` snapshot'ı,
  bütünlük doğrulaması, private izinler ve sınırlı retention
- Analizle aynı SQLite transaction'ında oluşan versionlanmış bildirim outbox'ı,
  cihaz bazlı delivery ve tekrar taramada bildirim dedup'ı
- Strict HTTPS abonelik API'si, VAPID/RFC 8291 Web Push göndericisi, sınırlı
  retry ve 404/410 cihaz temizliği
- Kullanıcı hareketiyle açılıp kapanan PWA bildirim aboneliği, güvenli service
  worker payload'ı ve doğru ilan detayını açan `?listing=<id>` deep-link'i
- SQLite ping'i ve paketlenmiş migration kayıtlarını doğrulayan ayrı `/ready`
  endpoint'i; bağımsız `/health` liveness yanıtı
- HTTP durum kodunu içeren yapılandırılmış request logları, API/PWA temel
  güvenlik başlıkları ve local HTTP'yi bozmayan HSTS sınırı
- API readiness'e bağlı Compose healthcheck'leri ve healthcheck aracı kurulu
  backend/frontend runtime image'ları
- Production secret dosyaları, fail-fast config ve mutation isteklerinde exact
  Origin/CSRF koruması
- Tek seferlik pre-deploy snapshot ve production verisine yazmayan restore ön
  kontrol binary'leri
- Digest sabitli, host portu açmayan Cloudflare Tunnel Compose paketi ile
  preflight, smoke ve image rollback scriptleri
- `amd64`/`arm64` GHCR image yayını, SBOM/provenance/attestation ve korumalı
  production environment deployment workflow'u
- Adapter adını fabrikaya eşleyen veri-odaklı `adapterFactories` dispatch tablosu
  ve kaynak başına `strategy` alanı (Faz 9)
- Faz 10 yapılandırılmış-veri adapter'ları: schema.org `JobPosting` JSON-LD
  (`json_ld`) ve herkese açık Greenhouse board API (`greenhouse`/`ats_api`);
  ikisi de AI'sız `RawListing`'e normalize edip mevcut dedup/analiz yoluna girer
- Faz 11 kaotik/bespoke kaynaklar için reduce-then-LLM `llm_generic` adapter'ı:
  deterministik reduce (anahtar kelime + yapısal pencereleme), content-hash
  kapısı + blok diff (değişmeyen taramada sıfır model çağrısı), enjekte
  `ListingExtractor` portu ve strict doğrulama; Gemini extractor ayrı pakette
- Faz 9 dispatch'inin `SourceDeps` ile genişletilmesi (shared extractor enjeksiyonu)
- Faz 12 `learned_selector` adapter'ı, strict ve sınırlı selector dili,
  kaynak kimliği/şema/golden-count onarım guard'ları ve versionlanmış SQLite
  reçete geçmişi
- Kaynak sağlık snapshot'ında strateji sürümü, son ilan sayısı ve ilan parmak izi
- Faz 11 blok-hash cache'inin restart'ı aşan SQLite kalıcılığı
- Analiz, generic extraction ve reçete öğreniminin aynı yapılandırılmış model
  provider örneğini paylaşan production wiring'i

## Doğrulanan çıkış kriterleri

- Backend testleri, `go vet` ve production build geçer.
- Frontend testleri, typecheck, production build ve `npm audit` geçer.
- API örnek config ve geçici SQLite ile açılır; health/dashboard yanıt verir.
- İlk fixture taraması iki yeni ilan, ikinci tarama sıfır yeni ilan üretir.
- Tanınmayan bir kaynak sayfası hata verirken diğer iki profil tamamlanır.
- İki ASELSAN profilindeki ortak ilan tek kayıt olur; rapor `partial` kapanır.
- 403, 418, 429, 5xx, timeout ve selector bozulması testleri geçer.
- İlk koruma hatasından sonra aynı domaindeki diğer profillere istek gönderilmez.
- Fake provider/HTTP cevaplarıyla uygun, uygun değil, karar bekliyor, bozuk JSON,
  şema hatası, timeout, kalıcı 4xx, 429, 5xx ve retry sınırı geçer.
- Başarısız analiz ham ilanı kaybetmez; dashboard karar kuyruğunda kalır ve
  kaynak fetch'i olmadan başarılı biçimde yeniden işlenir.
- Başarılı analizde provider, model, token kullanımı ve tahmini maliyet saklanır.
- Google Gemini adapter'ının header/schema/usage/hata davranışı fake transport ile,
  tam şema akışı ise opt-in canlı integration testiyle doğrulanabilir.
- Resmî tek ilan sayfasını güvenli alanlarla normalize eden Lever adapter'ı;
  aktif başvuru bağlantısı ve beklenen sayfa yapısı için fixture testleriyle doğrulanır.
- Fixture Lever kaynağı, strict fake model, geçici SQLite ve dashboard HTTP
  handler'ını iki taramada birleştiren Faz 3.5 normal kabul testi tamamlandı.
- Resmî Commencis Lever ilanı canlı Google `gemini-3.1-flash-lite` ile işlendi;
  zaman damgalı güvenli kanıt `docs/acceptance/phase-3.5-2026-08-03.md` içinde.
- Faz 4 fixture kabulü gerçek SQLite ve HTTP handler üzerinden ilan inceleme,
  başvuru güncelleme, tarih sıralaması ve manuel kontrol görünürlüğünü doğrular.
- Backup testi geçici SQLite snapshot'ının kaynak sonradan değişse bile
  restore edilebildiğini; integrity, `0700`/`0600` izinleri ve retention
  temizliğini doğrular.
- RFC 8291 resmî ciphertext vektörü, doğrulanabilir RFC 8292 ES256 JWT, fake
  multi-device sender ve gerçek SQLite delivery testleri ağsız geçer.
- Faz 5 fixture kabulü primary/açık/ilgili/uygun ilanı iki taramada tek outbox
  olayı ve tek deep-linked fake push olarak teslim eder.
- Faz 9 dispatch tablosu kayıtlı adapter'ları fabrikadan üretir, bilinmeyen
  adapter'ı reddeder; mevcut kariyer.net/Lever regresyonsuz çalışır.
- Faz 10 JSON-LD ve Greenhouse adapter'ları fixture'lardan strict şemaya
  (tag'siz metin, kanonik URL) normalize eder; JSON-LD bloğu/`JobPosting`
  eksikse veya başlık yoksa sessiz sıfır yerine hata verir; boş Greenhouse
  board'u geçerli sayılır.
- Faz 10 kabul testi JSON-LD sayfasını tam orchestrator → dedup → analiz →
  dashboard yolundan geçirir; ikinci taramada sıfır yeni kayıt ve tekrar analiz
  olmadığını doğrular.
- Faz 11 `llm_generic` adapter'ı bespoke fixture'ından iş bloklarını çıkarır,
  gezinme/footer/promo gürültüsünü modele göndermez; değişmeyen ikinci taramada
  hiç extractor çağrısı yapmaz; malformed model çıktısı kaynak hatası olur; aday
  blok yoksa sessiz sıfır yerine hata verir.
- Faz 11 kabul testi bespoke sayfayı fake extractor + fake model ile tam
  orchestrator → dedup → analiz → dashboard yolundan geçirir (2 ilan, ikinci
  taramada 0 yeni, tek extractor çağrısı).
- Faz 11 opt-in canlı integration testi gerçek `gemini-3.1-flash-lite` ile yerel
  sunulan bespoke sayfadan uçtan uca çıkarma + analiz + dedup'ı doğrular
  (2026-08-08: found=2, first_new=2, second_new=0, 475 token).
- Faz 12 fixture kabulü ilk taramada reçete v1 üretildiğini, yeni source
  instance'ında ek model çağrısı olmadan iki ilanın bulunduğunu ve değişen DOM'da
  sessiz sıfır yerine reçete v2 onarımı + bir yeni ilan oluştuğunu gerçek SQLite
  ve orchestrator üzerinden doğrular.
- PWA push/navigation helper testleri, TypeScript typecheck ve production build
  geçer; service worker yalnız aynı-origin hedefi açar.
- Gerçek geçici SQLite ve fake checker testleri `/ready` DB/migration
  doğrulamasını, `/health` bağımsız liveness'ini, hata ayrıntısı sızdırmayan
  `503` davranışını ve HTTP güvenlik/loglama middleware'ini kapsar.

## Sıradaki iş

Faz 13: farklı kaynaklardan gelen aynı ilanı şirket + normalize başlık + konum
üzerinden tek kanonik fırsata bağlamak; dashboard ve bildirim dedup'ını fırsat
düzeyine taşımak. Başlamadan önce bulanık eşleme eşikleri ve yanlış birleşmeleri
geri alma davranışı için ayrı test-first plan onayı alınmalıdır.

Faz 5'in production runbook'u geçerliliğini korur. Kullanıcı yerel Docker +
Cloudflare Tunnel üzerinden telefon erişimini doğrulamıştır; off-host yedek ve
gerçek hosting'e özgü operasyon kanıtları seçilen ortamda ayrıca tutulmalıdır.
