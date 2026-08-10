import { describe, expect, it } from "vitest";
import { readJSONResponse } from "./api";

describe("readJSONResponse", () => {
  it("returns a safe message for an HTML proxy error without parsing or exposing its body", async () => {
    const response = new Response("<html><body>cloud proxy internals</body></html>", {
      status: 502,
      headers: { "Content-Type": "text/html" },
    });

    await expect(readJSONResponse(response)).rejects.toThrow("Sunucu beklenmeyen bir yanıt döndürdü (HTTP 502).");
    await expect(readJSONResponse(new Response("upstream failed", { status: 503 }))).rejects.not.toThrow("upstream failed");
  });

  it("uses the API error from a valid JSON error response", async () => {
    const response = new Response(JSON.stringify({ error: "a scan is already in progress" }), {
      status: 409,
      headers: { "Content-Type": "application/json; charset=utf-8" },
    });

    await expect(readJSONResponse(response)).rejects.toThrow("a scan is already in progress");
  });

  it("returns a successful JSON payload", async () => {
    const response = new Response(JSON.stringify({ status: "completed" }), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    });

    await expect(readJSONResponse<{ status: string }>(response)).resolves.toEqual({ status: "completed" });
  });
});
