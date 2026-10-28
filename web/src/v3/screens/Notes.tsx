import { useEffect, useMemo, useState, type FormEvent } from "react";
import { Link, useRouter } from "../../lib/router";
import { useI18n } from "../../i18n";
import {
  api,
  localDay,
  type Entry,
  type Profile,
} from "../api";
import { usePrefs } from "../prefs";
import { useAnnounce, useLocale, useWords } from "../shared";
import { Icon } from "../icons";
import { MemoryPage, entryRoute } from "./Continuity";
import { WeeklyPage } from "./Weekly";
import { helpData } from "../helpdata";
import {
  Button,
  CheckboxRow,
  ChoiceGroup,
  EmptyState,
  ErrorNotice,
  InlineConfirm,
  LinkButton,
  Loading,
  PageHeader,
  ProvenanceBadge,
  ReviewStatusChip,
  StatusLine,
  Subnav,
  TextArea,
  TextField,
  buttonClass,
} from "../ui";

const moods = [
  { value: "very_heavy", dots: 1, id: "Berat", en: "Heavy" },
  { value: "heavy", dots: 2, id: "Kurang baik", en: "Not great" },
  { value: "okay", dots: 3, id: "Biasa saja", en: "Okay" },
  { value: "good", dots: 4, id: "Cukup baik", en: "Fairly good" },
  { value: "very_good", dots: 5, id: "Baik", en: "Good" },
  { value: "mixed", dots: 0, id: "Campur aduk / tidak yakin", en: "Mixed / not sure" },
] as const;

const energies = [
  { value: "low", id: "Rendah", en: "Low" },
  { value: "medium", id: "Sedang", en: "Medium" },
  { value: "high", id: "Tinggi", en: "High" },
] as const;

const tags = [
  { value: "sleep", id: "Tidur", en: "Sleep" },
  { value: "school", id: "Sekolah/kuliah", en: "School/study" },
  { value: "work", id: "Pekerjaan", en: "Work" },
  { value: "family", id: "Keluarga", en: "Family" },
  { value: "friends", id: "Teman", en: "Friends" },
  { value: "partner", id: "Pasangan", en: "Partner" },
  { value: "money", id: "Uang", en: "Money" },
  { value: "health", id: "Kesehatan", en: "Health" },
  { value: "loneliness", id: "Kesepian", en: "Loneliness" },
  { value: "other", id: "Lainnya", en: "Other" },
] as const;

export function moodLabel(value: string, lang: string): string {
  const m = moods.find((x) => x.value === value);
  return m ? (lang === "en" ? m.en : m.id) : value;
}

function useLeaveWarning(dirty: boolean) {
  useEffect(() => {
    if (!dirty) return;
    const onBefore = (e: BeforeUnloadEvent) => {
      e.preventDefault();
      e.returnValue = "";
    };
    window.addEventListener("beforeunload", onBefore);
    return () => window.removeEventListener("beforeunload", onBefore);
  }, [dirty]);
}

export function NotesPage({ user, path }: { user: Profile; path: string }) {
  if (path === "/notes" || path === "/notes/") return <Timeline user={user} />;
  if (path === "/notes/write") return <NoteEditor user={user} />;
  if (path === "/notes/check-in") return <CheckIn />;
  if (path === "/notes/patterns") return <Patterns />;
  if (path === "/notes/weekly") return <WeeklyPage />;
  if (path === "/notes/memory") return <MemoryPage />;
  const id = path.startsWith("/notes/") ? path.slice("/notes/".length) : "";
  if (id) return <EntryDetail id={id} />;
  return <Timeline user={user} />;
}

function useRecords() {
  const [records, setRecords] = useState<Entry[] | null>(null);
  const [error, setError] = useState<unknown>(null);
  const load = () => {
    setError(null);
    api<{ records: Entry[] }>("/records")
      .then((d) => setRecords(d.records))
      .catch((e) => {
        setError(e);
        setRecords([]);
      });
  };
  useEffect(load, []);
  return { records, error, load, setRecords };
}

function Timeline({ user }: { user: Profile }) {
  const w = useWords();
  const loc = useLocale();
  const { lang } = useI18n();
  const { query, navigate } = useRouter();
  const { prefs } = usePrefs();
  const { records, error, load } = useRecords();
  const filters = (query.get("types") ?? "").split(",").filter(Boolean);
  const visible = (records ?? []).filter((r) => filters.length === 0 || filters.includes(r.kind));

  const grouped = useMemo(() => {
    const map = new Map<string, Entry[]>();
    for (const r of visible) {
      const key = localDay(new Date(r.updatedAt));
      const list = map.get(key) ?? [];
      list.push(r);
      map.set(key, list);
    }
    return [...map.entries()];
  }, [visible]);

  const toggle = (kind: string) => {
    const next = filters.includes(kind)
      ? filters.filter((f) => f !== kind)
      : [...filters, kind];
    navigate("/notes" + (next.length ? "?types=" + next.join(",") : ""), { replace: true });
  };

  return (
    <>
      <PageHeader
        title={w("Catatan pribadiku", "My personal notes")}
        lead={w(
          "Semua yang kamu simpan, dari yang terbaru. Hanya kamu yang bisa melihatnya.",
          "Everything you've saved, newest first. Only you can see it.",
        )}
        actions={
          <>
            <LinkButton to="/notes/write" variant="primary" icon="notes">
              {w("Tulis catatan", "Write a note")}
            </LinkButton>
            <LinkButton to="/notes/check-in" variant="secondary" icon="interpretation">
              {w("Cek perasaan", "Check in")}
            </LinkButton>
          </>
        }
      />
      <Subnav
        label={w("Bagian Catatanku", "My notes sections")}
        current="/notes"
        items={[
          { to: "/notes", label: w("Linimasa", "Timeline") },
          { to: "/notes/weekly", label: w("Refleksi mingguan", "Weekly reflection") },
          { to: "/notes/memory", label: w("Yang Sela ingat", "What Sela remembers") },
        ]}
      />
      <div className="s-filters" role="group" aria-labelledby="filters-label">
        <span className="s-filters-label" id="filters-label">{w("Tampilkan saja", "Show only")}</span>
        {[
          { kind: "conversation", id: "Percakapan", en: "Conversations" },
          { kind: "note", id: "Catatan", en: "Notes" },
          { kind: "checkin", id: "Cek perasaan", en: "Check-ins" },
        ].map((f) => (
          <label key={f.kind} className="s-filter">
            <input
              type="checkbox"
              checked={filters.includes(f.kind)}
              onChange={() => toggle(f.kind)}
            />
            {filters.includes(f.kind) && <Icon name="check" size={16} />}
            <span>{w(f.id, f.en)}</span>
          </label>
        ))}
        {filters.length > 0 && (
          <button type="button" className={buttonClass("quiet")} onClick={() => navigate("/notes", { replace: true })}>
            {w("Hapus filter", "Clear filters")}
          </button>
        )}
      </div>
      <ErrorNotice error={error} onRetry={load} />
      {records === null ? (
        <Loading label={w("Memuat yang tersimpan…", "Loading what you've saved…")} />
      ) : visible.length === 0 ? (
        records.length === 0 ? (
          <EmptyState
            title={w("Belum ada yang tersimpan.", "Nothing saved yet.")}
            body={w(
              "Yang kamu simpan akan muncul di sini, dari yang terbaru.",
              "What you save will appear here, newest first.",
            )}
            actions={
              <>
                <LinkButton to="/talk/new" variant="secondary">
                  {w("Mulai bercerita", "Start talking")}
                </LinkButton>
                <LinkButton to="/notes/write" variant="secondary">
                  {w("Tulis catatan", "Write a note")}
                </LinkButton>
              </>
            }
            mark
          />
        ) : (
          <EmptyState
            title={w("Tidak ada entri untuk pilihan ini.", "No entries for this choice.")}
            actions={
              <Button variant="secondary" onClick={() => navigate("/notes", { replace: true })}>
                {w("Hapus filter", "Clear filters")}
              </Button>
            }
          />
        )
      ) : (
        grouped.map(([day, entries]) => (
          <section className="s-timeline-day" key={day} aria-labelledby={"day-" + day}>
            <h2 id={"day-" + day}>{loc.dayHeading(entries[0]!.updatedAt)}</h2>
            <ol className="s-timeline">
              {entries.map((r) => (
                <li key={r.id}>
                  <Link
                    to={entryRoute(r)}
                  >
                    <Icon
                      name={
                        r.kind === "conversation"
                          ? "talk"
                          : r.kind === "checkin"
                            ? "interpretation"
                            : "notes"
                      }
                    />
                    <span>
                      <span className="s-timeline-type">
                        {r.kind === "conversation"
                          ? w("Percakapan", "Conversation")
                          : r.kind === "checkin"
                            ? w("Cek perasaan", "Check-in")
                            : r.kind === "summary" ? w("Ringkasan", "Summary")
                            : r.kind === "memory" ? w("Memori", "Memory")
                            : r.kind === "followup" ? w("Tindak lanjut", "Follow-up")
                            : w("Catatan", "Note")}
                        {r.kind === "checkin" && r.mood ? " · " + moodLabel(r.mood, lang) : ""}
                      </span>
                      {!prefs.hidePreviews && (
                        <span className="s-timeline-preview" style={{ display: "block" }}>
                          {r.kind === "conversation"
                            ? r.title || w("Percakapan tersimpan", "Saved conversation")
                            : r.text.slice(0, 120) || w("Tanpa teks", "No text")}
                        </span>
                      )}
                    </span>
                    <time className="s-timeline-time" dateTime={r.updatedAt}>
                      {loc.time(r.updatedAt)}
                    </time>
                  </Link>
                </li>
              ))}
            </ol>
          </section>
        ))
      )}
      <div className="s-actions" style={{ marginTop: "var(--space-5)" }}>
        <LinkButton to="/notes/patterns" variant="quiet">
          {w("Lihat pola cek perasaan", "See patterns in your check-ins")}
        </LinkButton>
      </div>
      <p className="s-hint" style={{ marginTop: "var(--space-2)" }}>
        {w(
          "Pilih catatan yang ingin kamu bawa, susun ringkasan, lalu periksa sebelum mengunduhnya.",
          "Choose the notes you want to bring, build a summary, then review it before downloading.",
        )}
        {" "}<Link to="/support/summaries">{w("Buka ringkasan", "Open summaries")}</Link>
      </p>
      {user.kind === "guest" && user.guest && (
        <p className="s-retention">
          {w("Mode tamu · dihapus otomatis jika ", "Guest mode · deleted automatically after ")}
          {Math.floor(user.guest.inactiveTtlHours / 24)}
          {w(" hari tidak aktif, paling lambat ", " inactive days, at the latest ")}
          {loc.date(user.guest.maxAgePurgeAt)}
        </p>
      )}
    </>
  );
}

function NoteEditor({ user }: { user: Profile }) {
  const w = useWords();
  const { prefs } = usePrefs();
  const { navigate } = useRouter();
  const { announce } = useAnnounce();
  const [title, setTitle] = useState("");
  const [text, setText] = useState("");
  const [save, setSave] = useState(prefs.defaultStorage === "save");
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const [savedAt, setSavedAt] = useState("");
  useLeaveWarning(Boolean(text || title));

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    if (!save) {
      setError(w("Pilih dulu apakah catatan ini disimpan.", "Choose first whether this note is saved."));
      return;
    }
    setBusy(true);
    try {
      const record = await api<Entry>("/records", {
        kind: "note",
        title,
        text,
        mood: "",
        energy: "",
        tags: [],
        day: "",
        storageGranted: true,
        expectedVersion: 0,
      });
      announce("status", w("Catatan tersimpan.", "Note saved."));
      navigate("/notes/" + record.id);
    } catch (err) {
      setError(err);
      setSavedAt("");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="s-write">
      <PageHeader
        back={{ to: "/notes", label: w("Catatanku", "My notes") }}
        title={w("Tulis catatan", "Write a note")}
        lead={
          user.kind === "guest"
            ? w(
                "Catatan ini terikat ke browser ini. Siapa pun yang memakai browser ini bisa membukanya.",
                "This note is bound to this browser. Anyone using this browser can open it.",
              )
            : w(
                "Hanya kamu yang bisa membacanya.",
                "Only you can read it.",
              )
        }
      />
      <ErrorNotice error={error} />
      {savedAt && <StatusLine tone="success">{savedAt}</StatusLine>}
      <form className="s-editor s-editor--page" onSubmit={(e) => void submit(e)}>
        <div className="s-editor-main">
        <TextField
          label={w("Judul", "Title")}
          optional
          value={title}
          maxLength={120}
          onChange={(e) => setTitle(e.target.value)}
        />
        <TextArea
          label={w("Catatanmu", "Your note")}
          value={text}
          rows={8}
          maxLength={6000}
          required
          autoFocus
          onChange={(e) => setText(e.target.value)}
        />
        <CheckboxRow
          checked={save}
          onChange={setSave}
          hint={w(
            "Terenkripsi untukmu di server Sela. Kamu bisa menghapusnya kapan saja.",
            "Encrypted for you on Sela's server. You can delete it at any time.",
          )}
        >
          {w("Simpan catatan ini di Catatanku", "Save this note to My notes")}
        </CheckboxRow>
        <div className="s-actions">
          <Button type="submit" variant="primary" busy={busy} disabled={!save || !text.trim()}>
            {busy ? w("Menyimpan…", "Saving…") : w("Simpan catatan", "Save note")}
          </Button>
          <LinkButton to="/notes" variant="quiet">
            {w("Batal", "Cancel")}
          </LinkButton>
        </div>
        </div>
        <aside className="s-editor-aside" aria-labelledby="prompts-title">
          <h2 id="prompts-title">{w("Butuh pancingan?", "Need a prompt?")}</h2>
          <p className="s-hint">
            {w("Pilih satu; kalimatnya ditambahkan di awal tulisanmu.", "Pick one; it's added to the start of your writing.")}
          </p>
          <ul className="s-prompt-list">
            {([
              ["Apa yang paling terasa hari ini?", "What feels strongest today?"],
              ["Apa yang ingin kamu ingat?", "What do you want to remember?"],
              ["Apa yang membantu, sekecil apa pun?", "What helped, however small?"],
            ] as const).map(([id, en]) => (
              <li key={id}>
                <button
                  type="button"
                  className="s-prompt"
                  onClick={() => setText((t) => w(id, en) + "\n" + t)}
                >
                  <Icon name="plus" size={16} />
                  <span>{w(id, en)}</span>
                </button>
              </li>
            ))}
          </ul>
        </aside>
      </form>
    </div>
  );
}

function CheckIn() {
  const w = useWords();
  const { lang } = useI18n();
  const { prefs } = usePrefs();
  const { navigate } = useRouter();
  const { announce } = useAnnounce();
  const { records } = useRecords();
  const [mood, setMood] = useState<string | null>(null);
  const [energy, setEnergy] = useState<string | null>(null);
  const [selectedTags, setSelectedTags] = useState<string[]>([]);
  const [text, setText] = useState("");
  const [save, setSave] = useState(prefs.defaultStorage === "save");
  const [groupError, setGroupError] = useState("");
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  useLeaveWarning(Boolean(text || mood));
  const today = localDay();
  const existing = records?.find((r) => r.kind === "checkin" && r.day === today);

  if (existing) {
    return (
      <div className="s-narrow">
        <PageHeader
          back={{ to: "/notes", label: w("Catatanku", "My notes") }}
          title={w("Cek perasaan", "Check-in")}
        />
        <EmptyState
          title={w("Kamu sudah mencatat cek perasaan hari ini.", "You've already checked in today.")}
          body={w(
            "Satu cek perasaan per hari sudah cukup. Kamu bisa membuka yang tadi atau menulis catatan.",
            "One check-in a day is enough. You can open the earlier one or write a note.",
          )}
          actions={
            <>
              <LinkButton to={"/notes/" + existing.id} variant="primary">
                {w("Lihat cek perasaan", "See the check-in")}
              </LinkButton>
              <LinkButton to="/notes/write" variant="secondary">
                {w("Tulis catatan", "Write a note")}
              </LinkButton>
            </>
          }
        />
      </div>
    );
  }

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    if (!mood) {
      setGroupError(w("Pilih salah satu.", "Choose one."));
      return;
    }
    if (!save) {
      setError(w("Pilih dulu apakah cek perasaan ini disimpan.", "Choose first whether this check-in is saved."));
      return;
    }
    setBusy(true);
    try {
      const record = await api<Entry>("/records", {
        kind: "checkin",
        title: "",
        text,
        mood,
        energy: energy ?? "",
        tags: selectedTags,
        day: today,
        storageGranted: true,
        expectedVersion: 0,
      });
      announce("status", w("Tersimpan di Catatanku.", "Saved to My notes."));
      navigate("/notes/" + record.id);
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="s-narrow">
      <PageHeader
        back={{ to: "/notes", label: w("Catatanku", "My notes") }}
        title={w("Cek perasaan", "Check-in")}
        lead={w(
          "Tidak ada jawaban benar. Kamu boleh melewati pilihan apa pun.",
          "There is no right answer. You can skip any choice.",
        )}
      />
      <ErrorNotice error={error} />
      <form className="s-editor s-editor--sheet" onSubmit={(e) => void submit(e)}>
        <ChoiceGroup
          legend={w("Bagaimana perasaanmu sekarang?", "How are you feeling right now?")}
          name="mood"
          value={mood}
          onChange={setMood}
          error={groupError}
          variant="tiles"
          columns={2}
          options={moods.map((m) => ({
            value: m.value,
            label:
              (lang === "en" ? m.en : m.id) +
              (m.dots
                ? ""
                : ""),
          }))}
        />
        <ChoiceGroup
          legend={w("Tenaga", "Energy")}
          name="energy"
          value={energy}
          onChange={setEnergy}
          optional
          variant="chips"
          options={energies.map((e) => ({ value: e.value, label: w(e.id, e.en) }))}
        />
        <fieldset className="s-fieldset">
          <legend className="s-legend">
            {w("Yang sedang memengaruhi", "What's affecting you")}{" "}
            <span className="s-optional">{w("(opsional)", "(optional)")}</span>
          </legend>
          <div className="s-tag-grid">
            {tags.map((t) => {
              const checked = selectedTags.includes(t.value);
              return (
                <label key={t.value} className={"s-choice s-choice--chips" + (checked ? " is-selected" : "")}>
                  <input
                    type="checkbox"
                    checked={checked}
                    onChange={() =>
                      setSelectedTags((current) =>
                        current.includes(t.value)
                          ? current.filter((x) => x !== t.value)
                          : [...current, t.value],
                      )
                    }
                  />
                  {checked && <Icon name="check" size={16} />}
                  <span>{w(t.id, t.en)}</span>
                </label>
              );
            })}
          </div>
        </fieldset>
        <TextArea
          label={w("Catatan singkat", "Short note")}
          optional
          rows={4}
          maxLength={500}
          value={text}
          onChange={(e) => setText(e.target.value)}
        />
        <CheckboxRow
          checked={save}
          onChange={setSave}
          hint={w(
            "Terenkripsi untukmu di server Sela.",
            "Encrypted for you on Sela's server.",
          )}
        >
          {w("Simpan cek perasaan ini di Catatanku", "Save this check-in to My notes")}
        </CheckboxRow>
        <div className="s-actions">
          <Button type="submit" variant="primary" busy={busy} disabled={!save}>
            {busy ? w("Menyimpan…", "Saving…") : w("Simpan", "Save")}
          </Button>
          <LinkButton to="/notes" variant="quiet">
            {w("Batal", "Cancel")}
          </LinkButton>
        </div>
      </form>
    </div>
  );
}

function EntryDetail({ id }: { id: string }) {
  const w = useWords();
  const { lang } = useI18n();
  const loc = useLocale();
  const { navigate } = useRouter();
  const { announce } = useAnnounce();
  const [record, setRecord] = useState<Entry | null>(null);
  const [versions, setVersions] = useState<Entry[] | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [editing, setEditing] = useState(false);
  const [title, setTitle] = useState("");
  const [text, setText] = useState("");
  const [busy, setBusy] = useState(false);

  const load = () => {
    setError(null);
    api<Entry>("/records/" + id)
      .then((r) => {
        setRecord(r);
        setTitle(r.title);
        setText(r.text);
      })
      .catch(setError);
  };
  useEffect(load, [id]);

  const saveEdit = async () => {
    if (!record) return;
    setBusy(true);
    setError(null);
    try {
      const next = await api<Entry>(
        "/records/" + record.id,
        {
          kind: record.kind,
          title,
          text,
          mood: record.mood ?? "",
          energy: record.energy ?? "",
          tags: record.tags ?? [],
          day: record.day ?? "",
          storageGranted: true,
          expectedVersion: record.version,
        },
        "PATCH",
      );
      setRecord(next);
      setEditing(false);
      announce("status", w("Perubahan tersimpan sebagai versi baru.", "Saved as a new version."));
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  };

  const remove = async () => {
    if (!record) return;
    setBusy(true);
    try {
      await api("/records/" + record.id, { expectedVersion: record.version }, "DELETE");
      announce("status", w("Entri dihapus.", "Entry deleted."));
      navigate("/notes");
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  };

  const loadVersions = async () => {
    try {
      const d = await api<{ versions: Entry[] }>("/records/" + id + "/versions");
      setVersions(d.versions);
    } catch (e) {
      setError(e);
    }
  };

  if (!record) {
    return (
      <>
        <PageHeader
          back={{ to: "/notes", label: w("Catatanku", "My notes") }}
          title={w("Catatan", "Note")}
        />
        <ErrorNotice error={error} onRetry={load} />
        {!error && <Loading label={w("Memuat catatan…", "Loading the note…")} />}
      </>
    );
  }

  const isCheckin = record.kind === "checkin";
  const tagsList = (record.tags ?? []).map((t) => {
    const found = tags.find((x) => x.value === t);
    return found ? w(found.id, found.en) : t;
  });
  const energyLabel = record.energy ? energies.find((e) => e.value === record.energy) : null;

  return (
    <div className="s-narrow s-narrow--start">
      <PageHeader
        back={{ to: "/notes", label: w("Catatanku", "My notes") }}
        title={
          isCheckin
            ? w("Cek perasaan", "Check-in")
            : record.title || w("Catatan", "Note")
        }
        lead={loc.dateTime(record.updatedAt)}
      />
      <ErrorNotice error={error} />
      <article className="s-record">
        <div className="s-record-meta">
          <ProvenanceBadge value={record.provenance} />
          <ReviewStatusChip value={record.reviewStatus} />
          <span>{w("versi ", "version ")}{record.version}</span>
        </div>
        {isCheckin && (
          <dl className="s-dl">
            <div>
              <dt>{w("Perasaan", "Feeling")}</dt>
              <dd>{moodLabel(record.mood ?? "", lang)}</dd>
            </div>
            {energyLabel && (
              <div>
                <dt>{w("Tenaga", "Energy")}</dt>
                <dd>{w(energyLabel.id, energyLabel.en)}</dd>
              </div>
            )}
            {tagsList.length > 0 && (
              <div>
                <dt>{w("Yang memengaruhi", "What's affecting you")}</dt>
                <dd>{tagsList.join(", ")}</dd>
              </div>
            )}
          </dl>
        )}
        {editing ? (
          <div className="s-editor">
            {!isCheckin && (
              <TextField
                label={w("Judul", "Title")}
                optional
                value={title}
                maxLength={120}
                onChange={(e) => setTitle(e.target.value)}
              />
            )}
            <TextArea
              label={w("Tulisanmu", "Your words")}
              value={text}
              rows={10}
              maxLength={6000}
              onChange={(e) => setText(e.target.value)}
            />
            <div className="s-actions">
              <Button variant="primary" busy={busy} onClick={() => void saveEdit()}>
                {w("Simpan perubahan", "Save changes")}
              </Button>
              <Button variant="quiet" onClick={() => setEditing(false)}>
                {w("Batal", "Cancel")}
              </Button>
            </div>
          </div>
        ) : (
          <p className="s-preserve s-reading-body">{record.text}</p>
        )}
        {record.safety === "crisis" && <CrisisAfterSave />}
      </article>
      <div className="s-actions">
        {!isCheckin && !editing && (
          <Button variant="secondary" onClick={() => setEditing(true)}>
            {w("Ubah", "Edit")}
          </Button>
        )}
        {!isCheckin && (
          <Button variant="quiet" onClick={() => void loadVersions()}>
            {w("Lihat versi sebelumnya", "See earlier versions")}
          </Button>
        )}
        {isCheckin && (
          <LinkButton to="/notes/check-in" variant="quiet">
            {w("Cek perasaan lagi", "Check in again")}
          </LinkButton>
        )}
        <InlineConfirm
          label={isCheckin ? w("Hapus cek perasaan ini", "Delete this check-in") : w("Hapus catatan", "Delete note")}
          question={
            isCheckin
              ? w("Hapus cek perasaan ini?", "Delete this check-in?")
              : w("Hapus catatan ini? Semua versinya ikut terhapus.", "Delete this note? Every version goes with it.")
          }
          confirmLabel={w("Ya, hapus", "Yes, delete")}
          onConfirm={() => void remove()}
          busy={busy}
        />
      </div>
      {!isCheckin && (
        <p className="s-hint" style={{ marginTop: "var(--space-3)" }}>
          {w(
            "Kamu bisa memilih catatan ini saat menyusun ringkasan untuk konsultasi.",
            "You can choose this note when building a summary for a consultation.",
          )}
          {" "}<Link to="/support/summaries">{w("Buka ringkasan", "Open summaries")}</Link>
        </p>
      )}
      {versions && versions.length > 0 && (
        <section className="s-section">
          <h2>{w("Riwayat versi", "Version history")}</h2>
          {versions.map((v) => (
            <article className="s-history" key={v.version}>
              <p className="s-label">
                {w("VERSI ", "VERSION ")}
                {v.version} · {loc.dateTime(v.updatedAt)}
              </p>
              <p className="s-preserve">{v.text}</p>
            </article>
          ))}
        </section>
      )}
    </div>
  );
}

function CrisisAfterSave() {
  const w = useWords();
  const service = helpData.help[0];
  return (
    <section className="s-crisis-panel" aria-labelledby="crisis-save-title">
      <h2 id="crisis-save-title">{w("Tersimpan. Kamu tidak sendirian.", "Saved. You're not alone.")}</h2>
      <p>
        {w(
          "Karena ada kata-kata dalam tulisanmu yang terdengar sangat berat, Sela menunjukkan bantuan di bawah ini. Kamu tidak harus memakainya.",
          "Because some of your words sounded very heavy, Sela is showing help below. You don't have to use it.",
        )}
      </p>
      {service && (
        <p>
          <strong>{service.name}</strong> ·{" "}
          {document.documentElement.lang === "en"
            ? service.description.en
            : service.description.id}
        </p>
      )}
      <div className="s-actions">
        <LinkButton to="/help" variant="help" icon="help">
          {w("Lihat semua bantuan", "See all help")}
        </LinkButton>
      </div>
    </section>
  );
}

function Patterns() {
  const w = useWords();
  const { lang } = useI18n();
  const { announce } = useAnnounce();
  const [enabled, setEnabled] = useState(() => {
    try {
      return window.localStorage.getItem("sela.patterns") === "1";
    } catch {
      return false;
    }
  });
  const [days, setDays] = useState<14 | 30>(14);
  const { records, error, load } = useRecords();
  const checkins = (records ?? []).filter((r) => r.kind === "checkin");
  const since = Date.now() - days * 86_400_000;
  const windowed = checkins.filter((r) => Date.parse(r.updatedAt) >= since);
  const counts = moods.map((m) => ({
    ...m,
    count: windowed.filter((r) => r.mood === m.value).length,
  }));
  const heavy = windowed.filter((r) => ["very_heavy", "heavy"].includes(r.mood ?? ""));
  const tagCounts = tags
    .map((t) => ({ ...t, count: heavy.filter((r) => (r.tags ?? []).includes(t.value)).length }))
    .filter((t) => t.count > 0)
    .sort((a, b) => b.count - a.count);

  const enable = () => {
    try {
      window.localStorage.setItem("sela.patterns", "1");
    } catch {
      // the setting lasts for this tab
    }
    setEnabled(true);
    announce("status", w("Pola ditampilkan.", "Patterns are shown."));
  };

  return (
    <>
      <PageHeader
        back={{ to: "/notes", label: w("Catatanku", "My notes") }}
        title={w("Pola dari cek perasaanmu", "Patterns in your check-ins")}
        lead={w(
          "Ini hitungan sederhana, bukan penilaian.",
          "These are simple counts, not an assessment.",
        )}
      />
      <ErrorNotice error={error} onRetry={load} />
      {!enabled ? (
        <div className="s-notice s-notice--info">
          <h2 className="s-notice-title">{w("Tampilkan pola?", "Show patterns?")}</h2>
          <p>
            {w(
              "Pola dibuat dari cek perasaanmu sendiri dengan hitungan sederhana, tanpa AI. Kamu bisa mematikannya kapan saja.",
              "Patterns are made from your own check-ins with simple counts, without AI. You can turn them off at any time.",
            )}
          </p>
          <div className="s-actions">
            <Button variant="primary" onClick={enable}>
              {w("Tampilkan pola", "Show patterns")}
            </Button>
            <Button variant="quiet" onClick={() => setEnabled(false)}>
              {w("Tidak sekarang", "Not now")}
            </Button>
          </div>
        </div>
      ) : (
        <>
          <ChoiceGroup
            legend={w("Rentang waktu", "Time range")}
            name="pattern-days"
            value={String(days)}
            onChange={(v) => setDays(v === "30" ? 30 : 14)}
            variant="chips"
            options={[
              { value: "14", label: w("14 hari terakhir", "Last 14 days") },
              { value: "30", label: w("30 hari terakhir", "Last 30 days") },
            ]}
          />
          {windowed.length === 0 ? (
            <EmptyState
              title={w("Belum ada cek perasaan di rentang ini.", "No check-ins in this range.")}
              actions={
                <LinkButton to="/notes/check-in" variant="primary">
                  {w("Cek perasaan", "Check in")}
                </LinkButton>
              }
            />
          ) : (
            <section className="s-section" aria-labelledby="pattern-table-title">
              <h2 id="pattern-table-title">{w("Jumlah per perasaan", "Counts per feeling")}</h2>
              <table className="s-table">
                <caption className="sr-only">
                  {w(
                    "Jumlah cek perasaan dalam rentang yang dipilih.",
                    "Number of check-ins in the selected range.",
                  )}
                </caption>
                <thead>
                  <tr>
                    <th scope="col">{w("Perasaan", "Feeling")}</th>
                    <th scope="col">{w("Jumlah", "Count")}</th>
                  </tr>
                </thead>
                <tbody>
                  {counts.map((c) => (
                    <tr key={c.value}>
                      <th scope="row">{lang === "en" ? c.en : c.id}</th>
                      <td>{c.count}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
              <h2 className="s-section">{w("Yang sering muncul saat berat", "What often appears when it's heavy")}</h2>
              {tagCounts.length === 0 ? (
                <p className="s-muted">
                  {w("Belum ada tag yang cukup untuk ditampilkan.", "Not enough tags to show yet.")}
                </p>
              ) : (
                <ul>
                  {tagCounts.map((t) => (
                    <li key={t.value}>
                      {w(t.id, t.en)} · {t.count}
                    </li>
                  ))}
                </ul>
              )}
            </section>
          )}
          <Button
            variant="quiet"
            onClick={() => {
              try {
                window.localStorage.setItem("sela.patterns", "0");
              } catch {
                // no-op
              }
              setEnabled(false);
            }}
          >
            {w("Matikan pola", "Turn patterns off")}
          </Button>
        </>
      )}
    </>
  );
}
