import { describe, expect, it, vi } from "vitest";
import {
  deleteSubscription,
  disablePush,
  enablePush,
  fetchPublicKey,
  getPushStatus,
  urlBase64ToUint8Array,
  type PushRuntime,
} from "./push";

function response(body: unknown, status = 200): Response {
  return new Response(body === undefined ? null : JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function runtime(overrides: Partial<PushRuntime> = {}) {
  const subscription: Pick<PushSubscription, "endpoint" | "toJSON" | "unsubscribe"> = {
    endpoint: "https://push.example/subscription-1",
    toJSON: () => ({
      endpoint: "https://push.example/subscription-1",
      expirationTime: null,
      keys: { p256dh: "p256dh", auth: "auth" },
    }),
    unsubscribe: vi.fn(async () => true),
  };
  const registration = {
    pushManager: {
      getSubscription: vi.fn<() => Promise<typeof subscription | null>>(async () => null),
      subscribe: vi.fn<(options: PushSubscriptionOptionsInit) => Promise<typeof subscription>>(async () => subscription),
    },
  };
  const value: PushRuntime = {
    supported: true,
    permission: () => "granted",
    requestPermission: vi.fn(async () => "granted" as NotificationPermission),
    getRegistration: vi.fn(async () => registration),
    register: vi.fn(async () => registration),
    ready: vi.fn(async () => registration),
    ...overrides,
  };
  return { value, registration, subscription };
}

describe("urlBase64ToUint8Array", () => {
  it("decodes URL-safe VAPID public keys", () => {
    expect([...urlBase64ToUint8Array("AQID-vs")]).toEqual([1, 2, 3, 250, 251]);
  });
});

describe("push API", () => {
  it("reads the public key without making a real network request", async () => {
    const fetcher = vi.fn(async () => response({ public_key: "public-key" })) as unknown as typeof fetch;

    await expect(fetchPublicKey("http://localhost:8080/", fetcher)).resolves.toBe("public-key");
    expect(fetcher).toHaveBeenCalledWith("http://localhost:8080/api/v1/push/public-key", {
      headers: { Accept: "application/json" },
    });
  });

  it("sends the endpoint when deleting a subscription", async () => {
    const fetcher = vi.fn(async () => response(undefined, 204)) as unknown as typeof fetch;

    await deleteSubscription("", "https://push.example/subscription-1", fetcher);

    expect(fetcher).toHaveBeenCalledWith("/api/v1/push/subscriptions", expect.objectContaining({
      method: "DELETE",
      body: JSON.stringify({ endpoint: "https://push.example/subscription-1" }),
    }));
  });
});

describe("push preference", () => {
  it("requests permission, subscribes and persists the browser payload", async () => {
    let permission: NotificationPermission = "default";
    const fixture = runtime({
      permission: () => permission,
      requestPermission: vi.fn(async () => {
        permission = "granted";
        return permission;
      }),
    });
    const fetcher = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      if (String(input).endsWith("/public-key")) return response({ public_key: "AQID-vs" });
      expect(init?.method).toBe("PUT");
      expect(JSON.parse(String(init?.body))).toEqual(fixture.subscription.toJSON());
      return response({}, 201);
    }) as unknown as typeof fetch;

    await enablePush("https://app.example", { runtime: fixture.value, fetcher });

    expect(fixture.value.requestPermission).toHaveBeenCalledOnce();
    expect(fixture.registration.pushManager.subscribe).toHaveBeenCalledWith({
      userVisibleOnly: true,
      applicationServerKey: new Uint8Array([1, 2, 3, 250, 251]),
    });
    expect(fetcher).toHaveBeenCalledTimes(2);
  });

  it("does not contact the API after permission is denied", async () => {
    const fixture = runtime({ permission: () => "denied" });
    const fetcher = vi.fn() as unknown as typeof fetch;

    await expect(enablePush("", { runtime: fixture.value, fetcher })).rejects.toThrow("Bildirim izni verilmedi");
    expect(fetcher).not.toHaveBeenCalled();
  });

  it("removes the server record before unsubscribing locally", async () => {
    const fixture = runtime();
    fixture.registration.pushManager.getSubscription.mockResolvedValue(fixture.subscription);
    const fetcher = vi.fn(async () => response(undefined, 204)) as unknown as typeof fetch;

    await disablePush("", { runtime: fixture.value, fetcher });

    expect(fetcher).toHaveBeenCalledOnce();
    expect(fixture.subscription.unsubscribe).toHaveBeenCalledOnce();
  });

  it("reports unsupported browsers without touching service workers", async () => {
    const fixture = runtime({ supported: false });

    await expect(getPushStatus({ runtime: fixture.value })).resolves.toBe("unsupported");
    expect(fixture.value.getRegistration).not.toHaveBeenCalled();
  });
});
