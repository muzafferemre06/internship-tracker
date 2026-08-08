const CACHE_NAME = "staj-takip-v1";
const APP_SHELL = ["/", "/manifest.webmanifest", "/icon.svg", "/icon-192.png"];

self.addEventListener("install", (event) => {
  event.waitUntil(caches.open(CACHE_NAME).then((cache) => cache.addAll(APP_SHELL)));
  self.skipWaiting();
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches.keys().then((keys) =>
      Promise.all(keys.filter((key) => key !== CACHE_NAME).map((key) => caches.delete(key))),
    ),
  );
  self.clients.claim();
});

self.addEventListener("fetch", (event) => {
  if (event.request.method !== "GET") return;
  event.respondWith(fetch(event.request).catch(() => caches.match(event.request)));
});

self.addEventListener("push", (event) => {
  const fallback = {
    title: "Yeni staj duyurusu",
    body: "Dashboard'u kontrol et.",
    url: "/",
    tag: "staj-takip-yeni-ilan",
  };
  let received = {};

  if (event.data) {
    try {
      const decoded = event.data.json();
      if (decoded && typeof decoded === "object" && !Array.isArray(decoded)) received = decoded;
    } catch (_error) {
      // Keep the safe default notification for malformed payloads.
    }
  }

  const payload = {
    title: safeText(received.title, fallback.title, 120),
    body: safeText(received.body, fallback.body, 300),
    url: sameOriginPath(received.url),
    tag: safeText(received.tag, fallback.tag, 160),
  };

  event.waitUntil(
    self.registration.showNotification(payload.title, {
      body: payload.body,
      icon: "/icon-192.png",
      tag: payload.tag,
      data: { url: payload.url },
    }),
  );
});

self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  const targetURL = new URL(sameOriginPath(event.notification.data?.url), self.location.origin).href;
  event.waitUntil(
    self.clients.matchAll({ type: "window", includeUncontrolled: true }).then(async (windowClients) => {
      const client = windowClients.find((candidate) => {
        try {
          return new URL(candidate.url).origin === self.location.origin;
        } catch (_error) {
          return false;
        }
      });

      if (!client) return self.clients.openWindow(targetURL);
      const navigated = await client.navigate(targetURL);
      return (navigated || client).focus();
    }),
  );
});

function safeText(value, fallback, maxLength) {
  return typeof value === "string" && value.trim()
    ? value.trim().slice(0, maxLength)
    : fallback;
}

function sameOriginPath(value) {
  try {
    const url = new URL(typeof value === "string" ? value : "/", self.location.origin);
    if (url.origin !== self.location.origin) return "/";
    return `${url.pathname}${url.search}${url.hash}`;
  } catch (_error) {
    return "/";
  }
}
