import { useEffect, useRef, useState, type FormEvent } from "react";
import { Link, useRouter } from "../../lib/router";
import { useI18n } from "../../i18n";
import {
  api,
  isAbort,
  streamTurn,
  type Entry,
  type Profile,
  type Reflection,
  type Room,
  type Turn,
} from "../api";
import { usePrefs } from "../prefs";
import { useAnnounce, useLocale, useWords } from "../shared";
import { Icon, Mark } from "../icons";
import { helpData, channelHref } from "../helpdata";
import {
  Button,
  ChoiceGroup,
  Dialog,
  EmptyState,
  ErrorNotice,
  InlineConfirm,
  LinkButton,
  Loading,
  PageHeader,
  StatusLine,
  TextArea,
  buttonClass,
} from "../ui";

const HOLD_WARNING_MS = 5 * 60_000;
const HOLD_WARNING_TICK_MS = 30_000;

// Two ways Sela responds (D-067): the normal mode talks things through and
// looks for a way forward; "listen" is for being heard, with one small step.
const intents = [
  {
    value: "discuss",
    id: "Ngobrol & cari solusi",
    en: "Talk it through",
    hintId: "Sela ikut berpikir dan menawarkan jalan keluar.",
    hintEn: "Sela thinks it through with you and offers ways forward.",
  },
  {
    value: "listen",
    id: "Didengarkan saja",
    en: "Just listen",
    hintId: "Sela fokus mendengarkan, lalu menawarkan satu langkah kecil.",
    hintEn: "Sela focuses on listening, then offers one small step.",
  },
];
const intentValue = (v: string | null) => (v === "listen" ? "listen" : "discuss");

const starters = [
  { id: "Hari ini aku merasa ", en: "Today I feel " },
  { id: "Ada yang terus kupikirkan, yaitu ", en: "Something keeps going round in my head: " },
  { id: "Aku cuma ingin didengar dulu. ", en: "I just want to be heard for now. " },
];

const COMPOSER_MAX = 3000;

export function TalkPage({ user }: { user: Profile }) {
  const { path, query } = useRouter();
  if (path === "/talk/new") return <NewRoom user={user} />;
  const id = path.startsWith("/talk/") ? path.slice("/talk/".length) : "";
  if (id) return <RoomPage id={id} />;
  const record = query.get("record");
  if (record) return <OpenSavedRoom recordId={record} />;
  return <TalkList />;
}

function OpenSavedRoom({ recordId }: { recordId: string }) {
  const w = useWords();
  const { navigate } = useRouter();
  const [error, setError] = useState<unknown>(null);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let active = true;
    const open = async () => {
      try {
        const d = await api<{ conversations: Room[] }>("/conversations");
        const live = d.conversations.find((r) => r.recordId === recordId);
        if (live) {
          navigate("/talk/" + live.id, { replace: true });
          return;
        }
        const room = await api<Room>("/conversations", {
          save: true,
          recordId,
        });
        if (active) navigate("/talk/" + room.id, { replace: true });
      } catch (e) {
        if (active) setError(e);
      }
    };
    void open();
    return () => {
      active = false;
    };
  }, [navigate, recordId, attempt]);

  return (
    <>
      <PageHeader
        back={{ to: "/talk", label: w("Cerita", "Talk") }}
        title={w("Membuka percakapan", "Opening the conversation")}
      />
      <ErrorNotice
        error={error}
        onRetry={() => {
          setError(null);
          setAttempt((n) => n + 1);
        }}
      />
      {!error && <Loading label={w("Membuka percakapan…", "Opening the conversation…")} />}
      {error && (
        <LinkButton to="/talk" variant="secondary">
          {w("Kembali ke Cerita", "Back to Talk")}
        </LinkButton>
      )}
    </>
  );
}

function TalkList() {
  const w = useWords();
  const loc = useLocale();
  const { query } = useRouter();
  const { prefs } = usePrefs();
  const [records, setRecords] = useState<Entry[] | null>(null);
  const [rooms, setRooms] = useState<Room[]>([]);
  const [error, setError] = useState<unknown>(null);

  const load = () => {
    setError(null);
    Promise.all([
      api<{ records: Entry[] }>("/records"),
      api<{ conversations: Room[] }>("/conversations"),
    ])
      .then(([r, c]) => {
        setRecords(r.records.filter((x) => x.kind === "conversation"));
        setRooms(c.conversations);
      })
      .catch((e) => {
        setError(e);
        // Keep the list unknown; an error is not an empty account.
      });
  };
  useEffect(load, []);

  const live = rooms.find((r) => !r.save) ?? rooms[0];
  const selected = query.get("record");

  return (
    <>
      <PageHeader
        title={w("Cerita", "Talk")}
        lead={w(
          "Ceritakan apa saja. Kamu yang menentukan apakah percakapan disimpan.",
          "Talk about anything. You decide whether a conversation is saved.",
        )}
        actions={
          <>
            <LinkButton
              to={
                selected ? "/talk/new?record=" + selected : "/talk/new"
              }
              variant="primary"
              icon="send"
            >
              {w("Mulai bercerita", "Start talking")}
            </LinkButton>
          </>
        }
      />
      <ErrorNotice error={error} onRetry={load} />
      {live && (
        <section className="s-section" aria-labelledby="live-title">
          <div className="s-section-title">
            <h2 id="live-title">{w("Sedang berjalan", "In progress")}</h2>
          </div>
          <div className="s-entry-list s-live-room">
            <Link to={"/talk/" + live.id}>
              <span className="s-live-room-icon" aria-hidden="true">
                <Icon name="talk" size={20} />
              </span>
              <span>
                <strong>
                  {live.save
                    ? w("Percakapan tersimpan", "Saved conversation")
                    : w("Percakapan yang sedang berjalan (tidak disimpan)", "Conversation in progress (not saved)")}
                </strong>
                <small>
                  {live.turns.length
                    ? `${live.turns.length} ${w("pesan", "messages")}`
                    : w("Belum ada pesan", "No messages yet")}
                  {live.paused ? " · " + w("balasan dijeda", "replies paused") : ""}
                </small>
              </span>
              <Icon name="arrow-right" />
            </Link>
          </div>
        </section>
      )}
      <section className="s-section" aria-labelledby="saved-title">
        <div className="s-section-title">
          <h2 id="saved-title">{w("Percakapan tersimpan", "Saved conversations")}</h2>
        </div>
        {records === null ? (
          error ? (
            <p className="s-muted">
              {w("Belum bisa memuat percakapanmu.", "Couldn't load your conversations.")}
            </p>
          ) : (
            <Loading label={w("Memuat percakapanmu…", "Loading your conversations…")} />
          )
        ) : records.length === 0 ? (
          <EmptyState
            title={w("Belum ada percakapan tersimpan.", "No saved conversations yet.")}
            body={w(
              "Percakapan yang kamu simpan akan muncul di sini.",
              "Conversations you save will appear here.",
            )}
            actions={
              <LinkButton to="/talk/new" variant="primary">
                {w("Mulai bercerita", "Start talking")}
              </LinkButton>
            }
            mark
          />
        ) : (
          <div className="s-entry-list">
            {records.map((r) => (
              <Link key={r.id} to={"/talk?record=" + r.id}>
                <span className="s-entry-date">{loc.shortDate(r.updatedAt)}</span>
                <span>
                  <strong>
                    {prefs.hidePreviews
                      ? w("Percakapan", "Conversation")
                      : r.title || w("Percakapan", "Conversation")}
                  </strong>
                  {!prefs.hidePreviews && (
                    <small>
                      {w("Terakhir aktif ", "Last active ")}
                      {loc.dateTime(r.updatedAt)}
                      {r.turns?.length
                        ? ` · ${r.turns.length} ${w("pesan", "messages")}`
                        : ""}
                    </small>
                  )}
                </span>
                <Icon name="arrow-right" />
              </Link>
            ))}
          </div>
        )}
      </section>
    </>
  );
}

function NewRoom({ user }: { user: Profile }) {
  const w = useWords();
  const { prefs, set } = usePrefs();
  const { query, navigate } = useRouter();
  const { announce } = useAnnounce();
  const recordId = query.get("record") ?? "";
  const [save, setSave] = useState<boolean | null>(
    recordId ? true : prefs.defaultStorage === "save"
      ? true
      : prefs.defaultStorage === "dont_save"
        ? false
        : null,
  );
  const [groupError, setGroupError] = useState("");
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const [existing, setExisting] = useState<Room | null>(null);

  useEffect(() => {
    api<{ conversations: Room[] }>("/conversations")
      .then((d) => {
        const room = recordId
          ? d.conversations.find((r) => r.recordId === recordId)
          : d.conversations.find((r) => !r.save);
        setExisting(room ?? null);
      })
      .catch(() => setExisting(null));
  }, [recordId]);

  const start = async () => {
    if (save === null) {
      setGroupError(
        w(
          "Pilih dulu, simpan atau jangan simpan.",
          "Choose first, save or don't save.",
        ),
      );
      return;
    }
    setGroupError("");
    setError(null);
    setBusy(true);
    try {
      if (existing) {
        navigate("/talk/" + existing.id);
        return;
      }
      const room = await api<Room>("/conversations", {
        save: recordId ? true : save,
        recordId,
      });
      set(
        "defaultStorage",
        recordId ? "save" : save ? "save" : "dont_save",
      );
      announce("status", w("Ruang cerita dibuka.", "Your space is open."));
      navigate("/talk/" + room.id);
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="s-narrow">
      <PageHeader
        back={{ to: "/talk", label: w("Cerita", "Talk") }}
        title={
          recordId
            ? w("Lanjutkan percakapanmu", "Continue your conversation")
            : w("Mulai dari mana pun.", "Start wherever you are.")
        }
        lead={w(
          "Tidak perlu memperkenalkan diri atau menjelaskan semuanya sekaligus.",
          "You don't need to introduce yourself or explain everything at once.",
        )}
      />
      <ErrorNotice error={error} />
      {existing ? (
        <div className="s-notice s-notice--info">
          <p>
            {w(
              "Percakapan ini masih terbuka di tab ini.",
              "This conversation is still open in this tab.",
            )}
          </p>
          <div className="s-actions">
            <LinkButton to={"/talk/" + existing.id} variant="primary">
              {w("Buka percakapan", "Open conversation")}
            </LinkButton>
          </div>
        </div>
      ) : (
        <>
          {!recordId && (
            <ChoiceGroup
              legend={w("Simpan percakapan ini di Catatanku?", "Save this conversation to My notes?")}
              name="storage"
              value={save === null ? null : save ? "save" : "dont_save"}
              onChange={(v) => setSave(v === "save")}
              error={groupError}
              options={[
                {
                  value: "save",
                  label: w("Simpan di Catatanku", "Save to My notes"),
                  hint: w("Kamu bisa menghapusnya kapan saja.", "You can delete it at any time."),
                },
                {
                  value: "dont_save",
                  label: w("Jangan simpan", "Don't save"),
                  hint: w(
                    "Sela membalas tanpa menyimpan. Salinan server dibuang saat kamu mengakhiri, menutup tab, atau memakai Keluar cepat, dan paling lama 30 menit setelah aktivitas terakhir.",
                    "Sela replies without keeping it. The server copy is discarded when you end it, close the tab or use Quick exit, and after at most 30 minutes of inactivity.",
                  ),
                },
              ]}
            />
          )}
          <p className="s-hint">
            {w(
              "Penyedia AI yang membalas bisa menyimpan permintaan sementara sesuai kebijakannya. Lihat Privasi & data.",
              "The AI provider that replies may keep the request briefly under its own policy. See Privacy & data.",
            )}
          </p>
          {user.kind === "guest" && (
            <p className="s-hint">
              {w(
                "Mode tamu: cookie browser ini satu-satunya kunci aksesmu.",
                "Guest mode: this browser cookie is your only access key.",
              )}
            </p>
          )}
          <div className="s-onboard-actions">
            <Button variant="primary" icon="send" onClick={() => void start()} busy={busy}>
              {w("Buka ruang cerita", "Open my space")}
            </Button>
            <LinkButton to="/home" variant="quiet">
              {w("Kembali ke beranda", "Back to Home")}
            </LinkButton>
          </div>
        </>
      )}
    </div>
  );
}

type ProviderLabel = "meta" | "test" | "unknown";
type CheckLabel = "meta_verifier" | "test" | "not_required" | "not_requested" | "unavailable" | "unknown";

function providerLabel(turn: Turn): ProviderLabel {
  const draft = turn.aiProvenance?.draftProvenance ?? turn.provenance;
  const raw = turn.aiProvenance?.generator?.provider?.toLowerCase() ?? "";
  if (draft === "test_double" || raw.includes("test")) return "test";
  if (draft === "live_meta" || raw.includes("meta")) return "meta";
  return "unknown";
}

function checkedBy(turn: Turn): CheckLabel {
  const v = turn.aiProvenance?.verification;
  if (!v) return "unknown";
  switch (v.provenance) {
    case "live_meta_verifier":
      return "meta_verifier";
    case "test_double":
      return "test";
    case "not_required":
      return "not_required";
    case "not_requested":
      return "not_requested";
    case "unavailable":
      return "unavailable";
    default:
      return "unknown";
  }
}

function authorLine(turn: Turn, w: (id: string, en: string) => string): string {
  const provider = providerLabel(turn);
  const checked = checkedBy(turn);
  if (provider === "test") return w("Sela · balasan uji (bukan AI)", "Sela · test reply (not AI)");
  const name =
    provider === "meta"
      ? "Meta"
      : w("penyedia tidak tercatat", "provider not recorded");
  const base =
    provider === "meta" && turn.provenance === "recorded_demo"
      ? w("Sela · balasan contoh terekam", "Sela · recorded example reply")
      : w("Sela · balasan AI", "Sela · AI reply") + ` (${name})`;
  if (checked === "meta_verifier") return base + w(" · diperiksa oleh model Meta pemeriksa", " · checked by the Meta verifier model");
  if (checked === "test")
    return base + w(" · diperiksa oleh program uji (bukan AI)", " · checked by a test program (not AI)");
  return base;
}

function RoomPage({ id }: { id: string }) {
  const w = useWords();
  const loc = useLocale();
  const { query, navigate } = useRouter();
  const { prefs } = usePrefs();
  const { announce } = useAnnounce();
  const [room, setRoom] = useState<Room | null>(null);
  const [loading, setLoading] = useState(true);
  const [missing, setMissing] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [loadKey, setLoadKey] = useState(0);
  const [text, setText] = useState("");
  const [intent, setIntent] = useState<string>(intentValue(query.get("intent")));
  const [pendingText, setPendingText] = useState<string | null>(null);
  const [replyDraft, setReplyDraft] = useState("");
  // Index of the first reply that arrived without a visible stream, so it can
  // settle in gently; streamed replies are already on screen.
  const [freshFrom, setFreshFrom] = useState<number | null>(null);
  const streamed = useRef(false);
  const [holdWarning, setHoldWarning] = useState(false);
  const [busy, setBusy] = useState(false);
  const [stopped, setStopped] = useState(false);
  const [ended, setEnded] = useState<"saved" | "unsaved" | null>(null);
  const [detail, setDetail] = useState(false);
  const [newContent, setNewContent] = useState("");
  const [closePanel, setClosePanel] = useState(false);
  const [reflectionBusy, setReflectionBusy] = useState(false);
  const [reflectionsOpen, setReflectionsOpen] = useState(false);
  const [correcting, setCorrecting] = useState<string | null>(null);
  const [correction, setCorrection] = useState("");

  const focusComposer = () => (document.getElementById("composer-text") as HTMLElement | null)?.focus();
  const transcript = useRef<HTMLDivElement>(null);
  const atBottom = useRef(true);
  const request = useRef<AbortController | null>(null);
  const warnedHold = useRef<number | null>(null);

  useEffect(() => {
    let active = true;
    api<Room>("/conversations/" + id)
      .then((r) => {
        if (!active) return;
        setRoom(r);
        setLoading(false);
      })
      .catch((e) => {
        if (!active) return;
        setLoading(false);
        setError(e);
        const code = (e as { code?: string }).code;
        if (code === "not_found") setMissing(true);
      });
    return () => {
      active = false;
      request.current?.abort();
    };
  }, [id, loadKey]);

  const roomReady = Boolean(room);
  useEffect(() => {
    // The composer only exists once the room has loaded. On touch screens,
    // focusing it would pop the keyboard over the conversation, so wait for
    // the person to tap it.
    if (!roomReady) return;
    const coarse = window.matchMedia?.("(pointer: coarse)").matches ?? false;
    if (!coarse) focusComposer();
  }, [id, roomReady]);

  useEffect(() => {
    if (!room?.paused) return;
    setClosePanel(false);
  }, [room?.paused]);

  // Warn once, five minutes before the server discards an unsaved hold.
  // Every room response refreshes the server's activity timer, so a new
  // holdExpiresIn moves the local deadline forward.
  useEffect(() => {
    if (!room || room.save || ended !== null || !room.holdExpiresIn) {
      setHoldWarning(false);
      return;
    }
    const deadline = Date.now() + room.holdExpiresIn * 1000;
    if (warnedHold.current === deadline) return;
    const check = () => {
      if (Date.now() >= deadline - HOLD_WARNING_MS) {
        warnedHold.current = deadline;
        setHoldWarning(true);
      }
    };
    check();
    const interval = window.setInterval(check, HOLD_WARNING_TICK_MS);
    return () => window.clearInterval(interval);
  }, [room, ended]);

  useEffect(() => {
    if (!holdWarning) return;
    announce(
      "alert",
      w(
        "Percakapan ini tidak disimpan dan akan segera dibuang dari server.",
        "This conversation isn't saved and will be discarded from the server soon.",
      ),
    );
  }, [holdWarning, announce, w]);

  useEffect(() => {
    if (!room || room.save) return;
    const onHide = () => {
      try {
        void fetch("/api/v3/conversations/" + room.id, {
          method: "DELETE",
          credentials: "same-origin",
          keepalive: true,
          headers: {
            "X-Sela-Request": "1",
            "Content-Type": "application/json",
            "X-Sela-Language":
              document.documentElement.lang === "en" ? "en" : "id",
          },
          body: "{}",
        });
      } catch {
        // the bounded hold still expires server-side
      }
    };
    window.addEventListener("pagehide", onHide);
    return () => window.removeEventListener("pagehide", onHide);
  }, [room?.id, room?.save]);

  // The room scrolls as one page with the composer pinned at the bottom, so
  // "the end of the conversation" is the end of the document.
  const scrollToBottom = () => {
    const doc = document.documentElement;
    if (typeof window.scrollTo === "function") {
      try {
        window.scrollTo({ top: doc.scrollHeight, behavior: "auto" });
      } catch {
        // jsdom and very old engines
      }
    }
    atBottom.current = true;
    setNewContent("");
  };

  useEffect(() => {
    if (!room) return;
    if (room.paused) {
      window.sessionStorage.setItem("sela.crisis", "1");
      announce("alert", w("Bantuan ditampilkan di bawah pesanmu.", "Help is shown below your message."));
    }
  }, [room?.paused, announce, w]);

  const onScroll = () => {
    const doc = document.documentElement;
    atBottom.current = window.innerHeight + window.scrollY >= doc.scrollHeight - 160;
    if (atBottom.current) setNewContent("");
  };
  useEffect(() => {
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, []);
  useEffect(() => {
    // An empty room opens at the top so its welcome and controls stay in view.
    if (!room || room.turns.length === 0) return;
    const t = window.setTimeout(scrollToBottom, 40);
    return () => window.clearTimeout(t);
  }, [room?.id]);

  // Browsers without `field-sizing: content` grow the writing box here.
  useEffect(() => {
    const el = document.getElementById("composer-text") as HTMLTextAreaElement | null;
    if (!el) return;
    if (typeof CSS !== "undefined" && CSS.supports?.("field-sizing", "content")) return;
    el.style.height = "auto";
    if (el.scrollHeight > 0) {
      el.style.height = `${Math.min(el.scrollHeight + 2, Math.round(window.innerHeight * 0.4))}px`;
    }
  }, [text, roomReady]);

  const lastAssistant = (r: Room | null) =>
    r?.turns.filter((t) => t.role === "assistant").at(-1);

  // A failed send puts the words back, unless the person has already started
  // writing something new while waiting.
  const restoreDraft = (value: string) =>
    setText((current) => (current.trim() ? current : value));

  const send = async (e?: FormEvent) => {
    e?.preventDefault();
    if (!room || !text.trim() || busy) return;
    const value = text.trim();
    setError(null);
    setStopped(false);
    setText("");
    setPendingText(value);
    setReplyDraft("");
    streamed.current = false;
    window.setTimeout(scrollToBottom, 30);
    setBusy(true);
    const ac = new AbortController();
    request.current = ac;
    announce("status", w("Sela sedang menulis…", "Sela is writing…"));
    try {
      let next: Room;
      try {
        next = await streamTurn(
          "/conversations/" + room.id + "/turns/stream",
          { text: value, expectedVersion: room.version, intent },
          (event) => {
            if (event.type !== "chunk") return;
            streamed.current = true;
            setReplyDraft((draft) => draft + event.text);
            if (atBottom.current) window.requestAnimationFrame(scrollToBottom);
          },
          ac.signal,
          110_000,
        );
      } catch (streamError) {
        if (isAbort(streamError)) throw streamError;
        // An older server, a proxy or a dropped stream: keep the JSON route
        // as the compatibility path. The version check prevents a retry from
        // duplicating a turn the stream already stored on the server.
        setReplyDraft("");
        try {
          next = await api<Room>(
            "/conversations/" + room.id + "/turns",
            { text: value, expectedVersion: room.version, intent },
            "POST",
            ac.signal,
            110_000,
          );
        } catch (fallbackError) {
          if ((fallbackError as { code?: string }).code === "stale_version") {
            // The stream may have stored the turn before its done frame was
            // lost. Adopt the latest room only when this exact turn is there;
            // otherwise keep the writing and report the conflict.
            const latest = await api<Room>("/conversations/" + room.id);
            if (!latest.turns.some((turn) => turn.role === "user" && turn.text === value)) {
              throw fallbackError;
            }
            next = latest;
          } else {
            throw fallbackError;
          }
        }
      }
      setFreshFrom(streamed.current ? null : room.turns.length + 1);
      const previousLast = lastAssistant(room);
      const nextLast = lastAssistant(next);
      setReplyDraft("");
      setRoom(next);
      setPendingText(null);
      if (next.paused) {
        // announced by the paused effect
      } else if (nextLast && nextLast !== previousLast) {
        if (prefs.announceReplies) {
          announce("reply", `Sela: ${nextLast.text}`);
        } else {
          announce("status", w("Balasan selesai.", "Reply finished."));
        }
        if (next.aiState === "unavailable" || next.aiState === "not_consented") {
          announce(
            "status",
            w("Sela belum bisa membalas sekarang.", "Sela can't reply right now."),
          );
        }
      } else if (next.aiState === "not_consented") {
        announce(
          "status",
          w("Balasan AI tidak diizinkan.", "AI replies aren't allowed."),
        );
      } else if (next.aiState === "budget_limited") {
        announce(
          "status",
          w("Batas anggaran AI bulan ini tercapai.", "This month's AI budget is reached."),
        );
      } else if (next.aiState === "unavailable" || next.aiState === "rate_limited") {
        announce("status", w("Sela belum bisa membalas sekarang.", "Sela can't reply right now."));
      }
      if (!atBottom.current) {
        setNewContent(
          next.paused
            ? w("Bantuan ditampilkan di bawah pesanmu", "Help is shown below your message")
            : w("Pesan baru dari Sela", "New message from Sela"),
        );
      } else {
        window.setTimeout(scrollToBottom, 30);
      }
    } catch (e) {
      if (!isAbort(e)) {
        const code = (e as { code?: string }).code;
        if (code === "timeout") {
          try {
            const latest = await api<Room>("/conversations/" + room.id);
            setRoom(latest);
            if (
              !latest.turns.some(
                (t) => t.role === "user" && t.text === value,
              )
            ) {
              restoreDraft(value);
            }
            setError(e);
            announce(
              "status",
              w(
                "Koneksi terlalu lama. Periksa percakapan sebelum mengirim lagi.",
                "The connection took too long. Check the conversation before sending again.",
              ),
            );
          } catch {
            setError(e);
            restoreDraft(value);
          }
        } else {
          setError(e);
          restoreDraft(value);
          announce("status", w("Belum terkirim. Tulisanmu masih di sini.", "Not sent. Your writing is still here."));
        }
      }
    } finally {
      request.current = null;
      setBusy(false);
      setPendingText(null);
      setReplyDraft("");
      focusComposer();
    }
  };

  const stop = async () => {
    if (!room) return;
    try {
      const next = await api<Room>("/conversations/" + room.id + "/operation", undefined, "DELETE");
      setStopped(true);
      // The cancelled send answers with the settled room; keep whichever is newer.
      setRoom((current) => (current && current.version >= next.version ? current : next));
      announce("status", w("Balasan dihentikan sebelum ditampilkan.", "Reply stopped before it was shown."));
      focusComposer();
    } catch {
      // Cancelling is idempotent; the reply may already have arrived.
    }
  };

  const changeStorage = async (nextSave: boolean) => {
    if (!room) return;
    setError(null);
    try {
      const next = await api<Room>(
        "/conversations/" + room.id + "/storage",
        { save: nextSave, expectedVersion: room.version },
        "PATCH",
      );
      setRoom(next);
      announce(
        "status",
        nextSave
          ? w("Mulai sekarang percakapan ini disimpan.", "From now on this conversation is saved.")
          : w("Mulai sekarang percakapan ini tidak disimpan.", "From now on this conversation isn't saved."),
      );
    } catch (e) {
      setError(e);
    }
  };

  const resume = async () => {
    if (!room) return;
    try {
      const next = await api<Room>(
        "/conversations/" + room.id + "/resume",
        { expectedVersion: room.version },
      );
      setRoom(next);
      try {
        window.sessionStorage.removeItem("sela.crisis");
      } catch {
        // non-sensitive flag only
      }
      announce("status", w("Balasan Sela dilanjutkan.", "Sela's replies are resumed."));
    } catch (e) {
      setError(e);
    }
  };

  const captureReflections = async () => {
    if (!room || reflectionBusy) return;
    setReflectionBusy(true);
    setError(null);
    try {
      const next = await api<Room>(
        `/conversations/${room.id}/reflections`,
        { expectedVersion: room.version },
        "POST",
        undefined,
        110_000,
      );
      setRoom(next);
      setReflectionsOpen(true);
      announce(
        "status",
        w(
          "Sela menangkap beberapa hal. Periksa dan koreksi jika perlu.",
          "Sela noticed a few things. Review and correct them if needed.",
        ),
      );
    } catch (e) {
      setError(e);
    } finally {
      setReflectionBusy(false);
    }
  };

  const reviewReflection = async (reflection: Reflection, action: "confirm" | "correct" | "reject" | "undo", text?: string) => {
    if (!room) return;
    setReflectionBusy(true);
    setError(null);
    try {
      const next = await api<Room>(
        `/conversations/${room.id}/reflections/${reflection.id}`,
        { expectedVersion: room.version, action, text },
        "PATCH",
      );
      setRoom(next);
      setCorrecting(null);
      setCorrection("");
      announce(
        "status",
        action === "confirm"
          ? w("Tafsiran dikonfirmasi.", "Interpretation confirmed.")
          : action === "correct"
            ? w("Tafsiran diganti dengan kata-katamu.", "Interpretation replaced with your words.")
            : action === "reject"
              ? w("Tafsiran ditolak.", "Interpretation rejected.")
              : w("Tafsiran dikembalikan untuk diperiksa.", "Interpretation reopened for review."),
      );
    } catch (e) {
      setError(e);
    } finally {
      setReflectionBusy(false);
    }
  };

  const end = async () => {
    if (!room) return;
    request.current?.abort();
    try {
      const wasSaved = room.save;
      await api("/conversations/" + room.id, undefined, "DELETE");
      try {
        window.sessionStorage.removeItem("sela.crisis");
      } catch {
        // non-sensitive flag only
      }
      setEnded(wasSaved ? "saved" : "unsaved");
      setPendingText(null);
      announce(
        "status",
        wasSaved
          ? w("Percakapan selesai dan tersimpan.", "Conversation ended and saved.")
          : w("Percakapan selesai. Tidak disimpan.", "Conversation ended. Not saved."),
      );
      requestAnimationFrame(() => {
        document.getElementById("closing-title")?.focus();
      });
    } catch (e) {
      setError(e);
    }
  };

  // GET refreshes the server's activity timer for this hold; the room's new
  // holdExpiresIn moves the local warning deadline forward.
  const keepHoldOpen = async () => {
    if (!room) return;
    setHoldWarning(false);
    try {
      const next = await api<Room>("/conversations/" + room.id);
      setRoom(next);
      announce(
        "status",
        w(
          "Percakapan tetap terbuka.",
          "The conversation stays open.",
        ),
      );
    } catch (e) {
      if ((e as { code?: string }).code === "not_found") {
        setMissing(true);
      } else {
        setError(e);
      }
    }
  };

  const endFromWarning = async () => {
    setHoldWarning(false);
    await end();
  };

  const deleteConversation = async () => {
    if (!room?.recordId) return;
    try {
      const record = await api<Entry>("/records/" + room.recordId);
      await api(
        "/records/" + room.recordId,
        { expectedVersion: record.version },
        "DELETE",
      );
      await api("/conversations/" + room.id, undefined, "DELETE");
      announce("status", w("Percakapan dihapus.", "Conversation deleted."));
      navigate("/talk");
    } catch (e) {
      setError(e);
    }
  };

  if (loading) {
    return (
      <>
        <PageHeader title={w("Percakapan", "Conversation")} />
        <Loading label={w("Membuka percakapan…", "Opening the conversation…")} />
      </>
    );
  }
  if (!room && error && !missing) {
    return (
      <>
        <PageHeader title={w("Percakapan", "Conversation")} />
        <ErrorNotice
          error={error}
          title={w("Belum bisa membuka percakapan", "Couldn't open the conversation")}
          onRetry={() => {
            setError(null);
            setLoading(true);
            setLoadKey((n) => n + 1);
          }}
        />
        <LinkButton to="/talk" variant="secondary">
          {w("Kembali ke Cerita", "Back to Talk")}
        </LinkButton>
      </>
    );
  }
  if (missing || !room) {
    return (
      <>
        <PageHeader title={w("Percakapan", "Conversation")} />
        <EmptyState
          title={w("Percakapan ini sudah berakhir.", "This conversation has ended.")}
          body={w(
            "Percakapan yang tidak disimpan dibuang saat tab ditutup, percakapan baru dimulai, atau Keluar cepat dipakai.",
            "Unsaved conversations are discarded when the tab closes, a new conversation starts, or Quick exit is used.",
          )}
          actions={
            <>
              <LinkButton to="/talk/new" variant="primary">
                {w("Mulai percakapan baru", "Start a new conversation")}
              </LinkButton>
              <LinkButton to="/talk" variant="secondary">
                {w("Ke Cerita", "Go to Talk")}
              </LinkButton>
            </>
          }
          mark
        />
      </>
    );
  }

  const paused = room.paused;
  const showEnd = ended !== null;
  const lastUserIndex = room.turns.map((t) => t.role).lastIndexOf("user");
  const services = helpData.help.slice(0, 2);

  return (
    <>
      <div className="s-room-toolbar">
        <h1 id="page-title" className="sr-only">
          {w("Ruang cerita", "A quiet room")}
        </h1>
        <div className="s-room-topline">
          <Link to="/talk" className="s-back">
            <Icon name="arrow-left" size={18} />
            {w("Cerita", "Talk")}
          </Link>
          <span
            className={
              "s-room-status " +
              (room.save ? "s-room-status--saved" : "s-room-status--unsaved")
            }
          >
            <Icon name={room.save ? "bookmark" : "bookmark-x"} size={16} />
            {room.save ? w("Disimpan", "Saved") : w("Tidak disimpan", "Not saved")}
          </span>
          {!showEnd && (
            <Button
              variant="secondary"
              className="s-room-end"
              aria-label={w("Akhiri percakapan", "End conversation")}
              onClick={() => void end()}
            >
              <span className="s-room-end-long">{w("Akhiri percakapan", "End conversation")}</span>
              <span className="s-room-end-short">{w("Akhiri", "End")}</span>
            </Button>
          )}
        </div>
        {!showEnd && (
          <div className="s-room-mode">
            <div
              className="s-room-modes"
              role="radiogroup"
              aria-label={w("Cara Sela merespons", "How Sela responds")}
            >
              {intents.map((i) => (
                <label
                  key={i.value}
                  className={"s-room-mode-option" + (intent === i.value ? " is-selected" : "")}
                  title={w(i.hintId, i.hintEn)}
                >
                  <input
                    type="radio"
                    name="room-mode"
                    value={i.value}
                    checked={intent === i.value}
                    onChange={() => {
                      setIntent(i.value);
                      announce("status", w(i.hintId, i.hintEn));
                    }}
                  />
                  <span>{w(i.id, i.en)}</span>
                </label>
              ))}
            </div>
            <button
              type="button"
              className="s-text-button s-room-adjust"
              aria-label={w("Atur percakapan", "Conversation settings")}
              onClick={() => setDetail(true)}
            >
              <Icon name="settings" size={16} />
              {w("Atur", "Adjust")}
            </button>
          </div>
        )}
      </div>

      <ErrorNotice error={error} />

      <div className="s-room-layout">
        <div>
          <section aria-labelledby="transcript-title">
            <h2 id="transcript-title" className="sr-only">
              {w("Isi percakapan", "Conversation")}
            </h2>
            <div
              className="s-transcript"
              role="log"
              aria-live="off"
              ref={transcript}
              onScroll={onScroll}
            >
              {room.turns.length === 0 && pendingText === null && !showEnd && (
                <div className="s-room-empty">
                  <span className="s-room-empty-mark" aria-hidden="true">
                    <Mark size={44} />
                  </span>
                  <p className="s-room-empty-title">{w("Ruang ini milikmu.", "This space is yours.")}</p>
                  <p className="s-hint">
                    {w(
                      "Tidak perlu rapi. Sela adalah AI, bukan psikolog.",
                      "It doesn't need to be tidy. Sela is an AI, not a psychologist.",
                    )}
                  </p>
                  <div className="s-room-starters" role="group" aria-label={w("Contoh awal kalimat", "Ways to begin")}>
                    {starters.map((s) => (
                      <button
                        key={s.en}
                        type="button"
                        className="s-room-starter"
                        onClick={() => {
                          const value = w(s.id, s.en);
                          setText((current) => (current.trim() ? current : value));
                          requestAnimationFrame(() => {
                            const el = document.getElementById("composer-text") as HTMLTextAreaElement | null;
                            if (!el) return;
                            el.focus();
                            el.setSelectionRange(el.value.length, el.value.length);
                          });
                        }}
                      >
                        {w(s.id, s.en).replace(/[\s.,:]+$/, "")}…
                      </button>
                    ))}
                  </div>
                </div>
              )}
              {room.turns.map((turn, index) => (
                <div key={index} className="s-turn-row">
                  {turn.role === "user" ? (
                    <article className="s-turn s-turn--user">
                      <p className="s-turn-author">
                        <Icon name="quote" size={16} />
                        {w("Kamu · kata-katamu", "You · your words")}
                      </p>
                      <p className="s-turn-body">{turn.text}</p>
                      <p className="s-turn-meta">
                        <time dateTime={turn.createdAt}>{loc.dateTime(turn.createdAt)}</time>
                      </p>
                    </article>
                  ) : (
                    <article
                      className="s-turn s-turn--sela"
                      data-fresh={freshFrom !== null && index >= freshFrom ? "true" : undefined}
                    >
                      <p className="s-turn-author">
                        <span className="s-ai-mono" aria-hidden="true">
                          {"AI"}
                        </span>
                        <span>{authorLine(turn, w)}</span>
                      </p>
                      <ReplyText text={turn.text} />
                      <div className="s-turn-meta">
                        <time dateTime={turn.createdAt}>{loc.dateTime(turn.createdAt)}</time>
                        <details>
                          <summary>{w("Tentang balasan ini", "About this reply")}</summary>
                          <dl className="s-dl">
                            <div>
                              <dt>{w("Penyedia", "Provider")}</dt>
                              <dd>
                                {providerLabel(turn) === "test"
                                  ? w("Program uji (bukan AI)", "Test program (not AI)")
                                  : providerLabel(turn) === "meta"
                                    ? "Meta"
                                    : w("Tidak tercatat", "Not recorded")}
                              </dd>
                            </div>
                            <div>
                              <dt>{w("Model", "Model")}</dt>
                              <dd>{turn.model || turn.aiProvenance?.generator?.servedModel || "—"}</dd>
                            </div>
                            <div>
                              <dt>{w("Pemeriksaan", "Check")}</dt>
                              <dd>
                                {checkedBy(turn) === "meta_verifier"
                                  ? w("Diperiksa oleh model Meta pemeriksa", "Checked by the Meta verifier model")
                                  : checkedBy(turn) === "test"
                                    ? w("Diperiksa oleh program uji (bukan AI)", "Checked by a test program (not AI)")
                                      : checkedBy(turn) === "not_required"
                                        ? w("Tidak perlu diperiksa", "No check was needed")
                                        : checkedBy(turn) === "unavailable"
                                          ? w("Pemeriksaan tidak tersedia", "The check was unavailable")
                                          : w("Pemeriksaan tidak tercatat", "No check is recorded")}
                              </dd>
                            </div>
                            <div>
                              <dt>{w("Waktu", "Time")}</dt>
                              <dd>{loc.dateTime(turn.createdAt)}</dd>
                            </div>
                          </dl>
                        </details>
                      </div>
                    </article>
                  )}
                  {paused && index === lastUserIndex && (
                    <CrisisPanel
                      onClose={() => {
                        setClosePanel(true);
                        announce(
                          "status",
                          w(
                            "Bantuan ditutup. Balasan Sela tetap dijeda.",
                            "Help closed. Sela's replies stay paused.",
                          ),
                        );
                        focusComposer();
                      }}
                      onResume={() => void resume()}
                      onEnd={() => void end()}
                      hidden={closePanel}
                      services={services}
                    />
                  )}
                </div>
              ))}

              {pendingText !== null && (
                <article className="s-turn s-turn--user s-turn--pending">
                  <p className="s-turn-author">
                    <Icon name="quote" size={16} />
                    {w("Kamu · kata-katamu", "You · your words")}
                  </p>
                  <p className="s-turn-body">{pendingText}</p>
                  <p className="s-turn-meta">{w("Mengirim…", "Sending…")}</p>
                </article>
              )}
              {pendingText !== null && replyDraft === "" && !paused && (
                <p className="s-pending">
                  <span className="s-typing" aria-hidden="true">
                    <span />
                    <span />
                    <span />
                  </span>
                  {w("Sela sedang menulis…", "Sela is writing…")}
                </p>
              )}
              {replyDraft !== "" && (
                <article className="s-turn s-turn--sela" data-streaming="true">
                  <p className="s-turn-author">
                    <span className="s-ai-mono" aria-hidden="true">
                      {"AI"}
                    </span>
                    <span>{w("Sela · balasan AI", "Sela · AI reply")}</span>
                  </p>
                  <ReplyText text={replyDraft} />
                  <p className="s-turn-meta">
                    {w("Sedang menulis…", "Writing…")}
                  </p>
                </article>
              )}

              {showEnd && (
                <section className="s-closing" aria-labelledby="closing-title">
                  <h2 id="closing-title" tabIndex={-1}>
                    {w("Percakapan selesai.", "Conversation ended.")}
                  </h2>
                  <p>
                    {ended === "saved"
                      ? w("Tersimpan di Catatanku.", "Saved to My notes.")
                      : w(
                          "Percakapan ini tidak disimpan. Salinannya sudah dibuang dari server Sela saat kamu mengakhiri; yang tersisa hanya di tab ini sampai kamu meninggalkan halaman. Kamu masih bisa membuka Bantuan sekarang tanpa kehilangan tampilan ini.",
                          "It isn't saved. Its server copy was discarded when you ended it; what remains is only in this tab until you leave the page. You can still open Help now without losing this view.",
                        )}
                  </p>
                  <p className="s-hint">
                    {w(
                      "Penyedia AI memproses tulisanmu untuk membalas dan bisa menyimpannya sementara sesuai kebijakannya; lihat Privasi & data.",
                      "The AI provider processes your writing to reply and may keep it briefly under its policy; see Privacy & data.",
                    )}
                  </p>
                  <div className="s-actions">
                    <LinkButton to="/notes/write" variant="secondary">
                      {w("Tulis catatan", "Write a note")}
                    </LinkButton>
                    <LinkButton to="/notes/check-in" variant="secondary">
                      {w("Cek perasaan", "Check in")}
                    </LinkButton>
                    <LinkButton to="/home" variant="quiet">
                      {w("Kembali ke beranda", "Back to Home")}
                    </LinkButton>
                  </div>
                </section>
              )}
            </div>
          </section>

          {!showEnd && room.turns.length > 0 && (
            <details
              className="s-reflections"
              open={reflectionsOpen}
              onToggle={(e) => setReflectionsOpen((e.currentTarget as HTMLDetailsElement).open)}
            >
              <summary>
                <Icon name="interpretation" size={18} />
                <span id="reflections-title">{w("Yang Sela tangkap", "What Sela noticed")}</span>
                {(room.reflections?.length ?? 0) > 0 && (
                  <span className="s-reflections-count">
                    {room.reflections?.length}
                  </span>
                )}
                <Icon name="chevron-down" size={18} />
              </summary>
              <div className="s-reflections-actions">
                <Button
                  variant="secondary"
                  disabled={reflectionBusy || busy}
                  onClick={() => void captureReflections()}
                >
                  {reflectionBusy
                    ? w("Membaca ulang…", "Reading back…")
                    : w("Tangkap dari percakapan ini", "Notice something from this conversation")}
                </Button>
              </div>
              <p className="s-hint">
                {w(
                  "Ini tafsiran AI, bukan fakta. Kutipan di bawah adalah kata-katamu; konfirmasi, koreksi, atau tolak.",
                  "This is an AI interpretation, not a fact. The quote below is your own wording; confirm, correct or reject it.",
                )}
              </p>
              {(room.reflections ?? []).map((reflection) => (
                <article className="s-consent" key={reflection.id}>
                  <blockquote className="s-reflection-quote">“{reflection.quote}”</blockquote>
                  {correcting === reflection.id ? (
                    <TextArea
                      label={w("Kata-katamu sendiri", "Your own words")}
                      value={correction}
                      onChange={(e) => setCorrection(e.target.value)}
                      rows={3}
                    />
                  ) : (
                    <p>{reflection.text}</p>
                  )}
                  <p className="s-hint">
                    {reflection.reviewStatus === "confirmed"
                      ? w("Kamu konfirmasi.", "You confirmed this.")
                      : reflection.reviewStatus === "corrected"
                        ? w("Kamu koreksi.", "You corrected this.")
                        : reflection.reviewStatus === "rejected"
                          ? w("Kamu tolak.", "You rejected this.")
                          : w("Belum diperiksa.", "Not reviewed yet.")}
                  </p>
                  <div className="s-actions">
                    {correcting === reflection.id ? (
                      <>
                        <Button
                          variant="primary"
                          disabled={reflectionBusy || correction.trim() === ""}
                          onClick={() => void reviewReflection(reflection, "correct", correction)}
                        >
                          {w("Simpan koreksi", "Save correction")}
                        </Button>
                        <Button
                          variant="quiet"
                          disabled={reflectionBusy}
                          onClick={() => {
                            setCorrecting(null);
                            setCorrection("");
                          }}
                        >
                          {w("Batal", "Cancel")}
                        </Button>
                      </>
                    ) : (
                      <>
                        <Button
                          variant="secondary"
                          disabled={reflectionBusy}
                          onClick={() => void reviewReflection(reflection, "confirm")}
                        >
                          {w("Setuju", "Agree")}
                        </Button>
                        <Button
                          variant="secondary"
                          disabled={reflectionBusy}
                          onClick={() => {
                            setCorrecting(reflection.id);
                            setCorrection(reflection.text);
                          }}
                        >
                          {w("Koreksi", "Correct")}
                        </Button>
                        <Button
                          variant="quiet"
                          disabled={reflectionBusy}
                          onClick={() =>
                            void reviewReflection(
                              reflection,
                              reflection.reviewStatus === "rejected" ? "undo" : "reject",
                            )
                          }
                        >
                          {reflection.reviewStatus === "rejected"
                            ? w("Batalkan tolak", "Undo rejection")
                            : w("Bukan ini", "Not this")}
                        </Button>
                      </>
                    )}
                  </div>
                </article>
              ))}
            </details>
          )}

          {!showEnd && (
            <div className="s-room-endbar">
              {paused && (
                <div className="s-pause-line">
                  <Icon name="pause" size={18} />
                  <strong>
                    {w(
                      "Balasan Sela dijeda di percakapan ini. Kamu tetap bisa menulis.",
                      "Sela's replies are paused in this conversation. You can still write.",
                    )}
                  </strong>
                  {!closePanel && room.turns.length > 0 && (
                    <button
                      type="button"
                      className={buttonClass("quiet")}
                      onClick={() => {
                        const heading = document.getElementById("crisis-title");
                        heading?.setAttribute("tabindex", "-1");
                        heading?.focus();
                      }}
                    >
                      {w("Ke bantuan", "Go to help")}
                    </button>
                  )}
                  {closePanel && (
                    <Link to="/help" className={buttonClass("quiet")}>
                      {w("Lihat semua bantuan", "See all help")}
                    </Link>
                  )}
                  <button
                    type="button"
                    className={buttonClass("quiet")}
                    onClick={() => void resume()}
                  >
                    {w("Lanjutkan bercerita dengan Sela", "Keep talking with Sela")}
                  </button>
                </div>
              )}
              {lastAssistant(room) && (
                <button
                  type="button"
                  className={buttonClass("quiet", "s-jump-reply")}
                  onClick={() => {
                    const articles = transcript.current?.querySelectorAll(".s-turn--sela");
                    const last = articles?.[articles.length - 1] as HTMLElement | undefined;
                    if (last) {
                      last.tabIndex = -1;
                      last.scrollIntoView({ block: "center" });
                      last.focus();
                    }
                  }}
                >
                  {w("Ke balasan terakhir", "Go to the last reply")}
                </button>
              )}
            </div>
          )}

          {!showEnd &&
            !busy &&
            [
              "not_consented",
              "unavailable",
              "rate_limited",
              "budget_limited",
              "storage_unavailable",
            ].includes(
              room.aiState,
            ) && (
              <div className="s-room-ai-state">
                <Icon name="info" size={18} />
                <p>
                  <strong>
                    {room.aiState === "not_consented"
                      ? w("Balasan AI tidak diizinkan.", "AI replies aren't allowed.")
                      : w("Sela belum bisa membalas sekarang.", "Sela can't reply right now.")}
                  </strong>{" "}
                  {room.aiState === "not_consented"
                    ? w(
                        "Tulisanmu tetap di sini; kamu bisa terus menulis.",
                        "Your writing stays here; you can keep writing.",
                      )
                    : room.aiState === "rate_limited"
                      ? w(
                          "Batas balasan AI tercapai. Tulisanmu tetap di sini.",
                          "The AI reply limit is reached. Your writing stays here.",
                        )
                      : room.aiState === "budget_limited"
                        ? w(
                            "Batas anggaran AI bulan ini tercapai. Tulisanmu tetap di sini.",
                            "This month's AI budget is reached. Your writing stays here.",
                          )
                        : w(
                          "Tulisanmu tetap di sini. Coba lagi nanti, atau terus menulis.",
                          "Your writing stays here. Try again later, or keep writing.",
                        )}
                </p>
                <Link to="/settings/privacy#ai" className="s-text-button">
                  {w("Pengaturan AI", "AI settings")}
                </Link>
              </div>
            )}
          {!showEnd && (
            <>
              <form className="s-composer" id="composer" onSubmit={(e) => void send(e)}>
                {newContent && (
                  <button
                    type="button"
                    className="s-new-content"
                    onClick={() => {
                      if (paused) {
                        const heading = document.getElementById("crisis-title");
                        heading?.scrollIntoView({ block: "center" });
                        heading?.focus();
                      } else {
                        const articles = transcript.current?.querySelectorAll<HTMLElement>(
                          ".s-turn--sela",
                        );
                        const last = articles?.[articles.length - 1];
                        if (last) {
                          last.tabIndex = -1;
                          last.scrollIntoView({ block: "center" });
                          last.focus();
                        }
                      }
                      setNewContent("");
                    }}
                  >
                    {newContent} <span aria-hidden="true">↓</span>
                  </button>
                )}
                <div className="s-composer-box">
                  <TextArea
                    id="composer-text"
                    label={w("Tulis untuk Sela", "Write to Sela")}
                    hideLabel
                    describedBy="composer-hint"
                    placeholder={w("Tulis untuk Sela…", "Write to Sela…")}
                    rows={1}
                    value={text}
                    maxLength={COMPOSER_MAX}
                    onChange={(e) => setText(e.target.value)}
                    onKeyDown={(e) => {
                      // Never send while an input method is still composing a word.
                      if (e.nativeEvent.isComposing || e.keyCode === 229) return;
                      if (
                        prefs.enterToSend
                          ? e.key === "Enter" && !e.shiftKey
                          : e.key === "Enter" && (e.metaKey || e.ctrlKey)
                      ) {
                        e.preventDefault();
                        void send();
                      }
                    }}
                  />
                  <div className="s-composer-actions">
                    {busy ? (
                      <Button variant="secondary" icon="stop" onClick={() => void stop()}>
                        {w("Hentikan balasan", "Stop reply")}
                      </Button>
                    ) : (
                      <Button
                        type="submit"
                        variant="primary"
                        icon="send"
                        disabled={!text.trim()}
                      >
                        {w("Kirim", "Send")}
                      </Button>
                    )}
                  </div>
                </div>
                <div className="s-composer-foot">
                  <p className={"s-hint" + (busy ? "" : " s-composer-keys")} id="composer-hint">
                    {busy
                      ? w(
                          "Tunggu balasan Sela, atau hentikan dulu. Kamu masih bisa menulis.",
                          "Wait for Sela's reply, or stop it first. You can still write.",
                        )
                      : prefs.enterToSend
                        ? w(
                            "Enter untuk mengirim, Shift+Enter untuk baris baru.",
                            "Enter to send, Shift+Enter for a new line.",
                          )
                        : w(
                            "Ctrl atau Cmd+Enter untuk mengirim.",
                            "Ctrl or Cmd+Enter to send.",
                          )}
                  </p>
                  {text.length > COMPOSER_MAX * 0.8 && (
                    <p
                      className={
                        "s-composer-count" +
                        (text.length >= COMPOSER_MAX ? " s-composer-count--full" : "")
                      }
                    >
                      {text.length}/{COMPOSER_MAX}
                    </p>
                  )}
                </div>
                {stopped && (
                  <StatusLine>
                    {w(
                      "Balasan dihentikan sebelum ditampilkan.",
                      "Reply stopped before it was shown.",
                    )}
                  </StatusLine>
                )}
              </form>
            </>
          )}
        </div>

        <aside className="s-room-detail-panel" aria-label={w("Detail percakapan", "Conversation details")}>
          <h2>{w("Detail", "Detail")}</h2>
          <p className="s-small">
            {room.save
              ? w("Disimpan di Catatanku.", "Saved to My notes.")
              : w("Tidak disimpan.", "Not saved.")}
          </p>
          <p className="s-small s-muted">
            {w(
              "Konteks pilihanmu ada di Yang Sela ingat. Saat izin balasan AI aktif, memori yang dikonfirmasi bisa dipakai untuk membalas.",
              "Your chosen context is in What Sela remembers. While AI reply consent is on, confirmed memory may be used to reply.",
            )}
          </p>
          <LinkButton to="/help" variant="help" icon="help">
            {w("Butuh bantuan sekarang", "Need help now")}
          </LinkButton>
          {room.recordId && (
            <InlineConfirm
              label={w("Hapus percakapan", "Delete conversation")}
              question={w(
                "Hapus percakapan ini dan semua versinya?",
                "Delete this conversation and every version?",
              )}
              confirmLabel={w("Hapus percakapan", "Delete conversation")}
              onConfirm={() => void deleteConversation()}
            />
          )}
          {!showEnd && (
            <Button variant="secondary" onClick={() => void end()}>
              {w("Akhiri percakapan", "End conversation")}
            </Button>
          )}
          {paused && (
            <Button variant="secondary" onClick={() => void resume()}>
              {w("Lanjutkan bercerita dengan Sela", "Keep talking with Sela")}
            </Button>
          )}
        </aside>
      </div>

      <Dialog
        open={holdWarning}
        onClose={() => setHoldWarning(false)}
        title={w(
          "Percakapan akan segera dibuang",
          "This conversation will be discarded soon",
        )}
        alert
      >
        <p>
          {w(
            "Percakapan ini tidak disimpan. Salinan sementara di server Sela akan dibuang sekitar lima menit lagi. Kamu bisa tetap membukanya atau mengakhirinya sekarang.",
            "This conversation isn't saved. The temporary copy on Sela's server will be discarded in about five minutes. You can keep it open or end it now.",
          )}
        </p>
        <div className="s-actions">
          <Button variant="primary" onClick={() => void keepHoldOpen()}>
            {w("Tetap buka", "Keep it open")}
          </Button>
          <Button variant="secondary" onClick={() => void endFromWarning()}>
            {w("Akhiri sekarang", "End now")}
          </Button>
        </div>
      </Dialog>

      <Dialog
        open={detail}
        onClose={() => setDetail(false)}
        title={w("Detail percakapan", "Conversation details")}
        sheet
      >
        <div className="s-room-detail">
          <section>
            <h3>{w("Cara Sela merespons", "How Sela responds")}</h3>
            <ChoiceGroup
              legend={w("Saat ini kamu ingin…", "Right now you'd like…")}
              name="intent-detail"
              value={intent}
              onChange={(v) => setIntent(intentValue(v))}
              options={intents.map((i) => ({
                value: i.value,
                label: w(i.id, i.en),
                hint: w(i.hintId, i.hintEn),
              }))}
            />
          </section>
          <section>
            <h3>{w("Penyimpanan", "Storage")}</h3>
            <p className="s-small">
              {room.save
                ? w("Percakapan ini disimpan di Catatanku.", "This conversation is saved to My notes.")
                : w("Percakapan ini tidak disimpan.", "This conversation isn't saved.")}
            </p>
            {room.save ? (
              <InlineConfirm
                label={w("Berhenti menyimpan", "Stop saving")}
                question={w(
                  "Hapus juga yang sudah tersimpan dari percakapan ini?",
                  "Also delete what's already saved from this conversation?",
                )}
                confirmLabel={w("Hapus salinan tersimpan", "Delete the saved copy")}
                onConfirm={() => void changeStorage(false)}
              />
            ) : (
              <Button variant="secondary" onClick={() => void changeStorage(true)}>
                {w("Simpan percakapan ini", "Save this conversation")}
              </Button>
            )}
          </section>
          <section>
            <h3>{w("Yang Sela ingat", "What Sela remembers")}</h3>
            <p className="s-small s-muted">
              {w(
                "Belum tersedia di versi ini. Kamu tetap bisa menyimpan dan memeriksa catatanmu.",
                "Not available in this build yet. You can still save and review your notes.",
              )}
            </p>
          </section>
          <section>
            <h3>{w("Bantuan", "Help")}</h3>
            <LinkButton to="/help" variant="help" icon="help">
              {w("Butuh bantuan sekarang", "Need help now")}
            </LinkButton>
          </section>
          <section>
            <h3>{w("Akhiri atau hapus", "End or delete")}</h3>
            {!showEnd && (
              <Button variant="secondary" onClick={() => void end()}>
                {w("Akhiri percakapan", "End conversation")}
              </Button>
            )}
            {room.recordId && (
              <InlineConfirm
                label={w("Hapus percakapan", "Delete conversation")}
                question={w(
                  "Hapus percakapan ini dan semua versinya?",
                  "Delete this conversation and every version?",
                )}
                confirmLabel={w("Hapus percakapan", "Delete conversation")}
                onConfirm={() => void deleteConversation()}
              />
            )}
          </section>
        </div>
      </Dialog>
    </>
  );
}

// Sela's replies arrive as plain text; lines become paragraphs and lines that
// start with a bullet become a list, so a longer reply stays easy to read.
function ReplyText({ text }: { text: string }) {
  const blocks: Array<{ kind: "p"; text: string } | { kind: "ul"; items: string[] }> = [];
  for (const raw of text.split("\n")) {
    const line = raw.trim();
    if (!line) continue;
    const bullet = /^(?:[•\-*]|\d+[.)])\s+(.*)$/.exec(line);
    if (bullet) {
      const item = String(bullet[1] ?? "");
      const last = blocks[blocks.length - 1];
      if (last?.kind === "ul") last.items.push(item);
      else blocks.push({ kind: "ul", items: [item] });
    } else {
      blocks.push({ kind: "p", text: line });
    }
  }
  if (blocks.length <= 1 && blocks[0]?.kind !== "ul") {
    return <p className="s-turn-body">{text.trim()}</p>;
  }
  return (
    <div className="s-turn-body s-turn-body--rich">
      {blocks.map((b, i) =>
        b.kind === "p" ? (
          <p key={i}>{b.text}</p>
        ) : (
          <ul key={i}>
            {b.items.map((item, j) => (
              <li key={j}>{item}</li>
            ))}
          </ul>
        ),
      )}
    </div>
  );
}

function CrisisPanel({
  onClose,
  onResume,
  onEnd,
  hidden,
  services,
}: {
  onClose: () => void;
  onResume: () => void;
  onEnd: () => void;
  hidden: boolean;
  services: typeof helpData.help;
}) {
  const w = useWords();
  const { lang } = useI18n();
  if (hidden) return null;
  return (
    <section className="s-crisis-panel" aria-labelledby="crisis-title">
      <h3 id="crisis-title" tabIndex={-1}>
        {w("Kamu tidak sendirian", "You're not alone")}
      </h3>
      <p>
        {w(
          "Kalau kamu dalam bahaya sekarang, hubungi layanan darurat setempat atau orang terdekat yang bisa menemanimu.",
          "If you're in danger right now, contact local emergency services or someone nearby who can stay with you.",
        )}
      </p>
      <p className="s-small">
        {w(
          "Sela tidak bisa menghubungi siapa pun atas namamu.",
          "Sela can't contact anyone on your behalf.",
        )}
      </p>
      {services.map((s) => (
        <div className="s-service-mini" key={s.id}>
          <h4>{s.name}</h4>
          <p className="s-small">{s.description[lang]}</p>
          <div className="s-actions">
            {s.channels.map((c) => (
              <a
                key={c.kind + c.value}
                className={buttonClass("secondary")}
                href={channelHref(c)}
                target={c.kind === "web" ? "_blank" : undefined}
                rel={c.kind === "web" ? "noreferrer" : undefined}
              >
                {c.kind === "phone"
                  ? w("Telepon ", "Call ") + s.name
                  : c.kind === "whatsapp"
                    ? "WhatsApp · " + s.name
                    : s.name}
              </a>
            ))}
          </div>
        </div>
      ))}
      <div className="s-actions">
        <Link to="/help" className={buttonClass("help")} onClick={onClose}>
          <Icon name="help" />
          <span>{w("Lihat semua bantuan", "See all help")}</span>
        </Link>
        <Button variant="secondary" onClick={onResume}>
          {w("Lanjutkan bercerita dengan Sela", "Keep talking with Sela")}
        </Button>
        <Button variant="quiet" onClick={onEnd}>
          {w("Akhiri percakapan", "End conversation")}
        </Button>
        <Button variant="quiet" onClick={onClose}>
          {w("Tutup bantuan ini", "Close this help")}
        </Button>
      </div>
    </section>
  );
}
