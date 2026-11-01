import { useCallback, useEffect, useMemo, useRef, useState, type FormEvent } from "react";
import { useI18n } from "../i18n";
import { useWords } from "../v3/shared";
import {
  AdminError,
  adminApi,
  type Admin,
  type CareOrg,
  type PartnerCareRequest,
} from "./api";
import { OperatorHeader } from "./chrome";
import { mapLabel, ORG_KIND } from "./NetworkPanel";
import "./care-panels.css";
import "./network.css";

type Words = (id: string, en: string) => string;

const STATE: Record<string, [string, string]> = {
  sent: ["Baru", "New"],
  proposed: ["Waktu diusulkan", "Times proposed"],
  declined: ["Ditolak", "Declined"],
  confirmed: ["Dikonfirmasi pasien", "Confirmed by the person"],
  completed: ["Selesai", "Completed"],
  cancelled: ["Dibatalkan pasien", "Cancelled by the person"],
  expired: ["Kedaluwarsa", "Expired"],
};
const MODE: Record<string, [string, string]> = {
  online: ["Daring", "Online"],
  in_person: ["Tatap muka", "In person"],
  either: ["Daring atau tatap muka", "Online or in person"],
};
const DECLINE: Record<string, [string, string]> = {
  no_capacity: ["Jadwal penuh", "No capacity"],
  not_a_fit: ["Bukan layanan yang tepat", "Not the right service"],
  refer_elsewhere: ["Sebaiknya ke layanan lain", "Better served elsewhere"],
  other: ["Alasan lain", "Other reason"],
};

function useDateTime() {
  const { lang } = useI18n();
  return useMemo(() => {
    const locale = lang === "en" ? "en-GB" : "id-ID";
    const f = new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeStyle: "short" });
    return (iso?: string) => {
      if (!iso) return "";
      const d = new Date(iso);
      return Number.isNaN(d.getTime()) ? iso : f.format(d);
    };
  }, [lang]);
}

function errorText(err: unknown, w: Words): string {
  if (err instanceof AdminError) {
    if (err.status === 409) return w("Permintaan sudah berubah. Muat ulang lalu coba lagi.", "The request changed. Refresh and try again.");
    if (err.status === 400) return w("Periksa isian: waktu harus di masa depan (maks. 60 hari) dan cara kontak 3–300 karakter.", "Check the fields: times must be in the future (up to 60 days) and contact instructions 3–300 characters.");
    if (err.status === 403) return w("Peranmu tidak boleh melakukan ini.", "Your role may not do this.");
    if (err.status === 404) return w("Permintaan tidak ditemukan untuk organisasimu.", "The request was not found for your organisation.");
  }
  return w("Permintaan gagal. Coba lagi.", "The request failed. Try again.");
}

/**
 * PartnerConsole is the whole operator surface for a partner account
 * (D-076): it shows only the care requests of the account's organisation.
 */
export function PartnerConsole({
  admin,
  onSignOut,
  onExpired,
}: {
  admin: Admin;
  onSignOut: () => void;
  onExpired: () => void;
}) {
  const w = useWords();
  const [signingOut, setSigningOut] = useState(false);
  const [error, setError] = useState("");
  const titleRef = useRef<HTMLHeadingElement>(null);
  useEffect(() => {
    titleRef.current?.focus();
  }, []);

  const signOut = async () => {
    setSigningOut(true);
    try {
      await adminApi.logout();
      onSignOut();
    } catch (err) {
      if (err instanceof AdminError && err.status === 401) onSignOut();
      else setError(errorText(err, w));
    } finally {
      setSigningOut(false);
    }
  };

  return (
    <div className="op">
      <a className="op-skip" href="#op-main">
        {w("Lewati ke konten", "Skip to content")}
      </a>
      <OperatorHeader admin={admin} active="console" signingOut={signingOut} onSignOut={() => void signOut()} />
      <main className="op-main" id="op-main">
        <div className="op-intro">
          <div>
            <h1 ref={titleRef} tabIndex={-1}>
              {w("Ruang kerja profesional", "Professional workspace")}
            </h1>
            <p>
              {w(
                "Permintaan bantuan yang dikirim ke organisasimu. Rangkuman dibagikan oleh pasien sendiri dan hanya bisa dibaca selama permintaan aktif. Sela tidak memberikan kontak pasien.",
                "Care requests sent to your organisation. The summary was shared by the person themself and can only be read while the request is active. Sela does not give you the person's contact details.",
              )}
            </p>
          </div>
        </div>
        {error && <p className="op-alert" role="alert">{error}</p>}
        <PartnerRequestsPanel admin={admin} onExpired={onExpired} />
      </main>
    </div>
  );
}

export function PartnerRequestsPanel({ admin, onExpired }: { admin: Admin; onExpired: () => void }) {
  const w = useWords();
  const date = useDateTime();
  const canRespond = admin.permissions.includes("care-requests/respond");
  const [org, setOrg] = useState<CareOrg | null>(null);
  const [list, setList] = useState<PartnerCareRequest[] | null>(null);
  const [selected, setSelected] = useState("");
  const [detail, setDetail] = useState<PartnerCareRequest | null>(null);
  const [canRead, setCanRead] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");

  const fail = useCallback(
    (err: unknown) => {
      if (err instanceof AdminError && err.status === 401) onExpired();
      else setError(errorText(err, w));
    },
    [onExpired, w],
  );

  const loadList = useCallback(async () => {
    try {
      setList(await adminApi.partnerRequests());
    } catch (err) {
      fail(err);
    }
  }, [fail]);

  useEffect(() => {
    adminApi
      .partnerOrg()
      .then((r) => setOrg(r.org))
      .catch(fail);
    void loadList();
  }, [fail, loadList]);

  const open = async (id: string) => {
    setSelected(id);
    setDetail(null);
    setError("");
    setNotice("");
    try {
      const r = await adminApi.partnerRequest(id);
      setDetail(r.request);
      setCanRead(r.canReadContent);
    } catch (err) {
      fail(err);
    }
  };

  const act = async (job: () => Promise<PartnerCareRequest>, done: string) => {
    if (busy) return;
    setBusy(true);
    setError("");
    setNotice("");
    try {
      const updated = await job();
      setDetail((d) => (d ? { ...d, ...updated, summary: d.summary } : updated));
      setNotice(done);
      await loadList();
    } catch (err) {
      fail(err);
    } finally {
      setBusy(false);
    }
  };

  return (
    <section className="op-section" aria-labelledby="op-partner">
      <div className="op-section-head">
        <div>
          <h2 id="op-partner" className="op-partner-head">
            <span>{w("Permintaan bantuan", "Care requests")}</span>
          </h2>
          {org && (
            <p className="op-care-tags">
              <strong>{org.name}</strong>
              <span className="op-badge op-badge--soft">{mapLabel(ORG_KIND, org.kind, w)}</span>
              {org.synthetic && <span className="op-badge op-badge--demo">{w("Simulasi", "Simulation")}</span>}
              {org.city && <span className="op-hint">{org.city}</span>}
            </p>
          )}
          {org?.synthetic && (
            <p className="op-hint">
              {w(
                "Organisasi simulasi: permintaan di sini berasal dari pengguna yang tahu layanannya simulasi. Jangan memberi layanan klinis nyata dari sini.",
                "Simulated organisation: requests here come from people who know the service is simulated. Do not provide real clinical care from here.",
              )}
            </p>
          )}
        </div>
        <button type="button" className="op-btn op-btn--quiet" onClick={() => void loadList()}>
          {w("Muat ulang", "Refresh")}
        </button>
      </div>

      {error && <p className="op-alert" role="alert">{error}</p>}
      <p className="op-notice" role="status" hidden={!notice}>
        {notice}
      </p>

      <div className="op-partner-layout">
        <div>
          {list === null ? (
            <p className="op-hint">{w("Memuat permintaan…", "Loading requests…")}</p>
          ) : list.length === 0 ? (
            <p className="op-hint">{w("Belum ada permintaan.", "No requests yet.")}</p>
          ) : (
            <ul className="op-partner-list" aria-label={w("Daftar permintaan", "Request list")}>
              {list.map((r) => (
                <li key={r.id}>
                  <button type="button" aria-current={selected === r.id ? "true" : undefined} onClick={() => void open(r.id)}>
                    <span className="op-care-tags">
                      <span className="op-partner-ref">{r.reference}</span>
                      <span className={`op-care-status op-care-status--${r.state === "sent" ? "pending" : r.state === "declined" || r.state === "cancelled" || r.state === "expired" ? "revoked" : "verified"}`}>
                        {mapLabel(STATE, r.state, w)}
                      </span>
                    </span>
                    <span className="op-hint">
                      {mapLabel(MODE, r.mode, w)}
                      {r.practitionerName ? ` · ${r.practitionerName}` : ""}
                    </span>
                    <span className="op-hint">
                      {w("Masuk", "Received")} {date(r.createdAt)} · {w("Berlaku sampai", "Valid until")} {date(r.expiresAt)}
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>

        <div aria-live="polite">
          {selected && !detail && <p className="op-hint">{w("Memuat detail…", "Loading details…")}</p>}
          {detail && (
            <RequestDetail
              request={detail}
              canRead={canRead}
              canRespond={canRespond}
              busy={busy}
              date={date}
              onPropose={(input) =>
                void act(
                  () => adminApi.partnerRespond(detail.id, { decision: "propose", expectedVersion: detail.version, proposal: input }),
                  w("Usulan waktu terkirim. Pasien akan memilih salah satu.", "Proposed times sent. The person will choose one."),
                )
              }
              onDecline={(reason) =>
                void act(
                  () => adminApi.partnerRespond(detail.id, { decision: "decline", expectedVersion: detail.version, declineReason: reason }),
                  w("Permintaan ditolak. Pasien melihat alasannya.", "Request declined. The person sees the reason."),
                )
              }
              onComplete={() =>
                void act(
                  () => adminApi.partnerComplete(detail.id, detail.version),
                  w("Ditandai selesai.", "Marked as completed."),
                )
              }
            />
          )}
        </div>
      </div>
    </section>
  );
}

function RequestDetail({
  request,
  canRead,
  canRespond,
  busy,
  date,
  onPropose,
  onDecline,
  onComplete,
}: {
  request: PartnerCareRequest;
  canRead: boolean;
  canRespond: boolean;
  busy: boolean;
  date: (iso?: string) => string;
  onPropose: (input: { slots: string[]; mode: string; contact: string; note: string }) => void;
  onDecline: (reason: string) => void;
  onComplete: () => void;
}) {
  const w = useWords();
  const open = request.state === "sent" || request.state === "proposed";
  const s = request.summary;
  return (
    <article className="op-partner-detail" aria-labelledby="op-partner-detail-title">
      <div>
        <h3 id="op-partner-detail-title">
          <span className="op-partner-ref">{request.reference}</span> · {mapLabel(STATE, request.state, w)}
        </h3>
        <p className="op-hint">
          {w("Cara yang diminta", "Requested mode")}: {mapLabel(MODE, request.mode, w)}
          {request.practitionerName ? ` · ${w("Untuk", "For")} ${request.practitionerName}` : ""}
        </p>
        {request.confirmedSlot && (
          <p>
            <strong>{w("Waktu terkonfirmasi", "Confirmed time")}:</strong> {date(request.confirmedSlot)}
          </p>
        )}
        {request.declineReason && (
          <p className="op-hint">
            {w("Alasan penolakan", "Decline reason")}: {mapLabel(DECLINE, request.declineReason, w)}
          </p>
        )}
      </div>

      {s ? (
        <section className="op-partner-summary" aria-label={w("Rangkuman dari pasien", "Summary from the person")}>
          <h4>{s.title}</h4>
          <p className="op-hint">
            {w("Versi", "Version")} {s.summaryVersion} · {w("dibagikan", "shared")} {date(s.createdAt)}
          </p>
          {s.text && <p>{s.text}</p>}
          {s.prepared?.goal && (
            <p>
              <strong>{w("Tujuan konsultasi", "Consultation goal")}:</strong> {s.prepared.goal}
            </p>
          )}
          {s.prepared?.questions && s.prepared.questions.length > 0 && (
            <div>
              <strong>{w("Pertanyaan pasien", "The person's questions")}</strong>
              <ul>
                {s.prepared.questions.map((q, i) => (
                  <li key={i}>{q}</li>
                ))}
              </ul>
            </div>
          )}
          {s.prepared?.bring && s.prepared.bring.length > 0 && (
            <div>
              <strong>{w("Yang akan dibawa", "What they will bring")}</strong>
              <ul>
                {s.prepared.bring.map((b, i) => (
                  <li key={i}>{b}</li>
                ))}
              </ul>
            </div>
          )}
          {s.items && s.items.length > 0 && (
            <ol>
              {s.items.map((it) => (
                <li key={it.position}>
                  {it.text}
                  {it.quote && <blockquote>{it.quote}</blockquote>}
                  <span className="op-hint"> ({it.sourceLabel})</span>
                </li>
              ))}
            </ol>
          )}
          {s.sources.length > 0 && (
            <p className="op-hint">
              {w("Sumber", "Sources")}: {s.sources.join(", ")}
            </p>
          )}
          <p className="op-hint">
            {w(
              "Ini kata-kata pasien sendiri, bukan diagnosis atau catatan klinis Sela.",
              "These are the person's own words, not a diagnosis or a clinical note by Sela.",
            )}
          </p>
        </section>
      ) : (
        <p className="op-partner-note">
          {!canRead
            ? w("Isi rangkuman hanya untuk tenaga profesional dan admin klinik.", "The summary is only for practitioners and clinic administrators.")
            : !request.contentAvailable
              ? w("Akses berakhir: permintaan dibatalkan, kedaluwarsa, atau dibagikan lebih dari 7 hari lalu.", "Access ended: the request was cancelled, expired or shared more than 7 days ago.")
              : w("Rangkuman tidak tersedia.", "The summary is not available.")}
        </p>
      )}

      {request.proposal && (
        <div className="op-partner-proposal">
          <strong>{w("Usulan terakhir", "Latest proposal")}</strong>
          <span>{request.proposal.slots.map((t) => date(t)).join(" · ")}</span>
          <span className="op-hint">
            {mapLabel(MODE, request.proposal.mode, w)} · {request.proposal.contact}
          </span>
          {request.proposal.note && <span className="op-hint">{request.proposal.note}</span>}
        </div>
      )}

      {canRespond && open && <RespondForm busy={busy} onPropose={onPropose} onDecline={onDecline} />}
      {canRespond && request.state === "confirmed" && (
        <div>
          <button type="button" className="op-btn op-btn--secondary" disabled={busy} onClick={onComplete}>
            {busy ? w("Menyimpan…", "Saving…") : w("Tandai selesai", "Mark completed")}
          </button>
        </div>
      )}
    </article>
  );
}

function localToISO(value: string): string | null {
  if (!value) return null;
  const d = new Date(value);
  return Number.isNaN(d.getTime()) ? null : d.toISOString();
}

function RespondForm({
  busy,
  onPropose,
  onDecline,
}: {
  busy: boolean;
  onPropose: (input: { slots: string[]; mode: string; contact: string; note: string }) => void;
  onDecline: (reason: string) => void;
}) {
  const w = useWords();
  const [slots, setSlots] = useState(["", "", ""]);
  const [mode, setMode] = useState("online");
  const [contact, setContact] = useState("");
  const [note, setNote] = useState("");
  const [reason, setReason] = useState("no_capacity");
  const iso = slots.map(localToISO).filter((v): v is string => v !== null);
  const contactOk = contact.trim().length >= 3 && contact.trim().length <= 300;

  const submit = (e: FormEvent) => {
    e.preventDefault();
    if (iso.length === 0 || !contactOk || busy) return;
    onPropose({ slots: iso, mode, contact: contact.trim(), note: note.trim() });
  };

  return (
    <>
      <form className="op-partner-form" onSubmit={submit} aria-label={w("Usulkan waktu", "Propose times")}>
        <strong>{w("Usulkan hingga 3 waktu", "Propose up to 3 times")}</strong>
        <div className="op-partner-slots">
          {slots.map((v, i) => (
            <label key={i} className="op-field">
              <span>{w(`Waktu ${i + 1}`, `Time ${i + 1}`)}</span>
              <input
                className="op-input"
                type="datetime-local"
                value={v}
                required={i === 0}
                onChange={(e) => setSlots((s) => s.map((x, j) => (j === i ? e.target.value : x)))}
              />
            </label>
          ))}
        </div>
        <label className="op-field">
          <span>{w("Cara sesi", "Session mode")}</span>
          <select className="op-input" value={mode} onChange={(e) => setMode(e.target.value)}>
            <option value="online">{w("Daring", "Online")}</option>
            <option value="in_person">{w("Tatap muka", "In person")}</option>
          </select>
        </label>
        <div className="op-field">
          <label htmlFor="op-partner-contact">{w("Cara menghubungi klinik", "How to reach the clinic")}</label>
          <textarea
            id="op-partner-contact"
            className="op-input"
            rows={2}
            required
            minLength={3}
            maxLength={300}
            value={contact}
            aria-describedby="op-partner-contact-hint"
            onChange={(e) => setContact(e.target.value)}
          />
          <span className="op-hint" id="op-partner-contact-hint">
            {w(
              "Cara pasien menghubungi klinik, mis. nomor WhatsApp klinik dan kode permintaan. Sela tidak memberi kontak pasien.",
              "How the person contacts the clinic, e.g. the clinic's WhatsApp number and the request code. Sela does not give you the person's contact.",
            )}
          </span>
        </div>
        <label className="op-field">
          <span>{w("Catatan untuk pasien (opsional)", "Note for the person (optional)")}</span>
          <textarea className="op-input" rows={2} maxLength={500} value={note} onChange={(e) => setNote(e.target.value)} />
        </label>
        <div className="op-partner-row">
          <button type="submit" className="op-btn op-btn--primary" disabled={busy || iso.length === 0 || !contactOk}>
            {busy ? w("Mengirim…", "Sending…") : w("Kirim usulan", "Send proposal")}
          </button>
        </div>
      </form>
      <div className="op-partner-row">
        <label className="op-field">
          <span>{w("Alasan menolak", "Reason to decline")}</span>
          <select className="op-input" value={reason} onChange={(e) => setReason(e.target.value)}>
            {Object.entries(DECLINE).map(([k, v]) => (
              <option key={k} value={k}>
                {w(v[0], v[1])}
              </option>
            ))}
          </select>
        </label>
        <button type="button" className="op-btn op-btn--quiet op-btn--danger-text" disabled={busy} onClick={() => onDecline(reason)}>
          {w("Tolak permintaan", "Decline request")}
        </button>
      </div>
    </>
  );
}
