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

Tam kalite paketi sonuçları commit/push sonrasında bu kayda eklenecektir.
Production deploy ayrı kullanıcı onayı gerektirir ve henüz bu kabul kaydının
parçası değildir.
