# Production deployment runbook

Bu belge production sunucusunu ilk kez hazırlayan ve digest sabitli bir sürümü
yayına alan operatör içindir. Gerçek anahtar, token, domain veya sunucu adresi
repoda tutulmaz. Örneklerdeki `/srv/internship-tracker` yolu değiştirilebilir;
workflow değişkenleri ve `runtime.env` aynı mutlak yolları göstermelidir.

## Çalışma modeli

`deploy/compose.production.yml` image build etmez ve host portu açmaz. API yalnız
`internal: true` application ağına bağlıdır. Web hem API ağına hem tunnel ağına
bağlanır; `cloudflared` yalnız tunnel ağından `http://web:8080` adresine erişir.
Cloudflare Tunnel dışarı doğru bağlantı kurduğu için sunucu firewall'ında web veya
API için inbound port açılmaz. SSH erişimi yine işletim politikasına göre
sınırlandırılmalıdır.

Production Compose şu üç image değişkenini yalnız digest manifestinden alır:

```dotenv
API_IMAGE=ghcr.io/owner/internship_tracker/api@sha256:<64-hex>
WEB_IMAGE=ghcr.io/owner/internship_tracker/web@sha256:<64-hex>
CLOUDFLARED_IMAGE=docker.io/cloudflare/cloudflared@sha256:<64-hex>
DEPLOY_REVISION=<40-karakter-küçük-harf-git-commit-sha>
```

Deploy preflight'i etiketi, eksik digest'i veya tam commit SHA olmayan deploy
revision'ını reddeder. `publish.yml`, API ve web
image'larını `linux/amd64` ile `linux/arm64` için tam commit SHA etiketiyle bir kez
yayımlar; var olan commit etiketini ezmez. Aynı revision için tekrar çalıştırılan
workflow, registry inceleme çıktısını tamamen aldıktan sonra mevcut digest'i
yeniden kullanır; böylece immutable etiketi yeniden build etmez. BuildKit
provenance/SBOM ve GitHub
artifact attestation image digest'ine bağlanır. Artifact içindeki API/web digest
kayıtları, seçilmiş `cloudflared` digest'i eklenmeden production manifesti değildir.

Compose proje adı sabit `internship-tracker` olduğu için `tracker_data` ve
`tracker_backups` named volume'ları release dizini değişse de aynı kimlikle
yeniden bağlanır. Normal deploy yalnız `up --remove-orphans` kullanır; `down -v`,
volume silme veya farklı Compose proje adı bu runbook'un parçası değildir.
Restart/redeploy öncesi ve sonrası volume/DB kimliği ile güvenli satır sayımı
[operasyon rehberindeki](./operations.md) `dbinspect` akışıyla kaydedilir.

## Önkoşullar

- Desteklenen 64-bit Linux sunucusu ve güncel güvenlik yamaları
- Docker Engine ile `docker compose` v2; deploy operatörünün Docker erişimi
- `curl`, `awk`, `diff`, `find`, `grep`, `env`, `mktemp`, `sed`, `sha256sum`,
  `tar` ve GNU `stat`
- Cloudflare'da yönetilen domain, remotely-managed Tunnel ve Access uygulaması
- GHCR image'larını çekebilen Docker registry kimliği
- Production için ayrı, root olmayan bir deploy hesabı
- Host dışında saklanan ve düzenli restore testi yapılan yedek hedefi

Docker grubunun root yetkisine denk olduğu kabul edilmelidir. Hesap interaktif
genel kullanım için paylaşılmamalı; SSH anahtarı yalnız deployment amacıyla
kullanılmalıdır. Script root kullanıcıyla çalışmayı açıkça reddeder.

## Sunucu dosya düzeni

Bir yönetici dizinleri bir kez oluşturur. `<deploy-uid>` ve `<deploy-gid>`, root
olmayan deploy hesabının `id -u` ve `id -g` çıktılarıdır. Backend image'ındaki
`app` kullanıcısının UID'si `100` olduğundan API secret dizini owner UID `100`,
deploy hesabının primary grubu ise group olarak ayarlanır. Böylece container
secret'ı okuyabilir, preflight dosyanın varlığını sınayabilir ve diğer host
kullanıcıları okuyamaz.

```text
/srv/internship-tracker/
├── deploy/
│   └── releases/
│       ├── <commit-sha>/         # immutable compose, nginx ve scripts bundle'ı
│       └── <önceki-commit-sha>/  # rollback için korunur
├── config/
│   ├── candidate-profile.json
│   └── sources.json
├── secrets/
│   ├── api/
│   │   ├── web_push_private_key
│   │   ├── gemini_api_key        # yalnız seçilen sağlayıcı gerekiyorsa
│   │   └── openrouter_api_key    # yalnız seçilen sağlayıcı gerekiyorsa
│   ├── cloudflare_tunnel_token
│   ├── cf_access_client_id       # dış smoke Access arkasındaysa isteğe bağlı
│   └── cf_access_client_secret   # dış smoke Access arkasındaysa isteğe bağlı
├── runtime.env
└── state/
    ├── current.env               # ilk başarılı deploy'da oluşur
    └── previous.env              # ikinci başarılı deploy'da oluşur
```

Örnek tek seferlik hazırlık; gerçek kullanıcı/grup değerlerini kullanın:

```bash
sudo install -d -m 0750 -o <deploy-uid> -g <deploy-gid> /srv/internship-tracker
sudo install -d -m 0750 -o <deploy-uid> -g <deploy-gid> /srv/internship-tracker/{deploy,state,secrets}
sudo install -d -m 0750 -o <deploy-uid> -g <deploy-gid> /srv/internship-tracker/deploy/releases
sudo install -d -m 0750 -o 100 -g <deploy-gid> /srv/internship-tracker/config
sudo install -m 0640 -o 100 -g <deploy-gid> candidate-profile.json /srv/internship-tracker/config/candidate-profile.json
sudo install -m 0640 -o 100 -g <deploy-gid> sources.json /srv/internship-tracker/config/sources.json
sudo install -d -m 0750 -o 100 -g <deploy-gid> /srv/internship-tracker/secrets/api
sudo install -m 0640 -o 100 -g <deploy-gid> web_push_private_key /srv/internship-tracker/secrets/api/web_push_private_key
# Yalnız seçilen dış sağlayıcının anahtarını kurun:
sudo install -m 0640 -o 100 -g <deploy-gid> gemini_api_key /srv/internship-tracker/secrets/api/gemini_api_key
# veya: sudo install -m 0640 -o 100 -g <deploy-gid> openrouter_api_key /srv/internship-tracker/secrets/api/openrouter_api_key
sudo install -m 0600 -o <deploy-uid> -g <deploy-gid> cloudflare_tunnel_token /srv/internship-tracker/secrets/cloudflare_tunnel_token
sudo install -m 0600 -o <deploy-uid> -g <deploy-gid> runtime.env /srv/internship-tracker/runtime.env
```

Secret değerlerini shell history'ye yazmayın. Bir secret manager'dan dosyaya
aktarırken atomik değişim ve doğru owner/mode uygulayın. API secret dizininde
yalnız gereken dosyalar bulunabilir. Backend doğrudan secret env değerini de
destekler; ancak bu production deployment paketinin preflight'i doğrudan secret
değerlerini reddeder ve yalnız `_FILE` değişkenlerini kabul eder.

Workflow exact `github.sha` revision'ını sparse checkout eder; yalnız
`deploy/compose.production.yml`, `deploy/nginx.production.conf` ve
`deploy/scripts/*.sh` dosyalarını arşivler. Sunucu checksum, allowlist, regular
file/no-symlink sözleşmesi ve manifest revision eşleşmesini doğruladıktan sonra
bundle'ı `deploy/releases/<commit-sha>` altına atomik taşır. Aynı revision yeniden
çalıştırıldığında byte farkı varsa immutable release ezilmez ve deploy durur.
Kurulum `config`, `secrets`, `runtime.env` veya `state` içeriğini kopyalamaz ya da
ezmez. Compose `0640`, scriptler `0750` kurulur; secretsız nginx config ise
container UID `101` tarafından okunabilmesi için `0644` olur.

## Runtime yapılandırması

`runtime.env` image digest'i ve doğrudan secret içermez. Minimum örnek:

```dotenv
ALLOWED_ORIGIN=https://tracker.example.com
CANDIDATE_PROFILE_FILE=/srv/internship-tracker/config/candidate-profile.json
SOURCES_FILE=/srv/internship-tracker/config/sources.json
API_SECRETS_DIRECTORY=/srv/internship-tracker/secrets/api
CLOUDFLARE_TUNNEL_TOKEN_FILE=/srv/internship-tracker/secrets/cloudflare_tunnel_token
DEPLOY_UID=<deploy-uid>
DEPLOY_GID=<deploy-gid>

LLM_PROVIDER=google
LLM_MODEL=gemini-3.1-flash-lite
GEMINI_API_KEY_FILE=/run/secrets/gemini_api_key
LLM_THINKING_LEVEL=minimal
LLM_INPUT_COST_PER_MILLION_USD=0
LLM_OUTPUT_COST_PER_MILLION_USD=0

SCAN_SCHEDULE=0 9 * * 1
SCAN_TIMEZONE=Europe/Istanbul
BACKUP_TIME=02:00
BACKUP_TIMEZONE=Europe/Istanbul
BACKUP_RETENTION=7

WEB_PUSH_PUBLIC_KEY=<vapid-public-key>
WEB_PUSH_SUBJECT=mailto:operator@example.com
```

Deterministic sağlayıcıda `LLM_PROVIDER=deterministic` kullanın ve iki API key
file değişkenini de kaldırın. OpenRouter için yalnız
`OPENROUTER_API_KEY_FILE=/run/secrets/openrouter_api_key` eklenir. Production'da
HTTPS ve path içermeyen `ALLOWED_ORIGIN`, backup ve Web Push zorunludur.
`runtime.env` için `0600` kullanılır. Preflight, API container'ına bağlanan
profile/source dosyaları ile gerekli Web Push ve seçilmiş provider secret'larının
UID `100`, mode `0640`; API secret dizininin UID `100`, mode `0750` olmasını
zorunlu doğrular. Cloudflare Tunnel token'ı ise deploy kullanıcısına ait ve
`0600` olmalıdır. Nginx config yolu runtime ayarı değildir; Compose her
revision'ın doğrulanmış bundle'ındaki `nginx.production.conf` dosyasını
salt-okunur bağlar.

## Cloudflare hazırlığı

1. Remotely-managed Tunnel oluşturun ve token'ı yalnız
   `cloudflare_tunnel_token` dosyasına koyun.
2. Tunnel public hostname servis hedefini `http://web:8080` yapın. API için ayrı
   hostname veya route tanımlamayın.
3. Hostname önüne Cloudflare Access self-hosted uygulaması ekleyin ve yalnız
   kullanılacak hesabı/politikayı yetkilendirin.
4. HTTPS redirect ve edge TLS politikasını açın. Origin host portu olmadığı için
   Tunnel'ı devre dışı bırakmak uygulamayı internetten erişilemez bırakır.
5. Access arkasındaki hostname'e yapılan otomatik dış smoke için en dar kapsamlı
   Access service token ve onu kabul eden Service Auth politikası zorunludur;
   client ID/secret'ı ayrı `0600` dosyalarda saklayın. Service token
   kullanmayacaksanız `/ready` için açıkça incelenmiş bir bypass politikası gerekir.

Cloudflare, `cloudflared tunnel run --token-file` desteğini 2025.4.0 ve sonrasında
sağlar. Seçilen image digest'i bu alt sınırdan yeni ve Cloudflare'ın destek
penceresinde bir sürüme ait olmalıdır. Digest'i tag sayfasından kopyalamak yerine
güvenilen bir makinede çözümleyip doğrulayın:

```bash
docker pull docker.io/cloudflare/cloudflared:<reviewed-version>
docker image inspect --format '{{index .RepoDigests 0}}' docker.io/cloudflare/cloudflared:<reviewed-version>
```

## Manuel preflight ve deploy

Publish artifact'inden gelen API/web satırlarına onaylanmış cloudflared digest ve
bundle'ın tam commit SHA değerini ekleyip `release.env` oluşturun. Önce ilgili
commit'in allowlist ile sınırlı bundle'ını workflow ile aynı biçimde
`deploy/releases/<commit-sha>` altına kurun. Sonra root olmayan deploy hesabıyla:

```bash
/srv/internship-tracker/deploy/releases/<commit-sha>/scripts/preflight.sh \
  /srv/internship-tracker/release.env \
  /srv/internship-tracker/runtime.env \
  /srv/internship-tracker/deploy/releases/<commit-sha>/compose.production.yml \
  /srv/internship-tracker/state

/srv/internship-tracker/deploy/releases/<commit-sha>/scripts/deploy.sh \
  /srv/internship-tracker/release.env \
  /srv/internship-tracker/runtime.env \
  /srv/internship-tracker/state \
  https://tracker.example.com
```

Deploy sırası preflight, mevcut SQLite volume'undan zorunlu ve tutarlı
`/app/backup` snapshot'ı, digest pull, `docker compose up --wait`, container içi
gerçek GET `/ready` smoke ve isteğe bağlı dış HTTPS smoke'tur. API yalnız izin
verilen yöntemleri kabul ettiği için origin smoke HEAD/`wget --spider` kullanmaz. Snapshot başarısızsa
image veya container değiştirilmez. Başarısız candidate varsa
`state/current.env` image'ları, manifestteki `DEPLOY_REVISION` ile seçilen önceki
Compose ve smoke scriptiyle otomatik geri açılır. İlk deploy başarısızsa
yarım kalan container'lar volume silmeden durdurulur. Başarılı deploy mevcut
manifesti `previous.env` yapıp candidate'ı `current.env` olarak atomik kaydeder.
Henüz `current.env` bulunmayan ilk deployment'ta korunacak mevcut release olmadığı
için pre-deploy snapshot adımı atlanır.

Access arkasındaki dış smoke için komuttan önce credential dosya yollarını
verin; değerler argüman veya process listesine konmaz. `PUBLIC_ORIGIN`,
`runtime.env` içindeki `ALLOWED_ORIGIN` ile aynı olmalıdır; deploy scripti
eşleşmeyi zorunlu kılar:

```bash
export CF_ACCESS_CLIENT_ID_FILE=/srv/internship-tracker/secrets/cf_access_client_id
export CF_ACCESS_CLIENT_SECRET_FILE=/srv/internship-tracker/secrets/cf_access_client_secret
```

Manuel rollback'te önce `state/current.env` içindeki `DEPLOY_REVISION` değerini
okuyup tam olarak o bundle'ın rollback scriptini çalıştırın:

```bash
current_revision=$(awk -F= '$1 == "DEPLOY_REVISION" {print $2}' /srv/internship-tracker/state/current.env)
/srv/internship-tracker/deploy/releases/$current_revision/scripts/rollback.sh \
  /srv/internship-tracker/runtime.env \
  /srv/internship-tracker/state \
  https://tracker.example.com
```

Rollback önceki manifestin `DEPLOY_REVISION` bundle'ını doğrular ve yalnız önceki
image, Compose, nginx ile smoke sözleşmesi birlikte başarıyla çalışırsa
`current.env` ile `previous.env` dosyalarını değiştirir. Release dizinlerini
temizlerken en az current ve previous manifestlerinin işaret ettiği iki revision
korunmalıdır. Migration ileri
uyumluluğu ayrıca her sürüm değişikliğinde incelenmelidir; bu akış veritabanı
şemasını geri almaz.

## GitHub production environment

Workflow yalnız `main` push ve elle `workflow_dispatch` ile image yayınlar; pull
request secret'larına erişmez ve image push etmez. Gerçek deploy yalnız dispatch
ekranında `deploy=true` seçilirse çalışır. Aşağıdaki protected production
environment değerlerinden biri eksikse deploy sessizce atlanmaz; workflow hata
vererek durur.

Secrets:

- `PRODUCTION_HOST`
- `PRODUCTION_USER` (root olamaz)
- `PRODUCTION_SSH_PRIVATE_KEY`
- `PRODUCTION_KNOWN_HOSTS` (`ssh-keyscan` ile workflow içinde üretilmez; önceden
  bağımsız kanaldan doğrulanmış host key kaydıdır)

Variables:

- `PRODUCTION_SSH_PORT` (yoksa `22`)
- `PRODUCTION_DEPLOY_DIRECTORY`
- `PRODUCTION_RUNTIME_ENV_PATH`
- `PRODUCTION_STATE_DIRECTORY`
- `PRODUCTION_PUBLIC_ORIGIN`
- `PRODUCTION_CLOUDFLARED_IMAGE` (digest sabitli tam reference)
- `PRODUCTION_CF_ACCESS_CLIENT_ID_FILE` ve
  `PRODUCTION_CF_ACCESS_CLIENT_SECRET_FILE` (Access korumalı dış smoke için
  sunucudaki mutlak service-token dosya yolları; ikisi birlikte verilir)

GHCR paketleri private ise production host ayrıca yalnız `read:packages`
eşdeğeri pull erişimi olan registry kimliğiyle oturum açmış olmalıdır. GitHub
Actions runner'ının registry oturumu production hosta aktarılmaz. Credential'ı
`runtime.env`, release manifesti veya deploy komut satırına koymayın.

Production environment'a required reviewer eklenmesi önerilir. Workflow SSH'ta
`StrictHostKeyChecking=yes`, ayrılmış known-hosts dosyası, tek identity ve batch
mode kullanır; hedef hesabın UID'sini de uzaktan doğrular. Deploy job exact event
SHA'sını checkout eder ve yalnız production Compose/nginx/script allowlist'ini
taşır. Uzak kurulum arşiv SHA-256 değerini, manifest revision'ını ve tüm girdilerin
regular file olup symlink olmadığını deploy başlamadan önce yeniden doğrular.

## İşletim kontrolleri

Her deploy sonrasında en az şunları kaydedin:

- `current.env` içindeki üç digest ve Git commit SHA
- `/ready` dış smoke sonucu
- `docker compose ps` servis/health durumu
- son başarılı scheduled scan zamanı
- son SQLite snapshot zamanı, boyutu ve `integrity_check` sonucu
- offsite kopyanın yaşı ve ayrı dizinde yapılan son restore testi

`tracker_backups` named volume'u yalnız aynı hosttaki günlük snapshot'ları tutar;
disk veya hesap kaybına karşı offsite yedek değildir. Named volume silen `down -v`
ve benzeri komutlar bu runbook'un parçası değildir.

Cloudflare Tunnel'ın outbound-only modeli ve token kapsamı için Cloudflare'ın
[Tunnel dokümanı](https://developers.cloudflare.com/cloudflare-one/networks/connectors/cloudflare-tunnel/)
ile [token-file parametresi](https://developers.cloudflare.com/tunnel/advanced/run-parameters/#token-file),
container attestation izinleri için GitHub'ın
[artifact attestation rehberi](https://docs.github.com/en/actions/how-tos/secure-your-work/use-artifact-attestations/use-artifact-attestations)
esas alınır.
