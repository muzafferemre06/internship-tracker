# Staj Takip ve Başvuru Asistanı — Ürün/teknik spec (v2)

## 1. Belgenin amacı

Bu belge, ilk spec sonrasında yapılan ürün görüşmesindeki kararları toplar.
Projenin kısa vadeli amacı, seçili şirketlerin staj başvurularını güvenilir
biçimde takip edip kullanıcıya PWA üzerinden bildirmektir.

Uzun vadede sistem; ilan keşfi, basit başvuru takibi, hatırlatıcılar, şirket
keşfi ve başvuru süreçleri hakkında bilgi sunan kişisel bir kariyer asistanına
dönüşebilir. İlk sürüm bu uzun vadeli hedeflerin tamamını gerçekleştirmeye
çalışmayacaktır.

## 2. Kullanıcı profili

Başlangıç profili:

- Bilkent Üniversitesi CTIS öğrencisi
- 2026-2027 döneminde 2. sınıfa geçiyor
- GPA: 3.97
- ODTÜ Altek Teknofest Savaşan İHA takımında görev alıyor
- Deneyim odağı: otonom yazılım ve YKİ
- Kariyer odağı:
    - backend
    - network
    - sistem yönetimi
    - otonom yazılım/YKİ

Profil, kullanıcı tarafından hazırlanacak sürüm kontrollü bir aday profili
dosyasından güncellenecektir. Eğitim, GPA, deneyim, proje ve beceriler bu dosya
üzerinden zamanla genişletilebilir. CV ayrı tutulabilir ve aday profilinden
yararlanabilir.

LLM'e eğitim, deneyim, proje, beceri ve tercih bilgileri gönderilebilir. Ad,
telefon, e-posta, açık adres ve diğer doğrudan iletişim/kimlik bilgileri model
isteklerinden ve uygulama loglarından çıkarılmalıdır.

İlk sürüm tek kullanıcı içindir. Veri modeli ileride çok kullanıcılı yapıya
geçişi engellememeli; ancak MVP'de hesap yönetimi ve karmaşık yetkilendirme
geliştirilmeyecektir.

## 3. Temel kullanıcı ihtiyacı

Sistem, kullanıcının sürekli kariyer sayfası kontrol etme mental yükünü
azaltmalıdır. Yeni bir staj başvurusu açıldığında yönlendirici bir bildirim
vermelidir:

- hangi şirketin başvuru açtığı
- fırsatın türü
- önemli başvuru şartları
- son başvuru tarihi (varsa)
- kullanıcıya yaklaşık uygunluğu
- doğrudan incelenecek/başvurulacak adres

İlk sürüm otomatik başvuru yapmaz; form doldurmaz, kullanıcı adına cevap
üretmez veya başvuruyu göndermez.

## 4. Takip edilecek fırsatlar

### 4.1 Fırsat türleri

- Staj
- Uzun dönem staj
- Part-time pozisyon
- Yeni mezun pozisyonu

MVP'nin birincil sinyali, bir şirketin staj başvurusunu açmış olmasıdır.
Diğer fırsat türleri bulunabilir ve sınıflandırılabilir; ancak ilk bildirim
akışını karmaşıklaştırmamalıdır.

### 4.2 Rol uygunluğu

Öncelikli alanlar:

- Backend
- Network
- Sistem yönetimi
- Otonom yazılım/YKİ

İlk sürümde bu alanların dışında kişiselleştirilmiş ek kariyer alanları
tanımlanmayacaktır. Alanlar daha sonra ayarlardan genişletilebilir.

### 4.3 Eğitim şartı ve uygunluk

Bir ilan 3. veya 4. sınıf şartı koyduğu için tamamen gizlenmemelidir. İlanlar
şu seviyelerden biriyle gösterilmelidir:

- `uygun`
- `kismen_uygun`
- `uygun_degil`
- `karar_bekliyor`

Sınıf şartı karşılanmayan fakat diğer yönleriyle değerli ilanlar
`kismen_uygun` olarak görünmelidir. AI karar veremediğinde ilan elenmemeli,
`karar_bekliyor` listesine alınmalı ve kullanıcıya kısa bir soru sorulmalıdır.
Kullanıcı cevaplarından kalıcı tercih öğrenme sonraki aşama özelliğidir.

### 4.4 Lokasyon uygunluğu

- Ankara veya tamamen uzaktan: öncelikli öneri
- Yaz döneminde şehir dışı yüz yüze/hibrit: değerlendirilebilir
- Dönem içinde Ankara dışı part-time/hibrit: uygun değil; bilgi amaçlı
  gösterilebilir, öncelikli öneri olarak sunulmaz
- Ankara dışındaki diğer fırsatlar tamamen gizlenmez

## 5. Şirket kapsamı

### 5.1 Birincil şirket grubu

İlk mesajda `++++` ayırıcısından önce belirtilen şirketler birincil gruptur:

- Havelsan
- Akdoğan Tech (`https://akdogan.tech/`)
- Meteksan
- Baykar
- Turkcell
- Türk Telekom
- Jotform
- Samsung
- Commensis
- Akınsoft
- Roketsan
- ASELSAN

### 5.2 İkincil şirket grubu

- İnova
- İntertech
- ÜşüSebit (şirket adı/adresi doğrulanacak)
- DenizBank
- Mobiliz
- AI Studio (tam şirket adı/adresi doğrulanacak)
- Belsis
- Evreka
- Viseur AI
- MechSoft AI
- Layermark
- Actioner
- Bilishim (tam şirket adı/adresi doğrulanacak)
- Otsimo

Akınsoft ilk listede de bulunduğu için birincil grupta tek kayıt olarak
tutulacaktır.

### 5.3 İlk teknik doğrulama kaynakları

Önce ortak scraper altyapısını doğrulamak için mevcut ilk spec'teki kolay
kaynaklar kullanılacaktır:

1. Meteksan ile tek kaynaklı dikey dilim
2. Aynı generic kariyer.net scraper'ıyla ASELSAN, STM, Baykar ve Samsung
3. Ardından özel/öncelikli kaynaklar: Akdoğan Tech, Turkcell ve Havelsan

Bu sıra ürün önceliğini değiştirmez; teknik riski erken azaltmak içindir.

### 5.4 Teknokent şirket keşfi

Gelecekte şu bölgelerdeki yazılım şirketleri araştırılacaktır:

- Bilkent Cyberpark
- ODTÜ Teknokent
- Hacettepe Teknokent

Keşfedilen şirketler iki grupta tutulur:

- `aday`: otomatik keşfedilen, düşük olasılıklı ve düşük öncelikle izlenen
- `onaylanmis`: kullanıcıya sunulup onaylanan, normal veya yüksek önceliğe
  alınan

Bir şirketin önerilmeye değer olup olmadığı şu sinyallerle değerlendirilir:

- faaliyet alanının kullanıcı profiline uygunluğu
- kullanılan teknolojiler
- geçmişte stajyer veya ilgili pozisyon açmış olması
- şirketin güvenilirliği ve kurumsallığı

Ekip büyüklüğü bir eleme ölçütü değildir. Ölçütler ve ağırlıkları gelecekte
uyarlanabilir olmalıdır.

## 6. Kaynak önceliği ve erişilemeyen siteler

Kaynaklar iki aşamada ele alınır:

1. Resmî şirket kariyer, iş ilanı ve duyuru sayfaları
2. Resmî kaynak takibi güvenilir çalıştıktan sonra LinkedIn, kariyer
   platformları ve sosyal medya hesapları

Bot koruması, giriş duvarı veya CAPTCHA nedeniyle otomatik taranamayan bir
kaynak için korumayı aşmaya çalışmak MVP hedefi değildir. Bu kaynaklar manuel
kontrol listesine alınır. Sistem kullanıcıya şunları gösterebilir:

- kontrol edilmesi gereken site veya sosyal medya hesabı
- son kontrol zamanı
- tekrar kontrol zamanı
- doğrudan bağlantı

Bir kaynağın hatası diğer şirketlerin taranmasını durdurmamalıdır.

## 7. Ürün akışları

### 7.1 İlan keşfi

1. Zamanlanmış veya kullanıcı tarafından başlatılan tarama çalışır.
2. Aktif kaynaklar bağımsız olarak kontrol edilir.
3. İlanlar normalize edilir ve tekrar kontrolünden geçirilir.
4. Yeni veya anlamlı biçimde değişmiş ilan analiz edilir.
5. Sonuç veritabanına kaydedilir.
6. Bildirim kuralı uygulanır.
7. Tarama sonucu ve kaynak hataları dashboard'da gösterilir.

### 7.2 Başvuru takibi

Kullanıcı bir ilanı manuel olarak şu durumlardan birine alabilir:

- `incelenecek`
- `basvuruldu`
- `sinav_mulakat`
- `olumlu`
- `olumsuz`

Durum listesi ileride genişletilebilir. Otomatik başvuru ve form doldurma
MVP kapsamında değildir.

### 7.3 Hatırlatıcılar

- Son başvuru tarihi yaklaşırken artan sıklıkta hatırlatma
- Sınav/mülakat tarihi yaklaşırken artan sıklıkta hatırlatma
- Tarih otomatik çıkarılamazsa manuel tarih girişi
- Hatırlatmayı erteleme veya tamamlandı olarak kapatma

Kesin hatırlatma eşikleri kullanıcı geri bildirimiyle ayarlanacaktır.

### 7.4 Şirket başvuru süreci özeti

Şirket profilleri zamanla şu bilgileri gösterebilir:

- başvuru dönemleri ve tarihler
- süreç aşamaları
- teknik sınavlar
- video ve İK mülakatları
- geçmiş aday deneyimleri
- hazırlanılabilecek konu başlıkları

Resmî bilgi ile kullanıcı deneyimi birbirinden ayrılmalı; mümkün olduğunda
kaynak ve güncellik tarihi gösterilmelidir. Bu zengin araştırma akışı ilk
bildirim MVP'sinden sonra geliştirilebilir.

## 8. Bildirim modeli

Ana bildirim kanalı PWA push bildirimidir. Telegram MVP kapsamında değildir.

- Birincil şirketlerdeki yeni ve uygun staj fırsatları için ayrı/anlık
  bildirim
- Diğer şirketler ve daha düşük uygunluklu ilanlar için toplu günlük özet
- Belirsiz kararlar için `karar_bekliyor` göstergesi
- Aynı ilan için aynı olay nedeniyle tekrar bildirim göndermeme
- Bildirime dokununca ilgili ilanın bulunduğu PWA dashboard'unu açma

İlk yerel geliştirme diliminde tarama PWA'dan elle başlatılabilir. Gerçek push
ve zamanlanmış tarama için erişilebilir bir backend gerekir. Scraper ve
OpenRouter anahtarı tarayıcı/PWA içine konulmamalıdır.

## 9. Tarama takvimi

Başlangıç varsayımı:

- 1 Mart 2027'ye kadar haftada bir otomatik tarama
- 1 Mart 2027'den sonra her 3-4 günde bir otomatik tarama
- Kullanıcı istediğinde manuel tarama
- Şirket veya kaynak bazında farklı takvim tanımlayabilme

Takvim kod değişikliği gerektirmeden yapılandırılabilir olmalıdır. İlan
dönemleri hakkında yeni veri edinildiğinde başlangıç tarihi ve sıklık
güncellenebilir.

## 10. Dashboard

İlk dashboard sıralaması:

1. Kritik bildirimler ve yaklaşan tarihler
2. Yeni ve uygun ilanlar
3. Karar bekleyen ilanlar
4. İncelenecek ilanlar
5. Aktif başvurular
6. Son tarama sonucu ve kontrol edilemeyen kaynaklar

Tasarım sade ve responsive olmalı; telefon ve bilgisayarda kullanılmalıdır.
Kullanım geri bildirimlerine göre sıralama değiştirilebilir.

## 11. Teknik mimari

### 11.1 Teknoloji seçimi

- Backend ve orchestrator: Go
- Veritabanı: SQLite
- PWA: TypeScript + React/Vite
- AI erişimi: sağlayıcıdan bağımsız arayüz, ilk sağlayıcı OpenRouter
- JS-render gerektiren kaynaklar: gerektiğinde Playwright tabanlı yardımcı
  katman
- Paketleme: Docker
- CI/CD: GitHub Actions

Go bu proje için aynı zamanda backend ve DevOps öğrenme hedefini destekler.
Playwright ihtiyacı ana backend'in Go olmasını engellemez.

### 11.2 Bileşenler

```text
PWA
  -> Go HTTP API
      -> Orchestrator
          -> Source adapters/scrapers
          -> Dedup ve değişiklik tespiti
          -> Listing analyzer
              -> LLMProvider
                  -> OpenRouterProvider
          -> SQLite repository
          -> Notification service
              -> Web Push
```

### 11.3 Scraper arayüzü

Kavramsal Go arayüzü:

```go
type Source interface {
    FetchListings(ctx context.Context) ([]RawListing, error)
}
```

Bir şirket birden fazla kaynak veya profil URL'sine sahip olabilir. Ortak
HTML yapısı kullanan şirketler ayrı kod kopyaları yerine yapılandırılmış aynı
adapter'ı kullanmalıdır.

### 11.4 AI sağlayıcı soyutlaması

Kod Anthropic veya belirli bir OpenRouter modeline bağlanmamalıdır:

```go
type ListingAnalyzer interface {
    Analyze(ctx context.Context, listing RawListing, profile CandidateProfile) (ListingAnalysis, error)
}
```

Örnek yapılandırma:

```env
LLM_PROVIDER=openrouter
LLM_MODEL=<model-id>
OPENROUTER_API_KEY=<secret>
```

Model seçmeden önce ortalama ilan token miktarı, tarama başına yeni ilan
sayısı ve aylık tahmini maliyet hesaplanmalıdır. Model adı ve sağlayıcı kod
değişikliği olmadan değiştirilebilmelidir.

AI çıktısı şema doğrulamalı JSON olmalı ve en az şu alanları içermelidir:

```text
opportunity_type
is_application_open
is_relevant
matching_areas[]
class_year_requirement
gpa_requirement
location
work_model
eligibility_status
application_deadline
summary
confidence
needs_user_decision
decision_question
```

Geçersiz JSON veya API hatasında sınırlı retry yapılmalı; başarısız kayıt
`islenemedi` olarak saklanıp sonraki taramada yeniden denenmelidir.

Mümkün olan alanlar önce deterministik olarak çıkarılabilir. LLM yalnızca
yeni veya içerik özeti değişmiş ilanlarda çağrılarak maliyet azaltılmalıdır.

## 12. Veri modeli

MVP için mantıksal tablolar:

### `companies`

```text
id, name, priority_group, tracking_status, discovery_origin, approved_at
```

### `company_sources`

```text
id, company_id, source_type, url, adapter_type, enabled,
scan_schedule, last_success_at, last_error
```

### `listings`

```text
id, company_id, source_id, external_id, title, canonical_url,
raw_text, content_hash, first_seen_at, last_seen_at, status
```

### `listing_analyses`

```text
listing_id, opportunity_type, is_application_open, is_relevant,
matching_areas, class_year_requirement, gpa_requirement,
location, work_model, eligibility_status, application_deadline,
summary, confidence, needs_user_decision, decision_question,
provider, model, analyzed_at, processing_status, retry_count, last_error
```

### `application_tracking`

```text
id, listing_id, status, deadline, interview_at, notes,
created_at, updated_at
```

### `notifications`

```text
id, listing_id, event_type, channel, status, sent_at, dedup_key
```

### `scan_runs`

```text
id, trigger_type, started_at, finished_at, status,
sources_succeeded, sources_failed, new_listings_count, error_summary
```

Tablolar ileride `user_id` ile çok kullanıcılı hâle getirilebilir. MVP'de
tek kullanıcı için gereksiz ilişki ve yetkilendirme kodu yazılmamalıdır.

## 13. Tekrar ve değişiklik tespiti

- URL kaydedilmeden önce canonical hale getirilmelidir.
- Mümkünse kaynak sistemdeki ilan kimliği kullanılmalıdır.
- Aksi durumda şirket + canonical URL üzerinden kararlı kimlik üretilmelidir.
- Aynı ilan tekrar görülürse `last_seen_at` güncellenir.
- İçerik hash'i değişirse ilan yeniden analiz edilebilir.
- `islenemedi` analiz durumu yeni ilan dedup kontrolüne takılmadan tekrar
  işlenmelidir.
- Bildirim için ayrı `dedup_key` kullanılmalıdır.

## 14. Hata davranışı

- Bir kaynak başarısız olduğunda diğer kaynaklarla devam edilir.
- Hata kaynağı, zamanı ve kısa sebebi kaydedilir.
- Tekrarlanabilir ağ/LLM hatalarında sınırlı retry uygulanır.
- Selector değişmesi sessizce “sıfır ilan” kabul edilmemeli; beklenen sayfa
  işaretleri yoksa scraper hatası üretilmelidir.
- Otomatik erişilemeyen kaynak manuel kontrol listesine düşer.
- Kullanıcı tarama özetinde başarılı ve başarısız kaynakları görebilir.

## 15. Test stratejisi

Test altyapısı ilk fazda kurulacak ve özelliklerle birlikte büyütülecektir.

### 15.1 Scraper testleri

Canlı siteye bağımlı olmayan kaydedilmiş ve kişisel veri içermeyen HTML
fixture'ları kullanılmalıdır:

- sıfır ilan
- tek ilan
- birden fazla ilan
- aynı şirket için birden fazla profil URL'si
- eksik başlık veya URL
- sayfa yapısının/selector'ın değişmesi
- 403/418/429/5xx ve timeout
- aynı ilanın farklı takip parametreli URL'leri
- ilan içeriğinin sonradan değişmesi

### 15.2 AI analiz testleri

OpenRouter gerçek çağrısı yerine mock cevaplar kullanılır:

- geçerli yapılandırılmış cevap
- geçersiz JSON
- eksik alan
- rate limit ve timeout
- retry sonrası başarı
- sınıf şartı nedeniyle `kismen_uygun`
- Ankara dışı dönem içi part-time ilan
- belirsiz ilan ve `karar_bekliyor`
- iletişim bilgilerinin modele gönderilmemesi

Gerçek model çağrıları ayrı, isteğe bağlı entegrasyon testi olmalı ve normal
CI çalışmasında maliyet oluşturmamalıdır.

### 15.3 Depolama ve bildirim testleri

- yeni ilan kaydı
- tekrar ilanı ikinci kez kaydetmeme
- başarısız analizi sonraki çalışmada yeniden işleme
- içerik değişikliğini yakalama
- birincil şirket için tek anlık bildirim
- aynı olay için ikinci bildirim göndermeme
- ikincil ilanları günlük özette toplama
- bildirim bağlantısının doğru PWA ilanını açması

### 15.4 Uçtan uca senaryolar

En az şu kabul senaryosu otomatik test edilmelidir:

```text
Fixture'da yeni staj ilanı görünür
-> scraper ilanı normalize eder
-> dedup yeni olduğunu belirler
-> mock AI ilanı uygun sınıflandırır
-> kayıt SQLite'a yazılır
-> bildirim kuyruğu bir olay üretir
-> dashboard ilanı gösterir
-> ikinci tarama yeni bildirim üretmez
```

## 16. Teslimat fazları

### Faz 0 — Repo ve karar temeli

- Go backend ve React/Vite PWA iskeleti
- aday profili şeması
- yapılandırma ve secret yaklaşımı
- SQLite migration sistemi
- test komutları ve CI temeli

Çıkış kriteri: Backend ve frontend yerelde çalışır; testler tek komutla
koşar; secret repoya girmez.

### Faz 1 — Tek kaynaklı dikey dilim

- Meteksan/kariyer.net adapter'ı
- fixture tabanlı scraper testleri
- temel listing deposu ve dedup
- AI yerine deterministik/mock analiz
- sade API ve tarama sonucu

Çıkış kriteri: Tek bir kaynakta yeni ilan bulunur, kaydedilir ve ikinci
taramada tekrar sayılmaz.

### Faz 2 — Generic kolay-tier kaynaklar

- Çoklu profil URL desteği
- ASELSAN, STM, Baykar ve Samsung konfigürasyonu
- kaynak izolasyonu ve scan run raporu
- selector bozulması/hata senaryoları

Çıkış kriteri: Bir kaynak hata verdiğinde diğerleri tamamlanır ve bütün
fixture testleri geçer.

### Faz 3 — OpenRouter ve uygunluk analizi

- `ListingAnalyzer`/provider soyutlaması
- OpenRouter adapter'ı
- şema doğrulama, retry ve yeniden işleme
- profil bilgisini minimize etme
- maliyet ölçümü
- karar bekleyen ilan akışı

Çıkış kriteri: Model değiştirilebilir; sahte API cevaplarıyla bütün hata ve
uygunluk senaryoları test edilir; başarısız ilan kaybolmaz.

### Faz 4 — Sade PWA ve başvuru takibi

- responsive dashboard
- manuel tarama başlatma
- ilan detay/özet/uygunluk ekranı
- başvuru durumlarını değiştirme
- manuel tarih ve yaklaşan tarihler
- manuel kontrol listesi

Çıkış kriteri: Kullanıcı telefondan tarama başlatabilir, ilanı inceleyebilir
ve başvuru durumunu yönetebilir.

### Faz 5 — Otomatik çalışma ve PWA push

- uygun ücretsiz barındırma seçimi
- Docker image
- GitHub Actions test/build/deploy
- zamanlanmış taramalar
- Web Push aboneliği ve bildirim dedup'ı
- temel loglama, health check ve SQLite yedekleme

Çıkış kriteri: Kullanıcı uygulamayı açmadan zamanlanmış tarama çalışır;
birincil şirketteki yeni uygun staj ilanı PWA push bildirimi üretir.

### Faz 6 — Öncelikli özel kaynaklar

- Akdoğan Tech kaynak araştırması ve adapter
- Turkcell ATS teknik keşfi ve adapter
- Havelsan otomasyon uygunluğu; mümkün değilse manuel kontrol akışı
- bu kaynaklara özel fixture ve hata testleri

Çıkış kriteri: Her kaynak otomatik desteklenir veya açık biçimde manuel
kontrol olarak sınıflandırılır; sessiz başarısızlık olmaz.

### Faz 7 — Şirket kapsamını genişletme

- birincil listedeki kalan şirketler
- ikincil şirket listesi
- şirket öncelik yönetimi
- teknokent aday keşfi ve kullanıcı onayı
- düşük öncelikli günlük özet

Çıkış kriteri: Yeni bir şirket mevcut adapter'la konfigüre edilebilir veya
sınırları belirli yeni bir adapter olarak eklenebilir.

### Faz 8 — İkincil kaynaklar ve zengin başvuru bilgisi

- LinkedIn/kariyer platformları/sosyal medya kaynak değerlendirmesi
- şirket başvuru süreci özetleri
- kaynak ve güncellik bilgisi
- cevaplardan tercih öğrenme
- daha gelişmiş kişiselleştirme

Bu faz MVP sonrasıdır.

## 17. Aşamalı DevOps öğrenme planı

DevOps çalışmaları üründen kopuk örnekler yerine proje ilerledikçe eklenir:

1. Yerel geliştirme, test ve yapılandırma
2. Docker ile tekrarlanabilir çalıştırma
3. GitHub Actions ile otomatik test ve build
4. Ücretsiz ortama deployment
5. Zamanlanmış görevler ve güvenli secret yönetimi
6. Yapılandırılmış loglar, health check ve temel monitoring
7. SQLite yedekleme ve geri yükleme provası

## 18. MVP kapsamı dışında

- Otomatik form doldurma veya başvuru gönderme
- CV/ön yazı üretimi
- Kullanıcı adına kariyer sitesine giriş
- CAPTCHA veya bot korumasını aşma
- Cevaplardan otomatik ve kalıcı tercih öğrenme
- Çok kullanıcılı hesap sistemi
- Bütün sosyal medya kaynaklarını tarama
- Gelişmiş CRM ve analitik
- E-posta cevaplarından otomatik başvuru durumu çıkarma

## 19. MVP başarı ölçütleri

- Seçili resmî kaynaklarda yeni staj ilanı güvenilir biçimde yakalanır.
- Aynı ilan ikinci kez yeni ilan olarak bildirilmez.
- Ulaşılamayan veya yapısı değişen kaynak sessizce geçilmez.
- İlan kısa ve anlaşılır şart/uygunluk özetiyle PWA'da görünür.
- Öncelikli şirketteki yeni uygun ilan PWA push bildirimi üretir.
- Kullanıcı ilanı temel başvuru durumlarından biriyle takip edebilir.
- Son başvuru ve mülakat tarihleri için artan sıklıkta hatırlatma üretilebilir.
- OpenRouter modeli kod değişmeden değiştirilebilir.
- Kritik akışlar fixture, mock ve uçtan uca testlerle korunur.

## 20. Açık kararlar

- ÜşüSebit, AI Studio ve Bilishim şirketlerinin kesin adları/URL'leri
- Ücretsiz barındırma sağlayıcısı ve kalıcı SQLite disk desteği
- Web Push abonelik/anahtar yönetiminin deployment ayrıntıları
- Hatırlatıcıların kesin zaman eşikleri
- Mart 2027 yaklaşırken tarama takviminin yeniden doğrulanması
- Her şirket için doğrulanmış resmî kaynak URL'leri
- OpenRouter model seçimi ve ölçülmüş aylık maliyet
