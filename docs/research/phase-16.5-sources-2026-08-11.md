# Faz 16.5 kaynak araştırması — 11 Ağustos 2026

Bu kayıt, ayrı izleme bölümüne alınan on bir şirket için resmî kariyer sayfası,
ATS, robots, sitemap, RSS/Atom, yapılandırılmış ilan verisi ve açık API
kontrollerinin sonucudur. Hesap/CAPTCHA aşılmadı; LinkedIn, Indeed ve Youthall
taranmadı; özel API tersine mühendisliği yapılmadı.

| Şirket | Resmî/manual bağlantı | Sonuç | Takip kararı |
| --- | --- | --- | --- |
| İnnova | https://www.innova.com.tr/is-ilanlari | Sayfa ilan başlığı, konum/seviye ve LinkedIn başvuru hedefini açık HTML kartlarında sunuyor; robots `/` yoluna izin veriyor. | `automatic`: resmî kartlar fixture-first `career_links` ile okunur; LinkedIn hedefi saklanır fakat fetch edilmez. |
| İntertech | https://www.intertech.com.tr/career.html | Açık pozisyonlar LinkedIn'e; StartTech, InternTech ve FirstTech programları Youthall'a yönleniyor. | `third_party_restricted`: resmî sayfa manuel izlenir. |
| Sebit | https://sebitkariyerim.sebit.com.tr/#/home/ | Resmî Sebit sayfasının bağladığı PozitifIK portalı istemci-render SPA'dır; doğrulanmış açık ATS/API veya gerçek robots politikası bulunmadı. | `client_rendered_unverified`: portal manuel izlenir. |
| DenizBank | https://www.denizbank.com/yardim-merkezi/insan-kaynaklari | Resmî sayfa dönemsel programları tanımlar; aday yolu ayrı hesap/başvuru akışıdır. | `account_required`: program sayfası manuel izlenir. |
| Otsimo | https://otsimo.com/en/careers/ | Resmî kariyer sayfasındaki “Join our team” hedefi Indeed'dir; ayrı resmî ilan indeksi/ATS bulunmadı. | `third_party_restricted`: resmî sayfa manuel izlenir. |
| Mobiliz | https://mobiliz.com.tr/mobiliz-hakkinda/ | Robots ve WordPress sitemap erişilebilir; çalışan/stajyer aday metinleri var fakat kariyer, ilan, ATS veya başvuru sayfası yok. | `no_public_job_source`: resmî şirket sayfası manuel referanstır. |
| AI Studio | https://aistudio.com.tr/aistudio-nedir | Robots ve sitemap açık; kariyer/job/intern URL'si, ATS, RSS/Atom veya ilan verisi bulunmadı. | `no_public_job_source`: resmî şirket sayfası manuel referanstır. |
| Belsis | https://www.belsis.com.tr/Sayfa/Index/Kurumsal/Firmamiz_Hakkinda | Resmî kimlik ve iletişim doğrulandı; kariyer akışı bulunmadı. Ayrıca TLS zinciri standart doğrulayıcı istemciyle başarısız oluyor. | `source_unreachable`: TLS doğrulaması kapatılmadan manuel izlenir. |
| Viseur AI | https://viseur.ai/ | Robots izinli ve sitemap `/careers/` adresini listeliyor; adresin güncel yanıtı 404, sitede başka ilan/ATS akışı yok. | `no_public_job_source`: ana resmî sayfa manuel referanstır. |
| Actioner | https://actioner.com/contact | Robots/sitemap açık. `/build-with-us` ve `/build-with-us-v1` işe alım değil, Actioner müşterilerine danışmanlık/uygulama hizmeti sayfalarıdır. | `no_public_job_source`: resmî iletişim sayfası manuel izlenir. |
| Bilishim | https://bilishim.ai/ | Robots erişime izin veriyor; kariyer/ATS bulunmadı. Sitemap farklı ve yazım hatalı bir alana yönleniyor. | `no_public_job_source`: resmî ana sayfa manuel referanstır. |

İş önceliği değiştirilmedi: tüm kayıtlar `secondary` kalır. Ayrı sunum ve takip
kohortu `tracking_phase: "16.5"` alanıyla tanımlanır. Araştırma bulguları kaynak
config'inde ayrıntılı gerekçe, makinece okunur neden kodu ve
`2026-08-11T00:00:00+03:00` doğrulama zamanı olarak kalıcılaştırılmıştır.
