# Test yaklaşımı

## Backend

```bash
go test ./...
```

Scraper testleri canlı web sitelerine bağlanmamalıdır. Her adapter için
`testdata/` altında kaydedilmiş HTML fixture'ları kullanılacaktır. Canlı site
kontrolleri ayrı ve isteğe bağlı entegrasyon testleri olarak tutulmalıdır.

Kariyer.net adapter testi bellek içi sahte bir HTTP taşıma katmanı kullanır;
iki ilan, tekrarlı bağlantı, sıfır ilan, eksik başlık, değişmiş sayfa işareti,
HTTP 429 ve iptal edilen istek senaryolarını kapsar.

OpenRouter normal testlerde çağrılmaz. `ListingAnalyzer` fake/mock
uygulamalarıyla geçerli JSON, geçersiz cevap, timeout ve retry senaryoları
test edilir.

Veritabanı testleri geçici dizinde gerçek SQLite dosyası açar. Migration'ların
ilk açılışta uygulanması ve sonraki açılışlarda tekrar çalışmaması doğrulanır.
Repository testleri takip parametreleri farklı aynı URL'nin tek ilan olarak
kalmasını, içeriğin/`last_seen_at` alanının yenilenmesini ve analizin kalıcı
olarak yazılmasını doğrular.

## Frontend

```bash
npm --prefix web test
npm --prefix web run build
```

Saf sınıflandırma ve görünüm yardımcıları Vitest ile test edilir. Dashboard
akışları geliştikçe component ve tarayıcı tabanlı uçtan uca testler eklenir.

## CI

GitHub Actions her push ve pull request'te Go format kontrolü, backend
test/build ve frontend test/build adımlarını çalıştırır. Gerçek API anahtarı
ve ücretli entegrasyon çağrısı CI testlerine dahil edilmez.
