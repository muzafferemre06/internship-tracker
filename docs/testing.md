# Test yaklaşımı

## Backend

Faz 15 model testleri dönemsel program config doğrulamasını, geçersiz durum ve
tarih aralığının reddini, `program_windows` migration'ını ve aynı kanonik program
anahtarının güncel durumla idempotent biçimde güncellenmesini gerçek geçici
SQLite üzerinde kapsar.

Kaynak katalog testleri on iki kanonik birincil şirketin tamamını, Commencis
alias kararını ve her kaynağın kapsama/güven sınıfını production config'inden
doğrular. Tutarsız otomatik/disabled sınıflandırması startup'ta reddedilir;
SQLite testi sınıflandırmanın kaynak sağlık kaydına taşındığını kanıtlar.

Kapsama repository testi yalnız birincil şirketleri raporlar, beş durumun
sayımını ve manuel kaynağın otomatik kapsama paydasından çıkarılmasını gerçek
SQLite ile doğrular. HTTP testi `/api/v1/coverage` JSON sözleşmesini fake
repository ile ağsız sınar.

Frontend kapsama helper testi beş backend durumunun ayrı Türkçe etiket ve görsel
tonunu, bozuk durumun tehlike tonunu, yerel yüzde biçimini ve program durum
etiketlerini doğrular. Typecheck ve production build, endpoint sözleşmesinin
responsive PWA paneline eksiksiz bağlandığını denetler.

`TestPhase15PrimaryCoverageTrustAndProgramWindowEndToEnd`, production config'ini
geçici SQLite'a kaydeder; Commencis Lever ve toplayıcı fixture'larını iki taramada
işler. İki görünür listing'e karşı yalnız yüksek güvenli kaynaktan tek fake push,
12 şirketlik kapsama JSON'u ve ayrı kapalı Turkcell program penceresi beklenir.
Zaman damgalı sonuç `docs/acceptance/phase-15-2026-08-11.md` içindedir.

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

Faz 14 config testleri en uzun domain suffix'iyle politika çözümünü, duplicate
domain/mod/süre doğrulamasını ve `manual_only` kaynağın disabled + manual
adapter/strategy/tracking invariant'ını sınar. SQLite testi çözülmüş policy'nin
kalıcı kaynak alanlarına ve watchlist gerekçesine taşındığını; API wiring testi
manuel sosyal kaydın scraper oluşturmadan kaydedildiğini doğrular.

Faz 14 robots testleri yalnız fake HTTP transport ve yerel
`testdata/robots/phase14.txt` fixture'ını kullanır. Product-token gruplarının
birleşmesi, wildcard fallback, en uzun kural/eşitlikte allow, `*`/`$`, alan bazlı
24 saat cache ve 512 KiB sınırı kapsanır. 404 izinli; 403, 5xx, ağ hatası,
geçersiz hedef ve aşırı büyük gövde fail-closed beklenir. Orchestrator testleri
robots engelinde adapter'ın hiç çağrılmadığını ve nedenin kaydedildiğini;
`public_api` kaynağında checker'ın atlanıp fetch'in çalıştığını doğrular. API
wiring ve registry testleri en uzun suffix ile çözülen config politikasının
runtime `ProtectedSource` üzerindeki değer olduğunu sabitler. Normal testler
canlı kariyer sitesine gitmez.

Faz 14 uçtan uca fixture kabulü şu komutla ayrı çalıştırılır:

```bash
go test ./internal/acceptance -run TestPhase14 -count=1 -v
```

Test, production kaynak config'indeki Havelsan LinkedIn kaydını gerçek geçici
SQLite'a manuel-only olarak yazar; LinkedIn HTTP çağrısı oluşturmaz. İzinli ve
yasaklı iki yolu aynı robots fixture/cache'i üzerinden geçirir, yasaklı adapter'ın
çağrılmadığını ve anlık ikinci taramanın kalıcı domain minimum aralığında hiç
HTTP'ye ulaşmadığını doğrular. Zaman damgalı sonuç
`docs/acceptance/phase-14-2026-08-10.md` dosyasındadır.

Faz 12 birim testleri iki küçültülmüş DOM fixture'ı kullanır. İlk layout fake
learner ile reçete üretip store'a yazar; yeni bir source instance'ı aynı reçeteyi
model çağrısı olmadan çalıştırır. İkinci layout eski reçetenin kimlik/golden
guard'ını kırar, yeni reçetenin sürümünü artırır; geçersiz onarım sessiz sıfır
yerine kaynak hatasıdır. SQLite testleri iki sürüm geçmişi + tek aktif reçete,
atomik golden snapshot ve restart'ı aşan blok-hash cache'ini doğrular.

`internal/acceptance/phase12_test.go`, learned selector'ı gerçek migration,
SQLite repository, orchestrator, dedup ve analiz hattında üç taramayla sınar:
ilk öğrenme (2 yeni), yeni source instance'ı (0 yeni, 0 ek recipe çağrısı) ve
değişen layout onarımı (reçete v2, 1 yeni). Normal suite yalnız fake provider
kullanır.

Gerçek Gemini kabulü açıkça opt-in'dir ve yalnız yerel fixture sunucusunu dış
Google API ile birleştirir:

```bash
RUN_PHASE12_LIVE_ACCEPTANCE=1 \
GEMINI_API_KEY_FILE=/gizli/yol/gemini_api_key \
go test -tags=integration ./internal/acceptance \
  -run TestPhase12LiveGeminiLearnsPersistsAndRepairsRecipe -count=1 -v
```

Anahtar doğrudan `GEMINI_API_KEY` ile de verilebilir. Model varsayılan olarak
`gemini-3.1-flash-lite`; `PHASE12_LIVE_TEST_MODEL` ile değiştirilebilir. Test
gerçek kota kullanır, anahtarı/fixture gövdesini/SQLite dosyasını kaydetmez.

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

Faz 14.1 repository testi dashboard uygunluk kovalarına girmeyen analizli ve
analizsiz fırsatların tamamını sayfalı geçmişte tutar; yaşam döngüsü, şirket ve
metin filtrelerini, arşivin silme olmadığını ve geçersiz durumların reddini
doğrular. `internal/acceptance/phase141_test.go` aynı SQLite dosyasını kapatıp
yeniden açar, ardından tutarlı snapshot'tan restore eder; listing kimliği,
kanonik üyelik, analiz, lifecycle, başvuru durumu, iki tarih ve kullanıcı notunun
değişmediğini denetler. Aynı kabul scan başarı/409/500 yollarının geçerli JSON
ve doğru içerik türü taşıdığını doğrular. Frontend `api.test.ts`, JSON API
hatasını korurken HTML/plain-text proxy gövdesini ayrıştırmadığını veya
göstermediğini kapsar. `cmd/dbinspect` testleri read-only runtime kanıtının yalnız
DB yolu/metadata ve tablo sayıları içerdiğini doğrular.

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

Health testleri `GET /health` liveness yanıtının SQLite bağımlılığı olmadan
korunduğunu; `GET /ready` endpoint'inin fake başarılı/başarısız checker ile
`200`/ayrıntısız `503` döndürdüğünü doğrular. Database paketi ayrıca geçici,
gerçek SQLite üzerinde ping ve tüm migration kayıtlarını doğrular; bir migration
kaydı silindiğinde readiness başarısız olur. HTTP middleware testi güvenlik
başlıklarını ve logdaki HTTP durum kodunu kapsar. Bu testler ağ çağrısı yapmaz.

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

Faz 5 frontend helper testleri VAPID public key'in `Uint8Array` dönüşümünü,
izin yalnız kullanıcı eyleminde istendiğinde subscribe olmayı, browser
subscription JSON'unun PUT ile saklanmasını, backend hatasında yeni aboneliğin
geri alınmasını, DELETE + unsubscribe sırasını ve `?listing=<id>` URL
ekleme/temizleme davranışını mock runtime/fetch ile doğrular. Service worker
ayrıca `node --check` ile sözdizimi kontrolünden geçer; gerçek push servisi veya
tarayıcı notification ağı normal testlerde kullanılmaz.

Faz 13 eşleyici testleri Türkçe başlık normalizasyonunu, staj/intern
varyantlarını, `0.92` otomatik birleşme ve `0.80` belirsizlik sınırlarını, eksik
lokasyonun belirsiz kalmasını ve çelişen lokasyonun birleşmeyi engellemesini
tablo güdümlü ve ağsız olarak doğrular.

SQLite Faz 13 testleri iki kayıtlı fake kaynakta farklı URL'li aynı ilanların
tek fırsata bağlanmasını, eksik lokasyonun ayrı ve audit edilebilir kalmasını,
sonraki analizdeki lokasyon çelişkisinin veri silmeden split üretmesini ve eski
tekil backfill üyeliklerinin startup reconciliation ile birleşmesini sınar.
Dashboard/outbox testi aynı fırsatın yalnız bir kart, fırsat düzeyli tek
`dedup_key` ve abonelik başına tek delivery ürettiğini; domain testi ise payload
deep-link'inin kaynak listing'e çözülebilir kalırken Web Push `Topic` sınırına
uyduğunu doğrular.

`internal/acceptance/phase13_test.go`, iki kaynak gözlemini küçük JSON fixture'dan
okur; fake kaynaklar ve fake analizciyi gerçek orchestrator, migration/SQLite,
üretim dashboard HTTP handler'ı, outbox ve fake push göndericiyle birleştirir.
Normal kabul ağı kullanmaz. Ana senaryo iki listing/tek fırsat/tek kart/tek push;
eksik lokasyon senaryosu iki ayrı fırsat bekler.

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

GitHub Actions her push ve pull request'te Go format kontrolü, `go vet`,
`govulncheck`, backend test/build, frontend test/build, yüksek önem seviyesindeki `npm audit` bulguları
ve iki production Docker image'ının yayınlamayan build adımlarını çalıştırır.
Production deployment sözleşmesi ayrıca seçilen LLM secret dosyası yolunun
render edilmiş Compose config'ine taşınmasını ve `DEPLOY_REVISION` ile eşleşen
exact-commit deployment paketinin yalnız izin verilen dosyaları içermesini
denetler. Ardından ağ veya Docker daemon kullanmayan sahte `docker`/`curl`
fixture'larıyla mevcut sürümden alınan backup binary entrypoint'ini, public
origin eşleşmesini ve secret taşımadan `current.env`/`previous.env` manifest
geçişini çalıştırır.
Gerçek API anahtarı ve ücretli entegrasyon çağrısı CI testlerine dahil edilmez.
Compose healthcheck komutları runtime Dockerfile'larında açıkça kurulu `wget`
kullanır; `/ready`, DB/migration readiness'ini kontrol ederken `/health` yalnız
liveness için korunur.
