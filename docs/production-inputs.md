# Production için sizden gerekenler

Repository tarafındaki production kodu ve otomasyon hazır. Gerçek yayına geçmek
için aşağıdaki hesap ve kararlar kullanıcı tarafında tamamlanmalıdır. Private
key, API key veya Tunnel token'ı sohbete ya da Git'e göndermeyin; bunları ilgili
GitHub secret veya sunucu dosyasına doğrudan siz yerleştirin.

## İlk turda iletmeniz gereken kararlar

1. Oluşturacağınız GitHub repository URL'si ve GHCR paketlerinin `public` veya
   `private` olacağı.
2. Kullanılacak production hostname; örneğin `staj.example.com`.
3. Production host seçimi ve hazır olup olmadığı. OCI düşünülüyorsa home region,
   VM mimarisi ve hesapta uygun kapasitenin doğrulanıp doğrulanmadığını iletin;
   paket OCI'ye bağımlı değildir ve desteklenen başka bir 64-bit Linux hostta da
   çalışır. Secret veya SSH private key paylaşmayın.
4. Analiz seçimi: `deterministic`, doğrudan Google Gemini veya OpenRouter.
   Dış sağlayıcı seçilecekse kesin model kimliğini ve maliyet hesabı isteniyorsa
   güncel milyon input/output token ücretlerini de belirtin.
5. Off-host yedek hedefi. OCI kullanılacaksa Instance Principal ile OCI Object
   Storage önerilir; böylece ayrı bir storage secret'ı gerekmeyebilir.
6. VAPID iletişim URI'si; örneğin `mailto:operator@example.com`. Bu public
   iletişim bilgisi `WEB_PUSH_SUBJECT` olarak kullanılır ve private key değildir.
7. Alarm hedefi: scheduler/backup/deploy hatalarının gideceği e-posta adresi veya
   tercih edilen monitoring servisi. Alarm entegrasyonu hedef seçildikten sonra
   ayrıca tamamlanacaktır; repository şu anda yapılandırılmış hata logları üretir.

## Sizin hesabınızda yapılacaklar

- GitHub repository'yi oluşturun, Actions/Packages'i açın ve `production`
  environment'ına required reviewer ekleyin. GHCR private olacaksa sunucuda
  yalnız package pull yetkili registry kimliğini ayrıca hazırlayın.
- Seçilen Linux VM'de root olmayan deploy kullanıcısı ve doğrulanmış SSH host
  key kaydını hazırlayın. Uygulama için inbound web/API portu açmayın.
- Domain'i Cloudflare'a ekleyin; remotely-managed Tunnel ve yalnız kendi
  hesabınıza izin veren Access uygulaması oluşturun. Otomatik deploy smoke'u
  için Service Auth politikasıyla yetkilendirilmiş dar kapsamlı bir service
  token hazırlayın; token değerini paylaşmayın.
- Gerçek `candidate-profile.json` ve `sources.json` dosyalarını repository
  checkout'u dışında sunucuya koyun.
- Seçtiğiniz LLM anahtarını ve Tunnel token'ını runbook'taki izinlerle secret
  dosyalarına yazın. API secret dosyaları container UID `100` tarafından
  okunabilmesi için runbook'taki `100:<deploy-gid>` sahipliği ve `0640` modunu
  kullanmalıdır; deploy kullanıcısına ait `0600` dosyayı doğrudan bırakmayın.

VAPID key çifti bir kez, güvenilen yerel makinede oluşturulur:

```bash
go run ./cmd/vapidkey -private-output web_push_private_key
```

Komut public key'i terminale yazar; private key'i `0600` izinli yeni dosyaya
kaydeder ve var olan dosyanın üzerine yazmaz. Private dosyayı runbook'taki
`sudo install -m 0640 -o 100 -g <deploy-gid>` komutuyla sunucudaki
`secrets/api/` dizinine kurun, public değeri `runtime.env` içinde kullanın.
Key'i kaybederseniz mevcut tarayıcı abonelikleri yeniden kurulmak zorunda kalır.

## Sonraki ortak adım

Yukarıdaki ilk yedi kararı secret içermeden paylaşın. Ardından
[deployment runbook'undaki](./deployment.md) sunucu yollarını ve GitHub
environment değerlerini birlikte kesinleştirip ilk digest deployment'ını
çalıştıracağız. Faz 5 ancak gerçek HTTPS ortamında scheduled scan, cihaz push
deep-link'i, Access reddi, off-host yedek ve restore provası kanıtlandıktan sonra
kapanır.
