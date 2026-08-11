# Faz 16.5 kabul kanıtı — 11 Ağustos 2026

## Davranış kabulü

- On bir şirket `secondary` iş önceliğini koruyarak ayrı `tracking_phase=16.5`
  kohortuna taşındı.
- İnnova'nın resmî açık ilan kartları fixture-first deterministik adapter ile
  çıkarıldı; allowlist'teki LinkedIn başvuru hedeflerine hiçbir istek yapılmadı.
- Diğer on kaynak resmî/manual bağlantı, yapılandırılmış engel kodu, ayrıntılı
  gerekçe ve son doğrulama zamanıyla görünür kaldı.
- Migration 014, config → SQLite → coverage API → PWA boyunca takip fazı,
  araştırma metadata'sı ve birbirini dışlayan bölüm özetlerini taşır.

## Fixture kabul komutu

```bash
go test ./internal/acceptance -run 'TestPhase16' -count=1 -v
```

Sonuç: `TestPhase165ResearchCohortAndOfficialInnovaIndexEndToEnd` ve mevcut Faz
16 regresyonu geçti. Faz 16.5 özeti 11 şirket, 1 otomatik, 4 manuel, 6
araştırılıyor ve yaklaşık `%14,3`; normal ikincil bölüm dört otomatik şirkettir.

## Tam kalite ve production

Doğrulanan davranış revision'ı:
`27277f68db9fa5a0e03008faa8d9b1c593bf0ea8` (upstream `main`).

- `go test ./... -count=1`: PASS
- `go test -race ./... -count=1`: PASS
- `go vet ./...`: PASS
- API, backup, restorecheck, dbinspect ve vapidkey build'leri: PASS
- İzlenen Go dosyalarında `gofmt -l`: çıktı yok; `git diff --check`: PASS
- `govulncheck@v1.6.0 ./...`: çağrılan/import edilen kodda 0 açık
- Temiz `npm ci`, frontend testleri (5 dosya/19 test), typecheck ve production
  build: PASS
- `npm audit --audit-level=high`: 0 açık
- Production contract, deploy runtime, exact-commit workflow ve preflight izin
  testleri: PASS. Docker'ın `/tmp` namespace ayrımı nedeniyle contract testleri
  workspace altındaki mutlak geçici dizinle çalıştırıldı.
- Backend ve frontend gerçek Docker image build'leri: PASS
- Production'dan farklı `internship-tracker-phase165-qa` proje adıyla gerçek
  development Compose `up --build --wait`: API ve web healthy; `/health`,
  `/ready` ve proxied `/api/v1/coverage`: PASS
- Canlı coverage: toplam 27 şirket/29 kaynak/%52,38; birincil 12/14/%60;
  öncelik olarak ikincil 15/15/%45,45. Görünür normal ikincil bölüm 4/4/%100,
  Faz 16.5 bölümü 11/11, 1 otomatik, 4 manuel, 6 araştırılıyor/%14,29.
- Salt okunur `dbinspect`, QA volume'unda 27 şirket, 29 kaynak ve 14 uygulanmış
  migration doğruladı. QA container/network volume silinmeden kapatıldı;
  kalıcı kimlik `internship-tracker-phase165-qa_tracker_data` olarak korundu.

## Production deploy kanıtı

Kullanıcı ayrı production onayını verdikten sonra doğrulanmış davranış revision'ı
`27277f68db9fa5a0e03008faa8d9b1c593bf0ea8` yerel Docker + Cloudflare Tunnel
production ortamına deploy edildi.

- Pre-deploy DB kimliği 27 şirket, 29 kaynak, 8 listing, 8 fırsat, 8 üyelik,
  8 analiz, 3 scan ve 13 migration olarak kaydedildi.
- Tutarlı snapshot
  `internship-tracker-20260811T154910.407859512Z-7be359b75dff9692.db`
  oluşturuldu; mevcut production image'ının uygulanmış 13 migration sözleşmesi
  ve salt okunur bütünlük kontrolüyle doğrulandı.
- Candidate restorecheck snapshot'ta henüz uygulanmamış migration 014'ü beklediği
  için fail-closed durdu ve DB'ye yazmadı. Candidate startup migration 014'ü
  uyguladı; ardından `/ready` ve `dbinspect` 14 migration'ı doğruladı.
- API ve web exact revision etiketlerinde healthy; iç `/health` ve `/ready`
  smoke başarılıdır.
- Kalıcı `internship_tracker_data` volume kimliği değişmedi. Deploy sonrası
  listing/fırsat/üyelik/analiz sayıları `8/8/8/8`, şirket/kaynak `27/29` olarak
  korundu.
- Canlı coverage Faz 16.5 bölümünde 11 şirket, 1 otomatik, 4 manuel, 6
  araştırılıyor ve `%14,29`; normal ikincil bölümde 4/4 otomatik kaynak döndürdü.
- Cloudflare dış `/ready` isteği HTTP/2 üzerinden Access'ın beklenen `302`
  yanıtını verdi; tunnel `TUNNEL_TRANSPORT_PROTOCOL=http2` ayarını korudu.
- `current-local.env` exact Faz 16.5 revision'ına, `previous-local.env` önceki
  Faz 16 revision'ı `b66c5b2c110c179b2e2052b04fe187bb9ce1b061` değerine atomik
  geçirildi.
