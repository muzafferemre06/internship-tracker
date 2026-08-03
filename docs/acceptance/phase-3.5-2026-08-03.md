# Faz 3.5 gerçek ilan kabul kanıtı

## Resmî kaynak ve aktiflik

- Erişim zamanı: `2026-08-03T19:35:29Z` (`2026-08-03 22:35:29 TRT`)
- Şirket: Commencis
- Resmî ATS: Lever
- Kaynak ve canonical URL:
  `https://jobs.lever.co/commencis/04a5cd98-ab26-4b48-bb64-3397ffe79a55`
- Kısa aktiflik kanıtı: `Spring Boot Development Camp 2026` sayfası `Intern`,
  `Remote`, `Istanbul, Turkey`, `Application Deadline: August 14, 2026` ve aynı
  ilana ait aktif `/apply` bağlantısını taşıdı.
- Erişim politikası: `jobs.lever.co/robots.txt` genel erişime izin verip bir
  saniyelik crawl aralığı bildiriyordu. Adapter aynı aralığı kalıcı domain
  bütçesiyle uyguladı; koruma veya erişim engeli aşılmadı.

Canlı HTML gövdesi kaydedilmedi. Adapter yalnızca başlık, kategoriler, iş tanımı
ve gereksinimleri normalize etti.

## Uçtan uca sonuç

Opt-in `TestPhase35LiveOfficialListingWithGemini` testi resmî adapter,
orchestrator, gerçek migration/SQLite repository, canlı Google sağlayıcısı ve
dashboard HTTP handler'ını aynı geçici veritabanında çalıştırdı.

| Kanıt | Sonuç |
| --- | --- |
| İlk tarama | bulunan `1`, yeni `1`, işleme hatası `0` |
| İkinci tarama | bulunan `1`, yeni `0`, işleme hatası `0` |
| Kalıcı ilan sayısı | `1` |
| Provider | `google` |
| Model | `gemini-3.1-flash-lite` |
| Prompt/completion/toplam token | `1087 / 256 / 1343` |
| Tahmini maliyet | `$0.00065575` |
| Uygunluk | `karar_bekliyor` |
| Dashboard API | ilan `needs_decision` listesinde göründü |

Karar sorusu:

> İlan Eylül 2026'da başlıyor. Programa başladığınızda 3. sınıfa geçmiş olacak
> mısınız?

Bu sonuç, tarama anında 2. sınıf olan aday ile sonraki akademik dönemde başlayan
ve 3. sınıf şartı taşıyan program arasındaki gerçek belirsizliği kullanıcıya
bırakır; ilan sessizce elenmez.

## Tekrarlanabilirlik ve secret güvenliği

Normal `go test ./...` aynı yolu küçük Lever fixture'ı, strict fake provider ve
geçici SQLite ile ağ çağrısı yapmadan doğrular. Canlı komut yalnızca açık
`RUN_REAL_LISTING_ACCEPTANCE=1`, `integration` build etiketi ve ortamdan verilen
`GEMINI_API_KEY` ile çalışır.

API anahtarı, kişisel kurum adları, canlı sayfa gövdesi ve geçici SQLite dosyası
repoya veya test çıktısına yazılmadı. Test sonlandığında geçici veritabanı
silindi.
