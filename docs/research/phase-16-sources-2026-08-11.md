# Faz 16 kaynak doğrulaması — 11 Ağustos 2026

Bu kayıt, Faz 16 kataloğunun ticari kimlik, resmî kaynak ve erişim kararlarını
kanıtlar. Kontroller salt-okunur yapıldı; hesap/oturum/CAPTCHA yolu kullanılmadı
ve normal testlere canlı ağ çağrısı eklenmedi.

| Kanonik şirket | Doğrulanan resmî kaynak | Erişim / kapsama | Karar |
| --- | --- | --- | --- |
| İnnova | https://www.innova.com.tr/is-ilanlari | `manual_only` / manuel | İlanlar LinkedIn'e yönleniyor; `İnova` resmî yazıma alias'tır. |
| İntertech | https://www.intertech.com.tr/career.html | `manual_only` / manuel | Resmî sayfa başvuruları LinkedIn üzerinden alıyor. |
| Sebit | https://sebitkariyerim.sebit.com.tr/#/home/ | `manual_only` / manuel | Kullanıcı `ÜşüSebit` girdisini Sebit olarak onayladı; açık ATS/API doğrulanmadı. |
| DenizBank | https://www.denizbank.com/yardim-merkezi/insan-kaynaklari | `manual_only` / manuel | Resmî staj programları görünür; aday portalı otomasyona alınmadı. |
| Mobiliz | https://mobiliz.com.tr/mobiliz-hakkinda/ | `manual_only` / araştırılıyor | Şirket ve aday veri metinleri doğrulandı; açık ilan akışı bulunamadı. |
| AI Studio | https://aistudio.com.tr/aistudio-nedir | `manual_only` / araştırılıyor | Kullanıcı bu kimliği onayladı; açık kariyer kaynağı bulunamadı. |
| Belsis | https://www.belsis.com.tr/Sayfa/Index/Kurumsal/Firmamiz_Hakkinda | `manual_only` / araştırılıyor | Resmî yazılım şirketi doğrulandı; açık ilan akışı bulunamadı. |
| Evreka | https://evreka.co/career/ | `robots` / otomatik | Kariyer yolu robots tarafından engellenmiyor; nested `/career/` ilanları izleniyor. |
| Viseur AI | https://viseur.ai/ | `manual_only` / araştırılıyor | Resmî medikal AI kimliği doğrulandı; güncel erişilebilir kariyer akışı yok. |
| MechSoft | https://www.mechsoft.com.tr/jobs | `robots` / otomatik | Açık `/jobs/detail/` ilanları izleniyor; `MechSoft AI` resmî markaya alias'tır. |
| Layermark | https://www.layermark.com/careers/ | `robots` / otomatik | `open-positions` bölümündeki aynı-origin ilanlar izleniyor. |
| Actioner | https://actioner.com/contact | `manual_only` / araştırılıyor | Resmî alan adı doğrulandı; açık kariyer kaynağı bulunamadı. |
| Bilishim | https://bilishim.ai/ | `manual_only` / araştırılıyor | Kullanıcı Bilishim Cyber Security & AI kimliğini onayladı; açık kariyer akışı yok. |
| Otsimo | https://otsimo.com/en/careers/ | `manual_only` / manuel | Resmî sayfa Indeed'e yönlendiriyor; toplayıcı otomatik taranmıyor. |

Otomatik kaynaklarda minimum istek aralığı 24 saattir. Manuel ve araştırılıyor
kaynaklar katalog/watchlist görünürlüğü sağlar fakat otomatik kapsama başarısı
sayılmaz. Sınıflandırmalar yeni resmî ATS/feed keşfedildiğinde kanıt ve fixture
ile güncellenebilir.
