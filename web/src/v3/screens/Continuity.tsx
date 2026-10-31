import { useEffect, useState, type FormEvent } from "react";
import { Link, useRouter } from "../../lib/router";
import { api, download as downloadFile, type Consent, type Entry } from "../api";
import { Icon } from "../icons";
import { useSession } from "../session";
import { useLocale, useWords } from "../shared";
import {
  Button,
  CheckboxRow,
  EmptyState,
  ErrorNotice,
  InlineConfirm,
  LinkButton,
  Loading,
  Notice,
  PageHeader,
  ProvenanceBadge,
  ReviewStatusChip,
  TextArea,
  TextField,
} from "../ui";

export type SourceRef = { recordId: string; version: number };
export type ContinuityRecord = Entry & {
  sourceRefs?: SourceRef[];
  summaryState?: "draft" | "approved";
  prepared?: { goal: string; questions: string; bring: string };
  parentId?: string;
};
export const sourceText = (r: Entry) =>
  r.kind === "conversation"
    ? (r.turns ?? [])
        .filter((t) => t.role === "user")
        .map((t) => t.text)
        .join("\n\n")
    : r.text;
export const entryRoute = (r: Entry) =>
  r.kind === "summary"
    ? `/support/summaries/${r.id}`
    : r.kind === "memory"
      ? `/notes/memory/${r.id}`
      : r.kind === "conversation"
        ? `/talk?record=${r.id}`
        : `/notes/${r.id}`;

function ContinuityHelp({ records }: { records: Entry[] }) {
  const w = useWords();
  if (!records.some((r) => r.safety === "crisis" || r.safety === "elevated"))
    return null;
  return (
    <Notice
      tone="warning"
      title={w("Bantuan tetap dekat", "Support is close by")}
    >
      <p>
        {w(
          "Tulisanmu tetap disimpan sesuai pilihanmu. Kalau kamu merasa tidak aman sekarang, kamu bisa membuka layanan bantuan. Sela tidak menghubungi siapa pun.",
          "Your writing is kept according to your choice. If you feel unsafe now, you can open support services. Sela contacts no one.",
        )}
      </p>
      <LinkButton to="/help" variant="help" icon="help">
        {w("Butuh bantuan sekarang", "Need help now")}
      </LinkButton>
    </Notice>
  );
}

export function MemoryPage() {
  const w = useWords();
  const loc = useLocale();
  const { policies } = useSession();
  const [data, setData] = useState<{
    enabled: boolean;
    items: ContinuityRecord[];
  } | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const [title, setTitle] = useState("");
  const [text, setText] = useState("");
  const [allow, setAllow] = useState(false);
  const load = () => {
    setError(null);
    api<{ enabled: boolean; items: ContinuityRecord[] }>("/memory")
      .then(setData)
      .catch(setError);
  };
  useEffect(load, []);
  const consent = async (granted: boolean) => {
    setBusy(true);
    setError(null);
    try {
      const { consents } = await api<{ consents: Consent[] }>("/me/consents");
      const c = consents.find((c) => c.purpose === "personal_memory");
      const version =
        c?.currentPolicyVersion ?? policies.personal_memory?.version;
      if (!version)
        throw new Error(
          w(
            "Kebijakan memori belum dapat dimuat. Coba lagi.",
            "Memory policy could not be loaded. Try again.",
          ),
        );
      await api(
        "/me/consents/personal_memory",
        { granted, policyVersion: version },
        "PUT",
      );
      setAllow(false);
      setTitle("");
      setText("");
      load();
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  };
  const save = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await api(
        "/memory",
        { title, text, storageGranted: true },
        "POST",
      );
      setTitle("");
      setText("");
      load();
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  };
  return (
    <div className="s-narrow c-memory">
      <PageHeader
        back={{ to: "/notes", label: w("Catatanku", "My notes") }}
        title={w("Yang Sela ingat", "What Sela remembers")}
        lead={w(
          "Konteks yang kamu pilih sendiri. Bisa diperbaiki atau dihapus kapan saja.",
          "Context you choose yourself. Edit or delete it at any time.",
        )}
      />
      <ErrorNotice error={error} onRetry={load} />
      {!data && !error && <Loading label={w("Membuka…", "Opening…")} />}
      {data && !data.enabled && (
        <section className="c-consent">
          <h2>{w("Mulai dengan izinmu", "Start with your permission")}</h2>
          <p>
            {w(
              "Memori mati secara default. Jika diaktifkan, hanya hal yang kamu tulis di halaman ini yang disimpan, terenkripsi di server Sela. Tidak ada pengambilan cerita otomatis.",
              "Memory is off by default. When enabled, only what you write here is saved, encrypted on Sela's server. Stories are not collected automatically.",
            )}
          </p>
          <details className="c-policy-details">
            <summary>
              {w("Baca kebijakan memori", "Read the memory policy")}
            </summary>
            {policies.personal_memory?.body.map((p, i) => (
              <p className="s-small s-muted" key={i}>
                {w(p.id, p.en)}
              </p>
            ))}
          </details>
          <CheckboxRow checked={allow} onChange={setAllow}>
            {w(
              "Aku mengizinkan penyimpanan memori pilihanku",
              "I allow my chosen memories to be stored",
            )}
          </CheckboxRow>
          <Button
            variant="primary"
            disabled={!allow || busy}
            onClick={() => void consent(true)}
          >
            {w("Aktifkan memori", "Enable memory")}
          </Button>
        </section>
      )}
      {data?.enabled && (
        <>
          <div className="c-state-row">
            <span>
              {w(
                "Aktif · ditulis olehmu · dipakai membalas hanya saat izin AI aktif",
                "On · written by you · used for replies only while AI consent is on",
              )}
            </span>
            <Button
              variant="quiet"
              disabled={busy}
              onClick={() => void consent(false)}
            >
              {w("Matikan memori", "Turn memory off")}
            </Button>
          </div>
          <p className="s-hint">
            {w(
              "Saat izin balasan AI juga aktif, Sela dapat memakai memori yang sudah dikonfirmasi (paling banyak 5, dari yang terbaru) untuk membalas. Tanpa izin AI atau saat memori dimatikan, tidak ada memori yang dikirim.",
              "When AI reply consent is also on, Sela may use confirmed memory (at most 5, newest first) to reply. Without AI consent, or while memory is off, no memory is sent.",
            )}
          </p>
          <p className="s-hint">
            {w(
              "Mematikan menghentikan penggunaan memori; isi tersimpan tetap bisa dihapus setelah diaktifkan kembali atau lewat hapus semua data.",
              "Turning memory off stops its use; saved items can be deleted after re-enabling it, or by deleting all data.",
            )}
          </p>
          <form className="c-editor" onSubmit={save}>
            <h2>
              {w(
                "Apa yang ingin kamu simpan?",
                "What would you like to keep?",
              )}
            </h2>
            <TextField
              label={w("Judul", "Title")}
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              maxLength={120}
              optional
            />
            <TextArea
              label={w("Konteks pilihanmu", "Your chosen context")}
              value={text}
              onChange={(e) => setText(e.target.value)}
              required
              maxLength={6000}
              rows={4}
              hint={w(
                "Misalnya, cara kamu ingin didengarkan. Hindari detail identitas yang tidak diperlukan.",
                "For example, how you want to be heard. Avoid unnecessary identifying details.",
              )}
            />
            <div className="s-actions">
              <Button
                type="submit"
                variant="primary"
                disabled={busy || !text.trim()}
              >
                {w("Simpan konteks", "Save context")}
              </Button>
            </div>
          </form>
          <ContinuityHelp records={data.items} />
          <section
            className="c-document-list"
            aria-label={w("Memori tersimpan", "Saved memories")}
          >
            {data.items.length === 0 && (
              <EmptyState
                title={w("Belum ada konteks tersimpan", "No context saved yet")}
                body={w(
                  "Tidak ada profil tersembunyi. Semua yang disimpan akan terlihat di sini.",
                  "There is no hidden profile. Everything saved will appear here.",
                )}
              />
            )}
            {data.items.map((r) => (
              <Link
                key={r.id}
                to={`/notes/memory/${r.id}`}
                className="c-document-link"
              >
                <div>
                  <h2>{r.title || w("Konteks pribadi", "Personal context")}</h2>
                  <p className="s-muted">
                    {r.reviewStatus === "corrected"
                      ? w("Dikoreksi olehmu", "Corrected by you")
                      : w("Dikonfirmasi olehmu", "Confirmed by you")}{" "}
                    · {loc.shortDate(r.updatedAt)}
                  </p>
                </div>
                <Icon name="arrow-right" />
              </Link>
            ))}
          </section>
        </>
      )}
    </div>
  );
}

// MemoryDetailPage shows one owner-scoped memory with its provenance, its
// pinned sources and the same edit/delete actions as the memory form.
export function MemoryDetailPage({ id }: { id: string }) {
  const w = useWords();
  const loc = useLocale();
  const { navigate } = useRouter();
  const [record, setRecord] = useState<ContinuityRecord | null>(null);
  const [sources, setSources] = useState<
    Array<{ ref: SourceRef; entry: Entry | null }>
  >([]);
  const [error, setError] = useState<unknown>(null);
  const [editing, setEditing] = useState(false);
  const [title, setTitle] = useState("");
  const [text, setText] = useState("");
  const [busy, setBusy] = useState(false);
  const load = () => {
    setError(null);
    api<ContinuityRecord>(`/memory/${id}`)
      .then(async (r) => {
        setRecord(r);
        setTitle(r.title);
        setText(r.text);
        const refs = r.sourceRefs ?? [];
        setSources(
          await Promise.all(
            refs.map(async (ref) => {
              try {
                return { ref, entry: await api<Entry>(`/records/${ref.recordId}`) };
              } catch {
                return { ref, entry: null };
              }
            }),
          ),
        );
      })
      .catch(setError);
  };
  useEffect(load, [id]);
  const save = async (e: FormEvent) => {
    e.preventDefault();
    if (!record) return;
    setBusy(true);
    setError(null);
    try {
      const next = await api<ContinuityRecord>(
        `/memory/${record.id}`,
        { title, text, storageGranted: true, expectedVersion: record.version },
        "PATCH",
      );
      setRecord(next);
      setTitle(next.title);
      setText(next.text);
      setEditing(false);
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  };
  const sourceLabel = (r: Entry) =>
    r.title ||
    (r.kind === "conversation"
      ? w("Percakapan", "Conversation")
      : r.kind === "checkin"
        ? w("Cek perasaan", "Check-in")
        : w("Catatan", "Note"));
  return (
    <div className="s-narrow c-memory">
      <PageHeader
        back={{ to: "/notes/memory", label: w("Yang Sela ingat", "What Sela remembers") }}
        title={record?.title || w("Konteks pribadi", "Personal context")}
        lead={w(
          "Konteks ini kamu pilih sendiri. Saat izin balasan AI aktif, memori yang dikonfirmasi bisa dipakai Sela untuk membalas.",
          "You chose this context yourself. While AI reply consent is on, confirmed memory may be used by Sela to reply.",
        )}
      />
      <ErrorNotice error={error} onRetry={load} />
      {!record && !error && <Loading label={w("Membuka…", "Opening…")} />}
      {record && (
        <>
          <ContinuityHelp records={[record]} />
          <article className="c-paper">
            <p className="s-hint">
              {record.reviewStatus === "corrected"
                ? w(
                    "Kamu sudah mengoreksi konteks ini.",
                    "You corrected this context.",
                  )
                : w(
                    "Kamu sudah mengonfirmasi konteks ini.",
                    "You confirmed this context.",
                  )}{" "}
              {record.provenance === "user_correction"
                ? w(
                    "Yang tersimpan adalah versi koreksimu.",
                    "The saved version is your correction.",
                  )
                : w(
                    "Kata-katanya kamu tulis sendiri.",
                    "You wrote these words yourself.",
                  )}
            </p>
            <div className="s-record-meta">
              <ProvenanceBadge value={record.provenance} />
              <ReviewStatusChip value={record.reviewStatus} />
              <span>
                {w("versi ", "version ")}
                {record.version}
              </span>
            </div>
            <dl className="s-dl">
              <div>
                <dt>{w("Dibuat", "Created")}</dt>
                <dd>{loc.dateTime(record.createdAt)}</dd>
              </div>
              <div>
                <dt>{w("Terakhir diubah", "Last updated")}</dt>
                <dd>{loc.dateTime(record.updatedAt)}</dd>
              </div>
            </dl>
            {editing ? (
              <form className="c-editor" onSubmit={(e) => void save(e)}>
                <TextField
                  label={w("Judul", "Title")}
                  value={title}
                  onChange={(e) => setTitle(e.target.value)}
                  maxLength={120}
                  optional
                />
                <TextArea
                  label={w("Konteks pilihanmu", "Your chosen context")}
                  value={text}
                  onChange={(e) => setText(e.target.value)}
                  required
                  maxLength={6000}
                  rows={6}
                />
                <div className="s-actions">
                  <Button
                    type="submit"
                    variant="primary"
                    disabled={busy || !text.trim()}
                  >
                    {w("Simpan perbaikan", "Save changes")}
                  </Button>
                  <Button
                    onClick={() => {
                      setEditing(false);
                      setTitle(record.title);
                      setText(record.text);
                    }}
                  >
                    {w("Batal", "Cancel")}
                  </Button>
                </div>
              </form>
            ) : (
              <>
                <p className="c-original">{record.text}</p>
                <div className="s-actions">
                  <Button variant="secondary" onClick={() => setEditing(true)}>
                    {w("Perbaiki", "Edit")}
                  </Button>
                  <InlineConfirm
                    label={w("Hapus", "Delete")}
                    question={w("Hapus konteks ini?", "Delete this context?")}
                    confirmLabel={w("Hapus", "Delete")}
                    onConfirm={() => {
                      setBusy(true);
                      setError(null);
                      api(
                        `/memory/${record.id}`,
                        { expectedVersion: record.version },
                        "DELETE",
                      )
                        .then(() => navigate("/notes/memory"))
                        .catch(setError)
                        .finally(() => setBusy(false));
                    }}
                    busy={busy}
                  />
                </div>
              </>
            )}
          </article>
          <section className="c-source-references">
            <h2>{w("Sumber pilihanmu", "Your chosen sources")}</h2>
            {sources.length === 0 ? (
              <p>
                {w(
                  "Ditulis langsung olehmu tanpa sumber tersimpan.",
                  "Written directly by you without saved sources.",
                )}
              </p>
            ) : (
              <ul>
                {sources.map(({ ref, entry }) => (
                  <li key={ref.recordId}>
                    {entry ? (
                      <Link to={entryRoute(entry)}>{sourceLabel(entry)}</Link>
                    ) : (
                      w(
                        "Sumber sudah dihapus atau tidak dapat dibuka",
                        "Source deleted or unavailable",
                      )
                    )}{" "}
                    · v{ref.version}
                  </li>
                ))}
              </ul>
            )}
          </section>
        </>
      )}
    </div>
  );
}

export function SummariesPage({ path }: { path: string }) {
  const id = path.split("/")[3];
  return id && id !== "new" ? (
    <SummaryDetail key={id} id={id} />
  ) : id === "new" ? (
    <SummaryEditor />
  ) : (
    <SummaryList />
  );
}

function SummaryList() {
  const w = useWords();
  const loc = useLocale();
  const [items, setItems] = useState<ContinuityRecord[] | null>(null);
  const [error, setError] = useState<unknown>(null);
  const load = () => {
    setError(null);
    api<{ summaries: ContinuityRecord[] }>("/consultation-summaries")
      .then((d) => setItems(d.summaries))
      .catch(setError);
  };
  useEffect(load, []);
  return (
    <div className="c-summary-list">
      <PageHeader
        back={{ to: "/support", label: w("Dukungan", "Support") }}
        title={w("Ringkasan untuk konsultasi", "Summary for a consultation")}
        lead={w(
          "Bawa bagian cerita yang kamu pilih. Kata-katamu tetap milikmu.",
          "Bring the parts of your story you choose. Your words stay yours.",
        )}
        actions={
          <LinkButton to="/support/summaries/new" variant="primary">
            {w("Susun ringkasan", "Build a summary")}
          </LinkButton>
        }
      />
      <ErrorNotice error={error} onRetry={load} />
      {!items && !error && <Loading label={w("Membuka…", "Opening…")} />}
      {items?.length === 0 && (
        <EmptyState
          title={w("Belum ada ringkasan", "No summaries yet")}
          body={w(
            "Pilih catatan atau percakapan yang sudah disimpan. Tidak ada cerita yang dipilih otomatis.",
            "Choose saved notes or conversations. No story is selected automatically.",
          )}
          actions={
            <LinkButton to="/support/summaries/new" variant="secondary">
              {w("Susun ringkasan pertama", "Build your first summary")}
            </LinkButton>
          }
        />
      )}
      <div className="c-document-list">
        {items?.map((r) => (
          <Link
            key={r.id}
            to={`/support/summaries/${r.id}`}
            className="c-document-link"
          >
            <div>
              <h2>{r.title || w("Ringkasan pribadi", "Personal summary")}</h2>
              <p className="s-muted">
                {r.summaryState === "approved"
                  ? w("Sudah kamu setujui", "Approved by you")
                  : w("Draf untuk diperiksa", "Draft to review")}{" "}
                · {loc.shortDate(r.updatedAt)}
              </p>
            </div>
            <Icon name="arrow-right" />
          </Link>
        ))}
      </div>
    </div>
  );
}

function SummaryEditor() {
  const w = useWords();
  const { navigate } = useRouter();
  const loc = useLocale();
  const [records, setRecords] = useState<Entry[] | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [search, setSearch] = useState("");
  const [sourceLimit, setSourceLimit] = useState(8);
  const [selected, setSelected] = useState<string[]>([]);
  const [title, setTitle] = useState("");
  const [text, setText] = useState("");
  const [goal, setGoal] = useState("");
  const [questions, setQuestions] = useState("");
  const [bring, setBring] = useState("");
  const [allow, setAllow] = useState(false);
  const [busy, setBusy] = useState(false);
  const load = () => {
    setError(null);
    api<{ records: Entry[] }>("/records")
      .then((d) =>
        setRecords(
          d.records.filter(
            (r) =>
              ["note", "checkin", "conversation"].includes(r.kind) &&
              sourceText(r).trim(),
          ),
        ),
      )
      .catch(setError);
  };
  useEffect(load, []);
  const matching = (records ?? []).filter((r) =>
    (r.title + " " + sourceText(r))
      .toLowerCase()
      .includes(search.toLowerCase()),
  );
  const choose = (id: string, checked: boolean) => {
    const next = checked ? [...selected, id] : selected.filter((x) => x !== id);
    setSelected(next);
    setText(
      (records ?? [])
        .filter((r) => next.includes(r.id))
        .map(sourceText)
        .join("\n\n"),
    );
  };
  const save = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const chosen = (records ?? []).filter((r) => selected.includes(r.id));
      const r = await api<ContinuityRecord>(
        "/consultation-summaries",
        {
          title,
          text,
          sourceRefs: chosen.map((r) => ({
            recordId: r.id,
            version: r.version,
          })),
          prepared: { goal, questions, bring },
          storageGranted: true,
        },
        "POST",
      );
      navigate(`/support/summaries/${r.id}`);
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  };
  return (
    <div className="c-summary-workspace">
      <PageHeader
        back={{ to: "/support/summaries", label: w("Ringkasan", "Summaries") }}
        title={w("Pilih yang ingin kamu bawa", "Choose what to bring")}
        lead={w(
          "Tanpa AI. Pilih sumber, periksa kata-katanya, lalu simpan sebagai draf.",
          "Without AI. Choose sources, review the words, then save a draft.",
        )}
      />
      <ErrorNotice error={error} onRetry={load} />
      <form onSubmit={save} className="c-summary-grid">
        <section className="c-sources">
          <h2>{w("Dari Catatanku", "From My notes")}</h2>
          <p className="s-hint">
            {w(
              "Maksimal 12 sumber. Balasan AI tidak disalin.",
              "Up to 12 sources. AI replies are not copied.",
            )}
          </p>
          {!records && !error && <Loading label={w("Membuka…", "Opening…")} />}
          {records?.length === 0 && (
            <EmptyState
              title={w("Belum ada tulisan tersimpan", "No saved writing yet")}
              actions={
                <LinkButton to="/notes/write">
                  {w("Tulis catatan", "Write a note")}
                </LinkButton>
              }
            />
          )}
          <TextField
            type="search"
            label={w("Cari tulisan sumber", "Search source writing")}
            value={search}
            onChange={(e) => {
              setSearch(e.target.value);
              setSourceLimit(8);
            }}
          />
          {matching.slice(0, sourceLimit).map((r) => (
            <div className="c-source" key={r.id}>
              <CheckboxRow
                checked={selected.includes(r.id)}
                onChange={(v) => choose(r.id, v)}
                disabled={!selected.includes(r.id) && selected.length >= 12}
                hint={loc.shortDate(r.updatedAt)}
              >
                {r.title ||
                  (r.kind === "conversation"
                    ? w("Percakapan", "Conversation")
                    : r.kind === "checkin"
                      ? w("Cek perasaan", "Check-in")
                      : w("Catatan", "Note"))}
              </CheckboxRow>
              <p>
                {sourceText(r).slice(0, 160)}
                {sourceText(r).length > 160 ? "…" : ""}
              </p>
            </div>
          ))}
          {matching.length > sourceLimit && (
            <Button onClick={() => setSourceLimit((x) => x + 12)}>
              {w("Tampilkan sumber lainnya", "Show more sources")}
            </Button>
          )}
        </section>
        <section className="c-summary-draft">
          <h2>{w("Draf milikmu", "Your draft")}</h2>
          <TextField
            label={w("Judul ringkasan", "Summary title")}
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            maxLength={120}
            optional
          />
          <TextArea
            label={w("Yang ingin kamu ceritakan", "What you want to say")}
            value={text}
            onChange={(e) => setText(e.target.value)}
            maxLength={6000}
            rows={10}
            required
            hint={w(
              "Memilih ulang sumber akan menyusun ulang kutipan. Kamu boleh memperbaiki kata-katanya sebelum menyimpan.",
              "Changing source selection rebuilds the excerpts. You can edit the words before saving.",
            )}
          />
          <details className="c-preparation">
            <summary>
              {w("Siapkan percakapannya", "Prepare for the conversation")}
            </summary>
            <TextArea
              label={w("Yang ingin kubicarakan", "What I want to discuss")}
              value={goal}
              onChange={(e) => setGoal(e.target.value)}
              maxLength={1000}
              rows={3}
              optional
            />
            <TextArea
              label={w(
                "Pertanyaan yang ingin kutanyakan",
                "Questions I want to ask",
              )}
              value={questions}
              onChange={(e) => setQuestions(e.target.value)}
              maxLength={1000}
              rows={3}
              optional
            />
            <TextArea
              label={w("Yang ingin kubawa", "What I want to bring")}
              value={bring}
              onChange={(e) => setBring(e.target.value)}
              maxLength={1000}
              rows={3}
              optional
            />
          </details>
          <CheckboxRow checked={allow} onChange={setAllow}>
            {w(
              "Simpan draf ini di server Sela",
              "Save this draft on Sela's server",
            )}
          </CheckboxRow>
          <Button
            type="submit"
            variant="primary"
            disabled={
              !allow ||
              selected.length === 0 ||
              !text.trim() ||
              busy ||
              text.length > 6000
            }
          >
            {w("Simpan draf untuk diperiksa", "Save draft to review")}
          </Button>
        </section>
      </form>
    </div>
  );
}

function SummaryDetail({ id }: { id: string }) {
  const w = useWords();
  const loc = useLocale();
  const { navigate } = useRouter();
  const [r, setRecord] = useState<ContinuityRecord | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [sources, setSources] = useState<Entry[]>([]);
  const [followups, setFollowups] = useState<ContinuityRecord[]>([]);
  const [reviewed, setReviewed] = useState(false);
  const [busy, setBusy] = useState(false);
  const [editing, setEditing] = useState(false);
  const [text, setText] = useState("");
  const [after, setAfter] = useState("");
  const [saveAfter, setSaveAfter] = useState(false);
  const load = () => {
    setError(null);
    setReviewed(false);
    Promise.all([
      api<ContinuityRecord>(`/consultation-summaries/${id}`),
      api<{ records: Entry[] }>("/records"),
      api<{ followups: ContinuityRecord[] }>(
        `/consultation-summaries/${id}/followups`,
      ),
    ])
      .then(([summary, records, fs]) => {
        setRecord(summary);
        setText(summary.text);
        setSources(records.records);
        setFollowups(fs.followups);
      })
      .catch(setError);
  };
  useEffect(load, [id]);
  const stale =
    r?.sourceRefs?.some(
      (ref) =>
        !sources.some(
          (s) => s.id === ref.recordId && s.version === ref.version,
        ),
    ) ?? false;
  const run = async (action: () => Promise<unknown>) => {
    setError(null);
    setBusy(true);
    try {
      await action();
      load();
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  };
  const downloadSummary = async (format: "html" | "md") => {
    if (!r || stale || r.summaryState !== "approved") return;
    setBusy(true);
    setError(null);
    try {
      const current = await api<ContinuityRecord>(
        `/consultation-summaries/${id}`,
      );
      if (
        current.version !== r.version ||
        current.summaryState !== "approved"
      ) {
        setRecord(current);
        throw new Error(
          w(
            "Ringkasan atau sumbernya berubah. Periksa ulang sebelum mengunduh.",
            "The summary or its sources changed. Review again before downloading.",
          ),
        );
      }
      await downloadFile(
        `/consultation-summaries/${id}/export?format=${format}&lang=${loc.lang}`,
        `sela-ringkasan.${format}`,
      );
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  };
  return (
    <div className="s-narrow c-summary-detail">
      <PageHeader
        back={{ to: "/support/summaries", label: w("Ringkasan", "Summaries") }}
        title={r?.title || w("Ringkasan pribadi", "Personal summary")}
      />
      <ErrorNotice error={error} onRetry={load} />
      {!r && !error && <Loading label={w("Membuka…", "Opening…")} />}
      {r && (
        <>
          <ContinuityHelp records={[r, ...followups]} />
          <div className="c-state-row">
            <span>
              {r.summaryState === "approved" && !stale
                ? w("Versi ini sudah kamu setujui", "You approved this version")
                : w("Draf · perlu kamu periksa", "Draft · needs your review")}
            </span>
            <span className="s-hint">
              v{r.version} · {loc.shortDate(r.updatedAt)}
            </span>
          </div>
          {stale && (
            <Notice
              tone="warning"
              title={w("Sumber sudah berubah", "A source has changed")}
            >
              <p>
                {w(
                  "Ringkasan ini tidak lagi sesuai versi sumbernya. Susun ulang dari sumber terbaru sebelum menyetujui atau mengunduh.",
                  "This summary no longer matches its source versions. Rebuild it from the latest sources before approving or downloading.",
                )}
              </p>
              <LinkButton to="/support/summaries/new">
                {w("Susun ulang", "Rebuild summary")}
              </LinkButton>
            </Notice>
          )}
          <article className="c-paper">
            <p className="s-hint">
              {w(
                "Kata-kata pengguna · tanpa AI · bukan diagnosis",
                "User's words · without AI · not a diagnosis",
              )}
            </p>
            {editing ? (
              <TextArea
                label={w("Perbaiki ringkasan", "Edit summary")}
                value={text}
                onChange={(e) => setText(e.target.value)}
                maxLength={6000}
                rows={10}
              />
            ) : (
              <p className="c-original">{r.text}</p>
            )}
            {r.prepared && Object.values(r.prepared).some(Boolean) && (
              <section className="c-preparation">
                <h2>{w("Persiapanmu", "Your preparation")}</h2>
                {[
                  [
                    w("Yang ingin kubicarakan", "What I want to discuss"),
                    r.prepared.goal,
                  ],
                  [w("Pertanyaanku", "My questions"), r.prepared.questions],
                  [w("Yang kubawa", "What I am bringing"), r.prepared.bring],
                ]
                  .filter(([, v]) => v)
                  .map(([label, v]) => (
                    <div key={label}>
                      <h3>{label}</h3>
                      <p className="c-original">{v}</p>
                    </div>
                  ))}
              </section>
            )}
            <div className="s-actions">
              {editing ? (
                <>
                  <Button
                    variant="primary"
                    disabled={busy || !text.trim()}
                    onClick={() =>
                      void run(async () => {
                        await api(
                          `/consultation-summaries/${id}`,
                          {
                            title: r.title,
                            text,
                            sourceRefs: r.sourceRefs ?? [],
                            prepared: r.prepared,
                            storageGranted: true,
                            expectedVersion: r.version,
                          },
                          "PATCH",
                        );
                        setEditing(false);
                      })
                    }
                  >
                    {w("Simpan perbaikan", "Save changes")}
                  </Button>
                  <Button
                    onClick={() => {
                      setEditing(false);
                      setText(r.text);
                    }}
                  >
                    {w("Batal", "Cancel")}
                  </Button>
                </>
              ) : (
                <Button variant="quiet" onClick={() => setEditing(true)}>
                  {w("Perbaiki kata-kata", "Edit wording")}
                </Button>
              )}
            </div>
          </article>
          <section className="c-source-references">
            <h2>{w("Sumber pilihanmu", "Your chosen sources")}</h2>
            {(r.sourceRefs ?? []).length === 0 && (
              <p>
                {w(
                  "Ditulis langsung olehmu tanpa kutipan tersimpan.",
                  "Written directly by you without saved excerpts.",
                )}
              </p>
            )}
            <ul>
              {r.sourceRefs?.map((ref) => {
                const src = sources.find((x) => x.id === ref.recordId);
                return (
                  <li key={ref.recordId}>
                    {src ? (
                      <Link to={entryRoute(src)}>
                        {src.title || w("Tulisan asli", "Original writing")}
                      </Link>
                    ) : (
                      w("Sumber sudah dihapus", "Source deleted")
                    )}{" "}
                    · v{ref.version}
                  </li>
                );
              })}
            </ul>
          </section>
          {!editing && r.summaryState !== "approved" && (
            <section className="c-approval">
              <h2>
                {w(
                  "Periksa sebelum kamu bawa",
                  "Review before taking it with you",
                )}
              </h2>
              <CheckboxRow checked={reviewed} onChange={setReviewed}>
                {w(
                  "Aku sudah memeriksa versi ini dan isinya mewakili yang ingin kusampaikan",
                  "I reviewed this version and it represents what I want to say",
                )}
              </CheckboxRow>
              <Button
                variant="primary"
                disabled={busy || stale || !reviewed}
                onClick={() =>
                  void run(() =>
                    api(
                      `/consultation-summaries/${id}/approve`,
                      { expectedVersion: r.version },
                      "POST",
                    ),
                  )
                }
              >
                {w("Setujui versi ini", "Approve this version")}
              </Button>
            </section>
          )}
          {r.summaryState === "approved" && !stale && !editing && (
            <section className="c-export">
              <h2>{w("Bawa dengan caramu", "Take it your way")}</h2>
              <p>
                {w(
                  "Unduh untuk kamu tunjukkan atau cetak sendiri. Tidak ada penerima yang dihubungi. Salinan yang diunduh tidak bisa ditarik kembali.",
                  "Download to show or print yourself. No recipient is contacted. Downloaded copies cannot be recalled.",
                )}
              </p>
              <div className="s-actions">
                <Button
                  variant="primary"
                  disabled={busy}
                  onClick={() => void downloadSummary("html")}
                >
                  {w("Unduh HTML (untuk dicetak)", "Download HTML (to print)")}
                </Button>
                <Button
                  variant="secondary"
                  disabled={busy}
                  onClick={() => void downloadSummary("md")}
                >
                  {w("Unduh Markdown", "Download Markdown")}
                </Button>
              </div>
            </section>
          )}
          <section className="c-followups">
            <h2>{w("Setelah percakapan itu", "After that conversation")}</h2>
            <p>
              {w(
                "Catat yang ingin kamu ingat. Ini catatan pribadimu, bukan catatan resmi profesional.",
                "Keep what you want to remember. This is your personal note, not an official professional record.",
              )}
            </p>
            <form
              onSubmit={(e) => {
                e.preventDefault();
                void run(async () => {
                  await api(
                    `/consultation-summaries/${id}/followups`,
                    {
                      title: w("Setelah konsultasi", "After the consultation"),
                      text: after,
                      storageGranted: true,
                    },
                    "POST",
                  );
                  setAfter("");
                  setSaveAfter(false);
                });
              }}
            >
              <TextArea
                label={w(
                  "Catatan setelah konsultasi",
                  "Note after the consultation",
                )}
                value={after}
                onChange={(e) => setAfter(e.target.value)}
                maxLength={6000}
                rows={4}
                required
              />
              <CheckboxRow checked={saveAfter} onChange={setSaveAfter}>
                {w(
                  "Simpan catatan tindak lanjut ini",
                  "Save this follow-up note",
                )}
              </CheckboxRow>
              <Button
                type="submit"
                variant="primary"
                disabled={busy || !after.trim() || !saveAfter}
              >
                {w("Simpan tindak lanjut", "Save follow-up")}
              </Button>
            </form>
            {followups.map((f) => (
              <article className="c-followup" key={f.id}>
                <p className="s-hint">{loc.dateTime(f.createdAt)}</p>
                <p className="c-original">{f.text}</p>
                <Link to={`/notes/${f.id}`}>
                  {w("Buka catatan", "Open note")}
                </Link>
              </article>
            ))}
          </section>
          <div className="s-actions">
            <InlineConfirm
              label={w("Hapus ringkasan", "Delete summary")}
              question={w("Hapus ringkasan ini?", "Delete this summary?")}
              confirmLabel={w("Hapus ringkasan", "Delete summary")}
              busy={busy}
              onConfirm={() =>
                void run(async () => {
                  await api(
                    `/consultation-summaries/${id}`,
                    { expectedVersion: r.version },
                    "DELETE",
                  );
                  navigate("/support/summaries");
                })
              }
            />
          </div>
        </>
      )}
    </div>
  );
}
