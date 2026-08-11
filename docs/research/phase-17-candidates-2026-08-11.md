# Faz 17 üçüncül şirket araştırması — 11 Ağustos 2026

Bu rapor Bilkent CYBERPARK, ODTÜ TEKNOKENT ve Hacettepe Teknokent için sonlu
bir yazılım/BT aday kataloğudur. Her parkın güncel resmî dizini tarandı; yalnız
resmî şirket kimliği ile geçmiş staj veya açık pozisyon sinyali birlikte
bulunan beş aday tutuldu. Makinece doğrulanan veri
[`phase-17-candidates-2026-08-11.json`](phase-17-candidates-2026-08-11.json)
dosyasındadır.

## Sonuç

| Teknokent | Önerilen | Düşük sinyal | Manuel erişim | Toplam |
| --- | ---: | ---: | ---: | ---: |
| Bilkent CYBERPARK | 5 | 0 | 0 | 5 |
| ODTÜ TEKNOKENT | 4 | 0 | 1 | 5 |
| Hacettepe Teknokent | 3 | 1 | 1 | 5 |
| **Toplam** | **12** | **1** | **2** | **15** |

Önerilen ilk liste: Etiya, OBSS, SİMSOFT, T2 Software, MobileAction, Binalyze,
TaleWorlds Entertainment, Insider, Netaş, LOTEC, Bilişim AŞ ve Udemy.

Alictus yalnız LinkedIn şirket profili üzerinden fırsat izi verdiği için manuel
erişimde bırakıldı. Ankara Bilgi Teknolojileri için şirket ve teknokent kimliği
resmî, fakat staj izi eski bir üniversite ortakları belgesidir. Peaksoft'un
resmî sitesinde kariyer başlığı bulunmasına rağmen ayrı ve yapılandırılmış ilan
akışı olmadığı için düşük sinyal verildi.

## Tekrarlanabilir yöntem

1. Her teknokentin resmî yazılım/BT firma dizininden şirket kimliği ve domain
   eşleştirildi.
2. Şirketin resmî kariyer, ATS, staj programı veya başvuru sayfası arandı.
3. Resmî fırsat kaynağı yoksa yalnız doğrulanabilir geçmiş üçüncü taraf izi
   tutuldu ve otomatik erişime uygun sayılmadı.
4. Kimliği, teknoloji odağı veya fırsat izi yeterli olmayan şirketler rapora
   alınmadı; park başına üst sınırın altında kalındı.
5. JSON kabul testi HTTPS kanıtlarını, durum/erişim sözlüğünü, benzersiz şirket
   ve domainleri ve production kaynak kataloğundan izolasyonu doğrular.

## Sınır

Bu çıktı yalnız araştırmadır. On beş adayın hiçbiri `configs/sources.json`,
tarama programı veya bildirim akışına eklenmedi. Otomatik kaynak uygunluğu,
fixture ve yanlış bildirim korumaları Faz 18'de ayrı batch'lerle ele alınmalıdır.

