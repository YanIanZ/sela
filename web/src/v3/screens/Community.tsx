import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Link, useRouter } from "../../lib/router";
import { api, type Policy, type Profile } from "../api";
import { useConsent } from "../consent";
import { EmergencySheet } from "../emergency";
import { Icon } from "../icons";
import { useAnnounce, useLocale, useWords } from "../shared";
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
} from "../ui";

type Status = "published" | "held" | "hidden" | "removed";

type Post = {
  id: string;
  alias: string;
  topic: string;
  body: string;
  status: Status;
  labels?: string[];
  replies: number;
  mine: boolean;
  createdAt: string;
};

type Reply = {
  id: string;
  alias: string;
  body: string;
  status: Status;
  labels?: string[];
  mine: boolean;
  createdAt: string;
};

const topics = [
  { value: "daily", id: "Sehari-hari", en: "Everyday" },
  { value: "work", id: "Kerja", en: "Work" },
  { value: "study", id: "Sekolah & kuliah", en: "Study" },
  { value: "family", id: "Keluarga", en: "Family" },
  { value: "relationship", id: "Hubungan", en: "Relationships" },
  { value: "loneliness", id: "Kesepian", en: "Loneliness" },
  { value: "other", id: "Lainnya", en: "Other" },
];

const reasons = [
  { value: "bullying", id: "Perundungan", en: "Bullying" },
  { value: "harassment", id: "Pelecehan", en: "Harassment" },
  { value: "hate", id: "Ujaran kebencian", en: "Hate" },
  { value: "self_harm", id: "Mendorong menyakiti diri", en: "Encourages self-harm" },
  { value: "personal_info", id: "Data pribadi", en: "Personal information" },
  { value: "sexual", id: "Konten seksual", en: "Sexual content" },
  { value: "spam", id: "Spam", en: "Spam" },
  { value: "other", id: "Lainnya", en: "Other" },
];

function topicLabel(value: string, w: (id: string, en: string) => string) {
  const t = topics.find((x) => x.value === value);
  return t ? w(t.id, t.en) : value;
}

function StatusNote({ status }: { status: Status }) {
  const w = useWords();
  if (status === "published") return null;
  return (
    <p className={`c-community-status c-community-status--${status}`}>
      <Icon name={status === "held" ? "clock" : "eye-off"} size={16} />
      {status === "held"
        ? w("Sedang ditinjau — belum terlihat oleh orang lain.", "Under review — not visible to others yet.")
        : w(
            "Tidak ditampilkan karena terbaca bisa menyakiti orang lain. Operator akan meninjaunya.",
            "Not shown because it may hurt others. An operator will review it.",
          )}
    </p>
  );
}

/** Gate: adults with the community consent. Renders the gate or children. */
function CommunityGate({ user, policy, children }: { user: Profile; policy?: Policy; children: React.ReactNode }) {
  const w = useWords();
  const consent = useConsent("community", policy);
  const [allow, setAllow] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  if (user.ageBand !== "18_plus") {
    return (
      <Notice tone="info" title={w("Khusus 18 tahun ke atas", "For ages 18 and over")}>
        <p>
          {w(
            "Ruang berbagi hanya untuk pengguna dewasa. Kamu tetap bisa bercerita dengan Sela dan menulis catatan pribadi.",
            "The sharing space is for adults only. You can still talk with Sela and write private notes.",
          )}
        </p>
      </Notice>
    );
  }
  if (consent.loading) return <Loading label={w("Memuat…", "Loading…")} />;
  if (!consent.granted) {
    return (
      <section className="c-community-consent">
        <h2>{w("Sebelum ikut berbagi", "Before you join")}</h2>
        <ErrorNotice error={error ?? consent.error} />
        {policy?.body.map((b, i) => (
          <p key={i}>{w(b.id, b.en)}</p>
        ))}
        <CheckboxRow checked={allow} onChange={setAllow}>
          {w(
            "Aku setuju dengan aturan ruang berbagi dan Kode Etik Sela",
            "I agree to the sharing space rules and Sela's Code of Conduct",
          )}
        </CheckboxRow>
        <div className="s-actions">
          <Button
            variant="primary"
            disabled={!allow || busy}
            busy={busy}
            onClick={async () => {
              setBusy(true);
              setError(null);
              try {
                await consent.set(true);
              } catch (e) {
                setError(e);
              } finally {
                setBusy(false);
              }
            }}
          >
            {w("Masuk ruang berbagi", "Enter the sharing space")}
          </Button>
        </div>
      </section>
    );
  }
  return <>{children}</>;
}

export function CommunityPage({ user, policies }: { user: Profile; policies?: Record<string, Policy> }) {
  const { path } = useRouter();
  const id = path.startsWith("/support/community/") ? path.slice("/support/community/".length) : "";
  const w = useWords();
  return (
    <div className="c-community">
      <PageHeader
        back={{ to: "/support", label: w("Dukungan", "Support") }}
        title={w("Ruang berbagi", "Sharing space")}
        lead={w(
          "Bagikan ceritamu secara anonim dan saling menguatkan. Setiap tulisan diperiksa dulu sebelum tampil.",
          "Share your story anonymously and lift each other up. Everything is checked before it appears.",
        )}
      />
      <CommunityGate user={user} policy={policies?.community}>
        {id ? <Thread id={id} /> : <Feed />}
      </CommunityGate>
    </div>
  );
}

function useCrisis() {
  const [open, setOpen] = useState(false);
  const sheet = <EmergencySheet open={open} onClose={() => setOpen(false)} context="note" />;
  return { sheet, open: () => setOpen(true) };
}

function Feed() {
  const w = useWords();
  const loc = useLocale();
  const { announce } = useAnnounce();
  const crisis = useCrisis();
  const [topic, setTopic] = useState("");
  const [posts, setPosts] = useState<Post[] | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [body, setBody] = useState("");
  const [newTopic, setNewTopic] = useState("daily");
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState<"" | "held" | "hidden" | "support">("");

  const load = useCallback(() => {
    setError(null);
    api<{ posts: Post[] }>("/community/posts" + (topic ? "?topic=" + topic : ""))
      .then((d) => setPosts(d.posts))
      .catch((e) => {
        setError(e);
        setPosts([]);
      });
  }, [topic]);
  useEffect(load, [load]);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (!body.trim() || busy) return;
    setBusy(true);
    setError(null);
    setNotice("");
    try {
      const res = await api<{ post: Post; support: boolean }>("/community/posts", { body, topic: newTopic });
      setBody("");
      setNotice(res.post.status === "published" ? (res.support ? "support" : "") : (res.post.status as "held" | "hidden"));
      announce(
        "status",
        res.post.status === "published"
          ? w("Ceritamu sudah dibagikan.", "Your story is shared.")
          : w("Ceritamu belum ditampilkan.", "Your story isn't shown yet."),
      );
      load();
    } catch (err) {
      if ((err as { code?: string }).code === "crisis") crisis.open();
      else setError(err);
    } finally {
      setBusy(false);
    }
  };

  return (
    <>
      {crisis.sheet}
      <form className="c-community-compose" onSubmit={(e) => void submit(e)}>
        <h2>{w("Bagikan ceritamu", "Share your story")}</h2>
        <div className="c-community-topics" role="group" aria-label={w("Topik", "Topic")}>
          {topics.map((t) => (
            <button key={t.value} type="button" className="s-filter" aria-pressed={newTopic === t.value} onClick={() => setNewTopic(t.value)}>
              {w(t.id, t.en)}
            </button>
          ))}
        </div>
        <TextArea
          label={w("Ceritamu", "Your story")}
          hint={w(
            "Tampil dengan nama samaran acak. Jangan tulis nama, nomor, atau alamat siapa pun.",
            "Shown under a random alias. Don't write anyone's name, number or address.",
          )}
          rows={4}
          maxLength={2000}
          value={body}
          onChange={(e) => setBody(e.target.value)}
        />
        <div className="s-actions">
          <Button type="submit" variant="primary" icon="send" disabled={!body.trim() || busy} busy={busy}>
            {w("Bagikan", "Share")}
          </Button>
        </div>
        {notice === "held" && <StatusNote status="held" />}
        {notice === "hidden" && <StatusNote status="hidden" />}
        {notice === "support" && (
          <p className="c-community-support">
            {w("Ceritamu terdengar berat. Kalau butuh, bantuan selalu ada di sini.", "Your story sounds heavy. If you need it, help is always here.")}{" "}
            <Link to="/help">{w("Lihat bantuan", "See help")}</Link>
          </p>
        )}
      </form>

      <section className="c-community-feed" aria-labelledby="feed-title">
        <div className="s-section-title">
          <h2 id="feed-title">{w("Cerita dari sesama", "Stories from others")}</h2>
        </div>
        <div className="c-community-filter" role="group" aria-label={w("Saring topik", "Filter by topic")}>
          <button type="button" className="s-filter" aria-pressed={topic === ""} onClick={() => setTopic("")}>
            {w("Semua", "All")}
          </button>
          {topics.map((t) => (
            <button key={t.value} type="button" className="s-filter" aria-pressed={topic === t.value} onClick={() => setTopic(t.value)}>
              {w(t.id, t.en)}
            </button>
          ))}
        </div>
        <ErrorNotice error={error} onRetry={load} />
        {posts === null ? (
          <Loading label={w("Memuat cerita…", "Loading stories…")} />
        ) : posts.length === 0 ? (
          <EmptyState
            title={w("Belum ada cerita di sini.", "No stories here yet.")}
            body={w("Jadilah yang pertama berbagi, dengan cara yang nyaman untukmu.", "Be the first to share, in a way that feels right.")}
            mark
          />
        ) : (
          <ul className="c-community-list">
            {posts.map((p) => (
              <li key={p.id}>
                <Link to={"/support/community/" + p.id} className="c-community-card">
                  <span className="c-community-meta">
                    <span className="c-community-alias">{p.alias}</span>
                    <span>{topicLabel(p.topic, w)}</span>
                    <time dateTime={p.createdAt}>{loc.dateTime(p.createdAt)}</time>
                    {p.mine && <span className="c-community-mine">{w("Milikmu", "Yours")}</span>}
                  </span>
                  <span className="c-community-body">{p.body}</span>
                  <span className="c-community-foot">
                    <Icon name="talk" size={16} />
                    {p.replies} {w("balasan", "replies")}
                  </span>
                </Link>
                {p.mine && <StatusNote status={p.status} />}
              </li>
            ))}
          </ul>
        )}
      </section>
    </>
  );
}

function ReportButton({ kind, id, onDone }: { kind: "post" | "reply"; id: string; onDone: () => void }) {
  const w = useWords();
  const { announce } = useAnnounce();
  const [open, setOpen] = useState(false);
  const [sending, setSending] = useState(false);
  const [error, setError] = useState<unknown>(null);
  if (!open) {
    return (
      <button type="button" className="s-text-button c-community-report" onClick={() => setOpen(true)}>
        <Icon name="alert" size={14} />
        {w("Laporkan", "Report")}
      </button>
    );
  }
  return (
    <div className="c-community-reasons" role="group" aria-label={w("Alasan melaporkan", "Reason for reporting")}>
      <ErrorNotice error={error} />
      {reasons.map((r) => (
        <button
          key={r.value}
          type="button"
          className="s-filter"
          disabled={sending}
          aria-busy={sending || undefined}
          onClick={async () => {
            if (sending) return;
            setSending(true);
            setError(null);
            try {
              await api("/community/reports", { kind, id, reason: r.value });
              announce("status", w("Terima kasih, laporanmu diterima.", "Thank you, your report was received."));
              setOpen(false);
              onDone();
            } catch (e) {
              setError(e);
            } finally {
              setSending(false);
            }
          }}
        >
          {w(r.id, r.en)}
        </button>
      ))}
      <button type="button" className="s-text-button" disabled={sending} onClick={() => setOpen(false)}>
        {w("Batal", "Cancel")}
      </button>
    </div>
  );
}

function Thread({ id }: { id: string }) {
  const w = useWords();
  const loc = useLocale();
  const { navigate } = useRouter();
  const { announce } = useAnnounce();
  const crisis = useCrisis();
  const [post, setPost] = useState<Post | null>(null);
  const [replies, setReplies] = useState<Reply[]>([]);
  const [error, setError] = useState<unknown>(null);
  const [missing, setMissing] = useState(false);
  const [body, setBody] = useState("");
  const [busy, setBusy] = useState(false);
  // deleting is "post" or the id of the reply being deleted; one delete
  // runs at a time so repeated clicks send one request.
  const [deleting, setDeleting] = useState<string | null>(null);
  const [actionError, setActionError] = useState<unknown>(null);
  const remove = async (path: string, key: string, after: () => void) => {
    if (deleting) return;
    setDeleting(key);
    setActionError(null);
    try {
      await api(path, undefined, "DELETE");
      after();
    } catch (err) {
      setActionError(err);
    } finally {
      setDeleting(null);
    }
  };
  const load = useCallback(() => {
    api<{ post: Post; replies: Reply[] }>("/community/posts/" + id)
      .then((d) => {
        setPost(d.post);
        setReplies(d.replies);
      })
      .catch((e) => {
        if ((e as { code?: string }).code === "not_found") setMissing(true);
        else setError(e);
      });
  }, [id]);
  useEffect(load, [load]);

  const reply = async (e: FormEvent) => {
    e.preventDefault();
    if (!body.trim() || busy) return;
    setBusy(true);
    setError(null);
    try {
      const res = await api<{ reply: Reply }>("/community/posts/" + id + "/replies", { body });
      setBody("");
      announce(
        "status",
        res.reply.status === "published"
          ? w("Balasanmu terkirim.", "Your reply was sent.")
          : w("Balasanmu belum ditampilkan.", "Your reply isn't shown yet."),
      );
      load();
    } catch (err) {
      if ((err as { code?: string }).code === "crisis") crisis.open();
      else setError(err);
    } finally {
      setBusy(false);
    }
  };

  if (missing) {
    return (
      <EmptyState
        title={w("Cerita ini tidak tersedia.", "This story isn't available.")}
        body={w("Mungkin sudah dihapus penulisnya atau sedang ditinjau.", "It may have been deleted by its author or be under review.")}
        actions={<LinkButton to="/support/community" variant="secondary">{w("Kembali ke ruang berbagi", "Back to the sharing space")}</LinkButton>}
        mark
      />
    );
  }
  if (!post) return error ? <ErrorNotice error={error} onRetry={load} /> : <Loading label={w("Memuat cerita…", "Loading the story…")} />;

  return (
    <>
      {crisis.sheet}
      <ErrorNotice error={actionError} />
      <article className="c-community-thread">
        <p className="c-community-meta">
          <span className="c-community-alias">{post.alias}</span>
          <span>{topicLabel(post.topic, w)}</span>
          <time dateTime={post.createdAt}>{loc.dateTime(post.createdAt)}</time>
        </p>
        <p className="c-community-body c-community-body--full">{post.body}</p>
        {post.mine && <StatusNote status={post.status} />}
        <div className="c-community-actions">
          {post.mine ? (
            <InlineConfirm
              label={w("Hapus ceritaku", "Delete my story")}
              question={w("Hapus cerita ini beserta semua balasannya?", "Delete this story and all its replies?")}
              confirmLabel={w("Hapus", "Delete")}
              busy={deleting !== null}
              onConfirm={() =>
                void remove("/community/posts/" + post.id, "post", () => {
                  announce("status", w("Ceritamu dihapus.", "Your story was deleted."));
                  navigate("/support/community");
                })
              }
            />
          ) : (
            <ReportButton kind="post" id={post.id} onDone={load} />
          )}
        </div>
      </article>

      <section className="c-community-replies" aria-labelledby="replies-title">
        <h2 id="replies-title">{w("Balasan", "Replies")}</h2>
        {replies.length === 0 ? (
          <p className="s-muted">{w("Belum ada balasan. Kata-kata hangatmu bisa jadi yang pertama.", "No replies yet. Your kind words could be the first.")}</p>
        ) : (
          <ul className="c-community-list c-community-list--replies">
            {replies.map((r) => (
              <li key={r.id} className="c-community-reply">
                <p className="c-community-meta">
                  <span className="c-community-alias">{r.alias}</span>
                  <time dateTime={r.createdAt}>{loc.dateTime(r.createdAt)}</time>
                </p>
                <p className="c-community-body c-community-body--full">{r.body}</p>
                {r.mine && <StatusNote status={r.status} />}
                <div className="c-community-actions">
                  {r.mine ? (
                    <button
                      type="button"
                      className="s-text-button"
                      disabled={deleting !== null}
                      aria-busy={deleting === r.id || undefined}
                      onClick={() =>
                        void remove("/community/replies/" + r.id, r.id, () => {
                          announce("status", w("Balasanmu dihapus.", "Your reply was deleted."));
                          load();
                        })
                      }
                    >
                      {deleting === r.id ? w("Menghapus…", "Deleting…") : w("Hapus balasanku", "Delete my reply")}
                    </button>
                  ) : (
                    <ReportButton kind="reply" id={r.id} onDone={load} />
                  )}
                </div>
              </li>
            ))}
          </ul>
        )}
        {post.status === "published" && (
          <form className="c-community-compose c-community-compose--reply" onSubmit={(e) => void reply(e)}>
            <ErrorNotice error={error} />
            <TextArea
              label={w("Balas dengan dukungan", "Reply with support")}
              hint={w("Dengarkan dulu, jangan menghakimi. Balasan kasar akan disembunyikan.", "Listen first, don't judge. Harsh replies are hidden.")}
              rows={3}
              maxLength={1000}
              value={body}
              onChange={(e) => setBody(e.target.value)}
            />
            <div className="s-actions">
              <Button type="submit" variant="primary" icon="send" disabled={!body.trim() || busy} busy={busy}>
                {w("Kirim balasan", "Send reply")}
              </Button>
            </div>
          </form>
        )}
      </section>
    </>
  );
}
