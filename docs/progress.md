# Uygulama durumu

## Aktif faz

Faz 3 tamamlandı. Sağlayıcıdan bağımsız model analiz katmanı ve OpenRouter
adapter'ı eklendi. Model çıktısı katı doğrulanır, geçici/bozuk cevaplar sınırlı
retry alır ve başarısız analizler yeniden işlenebilir karar kuyruğunda saklanır.
Doğrudan Google Gemini adapter'ı ve opt-in canlı model testi de tamamlandı.
Sıradaki teslimat Faz 3.5 gerçek ilan kabul doğrulamasıdır; Faz 4 henüz
başlamamıştır.

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

## Sıradaki iş

Faz 3.5'te resmî bir kaynakta açık olduğu zaman damgasıyla doğrulanan en az bir
gerçek staj ilanı uygun adapter üzerinden alınacak. İlan canlı Gemini analizi,
SQLite kalıcılığı, dashboard görünürlüğü ve ikinci çalıştırmada dedup açısından
uçtan uca doğrulanacak. Canlı kabul testi opt-in kalacak; normal suite aynı akışı
fixture ve fake provider ile çalıştıracak. Bu kapı tamamlanmadan Faz 4'e
başlanmayacak.
