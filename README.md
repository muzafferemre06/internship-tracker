# Internship Tracker

Kişisel staj ilanı takip ve başvuru hatırlatma uygulaması. Backend Go,
istemci ise React/Vite tabanlı bir PWA olarak yapılandırılmıştır.

Ürün kararları için [v2 spec](./staj-takip-spec-v2.md), ilk fikir belgesi için
[ilk spec](./staj-takip-spec-initial.md) kullanılmalıdır.

## Gereksinimler

- Go 1.22+
- Node.js 20.19+
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

`lever` adapter'ı herkese açık resmî `https://jobs.lever.co/<şirket>/<ilan>`
sayfasındaki tek ilanı izler. Yalnızca aktif başvuru bağlantısı bulunan sayfaları
kabul eder; takip parametrelerini kaynak URL'sinden çıkarır ve başlık, ilan
kategorileri ile açıklama alanlarını normalize eder. Örnek kaynak dosyasında
Commencis'in resmî Lever ilanı bu adapter'ın yapılandırmasını gösterir.

## İlan analizi

Varsayılan `LLM_PROVIDER=deterministic` ayarı API anahtarı veya ağ çağrısı
gerektirmez. OpenRouter analizi için yerel `.env` dosyasında aşağıdaki alanlar
ayarlanır:

```dotenv
LLM_PROVIDER=openrouter
LLM_MODEL=provider/model-name
OPENROUTER_API_KEY=local-secret
LLM_INPUT_COST_PER_MILLION_USD=0
LLM_OUTPUT_COST_PER_MILLION_USD=0
```

Google Gemini API'yi doğrudan kullanmak için:

```dotenv
LLM_PROVIDER=google
LLM_MODEL=gemini-3.1-flash-lite
GEMINI_API_KEY=local-secret
LLM_THINKING_LEVEL=minimal
LLM_INPUT_COST_PER_MILLION_USD=0
LLM_OUTPUT_COST_PER_MILLION_USD=0
```

Model ve sağlayıcı yalnızca backend başlangıcında seçilir. OpenRouter veya Google
seçiliyken model adı ve ilgili API anahtarı zorunludur; maliyet oranları negatif
olmayan USD değerleri olmalıdır. `LLM_PROVIDER=gemini`, `google` için alias'tır.
Google adapter'ı Gemini 3 modellerinde `minimal`, `low`, `medium` veya `high`
düşünme seviyesini kullanır; Gemma modellerine Gemini'ye özgü bu alanı göndermez.
Google istekleri, düşünmeli modellerin gecikmesini karşılamak için 60 saniyelik
istemci timeout'u kullanır.
Gerçek model fiyatları değişebildiği için milyon input ve output token başına
oranlar seçilen modelin güncel fiyatıyla kullanıcı tarafından ayarlanır. Anahtar
ve yerel `.env` repoya eklenmez.

Modele adayın adı, iletişim bilgileri, üniversite adı veya deneyim kurumları
gönderilmez. Yalnızca bölüm/alan, sınıf, GPA, odak alanları, deneyim konu başlıkları
ve konum tercihleri; ilan başlığı ve metniyle birlikte gönderilir.

## Test

```bash
go test ./...
npm --prefix web test
```

Gerçek OpenRouter ve Google çağrıları normal test akışına dahil değildir. API
anahtarları yalnızca yerel `.env` veya deployment secret sistemi üzerinden
verilmelidir. Google adapter'ının opt-in canlı testi açıkça şöyle çalıştırılır:

```bash
GEMINI_API_KEY=... go test -tags=integration ./internal/analyzer \
  -run TestGoogleProviderLive -v
```

Test varsayılan olarak `gemini-3.1-flash-lite` kullanır;
`GEMINI_LIVE_TEST_MODEL` ile model değiştirilebilir. Bu komut gerçek API kotası
kullanır ve anahtar yoksa testi atlar.

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
- `POST /api/v1/analyses/retry`: en fazla 25 `pending` analizi, kaynak siteye
  yeniden bağlanmadan saklanan ham ilan metni üzerinden işler; işlenen ve tekrar
  başarısız olan kayıt sayılarını döndürür

Manuel tarama tamamlandıktan sonra PWA dashboard'u yeniden yükler. Bir kaynak
hatası diğer kaynakların çalışmasını durdurmaz; kısmi sonuç HTTP `207` ile
döner. Her kaynak için son başarı zamanı veya zaman damgalı kısa hata nedeni
SQLite'ta tutulur.

Kariyer.net kaynakları aynı domain erişim bütçesini paylaşır. Başarılı veya
başlatılmış iki Kariyer.net taraması arasında en az 24 saat bırakılır. İlk
403/429/challenge yanıtı kalan Kariyer.net profillerini çağırmadan durdurur;
cooldown ve en erken tekrar zamanı SQLite'ta kalır ve manuel tarama düğmesiyle
aşılamaz. API/PWA atlanan kaynakları ve tekrar zamanını gösterir.

Model cevabı JSON Schema ile istenir ve backend'de bilinmeyen alanları da reddeden
aynı katı sözleşmeyle doğrulanır. Bozuk JSON, şema hatası, timeout, 429 ve geçici
5xx cevapları en fazla üç denemeyle sınırlıdır.
Analiz yine başarısızsa ham ilan silinmez; `karar_bekliyor` ve `pending` olarak
hata/retry bilgisiyle saklanır. Başarılı analiz provider, model,
prompt/completion/toplam token ve yapılandırılan oranlardan hesaplanan tahmini
USD maliyetiyle kalıcılaştırılır.

Zamanlanmış taramayı ayrı bir sunucuda çalıştırmak ev bağlantısını otomasyon
trafiğinden izole eder, ancak erişim izni sağlamaz ve veri merkezi IP'leri de
engellenebilir. Aynı bütçe/devre kesici sunucuda korunmalı; proxy rotasyonu veya
challenge aşma yöntemi olarak kullanılmamalıdır.

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
