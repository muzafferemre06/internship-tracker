# Production için sizden gerekenler

Repository tarafındaki production kodu ve otomasyon hazır. Gerçek yayına geçmek
için aşağıdaki hesap ve kararlar kullanıcı tarafında tamamlanmalıdır. Private
key, API key veya Tunnel token'ı sohbete ya da Git'e göndermeyin; bunları ilgili
GitHub secret veya sunucu dosyasına doğrudan siz yerleştirin.

## İlk turda iletmeniz gereken kararlar

1. Oluşturacağınız GitHub repository URL'si ve GHCR paketlerinin `public` veya
   `private` olacağı.
2. Kullanılacak production hostname; örneğin `staj.example.com`.
3. OCI home region ve Always Free VM'in hazır olup olmadığı. Secret veya SSH
   private key yerine yalnız region ve hazırlık durumunu iletin.
4. Analiz seçimi: `deterministic`, doğrudan Google Gemini veya OpenRouter.
5. Off-host yedek hedefi. OCI kullanılacaksa Instance Principal ile OCI Object
   Storage önerilir; böylece ayrı bir storage secret'ı gerekmeyebilir.
6. Alarm hedefi: scheduler/backup/deploy hatalarının gideceği e-posta adresi veya
   tercih edilen monitoring servisi.

## Sizin hesabınızda yapılacaklar

- GitHub repository'yi oluşturun, Actions/Packages'i açın ve `production`
  environment'ına required reviewer ekleyin.
- OCI VM, root olmayan deploy kullanıcısı ve doğrulanmış SSH host key kaydını
  hazırlayın. Uygulama için inbound web/API portu açmayın.
- Domain'i Cloudflare'a ekleyin; remotely-managed Tunnel ve yalnız kendi
  hesabınıza izin veren Access uygulaması oluşturun.
- Gerçek `candidate-profile.json` ve `sources.json` dosyalarını repository
  checkout'u dışında sunucuya koyun.
- Seçtiğiniz LLM anahtarını ve Tunnel token'ını runbook'taki owner-only secret
  dosyalarına yazın.

VAPID key çifti bir kez, güvenilen yerel makinede oluşturulur:

```bash
go run ./cmd/vapidkey -private-output web_push_private_key
```

Komut public key'i terminale yazar; private key'i `0600` izinli yeni dosyaya
kaydeder ve var olan dosyanın üzerine yazmaz. Private dosyayı sunucudaki
`secrets/api/` dizinine aktarın, public değeri `runtime.env` içinde kullanın.
Key'i kaybederseniz mevcut tarayıcı abonelikleri yeniden kurulmak zorunda kalır.

## Sonraki ortak adım

Yukarıdaki ilk altı kararı secret içermeden paylaşın. Ardından
[deployment runbook'undaki](./deployment.md) sunucu yollarını ve GitHub
environment değerlerini birlikte kesinleştirip ilk digest deployment'ını
çalıştıracağız. Faz 5 ancak gerçek HTTPS ortamında scheduled scan, cihaz push
deep-link'i, Access reddi, off-host yedek ve restore provası kanıtlandıktan sonra
kapanır.
