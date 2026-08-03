import { describe, expect, it } from "vitest";
import { groupListings, type Listing } from "./listings";

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
