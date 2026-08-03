# Test yaklaşımı

## Backend

```bash
go test ./...
```

Scraper testleri canlı web sitelerine bağlanmamalıdır. Her adapter için
`testdata/` altında kaydedilmiş HTML fixture'ları kullanılacaktır. Canlı site
kontrolleri ayrı ve isteğe bağlı entegrasyon testleri olarak tutulmalıdır.

OpenRouter normal testlerde çağrılmaz. `ListingAnalyzer` fake/mock
uygulamalarıyla geçerli JSON, geçersiz cevap, timeout ve retry senaryoları
test edilir.

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
