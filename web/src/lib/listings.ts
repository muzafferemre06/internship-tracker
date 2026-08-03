export type Listing = {
  id: string;
  company: string;
  title: string;
  url: string;
  priority: "primary" | "secondary" | "candidate";
};

export function groupListings(listings: Listing[]) {
  return {
    priority: listings.filter((listing) => listing.priority === "primary"),
    other: listings.filter((listing) => listing.priority !== "primary"),
  };
}
