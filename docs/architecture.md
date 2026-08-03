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

Faz 1 analizcisi yalnızca deterministik metin sinyallerini kullanır. Fırsat
türü, profilin öncelikli alanları, sınıf şartı, Ankara/uzaktan sinyali ve
çalışma modeli çıkarılır. Bu sonuç bir LLM kararı sayılmaz; Faz 3'te aynı
`ListingAnalyzer` arayüzünün arkasına sağlayıcı tabanlı analiz eklenecektir.

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
