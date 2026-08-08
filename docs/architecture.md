# Mimari notları

## Sınırlar

- `cmd/api`: uygulamayı ayağa kaldırır; iş kuralları burada yazılmaz.
- `internal/domain`: framework ve servislerden bağımsız temel tipler.
- `internal/scraper`: şirket/kaynak adapter sözleşmeleri.
- `internal/analyzer`: deterministik ve LLM tabanlı ilan analizi sözleşmesi.
- `internal/store`: kalıcı veri erişimi sözleşmeleri.
- `internal/orchestrator`: kaynakları izole ederek uçtan uca taramayı yürütür.
- `internal/scheduler`: doğrulanmış cron/zaman dilimi ile orchestrator'ı process
  içinde zamanlanmış olarak tetikler.
- `internal/backup`: SQLite destekli snapshot, günlük tetikleme ve sınırlı
  retention uygular.
- `internal/push`: RFC 8291 payload şifreleme, RFC 8292 VAPID kimliği ve kalıcı
  delivery kuyruğunu işleyen sınırlı retry worker'ı.
- `internal/httpapi`: PWA'nın kullandığı HTTP uçları.
- `internal/database`: SQLite bağlantısı, sıralı migration uygulaması ve
  migration kayıtlarını doğrulayan readiness denetimi.
- `web`: backend secret'larına erişmeyen PWA istemcisi.

## Production runtime ve CI tabanı

Production image'ları desteklenen Go 1.26, Alpine 3.24, Node 24 LTS ve nginx
1.30 kararlı sürüm hatlarıyla oluşturulur. Backend image'ı derleme aşamasından
yalnızca statik API ikilisini ve migration'ları non-root Alpine runtime'a taşır;
frontend image'ı Vite çıktısını nginx ile sunar. GitHub Actions bu iki image'ı
test, typecheck ve bağımlılık denetiminden sonra `push: false` ile build eder.
`main` publish akışı aynı kapılardan sonra image'ları `amd64`/`arm64` GHCR
digest'leriyle, SBOM/provenance ve attestation ile yayımlar. Manuel production
adımı digest manifestini korumalı environment üzerinden, Cloudflare Tunnel
arkasındaki host portsuz Compose runtime'ına deploy eder; runtime secret'ları
repository dışında salt-okunur dosyalardan bağlanır.
Deploy job aynı event commit'ini sparse checkout ederek yalnız production
Compose, nginx ve shell scriptlerini checksum doğrulamalı bir bundle olarak
aktarır. Bundle commit SHA adlı immutable dizine atomik kurulur; image manifesti
aynı `DEPLOY_REVISION` değerini taşıdığı için otomatik recovery ve manuel rollback
her image setini kendi revision'ındaki Compose ve smoke sözleşmesiyle çalıştırır.

Kaynak ve aday profili ayarları `internal/config` tarafından katı biçimde
doğrulanır. Bilinmeyen JSON alanları kabul edilmez; böylece yazım hataları
sessizce varsayılan davranışa dönüşmez.

## Liveness, readiness ve HTTP sınırı

`GET /health` bağımlılık çağrısı yapmayan liveness endpoint'idir; process ayakta
olduğu sürece `200` döner. `GET /ready` ise iki saniyelik request context'i
içinde SQLite `Ping` yapar, `schema_migrations` tablosunu sorgular ve image ile
paketlenmiş bütün migration'ların kayıtlı olduğunu doğrular. Hata HTTP yanıtına
aktarılmaz; yalnız `503 {"status":"not_ready"}` döner ve ayrıntı
yapılandırılmış sunucu logunda kalır.

HTTP middleware API doğrudan erişildiğinde de `nosniff`, referrer, frame ve CSP
başlıklarını uygular; log kaydı method, path, durum ve süreyi içerir. Compose
nginx'i API başlıklarını tekrar etmeden aynı politikayı statik PWA'ya da uygular.
Bu nginx dosyası yerel plain-HTTP listener'dır; HSTS burada kasıtlı olarak yoktur.
HSTS yalnız HTTPS sonlandıran production proxy'de yapılandırılmalıdır.

Compose API healthcheck'i `/ready` çağırır. Web container'ı
`service_healthy` koşuluyla API sağlıklı olana kadar bekler ve kendi `/ready`
proxy healthcheck'ini çalıştırır. Her iki runtime image'ına healthcheck komutu
olarak kullanılan `wget` açıkça eklenmiştir.

## Bağımlılık yönü

Domain dış servisleri bilmez. Scraper, analyzer ve store katmanları domain
tiplerini kullanır. Orchestrator bu portları birleştirir. HTTP ve zamanlanmış
görevler orchestrator'ı tetikler.

```text
HTTP / scan scheduler ---> orchestrator ---> scraper adapters
                             |---------> listing analyzer ---> OpenRouter
                             |---------> repository --------> SQLite
backup timer ----------------'                              |
                                                           `--> consistent snapshot volume
notifications ---------------------------------------------> Web Push
```

## Web Push outbox ve teslimat semantiği

`notifications`, ilan olayı başına tek ve versionlanmış dedup kaydıdır.
`notification_payloads` kilit ekranına gidecek minimum başlık/gövde, aynı-origin
PWA deep-link'i ve Web Push `Topic` değerini snapshot olarak tutar.
`push_subscriptions` endpoint ile tarayıcının `p256dh`/`auth` sırlarını cihaz
bazında saklar; `notification_deliveries` ise olayın her cihaz için bağımsız
pending/sending/sent/failed/cancelled durumunu, lease'i ve deneme sayısını taşır.

`SQLiteRepository.SaveAnalysis`, ilk başarılı analiz bilgisini
`first_processed_at` ile kalıcılaştırır. Birincil şirket + açık başvuru + ilgili +
`uygun` koşulu sağlanırsa analiz upsert'i, kararlı
`listing:<id>:new-primary-suitable:v1` olayı, güvenli payload ve mevcut cihazlara
fan-out tek transaction'da commit edilir. Hiç abonelik yoksa olay `cancelled`
kapanır ve sonradan abone olan cihaza geçmiş bildirim gönderilmez. Böylece analiz
commit edilip outbox'ın kaybolduğu bir ara durum oluşmaz; unique event ve
event/device anahtarları tekrar taramada çoğalmayı önler.

Dispatcher ağ çağrısı boyunca SQLite transaction'ı tutmaz. Süresi dolmuş lease'i
yeniden alır; 2xx'i cihaz başarısı olarak kaydeder, 404/410 veya süresi dolmuş
aboneliği kaldırır, geçici ağ/408/425/429/5xx sonucunu sınırlı exponential backoff
ile erteler ve diğer 4xx sonuçlarını kalıcı hata sayar. Push servisinin isteği
kabul etmesiyle SQLite başarı kaydı arasındaki process çökmesinde taşıma en az bir
kez olabilir; Web Push `Topic`, service-worker notification `tag` ve outbox dedup
aynı olayın kullanıcıya tekrar görünme riskini sınırlar.

Push endpoint'i bir capability URL olduğu için API bunu yanıtta/logda göstermez;
yalnızca HTTPS, credential/fragment içermeyen ve localhost/private IP olmayan
adresler kabul edilir. Payload; şirket/ilan başlığı ve iç listing deep-link'iyle
sınırlıdır, aday profili, notlar, ham ilan metni ve resmî dış URL'yi taşımaz.

PWA bildirim iznini sayfa yüklenirken istemez. Kullanıcı eylemi public VAPID
key'i backend'den alır, service worker kaydındaki `PushManager` aboneliğini
idempotent API'ye yollar veya kapatma sırasında hem backend kaydını hem tarayıcı
aboneliğini kaldırır. `?listing=<id>` aynı dashboard bundle'ında ilan detayını
yükler; kart açma/kapama ve browser history aynı URL sözleşmesini paylaşır.
Service worker payload metinlerini sınırlar, dış origin URL'lerini `/` hedefine
indirger ve önce mevcut same-origin pencereyi navigate/focus eder.

## Zamanlanmış tarama ve eşzamanlılık

`cmd/api`, HTTP sunucusunu dinlemeye almadan önce `SCAN_SCHEDULE` için beş alanlı
cron sözleşmesini ve `SCAN_TIMEZONE` IANA değerini doğrular. Varsayılan zaman
dilimi `Europe/Istanbul`, varsayılan ifade `0 9 * * 1`'dir. Scheduler bir sonraki
yerel takvim dakikasını hesaplar ve `Run(ctx, "scheduled")` çağrısını yapar.
Uygulama yeniden başladığında geçmiş tetiklemeler yeniden oynatılmaz.

HTTP'nin manuel çağrısı ile scheduler, aynı `CoordinatedRunner` etrafında
toplanır. Bu process-içi kilit yalnızca bir `Run` akışına izin verir; ikinci
manuel çağrı `ErrScanInProgress` üzerinden HTTP `409` alır, eşzamanlı scheduled
çağrı ise hata olarak yapılandırılmış loga yazılır. `SIGINT` veya `SIGTERM` ana
context'i iptal eder; scheduler timer'ı ve aktif scheduled run bu context'i
görür, API shutdown'ı scheduler'ın durmasını en fazla aynı 10 saniyelik bütçe
içinde bekler.

## SQLite snapshot ve retention

`internal/backup`, etkinleştirildiğinde aynı açık SQLite bağlantısında `VACUUM
INTO` çalıştırır. Bu, WAL eşlikçi dosyalarını veya ana veritabanı dosyasını ham
kopyalamadan transactionally consistent bir snapshot üretir. Snapshot önce aynı
private dizindeki `.partial` dosyasına yazılır, SQLite `integrity_check` ile
doğrulanır, `0600` izni uygulanır ve atomik rename ile yayımlanır. Dizin `0700`
olarak zorlanır. Snapshot ismi zaman sıralı olduğundan yalnızca uygulamanın
`internship-tracker-*.db` dosyaları, `BACKUP_RETENTION` sınırının üstünde en
eskiden başlayarak temizlenir.

`BACKUP_ENABLED` varsayılan olarak `false` olduğundan geliştirme süreci disk
çıktısı üretmez. Etkin production kurulumunda dizin, günlük `HH:MM` saati, IANA
timezone ve pozitif retention değerinin tamamı dinleme öncesinde doğrulanır.
Ana signal context'i günlük timer'a ve çalışmakta olan `VACUUM INTO` çağrısına
aktarılır; graceful shutdown bu goroutine'i aynı kapanış bütçesinde bekler.

## Secret ilkeleri

- OpenRouter ve Web Push private key PWA bundle'ına girmez.
- `.env` Git tarafından yok sayılır.
- CI ve deployment ortamında secret store kullanılmalıdır.
- Model isteğinden doğrudan iletişim bilgileri çıkarılmalıdır.

## İlk uygulama adımı

İlk gerçek özellik, fixture tabanlı Meteksan/kariyer.net adapter'ı ile
SQLite repository'nin bağlandığı tek kaynaklı dikey dilimdir. `/api/v1/scan`
bu orchestrator zincirini senkron çalıştırır ve kaynak bazlı sonucu döndürür.

Kariyer.net adapter'ı şirket profilindeki `/is-ilani/` bağlantılarını standart
ilanlara dönüştürür. Şirket başlığı bulunamazsa sayfa değişmiş veya erişim
engellenmiş kabul edilir; bu durum geçerli bir “sıfır ilan” sonucu değildir.
Her profil URL'si bağımsız bir kaynak adapter'ıdır. Aynı şirket altındaki
birden fazla `sources[]` girdisi ortak şirket kimliğiyle dedup edilir;
iştiraklerde farklı olan beklenen `<h1>` değeri `page_name` ile tanımlanır.

Lever adapter'ı genel bir ilan listesi taramak yerine yapılandırılan resmî tek
ilan URL'sini okur. Yalnızca `jobs.lever.co` HTTPS URL'lerini ve şirket/ilan
kimliğinden oluşan yolu kabul eder. Sayfadaki `posting-page`, ilan başlığı ve aynı
ilana ait `/apply` bağlantısı birlikte bulunmadığında kaynak kapalı ya da yapısı
değişmiş sayılır. SQLite'a sayfanın tamamı yerine başlık, kategoriler, iş tanımı
ve gereksinimlerden oluşan normalize metin gider.
Lever'ın herkese açık robots politikasındaki bir saniyelik crawl aralığı aynı
kalıcı domain erişim bütçesiyle uygulanır. 403/429/challenge yanıtları kısa ve
güvenli teşhislerle devre kesiciyi tetikler; yanıt gövdesi saklanmaz.

### Kaynak strateji dispatch'i (Faz 9)

`internal/scraper/registry.go`, adapter adını (`kariyer_net`, `lever`, ...) bir
`SourceFactory`'ye eşleyen veri-odaklı bir tablo (`adapterFactories`) tutar.
`cmd/api/main.go`'daki `configureSources`, sabit kodlu bir `switch` yerine
`scraper.NewSource(adapter, spec)` ile bu tabloyu kullanır. Her kaynak ayrıca
bir `strategy` alanı taşır (`internal/config.SourceConfig.EffectiveStrategy`):
açıkça belirtilmemişse, Faz 9 öncesi elle yazılmış adapter'lar (`kariyer_net`,
`lever`) `legacy_html` stratejisine varsayılan olarak atanır. Strateji,
`company_sources.strategy` sütununda (`migrations/006_source_strategy.sql`)
kalıcı hale gelir. Faz 10-12'nin ekleyeceği `json_ld`, `ats_api`,
`learned_selector`, `llm_generic` ve el ile takip edilen `manual` stratejileri
aynı tabloya yeni fabrika kayıtları eklenerek bağlanır; downstream (dedup,
analiz, bildirim) değişmez. Ayrıntılı gerekçe için
`staj-takip-spec-v2.md` §16, Faz 9-14.

### Yapılandırılmış-veri adapter'ları (Faz 10)

Faz 10, ucuz ve deterministik (AI'sız) iki adapter'ı aynı `adapterFactories`
tablosuna ekler; ikisi de yalnızca `domain.RawListing`'e normalize edip mevcut
dedup/analiz yoluna girer:

- `json_ld` (`internal/scraper/jsonld.go`, strateji `json_ld`): Bir kariyer
  sayfasındaki `<script type="application/ld+json">` bloklarını okur, tekil
  obje / dizi / `@graph` sarmalını düzleştirir ve `schema.org` `JobPosting`
  objelerini çıkarır. `title` zorunludur; `url` verilmişse query/fragment
  soyularak kanonikleştirilir, yoksa sayfa URL'sine düşer; `employmentType`,
  `jobLocation.address.addressLocality`, `datePosted`, `validThrough` ve
  tag'leri sökülmüş `description` normalize metne katılır. Hiç JSON-LD bloğu,
  hiç `JobPosting` veya başlıksız `JobPosting` bulunması sessiz "sıfır ilan"
  değil, `ErrUnexpectedPage` üretir.
- `greenhouse` (`internal/scraper/greenhouse.go`, strateji `ats_api`): Herkese
  açık Greenhouse board API'sini (`boards-api.greenhouse.io/v1/boards/{token}/
  jobs`) tüketir; scraping değil yapılandırılmış JSON. `content=true` zorlanır,
  her ilanın HTML-escape'li `content` alanı bir kez unescape edilip tag'leri
  sökülerek metne indirilir. Başlıksız veya geçersiz `absolute_url`'lu ilan hata
  verir; boş board (`{"jobs":[]}`) ise geçerli deterministik bir sonuçtur.

Yeni adapter'lar `adapterDefaultStrategy` üzerinden stratejilerini kendiliğinden
çıkarır, böylece kaynak eklemek yalnızca bir kayıt (adapter + URL) olur.
Kabul kanıtı: `internal/acceptance/phase10_test.go`, bir JSON-LD sayfasını tam
orchestrator → dedup → analiz → dashboard yolundan geçirir ve değişmeyen ikinci
taramada sıfır yeni kayıt/tekrar analiz doğrular.

### İzleme listesi ve taranamayan kaynaklar (Faz 6 ön hazırlığı)

Dashboard iki ayrı, kesişmeyen liste sunar. `manual_checks`
(`company_sources.last_error IS NOT NULL AND companies.tracking_status !=
'manual'`) yalnız scraper'ın deneyip başarısız olduğu kaynakları gösterir —
tanı amaçlı, geçicidir. `watchlist` (`companies.tracking_status = 'manual'`)
kullanıcının bilinçli olarak elle takip etmeyi seçtiği, kalıcı bir listedir;
scraper hiç denenmez (`enabled: false`), hata durumundan bağımsızdır. Bir
kaynak asla iki listede birden görünmez. `company_sources.last_manual_check_at`
(`migrations/007_watchlist.sql`) kullanıcının "Kontrol ettim" ile işaretlediği
son zamanı tutar; `PUT /api/v1/watchlist/{id}/checked`
(`store.TrackingRepository.MarkSourceChecked`) bu alanı günceller ve güncel
dashboard snapshot'ını döner. `config.CompanyConfig.TrackingStatus` şirket
düzeyinde `active`/`manual`/`paused` değerini taşır ve `RegisterSource` ile
`companies.tracking_status`'a yazılır. İlk watchlist girdileri (Akdoğan Tech,
Turkcell, Havelsan) `configs/sources.json`'da `adapter: "manual"`,
`strategy: "manual"` ile tanımlıdır — bkz. `staj-takip-spec-v2.md` §16, Faz 6
kaynak keşif notları.

Faz 3.5 kabul akışı adapter'ı doğrudan SQLite repository ve `ModelAnalyzer` ile
orchestrator içinde çalıştırır. İkinci tarama aynı canonical URL'yi günceller,
ancak işlenmiş analizi yeniden çağırmaz. Sonuç, üretimde kullanılan dashboard
HTTP handler'ından okunur; kabul veritabanı yalnızca geçici dizinde oluşturulur.
Canlı çalışmanın repoda tutulan tek artefaktı kısa Markdown kanıtıdır; API
anahtarı, tam kaynak gövdesi ve SQLite dosyası kalıcılaştırılmaz.

SQLite repository kaynakları kararlı `source_key` değerleriyle şirketlere
bağlar. İlan kimliği şirket adı ve canonical URL'nin SHA-256 özetidir. Canonical
URL'den fragment, UTM ve bilinen reklam takip parametreleri çıkarılır; sorgu
parametreleri kararlı sıraya getirilir.

Deterministik analizci API anahtarı gerektirmeyen varsayılan seçenektir. Sağlayıcı
tabanlı akış aynı `ListingAnalyzer` portunun arkasındaki `ModelAnalyzer` ile
kurulur. `ModelProvider` yalnızca model isteğini ve ham cevabı taşır; OpenRouter
ve Google Gemini HTTP ayrıntıları kendi adapter'larında kalır. `LLM_PROVIDER` ve
`LLM_MODEL` uygulama başlangıcında seçilir; desteklenmeyen sağlayıcı veya seçilen
adapter'ın eksik secret'ı başlangıcı durdurur.

`ModelAnalyzer`, sağlayıcıya doğrudan config profilini vermez. Minimize edilmiş
istek yalnızca bölüm/alan, sınıf, GPA, odak alanları, deneyim alanları ve konum
tercihlerini içerir; üniversite ve deneyim kurumu adları dışarıda bırakılır.
OpenRouter isteği strict JSON Schema response formatını kullanır. Dönen içerik
backend'de `DisallowUnknownFields`, enum, aralık, zorunlu alan ve karar durumu
tutarlılığı kontrollerinden tekrar geçer.

Google adapter'ı aynı şemayı `responseJsonSchema` ve `application/json` response
mime type ile gönderir; `usageMetadata` alanını ortak prompt/completion/toplam
token tipine dönüştürür. Gemini 3 modellerinde çıktı bütçesinin düşünme tarafından
tüketilmesini sınırlamak için ayarlanabilir `thinkingLevel` kullanılır. Gemma
model adlarında Gemini'ye özgü thinking config gönderilmez. API anahtarı yalnızca
`x-goog-api-key` header'ında taşınır. Google istemcisinin 60 saniyelik timeout'u
ve API sunucusunun 90 saniyelik write timeout'u düşünmeli Flash modellerinin
ölçülen daha yüksek gecikmesine alan bırakır.

Geçici taşıma hataları, timeout, 408/429/5xx, bozuk JSON ve şema ihlalleri 100/200
ms beklemeli ve toplam üç denemeli sınırlı retry alır. 4xx gibi kalıcı sağlayıcı
hataları tekrarlanmaz. Başarılı denemelerin prompt/completion/toplam tokenları ve
model için ayarlanan milyon-token fiyatlarından hesaplanan tahmini USD maliyeti
`listing_analyses` içinde provider/model ile saklanır.

Model talimatı `karar_bekliyor`, `needs_user_decision` ve `decision_question`
alanlarının birlikte değişme kuralını açıkça taşır. Backend doğrulaması yine
otoritedir; cevap iş kuralını ihlal ederse sonraki sınırlı denemeye yalnızca kısa
doğrulama nedeni eklenir ve katı doğrulama gevşetilmez.

Model ilan metniyle birlikte kişisel olmayan `fetched_at` kanıt zamanını da alır.
Aday tarama anında ilan şartının tam bir sınıf altındaysa fakat fırsat sonraki
akademik dönemde başlıyorsa sınıf geçişi varsayılmaz; ilan elenmek yerine
başlangıçtaki sınıf durumunu soran `karar_bekliyor` sonucuna yönlendirilir.

Ham ilan analizden önce kalıcılaştırılır. Analiz başarısızlığı aynı ilan için
`eligibility_status=karar_bekliyor`, `processing_status=pending`, artan
`retry_count` ve kısaltılmış `last_error` üretir; kayıt dashboard karar kuyruğunda
kalır. Normal tarama duplicate bir pending ilanı yeniden işler. Ayrıca
`POST /api/v1/analyses/retry`, saklanan ham metni kullanarak kaynağa ve onun
erişim bütçesine dokunmadan en fazla 25 pending analizi işler.

## Tarama raporu ve kaynak izolasyonu

Orchestrator her manuel taramadan önce `scan_runs` kaydını `running` durumunda
açar. Her kaynak bağımsız işlenir; fetch, kayıt veya analiz hatası yalnızca o
kaynağı başarısız sayar ve sıradaki kaynak çalışmaya devam eder. Kaynak sonucu
`company_sources.last_success_at` veya zaman damgalı `last_error` alanına
yazılır.

Tarama sonunda rapor `completed`, `partial` ya da `failed` olarak kapatılır;
başarılı/başarısız kaynak sayıları, yeni ilan sayısı ve kısa JSON hata özeti
kalıcılaştırılır. HTTP yanıtı tarama kimliğini ve aynı özeti taşır, dashboard
ise en son tamamlanmış raporu SQLite'tan okur. İstek iptal edilmiş olsa bile
başlatılmış raporu kapatmak için yalnızca raporlama yazıları iptalden ayrılır.

Kariyer.net adapter'ı 403/429 ve HTTP 200 içindeki challenge sayfalarını tipli
erişim hatası olarak döndürür. Yanıt gövdesi saklanmaz; teşhis için yalnızca
durum kodu, `Retry-After`, `Server`, `CF-Ray` ve challenge işareti taşınır.
Domain erişim bütçesi `source_access_states` tablosunda tutulur. Son deneme,
sonraki izin zamanı, cooldown, ardışık koruma hatası ve son güvenli teşhis
alanları süreç yeniden başlatıldığında kaybolmaz.

Kariyer.net ve alt alan adları tek `kariyer.net` scope'una normalize edilir.
Orchestrator scan başında bu scope'u atomik olarak rezerve eder; 24 saatlik
minimum aralık eşzamanlı veya tekrarlı manuel taramalarla aşılamaz. İlk
403/429/challenge hatası 6, 12 ve en fazla 24 saatlik ardışık cooldown'u
başlatır, `Retry-After` daha ilerideyse onu seçer ve aynı scandeki kalan
Kariyer.net profillerini ağ çağrısı yapmadan atlar. Günlük erişim bütçesi daha
ilerideyse kullanıcıya gösterilen tekrar zamanı iki sınırın en geç olanıdır.

Devre kesici yalnızca aynı erişim scope'undaki kaynakları durdurur; başka
domainlerdeki kaynak izolasyonu devam eder. Ayrı sunucu dağıtımı ev IP'sini
izole eder fakat erişim politikasını değiştirmez ve korumayı aşma amacı taşımaz.

## Faz 4 başvuru takibi

Dashboard repository çıktısı, kartlarda gösterilecek analiz özeti ve son başvuru
tarihinin yanında aktif başvurunun durumunu, kullanıcı tarihini ve mülakat
tarihini taşır. İlanın tam analiz alanları yalnızca kullanıcı kartı açtığında
`GET /api/v1/listings/{id}` üzerinden okunur; ham ilan metni PWA'ya gönderilmez.

`PUT /api/v1/listings/{id}/application`, tek kullanıcıya ait `application_tracking`
kaydını upsert eder. Durum domain sözleşmesindeki beş değerden biri olmalıdır;
tarih alanları RFC3339 olarak alınır, UTC saklanır ve notlar 2000 karakterle
sınırlıdır. Kaydı olmayan bir ilan için `404`, bozuk durum/tarih için `400`
döner. Kaynak taraması sırasında yazılan zaman damgalı `last_error` kayıtları da
dashboard'un manuel kontrol listesine girer ve sonraki kaynak başarısında listeden
çıkar.

React istemcisi dashboard snapshot'ını ana görünümün tek okuma modeli olarak
kullanır. Liste kartı seçildiğinde detay ayrı istekle yüklenir ve sağ panel açılır;
başvuru kaydı başarıyla değişince hem panel hem dashboard yeniden okunur. Böylece
aktif başvuru sayısı, yaklaşan tarihler ve durum etiketi kalıcı SQLite sonucuyla
aynı kalır. İki kolonlu masaüstü görünümü dar ekranda tek kolona iner; detay
paneli telefonda ekran genişliğini kullanır.
