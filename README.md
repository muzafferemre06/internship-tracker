# Internship Tracker

Kişisel staj ilanı takip ve başvuru hatırlatma uygulaması. Backend Go,
istemci ise React/Vite tabanlı bir PWA olarak yapılandırılmıştır.

Ürün kararları için [v2 spec](./staj-takip-spec-v2.md), ilk fikir belgesi için
[ilk spec](./staj-takip-spec-initial.md) kullanılmalıdır.

## Gereksinimler

- Go 1.22+
- Node.js 20+
- npm 10+
- Docker ve Docker Compose (isteğe bağlı)

## Yerel geliştirme

```bash
cp .env.example .env
cp configs/candidate-profile.example.json configs/candidate-profile.json
cp configs/sources.example.json configs/sources.json
go run ./cmd/api
```

Başka bir terminalde:

```bash
npm --prefix web install
npm --prefix web run dev
```

API varsayılan olarak `http://localhost:8080`, PWA ise
`http://localhost:5173` adresinde çalışır.

API başlangıçta `DATABASE_PATH` altındaki SQLite dosyasını açar ve
`MIGRATIONS_PATH` içindeki uygulanmamış `.sql` dosyalarını alfabetik sırayla,
transaction içinde uygular. Uygulanan dosyalar `schema_migrations` tablosunda
izlenir.

Uygulama aday profili ve kaynak dosyalarını katı bir JSON şemasıyla okur;
bilinmeyen alanlar ve geçersiz şirket/kaynak değerleri başlangıç hatasıdır.
Dosya yolları `CANDIDATE_PROFILE_PATH` ve `SOURCES_PATH` ile değiştirilebilir.
Her kaynak, taramalar ve veritabanı kayıtları arasında değişmeyen benzersiz bir
`id` alanına sahip olmalıdır. Aynı şirket birden fazla `sources[]` girdisiyle
izlenebilir. İştirak profilinin sayfa başlığı ana şirket adından farklıysa
selector doğrulaması için kaynakta `page_name` belirtilir.

## Test

```bash
go test ./...
npm --prefix web test
```

Gerçek OpenRouter çağrıları normal test akışına dahil değildir. API anahtarları
yalnızca yerel `.env` veya deployment secret sistemi üzerinden verilmelidir.

## Docker ile çalıştırma

```bash
docker compose -f deploy/compose.yml up --build
```

PWA bu kurulumda `http://localhost:8081` adresinden açılır. Compose dosyası
yerel `configs/candidate-profile.json` ve `configs/sources.json` dosyalarını
salt okunur bağlar; SQLite verisini adlandırılmış bir volume içinde korur.

## API

- `GET /health`: süreç sağlık bilgisi
- `GET /api/v1/dashboard`: uygun yeni ilanlar, takip özetleri ve kalıcı son
  tarama raporu
- `POST /api/v1/scan`: etkin kaynakları hemen tarar; toplam bulunan/yeni ilan
  sayılarını, kalıcı tarama kimliğini/durumunu ve kaynak bazlı hataları döndürür

Manuel tarama tamamlandıktan sonra PWA dashboard'u yeniden yükler. Bir kaynak
hatası diğer kaynakların çalışmasını durdurmaz; kısmi sonuç HTTP `207` ile
döner. Her kaynak için son başarı zamanı veya zaman damgalı kısa hata nedeni
SQLite'ta tutulur.

## Dizinler

```text
cmd/api/             API uygulamasının giriş noktası
internal/            Backend domain ve uygulama katmanları
migrations/          SQLite migration dosyaları
configs/             Secret içermeyen örnek kaynak ayarları
web/                 React/Vite PWA
deploy/              Container ve deployment dosyaları
docs/                Mimari ve geliştirme notları
```

Güncel faz durumu ve sıradaki uygulama işi için
[`docs/progress.md`](./docs/progress.md) kullanılmalıdır.
