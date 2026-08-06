import { describe, expect, it } from "vitest";
import { listingIDFromURL, urlWithListing } from "./navigation";

describe("listing deep links", () => {
  it("reads a listing id from the query string", () => {
    expect(listingIDFromURL("https://app.example/?listing=listing%2F42")).toBe("listing/42");
    expect(listingIDFromURL("https://app.example/?listing=%20")).toBeNull();
  });

  it("adds and clears only the listing query parameter", () => {
    expect(urlWithListing("https://app.example/dashboard?filter=new#top", "listing/42"))
      .toBe("/dashboard?filter=new&listing=listing%2F42#top");
    expect(urlWithListing("https://app.example/?listing=42&filter=new", null))
      .toBe("/?filter=new");
  });
});
