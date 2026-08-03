# Uygulama durumu

## Aktif faz

Faz 0 tamamlanıyor. Repo ve uygulama iskeleti hazırdır; yapılandırma dosyaları
doğrulanır ve SQLite veritabanı başlangıçta migration kayıtları izlenerek
hazırlanır. Faz 1'in fixture tabanlı Meteksan/kariyer.net adapter'ı hazırdır.

## Tamamlananlar

- Ürün ve teknik kararları içeren v2 spec
- Go API ve React/Vite PWA iskeleti
- Aday profili ve şirket kaynak yapılandırması için doğrulanan yükleyiciler
- Transaction kullanan, tekrar uygulanmayan SQLite migration çalıştırıcısı
- Meteksan profilinden ilan bağlantılarını normalize eden kariyer.net adapter'ı
- Canonical URL, kararlı kimlik ve duplicate kontrolü yapan SQLite repository
- Test, CI, Docker ve secret yönetimi başlangıç dosyaları

## Sıradaki iş

Deterministik analizciyi ekleyip tarama akışını gerçek bağımlılıklarla kurmak.
