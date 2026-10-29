import { useEffect, useMemo, useState } from "react";
import { Link } from "../../lib/router";
import { api, type Entry, type Profile, type Room } from "../api";
import { usePrefs } from "../prefs";
import { useSession } from "../session";
import { useLocale, useWords } from "../shared";
import { Icon, MoodFace, Sparkle, type MoodValue } from "../icons";
import { entryRoute } from "./Continuity";
import { Loading, ErrorNotice, LinkButton } from "../ui";
import { moodLabel } from "./Notes";

// The server sweeps unsaved rooms after 30 minutes without activity; a saved
// conversation stays reachable for 12 hours (S07, docs/design/SCREENS.md).
const UNSAVED_WINDOW_MS = 30 * 60_000;
const SAVED_WINDOW_MS = 12 * 3600_000;

function lastActivity(room: Room): number {
  const last = room.turns.at(-1)?.createdAt;
  return last ? Date.parse(last) : 0;
}

export function HomePage({ user, previewEntries }: { user: Profile; previewEntries?: Entry[] }) {
  const w = useWords();
  const loc = useLocale();
  const { prefs } = usePrefs();
  const { refresh } = useSession();
  const [entries, setEntries] = useState<Entry[] | null>(previewEntries ?? null);
  const [entriesError, setEntriesError] = useState<unknown>(null);
  const [rooms, setRooms] = useState<Room[] | null>(previewEntries ? [] : null);
  const [roomsError, setRoomsError] = useState<unknown>(null);
  const [ageBusy, setAgeBusy] = useState(false);
  const [ageError, setAgeError] = useState(false);

  const loadEntries = () => {
    if (previewEntries) return;
    setEntriesError(null);
    api<{ records: Entry[] }>("/records")
      .then((r) => setEntries(r.records))
      .catch((e) => {
        setEntriesError(e);
        // Keep null so the card shows a retryable failure, not an empty state.
      });
  };
  const loadRooms = () => {
    if (previewEntries) return;
    setRoomsError(null);
    api<{ conversations: Room[] }>("/conversations")
      .then((r) => setRooms(r.conversations))
      .catch((e) => {
        setRoomsError(e);
      });
  };
  useEffect(loadEntries, []);
  useEffect(loadRooms, []);

  const greeting = useMemo(() => {
    const hour = new Date().getHours();
    const part =
      hour < 11
        ? w("Selamat pagi.", "Good morning.")
        : hour < 15
          ? w("Selamat siang.", "Good afternoon.")
          : hour < 19
            ? w("Selamat sore.", "Good afternoon.")
            : w("Selamat malam.", "Good evening.");
    return part + w(" Apa yang ingin kamu lakukan sekarang?", " What would you like to do now?");
  }, [w]);

  const sortedRooms = useMemo(
    () => [...(rooms ?? [])].sort((a, b) => lastActivity(b) - lastActivity(a)),
    [rooms],
  );
  const openRoom = sortedRooms.find((r) => !r.save);
  const savedRoom = sortedRooms.find((r) => r.save);
  const continueRoom =
    openRoom && Date.now() - lastActivity(openRoom) < UNSAVED_WINDOW_MS
      ? openRoom
      : savedRoom && Date.now() - lastActivity(savedRoom) < SAVED_WINDOW_MS
        ? savedRoom
        : null;
  const recentConversation =
    entries?.find((r) => r.kind === "conversation") ?? null;
  const continueSaved =
    !continueRoom &&
    recentConversation &&
    Date.now() - Date.parse(recentConversation.updatedAt) < SAVED_WINDOW_MS
      ? recentConversation
      : null;
  const continueRecord = continueRoom?.recordId
    ? entries?.find((e) => e.id === continueRoom.recordId) ?? null
    : null;
  const latest = (entries ?? []).slice(0, 3);
  const ageConfirm = user.actionsRequired?.includes("confirm_age_band");
  // A first visit has no conversation and nothing saved: the page shows the
  // greeting, the three actions and one empty sentence (S07 empty state).
  const firstVisit =
    entries !== null &&
    entries.length === 0 &&
    !continueRoom &&
    !continueSaved &&
    !entriesError;

  const dashboardHeading = (<>
      <section className="s-home-head" aria-labelledby="page-title">
        <h1 id="page-title" className="s-sr-only">{w("Beranda", "Home")}</h1>
        <p className="s-home-lead">{greeting}</p>
      </section>
  </>);
  const dashboardErrors = (<>
      {roomsError != null && (
        <p className="s-home-inline-error">
          {w("Belum bisa memuat percakapan.", "Couldn't load conversations.")}{" "}
          <button type="button" className="s-text-button" onClick={loadRooms}>
            {w("Coba lagi", "Try again")}
          </button>
        </p>
      )}


  </>);
  const dashboardContinue = (<>
      {(continueRoom || continueSaved) && (
        <section className="s-section" aria-labelledby="continue-title">
          <div className="s-section-title">
            <h2 id="continue-title">{w("Lanjutkan", "Continue")}</h2>
          </div>
          <div className="s-entry-list">
            {continueRoom ? (
              <Link to={"/talk/" + continueRoom.id}>
                <span className="s-entry-date">
                  {loc.shortDate(
                    continueRoom.turns.at(-1)?.createdAt ?? new Date().toISOString(),
                  )}
                </span>
                <span>
                  <strong>
                    {continueRecord && !prefs.hidePreviews
                      ? continueRecord.title || w("Percakapan", "Conversation")
                      : continueRoom.save
                        ? w("Percakapan tersimpan", "Saved conversation")
                        : w(
                            "Percakapan yang sedang berjalan (tidak disimpan)",
                            "Conversation in progress (not saved)",
                          )}
                  </strong>
                  <small>
                    {continueRoom.turns.length
                      ? `${continueRoom.turns.length} ${w("pesan", "messages")} · `
                      : ""}
                    {loc.dateTime(
                      continueRoom.turns.at(-1)?.createdAt ?? new Date().toISOString(),
                    )}
                  </small>
                </span>
                <Icon name="arrow-right" />
              </Link>
            ) : continueSaved ? (
              <Link to={"/talk?record=" + continueSaved.id}>
                <span className="s-entry-date">{loc.shortDate(continueSaved.updatedAt)}</span>
                <span>
                  <strong>
                    {prefs.hidePreviews
                      ? w("Percakapan", "Conversation")
                      : continueSaved.title || w("Percakapan", "Conversation")}
                  </strong>
                  <small>
                    {w("Terakhir aktif ", "Last active ")}
                    {loc.dateTime(continueSaved.updatedAt)}
                  </small>
                </span>
                <Icon name="arrow-right" />
              </Link>
            ) : null}
          </div>
        </section>
      )}


  </>);
  const dashboardHistory = (<>
      {firstVisit ? (
        <p className="s-home-empty">
          {w(
            "Belum ada yang tersimpan. Apa pun yang kamu simpan akan muncul di Catatanku.",
            "Nothing saved yet. Anything you save will appear in My notes.",
          )}
        </p>
      ) : (
        <>
          {entriesError != null && (
            <section className="s-section" aria-labelledby="latest-title">
              <div className="s-section-title">
                <h2 id="latest-title">{w("Terbaru di Catatanku", "Latest in My notes")}</h2>
              </div>
              <ErrorNotice
                error={entriesError}
                title={w("Belum bisa memuat yang terbaru", "Couldn't load the latest")}
                onRetry={loadEntries}
              />
            </section>
          )}

          {entries === null && entriesError == null && (
            <section className="s-section" aria-labelledby="latest-title">
              <div className="s-section-title">
                <h2 id="latest-title">{w("Terbaru di Catatanku", "Latest in My notes")}</h2>
              </div>
              <Loading label={w("Memuat yang tersimpan…", "Loading what you've saved…")} />
            </section>
          )}

          {entries !== null && entries.length > 0 && (
            <section className="s-section" aria-labelledby="latest-title">
              <div className="s-section-title">
                <h2 id="latest-title">{w("Terbaru di Catatanku", "Latest in My notes")}</h2>
                <Link to="/notes">{w("Lihat semua", "See all")}</Link>
              </div>
              <div className="s-entry-list">
                {latest.map((r) => (
                  <Link
                    key={r.id}
                    to={entryRoute(r)}
                  >
                    <span className="s-entry-date">{loc.shortDate(r.updatedAt)}</span>
                    <span>
                      <strong>
                        {r.kind === "conversation"
                          ? w("Percakapan", "Conversation")
                          : r.kind === "checkin"
                            ? w("Cek perasaan", "Check-in")
                            : r.kind === "summary" ? w("Ringkasan", "Summary")
                            : r.kind === "memory" ? w("Memori", "Memory")
                            : r.kind === "followup" ? w("Tindak lanjut", "Follow-up")
                            : w("Catatan", "Note")}
                      </strong>
                      {!prefs.hidePreviews && (
                        <small>
                          {r.kind === "conversation"
                            ? r.title || w("Percakapan tersimpan", "Saved conversation")
                            : r.kind === "checkin"
                              ? r.mood
                                ? moodLabel(r.mood, loc.lang)
                                : ""
                              : r.text.slice(0, 240) || w("Tanpa teks", "No text")}
                        </small>
                      )}
                    </span>
                    <Icon name="arrow-right" />
                  </Link>
                ))}
              </div>
            </section>
          )}

          {!firstVisit && (
            <section className="s-section s-home-memory" aria-labelledby="memory-title">
              <div className="s-section-title">
                <h2 id="memory-title">{w("Yang Sela ingat", "What Sela remembers")}</h2>
                <Link to="/notes/memory">{w("Buka", "Open")}</Link>
              </div>
              <p className="s-small s-muted">
                {w(
                  "Simpan konteks yang kamu pilih sendiri. Perbaiki atau hapus kapan saja; tidak ada pengambilan otomatis.",
                  "Keep context you choose yourself. Edit or delete it any time; nothing is collected automatically.",
                )}
              </p>
            </section>
          )}
        </>
      )}


  </>);
  const dashboardTools = (<>
      <section className="c-home-continuity" aria-label={w("Lanjutkan dengan caramu", "Continue your way")}>
        <Link to="/notes/weekly"><span><strong>{w("Lihat minggu ini", "Look back at this week")}</strong><small>{w("Tulisanmu, tanpa kesimpulan otomatis.", "Your writing, without automatic conclusions.")}</small></span><Icon name="arrow-right" /></Link>
        <Link to="/support/summaries"><span><strong>{w("Bawa yang penting", "Bring what matters")}</strong><small>{w("Ringkasan dari cerita yang kamu pilih.", "A summary from the stories you choose.")}</small></span><Icon name="arrow-right" /></Link>
      </section>

  </>);
  const dashboardRetention = (<>
      {user.kind === "guest" && user.guest && (
        <p className="s-retention">
          {w("Mode tamu · dihapus otomatis jika ", "Guest mode · deleted automatically after ")}
          {Math.floor(user.guest.inactiveTtlHours / 24)}
          {w(" hari tidak aktif, paling lambat ", " inactive days, at the latest ")}
          {loc.date(user.guest.maxAgePurgeAt)} ·{" "}
          <Link to="/settings/account/upgrade">{w("Simpan ke akun", "Save to an account")}</Link>
        </p>
      )}


  </>);
  const dashboardAge = (<>
      {ageConfirm && (
        <section className="s-section" aria-labelledby="age-check">
          <div className="s-support-card">
            <h2 id="age-check">{w("Apakah usiamu masih 16–17 tahun?", "Are you still 16–17?")}</h2>
            <p className="s-small s-muted">
              {w(
                "Kami menanyakan sekali setahun supaya perlindungan yang tepat tetap berlaku.",
                "We ask once a year so the right protections stay in place.",
              )}
            </p>
            <div className="s-actions">
              <button
                type="button"
                className="s-btn s-btn--secondary"
                disabled={ageBusy}
                onClick={async () => {
                  setAgeBusy(true);
                  setAgeError(false);
                  try {
                    await api<Profile>(
                      "/me",
                      { expectedVersion: user.version, ageBand: "18_plus" },
                      "PATCH",
                    );
                    await refresh();
                  } catch {
                    setAgeError(true);
                    setAgeBusy(false);
                  }
                }}
              >
                {w("Ya, aku 18 tahun ke atas", "Yes, I'm 18 or older")}
              </button>
              <LinkButton to="/settings/account" variant="quiet">
                {w("Ubah usia di Akun", "Change age in Account")}
              </LinkButton>
            </div>
            {ageError && (
              <p className="s-small s-muted" role="alert">
                {w(
                  "Belum bisa menyimpan. Coba lagi.",
                  "Couldn't save that. Please try again.",
                )}
              </p>
            )}
          </div>
        </section>
      )}

  </>);
  const dashboardStart = <section className="s-dash-start">
    <h2>{w("Ruang untuk apa pun yang terasa.", "Room for whatever you feel.")}</h2>
    <p>{w("Tidak perlu merangkai cerita yang sempurna. Mulai dari yang paling mudah untukmu.", "Your story does not need to be perfect. Start wherever feels easiest.")}</p>
        <div className="s-home-actions">
          <LinkButton to="/talk/new" variant="primary" icon="send" className="s-btn--go">
            {w("Mulai bercerita", "Start talking")}
          </LinkButton>
          <div className="s-activities">
            <Link to="/notes/write" className="s-activity s-activity--note">
              <span className="s-activity-title">{w("Tulis catatan", "Write a note")}</span>
              <span className="s-activity-go" aria-hidden="true"><Icon name="arrow-up-right" size={18} /></span>
              <JournalArt />
            </Link>
            <Link to="/notes/check-in" className="s-activity s-activity--checkin">
              <span className="s-activity-title">{w("Cek perasaan", "Check in")}</span>
              <span className="s-activity-go" aria-hidden="true"><Icon name="arrow-up-right" size={18} /></span>
              <span className="s-activity-art s-activity-art--faces" aria-hidden="true">
                <MoodFace mood="okay" size={46} />
                <MoodFace mood="good" size={58} />
              </span>
            </Link>
          </div>
        </div>
        <p className="s-hint">
          {w(
            "Kamu menentukan apa yang disimpan dan apakah balasan AI diizinkan.",
            "You decide what is kept and whether AI replies are allowed.",
          )}
        </p>
  </section>;
  const moodChoices: MoodValue[] = ["very_heavy", "heavy", "okay", "good", "very_good", "mixed"];
  const dashboardMood = <section className="s-mood-row" aria-labelledby="mood-title">
    <div className="s-mood-row-head">
      <h2 id="mood-title">{w("Bagaimana perasaanmu hari ini?", "How do you feel today?")}</h2>
      <p className="s-hint">{w("Pilih yang paling dekat. Kamu tetap meninjau dan memilih simpan sebelum apa pun dicatat.", "Pick the closest one. You still review and choose to save before anything is kept.")}</p>
    </div>
    <ul>
      {moodChoices.map((m) => (
        <li key={m}>
          <Link to={"/notes/check-in?mood=" + m} className="s-mood-link">
            <MoodFace mood={m} size={52} />
            <span>{moodLabel(m, loc.lang)}</span>
          </Link>
        </li>
      ))}
    </ul>
  </section>;
  const dashboardIndex = <section className="s-dash-index" aria-label={w("Ruang lainnya", "More spaces")}>
    <h2>{w("Sesuai kebutuhanmu", "At your own pace")}</h2>
    <Link to="/notes/memory"><Icon name="bookmark" /><span><strong>{w("Memori pilihanmu", "Memory you choose")}</strong><small>{w("Konteks yang kamu simpan sendiri", "Context you choose to keep")}</small></span><Icon name="arrow-right" /></Link>
    <Link to="/support/prepare"><Icon name="support" /><span><strong>{w("Siapkan bantuan", "Prepare for support")}</strong><small>{w("Tujuan, pertanyaan, dan catatan", "Goals, questions, and notes")}</small></span><Icon name="arrow-right" /></Link>
    <Link to="/support/after"><Icon name="calendar" /><span><strong>{w("Tindak lanjut", "Follow-up")}</strong><small>{w("Langkah yang kamu tentukan", "Steps you decide on")}</small></span><Icon name="arrow-right" /></Link>
    <Link to="/reading"><Icon name="notes" /><span><strong>{w("Bacaan untuk jeda", "A moment to read")}</strong><small>{w("Sumber yang bisa kamu telusuri", "Sources you can explore")}</small></span><Icon name="arrow-right" /></Link>
  </section>;
  return (
    <div className="s-home s-dashboard s-dashboard--today">
      <div className="s-cover s-dash-stage">
        <Sparkle size={34} className="s-sheet-spark s-sheet-spark--a" />
        <Sparkle size={18} className="s-sheet-spark s-sheet-spark--b" />
        <Sparkle size={12} className="s-sheet-spark s-sheet-spark--c" />
        {dashboardHeading}
        {dashboardStart}
      </div>
      {dashboardMood}
      <div className="s-dash-page">
        <div className="s-dash-history">
          {dashboardErrors}{dashboardContinue}{dashboardHistory}
        </div>
        <aside className="s-dash-side">{dashboardTools}{dashboardIndex}</aside>
      </div>
      {dashboardRetention}{dashboardAge}
    </div>
  );
}

function JournalArt() {
  // A closed notebook with a pencil, drawn from plain shapes.
  return (
    <svg className="s-activity-art" width="112" height="96" viewBox="0 0 112 96" aria-hidden="true" focusable="false">
      <g transform="rotate(-10 52 52)">
        <rect x="22" y="18" width="60" height="74" rx="8" fill="#2F6A43" />
        <rect x="28" y="14" width="58" height="74" rx="8" fill="#E9877A" stroke="#2A1F1A" strokeWidth="2.5" />
        <rect x="38" y="28" width="36" height="12" rx="4" fill="#FBFBF7" stroke="#2A1F1A" strokeWidth="2" />
        <path d="M40 52h30M40 61h22" stroke="#2A1F1A" strokeWidth="2.5" strokeLinecap="round" />
      </g>
      <g transform="rotate(28 92 54)">
        <rect x="86" y="18" width="13" height="54" rx="3" fill="#F6C26B" stroke="#2A1F1A" strokeWidth="2.5" />
        <rect x="86" y="18" width="13" height="10" rx="3" fill="#E9877A" stroke="#2A1F1A" strokeWidth="2.5" />
        <path d="M86 72h13l-6.5 13Z" fill="#FBE3C4" stroke="#2A1F1A" strokeWidth="2.5" strokeLinejoin="round" />
      </g>
    </svg>
  );
}
