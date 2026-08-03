import { useEffect, useState } from "react";
import { groupListings, type Listing } from "./lib/listings";

type DashboardResponse = {
  new_listings: Listing[];
  needs_decision: Listing[];
  active_applications: Listing[];
  last_scan: null | {
    id: number;
    finished_at: string;
    status: "completed" | "partial" | "failed";
    sources_succeeded: number;
    sources_failed: number;
    new_listings_count: number;
    error_summary?: string;
  };
};

type ScanResponse = {
  run_id: number;
  status: "completed" | "partial" | "failed";
  found: number;
  new: number;
  process_errors: number;
};

const apiBaseUrl = import.meta.env.DEV
  ? (import.meta.env.VITE_API_BASE_URL ?? "http://localhost:8080")
  : (import.meta.env.VITE_API_BASE_URL ?? "");

const emptyDashboard: DashboardResponse = {
  new_listings: [],
  needs_decision: [],
  active_applications: [],
  last_scan: null,
};

export default function App() {
  const [dashboard, setDashboard] = useState<DashboardResponse>(emptyDashboard);
  const [message, setMessage] = useState("Dashboard yükleniyor…");
  const [scanning, setScanning] = useState(false);

  useEffect(() => {
    void loadDashboard();
  }, []);

  async function loadDashboard() {
    try {
      const response = await fetch(`${apiBaseUrl}/api/v1/dashboard`);
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      const data = (await response.json()) as DashboardResponse;
      setDashboard(data);
      setMessage(data.last_scan ? "Son tarama yüklendi." : "Henüz tarama yapılmadı.");
    } catch {
      setMessage("Backend'e ulaşılamadı. API'nin çalıştığını kontrol et.");
    }
  }

  async function startScan() {
    setScanning(true);
    setMessage("Tarama başlatılıyor…");
    try {
      const response = await fetch(`${apiBaseUrl}/api/v1/scan`, { method: "POST" });
      if (!response.ok) {
        const payload = (await response.json()) as { error?: string };
        throw new Error(payload.error ?? `HTTP ${response.status}`);
      }
      const result = (await response.json()) as ScanResponse;
      await loadDashboard();
      const warning = result.status === "completed" ? "" : " Bazı kaynaklar tamamlanamadı.";
      setMessage(`${result.found} ilan kontrol edildi, ${result.new} yeni ilan bulundu.${warning}`);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Tarama başlatılamadı.");
    } finally {
      setScanning(false);
    }
  }

  const grouped = groupListings(dashboard.new_listings);

  return (
    <main className="shell">
      <header className="hero">
        <div>
          <p className="eyebrow">Kişisel kariyer asistanı</p>
          <h1>Staj Takip</h1>
          <p className="muted">Öncelikli şirketleri izle, yeni fırsatları kaçırma.</p>
        </div>
        <button type="button" onClick={startScan} disabled={scanning}>
          {scanning ? "Taranıyor…" : "Taramayı başlat"}
        </button>
      </header>

      <p className="status" role="status">{message}</p>
      {dashboard.last_scan ? (
        <p className="muted">
          Son tarama #{dashboard.last_scan.id}: {dashboard.last_scan.sources_succeeded} kaynak başarılı,
          {" "}{dashboard.last_scan.sources_failed} kaynak başarısız.
        </p>
      ) : null}

      <section className="metrics" aria-label="Özet">
        <article><strong>{grouped.priority.length}</strong><span>Öncelikli yeni ilan</span></article>
        <article><strong>{dashboard.needs_decision.length}</strong><span>Karar bekliyor</span></article>
        <article><strong>{dashboard.active_applications.length}</strong><span>Aktif başvuru</span></article>
      </section>

      <section className="panel">
        <div className="panel-heading">
          <h2>Yeni fırsatlar</h2>
          <span>{dashboard.new_listings.length} ilan</span>
        </div>
        {dashboard.new_listings.length === 0 ? (
          <p className="empty">Yeni ilan bulunmadı. İlk taramayla bu alan dolacak.</p>
        ) : (
          <ul className="listing-list">
            {dashboard.new_listings.map((listing) => (
              <li key={listing.id}>
                <a href={listing.url}>{listing.company} — {listing.title}</a>
              </li>
            ))}
          </ul>
        )}
      </section>
    </main>
  );
}
