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
bu dilim tamamlanana kadar kasıtlı olarak `501 Not Implemented` döndürür.
