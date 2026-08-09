# Scraper fixture'ları

Canlı kaynaklardan alınan ve kişisel veri içermeyen küçültülmüş HTML örnekleri
adapter bazında bu dizinde tutulacaktır. Fixture'ın alındığı tarih ve temsil
ettiği senaryo test dosyasında belirtilmelidir.

`kariyernet/` fixture'ları 3 Ağustos 2026 tarihinde doğrulanan Meteksan şirket
profili davranışının küçültülmüş ve kişisel veri içermeyen temsilleridir. Canlı
sayfanın birebir kopyası değildir; ilan bağlantısı, başlık, sıfır ilan ve
tanınmayan sayfa senaryolarını kapsar.

`aselsan-listings.html` ve `aselsannet-listings.html`, aynı şirket grubundaki
farklı profil başlıklarını ve profiller arası ortak ilan dedup davranışını
doğrular.

`access-challenge.html`, HTTP 200 dönse bile Cloudflare challenge işaretlerinin
geçerli bir şirket sayfası gibi işlenmemesini doğrular.

`learnedselector/layout-v1.html` ve `layout-v2.html`, aynı kurgusal kariyer
sayfasının iki farklı DOM sürümüdür. İlk fixture başlangıç reçetesini ve restart
sonrası AI'sız çalışmayı; ikinci fixture eski selector'ların sıfır ilana düşmesini
ve golden-snapshot guard üzerinden reçete onarımını doğrular. Canlı bir sitenin
kopyası değildir ve kişisel veri içermez.
