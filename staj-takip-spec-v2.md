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
güncellenebilir. MVP scheduler'ı beş alanlı cron ifadesi ve IANA zaman dilimiyle
başlangıçta doğrulanır; varsayılan `0 9 * * 1` / `Europe/Istanbul`'dur. Manuel ve
zamanlanmış taramalar tek çalışma yuvasını paylaşır; çakışan manuel istek açık
bir `409` yanıtı alır.

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

`karar_bekliyor`, `needs_user_decision` ve `decision_question` alanlarının
birlikte değişme kuralı model talimatında açık olmalı ve backend'de yeniden
doğrulanmalıdır. Şema/iş kuralı hatasında sonraki sınırlı denemeye kısa hata
geri bildirimi eklenebilir; backend doğrulaması gevşetilmemelidir.

İlanın erişim zamanı kişisel olmayan analiz bağlamı olarak modele verilebilir.
Aday erişim anında şartın tam bir sınıf altındaysa ve fırsat sonraki akademik
dönemde başlıyorsa sınıf geçişi varsayılmamalı; başlangıçtaki sınıf durumu
`karar_bekliyor` sorusuyla kullanıcıya bırakılmalıdır.

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

Uygulama notu: Sağlayıcı/model backend ortam ayarlarından seçilir. Aday profili
kurum ve doğrudan kimlik bilgileri çıkarılarak minimize edilir. Başarısız analiz
ham ilanla birlikte `karar_bekliyor/pending` saklanır ve kaynak siteye yeniden
erişmeden ayrı retry akışıyla işlenebilir. Başarılı analiz provider/model, token
kullanımı ve yapılandırılmış fiyat oranlarından hesaplanan tahmini maliyeti taşır.
OpenRouter yanında aynı `ModelProvider` portunu kullanan doğrudan Google Gemini
adapter'ı test/alternatif sağlayıcı olarak seçilebilir; normal testler her iki
adapter için de yalnızca fake HTTP cevapları kullanır.

### Faz 3.5 — Gerçek ilan kabul doğrulaması

- Resmî şirket kariyer sayfası veya herkese açık resmî ATS üzerinde, doğrulama
  anında başvurusu açık en az bir gerçek staj ilanı bulma
- Kaynak URL'sini, erişim zamanını ve ilanın açık olduğuna dair güvenli/kısa
  kanıtı kaydetme; erişim korumasını veya site şartlarını aşmama
- İlanı doğrudan veritabanına eklemek yerine uygun kaynak adapter'ı üzerinden
  normalizasyon, canonical URL ve dedup akışına alma
- Canlı Google Gemini sağlayıcısıyla minimize profil ve strict şema üzerinden
  analiz edip provider/model/token bilgisini SQLite'a yazma
- İlanın `uygun`, `kismen_uygun` veya gerekçeli `karar_bekliyor` sonucu ile
  dashboard API'sinde görünmesini doğrulama
- Aynı kaynağın ikinci işlenişinde duplicate ilan ve ikinci yeni kayıt
  oluşmadığını doğrulama
- Canlı kariyer sitesi ve Gemini çağrılarını opt-in kabul testiyle sınırlama;
  normal suite için küçük, güvenli ve tekrarlanabilir fixture/fake cevap ekleme
- Gerçek API anahtarını, kişisel veriyi, tüm canlı sayfa gövdesini veya yerel
  SQLite veritabanını repoya koymama

Çıkış kriteri: Zaman damgalı resmî kaynak kanıtıyla aktif olduğu doğrulanan en az
bir gerçek staj ilanı, korumaları aşmadan standart ingestion ve canlı Gemini
analizinden geçer; SQLite'ta kullanım metadatasıyla saklanır, dashboard API'sinde
görünür ve ikinci çalıştırmada duplicate olmaz. Normal testler canlı servislere
bağlanmadan aynı yolu fixture ve fake provider ile tekrarlar.

Uygulama notu: Herkese açık resmî Lever tek-ilan sayfaları `lever` adapter'ıyla
izlenir. Adapter yalnızca `jobs.lever.co` HTTPS URL'sini, beklenen ilan yapısını
ve aynı ilana ait aktif `/apply` bağlantısını kabul eder; sayfanın tamamı yerine
analiz için gereken normalize ilan alanları saklanır.

Kabul testi aynı orchestrator yolunu normal suite'te fixture/fake provider ve
geçici SQLite ile, `integration` etiketinde ise canlı Lever/Gemini ile çalıştırır.
Canlı test ayrıca açık opt-in ortam bayrağı olmadan başlamaz ve ikinci taramada
işlenmiş duplicate ilan için modeli yeniden çağırmaz.

Tamamlama kanıtı: 3 Ağustos 2026'da resmî Commencis Lever ilanı canlı Google
Gemini analiziyle `karar_bekliyor` sonucuna ulaştı; provider/model/token/maliyet
SQLite'ta saklandı, dashboard API karar kuyruğunda göründü ve ikinci tarama sıfır
yeni kayıt üretti. Güvenli kayıt `docs/acceptance/phase-3.5-2026-08-03.md`
dosyasındadır.

### Faz 4 — Sade PWA ve başvuru takibi

- responsive dashboard
- manuel tarama başlatma
- ilan detay/özet/uygunluk ekranı
- başvuru durumlarını değiştirme
- manuel tarih ve yaklaşan tarihler
- manuel kontrol listesi

Çıkış kriteri: Kullanıcı telefondan tarama başlatabilir, ilanı inceleyebilir
ve başvuru durumunu yönetebilir.

Tamamlama kanıtı: Responsive PWA kartları ve tam genişlik mobil detay paneli
manuel taramayı, uygunluk incelemeyi, beş durumlu başvuru takibini, kullanıcı
tarihi/mülakat zamanı/not kaydını ve hatalı kaynakların manuel kontrol listesini
birleştirir. Fixture tabanlı Faz 4 kabul testi aynı akışı gerçek SQLite ve üretim
HTTP handler'ı üzerinden ağ çağrısı olmadan doğrular.

### Faz 5 — Otomatik çalışma ve PWA push

- uygun ücretsiz barındırma seçimi
- Docker image
- GitHub Actions test/build/deploy
- zamanlanmış taramalar
- Web Push aboneliği ve bildirim dedup'ı
- temel loglama, health check ve SQLite yedekleme

Çıkış kriteri: Kullanıcı uygulamayı açmadan zamanlanmış tarama çalışır;
birincil şirketteki yeni uygun staj ilanı PWA push bildirimi üretir.

Uygulanan doğrulama: Process-içi cron scheduler ve tarama kilidi; analizle aynı
SQLite transaction'ında versionlanmış outbox/dedup; cihaz bazlı Web Push
delivery/retry/410 temizliği; kullanıcı eylemiyle PWA aboneliği ve doğru ilanı
açan aynı-origin deep-link fixture/fake testlerle tamamlandı. Faz 5'in bütününün
kapanması için production barındırma/güvenli erişim, image publish/deploy-smoke-
rollback, offsite backup ve secret yönetimi kanıtları ayrıca tamamlanmalıdır.

Health doğrulaması iki ayrı endpoint kullanır: `/health` bağımlılıksız liveness,
`/ready` ise SQLite ping'i ile bu image sürümünün migration kayıtlarını doğrulayan
readiness yanıtıdır. Compose API'yi `/ready` ile izler ve web container'ını
`service_healthy` sonrasına bırakır. API/PWA `nosniff`, referrer, frame ve
same-origin CSP başlıklarıyla korunur; HSTS yalnız gerçek HTTPS'i sonlandıran
production proxy katmanında eklenmelidir, yerel HTTP Compose listener'ında değil.

SQLite yedekleme, production'da açıkça etkinleştirilen günlük bir süreç olmalı;
çalışan SQLite dosyasının ham kopyası yerine SQLite'ın tutarlı snapshot yöntemi
kullanılmalıdır. Yedekler private dosya izinleriyle saklanmalı, sınırlı retention
uygulanmalı, bütünlük kontrolünden geçmeli ve restore edilebilirliği geçici
dizinli otomatik testle kanıtlanmalıdır. Yerel geliştirme varsayılanı yedek dosyası
oluşturmamalıdır.

### Faz 6 — Öncelikli özel kaynaklar

- Akdoğan Tech kaynak araştırması ve adapter
- Turkcell ATS teknik keşfi ve adapter
- Havelsan otomasyon uygunluğu; mümkün değilse manuel kontrol akışı
- bu kaynaklara özel fixture ve hata testleri

Çıkış kriteri: Her kaynak otomatik desteklenir veya açık biçimde manuel
kontrol olarak sınıflandırılır; sessiz başarısızlık olmaz.

#### Kaynak keşif notları (2026-08-08, web araştırması)

Aşağıdaki bulgular yalnız gözlemdir; henüz adapter/fixture kodu yazılmadı
(AGENTS.md'nin test-önce ve faz-öncesi onay kuralları gereği, kod yazımı ayrı
bir onay adımı bekliyor). Amaç, her şirketi bir tier'a (§16 arka plan, Faz
9-14) yerleştirmek.

**Akdoğan Tech** (`akdogan.tech/career`): Statik sayfa, ilan listesi yok, JSON-LD
yok, ATS entegrasyonu yok. Başvuru yalnız `career@akdogan.tech` adresine CV
göndermeyle yapılıyor — kazınabilir ayrı ilan yapısı yok. **Tier önerisi:
`manual`.** Otomatik adapter yazılabilecek bir yapı yok; sayfa periyodik
olarak (içerik değişikliği için) manuel kontrol listesine alınabilir.

**Turkcell** (`kariyerim.turkcell.com.tr`): Özel/JS tabanlı sistem; Workday,
SuccessFactors gibi bilinen bir ATS tespit edilmedi. JSON-LD yok. Ayrı ilan
başlıkları listeleyen bir sayfa yerine **program düzeyinde** açılış sayfaları
var (GNÇYTNK genç yetenek, Sınırsız Yetenek), "Hemen Başvur" bağlantıları
(`/gncytnkstaj`, `/genc-yetenek/basvuru`) program başvurusuna götürüyor. **Tier
önerisi: `manual` veya yeni bir `program_window` deseni** — mevcut
`RawListing` modeli (başlık + URL başına bir ilan) buraya tam oturmuyor; asıl
sinyal "program açık mı/ne zaman kapanıyor" oluyor. Bu, adapter yazmadan önce
ayrı bir tasarım kararı gerektiriyor (bkz. §20 açık kararlar'a eklenecek).

**Havelsan**: Resmî başvuru kanalları LinkedIn ve kendi "Kovan" portalı
(`kariyer.havelsan.com.tr`) olarak `havelsan.com/tr/alim-sureci` sayfasında
açıkça belirtiliyor. Kovan, istemci tarafında render edilen bir uygulama gibi
görünüyor (düz HTML çekiminde yalnız başlık geldi, ilan içeriği gelmedi) —
gerçek ilanları görmek muhtemelen headless render veya arkadaki API'nin
(varsa) keşfini gerektiriyor; bu oturumda doğrulanamadı. Ayrıca Havelsan'ın
eski bir kariyer.net profili de var
(`kariyer.net/firma-profil/havelsan-1148-1612`) ama bu oturumda 403 ile
engellendi — mevcut `kariyer_net` adapter'ının zaten ele aldığı Cloudflare
challenge/korumasıyla tutarlı (`AccessError.Protective()`), yeni bir sorun
değil. **Tier önerisi: `llm_generic` veya `learned_selector` adayı**, ama
Kovan'ın JS-render/API yapısı doğrulanmadan kesinleşemez; kariyer.net
kanalı yalnız erişim engeli kalkarsa yedek olabilir.

Ortak gözlem: Üçünde de mevcut iki adapter'dan (kariyer_net, lever) hiçbiri
doğrudan uygulanamıyor. Havelsan dışında hiçbiri şu an "ilan listesi" tarzı
kazınabilir bir yapı sunmuyor — bu, Faz 10'un yapılandırılmış-veri
kaynaklarının (JSON-LD/ATS API) bu üç şirket için düşük olasılıklı olduğunu,
Faz 11-12'nin (reduce-then-LLM + öğrenilmiş reçete) veya basit `manual`
sınıflandırmasının daha gerçekçi olduğunu gösteriyor.

#### İzleme listesi uygulaması (2026-08-08)

Üçü de (Akdoğan Tech, Turkcell, Havelsan) `manual` olarak sınıflandırıldı ve
dashboard'a **ayrı, kalıcı bir "İzleme listesi" paneli** eklendi — mevcut
"Manuel kontrol listesi" panelinin (adı "Taranamayan kaynaklar" olarak
netleştirildi) anlamı yalnız scraper hatası olan kaynaklara daraltıldı, ikisi
artık kesişmiyor. Kullanıcı "Kontrol ettim" ile son kontrol zamanını
kaydedebiliyor (`PUT /api/v1/watchlist/{id}/checked`). Bu, Faz 6'nın "her
kaynak otomatik desteklenir veya açık biçimde manuel kontrol olarak
sınıflandırılır" çıkış kriterinin bu üç şirket için karşılandığı anlamına
gelir — ama gerçek adapter/otomasyon çalışması (özellikle Havelsan'ın Kovan
portalı için, bkz. yukarıdaki açık karar) hâlâ yapılmadı; watchlist bir
nihai durum değil, o çalışma tamamlanana kadarki dürüst ara durumdur.
Turkcell'in program-düzeyi veri modeli sorusu (§20) da hâlâ açık.
Uygulama ve test detayları: `docs/architecture.md`, "İzleme listesi ve
taranamayan kaynaklar" bölümü.

### Faz 7 — Şirket kapsamını genişletme

- birincil listedeki kalan şirketler
- ikincil şirket listesi
- şirket öncelik yönetimi
- teknokent aday keşfi ve kullanıcı onayı
- düşük öncelikli günlük özet

Çıkış kriteri: Yeni bir şirket mevcut adapter'la konfigüre edilebilir veya
sınırları belirli yeni bir adapter olarak eklenebilir.

### Ölçeklenebilir kaynak kapsamı (Faz 9–14) — arka plan

Faz 9–14, 2026-08-08 ürün görüşmesinde alınan yön kararını uygular: uygulama
öncelikle **kişisel** bir araç olarak geliştirilecek (çok kullanıcılı ürün
hedefi ertelendi) ve gerçek faydası **kaynak kapsamını genişletmekten** gelecek
— altyapı (Faz 0–5) zaten olgun. Temel içgörü: bugün izlenen tüm kaynaklar
kariyer.net üzerinde ve kariyer.net'in kendi takip/uyarı özelliğini tekrarlıyor.
Asıl değer, kariyer.net'in görmediği kaynaklardan (şirket kariyer sayfaları,
ATS'ler, teknokent şirketleri) gelen ilanları tek bir yerde toplamakta.

Her şirket için elle scraper yazmak sürdürülemez bir bakım yüküdür ("selector
kırılması" tuzağı). Bu fazlar o yükü kademeli (tiered) bir kaynak-strateji
mimarisiyle çözer. Yönlendirici ilkeler:

- **Maliyet disiplini:** LLM yalnız yeni/değişmiş içerikte çağrılır; ucuz ve
  deterministik yollar her zaman önce denenir (§13 hash/dedup ile tutarlı).
- **Sessiz başarısızlık yok:** Yapı değişince "sıfır ilan" kabul edilmez; hata
  üretilir veya kendini onaran akış tetiklenir (§14 ile tutarlı).
- **Veri-odaklı esneklik:** Yeni şirket eklemek kod deploy'u değil, bir kayıt
  eklemektir.
- **Uyum:** Erişim koruması/CAPTCHA aşılmaz; site ToS ve robots.txt'e uyulur
  (§18 kapsam-dışı ile tutarlı).

Önerilen sıra değeri erken getirir: önce ucuz ve geniş kapsam (Faz 9–10), sonra
kaotik siteler için AI destekli çıkarım (Faz 11–12), sonra ölçek sorunları
(Faz 13–14).

### Faz 9 — Kaynak strateji soyutlaması (veri-odaklı, kademeli dispatch)

Amaç: kaynak davranışını bir **veri özelliği** yapmak. Her kaynak bir `strategy`
alanı taşır; orchestrator mevcut `Source` arayüzü arkasında bu stratejiye göre
adapter seçer, böylece downstream (dedup, analiz, bildirim) değişmez.

- Kaynak stratejileri: `json_ld | ats_api | learned_selector | llm_generic | manual`.
- Stratejiler ve parametreleri config/DB'de tanımlıdır; yeni şirket eklemek kod
  değişikliği gerektirmez.
- Her kaynak bir sağlık/durum snapshot'ı taşır (son başarı, son ilan sayısı, son
  hata, strateji sürümü).

Çıkış kriteri: Aynı orchestrator yolu kaynak başına strateji seçerek çalışır;
mevcut kariyer.net ve Lever adapter'ları strateji dispatch'i altında regresyonsuz
çalışır.

Neden: Şirket sayısı arttıkça asıl darboğaz koddur. Stratejiyi veriye taşımak
her yeni kaynağı "bir satır + tier seçimi"ne indirger ve sonraki tüm fazların
temelini kurar.

Uygulama durumu (2026-08-08): Dispatch soyutlaması tamamlandı —
`internal/scraper/registry.go`'daki veri-tablosu (`adapterFactories`) sabit
kodlu `switch`'in yerini aldı; `SourceConfig.EffectiveStrategy()` açık
`strategy` alanını veya `kariyer_net`/`lever` için `legacy_html` varsayılanını
döndürür ve `company_sources.strategy` sütununda saklanır
(`migrations/006_source_strategy.sql`). Mevcut iki adapter regresyonsuz
çalışıyor (`go test ./...` yeşil). **Eksik kalan:** kaynak bazlı
sağlık/durum snapshot'ı (son ilan sayısı, strateji sürümü) henüz yok — bu,
Faz 12'nin golden-snapshot guard'ıyla birlikte eklenecek, ayrı bırakıldı çünkü
anlamı yalnız öğrenilmiş reçeteler bağlamında netleşiyor.

### Faz 10 — Yapılandırılmış-veri-öncelikli ingestion (deterministik, AI'sız)

Amaç: birçok kariyer sayfasının veriyi zaten hazır verdiği ucuz yolları önce
kullanmak. En yüksek kapsam artışını en düşük maliyetle sağlar.

- **schema.org `JobPosting` JSON-LD**: Google for Jobs nedeniyle çok sayıda site
  `<script type="application/ld+json">` içinde standart ilan verisi (title,
  location, employmentType, datePosted, validThrough) gömer. Deterministik,
  ücretsiz, siteler arası standart ve layout değişimine dayanıklı.
- **sitemap.xml ve RSS/Atom** keşfi: ilan URL'lerini desen/feed üzerinden bulma.
- **ATS JSON API adapter'ları**: Greenhouse (`boards-api.greenhouse.io`), Workday;
  Lever zaten mevcut. Scraping değil, yapılandırılmış API — ucuz ve kararlı.
- Tier sırası: `json_ld → feed/sitemap → ats_api → (sonraki fazların fallback'i)`.

Çıkış kriteri: JSON-LD içeren bir fixture sayfası ve bir ATS API fixture'ı, hiç
AI çağrısı olmadan strict şemaya normalize edilip mevcut dedup/analiz yoluna girer.

Uygulama durumu (2026-08-08): Tamamlandı. İki deterministik adapter aynı
`adapterFactories` tablosuna eklendi: `json_ld` (`internal/scraper/jsonld.go`)
schema.org `JobPosting` JSON-LD'yi (@graph/dizi/tekil sarmalı düzleştirerek)
çıkarır; `greenhouse` (`internal/scraper/greenhouse.go`, strateji `ats_api`)
herkese açık Greenhouse board API JSON'unu tüketir. İkisi de yalnızca
`RawListing`'e normalize eder (tag'ler sökülür, URL kanonikleşir), yapı
değişiminde sessiz "sıfır ilan" yerine `ErrUnexpectedPage` üretir; boş
Greenhouse board'u ise geçerli sayılır. `adapterDefaultStrategy` stratejiyi
adapter'dan çıkardığı için kaynak eklemek tek kayıt. Fixture-tabanlı birim
testleri (`jsonld_test.go`, `greenhouse_test.go`) ve uçtan uca kabul testi
(`internal/acceptance/phase10_test.go`: tam orchestrator → dedup → analiz →
dashboard, değişmeyen ikinci taramada sıfır yeni/tekrar analiz) yeşil.
**Kapsam-dışı bırakılan:** sitemap.xml ve RSS/Atom keşfi ile Workday adapter'ı
sonraki artımlara bırakıldı; JSON-LD + Greenhouse çıkış kriterini karşılıyor.

Neden: Bu tier, muhtemelen herhangi bir akıllı sezgiselden daha çok şirketi,
sıfır AI maliyeti ve sıfır kırılgan selector ile kapsar. Pahalı yollara düşmeden
önce daima denenmelidir.

### Faz 11 — Kaotik/bespoke siteler için generic reduce-then-LLM adapter

Amaç: API'si ve kararlı yapısı olmayan uzun-kuyruk kariyer sayfaları ve (yalnız
yasal erişilebilen) kaotik içerik için, maliyeti kontrol altında tutan AI destekli
çıkarım.

- **Fetch (+ gerekirse headless render / Playwright)**: modern kariyer portalları
  çoğunlukla JS SPA'dır (§11.1 yardımcı katmanı).
- **Reduce (ucuz, deterministik, AI'sız)**: `script/style/nav/footer` çıkar,
  HTML'i düz metne/markdown'a indir, anahtar kelime (staj/başvur/ilan/açık
  pozisyon/intern/new grad) + **yapısal yakınlık** (aynı blok, tekrar eden
  kart/liste) ile aday blokları pencerele.
- **Ucuz sınıflandırıcı basamağı**: keyword/yakınlık filtresi (bedava) → embedding
  benzerliği (neredeyse bedava) → LLM extraction (nadir). Gürültünün çoğu ilk iki
  basamakta elenir.
- **Content-hash kapısı**: indirgenmiş içeriğin hash'i değişmedikçe LLM çağrılmaz
  (§13 ile tutarlı).
- **Diff-tabanlı çıkarım**: hash değiştiğinde, önceki indirgenmiş bloklara göre
  yalnız yeni/değişen blok(lar) modele gönderilir; tüm sayfa değil.
- **Strict şema doğrulama + deterministik dedup**: LLM çıktısı asla doğrudan
  bildirime gitmez; mevcut analyzer'ın şema/iş-kuralı yeniden doğrulaması uygulanır.
  Düşük güven → `karar_bekliyor`/inceleme kuyruğu, bildirim değil.

Çıkış kriteri: Bespoke bir HTML fixture'ından ilan(lar) doğru çıkarılır;
değişmeyen ikinci taramada hiç LLM çağrısı yapılmaz; geçersiz/eksik çıktı
`islenemedi`/`karar_bekliyor` yoluna düşer.

Uygulama durumu (2026-08-08): Tamamlandı. `llm_generic` adapter'ı
(`internal/scraper/llmgeneric.go`) aynı `adapterFactories` tablosuna eklendi;
downstream değişmedi. Akış: fetch → **reduce** (script/style/nav/footer/aside
atılır; aday bloklar anahtar kelime + yapısal yakınlık/innermost ile pencerelenir;
başvuru linkleri `LINK:` satırı olarak eklenir) → **content-hash kapısı + blok
bazlı diff** (yalnız yeni/değişen blok modele gider; değişmeyen taramada sıfır
LLM çağrısı — blok hash → çıkarılan ilan önbelleği) → enjekte `ListingExtractor`
portu → strict doğrulama (başlık + sayfaya göre çözümlenen mutlak URL; eksik
çıktı düşürülür, malformed model çıktısı hata=`islenemedi`). Aday bloğu
bulunmayan sayfa sessiz sıfır yerine `ErrUnexpectedPage` verir. Extractor portu
`scraper.ListingExtractor`; Gemini implementasyonu ayrı pakette
(`internal/extractor/gemini.go`) olduğundan scraper analyzer'a bağımlı olmaz.
Testler: fixture birim testleri (`llmgeneric_test.go`, `extractor/gemini_test.go`),
uçtan uca kabul (`phase11_test.go`), ve **canlı Gemini** integration testi
(`phase11_live_test.go`, opt-in): 2026-08-08'de gerçek `gemini-3.1-flash-lite`
ile yerel olarak sunulan bespoke sayfadan 2 ilan çıkarıldı, analiz edildi,
ikinci taramada 0 yeni (dedup), 475 token. Kapsam-dışı bırakıldı: embedding
benzerlik basamağı ve headless render (Playwright); şu an keyword/yakınlık →
LLM ve kalıcı (restart'ı aşan) blok önbelleği (Faz 12 reçete deposuyla gelecek).

Neden: kariyer.net gibi kararlı sözleşmesi olan siteleri LLM'e taşımak gerilemedir;
bu adapter yalnız gerçekten kaotik kaynaklar içindir. Reduce + hash + diff
basamakları, kapsamı genişletirken AI maliyetini taban seviyede tutar.

### Faz 12 — Kendini onaran öğrenilmiş çıkarım reçeteleri

Amaç: "her site için elle selector yazma/onarma" bakım yükünü ortadan kaldırmak.
Ajan reçeteyi **bir kez kurulumda üretir/onarır**; deterministik motor onu her
taramada ucuza çalıştırır.

- **Reçete**: selector/kural seti, sürüm, ve **son başarılı ilan sayısı/parmak izi
  (golden snapshot)**. Kaynak bazında DB'de saklanır.
- Her tarama reçeteyi deterministik çalıştırır (AI'sız, bedava).
- **Onarım guard'ı**: kimlik kontrolü kırılırsa, tarihsel N ilandan 0'a düşülürse
  veya şema doğrulaması kırılırsa → reçete AI ile yeniden türetilir, sürüm
  artırılır, saklanır.
- Reçeteler DB'de birikir; sistemin her sitenin şekline dair kurumsal bilgisi olur.
  Kullanıcı düzeltmeleri reçeteyi iyileştirir (hafif insan-döngüde).

Çıkış kriteri: Bir fixture site için AI ile üretilen reçete saklanır ve sonraki
taramalar AI'sız çalışır; yapısı değiştirilen bir fixture'da golden-snapshot
guard'ı sessiz "sıfır ilan" yerine yeniden-türetmeyi tetikler.

Uygulama durumu (2026-08-09): Tamamlandı. `learned_selector` adapter'ı ilk
tarama/onarımda strict JSON Schema kullanan `RecipeLearner` portundan sınırlı
selector reçetesi alır; normal taramalarda reçeteyi deterministik ve AI'sız
çalıştırır. Reçeteler SQLite'ta versionlanır, tek aktif sürüm ile golden ilan
sayısı/parmak izi kaynak sağlık snapshot'ına atomik bağlanır. Kimlik kontrolü,
kart şeması veya tarihsel pozitif sayıdan sıfıra düşüş bir defalık onarımı
tetikler; yeni reçete aynı sayfada geçerli ilan üretmeden saklanmaz. Faz 11 blok
cache'i de SQLite'a taşınarak restart sonrasında model çağrısı engellendi.
Fixture/fake kabulü ilk öğrenme, AI'sız restart ve layout değişiminde v2 onarımını
doğruladı.
Gerçek `gemini-3.1-flash-lite` opt-in kabulü de aynı üç aşamayı doğruladı;
güvenli canlı kanıt `docs/acceptance/phase-12-2026-08-09.md` içindedir.

Neden: Bu "ajan scraper'ı yazar/onarır, deterministik motor çalıştırır" kalıbıdır.
AI'yı her taramanın sıcak yolundan çıkarır; hem maliyeti hem de sessiz başarısızlık
riskini düşürür.

### Faz 13 — Kaynaklar arası dedup ve kanonik fırsat modeli

Amaç: Aynı ilan birden çok kaynakta (kariyer.net + şirket sayfası + LinkedIn)
göründüğünde tek fırsat = tek bildirim.

- **Kanonik fırsat**: `company + normalize edilmiş başlık (+ lokasyon)` üzerinden
  bulanık (fuzzy) eşleme.
- Birden çok listing tek fırsata bağlanır; bildirim `dedup_key` fırsat düzeyinde
  uygulanır (§8/§13 ile tutarlı).

Eşleme muhafazakârdır: şirket kimliği birebir aynı olmalıdır. Normalize başlığı
birebir aynı olan kayıtlar lokasyonları uyumluysa (veya ikisi de lokasyonsuzsa),
bulanık başlık skoru en az `0.92` olan kayıtlar ise iki lokasyon da mevcut ve
uyumluysa otomatik birleşir. `0.80 <= skor < 0.92`, tek tarafı eksik lokasyon ve
diğer yetersiz kanıtlar belirsiz karar olarak saklanır ama ayrı fırsatlarda
kalır. Açık lokasyon çelişkisi skordan bağımsız olarak birleşmeyi engeller.

Çıkış kriteri: İki farklı kaynaktan gelen aynı ilan tek fırsat olarak gösterilir
ve yalnız bir bildirim üretir.

Tamamlanma kanıtı: `internal/acceptance/phase13_test.go` içindeki JSON
fixture/fake analizci yolu iki farklı kaynak listing'ini gerçek migration ve
SQLite üzerinden tek fırsat, tek üretim dashboard kartı ve fake göndericiye tek
Web Push olarak doğrular. Eksik lokasyon kabul senaryosu iki kaydı ayrı tutar;
repository testleri audit, startup reconciliation ve çelişkide split davranışını
kapsar.

Neden: Kaynak sayısı arttıkça per-source dedup yetmez; aksi hâlde daha çok kaynak
= daha çok tekrar bildirim = kullanıcının bildirimleri kapatması.

### Faz 14 — Sosyal/manuel kaynaklar ve uyum (compliance)

Amaç: Erişim sınırlarını dürüstçe ele almak. Kaotik içerik için "reduce → AI"
doğru araçtır; ama bazı kaynaklarda sorun *parsing* değil *erişim*tir.

- **LinkedIn/sosyal medya doğrudan scraping YAPILMAZ**: agresif bot koruması,
  ToS/yasal risk ve kullanıcının kendi hesabının banlanma riski. Alternatifler:
  kullanıcının **kendi iş-uyarı e-postalarını** ayrıştırma (rıza dahilinde, kendi
  gelen kutusu), varsa RSS, ya da **manuel kontrol listesi**.
- **Per-domain erişim yönetimi**: mevcut `AccessPolicy` genişletilir — robots.txt
  saygısı, rate limit, cooldown birinci sınıf hâle gelir.
- Otomatik erişilemeyen her kaynak sessizce geçilmez; manuel kontrol listesine
  düşer (§6 ile tutarlı).

Domain politikası config'te `robots | public_api | manual_only` modlarından biri
ve otomatik modlarda minimum aralık/temel-maksimum cooldown ile tanımlanır; en
uzun domain suffix'i uygulanır ve çözülmüş değerler SQLite kaynak kaydında
saklanır. `manual_only` sosyal kaynak hiçbir scraper/HTTP istemcisi kurmadan
kalıcı watchlist'e girer. Faz 14 e-posta/IMAP/OAuth entegrasyonu eklemez; kişisel
veri ve rıza tasarımı gerektiren bu alternatif ayrı faza bırakılır.

`robots` modu her ilan isteğinden önce scheme/authority kökündeki `/robots.txt`
kararını zorunlu kılar. RFC 9309 product-token grup birleşimi, wildcard fallback,
en uzun yol ve eşitlikte allow kuralları uygulanır; `*` ve `$` desteklenir.
Başarılı veya 4xx kararlar domain bazında en çok 24 saat cache'lenir ve dosya
512 KiB ile sınırlanır. 404/410 unavailable kabul edilip erişime izin verir;
diğer 4xx engeller, 5xx/ağ/okuma/geçersiz hedef fail-closed davranır. `public_api`
robots kontrolünü atlar; iki otomatik mod da kalıcı minimum aralık/cooldown
bütçesinden geçer. Robots engeli sessiz başarı sayılmaz, fetch yapılmadan kaynak
durumuna güvenli gerekçeyle işlenir.

Çıkış kriteri: LinkedIn benzeri bir kaynak, scraping denemeden manuel-checklist
veya e-posta-ayrıştırma stratejisiyle temsil edilir; robots/rate-limit politikası
per-domain uygulanır.

Durum (10 Ağustos 2026): Tamamlandı. Havelsan'ın resmî LinkedIn profili
`manual_only` watchlist kaydıdır ve scraper üretmez. Fixture/fake HTTP kabulü
robots izni/yasağı ile kalıcı domain aralığını gerçek SQLite ve orchestrator
üzerinde doğrular; kanıt `docs/acceptance/phase-14-2026-08-10.md` içindedir.

Neden: Anti-bot sistemleriyle savaşmak sürdürülemez ve risklidir; yasal
erişilebilen içeriğe AI uygulanır, gerisi manuel listeye yönlendirilir (§18
kapsam-dışı ilkeleriyle tutarlı).

### Faz 15 — İkincil kaynaklar ve zengin başvuru bilgisi (eski Faz 8, ertelendi)

- LinkedIn/kariyer platformları/sosyal medya kaynak değerlendirmesi
- şirket başvuru süreci özetleri
- kaynak ve güncellik bilgisi
- cevaplardan tercih öğrenme
- daha gelişmiş kişiselleştirme

Bu faz MVP sonrasıdır ve 2026-08-08'de bilinçli olarak sona alındı: bu faz
Faz 9–14'ün inşa edeceği altyapıya bağımlıdır — LinkedIn/sosyal medya
değerlendirmesi Faz 14'ün "scraping yapılmaz, e-posta/manuel'e yönlendir"
sınırını önceden gerektirir; süreç özetleri ve zengin başvuru bilgisi Faz 11'in
reduce-then-LLM çıkarım motorunu kullanabilir. Bu fazı Faz 9–14'ten önce
çalışmak, henüz var olmayan bir uyum sınırına ve çıkarım motoruna dayanmak
anlamına gelirdi; bu yüzden sona alındı. (Faz numarası korunmuştur; Faz 9–14
zaten uygulanmış/dokümante edilmiş referanslarla eşleşsin diye yeniden
numaralandırılmamıştır.)

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
- Turkcell gibi program-düzeyinde (ilan başına değil) başvuru açan kaynaklar
  için veri modeli: mevcut `RawListing`/`listings` şeması (başlık + canonical
  URL başına bir kayıt) "program açık/kapalı + tarih aralığı" sinyaline tam
  oturmuyor; ayrı bir `program_window` kavramı mı gerekiyor yoksa program
  açılışı tek bir sentetik "ilan" olarak mı modellenmeli, karara bağlanmalı
  (bkz. Faz 6 kaynak keşif notları, 2026-08-08)
- Havelsan'ın "Kovan" portalının (`kariyer.havelsan.com.tr`) istemci tarafı
  render mimarisi ve varsa arkasındaki JSON API henüz doğrulanmadı; headless
  render veya ağ isteği incelemesi gerektiriyor (bkz. Faz 6 kaynak keşif
  notları)
