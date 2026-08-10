export async function readJSONResponse<T>(response: Response): Promise<T> {
  const contentType = response.headers.get("Content-Type") ?? "";
  if (!/^application\/(?:[a-z0-9.+-]*\+)?json(?:\s*;|\s*$)/i.test(contentType)) {
    throw new Error(`Sunucu beklenmeyen bir yanıt döndürdü (HTTP ${response.status}).`);
  }

  let payload: unknown;
  try {
    payload = await response.json();
  } catch {
    throw new Error(`Sunucudan geçersiz JSON yanıtı alındı (HTTP ${response.status}).`);
  }
  if (!response.ok) {
    const error = typeof payload === "object" && payload !== null && "error" in payload
      ? (payload as { error?: unknown }).error
      : undefined;
    throw new Error(typeof error === "string" && error.trim() ? error : `İstek başarısız oldu (HTTP ${response.status}).`);
  }
  return payload as T;
}
