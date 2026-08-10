# Faz 14.1 kabul kanıtı — 10 Ağustos 2026

Faz 14.1 normal testlerde canlı kariyer sitesi veya ücretli AI API çağrısı
yapmadan tamamlandı.

## Kök neden kanıtı

- Repository'de fırsat/listing silen bir scan yolu yoktu. Görünmezliğin yerel
  kod nedeni, dashboard sorgusunun yalnız uygun+açık, karar bekleyen veya aktif
  başvuru kovalarına giren kayıtları döndürmesiydi. Analizsiz, uygun olmayan ya
  da bu üç koşulun dışındaki kayıt SQLite'ta kalmasına rağmen PWA'da bulunamıyordu.
- PWA scan başarı ve hata gövdelerinde `Content-Type` kontrol etmeden doğrudan
  `response.json()` çağırıyordu. Proxy'nin HTML/plain-text cevabı bu nedenle
  `JSON.parse: unexpected character` hatasına dönüşebiliyordu.
- Production Compose, sabit `internship-tracker` proje adı ve `/app/data`
  altındaki `tracker_data` named volume sözleşmesini koruyor. Seçilmiş bir hosted
  runtime bu çalışma alanına bağlı olmadığı için eski canlı örneğin volume
  kimliği geriye dönük örneklenemedi; yeni `/app/dbinspect` ve operasyon komutları
  sonraki restart/redeploy öncesi ve sonrası DB yolu, mount ve satır sayılarını
  güvenli biçimde kanıtlamak için image'a eklendi.

## Davranış kabulü

- `011_opportunity_lifecycle.sql`, kanonikleştirme ve başvuru durumundan ayrı
  `yeni/acik/incelendi/basvuruldu/suresi_doldu/kapatildi/arsivlendi` yaşam
  döngüsünü ekledi.
- `GET /api/v1/opportunities`, dashboard kovalarından bağımsız tüm aktif kanonik
  fırsatları sayfalıyor; lifecycle, şirket ve başlık/özet filtresi uyguluyor.
- `PUT /api/v1/opportunities/{id}/lifecycle` yalnız izinli durumları kabul ediyor.
  Arşivlenen fırsat aynı kimlikle geçmişte kalıyor; fiziksel silme yok.
- PWA'da `Tüm fırsatlar / Geçmiş`, filtreleme, sayfalama ve detaydan lifecycle
  güncelleme çalışıyor.
- JSON API helper'ı HTML/plain-text proxy hatasını ayrıştırmıyor veya gövdeyi
  göstermiyor; status kodlu güvenli mesaj üretiyor. Backend scan başarı, `409`
  ve `500` yolları JSON içerik türünü koruyor.
- Gerçek SQLite kabulü aynı dosyayı kapatıp yeniden açtı ve `VACUUM INTO`
  snapshot'ından restore etti. Listing/fırsat kimliği, üyelik, analiz, lifecycle,
  başvuru durumu, iki tarih ve not değişmedi.

Kabul komutu:

```bash
go test ./internal/acceptance -run TestPhase141 -count=1 -v
```

Sonuç: PASS; scan `success`, `conflict` ve `failure` alt senaryoları da geçti.

## Kalite kapıları

- `go test ./... -count=1`: PASS
- `go vet ./...`: PASS
- `go test -race ./... -count=1`: PASS
- `go build ./cmd/api` ve `go build ./cmd/dbinspect`: PASS
- `govulncheck@v1.6.0 ./...`: çağrılan/import edilen kodda 0 güvenlik açığı
- `npm --prefix web test`: 4 dosya / 15 test PASS
- `npm --prefix web run typecheck`: PASS
- `npm --prefix web run build`: production bundle PASS (`/tmp` outDir)
- `npm --prefix web audit --audit-level=high`: 0 güvenlik açığı
- Docker Compose v5.1.4 ile production config render ve deployment contract: PASS
- İzlenen Go dosyalarında `gofmt -l`: çıktı yok; `git diff --check`: PASS

Docker daemon bu çalışma ortamında kullanılamadığı için image push veya çalışan
container mutasyonu yapılmadı. Backend image'ına eklenen `dbinspect` binary'si
Go build ile, Dockerfile kopyalama sözleşmesi ise deployment config/testleriyle
doğrulandı. Gerçek hosting seçildiğinde off-host yedek ve provider'a özgü
restart/redeploy işletim kaydı ayrıca tutulmalıdır.
