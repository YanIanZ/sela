import { useMemo, useState } from "react";
import { useI18n } from "../i18n";
import { useWords } from "../v3/shared";
import type { CareAudit, DirectoryEntry, SandboxRequest } from "./api";
import "./care-panels.css";

type Words = (id: string, en: string) => string;

const KIND_LABEL: Record<string, [string, string]> = {
  organization: ["Organisasi", "Organization"],
  facility: ["Fasilitas", "Facility"],
  practitioner: ["Praktisi", "Practitioner"],
};
const STATUS_LABEL: Record<string, [string, string]> = {
  draft: ["Draf", "Draft"],
  pending: ["Menunggu tinjauan", "Pending review"],
  verified: ["Terverifikasi", "Verified"],
  rejected: ["Ditolak", "Rejected"],
  stale: ["Kedaluwarsa", "Stale"],
  unpublished: ["Tidak diterbitkan", "Unpublished"],
  prepared: ["Disiapkan", "Prepared"],
  received: ["Diterima (simulasi)", "Received (simulation)"],
  acknowledged: ["Diakui (simulasi)", "Acknowledged (simulation)"],
  revoked: ["Dibatalkan", "Revoked"],
  closed: ["Selesai (simulasi)", "Closed (simulation)"],
};
const ACTION_LABEL: Record<string, [string, string]> = {
  "directory.create": ["Draf dibuat", "Draft created"],
  "directory.update": ["Draf diperbarui", "Draft updated"],
  "directory.submit": ["Diajukan untuk tinjauan", "Submitted for review"],
  "directory.verify": ["Entri diverifikasi", "Entry verified"],
  "directory.reject": ["Entri ditolak", "Entry rejected"],
  "directory.unpublish": ["Entri tidak diterbitkan", "Entry unpublished"],
  "sandbox.create": ["Permintaan simulasi dibuat", "Simulation request created"],
  "sandbox.transition": ["Status simulasi diubah", "Simulation status changed"],
};

function label(map: Record<string, [string, string]>, key: string, w: Words): string {
  const v = map[key];
  return v ? w(v[0], v[1]) : key;
}

function useDate() {
  const { lang } = useI18n();
  return useMemo(() => {
    const locale = lang === "en" ? "en-GB" : "id-ID";
    return (iso?: string) => {
      if (!iso) return "";
      const d = new Date(iso);
      return Number.isNaN(d.getTime())
        ? iso
        : new Intl.DateTimeFormat(locale, { dateStyle: "medium" }).format(d);
    };
  }, [lang]);
}

export function DirectoryPanel({
  entries,
  audit,
  busy,
  canWrite,
  canVerify,
  onCreate,
  onAction,
}: {
  entries: DirectoryEntry[] | null;
  audit: CareAudit[] | null;
  busy: string;
  canWrite: boolean;
  canVerify: boolean;
  onCreate: (input: Record<string, unknown>) => void;
  onAction: (id: string, action: "submit" | "verify" | "reject" | "unpublish", version: number) => void;
}) {
  const w = useWords();
  const date = useDate();
  const [form, setForm] = useState(false);
  const [name, setName] = useState("");
  const [kind, setKind] = useState("organization");
  const [city, setCity] = useState("");
  const [sourceUrl, setSourceUrl] = useState("");
  const [sourcePublisher, setSourcePublisher] = useState("");
  const [contactUrl, setContactUrl] = useState("");
  const [description, setDescription] = useState("");

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    onCreate({
      kind,
      name,
      city,
      description,
      contactUrl,
      sourceUrl,
      sourcePublisher,
      sourceAccessedAt: new Date().toISOString(),
    });
    setForm(false);
    setName("");
    setCity("");
    setSourceUrl("");
    setSourcePublisher("");
    setContactUrl("");
    setDescription("");
  };

  return (
    <section className="op-section" aria-labelledby="op-directory">
      <div className="op-section-head">
        <div>
          <h2 id="op-directory">{w("Direktori bantuan terencana", "Planned support directory")}</h2>
          <p className="op-hint">
            {w(
              "Entri publik harus punya sumber dan tanggal akses. Kreator dan peninjau harus berbeda; entri simulasi boleh diverifikasi sendiri untuk demo.",
              "Public entries need a source and access date. Creator and reviewer must differ; simulation entries may be self-verified for the demo.",
            )}
          </p>
        </div>
        {canWrite && (
          <button type="button" className="op-btn op-btn--secondary" onClick={() => setForm((v) => !v)}>
            {form ? w("Tutup formulir", "Close form") : w("Tambah draf", "Add draft")}
          </button>
        )}
      </div>
      {form && (
        <form className="op-care-form" onSubmit={submit}>
          <label className="op-field">
            <span>{w("Nama", "Name")}</span>
            <input className="op-input" required value={name} onChange={(e) => setName(e.target.value)} />
          </label>
          <label className="op-field">
            <span>{w("Jenis", "Kind")}</span>
            <select className="op-input" value={kind} onChange={(e) => setKind(e.target.value)}>
              <option value="organization">{w("Organisasi", "Organization")}</option>
              <option value="facility">{w("Fasilitas", "Facility")}</option>
              <option value="practitioner">{w("Praktisi", "Practitioner")}</option>
            </select>
          </label>
          <label className="op-field">
            <span>{w("Kota", "City")}</span>
            <input className="op-input" value={city} onChange={(e) => setCity(e.target.value)} />
          </label>
          <label className="op-field op-field--wide">
            <span>{w("URL sumber resmi", "Official source URL")}</span>
            <input className="op-input" type="url" required value={sourceUrl} onChange={(e) => setSourceUrl(e.target.value)} />
          </label>
          <label className="op-field">
            <span>{w("Penerbit sumber", "Source publisher")}</span>
            <input className="op-input" required value={sourcePublisher} onChange={(e) => setSourcePublisher(e.target.value)} />
          </label>
          <label className="op-field">
            <span>{w("URL kontak resmi", "Official contact URL")}</span>
            <input className="op-input" type="url" value={contactUrl} onChange={(e) => setContactUrl(e.target.value)} />
          </label>
          <label className="op-field op-field--wide">
            <span>{w("Deskripsi publik", "Public description")}</span>
            <textarea className="op-input" rows={2} value={description} onChange={(e) => setDescription(e.target.value)} />
          </label>
          <div className="op-actions">
            <button className="op-btn op-btn--primary" type="submit" disabled={busy === "directory-create"}>
              {busy === "directory-create" ? w("Menyimpan…", "Saving…") : w("Simpan draf", "Save draft")}
            </button>
          </div>
        </form>
      )}
      {entries === null ? (
        <p className="op-hint">{w("Memuat direktori…", "Loading directory…")}</p>
      ) : entries.length === 0 ? (
        <p className="op-hint">{w("Belum ada entri.", "No entries yet.")}</p>
      ) : (
        <ul className="op-care-list">
          {entries.map((e) => {
            const key = `directory:${e.id}`;
            return (
              <li key={e.id}>
                <div className="op-care-main">
                  <strong>{e.name}</strong>
                  <span className="op-care-tags">
                    <span className="op-badge op-badge--soft">{label(KIND_LABEL, e.kind, w)}</span>
                    <span className={`op-care-status op-care-status--${e.status}`}>{label(STATUS_LABEL, e.status, w)}</span>
                    {e.synthetic && <span className="op-badge op-badge--demo">{w("Simulasi", "Simulation")}</span>}
                    {e.city && <span className="op-hint">{e.city}</span>}
                  </span>
                  <p className="op-hint">
                    {w("Sumber", "Source")}: {e.sourcePublisher} · {date(e.sourceAccessedAt)}
                    {e.reverifyAt ? ` · ${w("Tinjau ulang", "Reverify")} ${date(e.reverifyAt)}` : ""}
                  </p>
                  {e.description && <p className="op-care-desc">{e.description}</p>}
                </div>
                <div className="op-care-actions">
                  {canWrite && (e.status === "draft" || e.status === "rejected") && (
                    <button type="button" className="op-btn op-btn--quiet" disabled={busy === key} onClick={() => onAction(e.id, "submit", e.version)}>
                      {w("Ajukan", "Submit")}
                    </button>
                  )}
                  {canVerify && e.status === "pending" && (
                    <>
                      <button type="button" className="op-btn op-btn--secondary" disabled={busy === key} onClick={() => onAction(e.id, "verify", e.version)}>
                        {w("Verifikasi", "Verify")}
                      </button>
                      <button type="button" className="op-btn op-btn--quiet op-btn--danger-text" disabled={busy === key} onClick={() => onAction(e.id, "reject", e.version)}>
                        {w("Tolak", "Reject")}
                      </button>
                    </>
                  )}
                  {canVerify && e.status === "verified" && (
                    <button type="button" className="op-btn op-btn--quiet" disabled={busy === key} onClick={() => onAction(e.id, "unpublish", e.version)}>
                      {w("Tarik dari publik", "Unpublish")}
                    </button>
                  )}
                </div>
              </li>
            );
          })}
        </ul>
      )}
      {audit && audit.length > 0 && (
        <details className="op-care-audit">
          <summary>{w("Aktivitas direktori", "Directory activity")}</summary>
          <ol>
            {audit.slice(0, 12).map((e) => (
              <li key={e.id}>
                <span>{label(ACTION_LABEL, e.action, w)}</span>
                <span className="op-hint">{date(e.createdAt)}</span>
              </li>
            ))}
          </ol>
        </details>
      )}
    </section>
  );
}

export function SandboxPanel({
  requests,
  busy,
  onCreate,
  onTransition,
}: {
  requests: SandboxRequest[] | null;
  busy: string;
  onCreate: (input: Record<string, unknown>) => void;
  onTransition: (id: string, version: number, status: string) => void;
}) {
  const w = useWords();
  const date = useDate();
  const [form, setForm] = useState(false);
  const [organization, setOrganization] = useState("");
  const [professional, setProfessional] = useState("");
  const [service, setService] = useState("");
  const [note, setNote] = useState("");
  const next: Record<string, Array<{ status: string; label: [string, string] }>> = {
    prepared: [{ status: "pending", label: ["Ajukan (simulasi)", "Submit (simulation)"] }, { status: "revoked", label: ["Batalkan", "Revoke"] }],
    pending: [{ status: "received", label: ["Tandai diterima (simulasi)", "Mark received (simulation)"] }, { status: "revoked", label: ["Batalkan", "Revoke"] }],
    received: [{ status: "acknowledged", label: ["Tandai diakui (simulasi)", "Mark acknowledged (simulation)"] }, { status: "revoked", label: ["Batalkan", "Revoke"] }],
    acknowledged: [{ status: "closed", label: ["Tutup (simulasi)", "Close (simulation)"] }, { status: "revoked", label: ["Batalkan", "Revoke"] }],
  };
  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    onCreate({ organization, professional, service, note });
    setForm(false);
    setOrganization("");
    setProfessional("");
    setService("");
    setNote("");
  };
  return (
    <section className="op-section" aria-labelledby="op-sandbox">
      <div className="op-section-head">
        <div>
          <h2 id="op-sandbox">{w("Sandbox bantuan terencana", "Planned support sandbox")}</h2>
          <p className="op-hint">
            {w(
              "Semua permintaan di sini fiktif dan berlabel Simulasi. Tidak ada email, webhook, pesanan, atau pengiriman ke fasilitas.",
              "Every request here is fictional and labelled Simulation. No email, webhook, booking or delivery to a facility.",
            )}
          </p>
        </div>
        <button type="button" className="op-btn op-btn--secondary" onClick={() => setForm((v) => !v)}>
          {form ? w("Tutup formulir", "Close form") : w("Buat permintaan simulasi", "Create simulation request")}
        </button>
      </div>
      {form && (
        <form className="op-care-form" onSubmit={submit}>
          <label className="op-field">
            <span>{w("Organisasi fiktif", "Fictional organization")}</span>
            <input className="op-input" required value={organization} onChange={(e) => setOrganization(e.target.value)} />
          </label>
          <label className="op-field">
            <span>{w("Praktisi fiktif", "Fictional practitioner")}</span>
            <input className="op-input" value={professional} onChange={(e) => setProfessional(e.target.value)} />
          </label>
          <label className="op-field">
            <span>{w("Layanan", "Service")}</span>
            <input className="op-input" value={service} onChange={(e) => setService(e.target.value)} />
          </label>
          <label className="op-field op-field--wide">
            <span>{w("Catatan", "Note")}</span>
            <input className="op-input" value={note} onChange={(e) => setNote(e.target.value)} />
          </label>
          <div className="op-actions">
            <button className="op-btn op-btn--primary" type="submit" disabled={busy === "sandbox-create"}>
              {w("Simpan simulasi", "Save simulation")}
            </button>
          </div>
        </form>
      )}
      {requests === null ? (
        <p className="op-hint">{w("Memuat sandbox…", "Loading sandbox…")}</p>
      ) : requests.length === 0 ? (
        <p className="op-hint">{w("Belum ada permintaan simulasi.", "No simulation requests yet.")}</p>
      ) : (
        <ul className="op-care-list">
          {requests.map((r) => (
            <li key={r.id}>
              <div className="op-care-main">
                <strong>{r.organization}</strong>
                <span className="op-care-tags">
                  <span className={`op-care-status op-care-status--${r.status}`}>{label(STATUS_LABEL, r.status, w)}</span>
                  <span className="op-badge op-badge--demo">{w("Simulasi", "Simulation")}</span>
                  {r.professional && <span className="op-hint">{r.professional}</span>}
                  {r.service && <span className="op-hint">{r.service}</span>}
                </span>
                {r.note && <p className="op-care-desc">{r.note}</p>}
                <p className="op-hint">{date(r.createdAt)}</p>
              </div>
              <div className="op-care-actions">
                {(next[r.status] ?? []).map((t) => (
                  <button
                    key={t.status}
                    type="button"
                    className={`op-btn ${t.status === "revoked" ? "op-btn--quiet op-btn--danger-text" : "op-btn--quiet"}`}
                    disabled={busy === `sandbox:${r.id}`}
                    onClick={() => onTransition(r.id, r.version, t.status)}
                  >
                    {w(t.label[0], t.label[1])}
                  </button>
                ))}
              </div>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
