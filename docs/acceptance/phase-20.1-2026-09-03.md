# Faz 20.1 kabul kaydı — 3 Eylül 2026

Kapsam dışı bir kullanıcı gözlemiyle başladı: panelde Brezilya'daki bir ilan
görünüyordu. Kök neden araştırması iki ayrı kusur ortaya çıkardı; bu kayıt her
ikisinin düzeltmesini ve production sonucunu belgeler.

## Bulgular

- **Yurtdışı filtresi etkisizdi.** `2d35473` ile eklenen
  `foreign_non_remote_location` kapısı `location` alanına bakıyordu; ülke ise
  ilan başlığındaydı. Kayıtlı `location` değeri `Belirtilmemiş` olduğu için
  `isDomesticLocation` bunu yurt içi sayıyor ve kapı hiç açılmıyordu.
- **Production'da hiç LLM çalışmamıştı.** `runtime.env` içinde
  `LLM_PROVIDER=deterministic` idi ve `secrets/api/` dizini yoktu. 193 analizin
  tamamı `provider=deterministic`, `total_tokens=0` idi.
- **Deterministik analizör production için uygun değildi.** `opportunityType`
  sayfa metninde `"intern"` alt dizesini arıyordu; `international` ve `internal`
  gibi boilerplate kelimeler kıdemli rolleri staj yapıyordu. `location` yalnız
  üç değer üretebiliyordu (Ankara / Uzaktan / Belirtilmemiş) ve DB dağılımı tam
  olarak buydu: 132 / 36 / 25. Analizör kendine `Confidence: 0.85` veriyordu;
  `matching.MinimumConfidence` 0.80 olduğu için keyword tahminleri push
  katmanına ulaşabiliyordu.
- **İki çelişen bildirim kapısı vardı.** `matching.Assess`, tür kontrolü
  (`notificationOpportunity`) nedeniyle `diger` türündeki bir ilanı `firsatlar`
  katmanında bırakıyordu; ancak `applyTrustedNotificationLayer` güvenilir
  kaynaktan gelen 80+ puanlı her ilanı bu kontrolü atlayarak `bildirim`e terfi
  ettiriyordu. Sonuç: "Çözüm Mimarı" gibi kıdemli tam zamanlı roller push
  bildirimi üretebiliyordu.

## Değişiklikler

- `c5efe7a` `cmd/reassess`: saklanan analizleri deterministik eşleştirme
  kurallarıyla yeniden skorlar. Model çağırmaz, bildirim kuyruğa atmaz,
  varsayılanı kuru çalıştırmadır.
- `8d9c023` deterministik analizör sertleştirildi: kelime sınırlı eşleşme
  (`containsWord`), rol türü yalnız başlıktan, kıdem kapısı (`isSeniorRole`),
  gerçek ülke/il tanıyan `location`, ve `Confidence` 0.85 -> 0.6. Son değişiklik
  keyword çıkarımının push katmanına ulaşmasını yapısal olarak engeller.
- `abe7150` `reassess -invalidate`: kayıtları `pending` durumuna alarak mevcut
  retry yolunun yapılandırılmış sağlayıcıyla yeniden çıkarım yapmasını sağlar.
  Yalnız `processing_status` değişir; önceki değerler başarılı bir yeniden
  analiz onları ezene kadar yerinde kalır.
- `aab9dfd` `applyTrustedNotificationLayer` artık
  `matching.NotificationOpportunity` kapısına bağlı. İki kapı tek politikada
  birleşti; saf karar `promotableToNotification` olarak ayrıldı ve DB'siz test
  edilir.

## Kalite

- Go 1.26 container'ında `gofmt`, `go vet` ve `go test ./...` tüm paketlerde
  yeşil (host Go 1.18 olduğundan testler `golang:1.26.5-alpine` içinde koştu).
- `TestNotificationRequiresHighTrustSource` fixture'ı düzeltildi:
  `suitablePrimaryAnalysis()` `OpportunityType` set etmiyordu, bu yüzden yeni
  tür kapısı onu haklı olarak reddetti. Fixture eksikti, kapı değil.
- Canlı sağlayıcı doğrulaması: `gemini-3.1-flash-lite` gerçek anahtarla
  `TestGoogleProviderLive` üzerinden geçti (418 token, 3.7 sn).

## Production sonucu

- Deploy revision: `aab9dfdeafef129f74e25129c7be12d28ae47e54`; rollback hedefi
  `state/previous-local.env` içinde `abe71505...` olarak korunur.
- Pre-deploy snapshot'lar:
  `internship-tracker-20260903T192544.970962266Z-a699fdd82f30a509.db` ve
  `internship-tracker-20260903T195441.643550797Z-1dec981650ca8fa0.db`.
- `LLM_PROVIDER=google`, `LLM_MODEL=gemini-3.1-flash-lite`. Anahtar
  `secrets/gemini_api_key` dosyasında (owner UID 100 = container `app`, grup
  deploy hesabı, mode 0640) ve yalnız `GEMINI_API_KEY_FILE` üzerinden okunur;
  env'de secret değeri bulunmaz.
- 193 kaydın tamamı Gemini ile yeniden analiz edildi: 127.617 token, 0,00 USD
  (free tier). Yeniden analiz free-tier dakika limiti nedeniyle tempolu
  çalıştırıldı; 429 alan kayıtlar `pending` kaldığı için veri kaybı olmadı.
- Kalıcı volume kimliği değişmedi (`internship_tracker_data`). 41 şirket,
  43 kaynak, 193 ilan, 193 analiz, 16 migration korundu.
- Katman dağılımı: `elenen` 166, `incelenecek` 27, `firsatlar` 10,
  **`bildirim` 0**. Yeniden analiz gerçek lokasyon çıkardığı için kanonik fırsat
  sayısı 195'ten 203'e çıktı (dedup anahtarı normalize lokasyonu içerir).
- Brezilya ilanlarının tamamı artık `elenen` / `foreign_non_remote_location`.
  Udemy kaynağındaki kıdemli tam zamanlı roller bildirim katmanından çıktı.
- Yeniden analiz sırasında yeni bildirim üretilmedi (`notifications` tablosu 6
  kayıtta sabit kaldı): mevcut kayıtların `first_processed_at` alanı dolu
  olduğundan ilk-analiz koşulu sağlanmaz.

## Kapsam dışı bırakılanlar

- **İçerik değişince otomatik yeniden analiz hâlâ yok.** `AnalysisRequired`
  yalnız "hiç analiz edilmiş mi" bakar, içerik hash'ine bakmaz. Bu faz manuel
  bir yeniden çıkarım aracı (`reassess -invalidate`) ekledi; otomatik tetikleme
  sistem geneli bir davranış değişikliği olarak açık kaldı.
- **Deterministik analizör hâlâ bir yedek yoldur.** Sertleştirildi ve artık
  push üretemez, ancak kalitesi model tabanlı çıkarımın yerini tutmaz.
- **`reassess` yalnız yeniden skorlar.** Kayıtlı çıkarım alanlarını düzeltmek
  için `-invalidate` ile yeniden analiz gerekir; bu ayrım bilinçlidir.
- **Ülke/şehir sözlüğü sonludur.** `location` tanınmayan bir yabancı şehri
  `Belirtilmemiş` olarak bırakır ve o ilan yurt içi sayılır. Model tabanlı
  çıkarım aktifken bu yol nadiren kullanılır.
