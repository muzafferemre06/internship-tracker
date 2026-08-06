export type PushStatus = "unsupported" | "denied" | "subscribed" | "unsubscribed";

type PushSubscriptionValue = Pick<PushSubscription, "endpoint" | "toJSON" | "unsubscribe">;

type PushRegistration = {
  pushManager: {
    getSubscription: () => Promise<PushSubscriptionValue | null>;
    subscribe: (options: PushSubscriptionOptionsInit) => Promise<PushSubscriptionValue>;
  };
};

export type PushRuntime = {
  supported: boolean;
  permission: () => NotificationPermission;
  requestPermission: () => Promise<NotificationPermission>;
  getRegistration: () => Promise<PushRegistration | undefined>;
  register: (scriptURL: string) => Promise<PushRegistration>;
  ready: () => Promise<PushRegistration>;
};

type PushOptions = {
  fetcher?: typeof fetch;
  runtime?: PushRuntime;
};

const publicKeyPath = "/api/v1/push/public-key";
const subscriptionsPath = "/api/v1/push/subscriptions";

export function urlBase64ToUint8Array(value: string): Uint8Array {
  const padding = "=".repeat((4 - (value.length % 4)) % 4);
  const base64 = (value + padding).replace(/-/g, "+").replace(/_/g, "/");
  const decoded = atob(base64);
  return Uint8Array.from(decoded, (character) => character.charCodeAt(0));
}

export async function getPushStatus(options: Pick<PushOptions, "runtime"> = {}): Promise<PushStatus> {
  const runtime = options.runtime ?? createBrowserRuntime();
  if (!runtime?.supported) return "unsupported";
  if (runtime.permission() === "denied") return "denied";
  if (runtime.permission() !== "granted") return "unsubscribed";

  const registration = await ensureRegistration(runtime);
  return (await registration.pushManager.getSubscription()) ? "subscribed" : "unsubscribed";
}

export async function enablePush(apiBaseUrl: string, options: PushOptions = {}): Promise<void> {
  const runtime = options.runtime ?? createBrowserRuntime();
  if (!runtime?.supported) throw new Error("Bu tarayıcı Web Push bildirimlerini desteklemiyor.");

  let permission = runtime.permission();
  if (permission === "default") permission = await runtime.requestPermission();
  if (permission !== "granted") {
    throw new Error("Bildirim izni verilmedi. Tarayıcı ayarlarından bildirimlere izin verebilirsin.");
  }

  const registration = await ensureRegistration(runtime);
  const existing = await registration.pushManager.getSubscription();
  const publicKey = await fetchPublicKey(apiBaseUrl, options.fetcher);
  const subscription = existing ?? await registration.pushManager.subscribe({
    userVisibleOnly: true,
    applicationServerKey: urlBase64ToUint8Array(publicKey),
  });

  try {
    await upsertSubscription(apiBaseUrl, subscription, options.fetcher);
  } catch (error) {
    if (!existing) await subscription.unsubscribe().catch(() => false);
    throw error;
  }
}

export async function disablePush(apiBaseUrl: string, options: PushOptions = {}): Promise<void> {
  const runtime = options.runtime ?? createBrowserRuntime();
  if (!runtime?.supported) throw new Error("Bu tarayıcı Web Push bildirimlerini desteklemiyor.");

  const registration = await ensureRegistration(runtime);
  const subscription = await registration.pushManager.getSubscription();
  if (!subscription) return;

  await deleteSubscription(apiBaseUrl, subscription.endpoint, options.fetcher);
  const removed = await subscription.unsubscribe();
  if (!removed) throw new Error("Tarayıcı bildirim aboneliğini kaldıramadı.");
}

export async function fetchPublicKey(apiBaseUrl: string, fetcher: typeof fetch = fetch): Promise<string> {
  const response = await fetcher(apiURL(apiBaseUrl, publicKeyPath), {
    headers: { Accept: "application/json" },
  });
  if (!response.ok) throw new Error(await responseError(response, "Bildirim anahtarı alınamadı."));

  const payload = (await response.json()) as { public_key?: unknown };
  if (typeof payload.public_key !== "string" || !payload.public_key.trim()) {
    throw new Error("Bildirim anahtarı geçersiz.");
  }
  return payload.public_key.trim();
}

export async function upsertSubscription(
  apiBaseUrl: string,
  subscription: Pick<PushSubscription, "toJSON">,
  fetcher: typeof fetch = fetch,
): Promise<void> {
  const response = await fetcher(apiURL(apiBaseUrl, subscriptionsPath), {
    method: "PUT",
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
    },
    body: JSON.stringify(subscription.toJSON()),
  });
  if (!response.ok) throw new Error(await responseError(response, "Bildirim aboneliği kaydedilemedi."));
}

export async function deleteSubscription(
  apiBaseUrl: string,
  endpoint: string,
  fetcher: typeof fetch = fetch,
): Promise<void> {
  const response = await fetcher(apiURL(apiBaseUrl, subscriptionsPath), {
    method: "DELETE",
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ endpoint }),
  });
  if (!response.ok) throw new Error(await responseError(response, "Bildirim aboneliği kaldırılamadı."));
}

function createBrowserRuntime(): PushRuntime | undefined {
  if (typeof window === "undefined" || typeof navigator === "undefined") return undefined;

  const supported = window.isSecureContext
    && "serviceWorker" in navigator
    && "PushManager" in window
    && "Notification" in window;

  return {
    supported,
    permission: () => Notification.permission,
    requestPermission: () => Notification.requestPermission(),
    getRegistration: () => navigator.serviceWorker.getRegistration(),
    register: (scriptURL) => navigator.serviceWorker.register(scriptURL),
    ready: () => navigator.serviceWorker.ready,
  };
}

async function ensureRegistration(runtime: PushRuntime): Promise<PushRegistration> {
  const current = await runtime.getRegistration();
  if (current) return current;
  await runtime.register("/sw.js");
  return runtime.ready();
}

function apiURL(apiBaseUrl: string, path: string): string {
  return `${apiBaseUrl.replace(/\/+$/, "")}${path}`;
}

async function responseError(response: Response, fallback: string): Promise<string> {
  try {
    const payload = (await response.json()) as { error?: unknown };
    return typeof payload.error === "string" && payload.error.trim() ? payload.error : fallback;
  } catch {
    return fallback;
  }
}
