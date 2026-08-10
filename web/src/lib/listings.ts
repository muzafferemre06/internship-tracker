export type Listing = {
  id: string;
  opportunity_id?: string;
  lifecycle_status?: OpportunityLifecycle;
  company: string;
  title: string;
  url: string;
  priority: "primary" | "secondary" | "candidate";
  eligibility?: "uygun" | "kismen_uygun" | "uygun_degil" | "karar_bekliyor";
  summary?: string;
  application_deadline?: string;
  application_status?: ApplicationStatus;
  tracking_deadline?: string;
  interview_at?: string;
};

export type OpportunityLifecycle = "yeni" | "acik" | "incelendi" | "basvuruldu" | "suresi_doldu" | "kapatildi" | "arsivlendi";

export const opportunityLifecycleLabels: Record<OpportunityLifecycle, string> = {
  yeni: "Yeni",
  acik: "Açık",
  incelendi: "İncelendi",
  basvuruldu: "Başvuruldu",
  suresi_doldu: "Süresi doldu",
  kapatildi: "Kapatıldı",
  arsivlendi: "Arşivlendi",
};

export type ApplicationStatus =
  | "incelenecek"
  | "basvuruldu"
  | "sinav_mulakat"
  | "olumlu"
  | "olumsuz";

export const applicationStatusLabels: Record<ApplicationStatus, string> = {
  incelenecek: "İncelenecek",
  basvuruldu: "Başvuruldu",
  sinav_mulakat: "Sınav / mülakat",
  olumlu: "Olumlu",
  olumsuz: "Olumsuz",
};

export const eligibilityLabels: Record<NonNullable<Listing["eligibility"]>, string> = {
  uygun: "Uygun",
  kismen_uygun: "Kısmen uygun",
  uygun_degil: "Uygun değil",
  karar_bekliyor: "Karar bekliyor",
};

export function groupListings(listings: Listing[]) {
  return {
    priority: listings.filter((listing) => listing.priority === "primary"),
    other: listings.filter((listing) => listing.priority !== "primary"),
  };
}

export function formatDate(value?: string) {
  if (!value) return "—";
  return new Intl.DateTimeFormat("tr-TR", {
    dateStyle: "medium",
    timeStyle: value.includes("T") ? "short" : undefined,
  }).format(new Date(value));
}

export function upcomingDate(listing: Listing) {
  const dates = [listing.tracking_deadline, listing.interview_at, listing.application_deadline]
    .filter((value): value is string => Boolean(value))
    .map((value) => ({ value, time: new Date(value).getTime() }))
    .filter(({ time }) => Number.isFinite(time))
    .sort((left, right) => left.time - right.time);
  return dates[0]?.value;
}

export function toDateTimeLocal(value?: string) {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60_000);
  return local.toISOString().slice(0, 16);
}
