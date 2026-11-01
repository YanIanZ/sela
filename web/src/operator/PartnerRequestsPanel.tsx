import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react";
import { useI18n } from "../i18n";
import { useWords } from "../v3/shared";
import {
  AdminError,
  adminApi,
  type Admin,
  type CareOrg,
  type CareOrgMember,
  type PartnerCareRequest,
} from "./api";
import { roleLabel } from "./chrome";
import { mapLabel, MODES, ORG_KIND, PROFESSIONAL_TYPE, STAGES } from "./NetworkPanel";
import { Page, Shell, useRouter } from "./shell";
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

const TABS: { key: string; states: string[]; label: [string, string] }[] = [
  { key: "new", states: ["sent"], label: ["Baru", "New"] },
  { key: "waiting", states: ["proposed"], label: ["Menunggu pasien", "Waiting for the person"] },
  { key: "confirmed", states: ["confirmed"], label: ["Terkonfirmasi", "Confirmed"] },
  { key: "done", states: ["completed"], label: ["Selesai", "Completed"] },
  { key: "closed", states: ["declined", "cancelled", "expired"], label: ["Ditutup", "Closed"] },
];

function statusTone(state: string): string {
  if (state === "sent") return "pending";
  if (state === "declined" || state === "cancelled" || state === "expired") return "revoked";
  return "verified";
}

/**
 * PartnerConsole is the whole operator surface for a partner account
 * (D-076): an inbox of its organisation's care requests, the confirmed
 * schedule and the clinic and team. Nothing internal is reachable.
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
  const { path, navigate } = useRouter();
  const [signingOut, setSigningOut] = useState(false);
  const [error, setError] = useState("");
  const [org, setOrg] = useState<CareOrg | null>(null);
  const [list, setList] = useState<PartnerCareRequest[] | null>(null);

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

  const pages = ["/operator", "/operator/schedule", "/operator/team"];
  const active = pages.includes(path) ? path : "/operator";
  useEffect(() => {
    if (!pages.includes(path)) navigate("/operator");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [path]);

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

  const open = (list ?? []).filter((r) => r.state === "sent").length;
  const upcoming = (list ?? []).filter((r) => r.state === "confirmed" && r.confirmedSlot && new Date(r.confirmedSlot) >= new Date()).length;

  return (
    <Shell
      admin={admin}
      active={active}
      signingOut={signingOut}
      onSignOut={() => void signOut()}
      context={org ? org.name : undefined}
      items={[
        { path: "/operator", label: w("Kotak masuk", "Inbox"), count: open },
        { path: "/operator/schedule", label: w("Jadwal", "Schedule"), count: upcoming },
        { path: "/operator/team", label: w("Tim & klinik", "Team & clinic") },
      ]}
    >
      {error && <p className="op-alert" role="alert">{error}</p>}
      {active === "/operator/schedule" ? (
        <PartnerSchedule list={list} />
      ) : active === "/operator/team" ? (
        <PartnerTeam admin={admin} org={org} fail={fail} />
      ) : (
        <PartnerInbox admin={admin} org={org} list={list} onReload={loadList} fail={fail} />
      )}
    </Shell>
  );
}

function SimulationNote({ org }: { org: CareOrg | null }) {
  const w = useWords();
  if (!org?.synthetic) return null;
  return (
    <p className="op-hint op-network-sim">
      {w(
        "Organisasi simulasi: permintaan di sini berasal dari pengguna yang tahu layanannya simulasi. Jangan memberi layanan klinis nyata dari sini.",
        "Simulated organisation: requests here come from people who know the service is simulated. Do not provide real clinical care from here.",
      )}
    </p>
  );
}

function PartnerInbox({
  admin,
  org,
  list,
  onReload,
  fail,
}: {
  admin: Admin;
  org: CareOrg | null;
  list: PartnerCareRequest[] | null;
  onReload: () => Promise<void>;
  fail: (err: unknown) => void;
}) {
  const w = useWords();
  const date = useDateTime();
  const canRespond = admin.permissions.includes("care-requests/respond");
  const [tab, setTab] = useState("new");
  const [selected, setSelected] = useState("");
  const [detail, setDetail] = useState<PartnerCareRequest | null>(null);
  const [canRead, setCanRead] = useState(false);
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState("");

  const counts = useMemo(() => {
    const c: Record<string, number> = {};
    for (const t of TABS) c[t.key] = (list ?? []).filter((r) => t.states.includes(r.state)).length;
    return c;
  }, [list]);
  const current = TABS.find((t) => t.key === tab) ?? TABS[0]!;
  const shown = (list ?? []).filter((r) => current.states.includes(r.state));

  const open = async (id: string) => {
    setSelected(id);
    setDetail(null);
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
    setNotice("");
    try {
      const updated = await job();
      setDetail((d) => (d ? { ...d, ...updated, summary: d.summary } : updated));
      setNotice(done);
      await onReload();
    } catch (err) {
      fail(err);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Page
      title={w("Kotak masuk", "Inbox")}
      lead={w(
        "Permintaan bantuan yang dikirim ke organisasimu. Rangkuman dibagikan oleh pasien sendiri dan hanya bisa dibaca selama permintaan aktif. Sela tidak memberikan kontak pasien.",
        "Care requests sent to your organisation. The summary was shared by the person themself and can only be read while the request is active. Sela does not give you the person's contact details.",
      )}
      actions={
        <button type="button" className="op-btn op-btn--quiet" onClick={() => void onReload()}>
          {w("Muat ulang", "Refresh")}
        </button>
      }
    >
      {org && (
        <p className="op-care-tags">
          <strong>{org.name}</strong>
          <span className="op-badge op-badge--soft">{mapLabel(ORG_KIND, org.kind, w)}</span>
          {org.synthetic && <span className="op-badge op-badge--demo">{w("Simulasi", "Simulation")}</span>}
          {org.city && <span className="op-hint">{org.city}</span>}
        </p>
      )}
      <SimulationNote org={org} />
      <p className="op-notice" role="status" hidden={!notice}>
        {notice}
      </p>
      <div className="op-tabs" role="group" aria-label={w("Saring menurut status", "Filter by status")}>
        {TABS.map((t) => (
          <button key={t.key} type="button" aria-pressed={tab === t.key} onClick={() => setTab(t.key)}>
            <span>{w(t.label[0], t.label[1])}</span>
            <span className="op-tab-count">{counts[t.key] ?? 0}</span>
          </button>
        ))}
      </div>
      <div className="op-partner-layout">
        <section className="op-section" aria-label={w("Daftar permintaan", "Request list")}>
          {list === null ? (
            <p className="op-hint">{w("Memuat permintaan…", "Loading requests…")}</p>
          ) : shown.length === 0 ? (
            <p className="op-hint">
              {list.length === 0 ? w("Belum ada permintaan.", "No requests yet.") : w("Tidak ada permintaan di status ini.", "No requests in this status.")}
            </p>
          ) : (
            <ul className="op-partner-list">
              {shown.map((r) => (
                <li key={r.id}>
                  <button type="button" aria-current={selected === r.id ? "true" : undefined} onClick={() => void open(r.id)}>
                    <span className="op-care-tags">
                      <span className="op-partner-ref">{r.reference}</span>
                      <span className={`op-care-status op-care-status--${statusTone(r.state)}`}>{mapLabel(STATE, r.state, w)}</span>
                    </span>
                    <span className="op-hint">
                      {mapLabel(MODE, r.mode, w)}
                      {r.practitionerName ? ` · ${r.practitionerName}` : ""}
                    </span>
                    <span className="op-hint">
                      {r.state === "confirmed" && r.confirmedSlot
                        ? `${w("Sesi", "Session")} ${date(r.confirmedSlot)}`
                        : `${w("Masuk", "Received")} ${date(r.createdAt)} · ${w("Berlaku sampai", "Valid until")} ${date(r.expiresAt)}`}
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </section>

        <div aria-live="polite">
          {selected && !detail && <p className="op-hint">{w("Memuat detail…", "Loading details…")}</p>}
          {!selected && list !== null && list.length > 0 && (
            <p className="op-hint">{w("Pilih permintaan untuk melihat detailnya.", "Choose a request to see its details.")}</p>
          )}
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
                void act(() => adminApi.partnerComplete(detail.id, detail.version), w("Ditandai selesai.", "Marked as completed."))
              }
            />
          )}
        </div>
      </div>
    </Page>
  );
}

function PartnerSchedule({ list }: { list: PartnerCareRequest[] | null }) {
  const w = useWords();
  const date = useDateTime();
  const now = Date.now();
  const confirmed = (list ?? [])
    .filter((r) => (r.state === "confirmed" || r.state === "completed") && r.confirmedSlot)
    .sort((a, b) => new Date(a.confirmedSlot ?? 0).getTime() - new Date(b.confirmedSlot ?? 0).getTime());
  const upcoming = confirmed.filter((r) => new Date(r.confirmedSlot ?? 0).getTime() >= now);
  const past = confirmed.filter((r) => new Date(r.confirmedSlot ?? 0).getTime() < now).reverse();
  const row = (r: PartnerCareRequest) => (
    <li key={r.id}>
      <time dateTime={r.confirmedSlot}>{date(r.confirmedSlot)}</time>
      <span className="op-care-tags">
        <span className="op-partner-ref">{r.reference}</span>
        <span className={`op-care-status op-care-status--${statusTone(r.state)}`}>{mapLabel(STATE, r.state, w)}</span>
      </span>
      <span className="op-hint">
        {mapLabel(MODE, r.proposal?.mode ?? r.mode, w)}
        {r.practitionerName ? ` · ${r.practitionerName}` : ""}
      </span>
    </li>
  );
  return (
    <Page
      title={w("Jadwal", "Schedule")}
      lead={w("Sesi yang sudah dikonfirmasi pasien, urut waktu.", "Sessions the person confirmed, in time order.")}
    >
      {list === null ? (
        <p className="op-hint" role="status">{w("Memuat jadwal…", "Loading the schedule…")}</p>
      ) : (
        <>
          <section className="op-section" aria-labelledby="op-schedule-next">
            <h2 id="op-schedule-next">{w("Akan datang", "Upcoming")}</h2>
            {upcoming.length === 0 ? (
              <p className="op-hint">{w("Belum ada sesi terjadwal.", "No sessions scheduled yet.")}</p>
            ) : (
              <ol className="op-schedule" aria-label={w("Sesi akan datang", "Upcoming sessions")}>{upcoming.map(row)}</ol>
            )}
          </section>
          <section className="op-section" aria-labelledby="op-schedule-past">
            <h2 id="op-schedule-past">{w("Sudah lewat", "Past")}</h2>
            {past.length === 0 ? (
              <p className="op-hint">{w("Belum ada sesi yang lewat.", "No past sessions yet.")}</p>
            ) : (
              <ol className="op-schedule">{past.map(row)}</ol>
            )}
          </section>
        </>
      )}
    </Page>
  );
}

function PartnerTeam({ admin, org, fail }: { admin: Admin; org: CareOrg | null; fail: (err: unknown) => void }) {
  const w = useWords();
  const [members, setMembers] = useState<CareOrgMember[] | null>(null);
  useEffect(() => {
    adminApi.partnerMembers().then(setMembers).catch(fail);
  }, [fail]);
  return (
    <Page
      title={w("Tim & klinik", "Team & clinic")}
      lead={w(
        "Informasi organisasimu seperti yang dilihat pengguna, dan akun timmu. Perubahan data klinik dilakukan oleh tim Sela.",
        "Your organisation as people see it, and your team's accounts. Clinic details are changed by the Sela team.",
      )}
    >
      <section className="op-section" aria-labelledby="op-team-org">
        <h2 id="op-team-org">{org?.name ?? w("Klinik", "Clinic")}</h2>
        {org ? (
          <>
            <p className="op-care-tags">
              <span className="op-badge op-badge--soft">{mapLabel(ORG_KIND, org.kind, w)}</span>
              {org.synthetic && <span className="op-badge op-badge--demo">{w("Simulasi", "Simulation")}</span>}
              {org.city && <span className="op-hint">{org.city}</span>}
            </p>
            {org.description && <p>{org.description}</p>}
            <p className="op-hint">
              {w("Cara layanan", "Service modes")}: {org.modes.map((m) => mapLabel(MODES, m, w)).join(", ") || "—"} ·{" "}
              {w("Cocok untuk tahap", "Fits stages")}:{" "}
              {org.stages
                .map((s) => {
                  const st = STAGES.find(([k]) => k === s);
                  return st ? w(st[1], st[2]) : s;
                })
                .join(", ") || "—"}
            </p>
            <SimulationNote org={org} />
          </>
        ) : (
          <p className="op-hint">{w("Memuat klinik…", "Loading the clinic…")}</p>
        )}
      </section>
      <section className="op-section" aria-labelledby="op-team-members">
        <h2 id="op-team-members">{w("Tim", "Team")}</h2>
        {members === null ? (
          <p className="op-hint">{w("Memuat tim…", "Loading the team…")}</p>
        ) : members.length === 0 ? (
          <p className="op-hint">{w("Belum ada akun.", "No accounts yet.")}</p>
        ) : (
          <ul className="op-network-member-list">
            {members.map((m) => (
              <li key={m.username}>
                <div className="op-care-main">
                  <strong>
                    {m.displayName || m.username}
                    {m.username === admin.username ? ` (${w("kamu", "you")})` : ""}
                  </strong>
                  <span className="op-care-tags">
                    <span className="op-badge op-badge--soft">{roleLabel(m.role, w)}</span>
                    {m.professionalType && <span className="op-hint">{mapLabel(PROFESSIONAL_TYPE, m.professionalType, w)}</span>}
                    {m.role === "practitioner" && (
                      <span className={`op-care-status op-care-status--${m.licenseVerified ? "verified" : "pending"}`}>
                        {m.licenseVerified ? w("Lisensi terverifikasi", "Licence verified") : w("Belum diverifikasi", "Not verified")}
                      </span>
                    )}
                    {m.status !== "active" && <span className="op-care-status op-care-status--revoked">{w("Dicabut", "Revoked")}</span>}
                  </span>
                  {m.licenseNo && (
                    <p className="op-hint">
                      {w("Lisensi", "Licence")}: {m.licenseNo}
                    </p>
                  )}
                </div>
              </li>
            ))}
          </ul>
        )}
      </section>
    </Page>
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
