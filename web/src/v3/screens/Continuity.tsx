import { useEffect, useState, type FormEvent } from "react";
import { Link, useRouter } from "../../lib/router";
import { api, type Consent, type Entry } from "../api";
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
      ? "/notes/memory"
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
  const { policies } = useSession();
  const [data, setData] = useState<{
    enabled: boolean;
    items: ContinuityRecord[];
  } | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const [editing, setEditing] = useState<ContinuityRecord | null>(null);
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
      setEditing(null);
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
        editing ? `/memory/${editing.id}` : "/memory",
        {
          title,
          text,
          storageGranted: true,
          ...(editing ? { expectedVersion: editing.version } : {}),
        },
        editing ? "PATCH" : "POST",
      );
      setEditing(null);
      setTitle("");
      setText("");
      load();
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  };
  const del = async (r: Entry) => {
    setBusy(true);
    setError(null);
    try {
      await api(`/memory/${r.id}`, { expectedVersion: r.version }, "DELETE");
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
              "Memori mati secara default. Jika diaktifkan, hanya hal yang kamu tulis di halaman ini yang disimpan, terenkripsi di server Sela. Tidak ada pengambilan cerita otomatis atau pengiriman memori ke AI.",
              "Memory is off by default. When enabled, only what you write here is saved, encrypted on Sela's server. Stories are not collected automatically and memory is not sent to AI.",
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
                "Aktif · ditulis olehmu, tanpa AI",
                "On · written by you, without AI",
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
              "Mematikan menghentikan penggunaan memori; isi tersimpan tetap bisa dihapus setelah diaktifkan kembali atau lewat hapus semua data.",
              "Turning memory off stops its use; saved items can be deleted after re-enabling it, or by deleting all data.",
            )}
          </p>
          <form className="c-editor" onSubmit={save}>
            <h2>
              {editing
                ? w("Perbaiki konteks", "Edit context")
                : w(
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
                {editing
                  ? w("Simpan perbaikan", "Save changes")
                  : w("Simpan konteks", "Save context")}
              </Button>
              {editing && (
                <Button
                  onClick={() => {
                    setEditing(null);
                    setText("");
                    setTitle("");
                  }}
                >
                  {w("Batal", "Cancel")}
                </Button>
              )}
            </div>
          </form>
          <ContinuityHelp records={data.items} />
          <section
            className="c-records"
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
              <article key={r.id} className="c-memory-item">
                <h2>{r.title || w("Konteks pribadi", "Personal context")}</h2>
                <p className="c-original">{r.text}</p>
                <div className="s-actions">
                  <Button
                    variant="quiet"
                    onClick={() => {
                      setEditing(r);
                      setTitle(r.title);
                      setText(r.text);
                    }}
                  >
                    {w("Perbaiki", "Edit")}
                  </Button>
                  <InlineConfirm
                    label={w("Hapus", "Delete")}
                    question={w("Hapus konteks ini?", "Delete this context?")}
                    confirmLabel={w("Hapus", "Delete")}
                    onConfirm={() => void del(r)}
                    busy={busy}
                  />
                </div>
              </article>
            ))}
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
  const download = async () => {
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
      const content = [
        r.title || w("Ringkasan pribadi", "Personal summary"),
        w(
          "Ditulis dan disetujui pengguna · bukan diagnosis atau catatan resmi tenaga profesional",
          "Written and approved by the user · not a diagnosis or official professional record",
        ),
        `v${r.version} · ${loc.dateTime(r.updatedAt)}`,
        r.text,
        ...(r.prepared
          ? [r.prepared.goal, r.prepared.questions, r.prepared.bring]
          : []),
        w("Sumber pilihan", "Chosen sources"),
        ...(r.sourceRefs ?? []).map((ref) => {
          const src = sources.find((x) => x.id === ref.recordId);
          return `${src?.title || src?.kind || ref.recordId} · v${ref.version}`;
        }),
      ]
        .filter(Boolean)
        .join("\n\n");
      const url = URL.createObjectURL(
        new Blob([content], { type: "text/plain;charset=utf-8" }),
      );
      const a = document.createElement("a");
      a.href = url;
      a.download = "sela-ringkasan.txt";
      a.click();
      setTimeout(() => URL.revokeObjectURL(url), 1000);
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
              <Button
                variant="primary"
                disabled={busy}
                onClick={() => void download()}
              >
                {w("Unduh ringkasan (.txt)", "Download summary (.txt)")}
              </Button>
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
