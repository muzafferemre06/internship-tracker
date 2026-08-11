export type CoverageStatus = "automatic" | "feed" | "manual" | "researching" | "broken";
export type ProgramStatus = "open" | "closed" | "unknown";

export type CoverageSource = {
  source_id: string;
  type: string;
  url: string;
  adapter: string;
  strategy: string;
  status: CoverageStatus;
  reason?: string;
  trust_level: "official_company" | "official_ats" | "verified_newsletter" | "aggregator";
  enabled: boolean;
  last_success_at?: string;
  last_error?: string;
};

export type CoverageResponse = {
  summary: {
    total_companies: number;
    total_sources: number;
    automatic_sources: number;
    feed_sources: number;
    manual_sources: number;
    researching_sources: number;
    broken_sources: number;
    automatic_eligible_sources: number;
    automatic_coverage_percent: number;
  };
  companies: Array<{
    name: string;
    priority: string;
    tracking_status: string;
    sources: CoverageSource[];
  }>;
  programs: Array<{
    program_id: string;
    company: string;
    name: string;
    type: string;
    url: string;
    status: ProgramStatus;
    opens_at?: string;
    closes_at?: string;
    last_verified_at?: string;
  }>;
};

export const coverageStatusLabels: Record<CoverageStatus, string> = {
  automatic: "Otomatik",
  feed: "Akış",
  manual: "Manuel",
  researching: "Araştırılıyor",
  broken: "Bozuk",
};

export const programStatusLabels: Record<ProgramStatus, string> = {
  open: "Başvuru açık",
  closed: "Başvuru kapalı",
  unknown: "Durum bilinmiyor",
};

export function coverageTone(status: CoverageStatus): "success" | "neutral" | "warning" | "danger" {
  switch (status) {
    case "automatic":
    case "feed":
      return "success";
    case "manual":
      return "neutral";
    case "researching":
      return "warning";
    case "broken":
      return "danger";
  }
}

export function formatCoveragePercent(value: number): string {
  return `%${new Intl.NumberFormat("tr-TR", { maximumFractionDigits: 1 }).format(value)}`;
}
