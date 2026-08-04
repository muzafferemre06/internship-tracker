# Uygulama durumu

## Aktif faz

Faz 3.5 tamamlandı; Faz 4 devam ediyor. Resmî Commencis Lever ilanı zaman
damgalı aktiflik kanıtıyla standart ingestion, canlı Google Gemini analizi,
SQLite kullanım kalıcılığı, dashboard karar kuyruğu ve ikinci tarama dedup
kapılarından geçti.

## Tamamlananlar

- Ürün ve teknik kararları içeren v2 spec
- Go API ve React/Vite PWA iskeleti
- Aday profili ve şirket kaynak yapılandırması için doğrulanan yükleyiciler
- Transaction kullanan, tekrar uygulanmayan SQLite migration çalıştırıcısı
- Meteksan profilinden ilan bağlantılarını normalize eden kariyer.net adapter'ı
- ASELSAN/ASELSANNET çoklu profili ile STM, Baykar ve Samsung kaynak ayarları
- Canonical URL, kararlı kimlik ve duplicate kontrolü yapan SQLite repository
- Profil alanlarıyla temel uygunluk üreten deterministik ilan analizcisi
- Gerçek bağımlılıklarla çalışan manuel scan API'si ve SQLite dashboard sorgusu
- Kalıcı `completed`/`partial`/`failed` scan-run raporu ve kaynak sağlık durumu
- Kalıcı domain erişim bütçesi, 24 saat alt sınır ve 403/429 challenge devre kesicisi
- PWA'da son taramanın başarılı ve başarısız kaynak sayıları
- Kilit dosyasıyla tekrarlanabilir frontend kurulumu ve sıfır npm audit bulgusu
- Test, CI, Docker ve secret yönetimi başlangıç dosyaları
- Ayarlardan seçilen deterministik/OpenRouter/Google analiz sağlayıcısı ve model
- Minimize aday profili, strict JSON Schema ve backend çıktı doğrulaması
- Sınırlı model retry'sı ile pending analizlerin kaynak bağımsız yeniden işlenmesi
- Provider/model, token kullanımı ve ayarlanabilir oranlarla tahmini maliyet kalıcılığı
- İlan detayını ve başvuru takibini sunan HTTP/repository sözleşmesi
- Durum, manuel tarih, mülakat zamanı ve notlar için doğrulanan SQLite upsert akışı
- Kaynak hatalarından üretilen manuel kontrol listesi

## Doğrulanan çıkış kriterleri

- Backend testleri, `go vet` ve production build geçer.
- Frontend testleri, typecheck, production build ve `npm audit` geçer.
- API örnek config ve geçici SQLite ile açılır; health/dashboard yanıt verir.
- İlk fixture taraması iki yeni ilan, ikinci tarama sıfır yeni ilan üretir.
- Tanınmayan bir kaynak sayfası hata verirken diğer iki profil tamamlanır.
- İki ASELSAN profilindeki ortak ilan tek kayıt olur; rapor `partial` kapanır.
- 403, 418, 429, 5xx, timeout ve selector bozulması testleri geçer.
- İlk koruma hatasından sonra aynı domaindeki diğer profillere istek gönderilmez.
- Fake provider/HTTP cevaplarıyla uygun, uygun değil, karar bekliyor, bozuk JSON,
  şema hatası, timeout, kalıcı 4xx, 429, 5xx ve retry sınırı geçer.
- Başarısız analiz ham ilanı kaybetmez; dashboard karar kuyruğunda kalır ve
  kaynak fetch'i olmadan başarılı biçimde yeniden işlenir.
- Başarılı analizde provider, model, token kullanımı ve tahmini maliyet saklanır.
- Google Gemini adapter'ının header/schema/usage/hata davranışı fake transport ile,
  tam şema akışı ise opt-in canlı integration testiyle doğrulanabilir.
- Resmî tek ilan sayfasını güvenli alanlarla normalize eden Lever adapter'ı;
  aktif başvuru bağlantısı ve beklenen sayfa yapısı için fixture testleriyle doğrulanır.
- Fixture Lever kaynağı, strict fake model, geçici SQLite ve dashboard HTTP
  handler'ını iki taramada birleştiren Faz 3.5 normal kabul testi tamamlandı.
- Resmî Commencis Lever ilanı canlı Google `gemini-3.1-flash-lite` ile işlendi;
  zaman damgalı güvenli kanıt `docs/acceptance/phase-3.5-2026-08-03.md` içinde.

## Sıradaki iş

Faz 4 sade PWA ve başvuru takibidir. Backend sözleşmesi tamamlandı; responsive
dashboard ve telefon kabul akışı doğrulanacaktır.
