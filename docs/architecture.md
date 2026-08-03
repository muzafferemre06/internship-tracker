# Mimari notları

## Sınırlar

- `cmd/api`: uygulamayı ayağa kaldırır; iş kuralları burada yazılmaz.
- `internal/domain`: framework ve servislerden bağımsız temel tipler.
- `internal/scraper`: şirket/kaynak adapter sözleşmeleri.
- `internal/analyzer`: deterministik ve LLM tabanlı ilan analizi sözleşmesi.
- `internal/store`: kalıcı veri erişimi sözleşmeleri.
- `internal/orchestrator`: kaynakları izole ederek uçtan uca taramayı yürütür.
- `internal/httpapi`: PWA'nın kullandığı HTTP uçları.
- `internal/database`: SQLite bağlantısı ve sıralı migration uygulaması.
- `web`: backend secret'larına erişmeyen PWA istemcisi.

Kaynak ve aday profili ayarları `internal/config` tarafından katı biçimde
doğrulanır. Bilinmeyen JSON alanları kabul edilmez; böylece yazım hataları
sessizce varsayılan davranışa dönüşmez.

## Bağımlılık yönü

Domain dış servisleri bilmez. Scraper, analyzer ve store katmanları domain
tiplerini kullanır. Orchestrator bu portları birleştirir. HTTP ve zamanlanmış
görevler orchestrator'ı tetikler.

```text
HTTP / scheduler
      |
      v
orchestrator ---> scraper adapters
      |--------> listing analyzer ---> OpenRouter
      |--------> repository --------> SQLite
      `--------> notifications -----> Web Push
```

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

SQLite repository kaynakları kararlı `source_key` değerleriyle şirketlere
bağlar. İlan kimliği şirket adı ve canonical URL'nin SHA-256 özetidir. Canonical
URL'den fragment, UTM ve bilinen reklam takip parametreleri çıkarılır; sorgu
parametreleri kararlı sıraya getirilir.

Deterministik analizci API anahtarı gerektirmeyen varsayılan seçenektir. Sağlayıcı
tabanlı akış aynı `ListingAnalyzer` portunun arkasındaki `ModelAnalyzer` ile
kurulur. `ModelProvider` yalnızca model isteğini ve ham cevabı taşır; OpenRouter
HTTP ayrıntıları adapter içinde kalır. `LLM_PROVIDER` ve `LLM_MODEL` uygulama
başlangıcında seçilir, desteklenmeyen sağlayıcı veya eksik OpenRouter secret'ı
başlangıcı durdurur.

`ModelAnalyzer`, sağlayıcıya doğrudan config profilini vermez. Minimize edilmiş
istek yalnızca bölüm/alan, sınıf, GPA, odak alanları, deneyim alanları ve konum
tercihlerini içerir; üniversite ve deneyim kurumu adları dışarıda bırakılır.
OpenRouter isteği strict JSON Schema response formatını kullanır. Dönen içerik
backend'de `DisallowUnknownFields`, enum, aralık, zorunlu alan ve karar durumu
tutarlılığı kontrollerinden tekrar geçer.

Geçici taşıma hataları, timeout, 408/429/5xx, bozuk JSON ve şema ihlalleri 100/200
ms beklemeli ve toplam üç denemeli sınırlı retry alır. 4xx gibi kalıcı sağlayıcı
hataları tekrarlanmaz. Başarılı denemelerin prompt/completion/toplam tokenları ve
model için ayarlanan milyon-token fiyatlarından hesaplanan tahmini USD maliyeti
`listing_analyses` içinde provider/model ile saklanır.

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
