# Test yaklaşımı

## Backend

```bash
go test ./...
go vet ./...
```

Scraper testleri canlı web sitelerine bağlanmamalıdır. Her adapter için
`testdata/` altında kaydedilmiş HTML fixture'ları kullanılacaktır. Canlı site
kontrolleri ayrı ve isteğe bağlı entegrasyon testleri olarak tutulmalıdır.

Lever adapter testi canlı ilanın tamamını kopyalamayan küçük bir HTML fixture'ı
kullanır. Resmî ilan URL'sinin ve tanıtıcı User-Agent'in istendiğini; başlık,
konum, çalışma türü, son başvuru tarihi ve gereksinimlerin normalize edildiğini;
aktif `/apply` bağlantısı olmayan veya beklenen yapıyı taşımayan sayfanın ilan
olarak kabul edilmediğini doğrular.
Adapter'ın `jobs.lever.co` için bir saniyelik kalıcı erişim bütçesi bildirdiği de
test edilir; fixture kabul testindeki saat ikinci taramadan önce bu kadar ilerler.

Kariyer.net adapter testi bellek içi sahte bir HTTP taşıma katmanı kullanır;
iki ilan, tekrarlı bağlantı, sıfır ilan, eksik başlık, değişmiş sayfa işareti,
HTTP 403/418/429/5xx, timeout ve iptal edilen istek senaryolarını kapsar.
HTTP 200 challenge fixture'ı ile 429 testi ayrıca `Retry-After`, `Server` ve
`CF-Ray` teşhis bilgilerinin tipli erişim hatasına aktarıldığını doğrular.

Faz 2 fixture kabul testi tanınmayan STM sayfasını önce çalıştırır; ardından
ASELSAN ve ASELSANNET profillerinin tamamlandığını, iki profilde görülen ortak
URL'nin tek ilan kaldığını ve taramanın `partial` raporuyla SQLite'a yazıldığını
doğrular. Böylece kaynak izolasyonu canlı kariyer sitesine bağlanmadan sınanır.

OpenRouter normal testlerde çağrılmaz. Sahte `ModelProvider` cevapları geçerli
uygun, uygun olmayan ve karar bekleyen sonuçları; bozuk JSON'u; bilinmeyen alanı;
enum/aralık şema hatasını; timeout'u; kalıcı 4xx'i; 429'u; geçici 5xx'i ve üç
denemelik retry sınırını kapsar. Sahte HTTP transport testi OpenRouter request'inin
model ve strict JSON Schema taşıdığını, Bearer anahtarını yalnızca backend
header'ında kullandığını ve usage alanlarını okuduğunu doğrular.
Karar durumu invariant'ının model talimatında bulunduğu ve ilk cevap bu kuralı
ihlal ettiğinde ikinci denemeye kısa doğrulama geri bildirimi eklendiği de fake
provider ile doğrulanır.
Profil minimizasyon testi ayrıca kişisel olmayan ilan erişim zamanının modele
iletildiğini ve gelecek akademik dönemdeki tek sınıflık geçiş belirsizliği için
karar bekleme talimatının bulunduğunu doğrular.

Google adapter testleri aynı şekilde sahte HTTP transport kullanır; endpoint/model
seçimini, `x-goog-api-key` header'ını, anahtarın JSON gövdeye girmediğini, strict
response schema'yı, Gemini düşünme seviyesini, Gemma için thinking config'in
çıkarılmasını, usage dönüşümünü ve kalıcı/geçici HTTP hata sınıflandırmasını
kapsar. `google_live_test.go` yalnızca `integration` build tag'i ve ortamdan
`GEMINI_API_KEY` ile açıkça çağrıldığında gerçek Gemini API'ye gider; normal
`go test ./...` canlı veya ücretli API çağrısı yapmaz.

Profil minimizasyon testi model input'unda yalnızca bölüm/alan, sınıf, GPA, odak,
deneyim alanı ve konum tercihlerinin bulunduğunu doğrular. Üniversite ve deneyim
kurumu adları `analyzerProfile` dönüşümünde atılır.

Deterministik analizci testleri ilgili bir backend stajını, daha yüksek sınıf
şartını, kapanmış ilanı ve iptal edilen context'i kapsar.

Veritabanı testleri geçici dizinde gerçek SQLite dosyası açar. Migration'ların
ilk açılışta uygulanması ve sonraki açılışlarda tekrar çalışmaması doğrulanır.
Repository testleri takip parametreleri farklı aynı URL'nin tek ilan olarak
kalmasını, içeriğin/`last_seen_at` alanının yenilenmesini ve analizin kalıcı
olarak yazılmasını doğrular. Faz 3 repository testi başarısız analizin
`karar_bekliyor/pending` kaldığını, dashboard'dan kaybolmadığını, retry sayısını,
provider/model ile token ve tahmini maliyet alanlarını; başarı sonrası hatanın
temizlenip kaydın `processed` olmasını doğrular. Scan repository testi kaynak hatasının zaman ve
kısa sebeple saklanmasını, sonraki başarıda temizlenmesini ve dashboard'un son
kalıcı tarama raporunu döndürmesini kapsar.
Erişim durumu testi ilk domain rezervasyonunun izinli, 24 saat içindeki ikinci
rezervasyonun engelli olduğunu; `Retry-After` değerinin cooldown'u uzattığını,
6/12/24 saat geri çekilme tavanını ve başarı sonrası hata durumunun sıfırlandığını
gerçek SQLite üzerinde doğrular.

Faz 4 repository testleri analizli bir ilanın ayrıntı alanlarını okur; başvuru
durumu, kullanıcı son tarihi, mülakat zamanı ve notlarını gerçek SQLite üzerinde
upsert edip dashboard aktif başvuru görünümünü doğrular. Hatalı durum ve olmayan
ilan reddedilir. Kaynak hatasının manuel kontrol listesine girdiği de tarama
raporu testinin parçasıdır. HTTP testleri ilan ayrıntısı ile RFC3339 tarihli
başvuru güncellemesini ve bozuk tarih için `400` yanıtını fake repository ile
kapsar.

Orchestrator yeniden işleme testleri duplicate pending ilanın sonraki taramada
yeniden analiz edilip yeni ilan sayılmadığını ve ayrı retry API akışının kaynak
fetch'i yapmadan saklanan ham metni işlediğini doğrular. HTTP testi kısmi yeniden
işleme sonucunu `207` olarak döndürür.

Scheduler testleri ağ veya gerçek saat beklemesi kullanmaz. Beş alanlı cron
ifadesinin ve IANA timezone'un startup'ta reddedilmesini, `Europe/Istanbul`
takviminde bir sonraki haftalık zamanın hesaplanmasını, tetiklemenin
`Run(ctx, "scheduled")` çağrısını ve context iptalinde scheduler'ın durmasını
fake runner ile doğrular. Orchestrator eşzamanlılık testi bloklayan fake runner
ile ikinci taramanın `ErrScanInProgress` döndürdüğünü; HTTP testi bunun kullanıcıya
`409` olarak yansıdığını kapsar.

SQLite backup testleri yalnızca geçici dizin ve SQLite kullanır. Disabled
varsayılanın dizin/dosya üretmediğini; etkin yanlış environment değerlerinin
startup yapılandırmasında reddedildiğini; günlük saatin IANA timezone'a göre
hesaplandığını doğrular. Snapshot testi kaynak veritabanını yazdıktan sonra
`VACUUM INTO` ile backup alır, kaynağa yeni satır ekler, snapshot'ı ayrı SQLite
bağlantısı olarak açarak ilk durumun restore edilebildiğini kanıtlar. Aynı test
`integrity_check`, `0700` dizin/`0600` dosya izinleri ve iki kayıtlık retention
sınırını ağ erişimi olmadan kapsar.

## Faz 3.5 canlı kabul testi

Gerçek ilan kabul testi normal `go test ./...` akışından ayrılır. Test yalnızca
açıkça verilen build tag/ortam ayarlarıyla resmî kariyer kaynağına ve Gemini
API'ye gider. Yalnızca herkese açık tek Lever ilan URL'sini okur; erişim engeli
veya kapalı ilanı aşmaya çalışmaz. Çıktı aktif ilan
kanıt zamanını, canonical URL'yi, analiz durumunu, provider/model/token bilgisini,
dashboard görünürlüğünü ve ikinci çalıştırmadaki dedup sonucunu raporlar.

Aynı yolun CI karşılığı küçük ve güvenli bir kaynak fixture'ı ile fake model
cevabı kullanır. Fixture yalnızca parser ve durum kanıtı için gerekli alanları
içerir; canlı sayfanın tamamı, API anahtarı ve üretilen SQLite dosyası repoya
eklenmez. Canlı kaynak erişilemiyor veya başvuru kapanmışsa test başarılı sayılmaz
ve sahte ilanla çıkış kriteri karşılanmış kabul edilmez.

Normal `internal/acceptance` testi fixture Lever adapter'ını, strict şemadan
geçen fake Google cevabını, gerçek migration/SQLite repository'yi ve dashboard
HTTP handler'ını birlikte kullanır. İlk taramada bir, ikinci taramada sıfır yeni
ilan; tek kalıcı kayıt; tek model çağrısı; provider/model/token/tahmini maliyet
alanları ve `kismen_uygun` dashboard sonucu beklenir. Canlı karşılığı yalnızca
`RUN_REAL_LISTING_ACCEPTANCE=1`, `integration` etiketi ve `GEMINI_API_KEY` birlikte
verildiğinde çalışır. Canlı analiz başarısız olursa repoya veya test artefaktına
ham sağlayıcı cevabı yazmadan SQLite'taki 500 baytla sınırlı güvenli hata nedenini
test çıktısında gösterir.

3 Ağustos 2026 başarılı canlı çalışmasının kaynak zamanı, kısa aktiflik kanıtı,
canonical URL, kullanım metadatası, dashboard sonucu ve dedup sayıları
`docs/acceptance/phase-3.5-2026-08-03.md` dosyasında kayıtlıdır.

Orchestrator devre kesici testi aynı scope'taki ilk kaynağın 403 vermesinden
sonra ikinci kaynağın taşıma katmanına hiç ulaşmadığını, tek rezervasyon
yapıldığını, güvenli teşhislerin repository'ye aktarıldığını ve başarı kaydıyla
cooldown'un yanlışlıkla temizlenmediğini doğrular. HTTP testi atlanan kaynağın
`skipped` ve `retry_at` alanlarını `207` yanıtta gösterir.

## Frontend

```bash
npm --prefix web test
npm --prefix web run build
npm --prefix web audit --audit-level=high
```

Saf sınıflandırma ve görünüm yardımcıları Vitest ile test edilir. Dashboard
akışları geliştikçe component ve tarayıcı tabanlı uçtan uca testler eklenir.
Frontend araçları Node 24.18 veya daha yeni bir Node 24 LTS sürümü gerektirir;
Vite 7 ve Vitest 4 güvenlik yamaları alınmış sabit sürümlere kilitlenir.
Vite yapılandırması TypeScript tarafından yalnızca tip kontrolünden geçirilir;
`npm run typecheck` uygulama ve Vite yapılandırmasını ayrı ayrı `noEmit` ile
kontrol eder ve kaynak ağacına JavaScript/derleme-meta çıktısı üretmez.

Faz 4 frontend testi öncelik gruplamasının yanında analiz, manuel takip ve
mülakat tarihlerinden en erkenini seçen yaklaşan tarih yardımcısını kapsar.
Typecheck; dashboard, detay ve başvuru formu API sözleşmelerinin React kullanımıyla
uyumunu doğrular. Production build responsive dashboard, detay paneli ve formu
aynı PWA bundle'ında üretir.

`internal/acceptance/phase4_test.go`, gerçek migration/SQLite ile analizli ilan
oluşturur; üretim HTTP handler'ından detayı okur, RFC3339 tarihli başvuru kaydını
günceller ve son dashboard yanıtında aktif başvuruyu, yaklaşan tarihi ve manuel
kontrol kaynağını birlikte doğrular. Test ağ veya ücretli sağlayıcı kullanmaz.

Web Push store testleri analiz + outbox atomikliğini hata veren SQLite trigger ile,
versionlanmış olay dedup'ını, yalnız `primary`/açık/ilgili/`uygun` koşulunu,
başarısız analiz sonrası ilk başarıyı ve 410 temizliğinin yalnız ilgili cihazı
etkilemesini doğrular. Dispatcher testleri fake sender ile çoklu cihaz, 2xx,
429 retry, toplam deneme sınırı ve 410 yollarını ağsız çalıştırır. Şifreleme testi
RFC 8291 Appendix A'nın yayımlanmış vektörüyle ciphertext'i byte düzeyinde;
VAPID testi RFC 8292 JWT audience/expiry/subject alanlarını ve ham ES256 imzasını
bağımsız doğrular. Normal testler gerçek push endpoint'ine istek göndermez.

`internal/acceptance/phase5_test.go`, Lever HTML fixture'ını primary şirket,
fake uygun analizci, gerçek migration/SQLite outbox ve fake push sender üzerinden
iki kez tarar. İlk tarama tek güvenli/deep-linked payload teslim eder; ikinci
tarama yeni event veya delivery üretmez.

Faz 1 backend kabul testi aynı Meteksan fixture'ını scraper, deterministik
analizci ve gerçek SQLite repository üzerinden iki kez çalıştırır. İlk tarama
iki yeni kayıt, ikinci tarama sıfır yeni kayıt üretmeli; uygun staj dashboard
sorgusunda görünmelidir.

## CI

GitHub Actions her push ve pull request'te Go format kontrolü, `go vet`, backend
test/build, frontend test/build, yüksek önem seviyesindeki `npm audit` bulguları
ve iki production Docker image'ının yayınlamayan build adımlarını çalıştırır.
Gerçek API anahtarı ve ücretli entegrasyon çağrısı CI testlerine dahil edilmez.
