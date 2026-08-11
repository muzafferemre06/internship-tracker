# Faz 18 şirket durum tespiti — 11 Ağustos 2026

Faz 17'de seçilen 15 şirketin tamamı yeniden incelendi. İnceleme şirketin bugünkü
kimliğini, resmî fırsat kaynağını, öğrenci/staj sinyalini, erişim biçimini ve
güvenli otomasyon kararını birbirinden ayırır. Ayrıntılı ve makinece doğrulanan
kayıt [`phase-18-company-diligence-2026-08-11.json`](phase-18-company-diligence-2026-08-11.json)
dosyasındadır.

## Özet karar

| Karar | Şirketler | Sayı |
| --- | --- | ---: |
| İlk batch'te resmî ATS otomasyonu | MobileAction | 1 |
| İlk batch'te dürüst manuel kaynak | SİMSOFT, Netaş, Bilişim AŞ | 3 |
| Sonraki batch için güçlü otomasyon adayı | Binalyze, Insider | 2 |
| Sonraki batch için fixture/erişim araştırması | Etiya, OBSS, T2 Software, TaleWorlds, LOTEC, Udemy | 6 |
| Sonraki batch için yalnız manuel/genel başvuru adayı | Ankara Bilgi Teknolojileri, Peaksoft Consulting | 2 |
| Eski kimlik; bu adla eklenmemeli | Alictus | 1 |

## Şirket bazında bulgular

- **MobileAction:** Resmî kariyer sayfası açık `jobs.lever.co/mobile-action`
  panosuna bağlanıyor. Kanonik ilan URL'leri otomasyona uygun. Güncel panoda staj
  yok; kıdemli ve genel tam zamanlı roller öğrenci bildirimi üretmemeli.
- **SİMSOFT:** Aday mühendis/stajyer formu güncel ve öğrenci sinyali çok güçlü.
  Form CAPTCHA ve CV yükleme içerdiği için yalnız manuel bağlantı olarak
  gösterilmeli; sistem form göndermez.
- **Netaş:** COOP sayfası 3./4. sınıf mühendislik öğrencileri için güz, bahar ve
  yaz dönemlerini açıklıyor. Tarihli açık başvuru penceresi olmadığı için program
  `unknown` durumuyla manuel/periyodik izlenmeli.
- **Bilişim AŞ:** Resmî sayfa staj kabulü, proje ataması ve olası yarı zamanlı
  devamı açıklıyor; ilan listesi veya form yok. Süreç sayfası manuel kaynak olur.
- **Etiya:** Kariyer ve açık pozisyon sayfaları güncel, fakat görünür roller
  kıdemli ve Etiya Academy mezun odaklı. Öğrenci ilanı ile kararlı adapter henüz
  doğrulanmadı.
- **OBSS:** Resmî kurumsal sunum geçmiş geniş staj programını doğruluyor. Eski
  program kanıtı güncel açık pencere sayılmadığından canlı ilan kaynağı ayrıca
  bulunmalı.
- **T2 Software:** Resmî sayfada yazılım rolleri var; ayrıntılar ayrı URL yerine
  sayfa içi açılıyor ve daha junior rol de deneyim istiyor. Kimlik/dedup tasarımı
  fixture gerektiriyor.
- **Binalyze:** Kariyer akışı açık Ashby panosuna çıkıyor; marka web varlığı
  `binalyze.ai` yönüne taşınıyor. Domain geçişiyle birlikte sonraki batch'te ele
  alınabilecek güçlü otomasyon adayı.
- **TaleWorlds:** Resmî kariyer metni ilan dışı staj başvurusunu açıkça kabul
  ediyor, fakat doğrudan istek 403 döndürüyor. Engel aşılmadan manuel kalmalı.
- **Insider:** Güncel marka/ATS anahtarı `Insider One` / `insiderone`. Açık Lever
  panosunda Türkiye rolleri ve küresel stajlar var. Ankara/öğrenci filtreleriyle
  sonraki batch için güçlü aday.
- **Alictus:** Güncel profil, Alictus'un artık SciPlay olduğunu söylüyor. Eski
  şirket anahtarı eklenirse aynı organizasyon iki kez izlenir; SciPlay Türkiye
  daha sonra yeni aday olarak araştırılmalı.
- **LOTEC:** Resmî kariyer sayfası arama dizininde teknik roller gösteriyor,
  doğrudan erişimde bot challenge var. Challenge aşılmadan otomasyon açılmaz.
- **Udemy:** Resmî kariyer sayfası Ankara'yı listeliyor ve geçmiş staj programı
  kanıtı var. Güncel işler istemci tarafı bileşenden geldiği için sağlayıcı ve
  Ankara filtresi fixture ile doğrulanmalı.
- **Ankara Bilgi Teknolojileri:** Şirket ve teknokent kimliği gerçek; geçmiş staj
  ve yakın dönem e-posta başvuru izi var. Birinci taraf kariyer akışı olmadığı
  için üçüncü taraf ilan otomatik yüksek güven kaynağı yapılmamalı.
- **Peaksoft Consulting:** Resmî ana sayfada yalnız e-posta ile CV gönderilen
  genel kariyer bölümü var. Öğrenciye özel sinyal ve ilan listesi yok; en fazla
  manuel genel başvuru kaynağı olabilir.

## İlk batch erişim politikası

| Şirket | Kaynak | Davranış |
| --- | --- | --- |
| MobileAction | Resmî Lever panosu | `jobs.lever.co` alanında en az 1 saniye aralıkla otomatik GET |
| SİMSOFT | Resmî aday mühendis/stajyer formu | `manual_only`; CAPTCHA/form gönderimi yok |
| Netaş | Resmî COOP program sayfası | `manual_only`; tarihsiz pencere `unknown` |
| Bilişim AŞ | Resmî staj süreci sayfası | `manual_only`; süreç metninden sahte ilan yok |

## Sınır

Bu batch yalnız onaylanan dört şirketi production kaynak kataloğuna taşır.
Diğer 11 şirket hakkındaki araştırma tamamlanmış olsa da onaylı batch dışında
config, tarama veya bildirim akışına eklenmez. Başvuru formları gönderilmez,
bot challenge aşılmaz ve üçüncü taraf ilan resmî kanıt yerine kullanılmaz.

## Sonraki batch uygulama sonucu

Kullanıcının Faz 18'in kalanını uygulama onayından sonra resmî kaynaklar yeniden
doğrulandı. Binalyze/Ashby ve Insider One/Lever ikinci batch'te; Etiya'nın
sunucu taraflı PeopleBox bağlantılı tablosu ile Udemy sayfasının kullandığı
Greenhouse public API üçüncü batch'te otomatikleştirildi. OBSS yalnız LinkedIn,
T2 yalnız mailto sunduğu için manuel; TaleWorlds güvenli istemciye HTTP 403
döndürdüğü için manuel; LOTEC HTTP 500/koruma verdiği için araştırılıyor kaldı.
Udemy sayfası Coursera ve Udemy'nin artık tek şirket olduğunu bildirirken
resmî `udemy` Greenhouse board'u çalışmaya devam etmektedir; kaynak bu kanonik
board kimliğiyle tutulur. Hiçbir LinkedIn/apply/e-posta hedefi fetch edilmez veya
otomatik gönderilmez, challenge aşılmaz.

Son katalog doğrulamasında `ankarabt.com` ve `peaksoftcon.com` HTTP 200 verdi
ancak kararlı ilan/ATS akışı sunmadı; ikisi manuel resmî şirket kaynağı olarak
eklendi. `alictus.com` doğrudan `sciplaygamesturkey.com` alanına yönlendi ve
sayfa “Alictus is now SciPlay Games Turkey” kimliğini gösterdi. Bu nedenle eski
Alictus adı production'a eklenmedi; SciPlay ayrı kanonik aday olarak ileride
yeniden araştırılmalıdır.
