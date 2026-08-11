# Uygulama durumu

## Aktif faz

Faz 18'in uygulama ve kalite çıkış kriterleri tamamlandı. İlk batch production'dadır. İkinci batch Binalyze'ın
resmî Ashby public API panosunu ve Insider One'ın resmî Lever panosunu otomatik
izlemeye alır. Fixture tabanlı adapter, iki taramalı dedup ve yanlış öğrenci
bildirimi korumaları `docs/acceptance/phase-18-batch-2-2026-08-11.md`
dosyasındadır. Kodlama öncesindeki katalog 31 şirket/33 kaynak/12 otomatik
kaynak/11 otomatik şirketken ikinci batch ile 33 şirket/35 kaynak/14 otomatik
kaynak/13 otomatik şirkete çıktı.

Üçüncü batch Etiya ve Udemy'yi otomatik; OBSS, T2 Software ve TaleWorlds'ü
manuel; LOTEC'i araştırılıyor olarak ekler. Etiya tablo satırı ve Udemy
Greenhouse URL kanonikleştirmesi fixture-first doğrulandı; iki otomatik kaynağın
çift taraması dört görünür ilan, sıfır duplicate ve yalnız bir güçlü staj push'ı
üretti. Katalog bu batch ile 39 şirket/41 kaynak/16 otomatik kaynak/15 otomatik
şirkettir. Kanıt `docs/acceptance/phase-18-batch-3-2026-08-11.md` dosyasındadır.

Son katalog batch'i Ankara Bilgi Teknolojileri ve Peaksoft Consulting'i
otomatik form/e-posta göndermeyen manuel resmî kaynaklarla ekler. Alictus alan
adı SciPlay Games Turkey'ye yönlendiğinden eski kimlik production'a alınmaz.
Faz 17'deki 15 adayın tamamı 14 dürüst katalog kaydı ve 1 bilinçli kimlik
dışlamasıyla hesaplanmıştır. Final katalog 41 şirket, 43 kaynak, 16 otomatik
kaynak ve 15 otomatik şirkettir; kanıt
`docs/acceptance/phase-18-batch-4-2026-08-11.md` dosyasındadır.

On beş Faz 17
şirketinin ayrıntılı durum tespiti `03dc03f`; MobileAction, SİMSOFT, Netaş ve
Bilişim AŞ kaynak uygulaması `b409fa1`; uzun hata mesajı taşma düzeltmesi ve tam
kaliteye giren birleşik revision `eec2f63949f9f0de77830a0ebd62577631bce1c7`
revision'ıdır. Tam backend/frontend/güvenlik/deployment sözleşmesi, gerçek image
build'i ve izole Compose health/smoke kanıtı
`docs/acceptance/phase-18-2026-08-11.md` dosyasındadır.

Faz 18 ilk batch'i kullanıcıdan alınan ayrı production onayı sonrasında
`eec2f63949f9f0de77830a0ebd62577631bce1c7` exact revision'ıyla deploy edildi.
Pre-deploy snapshot iki release'in migration setiyle restore kontrolünden geçti;
API/web healthy, Cloudflare HTTP/2 tunnel running ve dış Access yanıtı beklenen
HTTP 302'dir. Aynı kalıcı production volume'unda ilan/fırsat sayısı 31 olarak
korundu; katalog 31 şirket/33 kaynak/12 otomatik kaynağa uzlaştırıldı ve 11
benzersiz şirket otomatik izleniyor. Ayrıntılı rollout kanıtı
`docs/acceptance/phase-18-2026-08-11.md` dosyasındadır.

İnnova, İntertech, Sebit, DenizBank, Otsimo, Mobiliz, AI Studio, Belsis, Viseur
AI, Actioner ve Bilishim iş kurallarında `secondary` kalırken API/PWA'da ayrı
Faz 16.5 kaynak araştırması ve manuel takip bölümündedir. İnnova resmî kartlardan
otomatik izlenir; LinkedIn yalnız allowlist'teki başvuru hedefidir ve fetch
edilmez. Diğer on kaynak resmî bağlantı, yapılandırılmış engel kodu, ayrıntılı
neden ve doğrulama tarihi taşır. Kaynak matrisi
`docs/research/phase-16.5-sources-2026-08-11.md`, fixture kabulü
`docs/acceptance/phase-16.5-2026-08-11.md` dosyasındadır.

Kullanıcı 11 Ağustos 2026'da kapsamı ve kimlik kararlarını onayladı:
`ÜşüSebit` kanonik `Sebit`, `AI Studio` `aistudio.com.tr`, `Bilishim` ise
`bilishim.ai` olarak izlenecek. Resmî kanıt ve erişim sınıfları
`docs/research/phase-16-sources-2026-08-11.md` dosyasındadır.

On dört Faz 16 şirketi production kataloğuna eklendi. Açık ve robots-uyumlu
Evreka, MechSoft, Layermark ve Faz 16.5 kapsamındaki İnnova kariyer indeksleri fixture-first deterministik
`career_links` adapter'ıyla otomatik izlenir. Oturum/toplayıcı yolu kullanan veya
açık ilan akışı bulunmayan diğer on kaynak manuel ya da araştırılıyor olarak
gerekçesiyle görünür tutulur. Coverage API/PWA artık genel toplamın yanında
birincil, ikincil ve Faz 16.5 gruplarının şirket, kaynak, durum ve otomatik oranlarını ayrı
gösterir; manuel kaynak her iki oranın da paydasından çıkarılır. İkincil fırsat
yalnız boş olmayan odak eşleşmesi, sabit `0.7` güven ve yüksek kaynak güvenini
birlikte sağladığında ayrı sürümlü event ile push üretir. Uçtan uca Faz 16
kabul testi de production kataloğunu, üç production-shape fixture'ını, iki
tarama dedup'ını, zayıf aday görünürlüğünü, tek güçlü push'ı ve ikincil coverage
kırılımını doğruladı. Faz 16 kabul ve tam kalite kanıtı
`docs/acceptance/phase-16-2026-08-11.md` dosyasındadır.

11 Ağustos 2026'da kullanıcı, spec'teki `Commensis` yazımının resmî `Commencis`
kimliğiyle birleştirilmesini ve Turkcell için sentetik ilan yerine ayrı minimal
`program_windows` modelini onayladı.

On iki kanonik birincil şirket production kaynak kataloğunda tanımlandı.
Commencis birincil gruba alındı; Türk Telekom, Jotform, Akınsoft ve Roketsan
doğrulanmış resmî URL ve açık manuel/araştırılıyor gerekçeleriyle eklendi.
Kaynak kapsama ve güven sınıfları SQLite'a kalıcı yazılır.

Minimal `GET /api/v1/coverage` endpoint'i birincil, ikincil ve Faz 16.5 takip
bölümünü; kaynak sağlık/kapsama ayrıntılarını, neden kodu/doğrulama zamanını,
dönemsel programları ve manuel kaynakları dışlayan genel, öncelik ve bölüm
kırılımlı otomatik kapsama oranlarını sunar.

PWA kapsama paneli birincil, ikincil ve Faz 16.5 şirketlerini ayrı grup başlıkları
ve otomatik oranlarla sunar. Panel beş kaynak durumunu, açık gerekçeleri ve Turkcell
dahil dönemsel programların açık/kapalı/bilinmiyor durumunu mobilde tek kolona
inen görünümde korur.

Kaynak güveni bildirim kapısına bağlandı: resmî şirket/ATS veya doğrulanmış
bülten güçlü birincil eşleşmede; Faz 16'dan itibaren odak alanı ve sabit güven
koşulunu da sağlayan güçlü ikincil eşleşmede push üretebilir. Toplayıcı adaylar
görünür kalır ama push üretmez. Faz 15 fixture kabulü 12 şirket, 14 kaynak, Turkcell program
penceresi, iki listing/tek push ve ikinci tarama dedup'ını doğrular.

## Tamamlananlar

- Ürün ve teknik kararları içeren v2 spec
- Go API ve React/Vite PWA iskeleti
- Aday profili ve şirket kaynak yapılandırması için doğrulanan yükleyiciler
- Transaction kullanan, tekrar uygulanmayan SQLite migration çalıştırıcısı
- Kaynak listing'lerini koruyan kanonik fırsat üyelikleri ve startup uzlaştırması
- `0.92` otomatik / `0.80` belirsiz fuzzy eşiği, lokasyon guard'ı ve split audit'i
- Fırsat başına tek dashboard kartı ve fırsat düzeyli bildirim `dedup_key`'i
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
- Config'te en uzun domain suffix'iyle çözülen `robots`, `public_api` ve
  `manual_only` erişim modları ile migration 010 kalıcı policy alanları
- RFC 9309 product-token/wildcard grup seçimi, en uzun yol/eşitlikte allow,
  `*`/`$`, 24 saat cache ve 512 KiB sınırı uygulayan fail-closed robots checker
- Robots izni öncesi hiç fetch yapmayan orchestrator kapısı; otomatik kaynaklarda
  kalıcı minimum aralık ve cooldown, manuel sosyal kaynaklarda açıklamalı watchlist
- Dashboard kovalarından bağımsız, filtreli/sayfalı tüm fırsat geçmişi ve
  başvuru takibinden ayrı yedi durumlu kalıcı fırsat yaşam döngüsü
- JSON olmayan proxy/scan yanıtını parse exception veya gövde sızıntısı olmadan
  güvenli mesaja çeviren PWA yanıt sözleşmesi
- Aynı SQLite dosyasında restart ve snapshot restore boyunca fırsat üyeliği,
  analiz, lifecycle, başvuru tarihleri ve notları koruyan Faz 14.1 kabulü
- Production DB yolunu ve yalnız güvenli tablo sayılarını read-only raporlayan
  `dbinspect` aracı ile volume/DB kimliği operasyon akışı
- On dört yeni ikincil şirket için kanonik kimlik ve erişim sınıfları; Evreka,
  MechSoft, Layermark ve İnnova için deterministik `career_links` adapter'ı
- Coverage API/PWA'da genel toplamdan ayrı birincil, ikincil ve Faz 16.5 özetleri
- Faz 16.5 kaynaklarında yapılandırılmış engel kodu ve doğrulama zamanı
- Güçlü ikincil fırsatlarda odak alanı, sabit `0.7` güven ve yüksek kaynak
  güvenini birlikte isteyen versionlanmış bildirim kapısı

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
- Faz 12 opt-in canlı kabulü gerçek `gemini-3.1-flash-lite` ile yerel iki fixture
  layout'unda geçti (2026-08-09: first_new=2, second_new=0, repair_new=1,
  recipe_version=2, recipe_calls=2); güvenli kanıt
  `docs/acceptance/phase-12-2026-08-09.md` içindedir.
- PWA push/navigation helper testleri, TypeScript typecheck ve production build
  geçer; service worker yalnız aynı-origin hedefi açar.
- Gerçek geçici SQLite ve fake checker testleri `/ready` DB/migration
  doğrulamasını, `/health` bağımsız liveness'ini, hata ayrıntısı sızdırmayan
  `503` davranışını ve HTTP güvenlik/loglama middleware'ini kapsar.
- Faz 13 fixture/fake kabulü iki kaynak listing'ini tek fırsat, tek dashboard
  kartı ve tek push olarak doğruladı; güvenli kanıt
  `docs/acceptance/phase-13-2026-08-09.md` içindedir.
- Faz 14 fixture/fake kabulü gerçek Havelsan LinkedIn config'inin sıfır HTTP ile
  manuel watchlist'e girdiğini; izinli/yasaklı iki robots yolunu ve restart'ı
  aşan domain aralığını gerçek SQLite/orchestrator ile doğruladı. Güvenli kanıt
  `docs/acceptance/phase-14-2026-08-10.md` içindedir.
- Faz 14.1 kabulü dashboard dışındaki fırsatı geçmişte buldu; aynı SQLite'ın
  yeniden açılması ve snapshot restore sonrasında kimlik/üyelik/analiz/lifecycle/
  başvuru verilerini korudu; scan JSON/JSON-olmayan istemci yollarını güvenli
  sözleşmeyle doğruladı. Kanıt `docs/acceptance/phase-14.1-2026-08-10.md` içindedir.
- Faz 16 kabulü production ikincil kataloğunu, üç fixture-first otomatik kaynağı,
  iki taramalı dedup'ı, zayıf aday görünürlüğünü, tek güçlü ikincil push'ı ve
  %40 ikincil otomatik kapsama oranını doğruladı. Kanıt
  `docs/acceptance/phase-16-2026-08-11.md` içindedir.
- Faz 16.5 fixture kabulü on bir şirketin ayrı bölümünü, İnnova resmî indeksinin
  dış başvuru hedefine istek göndermeden taranmasını ve manuel/araştırma
  kayıtlarının neden/tarih metadata'sını doğruladı. Kanıt
  `docs/acceptance/phase-16.5-2026-08-11.md` içindedir.

## Sıradaki iş

Faz 18'in dört batch'i, tam kalite paketi ve upstream push'ları tamamlandı.
Sıradaki zorunlu adım ayrı kullanıcı onayıyla verified `ec9f4ac` revision'ının
production'a deploy edilmesidir. Production şu anda ilk batch `eec2f63`
binary'siyle healthy çalışır; bind-mount edilen güncel config `ashby_board`
adapter'ını içerdiğinden deploy öncesi container restart'ı eski binary/yeni
config uyumsuzluğu yaratabilir. Snapshot, restorecheck, exact revision rollout,
health/smoke ve kalıcı DB kimliği deployment sırasında tekrar doğrulanmalıdır.

Bu rollout tamamlandıktan sonraki ürün fazı Faz 19 genel fırsat modeli ve
bildirim katmanlarıdır. Taksonomi, program penceresi şeması ve fixture eval altın
kümesi koddan önce ayrı plan/onay kapısından geçer. RSS/e-posta Faz 20–21'de,
analitik/öğrenme Faz 23'te kalır; ayrıntılar `docs/roadmap.md` içindedir.

Faz 5'in production runbook'u geçerliliğini korur. Kullanıcı yerel Docker +
Cloudflare Tunnel üzerinden telefon erişimini doğrulamıştır; off-host yedek ve
gerçek hosting'e özgü operasyon kanıtları seçilen ortamda ayrıca tutulmalıdır.
