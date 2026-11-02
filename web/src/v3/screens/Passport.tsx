import { useEffect, useState } from "react";
import { Link } from "../../lib/router";
import { api, download as downloadFile, V3Error, type Entry } from "../api";
import { Icon, type IconName } from "../icons";
import { staleCopy, type StaleInfo } from "../meaning";
import { useLocale, useWords } from "../shared";
import {
  Button,
  CheckboxRow,
  EmptyState,
  ErrorNotice,
  LinkButton,
  Loading,
  Notice,
  PageHeader,
  ProvenanceBadge,
  TextArea,
  TextField,
} from "../ui";
import { entryRoute, MeaningLineage, type ContinuityRecord } from "./Continuity";

/*
 * Care Passport (BW-2) is a preset of the existing consultation summary:
 * the summary stays the single source of truth, approval, export, share and
 * care request reuse the summary routes, and the recipient preview comes
 * from GET /consultation-summaries/{id}/passport, which renders the same
 * projection a share freezes. It is not a medical passport or record.
 */

type Words = (id: string, en: string) => string;

export type PassportState = "draft" | "approved" | "needs_review";

type PassportSource = { recordId: string; version: number; kind?: string; label: string; current: boolean };
type RecipientItem = { text: string; quote?: string; sourceLabel: string; position: number };
type RecipientContent = {
  summaryVersion: number;
  title: string;
  text: string;
  prepared?: { goal: string; questions: string; bring: string };
  items: RecipientItem[];
  sources: string[];
};
export type Passport = {
  summaryId: string;
  summaryVersion: number;
  state: PassportState;
  approvedVersion?: number;
  shareable: boolean;
  stale?: StaleInfo;
  sources: PassportSource[];
  recipient: RecipientContent;
  updatedAt: string;
};
type ShareMeta = { id: string; summaryId: string; summaryVersion: number; label: string; state: string; endReason?: string };
type CareRequestMeta = {
  id: string;
  summaryId: string;
  summaryVersion: number;
  orgName: string;
  orgSynthetic: boolean;
  state: string;
};
type SummaryRecord = ContinuityRecord & { approvedVersion?: number };

export const MAX_PASSPORT_QUESTIONS = 5;

/** One question per non-blank line, as the export preset renders them. */
export const passportQuestions = (text: string | undefined) =>
  (text ?? "")
    .split("\n")
    .map((q) => q.trim())
    .filter(Boolean);

/** The list status of a summary from server fields only (CP-10). */
export function summaryPassportState(r: SummaryRecord): PassportState {
  if (r.stale) return "needs_review";
  if (r.summaryState === "approved") return "approved";
  // An approval the server demoted on read: a source changed or vanished.
  if (r.approvedVersion && r.approvedVersion === r.version) return "needs_review";
  return "draft";
}

function stateCopy(state: PassportState, version: number, w: Words): { label: string; icon: IconName } {
  switch (state) {
    case "approved":
      return { label: w(`Versi disetujui · v${version}`, `Approved version · v${version}`), icon: "check" };
    case "needs_review":
      return { label: w("Perlu ditinjau ulang", "Needs review again"), icon: "alert" };
    default:
      return { label: w(`Draf · v${version}`, `Draft · v${version}`), icon: "pencil" };
  }
}

/** Status is always icon + words, never colour alone. */
export function PassportStatus({ state, version }: { state: PassportState; version: number }) {
  const w = useWords();
  const c = stateCopy(state, version, w);
  return (
    <span className={`c-passport-status c-passport-status--${state}`}>
      <Icon name={c.icon} size={16} />
      <span>{c.label}</span>
    </span>
  );
}

function shareStateLabel(s: ShareMeta, w: Words): string {
  if (s.state === "active") return w("Aktif", "Active");
  if (s.endReason === "meaning_invalidated") return w("Berakhir · sumber berubah", "Ended · source changed");
  if (s.state === "expired") return w("Berakhir · kedaluwarsa", "Ended · expired");
  if (s.state === "revoked") return w("Berakhir · dicabut", "Ended · revoked");
  return w("Berakhir", "Ended");
}

function careStateLabel(state: string, w: Words): string {
  switch (state) {
    case "sent":
      return w("Terkirim", "Sent");
    case "proposed":
      return w("Ada usulan waktu", "Times proposed");
    case "confirmed":
      return w("Waktu dipilih", "Time chosen");
    case "completed":
      return w("Selesai", "Completed");
    case "declined":
      return w("Ditolak layanan", "Declined by the service");
    case "expired":
      return w("Berakhir · kedaluwarsa", "Ended · expired");
    default:
      return w("Berakhir · dibatalkan", "Ended · cancelled");
  }
}

function SyntheticBadge() {
  const w = useWords();
  return (
    <span className="c-plan-badge c-plan-badge--sim c-passport-sim">
      {w(
        "SIMULASI — penerima fiktif; tidak ada konsultasi sungguhan terjadi.",
        "SIMULATION — fictional recipient; no real consultation takes place.",
      )}
    </span>
  );
}

function NotMedical() {
  const w = useWords();
  return (
    <Notice tone="info" title={w("Bukan paspor medis", "Not a medical passport")}>
      <p>
        {w(
          "“Care Passport” hanya nama lain lembar persiapan ini. Ini bukan paspor kesehatan resmi, rekam medis, diagnosis, atau dokumen dari tenaga profesional.",
          "“Care Passport” is just a nickname for this preparation sheet. It is not an official health passport, a medical record, a diagnosis or a document from a professional.",
        )}
      </p>
      <p>
        {w(
          "Sela tidak memeriksa atau menetapkan diagnosis. Penerima bisa menyimpan salinan yang sudah dilihat; pencabutan memblokir akses berikutnya, bukan salinan yang sudah diunduh.",
          "Sela does not check or set any diagnosis. A recipient may keep a copy they have seen; revoking blocks later access, not copies already downloaded.",
        )}
      </p>
    </Notice>
  );
}

export function PassportPage({ path }: { path: string }) {
  const id = path.split("/")[3];
  return id ? <PassportDetail key={id} id={id} /> : <PassportList />;
}

function PassportList() {
  const w = useWords();
  const loc = useLocale();
  const [items, setItems] = useState<SummaryRecord[] | null>(null);
  const [shares, setShares] = useState<ShareMeta[]>([]);
  const [error, setError] = useState<unknown>(null);
  const load = () => {
    setError(null);
    api<{ summaries: SummaryRecord[] }>("/consultation-summaries")
      .then((d) => setItems(d.summaries))
      .catch(setError);
    api<{ shares: ShareMeta[] }>("/shares")
      .then((d) => setShares(d.shares))
      .catch(() => setShares([]));
  };
  useEffect(load, []);
  return (
    <div className="c-passport">
      <PageHeader
        back={{ to: "/support", label: w("Dukungan", "Support") }}
        title={w("Ringkasan untuk dibawa", "My consultation preparation")}
        lead={w(
          "Pilih bagian ceritamu yang ingin kamu jelaskan. Kamu bebas mengubah atau tidak membagikannya.",
          "Choose the parts of your story you want to explain. You can edit or keep this private.",
        )}
        actions={
          <LinkButton to="/support/summaries/new" variant="primary">
            {w("Susun ringkasan", "Build a summary")}
          </LinkButton>
        }
      />
      <NotMedical />
      <ErrorNotice error={error} onRetry={load} />
      {!items && !error && <Loading label={w("Membuka…", "Opening…")} />}
      {items?.length === 0 && (
        <EmptyState
          title={w("Belum ada ringkasan", "No summaries yet")}
          body={w(
            "Care Passport memakai ringkasan konsultasi yang kamu susun sendiri. Tidak ada cerita yang dipilih otomatis.",
            "A Care Passport uses a consultation summary you build yourself. No story is selected automatically.",
          )}
          actions={
            <LinkButton to="/support/summaries/new" variant="secondary">
              {w("Susun ringkasan pertama", "Build your first summary")}
            </LinkButton>
          }
        />
      )}
      {items && items.length > 0 && (
        <ul className="c-passport-list">
          {items.map((r) => {
            const state = summaryPassportState(r);
            const mine = shares.filter((s) => s.summaryId === r.id);
            const active = mine.filter((s) => s.state === "active").length;
            const headingId = `passport-${r.id}`;
            return (
              <li key={r.id} className="c-passport-card" aria-labelledby={headingId}>
                <h2 id={headingId}>{r.title || w("Ringkasan pribadi", "Personal summary")}</h2>
                <p className="c-passport-meta">
                  <PassportStatus state={state} version={r.version} />
                  <span className="s-hint">{loc.shortDate(r.updatedAt)}</span>
                </p>
                {mine.length > 0 && (
                  <p className="s-hint">
                    {active > 0
                      ? w(`Dibagikan · ${active} tautan aktif`, `Shared · ${active} active link(s)`)
                      : w("Berbagi sudah berakhir", "Sharing ended")}
                  </p>
                )}
                <div className="s-actions">
                  {state === "needs_review" ? (
                    <LinkButton to={`/support/summaries/${r.id}`} variant="primary">
                      {w("Tinjau ulang", "Review again")}
                    </LinkButton>
                  ) : null}
                  <LinkButton to={`/support/passport/${r.id}`} variant={state === "needs_review" ? "quiet" : "secondary"}>
                    {w("Buka pratinjau", "Open preview")}
                  </LinkButton>
                </div>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}

const conflictMessage = (w: Words) =>
  w(
    "Versi ini sudah berubah sejak kamu membukanya. Kami memuat versi terbaru; periksa lagi sebelum melanjutkan.",
    "This version changed since you opened it. We loaded the latest version; check it again before continuing.",
  );

function PassportDetail({ id }: { id: string }) {
  const w = useWords();
  const loc = useLocale();
  const [passport, setPassport] = useState<Passport | null>(null);
  const [record, setRecord] = useState<SummaryRecord | null>(null);
  const [records, setRecords] = useState<Entry[]>([]);
  const [shares, setShares] = useState<ShareMeta[]>([]);
  const [requests, setRequests] = useState<CareRequestMeta[]>([]);
  const [error, setError] = useState<unknown>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [reviewed, setReviewed] = useState(false);
  const [editing, setEditing] = useState(false);
  const [goal, setGoal] = useState("");
  const [questions, setQuestions] = useState<string[]>([""]);
  const [selected, setSelected] = useState<Record<string, boolean>>({});

  const load = () => {
    setError(null);
    setReviewed(false);
    Promise.all([
      api<{ passport: Passport }>(`/consultation-summaries/${id}/passport`),
      api<SummaryRecord>(`/consultation-summaries/${id}`),
    ])
      .then(([p, r]) => {
        setPassport(p.passport);
        setRecord(r);
        setGoal(r.prepared?.goal ?? "");
        const qs = passportQuestions(r.prepared?.questions);
        setQuestions(qs.length > 0 ? qs.slice(0, MAX_PASSPORT_QUESTIONS) : [""]);
        setSelected(Object.fromEntries((r.sourceRefs ?? []).map((s) => [s.recordId, true])));
      })
      .catch(setError);
    api<{ records: Entry[] }>("/records")
      .then((d) => setRecords(d.records))
      .catch(() => setRecords([]));
    api<{ shares: ShareMeta[] }>("/shares")
      .then((d) => setShares(d.shares.filter((s) => s.summaryId === id)))
      .catch(() => setShares([]));
    api<{ requests: CareRequestMeta[] }>("/care-requests")
      .then((d) => setRequests(d.requests.filter((r) => r.summaryId === id)))
      .catch(() => setRequests([]));
  };
  useEffect(load, [id]);

  const run = async (action: () => Promise<unknown>, done?: string) => {
    setError(null);
    setNotice(null);
    setBusy(true);
    try {
      await action();
      if (done) setNotice(done);
      load();
    } catch (e) {
      if (e instanceof V3Error && e.code === "stale_version") {
        setNotice(conflictMessage(w));
        load();
      } else {
        setError(e);
      }
    } finally {
      setBusy(false);
    }
  };

  const patch = (body: Record<string, unknown>) => {
    if (!record) return Promise.resolve();
    return api(
      `/consultation-summaries/${id}`,
      {
        title: record.title,
        text: record.text,
        sourceRefs: record.sourceRefs ?? [],
        prepared: record.prepared ?? { goal: "", questions: "", bring: "" },
        storageGranted: true,
        expectedVersion: record.version,
        ...body,
      },
      "PATCH",
    );
  };

  // Re-check the server before any download so a stale page never exports
  // as approved (CP-10).
  const downloadPassport = async (format: "html" | "md") => {
    if (!passport?.shareable) return;
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      const fresh = await api<{ passport: Passport }>(`/consultation-summaries/${id}/passport`);
      if (!fresh.passport.shareable || fresh.passport.summaryVersion !== passport.summaryVersion) {
        setPassport(fresh.passport);
        setNotice(conflictMessage(w));
        load();
        return;
      }
      await downloadFile(
        `/consultation-summaries/${id}/export?format=${format}&preset=passport&lang=${loc.lang}`,
        `sela-persiapan-konsultasi.${format}`,
      );
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  };

  const sourceRecord = (recordId: string) => records.find((r) => r.id === recordId);
  const ownQuestions = passportQuestions(record?.prepared?.questions);
  const questionsOk = ownQuestions.length >= 1 && ownQuestions.length <= MAX_PASSPORT_QUESTIONS;
  const draftQuestions = questions.map((q) => q.trim()).filter(Boolean);
  const itemsBySource = (recordId: string) => (record?.items ?? []).filter((i) => i.sourceRef.recordId === recordId).length;
  const selectedCount = Object.values(selected).filter(Boolean).length;
  const selectionChanged = (record?.sourceRefs ?? []).some((s) => !selected[s.recordId]);
  const synthetic = requests.some((r) => r.orgSynthetic);

  return (
    <div className="s-narrow c-passport">
      <PageHeader
        back={{ to: "/support/passport", label: w("Ringkasan untuk dibawa", "My consultation preparation") }}
        title={w("Ringkasan untuk dibawa", "My consultation preparation")}
        lead={w(
          "Pilih bagian ceritamu yang ingin kamu jelaskan. Kamu bebas mengubah atau tidak membagikannya.",
          "Choose the parts of your story you want to explain. You can edit or keep this private.",
        )}
      />
      <ErrorNotice error={error} onRetry={load} />
      {notice && (
        <div role="status">
          <Notice tone="info">
            <p>{notice}</p>
          </Notice>
        </div>
      )}
      {(!passport || !record) && !error && <Loading label={w("Membuka…", "Opening…")} />}
      {passport && record && (
        <>
          <section className="c-passport-head" aria-labelledby="passport-status-title">
            <h2 id="passport-status-title">{record.title || w("Ringkasan pribadi", "Personal summary")}</h2>
            <p className="c-passport-meta">
              <PassportStatus state={passport.state} version={passport.summaryVersion} />
              <span className="s-hint">{loc.shortDate(passport.updatedAt)}</span>
            </p>
            {synthetic && <SyntheticBadge />}
          </section>
          <NotMedical />

          {passport.state === "needs_review" && (
            <Notice
              tone="warning"
              title={w("Perlu ditinjau ulang", "Needs review again")}
              actions={
                <LinkButton to={`/support/summaries/${id}`} variant="primary">
                  {w("Tinjau ulang ringkasan", "Review the summary again")}
                </LinkButton>
              }
            >
              <p>
                {passport.stale
                  ? staleCopy(passport.stale.reason, w).body
                  : w(
                      "Salah satu sumber yang kamu pilih berubah atau dihapus. Versi ini tidak bisa disetujui, diunduh, atau dibagikan sampai kamu meninjaunya.",
                      "One of the sources you chose changed or was deleted. This version cannot be approved, downloaded or shared until you review it.",
                    )}
              </p>
              <p>
                {w(
                  "Tautan dan permintaan yang memakai versi lama sudah berakhir. Salinan yang sudah diunduh penerima tidak bisa ditarik kembali.",
                  "Links and requests that used the old version have ended. Copies a recipient already downloaded cannot be recalled.",
                )}
              </p>
            </Notice>
          )}

          <section className="c-passport-section" aria-labelledby="passport-goal">
            <h2 id="passport-goal">{w("Tujuanku", "My purpose")}</h2>
            {record.prepared?.goal?.trim() ? (
              <p className="c-original">{record.prepared.goal}</p>
            ) : (
              <p className="s-hint">{w("Belum diisi. Tujuan boleh kosong.", "Not filled in. A purpose is optional.")}</p>
            )}
          </section>

          <section className="c-passport-section" aria-labelledby="passport-share">
            <h2 id="passport-share">{w("Yang ingin kubagikan", "What I want to share")}</h2>
            <p className="s-hint">
              {w(
                "Hanya sumber yang dicentang yang ikut. Catatan lain tidak pernah masuk ke pratinjau, unduhan, atau tautan.",
                "Only checked sources are included. Other notes never enter the preview, download or link.",
              )}
            </p>
            <ul className="c-passport-sources">
              {passport.sources.map((s) => {
                const src = sourceRecord(s.recordId);
                const usedBy = itemsBySource(s.recordId);
                const lastOne = selected[s.recordId] && selectedCount === 1;
                return (
                  <li key={s.recordId} className="c-passport-source">
                    <CheckboxRow
                      checked={Boolean(selected[s.recordId])}
                      onChange={(v) => setSelected({ ...selected, [s.recordId]: v })}
                      disabled={busy || usedBy > 0 || lastOne || passport.state === "needs_review"}
                      hint={
                        usedBy > 0
                          ? w(
                              `Dipakai ${usedBy} pernyataan; hapus pernyataannya dulu di ringkasan.`,
                              `Used by ${usedBy} statement(s); remove them in the summary first.`,
                            )
                          : lastOne
                            ? w("Minimal satu sumber tetap dipilih.", "At least one source stays selected.")
                            : undefined
                      }
                    >
                      {s.label || w("Tulisan asli", "Original writing")} · v{s.version}
                    </CheckboxRow>
                    <p className="s-hint">
                      {s.current
                        ? w("Masih versi yang sama", "Still the same version")
                        : src
                          ? w("Sumber sudah berubah sejak dipilih", "The source changed since you chose it")
                          : w("Sumber sudah dihapus", "Source deleted")}
                      {src && (
                        <>
                          {" · "}
                          <Link to={entryRoute(src)}>{w("Periksa sumber", "Recheck source")}</Link>
                        </>
                      )}
                    </p>
                  </li>
                );
              })}
            </ul>
            {selectionChanged && (
              <div className="s-actions">
                <Button
                  variant="primary"
                  disabled={busy || selectedCount === 0}
                  onClick={() =>
                    void run(
                      () => patch({ sourceRefs: (record.sourceRefs ?? []).filter((s) => selected[s.recordId]) }),
                      w("Pilihan sumber disimpan sebagai draf baru.", "Source choice saved as a new draft."),
                    )
                  }
                >
                  {w("Simpan pilihan sumber", "Save source choice")}
                </Button>
                <Button
                  disabled={busy}
                  onClick={() => setSelected(Object.fromEntries((record.sourceRefs ?? []).map((s) => [s.recordId, true])))}
                >
                  {w("Batal", "Cancel")}
                </Button>
              </div>
            )}
            <p className="s-hint">
              <Link to="/support/summaries/new">{w("Tambah sumber lewat penyusun ringkasan", "Add sources in the summary builder")}</Link>
            </p>
          </section>

          <section className="c-passport-section" aria-labelledby="passport-words">
            <h2 id="passport-words">{w("Dengan kata-kataku", "In my words")}</h2>
            {(record.items ?? []).length === 0 ? (
              <p className="s-hint">
                {w("Belum ada pernyataan berkutipan. Kamu bisa menambahkannya di ringkasan.", "No quote-backed statements yet. You can add them in the summary.")}
              </p>
            ) : (
              <ol className="c-summary-items-list">
                {(record.items ?? []).map((item, index) => {
                  const src = sourceRecord(item.sourceRef.recordId);
                  const label = passport.sources.find((s) => s.recordId === item.sourceRef.recordId)?.label;
                  const projected = passport.recipient.items[index];
                  const masked = projected && (projected.quote ?? "") !== (item.quote ?? "");
                  return (
                    <li key={item.id} className="c-summary-item">
                      <p className="c-original">{item.text}</p>
                      {item.quote?.trim() && (
                        <blockquote className="c-memory-quote">
                          <p>{item.quote}</p>
                        </blockquote>
                      )}
                      <p className="c-passport-meta">
                        <ProvenanceBadge value={item.provenance} />
                        <span className="s-hint">
                          {label || w("Sumber", "Source")} · v{item.sourceRef.version}
                        </span>
                      </p>
                      {masked && (
                        <p className="s-hint">
                          {w("Untuk penerima, nama pribadi disamarkan: ", "For the recipient, private names are masked: ")}
                          {projected.quote}
                        </p>
                      )}
                      <p className="s-hint">
                        {src ? (
                          <Link to={entryRoute(src)}>{w("Periksa kutipan di sumbernya", "Recheck the quote in its source")}</Link>
                        ) : (
                          w("Sumber sudah dihapus", "Source deleted")
                        )}
                        {item.claimRef && (
                          <>
                            {" · "}
                            <Link to={`/notes/memory/${item.claimRef.recordId}`}>
                              {w("Pemahaman yang kamu konfirmasi", "The meaning you confirmed")}
                            </Link>
                          </>
                        )}
                      </p>
                    </li>
                  );
                })}
              </ol>
            )}
            <MeaningLineage
              id={record.id}
              version={record.version}
              label={(n) => passport.sources.find((s) => s.recordId === n.id)?.label || w("Sumber", "Source")}
            />
          </section>

          <section className="c-passport-section" aria-labelledby="passport-ask">
            <h2 id="passport-ask">{w("Yang ingin kutanyakan", "What I want to ask")}</h2>
            {ownQuestions.length > 0 ? (
              <ol className="c-passport-questions">
                {ownQuestions.map((q, i) => (
                  <li key={i}>{q}</li>
                ))}
              </ol>
            ) : (
              <p className="s-hint">{w("Belum ada pertanyaan. Tulis 1–5 pertanyaan.", "No questions yet. Write 1–5 questions.")}</p>
            )}
            {ownQuestions.length > MAX_PASSPORT_QUESTIONS && (
              <p className="s-hint">{w("Pilih paling banyak 5 pertanyaan.", "Choose at most 5 questions.")}</p>
            )}
          </section>

          {editing ? (
            <form
              className="c-passport-section c-passport-edit"
              aria-labelledby="passport-edit-title"
              onSubmit={(e) => {
                e.preventDefault();
                void run(async () => {
                  await patch({
                    prepared: {
                      goal: goal.trim(),
                      questions: draftQuestions.join("\n"),
                      bring: record.prepared?.bring ?? "",
                    },
                  });
                  setEditing(false);
                }, w("Persiapan disimpan sebagai draf baru. Setujui lagi sebelum dibawa.", "Preparation saved as a new draft. Approve it again before taking it."));
              }}
            >
              <h2 id="passport-edit-title">{w("Ubah tujuan dan pertanyaan", "Edit purpose and questions")}</h2>
              <TextArea
                label={w("Tujuanku", "My purpose")}
                hint={w("Misalnya: “Aku ingin bisa menjelaskan …”", "For example: “I want to be able to explain …”")}
                value={goal}
                onChange={(e) => setGoal(e.target.value)}
                maxLength={6000}
                rows={3}
                optional
              />
              {questions.map((q, i) => (
                <TextField
                  key={i}
                  label={w(`Pertanyaan ${i + 1}`, `Question ${i + 1}`)}
                  value={q}
                  maxLength={400}
                  onChange={(e) => setQuestions(questions.map((x, j) => (j === i ? e.target.value.replace(/\n/g, " ") : x)))}
                  optional={i > 0}
                />
              ))}
              <div className="s-actions">
                {questions.length < MAX_PASSPORT_QUESTIONS && (
                  <Button onClick={() => setQuestions([...questions, ""])} disabled={busy}>
                    {w("Tambah pertanyaan", "Add a question")}
                  </Button>
                )}
                {questions.length > 1 && (
                  <Button variant="quiet" onClick={() => setQuestions(questions.slice(0, -1))} disabled={busy}>
                    {w("Hapus pertanyaan terakhir", "Remove the last question")}
                  </Button>
                )}
              </div>
              <p className="s-hint">
                {w(
                  "Menyimpan membuat versi baru dan membatalkan persetujuan versi lama.",
                  "Saving creates a new version and withdraws approval of the old one.",
                )}
              </p>
              <div className="s-actions">
                <Button type="submit" variant="primary" disabled={busy || draftQuestions.length > MAX_PASSPORT_QUESTIONS}>
                  {w("Simpan perubahan", "Save changes")}
                </Button>
                <Button onClick={() => setEditing(false)} disabled={busy}>
                  {w("Batal", "Cancel")}
                </Button>
              </div>
            </form>
          ) : (
            passport.state !== "needs_review" && (
              <div className="s-actions">
                <Button variant="secondary" icon="pencil" onClick={() => setEditing(true)} disabled={busy}>
                  {w("Ubah tujuan dan pertanyaan", "Edit purpose and questions")}
                </Button>
              </div>
            )
          )}

          <section className="c-passport-section c-passport-preview" aria-labelledby="passport-preview">
            <h2 id="passport-preview">{w("Yang diterima penerima", "What a recipient receives")}</h2>
            <p>
              {w(
                "Inilah persis isi yang diterima penerima bila kamu membagikan versi ini, dari server yang sama dengan tautan berbagi.",
                "This is exactly what a recipient gets if you share this version, from the same server projection as a share link.",
              )}
            </p>
            <article className="c-paper" aria-label={w("Pratinjau penerima", "Recipient preview")}>
              {passport.recipient.title && <h3>{passport.recipient.title}</h3>}
              <p className="s-hint">
                {w("Versi", "Version")} {passport.recipient.summaryVersion}
              </p>
              {passport.recipient.prepared?.goal?.trim() && (
                <>
                  <h4>{w("Tujuanku", "My purpose")}</h4>
                  <p className="c-original">{passport.recipient.prepared.goal}</p>
                </>
              )}
              <h4>{w("Sumber yang dipilih", "Chosen sources")}</h4>
              <ul>
                {passport.recipient.sources.map((s, i) => (
                  <li key={i}>{s}</li>
                ))}
              </ul>
              {passport.recipient.items.length > 0 && (
                <>
                  <h4>{w("Dengan kata-kataku", "In my words")}</h4>
                  <ol>
                    {passport.recipient.items.map((item) => (
                      <li key={item.position}>
                        <p className="c-original">{item.text}</p>
                        {item.quote?.trim() && (
                          <blockquote className="c-memory-quote">
                            <p>{item.quote}</p>
                          </blockquote>
                        )}
                        <p className="s-hint">{item.sourceLabel}</p>
                      </li>
                    ))}
                  </ol>
                </>
              )}
              {passport.recipient.text.trim() && (
                <>
                  <h4>{w("Ringkasanku", "My summary")}</h4>
                  <p className="c-original">{passport.recipient.text}</p>
                </>
              )}
              {passportQuestions(passport.recipient.prepared?.questions).length > 0 && (
                <>
                  <h4>{w("Yang ingin kutanyakan", "What I want to ask")}</h4>
                  <ol>
                    {passportQuestions(passport.recipient.prepared?.questions).map((q, i) => (
                      <li key={i}>{q}</li>
                    ))}
                  </ol>
                </>
              )}
              {passport.recipient.prepared?.bring?.trim() && (
                <>
                  <h4>{w("Yang kubawa", "What I will bring")}</h4>
                  <p className="c-original">{passport.recipient.prepared.bring}</p>
                </>
              )}
            </article>
            <h3>{w("Tidak ikut dikirim", "Never sent")}</h3>
            <ul>
              <li>{w("ID akun, profil, dan sesi kamu", "Your account ID, profile and sessions")}</li>
              <li>{w("Catatan yang tidak kamu pilih", "Notes you did not choose")}</li>
              <li>{w("Nama yang kamu tandai pribadi (diganti [NAMA-n])", "Names you marked private (replaced by [NAMA-n])")}</li>
              <li>{w("Label asal dan status tinjauan internal", "Internal provenance labels and review states")}</li>
            </ul>
          </section>

          {passport.state === "draft" && !editing && (
            <section className="c-passport-section c-approval" aria-labelledby="passport-approve">
              <h2 id="passport-approve">{w("Periksa dan setujui versi ini", "Review and approve this version")}</h2>
              {!questionsOk && (
                <p className="s-hint">
                  {w(
                    "Tulis 1–5 pertanyaan lewat “Ubah tujuan dan pertanyaan” sebelum menyetujui.",
                    "Write 1–5 questions with “Edit purpose and questions” before approving.",
                  )}
                </p>
              )}
              <CheckboxRow checked={reviewed} onChange={setReviewed} disabled={busy || !questionsOk}>
                {w(
                  `Aku sudah memeriksa versi ${passport.summaryVersion} dan isinya mewakili yang ingin kusampaikan`,
                  `I reviewed version ${passport.summaryVersion} and it represents what I want to say`,
                )}
              </CheckboxRow>
              <Button
                variant="primary"
                disabled={busy || !reviewed || !questionsOk}
                onClick={() =>
                  void run(
                    () => api(`/consultation-summaries/${id}/approve`, { expectedVersion: passport.summaryVersion }, "POST"),
                    w(`Versi ${passport.summaryVersion} disetujui.`, `Version ${passport.summaryVersion} approved.`),
                  )
                }
              >
                {w("Setujui versi ini", "Approve this version")}
              </Button>
            </section>
          )}

          {passport.shareable && !editing && (
            <section className="c-passport-section c-export" aria-labelledby="passport-use">
              <h2 id="passport-use">{w("Bawa dengan caramu", "Take it your way")}</h2>
              <p>
                {w(
                  "Kamu boleh menyimpannya sendiri saja. Tidak ada yang dikirim otomatis dan tidak ada penerima yang dihubungi.",
                  "You may keep it to yourself. Nothing is sent automatically and no recipient is contacted.",
                )}
              </p>
              <div className="s-actions">
                <Button variant="primary" icon="download" disabled={busy} onClick={() => void downloadPassport("html")}>
                  {w("Unduh HTML (untuk dicetak)", "Download HTML (to print)")}
                </Button>
                <Button variant="secondary" disabled={busy} onClick={() => void downloadPassport("md")}>
                  {w("Unduh Markdown", "Download Markdown")}
                </Button>
              </div>
              <div className="s-actions">
                <LinkButton to={`/support/shares?summary=${encodeURIComponent(id)}`} variant="secondary" icon="share">
                  {w("Bagikan dengan tautan dan kode", "Share with a link and code")}
                </LinkButton>
                <LinkButton to={`/support/plan?summary=${encodeURIComponent(id)}`} variant="quiet">
                  {w("Kirim ke layanan di jaringan", "Send to a network service")}
                </LinkButton>
              </div>
              <p className="s-hint">
                {w(
                  "Layanan simulasi selalu diberi label SIMULASI. Mengirim tidak berarti janji temu sudah dipesan.",
                  "Simulated services are always labelled SIMULATION. Sending does not mean an appointment is booked.",
                )}
              </p>
            </section>
          )}

          {(shares.length > 0 || requests.length > 0) && (
            <section className="c-passport-section" aria-labelledby="passport-recipients">
              <h2 id="passport-recipients">{w("Penerima dan statusnya", "Recipients and their status")}</h2>
              <ul className="c-passport-recipients">
                {shares.map((s) => (
                  <li key={s.id}>
                    <strong>{s.label}</strong> · v{s.summaryVersion} · {shareStateLabel(s, w)}
                  </li>
                ))}
                {requests.map((r) => (
                  <li key={r.id}>
                    <strong>{r.orgName}</strong> · v{r.summaryVersion} · {careStateLabel(r.state, w)}
                    {r.orgSynthetic && (
                      <>
                        {" "}
                        <SyntheticBadge />
                      </>
                    )}
                  </li>
                ))}
              </ul>
              <p className="s-hint">
                {w(
                  "Mencabut atau mengubah sumber hanya menghentikan akses berikutnya. Yang sudah dilihat atau diunduh penerima tidak bisa ditarik kembali.",
                  "Revoking or changing a source only stops later access. What a recipient already saw or downloaded cannot be recalled.",
                )}
              </p>
              <LinkButton to="/support/shares" variant="quiet">
                {w("Kelola tautan berbagi", "Manage share links")}
              </LinkButton>
            </section>
          )}
        </>
      )}
    </div>
  );
}
