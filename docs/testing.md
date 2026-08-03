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
HTTP 403/418/429/5xx, timeout ve iptal edilen istek senaryolarını kapsar.
HTTP 200 challenge fixture'ı ile 429 testi ayrıca `Retry-After`, `Server` ve
`CF-Ray` teşhis bilgilerinin tipli erişim hatasına aktarıldığını doğrular.

Faz 2 fixture kabul testi tanınmayan STM sayfasını önce çalıştırır; ardından
ASELSAN ve ASELSANNET profillerinin tamamlandığını, iki profilde görülen ortak
URL'nin tek ilan kaldığını ve taramanın `partial` raporuyla SQLite'a yazıldığını
doğrular. Böylece kaynak izolasyonu canlı kariyer sitesine bağlanmadan sınanır.

OpenRouter normal testlerde çağrılmaz. `ListingAnalyzer` fake/mock
uygulamalarıyla geçerli JSON, geçersiz cevap, timeout ve retry senaryoları
test edilir.

Deterministik analizci testleri ilgili bir backend stajını, daha yüksek sınıf
şartını, kapanmış ilanı ve iptal edilen context'i kapsar.

Veritabanı testleri geçici dizinde gerçek SQLite dosyası açar. Migration'ların
ilk açılışta uygulanması ve sonraki açılışlarda tekrar çalışmaması doğrulanır.
Repository testleri takip parametreleri farklı aynı URL'nin tek ilan olarak
kalmasını, içeriğin/`last_seen_at` alanının yenilenmesini ve analizin kalıcı
olarak yazılmasını doğrular. Scan repository testi kaynak hatasının zaman ve
kısa sebeple saklanmasını, sonraki başarıda temizlenmesini ve dashboard'un son
kalıcı tarama raporunu döndürmesini kapsar.
Erişim durumu testi ilk domain rezervasyonunun izinli, 24 saat içindeki ikinci
rezervasyonun engelli olduğunu; `Retry-After` değerinin cooldown'u uzattığını,
6/12/24 saat geri çekilme tavanını ve başarı sonrası hata durumunun sıfırlandığını
gerçek SQLite üzerinde doğrular.

Orchestrator devre kesici testi aynı scope'taki ilk kaynağın 403 vermesinden
sonra ikinci kaynağın taşıma katmanına hiç ulaşmadığını, tek rezervasyon
yapıldığını, güvenli teşhislerin repository'ye aktarıldığını ve başarı kaydıyla
cooldown'un yanlışlıkla temizlenmediğini doğrular. HTTP testi atlanan kaynağın
`skipped` ve `retry_at` alanlarını `207` yanıtta gösterir.

## Frontend

```bash
npm --prefix web test
npm --prefix web run build
```

Saf sınıflandırma ve görünüm yardımcıları Vitest ile test edilir. Dashboard
akışları geliştikçe component ve tarayıcı tabanlı uçtan uca testler eklenir.
Frontend araçları Node 20.19 veya daha yeni bir Node 20 sürümü gerektirir;
Vite 7 ve Vitest 4 güvenlik yamaları alınmış sabit sürümlere kilitlenir.
Vite yapılandırması TypeScript tarafından yalnızca tip kontrolünden geçirilir;
`npm run typecheck` uygulama ve Vite yapılandırmasını ayrı ayrı `noEmit` ile
kontrol eder ve kaynak ağacına JavaScript/derleme-meta çıktısı üretmez.

Faz 1 backend kabul testi aynı Meteksan fixture'ını scraper, deterministik
analizci ve gerçek SQLite repository üzerinden iki kez çalıştırır. İlk tarama
iki yeni kayıt, ikinci tarama sıfır yeni kayıt üretmeli; uygun staj dashboard
sorgusunda görünmelidir.

## CI

GitHub Actions her push ve pull request'te Go format kontrolü, backend
test/build ve frontend test/build adımlarını çalıştırır. Gerçek API anahtarı
ve ücretli entegrasyon çağrısı CI testlerine dahil edilmez.
