export type CoverageStatus = "automatic" | "feed" | "manual" | "researching" | "broken";
export type ProgramStatus = "open" | "closed" | "unknown";
export type CoveragePriority = "primary" | "secondary";
export type CoverageSection = CoveragePriority | "phase_16_5";
export type CoverageReasonCode = "account_required" | "third_party_restricted" | "no_public_job_source" | "client_rendered_unverified" | "periodic_program" | "source_unreachable";

export type CoverageSource = {
  source_id: string;
  type: string;
  url: string;
  adapter: string;
  strategy: string;
  status: CoverageStatus;
  reason?: string;
  reason_code?: CoverageReasonCode;
  last_verified_at?: string;
  trust_level: "official_company" | "official_ats" | "verified_newsletter" | "aggregator";
  enabled: boolean;
  last_success_at?: string;
  last_error?: string;
};

export type CoverageSummary = {
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

export type CoverageResponse = {
  summary: CoverageSummary;
  priority_summaries: Record<CoveragePriority, CoverageSummary>;
  section_summaries: Record<CoverageSection, CoverageSummary>;
  companies: Array<{
    name: string;
    priority: CoveragePriority;
    tracking_status: string;
    tracking_phase?: "16.5";
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

export function coverageForPriority(coverage: CoverageResponse, priority: CoveragePriority): CoverageSummary {
  return coverage.priority_summaries[priority];
}

export function coverageForSection(coverage: CoverageResponse, section: CoverageSection): CoverageSummary {
  return coverage.section_summaries[section];
}

export function companiesForSection(coverage: CoverageResponse, section: CoverageSection): CoverageResponse["companies"] {
  return coverage.companies.filter((company) => {
    if (section === "phase_16_5") return company.tracking_phase === "16.5";
    return company.tracking_phase !== "16.5" && company.priority === section;
  });
}

export const coverageStatusLabels: Record<CoverageStatus, string> = {
  automatic: "Otomatik",
  feed: "Akış",
  manual: "Manuel",
  researching: "Araştırılıyor",
  broken: "Bozuk",
};

export const coverageReasonLabels: Record<CoverageReasonCode, string> = {
  account_required: "Aday hesabı gerekiyor",
  third_party_restricted: "Üçüncü taraf erişimi kısıtlı",
  no_public_job_source: "Açık ilan kaynağı yok",
  client_rendered_unverified: "İstemci tarafı akışı doğrulanamadı",
  periodic_program: "Dönemsel program",
  source_unreachable: "Kaynak güvenli istemciyle erişilemiyor",
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
