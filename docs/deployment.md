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
```

Deploy preflight'i etiketi veya eksik digest'i reddeder. `publish.yml`, API ve web
image'larını `linux/amd64` ile `linux/arm64` için tam commit SHA etiketiyle bir kez
yayımlar; var olan commit etiketini ezmez. BuildKit provenance/SBOM ve GitHub
artifact attestation image digest'ine bağlanır. Artifact içindeki API/web digest
kayıtları, seçilmiş `cloudflared` digest'i eklenmeden production manifesti değildir.

## Önkoşullar

- Desteklenen 64-bit Linux sunucusu ve güncel güvenlik yamaları
- Docker Engine ile `docker compose` v2; deploy operatörünün Docker erişimi
- `curl`, `awk`, `grep`, `env`, `mktemp`, `sed` ve GNU `stat`
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
├── deploy/                       # compose, nginx ve scripts; repodan kopyalanır
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
sudo install -d -m 0750 -o <deploy-uid> -g <deploy-gid> /srv/internship-tracker/{deploy,config,state,secrets}
sudo install -d -m 0750 -o 100 -g <deploy-gid> /srv/internship-tracker/secrets/api
sudo install -m 0640 -o 100 -g <deploy-gid> web_push_private_key /srv/internship-tracker/secrets/api/web_push_private_key
sudo install -m 0600 -o <deploy-uid> -g <deploy-gid> cloudflare_tunnel_token /srv/internship-tracker/secrets/cloudflare_tunnel_token
```

Secret değerlerini shell history'ye yazmayın. Bir secret manager'dan dosyaya
aktarırken atomik değişim ve doğru owner/mode uygulayın. API secret dizininde
yalnız gereken dosyalar bulunabilir. Production backend doğrudan secret env
değerini kabul etmez; file değişkeni kullanır.

Deploy paketini ilgili committen sunucuya kopyalayın ve scriptleri executable
yapın:

```bash
chmod 0755 /srv/internship-tracker/deploy/scripts/*.sh
```

## Runtime yapılandırması

`runtime.env` image digest'i ve doğrudan secret içermez. Minimum örnek:

```dotenv
ALLOWED_ORIGIN=https://tracker.example.com
CANDIDATE_PROFILE_FILE=/srv/internship-tracker/config/candidate-profile.json
SOURCES_FILE=/srv/internship-tracker/config/sources.json
NGINX_CONFIG_FILE=/srv/internship-tracker/deploy/nginx.production.conf
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
`runtime.env` için `0600`, profile/source/nginx dosyaları için en fazla `0640`
önerilir.

## Cloudflare hazırlığı

1. Remotely-managed Tunnel oluşturun ve token'ı yalnız
   `cloudflare_tunnel_token` dosyasına koyun.
2. Tunnel public hostname servis hedefini `http://web:8080` yapın. API için ayrı
   hostname veya route tanımlamayın.
3. Hostname önüne Cloudflare Access self-hosted uygulaması ekleyin ve yalnız
   kullanılacak hesabı/politikayı yetkilendirin.
4. HTTPS redirect ve edge TLS politikasını açın. Origin host portu olmadığı için
   Tunnel'ı devre dışı bırakmak uygulamayı internetten erişilemez bırakır.
5. Otomatik dış smoke kullanılacaksa en dar kapsamlı Access service token üretin;
   client ID/secret'ı ayrı `0600` dosyalarda saklayın.

Cloudflare, `cloudflared tunnel run --token-file` desteğini 2025.4.0 ve sonrasında
sağlar. Seçilen image digest'i bu alt sınırdan yeni ve Cloudflare'ın destek
penceresinde bir sürüme ait olmalıdır. Digest'i tag sayfasından kopyalamak yerine
güvenilen bir makinede çözümleyip doğrulayın:

```bash
docker pull docker.io/cloudflare/cloudflared:<reviewed-version>
docker image inspect --format '{{index .RepoDigests 0}}' docker.io/cloudflare/cloudflared:<reviewed-version>
```

## Manuel preflight ve deploy

Publish artifact'inden gelen API/web satırlarına onaylanmış cloudflared digest
satırını ekleyip `release.env` oluşturun. Sonra root olmayan deploy hesabıyla:

```bash
/srv/internship-tracker/deploy/scripts/preflight.sh \
  /srv/internship-tracker/release.env \
  /srv/internship-tracker/runtime.env \
  /srv/internship-tracker/deploy/compose.production.yml \
  /srv/internship-tracker/state

/srv/internship-tracker/deploy/scripts/deploy.sh \
  /srv/internship-tracker/release.env \
  /srv/internship-tracker/runtime.env \
  /srv/internship-tracker/deploy/compose.production.yml \
  /srv/internship-tracker/state \
  https://tracker.example.com
```

Deploy sırası preflight, mevcut SQLite volume'undan zorunlu ve tutarlı
`/app/backup` snapshot'ı, digest pull, `docker compose up --wait`, container içi
`/ready` smoke ve isteğe bağlı dış HTTPS smoke'tur. Snapshot başarısızsa
image veya container değiştirilmez. Başarısız candidate varsa
`state/current.env` image'ları otomatik geri açılır. İlk deploy başarısızsa
yarım kalan container'lar volume silmeden durdurulur. Başarılı deploy mevcut
manifesti `previous.env` yapıp candidate'ı `current.env` olarak atomik kaydeder.
Henüz `current.env` bulunmayan ilk deployment'ta korunacak mevcut release olmadığı
için pre-deploy snapshot adımı atlanır.

Access arkasındaki dış smoke için komuttan önce credential dosya yollarını
verin; değerler argüman veya process listesine konmaz:

```bash
export CF_ACCESS_CLIENT_ID_FILE=/srv/internship-tracker/secrets/cf_access_client_id
export CF_ACCESS_CLIENT_SECRET_FILE=/srv/internship-tracker/secrets/cf_access_client_secret
```

Manuel rollback:

```bash
/srv/internship-tracker/deploy/scripts/rollback.sh \
  /srv/internship-tracker/runtime.env \
  /srv/internship-tracker/deploy/compose.production.yml \
  /srv/internship-tracker/state \
  https://tracker.example.com
```

Rollback ancak önceki image manifesti pull, readiness ve smoke kontrollerinden
geçerse `current.env` ile `previous.env` dosyalarını değiştirir. Migration ileri
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
  `PRODUCTION_CF_ACCESS_CLIENT_SECRET_FILE` (dış smoke Access service token
  kullanıyorsa sunucudaki mutlak dosya yolları; ikisi birlikte verilir)

Production environment'a required reviewer eklenmesi önerilir. Workflow SSH'ta
`StrictHostKeyChecking=yes`, ayrılmış known-hosts dosyası, tek identity ve batch
mode kullanır; hedef hesabın UID'sini de uzaktan doğrular. Sunucudaki deploy
paketi workflow commit'iyle eşleşmelidir; package güncellemesi ayrı, incelenebilir
bir provisioning işlemidir.

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
