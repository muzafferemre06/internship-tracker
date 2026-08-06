import { useEffect, useMemo, useState } from "react";
import {
  applicationStatusLabels,
  eligibilityLabels,
  formatDate,
  groupListings,
  toDateTimeLocal,
  upcomingDate,
  type ApplicationStatus,
  type Listing,
} from "./lib/listings";
import { listingIDFromURL, urlWithListing } from "./lib/navigation";
import { disablePush, enablePush, getPushStatus, type PushStatus } from "./lib/push";

type ManualCheck = {
  source_id: string;
  company: string;
  url: string;
  reason: string;
  last_success_at?: string;
};

type DashboardResponse = {
  new_listings: Listing[];
  needs_decision: Listing[];
  active_applications: Listing[];
  manual_checks: ManualCheck[];
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

type ListingDetail = Listing & {
  opportunity_type?: string;
  application_open: boolean;
  relevant: boolean;
  matching_areas: string[];
  class_year_requirement?: number;
  gpa_requirement?: number;
  location?: string;
  work_model?: string;
  confidence: number;
  needs_user_decision: boolean;
  decision_question?: string;
  first_seen_at: string;
  last_seen_at: string;
  application?: {
    status: ApplicationStatus;
    deadline?: string;
    interview_at?: string;
    notes?: string;
  };
};

type ScanResponse = {
  run_id: number;
  status: "completed" | "partial" | "failed";
  found: number;
  new: number;
  process_errors: number;
  sources: Array<{ source: string; skipped?: boolean; retry_at?: string }>;
};

type ApplicationForm = {
  status: ApplicationStatus;
  deadline: string;
  interviewAt: string;
  notes: string;
};

const apiBaseUrl = import.meta.env.DEV
  ? (import.meta.env.VITE_API_BASE_URL ?? "http://localhost:8080")
  : (import.meta.env.VITE_API_BASE_URL ?? "");

const emptyDashboard: DashboardResponse = {
  new_listings: [],
  needs_decision: [],
  active_applications: [],
  manual_checks: [],
  last_scan: null,
};

const emptyApplication: ApplicationForm = {
  status: "incelenecek",
  deadline: "",
  interviewAt: "",
  notes: "",
};

const pushStatusLabels: Record<PushStatus | "loading", string> = {
  loading: "Bildirim durumu kontrol ediliyor…",
  unsupported: "Bu cihazda bildirim kullanılamıyor.",
  denied: "Bildirim izni tarayıcıda engellenmiş.",
  subscribed: "Yeni uygun ilan bildirimleri açık.",
  unsubscribed: "Yeni ilan bildirimleri kapalı.",
};

export default function App() {
  const [dashboard, setDashboard] = useState<DashboardResponse>(emptyDashboard);
  const [message, setMessage] = useState("Dashboard yükleniyor…");
  const [scanning, setScanning] = useState(false);
  const [selected, setSelected] = useState<ListingDetail | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [application, setApplication] = useState<ApplicationForm>(emptyApplication);
  const [pushStatus, setPushStatus] = useState<PushStatus | "loading">("loading");
  const [pushBusy, setPushBusy] = useState(false);

  useEffect(() => {
    void loadDashboard();

    const listingID = listingIDFromURL(window.location.href);
    if (listingID) void openListingByID(listingID);

    const handlePopState = () => {
      const nextListingID = listingIDFromURL(window.location.href);
      if (nextListingID) void openListingByID(nextListingID);
      else setSelected(null);
    };
    window.addEventListener("popstate", handlePopState);
    return () => window.removeEventListener("popstate", handlePopState);
  }, []);

  useEffect(() => {
    void getPushStatus()
      .then(setPushStatus)
      .catch(() => setPushStatus("unsupported"));
  }, []);

  async function loadDashboard() {
    try {
      const response = await fetch(`${apiBaseUrl}/api/v1/dashboard`);
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      const data = (await response.json()) as DashboardResponse;
      setDashboard({ ...emptyDashboard, ...data });
      setMessage(data.last_scan ? "Güncel tarama sonucu hazır." : "Henüz tarama yapılmadı.");
    } catch {
      setMessage("Backend'e ulaşılamadı. API'nin çalıştığını kontrol et.");
    }
  }

  async function startScan() {
    setScanning(true);
    setMessage("Kaynaklar taranıyor…");
    try {
      const response = await fetch(`${apiBaseUrl}/api/v1/scan`, { method: "POST" });
      if (!response.ok) {
        const payload = (await response.json()) as { error?: string };
        throw new Error(payload.error ?? `HTTP ${response.status}`);
      }
      const result = (await response.json()) as ScanResponse;
      await loadDashboard();
      const retryAt = result.sources.find((source) => source.retry_at)?.retry_at;
      const retryMessage = retryAt ? ` Tekrar deneme: ${formatDate(retryAt)}.` : "";
      const warning = result.status === "completed" ? "" : ` Bazı kaynaklar tamamlanamadı.${retryMessage}`;
      setMessage(`${result.found} ilan kontrol edildi, ${result.new} yeni ilan bulundu.${warning}`);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Tarama başlatılamadı.");
    } finally {
      setScanning(false);
    }
  }

  async function openListing(listing: Listing) {
    await openListingByID(listing.id, listing.company, true);
  }

  async function openListingByID(listingID: string, company?: string, updateURL = false) {
    setDetailLoading(true);
    setMessage(company ? `${company} ilanı yükleniyor…` : "İlan yükleniyor…");
    try {
      const response = await fetch(`${apiBaseUrl}/api/v1/listings/${encodeURIComponent(listingID)}`);
      if (!response.ok) throw new Error(`İlan yüklenemedi (HTTP ${response.status}).`);
      const detail = (await response.json()) as ListingDetail;
      setSelected(detail);
      setApplication({
        status: detail.application?.status ?? "incelenecek",
        deadline: toDateTimeLocal(detail.application?.deadline),
        interviewAt: toDateTimeLocal(detail.application?.interview_at),
        notes: detail.application?.notes ?? "",
      });
      if (updateURL && listingIDFromURL(window.location.href) !== detail.id) {
        window.history.pushState(null, "", urlWithListing(window.location.href, detail.id));
      }
      setMessage("İlan ayrıntıları hazır.");
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "İlan yüklenemedi.");
    } finally {
      setDetailLoading(false);
    }
  }

  function closeListing() {
    setSelected(null);
    window.history.replaceState(null, "", urlWithListing(window.location.href, null));
  }

  async function togglePush() {
    if (pushStatus === "loading" || pushStatus === "unsupported" || pushStatus === "denied") return;
    setPushBusy(true);
    try {
      if (pushStatus === "subscribed") {
        await disablePush(apiBaseUrl);
        setPushStatus("unsubscribed");
        setMessage("Yeni ilan bildirimleri kapatıldı.");
      } else {
        await enablePush(apiBaseUrl);
        setPushStatus("subscribed");
        setMessage("Yeni uygun ilan bildirimleri açıldı.");
      }
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Bildirim tercihi güncellenemedi.");
      setPushStatus(await getPushStatus().catch(() => "unsupported"));
    } finally {
      setPushBusy(false);
    }
  }

  async function saveApplication(event: React.FormEvent) {
    event.preventDefault();
    if (!selected) return;
    setSaving(true);
    try {
      const response = await fetch(`${apiBaseUrl}/api/v1/listings/${encodeURIComponent(selected.id)}/application`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          status: application.status,
          deadline: application.deadline ? new Date(application.deadline).toISOString() : null,
          interview_at: application.interviewAt ? new Date(application.interviewAt).toISOString() : null,
          notes: application.notes,
        }),
      });
      const payload = (await response.json()) as ListingDetail | { error?: string };
      if (!response.ok) throw new Error("error" in payload ? (payload.error ?? "Başvuru kaydedilemedi.") : "Başvuru kaydedilemedi.");
      setSelected(payload as ListingDetail);
      await loadDashboard();
      setMessage("Başvuru durumu ve tarihler kaydedildi.");
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Başvuru kaydedilemedi.");
    } finally {
      setSaving(false);
    }
  }

  const grouped = groupListings(dashboard.new_listings);
  const upcoming = useMemo(
    () => dashboard.active_applications
      .map((listing) => ({ listing, date: upcomingDate(listing) }))
      .filter((item): item is { listing: Listing; date: string } => Boolean(item.date))
      .sort((left, right) => new Date(left.date).getTime() - new Date(right.date).getTime()),
    [dashboard.active_applications],
  );

  return (
    <main className="shell">
      <header className="hero">
        <div>
          <p className="eyebrow">Kişisel kariyer asistanı</p>
          <h1>Staj Takip</h1>
          <p className="muted">İlanı bul, kararını ver, başvuru tarihlerini tek yerde tut.</p>
        </div>
        <div className="hero-actions">
          <button
            className="notification-button"
            type="button"
            aria-pressed={pushStatus === "subscribed"}
            onClick={togglePush}
            disabled={pushBusy || pushStatus === "loading" || pushStatus === "unsupported" || pushStatus === "denied"}
          >
            {pushBusy ? "Güncelleniyor…" : pushStatus === "subscribed" ? "Bildirimleri kapat" : "Bildirimleri aç"}
          </button>
          <span className="notification-state">{pushStatusLabels[pushStatus]}</span>
          <button type="button" onClick={startScan} disabled={scanning}>
            {scanning ? "Taranıyor…" : "Taramayı başlat"}
          </button>
        </div>
      </header>

      <div className="status-row">
        <p className="status" role="status">{message}</p>
        {dashboard.last_scan ? (
          <span className={`scan-badge ${dashboard.last_scan.status}`}>
            Son tarama: {dashboard.last_scan.sources_succeeded} başarılı / {dashboard.last_scan.sources_failed} sorunlu
          </span>
        ) : null}
      </div>

      <section className="metrics" aria-label="Özet">
        <article><strong>{grouped.priority.length}</strong><span>Öncelikli yeni ilan</span></article>
        <article><strong>{dashboard.needs_decision.length}</strong><span>Karar bekliyor</span></article>
        <article><strong>{dashboard.active_applications.length}</strong><span>Aktif başvuru</span></article>
        <article><strong>{dashboard.manual_checks.length}</strong><span>Manuel kontrol</span></article>
      </section>

      <div className="dashboard-grid">
        <ListingSection title="Yeni ve uygun fırsatlar" listings={dashboard.new_listings} empty="Yeni uygun ilan yok." onOpen={openListing} loading={detailLoading} />
        <ListingSection title="Karar bekleyenler" listings={dashboard.needs_decision} empty="Yanıt bekleyen karar yok." onOpen={openListing} loading={detailLoading} />
        <ListingSection title="Aktif başvurular" listings={dashboard.active_applications} empty="Henüz takip edilen başvuru yok." onOpen={openListing} loading={detailLoading} />

        <section className="panel">
          <div className="panel-heading"><h2>Yaklaşan tarihler</h2><span>{upcoming.length}</span></div>
          {upcoming.length === 0 ? <p className="empty">Başvurulara manuel tarih ekleyebilirsin.</p> : (
            <ul className="timeline">
              {upcoming.map(({ listing, date }) => (
                <li key={`${listing.id}-${date}`}>
                  <time dateTime={date}>{formatDate(date)}</time>
                  <button className="text-button" type="button" onClick={() => openListing(listing)}>{listing.company} — {listing.title}</button>
                </li>
              ))}
            </ul>
          )}
        </section>

        <section className="panel manual-panel">
          <div className="panel-heading"><h2>Manuel kontrol listesi</h2><span>{dashboard.manual_checks.length}</span></div>
          {dashboard.manual_checks.length === 0 ? <p className="empty">Manuel kontrol gerektiren kaynak yok.</p> : (
            <ul className="manual-list">
              {dashboard.manual_checks.map((check) => (
                <li key={check.source_id}>
                  <div><strong>{check.company}</strong><p>{check.reason}</p></div>
                  <a href={check.url} target="_blank" rel="noreferrer">Kaynağı aç <span aria-hidden="true">↗</span></a>
                </li>
              ))}
            </ul>
          )}
        </section>
      </div>

      {selected ? (
        <div className="detail-backdrop" role="presentation" onMouseDown={(event) => {
          if (event.currentTarget === event.target) closeListing();
        }}>
          <aside className="detail-panel" role="dialog" aria-modal="true" aria-labelledby="detail-title">
            <button className="close-button" type="button" aria-label="İlan detayını kapat" onClick={closeListing}>×</button>
            <p className="eyebrow dark">{selected.company}</p>
            <h2 id="detail-title">{selected.title}</h2>
            <div className="chips">
              {selected.eligibility ? <span>{eligibilityLabels[selected.eligibility]}</span> : null}
              {selected.location ? <span>{selected.location}</span> : null}
              {selected.work_model ? <span>{selected.work_model}</span> : null}
              {selected.application_deadline ? <span>Son başvuru {formatDate(selected.application_deadline)}</span> : null}
            </div>
            {selected.decision_question ? <div className="decision-box"><strong>Kararın gerekiyor</strong><p>{selected.decision_question}</p></div> : null}
            <section className="detail-section">
              <h3>Uygunluk özeti</h3>
              <p>{selected.summary || "Analiz özeti bulunmuyor."}</p>
              {selected.matching_areas.length > 0 ? <p><strong>Eşleşen alanlar:</strong> {selected.matching_areas.join(", ")}</p> : null}
              <a className="external-link" href={selected.url} target="_blank" rel="noreferrer">Orijinal ilanı aç ↗</a>
            </section>
            <form className="tracking-form" onSubmit={saveApplication}>
              <h3>Başvuru takibi</h3>
              <label>Durum
                <select value={application.status} onChange={(event) => setApplication({ ...application, status: event.target.value as ApplicationStatus })}>
                  {Object.entries(applicationStatusLabels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
                </select>
              </label>
              <div className="form-row">
                <label>Takip / son tarih
                  <input type="datetime-local" value={application.deadline} onChange={(event) => setApplication({ ...application, deadline: event.target.value })} />
                </label>
                <label>Sınav / mülakat
                  <input type="datetime-local" value={application.interviewAt} onChange={(event) => setApplication({ ...application, interviewAt: event.target.value })} />
                </label>
              </div>
              <label>Notlar
                <textarea maxLength={2000} rows={4} value={application.notes} onChange={(event) => setApplication({ ...application, notes: event.target.value })} placeholder="Başvuru bağlantısı, dönüş notu veya hazırlanılacak konular…" />
              </label>
              <button type="submit" disabled={saving}>{saving ? "Kaydediliyor…" : "Başvuruyu kaydet"}</button>
            </form>
          </aside>
        </div>
      ) : null}
    </main>
  );
}

function ListingSection({
  title,
  listings,
  empty,
  onOpen,
  loading,
}: {
  title: string;
  listings: Listing[];
  empty: string;
  onOpen: (listing: Listing) => void;
  loading: boolean;
}) {
  return (
    <section className="panel">
      <div className="panel-heading"><h2>{title}</h2><span>{listings.length}</span></div>
      {listings.length === 0 ? <p className="empty">{empty}</p> : (
        <ul className="listing-list">
          {listings.map((listing) => (
            <li key={listing.id}>
              <button className="listing-card" type="button" disabled={loading} onClick={() => onOpen(listing)}>
                <span className="listing-meta">
                  <strong>{listing.company}</strong>
                  {listing.eligibility ? <small>{eligibilityLabels[listing.eligibility]}</small> : null}
                </span>
                <span className="listing-title">{listing.title}</span>
                {listing.summary ? <span className="listing-summary">{listing.summary}</span> : null}
                {upcomingDate(listing) ? <time dateTime={upcomingDate(listing)}>Yaklaşan: {formatDate(upcomingDate(listing))}</time> : null}
              </button>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
