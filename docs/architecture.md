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
- `internal/httpapi`: PWA'nın kullandığı HTTP uçları.
- `internal/database`: SQLite bağlantısı ve sıralı migration uygulaması.
- `web`: backend secret'larına erişmeyen PWA istemcisi.

## Production runtime ve CI tabanı

Production image'ları desteklenen Go 1.26, Alpine 3.24, Node 24 LTS ve nginx
1.30 kararlı sürüm hatlarıyla oluşturulur. Backend image'ı derleme aşamasından
yalnızca statik API ikilisini ve migration'ları non-root Alpine runtime'a taşır;
frontend image'ı Vite çıktısını nginx ile sunar. GitHub Actions bu iki image'ı
test, typecheck ve bağımlılık denetiminden sonra `push: false` ile build eder.
Image registry, deployment hedefi ve runtime secret'ları bu güvenlik kapısından
ayrı, sonraki Faz 5 tesliminde eklenir.

Kaynak ve aday profili ayarları `internal/config` tarafından katı biçimde
doğrulanır. Bilinmeyen JSON alanları kabul edilmez; böylece yazım hataları
sessizce varsayılan davranışa dönüşmez.

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
