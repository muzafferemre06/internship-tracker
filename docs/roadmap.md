# Ürün yol haritası

Bu belge, Faz 14 sonrasında izlenecek ürün sırasının ve ileride yanlış varsayımla
uygulanmaması gereken karar kapılarının otoritatif özetidir. Ayrıntılı ürün
gerekçeleri `staj-takip-spec-v2.md`, tamamlanan işlerin kanıtı
`docs/progress.md` ve `docs/acceptance/` altındadır.

Son kullanıcı yönü 10 Ağustos 2026'da yeniden netleştirildi: öncelik gelişmiş
analitik değil; kalıcı fırsat hafızası, spec'teki şirketlerin tamamına genişleme
ve şirket sitesi dışındaki izinli e-posta/RSS akışlarından yüksek kapsamayla
fırsat keşfidir.

## Değişmez ürün sınırları

- Uygulama otomatik başvuru göndermez veya form doldurmaz.
- Kullanıcı hesabıyla kariyer sitesine giriş yapmaz.
- CAPTCHA, bot koruması veya erişim kontrolü aşılmaz.
- LinkedIn ve benzeri sosyal platformlar doğrudan scrape edilmez.
- Normal testler canlı kariyer sitesi veya ücretli AI API kullanmaz.
- Fırsat kanıtları fiziksel olarak silinmez; yaşam döngüsü durumu ve arşivle
  görünürlük yönetilir.
- Yalnız kullanıcının ilgi alanlarıyla güçlü biçimde eşleşen, güvenilir fırsat
  anlık bildirim üretir. Diğer makul adaylar “Fırsatlar” penceresine gider.
- Gelişmiş analitik ve otomatik/kalıcı tercih öğrenme şirket ve akış kapsaması
  tamamlanana kadar ertelenir.

## Kapsama ölçümü

Bir şirketin yalnız config'te bulunması “otomatik izleniyor” anlamına gelmez.
Raporlar aşağıdaki durumları ayrı göstermelidir:

- `otomatik`: En az bir izinli kaynak zamanlanmış olarak çalışıyor.
- `akış`: RSS/Atom/e-posta gibi izinli bir akıştan izleniyor.
- `manuel`: Kaynak katalogda ve watchlist'te, fakat otomatik istek yapılmıyor.
- `araştırılıyor`: Kimlik veya kaynak henüz doğrulanmadı.
- `bozuk`: Daha önce otomatik olan kaynak güncel olarak hata veriyor.

Faz kabulünde “katalog kapsamı” ile “otomatik/akış kapsamı” ayrı sayılarla
kanıtlanır; manuel kaynak otomatik başarı gibi raporlanmaz.

## Faz sırası

1. Faz 14.1 — Kalıcılık, geçmiş ve scan hata sözleşmesi
2. Faz 15 — Birincil şirketlerin tamamlanması
3. Faz 16 — İkincil şirketlerin tamamlanması
4. Faz 17 — Üçüncül şirket araştırması
5. Faz 18 — Onaylanan üçüncül şirketlerin eklenmesi
6. Faz 19 — Genel fırsat modeli ve bildirim katmanları
7. Faz 20 — RSS/Atom ve açık akışlar
8. Faz 21 — Ayrı posta kutusuyla güvenli e-posta fırsat akışı
9. Faz 22 — Sürekli kaynak keşfi ve kapsama sağlığı
10. Faz 23 — Ertelenmiş analitik ve açık geri bildirimle kişiselleştirme

Her faz için uygulamadan önce somut test/commit planı ayrıca sunulur ve açık
kullanıcı onayı alınır. Bir fazın çıkış kriterleri kanıtlanmadan sonraki faza
geçilmez.

## Faz 14.1 — Kalıcılık, geçmiş ve scan hata sözleşmesi

Bu çalışma yeni özellik fazından önce gelen engelleyici stabilizasyon dilimidir;
Faz 15'in şirket kapsamını büyütmesinden önce tamamlanır.

### Gözlem ve karar (10 Ağustos 2026)

- Kayıp yalnız **deployed/canlı** örnekte gözlemlendi; yerelde önceki taramaların
  verisi duruyor. İlk şüphe deployment'ta **farklı SQLite yolu/volume**dir.
  Yaklaşım: migration veya sorgu değiştirmeden önce **runtime teşhis** (DB
  yolu/volume, satır sayıları, API status, `Content-Type`, güvenli yanıt örneği)
  kanıtla toplanır; düzeltme yolu ona göre netleşir.
- Kalıcı disk destekli **barındırma sağlayıcısı henüz seçilmedi**; 14.1'in
  redeploy/restore kalıcılık kanıtı bu seçime bağlıdır ve teşhis sonrası verilir.
- Fırsat **yaşam döngüsü** (`yeni/açık/incelendi/başvuruldu/süresi_doldu/
  kapatıldı/arşivlendi`) ile mevcut `application_tracking.status`
  (`incelenecek/basvuruldu/sinav_mulakat`) **ayrı iki kavram** olarak modellenir:
  biri sistem/kaynak durumu, biri kullanıcının başvuru tarafı; tek durum
  makinesinde birleştirilmez.

### Kapsam

- Kullanıcının “önceki gün taranan kaynak/fırsatlar görünmüyor” gözleminin DB,
  API ve UI katmanlarında kanıtla teşhisi.
- Yerel çalıştırma, container restart ve deployment sonrasında aynı SQLite
  dosya/volume kimliğinin kullanıldığının doğrulanması.
- Mevcut dashboard kovalarına girmeyen kayıtlar için “Tüm fırsatlar / Geçmiş”
  görünümü ve sayfalama/filtreleme sözleşmesi.
- Fırsat yaşam döngüsü: `yeni`, `açık`, `incelendi`, `başvuruldu`,
  `süresi_doldu`, `kapatıldı`, `arşivlendi`.
- Scan endpoint'inin başarı ve hata yanıtlarında JSON içerik türü sözleşmesi;
  proxy/HTML/plain-text hata yanıtında frontend'in güvenli mesaj göstermesi.
- Aynı DB ile restart/redeploy ve backup/restore kabul testi; listing,
  opportunity membership, analiz, başvuru durumu, tarih ve notların korunması.

### Karar kapıları

- **Kök neden kanıtı:** Veri gerçekten siliniyor mu, farklı DB mi açılıyor,
  yoksa kayıt yalnız dashboard filtresinin dışında mı kalıyor? Migration veya
  sorgu değiştirilmeden önce runtime DB yolu/volume, satır sayıları, API status,
  `Content-Type` ve güvenli yanıt örneği kaydedilir.
- **Saklama:** Normalize fırsat ve kullanıcı başvuru geçmişi süresiz saklanır.
  Ham HTML/e-posta gövdeleri bu karara dahil değildir ve varsayılan olarak
  kalıcılaştırılmaz.
- **Arşiv:** Arşivlemek silmek değildir. Fiziksel silme özelliği bu fazın
  kapsamına alınmaz.

### Çıkış kriteri

Önceki günün fırsatı “Geçmiş” ekranından bulunur; process/container restart ve
deployment provasında kimliği ile kullanıcı verileri değişmez. Scan'in JSON ve
JSON olmayan hata senaryoları parse exception yerine anlaşılır hata gösterir.

## Faz 15 — Birincil şirketlerin tamamlanması

### Kapsam

Spec'teki birincil grup: Havelsan, Akdoğan Tech, Meteksan, Baykar, Turkcell,
Türk Telekom, Jotform, Samsung, Commensis/Commencis, Akınsoft, Roketsan ve
ASELSAN.

- Her şirket için kanonik kimlik, resmî kariyer/program/duyuru URL'leri ve varsa
  resmî ATS/akışların doğrulanması.
- Kaynakların güven sırası: resmî şirket kaynağı → resmî ATS/feed → doğrulanmış
  şirket bülteni. Toplayıcı veya topluluk kaynağı tek başına yüksek güvenli
  bildirim üretmez.
- Her kaynağın `robots`, `public_api`, `manual_only` veya ileriki `feed`
  sınıfına ayrılması.
- İzinli otomatik kaynakların mevcut adapter'larla veya fixture-first yeni
  adapter/reçeteyle etkinleştirilmesi.
- Otomatik erişilemeyen şirketlerin sessizce “tamamlandı” sayılmadan manuel
  watchlist ve açık gerekçeyle gösterilmesi.
- Şirket/kaynak kapsama durumunun **bu fazda minimal bir kapsama raporu/
  endpoint** ile görünür olması (otomatik/akış/manuel/araştırılıyor/bozuk ayrı
  sayılır); tam kapsama dashboard'u Faz 22'de zenginleştirilir (karar 10 Ağustos
  2026, 22'ye kadar beklenmez).

### Karar kapıları

- **Şirket kimliği:** Spec'teki “Commensis” ile mevcut “Commencis” kaydının aynı
  şirket olup olmadığı doğrulanır; kullanıcı onayı olmadan iki şirket birleştirilmez.
- **Program modeli:** Turkcell gibi tekil ilan yerine dönemsel program açan
  kaynak için sentetik listing mi, ayrı program penceresi mi kullanılacağı **bu
  fazda karara bağlanır** (Faz 19'a ertelenmez; karar 10 Ağustos 2026). Turkcell
  birincil grupta olduğundan tamamlanması bu şema seçimine bağlıdır.
- **Kapsama kabulü:** Manuel kaynak katalog kapsamına dahil olabilir, fakat
  otomatik kapsama yüzdesine dahil edilmez.
- **Canlı doğrulama:** Normal testler fixture/fake kalır. Resmî kaynağa opt-in
  canlı kabul gerekiyorsa hedef URL ve çağrı bütçesi kullanıcı onayıyla belirlenir.

### Çıkış kriteri

On iki kanonik birincil şirketin tamamı doğrulanmış kaynak ve ayrı kapsama
durumuyla uygulamadadır. İzinli otomatik kaynaklar uçtan uca taranır; manuel veya
araştırma gerektirenler açıkça ayrılır. Yüksek güvenli ve güçlü eşleşen birincil
fırsat tek push üretir, diğer adaylar bildirim üretmeden Fırsatlar'a gider.

## Faz 16 — İkincil şirketlerin tamamlanması

### Kapsam

İnova, İntertech, ÜşüSebit, DenizBank, Mobiliz, AI Studio, Belsis, Evreka,
Viseur AI, MechSoft AI, Layermark, Actioner, Bilishim ve Otsimo için Faz 15'teki
aynı kaynak doğrulama, erişim sınıflandırma, fixture ve kapsama raporu uygulanır.

### Karar kapıları

- ÜşüSebit, AI Studio ve Bilishim'in kesin ticari kimliği/domain'i kullanıcıya
  kanıtlarıyla sunulur; onay gelmeden kaynak aktive edilmez.
- Banka/kariyer platformlarında hesap veya oturum gerektiren yol kullanılmaz;
  resmî açık ATS, RSS, bülten veya manuel yol seçilir.
- İkincil fırsatlar varsayılan olarak anlık push üretmez. Ancak güçlü profil
  eşleşmesi ve yüksek kaynak güveni birlikte sağlanırsa aynı bildirim kuralına
  girebilir; sayısal eşik bu fazın fixture sonuçlarıyla onaylanır.
- **Eşleşme modeli (karar 10 Ağustos 2026):** Bu faz için "güçlü eşleşme"
  ölçütü **basit tutulur** — mevcut boolean `focus_areas` eşleşmesi + sabit
  güven (`internal/analyzer/deterministic.go`). Skaler eşleşme skoru ve
  kullanıcının gerçek ilgisini temsil eden fixture eval altın kümesi işi **Faz
  19'a ertelenir**; sayısal eşik gevşetme kararı orada verilir.

### Çıkış kriteri

On dört ikincil şirketin tamamı kanonik kimlik, doğrulanmış kaynak ve dürüst
kapsama durumuyla uygulamadadır; otomatik ve manuel oranları ayrı raporlanır.

## Faz 17 — Üçüncül şirket araştırması

### Kapsam

- Bilkent Cyberpark, ODTÜ Teknokent ve Hacettepe Teknokent şirketlerinin
  araştırılması.
- **Dar kapsam (karar 10 Ağustos 2026):** Teknokent başına yalnız **yazılım/BT
  odaklı** ve **geçmişte staj/açık pozisyon sinyali** olan şirketler; her
  teknokent için **üst sınır ≈ 15-20 aday**. Amaç sonlu, tekrarlanabilir bir
  rapor; tüm teknokent firmalarını tarayan geniş bir liste hedeflenmez.
- Şirket kimliği, resmî domain, faaliyet alanı, teknoloji sinyalleri, geçmiş
  staj/fırsat izi, kaynak türü ve erişim uygunluğundan aday katalog üretilmesi.
- Adayların `önerilen`, `düşük_sinyal`, `kimlik_belirsiz`, `erişim_manuel`
  durumlarıyla sunulması.

### Değişmez sınır ve karar kapısı

Bu faz yalnız araştırmadır. Bulunan şirketler otomatik olarak production
config'ine, tarama programına veya bildirim sistemine eklenmez. Araştırma raporu
sonunda kullanıcı hangi şirketlerin onaylandığını ve önceliğini açıkça seçer.

### Çıkış kriteri

Kaynak bağlantıları ve değerlendirme gerekçeleri bulunan, tekrar üretilebilir
bir aday şirket raporu ve kullanıcı onay listesi oluşur; uygulama davranışı
değişmez.

## Faz 18 — Onaylanan üçüncül şirketlerin eklenmesi

### Kapsam

- Faz 17'de kullanıcı tarafından onaylanan şirketlerin küçük batch'lerle
  canonical katalog ve kaynak config'ine eklenmesi.
- Her batch için erişim politikası, fixture/fake kabulü, kapsama sağlığı ve
  yanlış bildirim guard'ları.
- Kaynağı olmayan şirketin manuel/araştırılıyor kalması; uydurma veya doğrulanmamış
  URL kullanılmaması.

### Karar kapıları

- Batch boyutu ve şirket önceliği her batch başında onaylanır.
- Topluluk/toplayıcı kaynakları yalnız fırsat adayı üretir; resmî kanıtla
  desteklenmeden yüksek güvenli push üretmez.

### Çıkış kriteri

Onaylanan şirketler dürüst kapsama durumlarıyla çalışır; kaynak sayısındaki artış
dedup, domain bütçesi veya diğer şirketlerin hata izolasyonunu bozmaz.

## Faz 19 — Genel fırsat modeli ve bildirim katmanları

### Kapsam

- Staj, uzun dönem staj, part-time öğrenci pozisyonu, yeni mezun programı,
  bootcamp, hackathon, yarışma, burs, üniversite–şirket programı ve teknik
  etkinlik/eğitim için ortak fırsat modeli.
- İlan/program açılış-kapanış aralığı, son başvuru ve etkinlik tarihleri.
- Kaynak URL, kaynak türü, ilk/son gözlem zamanı ve güncellik kanıtı.
- Web, RSS ve ileriki e-posta sinyallerinin aynı kanonik fırsata dedup edilmesi.
- Görünürlük katmanları:
  - `bildirim`: yüksek kaynak güveni + yüksek çıkarım güveni + güçlü profil uyumu
  - `firsatlar`: makul aday, push yok
  - `incelenecek`: düşük güven veya eksik kanıt
  - `elenen`: gürültü; audit için minimum gerekçe

### Karar kapıları

- Fırsat türü taksonomisi ve program penceresinin listing'den ayrı olup
  olmayacağı şema yazılmadan önce onaylanır.
- Bildirim/eşik değerleri gerçek kullanıcı ilgilerini temsil eden fixture eval
  setiyle ölçülür. Yalnız modelin kendi confidence alanı karar için yeterli
  kabul edilmez. **Skaler eşleşme skoru ve fixture eval altın kümesi bu fazda
  hayata geçer** (Faz 16'da basit boolean eşleşmeden bilinçli olarak ertelendi;
  karar 10 Ağustos 2026). Altın kümenin nasıl/kimden üretileceği bu faz başında
  ayrıca kararlaştırılır.
- Kaçırmayı azaltmak için belirsiz adaylar silinmez; `incelenecek` kuyruğuna
  düşer. False-positive oranı raporlanmadan eşik gevşetilmez.

### Çıkış kriteri

Her desteklenen fırsat türü kaynak kanıtıyla kalıcıdır; güçlü eşleşme tek push,
diğer makul adaylar yalnız Fırsatlar/İncelenecek görünümü üretir.

## Faz 20 — RSS/Atom ve açık akışlar

### Kapsam

- RSS 2.0 ve Atom; gerçek ihtiyaç doğrulanırsa JSON Feed.
- Feed URL keşfi, ETag/Last-Modified ve kalıcı checkpoint.
- Yeni, güncellenen ve tekrar gelen öğelerin ayrılması.
- Feed öğelerinin Faz 19 fırsat modeline normalize edilmesi ve web sinyalleriyle
  dedup edilmesi.
- Bozuk feed'in diğer kaynakları etkilememesi ve manuel kontrol görünürlüğü.

### Karar kapıları

- Otomatik feed discovery yalnız aynı resmî domain veya açıkça doğrulanmış feed
  bağlantısıyla sınırlıdır.
- JSON Feed yalnız gerçek kaynak gerektiriyorsa eklenir; varsayımsal destek için
  kapsam büyütülmez.
- Feed içeriği tam gövde olarak süresiz tutulmaz; kimlik/checkpoint ve normalize
  fırsat kanıtı saklanır.

### Çıkış kriteri

Fixture RSS/Atom akışları restart sonrasında tekrar bildirim üretmeden yeni ve
güncellenen fırsatları ortak modele aktarır.

## Faz 21 — Ayrı posta kutusuyla güvenli e-posta fırsat akışı

### Kapsam

- Yalnız bu proje için açılan ayrı posta kutusu.
- Kullanıcıyla seçilen şirket bülteni, iş alarmı ve fırsat bültenlerine kullanıcı
  tarafından abonelik.
- Salt okunur OAuth erişimi; şifre/basic-auth saklanmaması.
- Yalnız seçilen klasör/etiketin okunması.
- Mesaj kimliği/hash checkpoint'i ve fırsat çıkarımı; web/RSS ile dedup.
- Ham mesaj yerine normalize fırsat, kaynak gönderici, konu, mesaj zamanı,
  güvenli referans kimliği ve çıkarım sürümünün saklanması.

### Karar kapıları

- **Sağlayıcı:** Gmail API, Outlook Graph veya IMAP OAuth seçenekleri faz
  başında bakım maliyeti, ücretsiz kota, en dar yetki ve yerel/deployment
  uygunluğuyla araştırılır; seçim kanıtıyla kullanıcıya sunulur.
- **Retention:** Ham e-posta gövdesi varsayılan olarak kalıcı saklanmaz. Hata
  ayıklama için geçici şifreli cache istenirse süre ve silme davranışı ayrıca
  onaylanır.
- **Kapsam:** Başvuru sonucu/kişisel yazışma yorumlanmaz; yalnız fırsat bülteni
  ve alarm mesajları işlenir.
- Uygulama kullanıcı adına bültene abone olmaz, e-posta göndermez veya mesaj
  silmez/işaretlemez.

### Çıkış kriteri

Seçili klasördeki fixture/fake-provider mesajı tek normalize fırsata dönüşür;
restart aynı mesajı tekrar işlemez, ham gövde DB/log/fixture artefaktında kalmaz
ve OAuth token'ı secret store dışında bulunmaz.

## Faz 22 — Sürekli kaynak keşfi ve kapsama sağlığı

- Yeni şirket/kaynak adayları için periyodik, onay kapılı araştırma akışı.
- Kayıtlı şirket, otomatik kaynak, feed/e-posta, manuel, bozuk ve araştırılıyor
  sayılarıyla kapsama dashboard'u.
- Uzun süredir sinyal vermeyen kaynak ile gerçekten sıfır fırsat üreten kaynağın
  ayrılması.
- Yeni şirket veya kaynak production'a kullanıcı onayı olmadan eklenmez.

## Faz 23 — Ertelenmiş analitik ve kişiselleştirme

Bu faz ancak yeterli gerçek başvuru geçmişi oluştuğunda planlanır. Şirket/fırsat
dönüşümleri, sıralama ve açık kullanıcı geri bildirimleri değerlendirilebilir.
Otomatik ve geri alınamaz tercih öğrenme varsayılan değildir; kullanılacak
sinyaller, açıklanabilirlik, düzeltme/silme ve veri saklama davranışı yeni bir
karar planıyla açıkça onaylanır.
