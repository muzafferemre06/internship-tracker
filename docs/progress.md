# Uygulama durumu

## Aktif faz

Faz 1 tamamlandı. Meteksan fixture'ı scraper, deterministik analizci ve gerçek
SQLite repository üzerinden uçtan uca işlenir. İlk tarama kayıtları yeni sayar;
aynı fixture'ın ikinci taraması yeni kayıt üretmez. Manuel scan API'si ve
dashboard kalıcı veriyi kullanır.

## Tamamlananlar

- Ürün ve teknik kararları içeren v2 spec
- Go API ve React/Vite PWA iskeleti
- Aday profili ve şirket kaynak yapılandırması için doğrulanan yükleyiciler
- Transaction kullanan, tekrar uygulanmayan SQLite migration çalıştırıcısı
- Meteksan profilinden ilan bağlantılarını normalize eden kariyer.net adapter'ı
- Canonical URL, kararlı kimlik ve duplicate kontrolü yapan SQLite repository
- Profil alanlarıyla temel uygunluk üreten deterministik ilan analizcisi
- Gerçek bağımlılıklarla çalışan manuel scan API'si ve SQLite dashboard sorgusu
- Kilit dosyasıyla tekrarlanabilir frontend kurulumu ve sıfır npm audit bulgusu
- Test, CI, Docker ve secret yönetimi başlangıç dosyaları

## Doğrulanan çıkış kriterleri

- Backend testleri, `go vet` ve production build geçer.
- Frontend testleri, typecheck, production build ve `npm audit` geçer.
- API örnek config ve geçici SQLite ile açılır; health/dashboard yanıt verir.
- İlk fixture taraması iki yeni ilan, ikinci tarama sıfır yeni ilan üretir.

## Sıradaki iş

Faz 2'de generic kariyer.net adapter'ını çoklu profil URL desteğiyle ASELSAN,
STM, Baykar ve Samsung kaynaklarına genişletmek; scan run raporunu kalıcılaştırmak.
