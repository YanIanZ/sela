import { useEffect, useMemo, useState, type FormEvent } from "react";
import { Link, useRouter } from "../../lib/router";
import { api, V3Error, type Entry, type Profile } from "../api";
import { useConsent } from "../consent";
import { useAnnounce, useLocale, useWords } from "../shared";
import {
  Button,
  EmptyState,
  ErrorNotice,
  InlineConfirm,
  LinkButton,
  Loading,
  Notice,
  PageHeader,
  SelectField,
  StatusLine,
  TextArea,
  TextField,
} from "../ui";
import "../actionsteps.css";

/*
 * Satu langkah, pilihanku / One step I choose (BW-3). One small, optional
 * step in the owner's own words, or a tentative Meta proposal the owner can
 * accept, rewrite or reject. Nothing is stored until the owner saves it;
 * there are no reminders, streaks or grades, and "Sudah kucoba" is the
 * owner's own report. The whole page works with AI consent off.
 */

export type ActionStatus = "chosen" | "tried" | "paused" | "discarded" | "needs_review";

export type ActionSourceRef = { recordId: string; version: number; kind?: string; current: boolean };

export type ActionStep = {
  id: string;
  version: number;
  text: string;
  status: ActionStatus;
  dueDate?: string;
  sourceRefs: ActionSourceRef[];
  origin: { kind: "user_written" | "meta_proposal"; edited?: boolean; model?: string; draft?: string };
  review: { userApproved: boolean; stale: boolean; staleReason?: string; staleAt?: string };
  scope: "private";
  safety: string;
  createdAt: string;
  updatedAt: string;
};

export type ActionProposal = {
  text: string;
  token: string;
  sourceRef: { recordId: string; version: number };
  origin: { kind: "meta_proposal"; model?: string; draft?: string };
  tentative: boolean;
  expiresInSeconds: number;
};

type SuggestResult = { aiState: string; proposal: ActionProposal | null; help: boolean };

type Mode = "start" | "write" | "suggest" | "skipped";

const SOURCE_KINDS = new Set(["note", "checkin", "conversation"]);

function sourcePath(id: string, kind?: string): string {
  return kind === "conversation" ? `/talk?record=${id}` : `/notes/${id}`;
}

export function ActionStepsPage({ user }: { user: Profile }) {
  const w = useWords();
  const loc = useLocale();
  const { query } = useRouter();
  const { announce } = useAnnounce();
  const aiConsent = useConsent("ai_processing");
  const preselected = query.get("source") ?? "";

  const [steps, setSteps] = useState<ActionStep[] | null>(null);
  const [loadError, setLoadError] = useState<unknown>(null);
  const [sources, setSources] = useState<Entry[]>([]);
  const [mode, setMode] = useState<Mode>("start");

  const [text, setText] = useState("");
  const [due, setDue] = useState("");
  const [attach, setAttach] = useState(preselected);
  const [accepted, setAccepted] = useState<ActionProposal | null>(null);
  const [saveError, setSaveError] = useState<unknown>(null);
  const [textError, setTextError] = useState("");
  const [busy, setBusy] = useState(false);

  const [suggestSource, setSuggestSource] = useState(preselected);
  const [suggesting, setSuggesting] = useState(false);
  const [result, setResult] = useState<SuggestResult | null>(null);
  const [suggestError, setSuggestError] = useState<unknown>(null);

  const [cardError, setCardError] = useState<{ id: string; error: unknown } | null>(null);
  const [cardBusy, setCardBusy] = useState("");
  const [editing, setEditing] = useState<{ id: string; text: string } | null>(null);
  const [notice, setNotice] = useState("");

  const load = () => {
    setLoadError(null);
    api<{ actionSteps: ActionStep[] }>("/action-steps")
      .then((d) => setSteps(d.actionSteps))
      .catch((e) => setLoadError(e));
    api<{ records: Entry[] }>("/records")
      .then((d) => setSources(d.records.filter((r) => SOURCE_KINDS.has(r.kind))))
      .catch(() => setSources([]));
  };
  useEffect(load, []);

  const sourceById = useMemo(() => new Map(sources.map((s) => [s.id, s])), [sources]);

  const sourceLabel = (s: Entry) => {
    const kind =
      s.kind === "conversation"
        ? w("Percakapan", "Conversation")
        : s.kind === "checkin"
          ? w("Cek perasaan", "Check-in")
          : w("Catatan", "Note");
    const title = (s.title || s.text || "").trim().slice(0, 48);
    return `${kind} · ${title || loc.date(s.updatedAt)}`;
  };

  const statusLabel = (s: ActionStatus) =>
    s === "tried"
      ? w("Sudah kucoba", "I tried it")
      : s === "paused"
        ? w("Nanti dulu", "Not now")
        : s === "discarded"
          ? w("Tidak cocok", "Not for me")
          : s === "needs_review"
            ? w("Perlu ditinjau", "Needs review")
            : w("Kupilih", "Chosen");

  const originLabel = (step: ActionStep) =>
    step.origin.kind === "meta_proposal"
      ? step.origin.edited
        ? w("Usulan Meta yang kamu ubah lalu pilih", "A Meta suggestion you rewrote and chose")
        : w("Usulan Meta yang kamu pilih", "A Meta suggestion you chose")
      : w("Ditulis olehmu", "Written by you");

  const resetEditor = () => {
    setText("");
    setDue("");
    setAttach(preselected);
    setAccepted(null);
    setSaveError(null);
    setTextError("");
  };

  const choose = (next: Mode) => {
    setNotice("");
    setResult(null);
    setSuggestError(null);
    if (next === "write") resetEditor();
    setMode(next);
    if (next === "skipped") {
      announce("status", w("Tidak apa-apa. Tidak ada yang disimpan.", "That's fine. Nothing was saved."));
    }
  };

  const save = async (e: FormEvent) => {
    e.preventDefault();
    if (!text.trim()) {
      setTextError(w("Tulis langkahnya dengan kata-katamu sendiri.", "Write the step in your own words."));
      return;
    }
    setTextError("");
    setSaveError(null);
    setBusy(true);
    const src = attach ? sourceById.get(attach) : undefined;
    try {
      const step = await api<ActionStep>("/action-steps", {
        text,
        dueDate: due,
        storageGranted: true,
        sourceRefs: src ? [{ recordId: src.id, version: src.version }] : [],
        ...(accepted ? { proposal: { token: accepted.token, text: accepted.text } } : {}),
      });
      setSteps((list) => [step, ...(list ?? [])]);
      resetEditor();
      setMode("start");
      setNotice(w("Langkahmu tersimpan. Kamu bisa mengubah atau membuangnya kapan saja.", "Your step is saved. You can change or discard it any time."));
      announce("status", w("Langkahmu tersimpan.", "Your step is saved."));
    } catch (err) {
      setSaveError(err);
    } finally {
      setBusy(false);
    }
  };

  const askSuggestion = async () => {
    const src = sourceById.get(suggestSource);
    if (!src) return;
    setSuggesting(true);
    setSuggestError(null);
    setResult(null);
    try {
      const r = await api<SuggestResult>("/action-steps/suggest", { sourceRef: { recordId: src.id, version: src.version } }, "POST", undefined, 100_000);
      setResult(r);
      announce(
        "status",
        r.aiState === "released"
          ? w("Ada satu usulan untuk kamu pertimbangkan.", "There is one suggestion for you to consider.")
          : w("Usulan tidak tersedia. Kamu tetap bisa menulis sendiri.", "No suggestion is available. You can still write your own."),
      );
    } catch (err) {
      setSuggestError(err);
    } finally {
      setSuggesting(false);
    }
  };

  const acceptProposal = (p: ActionProposal) => {
    resetEditor();
    setAccepted(p);
    setText(p.text);
    // The source stays detached unless the owner ticks it.
    setAttach("");
    setResult(null);
    setMode("write");
  };

  const rejectProposal = () => {
    setResult(null);
    setMode("start");
    announce("status", w("Usulan dibuang. Tidak ada yang disimpan.", "Suggestion dismissed. Nothing was saved."));
  };

  const patch = async (step: ActionStep, body: Record<string, unknown>) => {
    setCardError(null);
    setCardBusy(step.id);
    try {
      const next = await api<ActionStep>("/action-steps/" + step.id, { expectedVersion: step.version, ...body }, "PATCH");
      setSteps((list) => (list ?? []).map((s) => (s.id === step.id ? next : s)));
      setEditing(null);
      announce("status", w("Langkah diperbarui: ", "Step updated: ") + statusLabel(next.status));
    } catch (err) {
      setCardError({ id: step.id, error: err });
      if (err instanceof V3Error && err.code === "stale_version") load();
    } finally {
      setCardBusy("");
    }
  };

  const remove = async (step: ActionStep) => {
    setCardError(null);
    setCardBusy(step.id);
    try {
      await api("/action-steps/" + step.id, { expectedVersion: step.version }, "DELETE");
      setSteps((list) => (list ?? []).filter((s) => s.id !== step.id));
      announce("status", w("Langkah dihapus.", "Step deleted."));
    } catch (err) {
      setCardError({ id: step.id, error: err });
      if (err instanceof V3Error && err.code === "stale_version") load();
    } finally {
      setCardBusy("");
    }
  };

  const sourceOptions = sources.map((s) => (
    <option key={s.id} value={s.id}>
      {sourceLabel(s)}
    </option>
  ));

  return (
    <div className="a-steps">
      <PageHeader
        title={w("Satu langkah, pilihanku", "One step I choose")}
        lead={w(
          "Kalau kamu mau, pilih satu langkah kecil yang terasa bisa dicoba. Boleh juga dilewati.",
          "If you like, choose one small step that feels doable. Skipping is fine too.",
        )}
        back={{ to: "/notes", label: w("Catatanku", "My notes") }}
      />
      <p className="a-steps-promise">
        {w(
          "Hanya kamu yang memilih apa yang dicoba; tidak ada nilai.",
          "Only you choose what to try; there are no grades.",
        )}{" "}
        {w(
          "Tidak ada pengingat atau hitungan hari. Langkah ini bukan pengobatan.",
          "There are no reminders or day counts. A step is not treatment.",
        )}
      </p>

      {notice && <StatusLine tone="success">{notice}</StatusLine>}

      {mode === "start" || mode === "skipped" ? (
        <section className="a-steps-start" aria-labelledby="steps-start">
          <h2 id="steps-start">{w("Mau mulai dari mana?", "Where would you like to start?")}</h2>
          {mode === "skipped" && (
            <StatusLine>{w("Tidak apa-apa. Tidak ada yang disimpan.", "That's fine. Nothing was saved.")}</StatusLine>
          )}
          <div className="s-actions">
            <Button variant="primary" icon="pencil" onClick={() => choose("write")}>
              {w("Tulis sendiri", "Write my own")}
            </Button>
            <Button variant="secondary" onClick={() => choose("suggest")}>
              {w("Lihat usulan", "See a suggestion")}
            </Button>
            <Button variant="quiet" onClick={() => choose("skipped")}>
              {w("Lewati", "Not now")}
            </Button>
          </div>
        </section>
      ) : null}

      {mode === "write" && (
        <form className="a-steps-editor" onSubmit={save} aria-labelledby="steps-write" noValidate>
          <h2 id="steps-write">{w("Langkahku", "My step")}</h2>
          {accepted && (
            <p className="s-hint">
              {w(
                "Ini usulan Meta. Ubah sesukamu; yang tersimpan adalah kata-kata yang kamu pilih.",
                "This is a Meta suggestion. Change it as you like; what is saved is the wording you choose.",
              )}
            </p>
          )}
          <TextArea
            label={w("Dengan kata-katamu sendiri", "In your own words")}
            hint={w("Satu hal kecil, misalnya menulis dua pertanyaan.", "One small thing, for example writing down two questions.")}
            value={text}
            onChange={(e) => setText(e.target.value)}
            maxLength={500}
            rows={3}
            error={textError || undefined}
          />
          <TextField
            label={w("Tanggal rencana", "Planned date")}
            hint={w("Boleh dikosongkan. Sela tidak akan mengingatkan.", "You can leave it empty. Sela won't remind you.")}
            type="date"
            optional
            value={due}
            onChange={(e) => setDue(e.target.value)}
          />
          <SelectField
            label={w("Kaitkan dengan catatan", "Link to a note")}
            hint={w(
              "Hanya jika kamu mau. Jika catatan itu berubah, langkah ini akan ditandai untuk ditinjau.",
              "Only if you want. If that note changes, this step is marked for review.",
            )}
            optional
            value={attach}
            onChange={(e) => setAttach(e.target.value)}
          >
            <option value="">{w("Tanpa catatan", "No note")}</option>
            {sourceOptions}
          </SelectField>
          <ErrorNotice error={saveError} />
          <div className="s-actions">
            <Button type="submit" variant="primary" busy={busy} disabled={busy}>
              {w("Simpan langkah", "Save step")}
            </Button>
            <Button
              variant="quiet"
              onClick={() => {
                resetEditor();
                setMode("start");
              }}
            >
              {w("Batal", "Cancel")}
            </Button>
          </div>
        </form>
      )}

      {mode === "suggest" && (
        <section className="a-steps-proposal" aria-labelledby="steps-suggest">
          <h2 id="steps-suggest">{w("Usulan dari Meta", "A suggestion from Meta")}</h2>
          {aiConsent.loading ? (
            <Loading label={w("Memeriksa izin…", "Checking permission…")} />
          ) : !aiConsent.granted ? (
            <Notice tone="info" title={w("Usulan memerlukan izin pemrosesan AI", "Suggestions need AI processing permission")}>
              <p>
                {w(
                  "Izin itu sedang mati, jadi tidak ada yang dikirim ke AI. Kamu tetap bisa menulis langkahmu sendiri.",
                  "That permission is off, so nothing is sent to AI. You can still write your own step.",
                )}
              </p>
              <div className="s-actions">
                <Button variant="primary" onClick={() => choose("write")}>
                  {w("Tulis sendiri", "Write my own")}
                </Button>
                <LinkButton to="/settings/privacy" variant="quiet">
                  {w("Atur izin", "Manage permissions")}
                </LinkButton>
              </div>
            </Notice>
          ) : (
            <>
              <p className="s-hint">
                {w(
                  "Pilih satu catatan tersimpan. Hanya potongan singkatnya yang dikirim ke Meta, dan usulannya tidak disimpan kecuali kamu memilihnya.",
                  "Choose one saved note. Only a short excerpt is sent to Meta, and the suggestion is not saved unless you choose it.",
                )}
              </p>
              {sources.length === 0 ? (
                <p>{w("Belum ada catatan tersimpan untuk dijadikan dasar.", "There is no saved note to base it on yet.")}</p>
              ) : (
                <SelectField
                  label={w("Catatan yang jadi dasar", "The note to base it on")}
                  value={suggestSource}
                  onChange={(e) => setSuggestSource(e.target.value)}
                >
                  <option value="">{w("Pilih catatan", "Choose a note")}</option>
                  {sourceOptions}
                </SelectField>
              )}
              {!result && (
                <div className="s-actions">
                  <Button variant="primary" busy={suggesting} disabled={suggesting || !suggestSource} onClick={() => void askSuggestion()}>
                    {suggesting ? w("Meminta usulan…", "Asking…") : w("Minta satu usulan", "Ask for one suggestion")}
                  </Button>
                  <Button variant="quiet" onClick={() => choose("write")}>
                    {w("Tulis sendiri saja", "Write my own instead")}
                  </Button>
                </div>
              )}
              <ErrorNotice error={suggestError} />
              {result?.aiState === "crisis" && (
                <Notice tone="help" title={w("Kamu tidak harus menghadapinya sendirian", "You don't have to face this alone")}>
                  <p>
                    {w(
                      "Tulisanmu terdengar sangat berat. Sela tidak memberi usulan langkah sekarang. Bantuan bisa dihubungi kapan saja.",
                      "Your words sound very heavy. Sela is not suggesting a step right now. Help is available any time.",
                    )}
                  </p>
                  <div className="s-actions">
                    <LinkButton to="/help" variant="help" icon="help">
                      {w("Bantuan sekarang", "Help now")}
                    </LinkButton>
                  </div>
                </Notice>
              )}
              {result && result.aiState !== "crisis" && !result.proposal && (
                <Notice tone="info" title={w("Usulan tidak tersedia sekarang", "No suggestion right now")}>
                  <p>
                    {result.aiState === "stale"
                      ? w("Catatannya berubah saat usulan dibuat, jadi usulan itu dibuang.", "The note changed while the suggestion was made, so it was discarded.")
                      : w("Tidak ada usulan yang lolos pemeriksaan. Kamu tetap bisa menulis sendiri.", "No suggestion passed the checks. You can still write your own.")}
                  </p>
                  <div className="s-actions">
                    <Button variant="primary" onClick={() => choose("write")}>
                      {w("Tulis sendiri", "Write my own")}
                    </Button>
                  </div>
                </Notice>
              )}
              {result?.proposal && (
                <article className="a-step" aria-labelledby="proposal-title">
                  <h3 id="proposal-title" className="a-step-meta">
                    {w("Usulan sementara dari Meta · belum disimpan", "Tentative suggestion from Meta · not saved")}
                  </h3>
                  <p className="a-step-text">{result.proposal.text}</p>
                  <details className="a-step-why">
                    <summary>{w("Kenapa ini?", "Why this?")}</summary>
                    <p>
                      {w("Dibuat dari ", "Made from ")}
                      <Link to={sourcePath(result.proposal.sourceRef.recordId, sourceById.get(result.proposal.sourceRef.recordId)?.kind)}>
                        {w("catatan yang kamu pilih", "the note you chose")}
                      </Link>
                      {w(" (versi ", " (version ")}
                      {result.proposal.sourceRef.version}
                      {"). "}
                      {w("Ini hanya usulan; kamu bebas mengubah atau menolaknya.", "It is only a suggestion; you are free to change or reject it.")}
                    </p>
                  </details>
                  <div className="s-actions">
                    <Button variant="primary" onClick={() => acceptProposal(result.proposal!)}>
                      {w("Pakai atau ubah", "Use or rewrite")}
                    </Button>
                    <Button variant="quiet" onClick={rejectProposal}>
                      {w("Tolak", "Reject")}
                    </Button>
                  </div>
                </article>
              )}
            </>
          )}
          {!result?.proposal && (
            <Button variant="quiet" onClick={() => choose("start")}>
              {w("Kembali", "Back")}
            </Button>
          )}
        </section>
      )}

      <section aria-labelledby="steps-list">
        <h2 id="steps-list">{w("Langkah yang kamu pilih", "Steps you chose")}</h2>
        {loadError ? (
          <ErrorNotice error={loadError} onRetry={load} />
        ) : steps === null ? (
          <Loading label={w("Memuat langkahmu…", "Loading your steps…")} />
        ) : steps.length === 0 ? (
          <EmptyState
            title={w("Belum ada langkah", "No steps yet")}
            body={w("Tidak apa-apa. Menulis dan didengarkan saja juga cukup.", "That's fine. Writing and being heard is enough too.")}
          />
        ) : (
          <ul className="a-steps-list">
            {steps.map((step) => (
              <li key={step.id} className={"a-step" + (step.status === "needs_review" ? " a-step--review" : "")}>
                <p className="a-step-meta">
                  <span className="a-step-status">{statusLabel(step.status)}</span>
                  <span>{originLabel(step)}</span>
                  {step.dueDate && (
                    <span>
                      {w("Rencana: ", "Planned: ")}
                      {loc.date(step.dueDate + "T12:00:00")}
                    </span>
                  )}
                  <span>{w("Pribadi", "Private")}</span>
                </p>
                {editing?.id === step.id ? (
                  <form
                    onSubmit={(e) => {
                      e.preventDefault();
                      if (editing.text.trim()) void patch(step, { text: editing.text });
                    }}
                  >
                    <TextArea
                      label={w("Ubah langkah", "Edit step")}
                      value={editing.text}
                      onChange={(e) => setEditing({ id: step.id, text: e.target.value })}
                      maxLength={500}
                      rows={3}
                    />
                    <div className="s-actions">
                      <Button type="submit" variant="primary" disabled={cardBusy === step.id || !editing.text.trim()}>
                        {w("Simpan perubahan", "Save changes")}
                      </Button>
                      <Button variant="quiet" onClick={() => setEditing(null)}>
                        {w("Batal", "Cancel")}
                      </Button>
                    </div>
                  </form>
                ) : (
                  <p className="a-step-text">{step.text}</p>
                )}
                {step.safety === "crisis" && (
                  <Notice tone="help" title={w("Bantuan ada kapan saja", "Help is available any time")}>
                    <div className="s-actions">
                      <LinkButton to="/help" variant="help" icon="help">
                        {w("Bantuan sekarang", "Help now")}
                      </LinkButton>
                    </div>
                  </Notice>
                )}
                {step.status === "needs_review" && (
                  <Notice tone="warning" title={w("Catatan sumbernya berubah", "Its source note changed")}>
                    <p>
                      {step.review.staleReason === "source_deleted"
                        ? w("Catatan yang dikaitkan sudah dihapus. Kata-katamu tetap di sini; pilih apa yang ingin kamu lakukan.", "The linked note was deleted. Your words stay here; choose what you want to do.")
                        : w("Catatan yang dikaitkan sudah diubah. Kata-katamu tetap di sini; tinjau apakah langkah ini masih cocok.", "The linked note was edited. Your words stay here; check whether this step still fits.")}
                    </p>
                    <div className="s-actions">
                      {step.review.staleReason !== "source_deleted" && step.sourceRefs.some((r) => r.kind) && (
                        <Button variant="primary" disabled={cardBusy === step.id} onClick={() => void patch(step, { reaffirm: true })}>
                          {w("Tetap pakai dengan versi baru", "Keep it with the new version")}
                        </Button>
                      )}
                      <Button variant="secondary" disabled={cardBusy === step.id} onClick={() => void patch(step, { sourceRefs: [] })}>
                        {w("Lepas dari catatan", "Unlink from the note")}
                      </Button>
                      <Button variant="quiet" disabled={cardBusy === step.id} onClick={() => void patch(step, { status: "discarded" })}>
                        {w("Tidak cocok", "Not for me")}
                      </Button>
                    </div>
                  </Notice>
                )}
                {(step.sourceRefs.length > 0 || step.origin.kind === "meta_proposal") && (
                  <details className="a-step-why">
                    <summary>{w("Kenapa ini?", "Why this?")}</summary>
                    <ul>
                      {step.origin.kind === "meta_proposal" && (
                        <li>
                          {w("Berawal dari usulan Meta", "Started as a Meta suggestion")}
                          {step.origin.model ? ` (${step.origin.model})` : ""}
                          {w("; kamu yang memilih menyimpannya.", "; you chose to save it.")}
                        </li>
                      )}
                      {step.sourceRefs.map((r) => (
                        <li key={r.recordId}>
                          {r.kind ? (
                            <Link to={sourcePath(r.recordId, r.kind)}>{w("Catatan sumber", "Source note")}</Link>
                          ) : (
                            <span>{w("Catatan sumber (sudah dihapus)", "Source note (deleted)")}</span>
                          )}
                          {w(" · versi ", " · version ")}
                          {r.version}
                          {r.kind && !r.current ? w(" · sudah ada versi lebih baru", " · a newer version exists") : ""}
                        </li>
                      ))}
                    </ul>
                  </details>
                )}
                {cardError?.id === step.id && <ErrorNotice error={cardError.error} />}
                {editing?.id !== step.id && (
                  <div className="s-actions">
                    {step.status === "chosen" && (
                      <>
                        <Button variant="secondary" disabled={cardBusy === step.id} onClick={() => void patch(step, { status: "tried" })}>
                          {w("Sudah kucoba", "I tried it")}
                        </Button>
                        <Button variant="secondary" disabled={cardBusy === step.id} onClick={() => void patch(step, { status: "paused" })}>
                          {w("Nanti dulu", "Not now")}
                        </Button>
                        <Button variant="quiet" disabled={cardBusy === step.id} onClick={() => void patch(step, { status: "discarded" })}>
                          {w("Tidak cocok", "Not for me")}
                        </Button>
                      </>
                    )}
                    {(step.status === "tried" || step.status === "paused") && (
                      <>
                        <Button variant="secondary" disabled={cardBusy === step.id} onClick={() => void patch(step, { status: "chosen" })}>
                          {step.status === "tried" ? w("Buka lagi", "Reopen") : w("Lanjutkan", "Resume")}
                        </Button>
                        <Button variant="quiet" disabled={cardBusy === step.id} onClick={() => void patch(step, { status: "discarded" })}>
                          {w("Tidak cocok", "Not for me")}
                        </Button>
                      </>
                    )}
                    {step.status === "discarded" && (
                      <Button variant="secondary" disabled={cardBusy === step.id} onClick={() => void patch(step, { status: "chosen" })}>
                        {w("Pakai lagi", "Use it again")}
                      </Button>
                    )}
                    {step.status !== "discarded" && step.status !== "needs_review" && (
                      <Button variant="quiet" onClick={() => setEditing({ id: step.id, text: step.text })}>
                        {w("Ubah kata-kata", "Edit wording")}
                      </Button>
                    )}
                    <InlineConfirm
                      label={w("Hapus", "Delete")}
                      question={w("Hapus langkah ini dan semua versinya?", "Delete this step and every version?")}
                      confirmLabel={w("Hapus langkah", "Delete step")}
                      onConfirm={() => void remove(step)}
                      busy={cardBusy === step.id}
                    />
                  </div>
                )}
              </li>
            ))}
          </ul>
        )}
      </section>
      {user.kind === "guest" && (
        <p className="s-hint">
          {w(
            "Mode tamu: langkahmu ikut terhapus bersama data tamu.",
            "Guest mode: your steps are deleted together with your guest data.",
          )}
        </p>
      )}
    </div>
  );
}
