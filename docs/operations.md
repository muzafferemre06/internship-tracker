# Production operasyonları

Bu belge, SQLite tabanlı uygulamanın deploy öncesi snapshot, dış yedekleme ve
restore prova akışını tanımlar. Otomatik günlük snapshot uygulama içinden alınır;
tek seferlik `cmd/backup` ise deployment başlamadan önce aynı tutarlı snapshot'ı
üretir.

## Kalıcılık ve görünürlük teşhisi

Veri kaybı şüphesinde migration veya dashboard sorgusu değiştirmeden önce çalışan
release'in gerçek DB yolu ve volume'u kaydedilir. API başlangıç logu
`database_path` alanını yazar. Container içindeki güvenli read-only sayım:

```bash
current_revision=$(awk -F= '$1 == "DEPLOY_REVISION" {print $2}' state/current.env)
docker compose \
  --project-directory "deploy/releases/$current_revision" \
  --env-file runtime.env --env-file state/current.env \
  -f "deploy/releases/$current_revision/compose.production.yml" \
  exec --no-TTY api \
  /app/dbinspect -database /app/data/internship-tracker.db
```

Çıktı mutlak DB yolu, dosya boyutu/değişim zamanı ve yalnız tablo satır sayılarını
verir; ilan metni, başvuru notu veya secret yazmaz. Aynı komutu restart/redeploy
öncesi ve sonrası çalıştırıp `database_path`, `listings`, `opportunities`,
`memberships`, `analyses` ve `applications` değerlerini karşılaştırın. API
container'ının `/app/data` mount kimliğini ayrıca kaydedin:

```bash
container_id=$(docker compose \
  --project-directory "deploy/releases/$current_revision" \
  --env-file runtime.env --env-file state/current.env \
  -f "deploy/releases/$current_revision/compose.production.yml" ps -q api)
docker inspect --format '{{range .Mounts}}{{if eq .Destination "/app/data"}}{{.Name}} {{.Source}}{{end}}{{end}}' "$container_id"
```

Farklı mount adı/yolu farklı SQLite açıldığını kanıtlar. Sayılar aynıyken kayıt
yalnız dashboard'da görünmüyorsa `GET /api/v1/opportunities` ile geçmiş sorgusu
kontrol edilir. Scan hatasında status ve `Content-Type` header'ı kaydedilir;
HTML/plain-text gövde secret içerebileceğinden işletim kaydına kopyalanmaz.

## Snapshot alma

Çalışan production release'i için `/srv/internship-tracker` dizininde çalıştırın:

```bash
current_revision=$(awk -F= '$1 == "DEPLOY_REVISION" {print $2}' state/current.env)
docker compose \
  --project-directory "deploy/releases/$current_revision" \
  --env-file runtime.env \
  --env-file state/current.env \
  -f "deploy/releases/$current_revision/compose.production.yml" \
  exec api \
  /app/backup -database /app/data/internship-tracker.db -directory /app/backups
```

Normal deployment bu işlemi `deploy.sh` içinde zorunlu olarak ve backup
binary'sini açık entrypoint seçerek yapar. Bu manuel komut yalnız ek snapshot
gerektiğinde kullanılır.

Komut yalnız var olan, normal bir SQLite dosyasını `mode=rw` ile açar; hatalı
bir yol nedeniyle boş bir veritabanı oluşturmaz. Adı kullanıcı tarafından
verilemeyen, zaman damgalı ve rastgele son ekli bir dosya üretir. Böylece mevcut
bir snapshot'ın üzerine yazılamaz. Snapshot `VACUUM INTO` ile alınır, önce
geçici dosyada `integrity_check` uygulanır, ardından atomik biçimde yayınlanır.
Backup dizini `0700`, snapshot dosyası `0600` iznine zorlanır.

Başarılı komut çıktısındaki `snapshot=...` yolunu kaydedin. Günlük uygulama
yedekleri ile tek seferlik snapshot aynı `tracker_backups` volume'unda kalabilir.
Bu volume ana SQLite volume'undan ayrıdır, ancak yereldir: VM, host veya volume
kaybedilirse tek başına kurtarma sağlamaz.

## Restore ön kontrolü

Bir snapshot'ı production veritabanına yazmadan önce aşağıdaki komutla
denetleyin:

```bash
current_revision=$(awk -F= '$1 == "DEPLOY_REVISION" {print $2}' state/current.env)
docker compose \
  --project-directory "deploy/releases/$current_revision" \
  --env-file runtime.env \
  --env-file state/current.env \
  -f "deploy/releases/$current_revision/compose.production.yml" \
  exec api \
  /app/restorecheck -backup /app/backups/internship-tracker-....db \
  -migrations /app/migrations
```

`restorecheck` snapshot'ı yalnız `mode=ro&immutable=1` SQLite URI'siyle açar.
`PRAGMA integrity_check` sonucu `ok` olmalı ve deploy edilecek image içindeki
her `.sql` migration dosyasının adı snapshot'ın `schema_migrations` tablosunda
bulunmalıdır. Başarılı çıktı `verified=...` olur. Bu komut migration çalıştırmaz
ve production DB'ye hiçbir yazma yapmaz.

Yerelde aynı komutlar doğrudan çalıştırılabilir:

```bash
go run ./cmd/backup -database data/internship-tracker.db -directory backups
go run ./cmd/restorecheck -backup backups/internship-tracker-....db -migrations migrations
```

## Off-host yedekleme ve retention

Bu repository dış hedefe otomatik upload yapmaz. Hedef ve kimlik bilgisi henüz
kullanıcı tarafından seçilmediğinden restic deposu, OCI Object Storage bucket'ı
veya başka bir dış sağlayıcı için bağlantı/secret eklenmemelidir. Yerel snapshot
başarılı olsa bile off-host kopya değildir.

Hedef seçildiğinde snapshot'ları şifreli olarak dışarı aktarın; restic ya da
OCI Object Storage uygun seçeneklerdir. Başlangıç retention politikası:

- Son 7 günlük snapshot.
- Son 4 haftanın her biri için bir snapshot.
- Her deployment öncesinde alınan snapshot, en az ilgili deploy doğrulanana
  kadar korunur.

Dışa aktarma işinin sonucu, son başarılı upload zamanı ve hata durumu izlenmeli;
başarısız upload günlük yerel snapshot başarısı olarak raporlanmamalıdır.

## Manuel restore provası

Restore provası production volume üzerinde yapılmaz. En az her büyük deploydan
sonra veya ayda bir aşağıdaki akışı izleyin:

1. Dış hedefteki bir snapshot'ı geçici ve erişimi sınırlı bir dizine indirin.
2. İndirdiğiniz dosyada `restorecheck` çalıştırın; mevcut release migration'ları
   ile doğrulama başarısızsa deploy veya restore işlemine devam etmeyin.
3. Geçici dizinde ayrı bir SQLite dosyası olarak snapshot'ı saklayın; production
   `tracker_data` volume'unu bağlamayın ya da üzerine yazmayın.
4. Uygulamayı yalnız bu geçici veriyle, izole bir port/ortamda başlatın ve
   `/ready`, dashboard ve son scan kaydını kontrol edin.
5. Provanın tarihi, kullanılan snapshot kimliği, doğrulama sonucu ve gözlenen
   sorunları güvenli işletim kaydına yazın. Geçici kopyayı kurum politikasına
   göre güvenle kaldırın.

Gerçek production restore, ancak bu prova başarılı olduktan ve mevcut
production DB için yeni bir pre-restore snapshot alındıktan sonra yapılır.
