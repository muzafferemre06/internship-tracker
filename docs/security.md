# Production güvenlik yapılandırması

`APP_ENV=production` yalnızca path içermeyen; `localhost`, `.localhost` alt alanı
ve loopback IP kullanmayan bir HTTPS `ALLOWED_ORIGIN` ile başlar. SQLite,
migration, aday profili ve kaynak
dosyalarının yolları ile backup dizini mutlak olmalıdır; `:memory:` ve SQLite
URI veritabanları kabul edilmez. Backup etkin olmalı, Web Push etkin olmalı ve
VAPID private key verilmelidir. Varsayılan deterministik analizci, bir dış LLM
anahtarına ihtiyaç duymayan bilinçli production seçeneğidir.

## Secret teslimi

`OPENROUTER_API_KEY`, `GEMINI_API_KEY` ve `WEB_PUSH_PRIVATE_KEY` doğrudan
environment değeriyle veya karşılık gelen `_FILE` secret dosyasıyla verilir.
Bir secret için ikisi aynı anda ayarlanamaz. `_FILE` boş bir yol, okunamayan
dosya veya boş içerik gösterirse uygulama dinlemeye başlamaz. Production'da
secret dosya yolları mutlak olmalıdır. Secret değerleri hata ve yapılandırılmış
loglara yazılmaz.

## Tarayıcı mutation koruması

Production HTTP katmanı `POST`, `PUT` ve `DELETE` isteklerinde `Origin`
header'ını `ALLOWED_ORIGIN` ile tam olarak eşleştirir; eksik ya da farklı origin
`403` döner. `GET /health`, `GET /ready` ve diğer GET uçları etkilenmez. CORS
preflight, yalnız yapılandırılmış origin için izin başlıklarıyla `204` döner;
başka origin `403` alır.
