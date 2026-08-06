export function listingIDFromURL(value: string | URL): string | null {
  const id = new URL(value, "http://localhost").searchParams.get("listing")?.trim();
  return id || null;
}

export function urlWithListing(value: string | URL, listingID: string | null): string {
  const url = new URL(value, "http://localhost");
  if (listingID) url.searchParams.set("listing", listingID);
  else url.searchParams.delete("listing");
  return `${url.pathname}${url.search}${url.hash}`;
}
