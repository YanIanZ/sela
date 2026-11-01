import { useEffect, useState } from "react";
import { api, type Policy, type Profile } from "../api";
import { ConditionMapSection, DemoPractitioners } from "./ConditionMap";
import { useLocale, useWords } from "../shared";
import { ErrorNotice, Loading, PageHeader, TextField } from "../ui";
import { LinkButton } from "../ui";

type Entry = {
  id: string;
  kind: "organization" | "facility" | "practitioner";
  name: string;
  city?: string;
  description?: string;
  contactUrl?: string;
  sourceUrl: string;
  sourcePublisher: string;
  sourceAccessedAt: string;
  synthetic: boolean;
  reverifyAt?: string;
};

const kinds = [
  { value: "", id: "Semua", en: "All" },
  { value: "organization", id: "Organisasi", en: "Organization" },
  { value: "facility", id: "Fasilitas", en: "Facility" },
  { value: "practitioner", id: "Praktisi", en: "Practitioner" },
] as const;

export function PlannedCarePage({ user, policies }: { user?: Profile | null; policies?: Record<string, Policy> }) {
  const w = useWords();
  const loc = useLocale();
  const [q, setQ] = useState("");
  const [city, setCity] = useState("");
  const [kind, setKind] = useState("");
  const [entries, setEntries] = useState<Entry[] | null>(null);
  const [error, setError] = useState<unknown>(null);

  useEffect(() => {
    const controller = new AbortController();
    const timer = window.setTimeout(() => {
      setError(null);
      const params = new URLSearchParams();
      if (q.trim()) params.set("q", q.trim());
      if (city.trim()) params.set("city", city.trim());
      if (kind) params.set("kind", kind);
      api<{ entries: Entry[] }>("/care-directory?" + params.toString(), undefined, "GET", controller.signal)
        .then((d) => setEntries(d.entries ?? []))
        .catch((e) => {
          if (!controller.signal.aborted) {
            setError(e);
            setEntries([]);
          }
        });
    }, 220);
    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [q, city, kind]);

  return (
    <div className="c-care">
      <PageHeader
        back={{ to: "/support", label: w("Dukungan", "Support") }}
        title={w("Bantuan terencana", "Planned support")}
        lead={w(
          "Cari organisasi, fasilitas, atau praktisi dari sumber publik. Kamu yang memilih dan menghubungi; Sela tidak memesan atau mengirim permintaan.",
          "Find organizations, facilities or practitioners from public sources. You choose and contact them; Sela does not book or send a request.",
        )}
      />
      {user && <ConditionMapSection policy={policies?.condition_map} />}
      <h2 className="c-care-directory-title">{w("Direktori dari sumber publik", "Directory from public sources")}</h2>
      <div className="c-care-intro">
        <p>
          {w(
            "Tidak perlu lokasi. Kota hanya kamu isi sendiri jika ingin mempersempit hasil. Hasil bukan janji ketersediaan, jadwal, atau kerja sama.",
            "No location is needed. Type a city yourself only if you want to narrow the results. A result is not a promise of availability, schedule or partnership.",
          )}
        </p>
        {entries?.some((e) => e.synthetic) && (
          <p className="c-care-sim-note" role="note">
            {w(
              "Entri bertanda Simulasi adalah data fiktif untuk demo, bukan fasilitas atau orang nyata.",
              "Entries marked Simulation are fictional demo data, not real facilities or people.",
            )}
          </p>
        )}
      </div>
      <div className="c-care-filters">
        <TextField
          type="search"
          label={w("Cari nama atau sumber", "Search name or source")}
          value={q}
          onChange={(e) => setQ(e.target.value)}
        />
        <TextField
          label={w("Kota (opsional)", "City (optional)")}
          value={city}
          onChange={(e) => setCity(e.target.value)}
        />
        <div className="c-care-kinds" role="group" aria-label={w("Jenis", "Kind")}>
          {kinds.map((k) => (
            <button
              key={k.value}
              type="button"
              className="c-care-kind"
              aria-pressed={kind === k.value}
              onClick={() => setKind(k.value)}
            >
              {w(k.id, k.en)}
            </button>
          ))}
        </div>
      </div>
      <ErrorNotice error={error} />
      {entries === null && !error && <Loading label={w("Mencari sumber…", "Searching sources…")} />}
      {entries?.length === 0 && !error && (
        <section className="s-empty">
          <h2>{w("Belum ada hasil", "No results yet")}</h2>
          <p>
            {w(
              "Coba kata lain atau kosongkan kota. Direktori ini hanya menampilkan entri yang sudah diverifikasi.",
              "Try another word or clear the city. This directory shows only verified entries.",
            )}
          </p>
        </section>
      )}
      {entries && entries.length > 0 && (
        <div className="c-care-grid">
          {entries.map((e) => (
            <article className="c-care-card" key={e.id}>
              <div className="c-care-tags">
                <span className="c-care-kind-label">
                  {e.synthetic
                    ? w("Simulasi", "Simulation")
                    : e.kind === "facility"
                      ? w("Fasilitas", "Facility")
                      : e.kind === "practitioner"
                        ? w("Praktisi", "Practitioner")
                        : w("Organisasi", "Organization")}
                </span>
                {e.city && <span className="s-hint">{e.city}</span>}
              </div>
              <h2>{e.name}</h2>
              {e.description && <p>{e.description}</p>}
              <p className="s-hint">
                {w("Sumber", "Source")}: {e.sourcePublisher} · {loc.shortDate(e.sourceAccessedAt)}
                {e.reverifyAt ? ` · ${w("Diverifikasi ulang", "Reverify by")} ${loc.shortDate(e.reverifyAt)}` : ""}
              </p>
              {e.synthetic ? (
                // A simulated entry has no real source or contact: no link
                // may suggest it can be reached.
                <p className="s-hint c-care-synthetic">
                  {w(
                    "Entri simulasi: tidak ada sumber atau kontak nyata.",
                    "Simulated entry: no real source or contact.",
                  )}
                </p>
              ) : (
                <div className="c-care-actions">
                  <a href={e.sourceUrl} target="_blank" rel="noopener noreferrer" referrerPolicy="no-referrer">
                    {w("Lihat sumber resmi", "Open official source")}
                    <span className="s-sr-only">{w(" (tab baru)", " (new tab)")}</span>
                  </a>
                  {e.contactUrl && (
                    <a href={e.contactUrl} target="_blank" rel="noopener noreferrer" referrerPolicy="no-referrer">
                      {w("Buka kontak resmi", "Open official contact")}
                      <span className="s-sr-only">{w(" (tab baru)", " (new tab)")}</span>
                    </a>
                  )}
                </div>
              )}
            </article>
          ))}
        </div>
      )}
      {!user && <DemoPractitioners />}
      <p className="s-hint c-care-foot">
        {w(
          "Sela tidak menghubungi siapa pun dan tidak membagikan ceritamu ke entri ini. Untuk bantuan mendesak, buka Bantuan sekarang.",
          "Sela contacts no one and does not share your story with these entries. For urgent help, open Help now.",
        )}
      </p>
      <div className="s-actions">
        <LinkButton to="/help" variant="help" icon="help">
          {w("Bantuan sekarang", "Help now")}
        </LinkButton>
      </div>
    </div>
  );
}
