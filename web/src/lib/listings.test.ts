import { describe, expect, it } from "vitest";
import { groupListings, toDateTimeLocal, upcomingDate, type Listing } from "./listings";

describe("groupListings", () => {
  it("separates primary companies from the digest", () => {
    const listings: Listing[] = [
      { id: "1", company: "Havelsan", title: "Staj", url: "/1", priority: "primary" },
      { id: "2", company: "Example", title: "Staj", url: "/2", priority: "candidate" },
    ];

    const result = groupListings(listings);

    expect(result.priority).toHaveLength(1);
    expect(result.other).toHaveLength(1);
  });
});

describe("upcomingDate", () => {
  it("selects the earliest analysis or manually tracked date", () => {
    const listing: Listing = {
      id: "1",
      company: "Havelsan",
      title: "Staj",
      url: "/1",
      priority: "primary",
      application_deadline: "2026-08-20T18:00:00Z",
      tracking_deadline: "2026-08-10T18:00:00Z",
      interview_at: "2026-08-15T09:00:00Z",
    };

    expect(upcomingDate(listing)).toBe("2026-08-10T18:00:00Z");
  });
});

describe("toDateTimeLocal", () => {
  it("returns an empty value for missing or invalid dates", () => {
    expect(toDateTimeLocal()).toBe("");
    expect(toDateTimeLocal("invalid")).toBe("");
  });
});
