# Internship Tracker

Kişisel staj ilanı takip ve başvuru hatırlatma uygulaması. Backend Go,
istemci ise React/Vite tabanlı bir PWA olarak yapılandırılmıştır.

Ürün kararları için [v2 spec](./staj-takip-spec-v2.md), ilk fikir belgesi için
[ilk spec](./staj-takip-spec-initial.md) kullanılmalıdır.

## Gereksinimler

- Go 1.26+
- Node.js 24.18+ LTS
- npm 10+
- Docker ve Docker Compose (isteğe bağlı)

## Yerel geliştirme

```bash
cp .env.example .env
cp configs/candidate-profile.example.json configs/candidate-profile.json
cp configs/sources.example.json configs/sources.json
go run ./cmd/api
```

Başka bir terminalde:

```bash
npm --prefix web install
npm --prefix web run dev
```

API varsayılan olarak `http://localhost:8080`, PWA ise
`http://localhost:5173` adresinde çalışır.

API başlangıçta `DATABASE_PATH` altındaki SQLite dosyasını açar ve
`MIGRATIONS_PATH` içindeki uygulanmamış `.sql` dosyalarını alfabetik sırayla,
transaction içinde uygular. Uygulanan dosyalar `schema_migrations` tablosunda
izlenir. Kanonik fırsat migration'ı eski listing'leri veri kaybetmeden başlangıç
fırsatlarına bağlar; API açılmadan önce analizli kayıtlar güncel deterministik
eşleme kurallarıyla idempotent olarak uzlaştırılır.

Uygulama aday profili ve kaynak dosyalarını katı bir JSON şemasıyla okur;
bilinmeyen alanlar ve geçersiz şirket/kaynak değerleri başlangıç hatasıdır.
Dosya yolları `CANDIDATE_PROFILE_PATH` ve `SOURCES_PATH` ile değiştirilebilir.
Her kaynak, taramalar ve veritabanı kayıtları arasında değişmeyen benzersiz bir
`id` alanına sahip olmalıdır. Aynı şirket birden fazla `sources[]` girdisiyle
izlenebilir. İştirak profilinin sayfa başlığı ana şirket adından farklıysa
selector doğrulaması için kaynakta `page_name` belirtilir.

`lever` adapter'ı herkese açık resmî `https://jobs.lever.co/<şirket>/<ilan>`
sayfasındaki tek ilanı izler. Yalnızca aktif başvuru bağlantısı bulunan sayfaları
kabul eder; takip parametrelerini kaynak URL'sinden çıkarır ve başlık, ilan
kategorileri ile açıklama alanlarını normalize eder. Örnek kaynak dosyasında
Commencis'in resmî Lever ilanı bu adapter'ın yapılandırmasını gösterir.
Lever istekleri alanın robots politikasındaki bir saniyelik minimum aralıkla
kalıcı erişim bütçesinden geçirilir.

## İlan analizi

Varsayılan `LLM_PROVIDER=deterministic` ayarı API anahtarı veya ağ çağrısı
gerektirmez. OpenRouter analizi için yerel `.env` dosyasında aşağıdaki alanlar
ayarlanır:

```dotenv
LLM_PROVIDER=openrouter
LLM_MODEL=provider/model-name
OPENROUTER_API_KEY=local-secret
LLM_INPUT_COST_PER_MILLION_USD=0
LLM_OUTPUT_COST_PER_MILLION_USD=0
```

Google Gemini API'yi doğrudan kullanmak için:

```dotenv
LLM_PROVIDER=google
LLM_MODEL=gemini-3.1-flash-lite
GEMINI_API_KEY=local-secret
LLM_THINKING_LEVEL=minimal
LLM_INPUT_COST_PER_MILLION_USD=0
LLM_OUTPUT_COST_PER_MILLION_USD=0
```

Model ve sağlayıcı yalnızca backend başlangıcında seçilir. OpenRouter veya Google
seçiliyken model adı ve ilgili API anahtarı zorunludur; maliyet oranları negatif
olmayan USD değerleri olmalıdır. `LLM_PROVIDER=gemini`, `google` için alias'tır.
Google adapter'ı Gemini 3 modellerinde `minimal`, `low`, `medium` veya `high`
düşünme seviyesini kullanır; Gemma modellerine Gemini'ye özgü bu alanı göndermez.
Google istekleri, düşünmeli modellerin gecikmesini karşılamak için 60 saniyelik
istemci timeout'u kullanır.
Gerçek model fiyatları değişebildiği için milyon input ve output token başına
oranlar seçilen modelin güncel fiyatıyla kullanıcı tarafından ayarlanır. Anahtar
ve yerel `.env` repoya eklenmez.

## Öğrenilmiş kaynak reçeteleri

Kararlı API/JSON-LD sunmayan fakat DOM yapısı deterministik çalıştırılabilecek
bir kariyer sayfası `adapter: "learned_selector"` ile tanımlanabilir. Strateji
otomatik olarak `learned_selector` olur. Bu adapter ilk taramada yapılandırılmış
LLM sağlayıcısıyla selector reçetesi üretir; reçete ve golden ilan snapshot'ı
SQLite'ta versionlanır. Sonraki taramalar, process restart'ından sonra da AI
çağrısı olmadan aynı reçeteyi çalıştırır. Sayfa kimliği/ilan şeması bozulur veya
önceden pozitif ilan sayısı sıfıra düşerse bir onarım çağrısı yapılır.

```json
{
  "id": "ornek-kariyer",
  "type": "career_page",
  "url": "https://example.com/careers",
  "adapter": "learned_selector",
  "enabled": true
}
```

Etkin `learned_selector` ve `llm_generic` kaynakları model provider gerektirir;
`LLM_PROVIDER=deterministic` ile uygulama bu kaynakları sessizce atlamak yerine
başlangıçta açık hata verir. Model provider tek kez oluşturulur ve analiz,
generic ilan extraction'ı ve reçete öğrenimi arasında paylaşılır.

## Production yapılandırması ve secret'lar

`APP_ENV=production` başlangıçta güvenlik kapılarını zorunlu kılar:
`ALLOWED_ORIGIN` path içermeyen, localhost olmayan bir `https://` origin'i
olmalı; SQLite, migration, aday profili ve kaynak yolları ile backup dizini
mutlak yol olmalıdır. SQLite bellek/URI veritabanı kabul edilmez.
`BACKUP_ENABLED=true`, `WEB_PUSH_ENABLED=true` ve Web Push private key zorunlu
olur. Varsayılan `LLM_PROVIDER=deterministic` production'da bilinçli ve geçerli
bir seçimdir; OpenRouter/Gemini yalnız seçildiklerinde ilgili anahtarı ister.

`OPENROUTER_API_KEY`, `GEMINI_API_KEY` ve `WEB_PUSH_PRIVATE_KEY` doğrudan ortam
değeri yerine karşılık gelen `_FILE` değişkeniyle secret dosyasından okunabilir.
Bir secret için yalnızca biri ayarlanır; dosya yolu boş, okunamaz veya dosya
boşsa uygulama dinlemeye başlamaz. Production'da secret dosya yolu da mutlak
olmalıdır. Hata ve yapılandırılmış loglar secret değerini içermez.

Production'daki `POST`, `PUT` ve `DELETE` API istekleri, tarayıcının `Origin`
header'ını `ALLOWED_ORIGIN` ile tam eşleştirmelidir. Eksik veya farklı origin
`403` döner. `GET /health`, `GET /ready` ve okuma uçları etkilenmez; CORS
preflight yalnız yapılandırılmış origin için izin alır.

Production kurulumu için [deployment runbook'u](./docs/deployment.md), secret ve
origin ilkeleri için [güvenlik rehberi](./docs/security.md), yedek/restore için
[operasyon rehberi](./docs/operations.md) kullanılır. Hesap veya sunucu secret'ı
paylaşmadan tamamlanacak kısa hazırlık listesi
[production girdileri belgesindedir](./docs/production-inputs.md).

Modele adayın adı, iletişim bilgileri, üniversite adı veya deneyim kurumları
gönderilmez. Yalnızca bölüm/alan, sınıf, GPA, odak alanları, deneyim konu başlıkları
ve konum tercihleri; ilan başlığı ve metniyle birlikte gönderilir.

## Web Push backend

Web Push geliştirmede varsayılan olarak kapalıdır. Etkinleştirmek için aynı
P-256 VAPID anahtar çiftinin URL-safe base64 biçimi ve geçerli bir iletişim URI'si
yalnızca backend ortamına verilir:

```dotenv
WEB_PUSH_ENABLED=true
WEB_PUSH_PUBLIC_KEY=base64url-public-key
WEB_PUSH_PRIVATE_KEY=base64url-private-key
WEB_PUSH_SUBJECT=mailto:operator@example.com
```

Etkin ayarda public/private anahtarların P-256 biçimi ve birbiriyle eşleşmesi ile
`mailto:`/`https:` subject değeri uygulama dinlemeye başlamadan doğrulanır. Private
key PWA bundle'ına veya API yanıtına girmez; production'da `.env` yerine deployment
secret sistemi kullanılmalıdır.

İlk başarılı analizde yalnızca birincil şirkete ait, başvurusu açık, ilgili ve
`uygun` fırsat için versionlanmış tek bildirim olayı oluşur. Analiz, fırsat
çözümleme ve olay/outbox aynı SQLite transaction'ındadır. Olay mevcut cihaz
aboneliklerine ayrı delivery kayıtlarıyla dağıtılır; aynı fırsatın başka
kaynaktaki listing'i `opportunity:<id>:new-primary-suitable:v1` anahtarı nedeniyle
yeni olay oluşturmaz. Gönderici geçici ağ/408/425/429/5xx hatalarını en fazla beş toplam
denemeyle erteler, `Retry-After` değerini en fazla 24 saatle sınırlar ve 404/410
dönen aboneliği diğer cihazlara dokunmadan kaldırır.

## Test

```bash
go test ./...
go vet ./...
npm --prefix web test
npm --prefix web audit --audit-level=high
```

Gerçek OpenRouter ve Google çağrıları normal test akışına dahil değildir. API
anahtarları yalnızca yerel `.env` veya deployment secret sistemi üzerinden
verilmelidir. Google adapter'ının opt-in canlı testi açıkça şöyle çalıştırılır:

```bash
GEMINI_API_KEY=... go test -tags=integration ./internal/analyzer \
  -run TestGoogleProviderLive -v
```

Test varsayılan olarak `gemini-3.1-flash-lite` kullanır;
`GEMINI_LIVE_TEST_MODEL` ile model değiştirilebilir. Bu komut gerçek API kotası
kullanır ve anahtar yoksa testi atlar.

Faz 3.5 uçtan uca canlı kabul testi ayrıca açıkça etkinleştirilmelidir:

```bash
RUN_REAL_LISTING_ACCEPTANCE=1 GEMINI_API_KEY=... \
  go test -tags=integration ./internal/acceptance \
  -run TestPhase35LiveOfficialListingWithGemini -v
```

Test varsayılan Commencis Lever ilanını geçici SQLite üzerinde iki kez işler;
canlı Gemini kullanım metadatasını, dashboard API görünürlüğünü ve ikinci
çalıştırmada sıfır yeni kayıt sonucunu doğrular. İlan kapandığında yeni bir resmî
Lever sayfası `REAL_LISTING_URL`, `REAL_LISTING_COMPANY` ve
`REAL_LISTING_EXPECTED_TITLE` ile verilebilir. Geçici veritabanı test sonunda
silinir; API anahtarı ve canlı sayfa gövdesi kaydedilmez.
Başarılı 3 Ağustos 2026 çalışmasının kısa ve güvenli çıktısı
[Faz 3.5 kabul kanıtında](./docs/acceptance/phase-3.5-2026-08-03.md) kayıtlıdır.

Faz 12 learned-selector canlı kabulü yerel fixture'larla ayrıca çalıştırılabilir:

```bash
RUN_PHASE12_LIVE_ACCEPTANCE=1 \
GEMINI_API_KEY_FILE=/gizli/yol/gemini_api_key \
go test -tags=integration ./internal/acceptance \
  -run TestPhase12LiveGeminiLearnsPersistsAndRepairsRecipe -count=1 -v
```

Bu test ilk reçete öğrenimini, restart sonrası AI'sız taramayı ve değiştirilmiş
DOM fixture'ında otomatik onarımı gerçek Gemini ile doğrular. 9 Ağustos 2026
başarılı çalışmasının güvenli özeti
[Faz 12 kabul kanıtındadır](./docs/acceptance/phase-12-2026-08-09.md).

## Docker ile çalıştırma

```bash
docker compose -f deploy/compose.yml up --build
```

PWA bu kurulumda `http://localhost:8081` adresinden açılır. Compose dosyası
yerel `configs/candidate-profile.json` ve `configs/sources.json` dosyalarını
salt okunur bağlar; SQLite verisini adlandırılmış bir volume içinde korur.
API, SQLite bağlantısını ve bu image ile gelen tüm migration kayıtlarını doğrulayan
`/ready` yanıtı sağlıklı olmadan web container'ı başlatılmaz. Her iki image, kendi
yerel healthcheck'i için image içinde açıkça kurulu `wget` kullanır.
Production compose ayrıca günlük SQLite snapshot'larını `tracker_backups`
volume'unda tutar; bu volume'u ana veritabanı volume'undan ayrı bir host/offsite
backup hedefine düzenli olarak dışa aktarmak işletim sorumluluğudur.
Container image'ları Go 1.26, Alpine 3.24, Node 24 LTS ve nginx 1.30 kararlı
sürüm hattını kullanır. Pull request CI'ı image'ları yayınlamadan build eder;
`main` push'u ve manuel workflow ise digest sabitli çok mimarili GHCR image'ları
yayımlar. Korumalı production deployment yalnız manuel `deploy=true` seçimiyle
çalışır. Deploy bundle'ı aynı committen yalnız production Compose, nginx ve shell
script allowlist'iyle üretilir; sunucuda checksum/revision doğrulamasından sonra
immutable revision dizinine kurulur. Ayrıntılar deployment runbook'undadır.

## API

- `GET /health`: bağımlılıklardan bağımsız süreç liveness bilgisi
- `GET /ready`: SQLite `Ping` ve bu sürümün migration kayıtlarını doğrulayan
  readiness bilgisi; hazır değilse ayrıntı sızdırmadan `503` döner
- `GET /api/v1/dashboard`: uygun yeni ilanlar, takip özetleri ve kalıcı son
  tarama raporu
- `POST /api/v1/scan`: etkin kaynakları hemen tarar; toplam bulunan/yeni ilan
  sayılarını, kalıcı tarama kimliğini/durumunu ve kaynak bazlı hataları döndürür;
  başka bir tarama çalışıyorsa `409` döner
- `POST /api/v1/analyses/retry`: en fazla 25 `pending` analizi, kaynak siteye
  yeniden bağlanmadan saklanan ham ilan metni üzerinden işler; işlenen ve tekrar
  başarısız olan kayıt sayılarını döndürür
- `GET /api/v1/listings/{id}`: PWA detay paneli için normalize ilan, analiz ve
  mevcut başvuru takibini döndürür
- `PUT /api/v1/listings/{id}/application`: başvuru durumunu, manuel takip/son
  tarihi, sınav-mülakat zamanını ve kullanıcı notunu kaydeder
- `GET /api/v1/push/public-key`: Web Push etkinse PWA aboneliği için yalnızca
  VAPID public key'i döndürür
- `PUT /api/v1/push/subscriptions`: tarayıcının HTTPS endpoint ve `p256dh`/`auth`
  anahtarlarını katı ve boyut sınırlı JSON gövdesiyle idempotent kaydeder
- `DELETE /api/v1/push/subscriptions`: endpoint gövdesiyle ilgili cihaz
  aboneliğini idempotent kaldırır

Manuel tarama tamamlandıktan sonra PWA dashboard'u yeniden yükler. Bir kaynak
hatası diğer kaynakların çalışmasını durdurmaz; kısmi sonuç HTTP `207` ile
döner. Her kaynak için son başarı zamanı veya zaman damgalı kısa hata nedeni
SQLite'ta tutulur.

PWA kartları yeni/uygun, karar bekleyen ve aktif başvuruları ayrı bölümlerde
gösterir. Bir karta dokunulduğunda uygunluk özeti, eşleşen alanlar, konum ve son
başvuru tarihi açılır; aynı panelden başvuru durumu, manuel takip tarihi,
sınav-mülakat zamanı ve notlar düzenlenir. Tarihi olan aktif başvurular yaklaşan
tarihlerde sıralanır. Son taramada hata veren kaynaklar manuel kontrol listesinde
resmî kaynak bağlantısıyla görünür.

Başvuru güncelleme gövdesindeki `deadline` ve `interview_at` alanları RFC3339
zaman damgası veya `null` olmalıdır. Durum; `incelenecek`, `basvuruldu`,
`sinav_mulakat`, `olumlu` ya da `olumsuz` değerlerinden biridir. API ham ilan
metnini detay yanıtına koymaz.

Bildirim kontrolü tarayıcı iznini yalnız kullanıcının düğmeye dokunmasıyla ister.
Abonelik açılıp kapatılabilir; tarayıcı desteği veya reddedilmiş izin ayrı durum
olarak gösterilir. Web Push production'da HTTPS ve desteklenen tarayıcı/PWA
kurulumu gerektirir (localhost geliştirme istisnasıdır). Bildirime dokunmak
`?listing=<id>` deep-link'iyle doğru detay panelini açar; açık bir PWA penceresi
varsa yeni pencere çoğaltmak yerine o pencere yönlendirilip öne getirilir.
Service worker yalnız aynı-origin hedefleri kabul eder ve event `tag` değeriyle
aynı bildirimin görünür tekrarını bastırır.

## HTTP health ve güvenlik başlıkları

API middleware'i her yanıta `X-Content-Type-Options: nosniff`,
`Referrer-Policy: strict-origin-when-cross-origin`, `X-Frame-Options: DENY` ve
same-origin PWA için sınırlı bir Content Security Policy ekler. Yapılandırılmış
HTTP logu method, query string içermeyen path, HTTP durum kodu ve süreyi taşır.

`/health` yalnız process'in dinleyebildiğini ölçer; container/deployment
healthcheck'leri `/ready` kullanır. Nginx yerel Compose'ta bilerek HTTP
dinlediği için HSTS eklemez. HSTS, ancak gerçek HTTPS'i sonlandıran production
reverse proxy katmanında eklenmelidir; HTTP ortamında göndermek ilk ziyaret ve
yerel geliştirmeyi bozabilir.

Kariyer.net kaynakları aynı domain erişim bütçesini paylaşır. Başarılı veya
başlatılmış iki Kariyer.net taraması arasında en az 24 saat bırakılır. İlk
403/429/challenge yanıtı kalan Kariyer.net profillerini çağırmadan durdurur;
cooldown ve en erken tekrar zamanı SQLite'ta kalır ve manuel tarama düğmesiyle
aşılamaz. API/PWA atlanan kaynakları ve tekrar zamanını gösterir.

## Zamanlanmış tarama

API, uygulama açılırken `SCAN_SCHEDULE` içindeki beş alanlı cron ifadesini
(`dakika saat ayın-günü ay haftanın-günü`) ve `SCAN_TIMEZONE` IANA zaman dilimini
doğrular; geçersiz ayar servis başlamadan hata verir. Varsayılan
`0 9 * * 1` ve `Europe/Istanbul`, her pazartesi İstanbul saatine göre 09:00
tarama yapar. Alanlar `*`, sayı, liste, aralık ve adım (`*/15`, `1-5`, `1,3,5`)
biçimlerini kabul eder.

Zamanlanmış ve `POST /api/v1/scan` tetiklemeleri aynı process içi tarama
kilidini paylaşır. Çakışan manuel istek `409` döner; zamanlanmış tetikleme de
çalışan taramayı beklemeden loglanır. `SIGINT`/`SIGTERM`, sonraki tetiklemeyi
durdurur ve aktif zamanlanmış taramanın context'ini iptal ederek kapanma akışına
katılır. Scheduler yalnızca uygulama process'i çalışırken görev çalıştırır;
kesinti sonrası kaçan çalışmayı telafi etmez.

## SQLite yedekleme

Yerel geliştirmede `BACKUP_ENABLED=false` varsayılandır; uygulama yedek dizini
oluşturmaz ve dosya yazmaz. Production'da backup, `BACKUP_ENABLED=true` ile
açıkça etkinleştirilmelidir. Etkinken `BACKUP_DIRECTORY` zorunludur;
`BACKUP_TIME` günlük yerel saati `HH:MM`, `BACKUP_TIMEZONE` IANA zaman dilimini,
`BACKUP_RETENTION` ise saklanacak günlük snapshot sayısını belirler. Varsayılan
zaman `02:00`, zaman dilimi `Europe/Istanbul` ve retention `7` gündür.

Geçersiz etkin backup ayarı API dinlemeye başlamadan süreci durdurur. Snapshot,
çalışan veritabanı dosyasını kopyalamak yerine SQLite'ın `VACUUM INTO` desteğiyle
üretilir; geçici dosyanın `integrity_check` sonucu `ok` değilse yayımlanmaz.
Başarılı snapshot atomik olarak yayımlanır, dosya izni `0600`, dizin izni `0700`
olur ve yalnızca uygulamanın kendi eski snapshot'ları retention sınırını aşınca
silinir. Kapanış sinyali bekleyen günlük timer'ı ve aktif backup context'ini
iptal eder.

`deploy/compose.yml`, yedeklemeyi açık ve `/app/backups` için ayrı kalıcı volume
ile kurar. Bu, host kaybına karşı tek başına yeterli değildir: volume snapshot'ı
veya şifreli dışa aktarım ayrı bir retention politikasıyla tutulmalı; her deploy
öncesinde bir snapshot alınmalı ve en az bir restore provası yapılmalıdır.
Tek seferlik snapshot, salt-okunur restore ön kontrolü ve off-host retention
politikası için [production operasyon rehberine](./docs/operations.md) bakın.

Model cevabı JSON Schema ile istenir ve backend'de bilinmeyen alanları da reddeden
aynı katı sözleşmeyle doğrulanır. Bozuk JSON, şema hatası, timeout, 429 ve geçici
5xx cevapları en fazla üç denemeyle sınırlıdır.
Analiz yine başarısızsa ham ilan silinmez; `karar_bekliyor` ve `pending` olarak
hata/retry bilgisiyle saklanır. Başarılı analiz provider, model,
prompt/completion/toplam token ve yapılandırılan oranlardan hesaplanan tahmini
USD maliyetiyle kalıcılaştırılır.

Zamanlanmış taramayı ayrı bir sunucuda çalıştırmak ev bağlantısını otomasyon
trafiğinden izole eder, ancak erişim izni sağlamaz ve veri merkezi IP'leri de
engellenebilir. Aynı bütçe/devre kesici sunucuda korunmalı; proxy rotasyonu veya
challenge aşma yöntemi olarak kullanılmamalıdır.

## Dizinler

```text
cmd/api/             API uygulamasının giriş noktası
internal/            Backend domain ve uygulama katmanları
migrations/          SQLite migration dosyaları
configs/             Secret içermeyen örnek kaynak ayarları
web/                 React/Vite PWA
deploy/              Container ve deployment dosyaları
docs/                Mimari ve geliştirme notları
```

Güncel faz durumu ve sıradaki uygulama işi için
[`docs/progress.md`](./docs/progress.md) kullanılmalıdır.
