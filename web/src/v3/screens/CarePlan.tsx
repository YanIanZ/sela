import { useCallback, useEffect, useState } from "react";
import { useRouter } from "../../lib/router";
import { api, type Entry } from "../api";
import { useConsent } from "../consent";
import { useSession } from "../session";
import { useAnnounce, useLocale, useWords } from "../shared";
import {
  Button,
  CheckboxRow,
  ChoiceGroup,
  ErrorNotice,
  InlineConfirm,
  LinkButton,
  Loading,
  Notice,
  PageHeader,
} from "../ui";

// Bantuan terencana (D-076): one guided path from the condition map and an
// approved summary to a structured request to a verified organisation, and
// its answer. Sela contacts no one on the person's behalf and never passes
// on their phone number or email; the organisation reads only the approved
// summary, for seven days, and cancelling ends that access.

type Stage = "everyday" | "attention" | "support" | "urgent";
type Mode = "online" | "in_person" | "either";

type Practitioner = { id: string; displayName: string; professionalType: string; licenseVerified: boolean };
type CareOrg = {
  id: string;
  kind: string;
  name: string;
  city?: string;
  description?: string;
  modes: string[];
  stages: Stage[];
  synthetic: boolean;
  practitioners: Practitioner[];
};
type CareProposal = { slots: string[]; mode: "online" | "in_person"; contact: string; note?: string };
type CareRequest = {
  id: string;
  reference: string;
  orgId: string;
  orgName: string;
  orgSynthetic: boolean;
  practitionerId?: string;
  practitionerName?: string;
  shareId: string;
  summaryId: string;
  summaryVersion: number;
  mode: Mode;
  state: "sent" | "proposed" | "declined" | "confirmed" | "completed" | "cancelled" | "expired";
  proposal?: CareProposal;
  confirmedSlot?: string;
  declineReason?: string;
  version: number;
  expiresAt: string;
  createdAt: string;
  updatedAt: string;
};

type Words = (id: string, en: string) => string;

const stageLabel = (s: Stage, w: Words) =>
  ({
    everyday: w("Sehari-hari", "Everyday"),
    attention: w("Perlu perhatian", "Needs attention"),
    support: w("Butuh dukungan", "Needs support"),
    urgent: w("Butuh bantuan segera", "Needs help now"),
  })[s];

const kindLabel = (k: string, w: Words) =>
  ({
    clinic: w("Klinik", "Clinic"),
    hospital: w("Rumah sakit", "Hospital"),
    puskesmas: w("Puskesmas", "Community health centre"),
    practice: w("Praktik mandiri", "Private practice"),
    organization: w("Organisasi", "Organisation"),
  })[k] ?? k;

const professionLabel = (p: string, w: Words) =>
  ({
    psychologist: w("Psikolog", "Psychologist"),
    psychiatrist: w("Psikiater", "Psychiatrist"),
    doctor: w("Dokter", "Doctor"),
    counsellor: w("Konselor", "Counsellor"),
    nurse: w("Perawat", "Nurse"),
    social_worker: w("Pekerja sosial", "Social worker"),
  })[p] ?? p;

const modeLabel = (m: string, w: Words) =>
  ({
    online: w("Daring", "Online"),
    in_person: w("Tatap muka", "In person"),
    either: w("Mana saja", "Either"),
  })[m] ?? m;

const stateLabel = (s: CareRequest["state"], w: Words) =>
  ({
    sent: w("Terkirim", "Sent"),
    proposed: w("Ada usulan waktu", "Times proposed"),
    declined: w("Ditolak", "Declined"),
    confirmed: w("Terkonfirmasi", "Confirmed"),
    completed: w("Selesai", "Completed"),
    cancelled: w("Dibatalkan", "Cancelled"),
    expired: w("Kedaluwarsa", "Expired"),
  })[s];

const declineLabel = (r: string, w: Words) =>
  ({
    no_capacity: w("Jadwal mereka sedang penuh", "Their schedule is full"),
    not_a_fit: w("Kurang cocok dengan layanan mereka", "Not a fit for their service"),
    refer_elsewhere: w("Mereka menyarankan layanan lain", "They suggest another service"),
    other: w("Alasan lain", "Another reason"),
  })[r] ?? r;

const open = (s: CareRequest["state"]) => s === "sent" || s === "proposed" || s === "confirmed";

export function CarePlanPage() {
  const w = useWords();
  const loc = useLocale();
  const { query } = useRouter();
  const { announce } = useAnnounce();
  const { policies } = useSession();
  const sharing = useConsent("sharing", policies.sharing);
  const mapConsent = useConsent("condition_map", policies.condition_map);

  const [summaries, setSummaries] = useState<Entry[] | null>(null);
  const [orgs, setOrgs] = useState<CareOrg[] | null>(null);
  const [requests, setRequests] = useState<CareRequest[] | null>(null);
  const [stage, setStage] = useState<Stage | null>(null);
  const [loadError, setLoadError] = useState<unknown>(null);
  const [error, setError] = useState<{ at: string; error: unknown } | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [sent, setSent] = useState<CareRequest | null>(null);

  const [summaryId, setSummaryId] = useState<string | null>(query.get("summary"));
  const [orgId, setOrgId] = useState<string | null>(null);
  const [practitionerId, setPractitionerId] = useState<string>("");
  const [mode, setMode] = useState<Mode | null>(null);
  const [agree, setAgree] = useState(false);

  const loadRequests = useCallback(() => {
    api<{ requests: CareRequest[] }>("/care-requests")
      .then((r) => setRequests(r.requests))
      .catch((e) => {
        setRequests([]);
        setLoadError(e);
      });
  }, []);

  const load = useCallback(() => {
    setLoadError(null);
    api<{ summaries: Entry[] }>("/consultation-summaries")
      .then((r) => setSummaries(r.summaries))
      .catch((e) => {
        setSummaries([]);
        setLoadError(e);
      });
    api<{ orgs: CareOrg[] }>("/care-network")
      .then((r) => setOrgs(r.orgs))
      .catch((e) => {
        setOrgs([]);
        setLoadError(e);
      });
    loadRequests();
  }, [loadRequests]);
  useEffect(load, [load]);

  useEffect(() => {
    if (!mapConsent.granted) return;
    api<{ stage: Stage }>("/condition-map")
      .then((m) => setStage(m.stage))
      .catch(() => setStage(null));
  }, [mapConsent.granted]);

  const approved = (summaries ?? []).filter((s) => s.summaryState === "approved");
  const drafts = (summaries ?? []).filter((s) => s.summaryState !== "approved");
  const summary = approved.find((s) => s.id === summaryId) ?? null;
  const org = (orgs ?? []).find((o) => o.id === orgId) ?? null;
  const practitioner = org?.practitioners.find((p) => p.id === practitionerId) ?? null;

  // one request at a time: repeated clicks never send twice
  const act = async (key: string, fn: () => Promise<void>) => {
    if (busy) return;
    setBusy(key);
    setError(null);
    try {
      await fn();
    } catch (e) {
      setError({ at: key === "send" ? "send" : "status", error: e });
    } finally {
      setBusy(null);
    }
  };

  const send = () =>
    act("send", async () => {
      if (!summary || !org || !mode || !agree) return;
      if (!sharing.granted) {
        try {
          await sharing.set(true);
        } catch (e) {
          if (e instanceof Error && e.message === "policy") {
            throw new Error(
              w(
                "Izin berbagi belum bisa diaktifkan karena kebijakannya belum termuat. Muat ulang halaman lalu coba lagi.",
                "Sharing permission can't be turned on because its policy hasn't loaded. Reload the page and try again.",
              ),
            );
          }
          throw e;
        }
      }
      const res = await api<{ request: CareRequest }>("/care-requests", {
        summaryId: summary.id,
        orgId: org.id,
        practitionerId: practitioner?.id ?? "",
        mode,
      });
      setSent(res.request);
      setAgree(false);
      setOrgId(null);
      setPractitionerId("");
      setMode(null);
      announce("status", w(`Permintaan terkirim. Kode ${res.request.reference}.`, `Request sent. Code ${res.request.reference}.`));
      loadRequests();
    });

  const confirm = (r: CareRequest, slot: string) =>
    act(`confirm:${r.id}`, async () => {
      await api(`/care-requests/${r.id}/confirm`, { slot, expectedVersion: r.version });
      announce("status", w("Waktu dikonfirmasi.", "Time confirmed."));
      loadRequests();
    });

  const cancel = (r: CareRequest) =>
    act(`cancel:${r.id}`, async () => {
      await api(`/care-requests/${r.id}/cancel`, { expectedVersion: r.version });
      announce("status", w("Permintaan dibatalkan. Layanan itu tidak bisa lagi membaca ringkasanmu.", "Request cancelled. That service can no longer read your summary."));
      loadRequests();
    });

  if (!summaries || !orgs || !requests) {
    return <Loading label={w("Memuat bantuan terencana…", "Loading planned support…")} />;
  }

  const canSend = Boolean(summary && org && mode && agree) && busy === null;

  return (
    <div className="c-plan">
      <PageHeader
        back={{ to: "/support", label: w("Dukungan", "Support") }}
        title={w("Bantuan terencana", "Planned support")}
        lead={w(
          "Dari peta kondisi dan ringkasan yang kamu setujui, sampai permintaan ke tenaga profesional — langkah demi langkah.",
          "From your condition map and an approved summary to a request to a professional — one step at a time.",
        )}
      />
      <Notice tone="info" title={w("Cara kerjanya", "How it works")}>
        <ul className="c-plan-rules">
          <li>{w("Sela tidak menghubungi siapa pun atas namamu dan tidak membagikan nomor telepon atau emailmu.", "Sela contacts no one on your behalf and does not share your phone number or email.")}</li>
          <li>{w("Layanan yang kamu pilih hanya membaca ringkasan yang sudah kamu setujui, selama 7 hari.", "The service you choose reads only the summary you approved, for 7 days.")}</li>
          <li>{w("Mereka menjawab dengan usulan waktu dan cara menghubungi mereka; kamu yang memilih dan menghubungi.", "They answer with proposed times and how to reach them; you choose and get in touch.")}</li>
          <li>{w("Membatalkan permintaan langsung menghentikan akses mereka ke ringkasanmu.", "Cancelling a request stops their access to your summary at once.")}</li>
        </ul>
      </Notice>
      <ErrorNotice error={loadError} onRetry={load} />

      <ol className="c-plan-steps">
        <li className="c-plan-step">
          <section aria-labelledby="plan-step-1">
            <h2 id="plan-step-1">
              <span className="c-plan-num" aria-hidden="true">1</span>
              {w("Peta kondisi", "Condition map")}
              <span className="s-optional"> {w("(boleh dilewati)", "(optional)")}</span>
            </h2>
            {stage ? (
              <>
                <p>
                  {w("Tahapmu sekarang:", "Your stage now:")} <strong>{stageLabel(stage, w)}</strong>
                </p>
                {stage === "urgent" && (
                  <Notice tone="help" title={w("Kalau kamu dalam bahaya sekarang", "If you are in danger now")}>
                    <p>{w("Permintaan terencana bisa butuh beberapa hari. Untuk bantuan segera, buka Bantuan sekarang.", "A planned request can take days. For immediate help, open Help now.")}</p>
                    <div className="s-actions">
                      <LinkButton to="/help" variant="help" icon="help">
                        {w("Bantuan sekarang", "Help now")}
                      </LinkButton>
                    </div>
                  </Notice>
                )}
              </>
            ) : (
              <p className="s-hint">
                {w(
                  "Aktifkan peta kondisi untuk melihat layanan yang cocok dengan tahapmu. Bukan diagnosis.",
                  "Turn on the condition map to see services that fit your stage. Not a diagnosis.",
                )}
              </p>
            )}
            <div className="s-actions">
              <LinkButton to="/support/directory" variant="quiet">
                {stage ? w("Lihat peta kondisi", "See the condition map") : w("Buka peta kondisi", "Open the condition map")}
              </LinkButton>
            </div>
          </section>
        </li>

        <li className="c-plan-step">
          <section aria-labelledby="plan-step-2">
            <h2 id="plan-step-2">
              <span className="c-plan-num" aria-hidden="true">2</span>
              {w("Susun rangkuman", "Build a summary")}
            </h2>
            {approved.length === 0 ? (
              <p>
                {w(
                  "Belum ada ringkasan yang kamu setujui. Susun ringkasan dari catatanmu, periksa, lalu setujui satu versinya.",
                  "You haven't approved a summary yet. Build one from your notes, review it, then approve one version.",
                )}
              </p>
            ) : (
              <ChoiceGroup
                legend={w("Ringkasan yang akan dikirim", "Summary to send")}
                name="plan-summary"
                value={summary ? summary.id : null}
                onChange={(v) => setSummaryId(v)}
                options={approved.map((s) => ({
                  value: s.id,
                  label: s.title || w("Ringkasan tanpa judul", "Untitled summary"),
                  hint: `v${s.version} · ${w("disetujui", "approved")} · ${loc.shortDate(s.updatedAt)}`,
                }))}
              />
            )}
            {drafts.length > 0 && (
              <p className="s-hint">
                {w(
                  `${drafts.length} draf belum disetujui. Hanya versi yang kamu setujui yang bisa dikirim.`,
                  `${drafts.length} draft(s) not approved yet. Only an approved version can be sent.`,
                )}
              </p>
            )}
            <div className="s-actions">
              <LinkButton to="/support/summaries/new" variant={approved.length === 0 ? "primary" : "quiet"}>
                {w("Susun ringkasan baru", "Build a new summary")}
              </LinkButton>
              {drafts.length > 0 && (
                <LinkButton to="/support/summaries" variant="quiet">
                  {w("Periksa & setujui draf", "Review and approve drafts")}
                </LinkButton>
              )}
            </div>
          </section>
        </li>

        <li className="c-plan-step">
          <section aria-labelledby="plan-step-3">
            <h2 id="plan-step-3">
              <span className="c-plan-num" aria-hidden="true">3</span>
              {w("Pilih tenaga profesional", "Choose a professional")}
            </h2>
            {orgs.length === 0 ? (
              <p>
                {w(
                  "Belum ada layanan yang menerima permintaan. Kamu tetap bisa melihat direktori bersumber dan menghubungi mereka sendiri.",
                  "No service is taking requests yet. You can still browse the sourced directory and contact them yourself.",
                )}
              </p>
            ) : (
              <ul className="c-plan-orgs">
                {orgs.map((o) => {
                  const selected = o.id === orgId;
                  const fits = stage !== null && o.stages.includes(stage);
                  return (
                    <li key={o.id} className={`c-plan-org${selected ? " is-selected" : ""}${fits ? " is-match" : ""}`}>
                      <div className="c-plan-org-head">
                        <h3>{o.name}</h3>
                        <span className="c-plan-tags">
                          {o.synthetic && <span className="c-plan-badge c-plan-badge--sim">{w("Simulasi", "Simulation")}</span>}
                          <span className="c-plan-badge">{kindLabel(o.kind, w)}</span>
                        </span>
                      </div>
                      <p className="s-hint">
                        {[o.city, o.modes.map((m) => modeLabel(m, w)).join(" · ")].filter(Boolean).join(" · ")}
                      </p>
                      {o.description && <p>{o.description}</p>}
                      {o.synthetic && (
                        <p className="c-plan-sim">
                          {w(
                            "Ini simulasi yang dijalankan tim Sela, bukan layanan nyata. Permintaanmu dibaca akun simulasi milik tim Sela.",
                            "This is a simulation run by the Sela team, not a real service. Your request is read by a simulated account run by the Sela team.",
                          )}
                        </p>
                      )}
                      {o.stages.length > 0 && (
                        <p className="s-hint">
                          {w("Untuk tahap", "For stages")}: {o.stages.map((s) => stageLabel(s, w)).join(", ")}
                          {fits ? ` · ${w("cocok untuk tahapmu", "fits your stage")}` : ""}
                        </p>
                      )}
                      {o.practitioners.length > 0 && (
                        <ul className="c-plan-people">
                          {o.practitioners.map((p) => (
                            <li key={p.id}>
                              <span>
                                <strong>{p.displayName}</strong> · {professionLabel(p.professionalType, w)}
                              </span>
                              {p.licenseVerified && (
                                <span className="c-plan-badge c-plan-badge--ok">{w("Lisensi terverifikasi", "Licence verified")}</span>
                              )}
                            </li>
                          ))}
                        </ul>
                      )}
                      <div className="s-actions">
                        <Button
                          variant={selected ? "primary" : "secondary"}
                          aria-pressed={selected}
                          onClick={() => {
                            setOrgId(selected ? null : o.id);
                            setPractitionerId("");
                          }}
                        >
                          {selected ? w("Dipilih", "Selected") : w(`Pilih ${o.name}`, `Choose ${o.name}`)}
                        </Button>
                      </div>
                      {selected && o.practitioners.length > 0 && (
                        <ChoiceGroup
                          legend={w("Siapa yang kamu tuju?", "Who would you like?")}
                          name={`plan-practitioner-${o.id}`}
                          value={practitionerId}
                          onChange={(v) => setPractitionerId(v)}
                          options={[
                            { value: "", label: w("Siapa saja di layanan ini", "Anyone at this service") },
                            ...o.practitioners.map((p) => ({ value: p.id, label: `${p.displayName} · ${professionLabel(p.professionalType, w)}` })),
                          ]}
                        />
                      )}
                    </li>
                  );
                })}
              </ul>
            )}
            <div className="s-actions">
              <LinkButton to="/support/directory" variant="quiet">
                {w("Lihat direktori bersumber", "See the sourced directory")}
              </LinkButton>
            </div>
          </section>
        </li>

        <li className="c-plan-step">
          <section aria-labelledby="plan-step-4">
            <h2 id="plan-step-4">
              <span className="c-plan-num" aria-hidden="true">4</span>
              {w("Kirim permintaan", "Send the request")}
            </h2>
            {sent && (
              <Notice tone="success" title={w("Permintaan terkirim", "Request sent")}>
                <p>
                  {w("Kode permintaan", "Request code")}: <code>{sent.reference}</code>.{" "}
                  {w("Sebutkan kode ini saat menghubungi layanan.", "Mention this code when you contact the service.")}
                </p>
              </Notice>
            )}
            {!summary || !org ? (
              <p className="s-hint">
                {w("Pilih ringkasan yang disetujui (langkah 2) dan satu layanan (langkah 3) dulu.", "Choose an approved summary (step 2) and one service (step 3) first.")}
              </p>
            ) : (
              <div className="c-plan-send">
                <ChoiceGroup
                  legend={w("Cara bertemu yang kamu inginkan", "How you'd like to meet")}
                  name="plan-mode"
                  variant="chips"
                  value={mode}
                  onChange={(v) => setMode(v)}
                  options={(["online", "in_person", "either"] as Mode[]).map((m) => ({ value: m, label: modeLabel(m, w) }))}
                />
                <dl className="c-plan-preview">
                  <div>
                    <dt>{w("Yang dikirim", "What is sent")}</dt>
                    <dd>
                      {w("Ringkasan", "Summary")} “{summary.title || w("tanpa judul", "untitled")}” · v{summary.version}
                    </dd>
                  </div>
                  <div>
                    <dt>{w("Kepada", "To")}</dt>
                    <dd>
                      {org.name}
                      {org.synthetic && !/simulasi|simulation/i.test(org.name) ? ` (${w("simulasi", "simulation")})` : ""} ·{" "}
                      {practitioner ? `${practitioner.displayName}, ${professionLabel(practitioner.professionalType, w)}` : w("siapa saja di layanan ini", "anyone at this service")}
                    </dd>
                  </div>
                  <div>
                    <dt>{w("Tidak dikirim", "Not sent")}</dt>
                    <dd>{w("Nomor telepon, email, catatan lain, dan percakapanmu.", "Your phone number, email, other notes and conversations.")}</dd>
                  </div>
                </dl>
                <CheckboxRow
                  checked={agree}
                  onChange={setAgree}
                  hint={
                    sharing.granted
                      ? undefined
                      : w(
                          "Ini juga mengaktifkan izin berbagi ringkasan, yang bisa kamu matikan di Privasi & data.",
                          "This also turns on summary sharing, which you can turn off in Privacy & data.",
                        )
                  }
                >
                  {w(
                    `Aku setuju mengirim versi ${summary.version} ringkasan ini ke ${org.name}. Mereka bisa membacanya selama 7 hari; aku bisa membatalkan kapan saja.`,
                    `I agree to send version ${summary.version} of this summary to ${org.name}. They can read it for 7 days; I can cancel at any time.`,
                  )}
                </CheckboxRow>
                <ErrorNotice error={error?.at === "send" ? error.error : null} />
                <div className="s-actions">
                  <Button variant="primary" icon="send" disabled={!canSend} busy={busy === "send"} onClick={() => void send()}>
                    {w("Kirim permintaan", "Send request")}
                  </Button>
                </div>
              </div>
            )}
          </section>
        </li>

        <li className="c-plan-step">
          <section aria-labelledby="plan-step-5">
            <h2 id="plan-step-5">
              <span className="c-plan-num" aria-hidden="true">5</span>
              {w("Status permintaanmu", "Your requests")}
            </h2>
            {requests.length === 0 ? (
              <p className="s-hint">{w("Belum ada permintaan.", "No requests yet.")}</p>
            ) : (
              <ul className="c-plan-requests">
                {requests.map((r) => (
                  <li key={r.id} className={`c-plan-request c-plan-request--${r.state}`}>
                    <div className="c-plan-org-head">
                      <h3>
                        {r.orgName}
                        {r.orgSynthetic && <span className="c-plan-badge c-plan-badge--sim">{w("Simulasi", "Simulation")}</span>}
                      </h3>
                      <span className={`c-plan-state c-plan-state--${r.state}`}>{stateLabel(r.state, w)}</span>
                    </div>
                    <p className="s-hint">
                      {w("Kode", "Code")} <code>{r.reference}</code> · {modeLabel(r.mode, w)}
                      {r.practitionerName ? ` · ${r.practitionerName}` : ""} · {loc.dateTime(r.updatedAt)}
                    </p>
                    {(r.state === "sent" || r.state === "proposed") && (
                      <p className="s-hint">
                        {w("Layanan bisa membaca ringkasanmu sampai", "The service can read your summary until")} {loc.dateTime(r.expiresAt)}.
                      </p>
                    )}
                    {r.state === "declined" && r.declineReason && <p>{declineLabel(r.declineReason, w)}</p>}
                    {r.proposal && (r.state === "proposed" || r.state === "confirmed" || r.state === "completed") && (
                      <div className="c-plan-proposal">
                        {r.state === "proposed" ? (
                          <>
                            <p>
                              <strong>{w("Usulan waktu", "Proposed times")}</strong> · {modeLabel(r.proposal.mode, w)}
                            </p>
                            <ul className="c-plan-slots">
                              {r.proposal.slots.map((slot) => (
                                <li key={slot}>
                                  <span>{loc.dateTime(slot)}</span>
                                  <Button
                                    variant="secondary"
                                    disabled={busy !== null}
                                    busy={busy === `confirm:${r.id}`}
                                    onClick={() => void confirm(r, slot)}
                                  >
                                    {w(`Pilih ${loc.dateTime(slot)}`, `Choose ${loc.dateTime(slot)}`)}
                                  </Button>
                                </li>
                              ))}
                            </ul>
                          </>
                        ) : (
                          r.confirmedSlot && (
                            <p>
                              <strong>{w("Waktu terkonfirmasi", "Confirmed time")}</strong>: {loc.dateTime(r.confirmedSlot)} · {modeLabel(r.proposal.mode, w)}
                            </p>
                          )
                        )}
                        <p className="c-plan-contact">
                          <strong>{w("Cara menghubungi mereka", "How to reach them")}</strong>: {r.proposal.contact}
                        </p>
                        {r.proposal.note && <p className="c-plan-contact">{r.proposal.note}</p>}
                      </div>
                    )}
                    {open(r.state) && (
                      <InlineConfirm
                        label={w("Batalkan permintaan", "Cancel request")}
                        question={w(
                          "Batalkan permintaan ini? Layanan itu langsung tidak bisa lagi membaca ringkasanmu.",
                          "Cancel this request? That service immediately loses access to your summary.",
                        )}
                        confirmLabel={w("Batalkan", "Cancel request")}
                        cancelLabel={w("Jangan", "Keep it")}
                        icon="close"
                        busy={busy !== null}
                        onConfirm={() => void cancel(r)}
                      />
                    )}
                  </li>
                ))}
              </ul>
            )}
            <ErrorNotice error={error?.at === "status" ? error.error : null} />
          </section>
        </li>
      </ol>
    </div>
  );
}
