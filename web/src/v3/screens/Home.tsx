import { useEffect, useState } from "react";
import { Link } from "../../lib/router";
import { useI18n } from "../../i18n";
import { api, type Entry, type Profile, type Room } from "../api";
import { usePrefs } from "../prefs";
import { useSession } from "../session";
import { useLocale, useWords } from "../shared";
import { Icon } from "../icons";
import { Loading, ErrorNotice, EmptyState, LinkButton } from "../ui";

function greeting(lang: string): string {
  const h = new Date().getHours();
  if (lang === "id") {
    if (h < 11) return "Selamat pagi.";
    if (h < 15) return "Selamat siang.";
    if (h < 19) return "Selamat sore.";
    return "Selamat malam.";
  }
  if (h < 12) return "Good morning.";
  if (h < 18) return "Good afternoon.";
  return "Good evening.";
}

export function HomePage({ user }: { user: Profile }) {
  const w = useWords();
  const { lang } = useI18n();
  const loc = useLocale();
  const { prefs } = usePrefs();
  const { refresh } = useSession();
  const [entries, setEntries] = useState<Entry[] | null>(null);
  const [rooms, setRooms] = useState<Room[] | null>(null);
  const [error, setError] = useState<unknown>(null);

  const load = () => {
    setError(null);
    Promise.all([
      api<{ records: Entry[] }>("/records"),
      api<{ conversations: Room[] }>("/conversations"),
    ])
      .then(([r, c]) => {
        setEntries(r.records);
        setRooms(c.conversations);
      })
      .catch((e) => {
        setError(e);
        // Keep null so the cards show a retryable failure, not an empty state.
      });
  };
  useEffect(load, []);

  const openRoom = rooms?.find((r) => !r.save) ?? rooms?.[0];
  const recentConversation =
    entries
      ?.filter((r) => r.kind === "conversation")
      .sort((a, b) => Date.parse(b.updatedAt) - Date.parse(a.updatedAt))[0] ?? null;
  const continueRoom =
    openRoom &&
    (!openRoom.recordId ||
      Date.now() - Date.parse(openRoom.turns.at(-1)?.createdAt ?? new Date().toISOString()) <
        12 * 3600_000)
      ? openRoom
      : null;
  const continueSaved =
    !continueRoom &&
    recentConversation &&
    Date.now() - Date.parse(recentConversation.updatedAt) < 12 * 3600_000
      ? recentConversation
      : null;
  const latest = (entries ?? []).slice(0, 3);
  const ageConfirm = user.actionsRequired?.includes("confirm_age_band");

  return (
    <div className="s-home">
      <header className="s-home-head">
        <p className="s-overline">{loc.date(new Date().toISOString())}</p>
        <h1 id="page-title">{w("Ada ruang untukmu hari ini.", "There is room for you today.")}</h1>
        <p className="s-home-lead">
          {w(
            `${greeting(lang)} Apa yang paling terasa hari ini?`,
            `${greeting(lang)} What feels most present today?`,
          )}
        </p>
      </header>

      {error != null && (
        <ErrorNotice
          error={error}
          title={w("Sebagian belum bisa dimuat", "Some of this couldn't load")}
          onRetry={load}
        />
      )}

      <section className="s-room-invite" aria-labelledby="home-start-title">
        <div>
          <p className="s-overline">{w("RUANG CERITA", "SPACE TO TALK")}</p>
          <h2 id="home-start-title">
            {w("Ceritakan apa yang sedang kamu bawa.", "Talk about what's on your mind.")}
          </h2>
          <p>
            {w(
              "Tidak perlu rapi atau lengkap. Sela mendengarkan lebih dulu.",
              "It doesn't have to be tidy or complete. Sela listens first.",
            )}
          </p>
          <LinkButton to="/talk/new" variant="primary" icon="send">
            {w("Mulai bercerita", "Start talking")}
          </LinkButton>
          <p className="s-hint">
            {w(
              "Bisa juga tanpa balasan AI. Kamu yang memilih apa yang disimpan.",
              "It can work without AI replies too. You choose what is kept.",
            )}
          </p>
        </div>
        <div className="s-room-edge" aria-hidden="true">
          <span />
          <span />
          <span />
          <i />
          <span />
          <span />
        </div>
      </section>

      <div className="s-quick-actions">
        <Link to="/notes/write">
          <Icon name="notes" />
          <span>
            <strong>{w("Tulis catatan", "Write a note")}</strong>
            <small>{w("Dengan kata-katamu sendiri.", "In your own words.")}</small>
          </span>
          <Icon name="arrow-right" />
        </Link>
        <Link to="/notes/check-in">
          <Icon name="interpretation" />
          <span>
            <strong>{w("Cek perasaan", "Check in")}</strong>
            <small>{w("Sejenak, mengenali perasaanmu.", "A moment to notice how you feel.")}</small>
          </span>
          <Icon name="arrow-right" />
        </Link>
      </div>

      {continueRoom && (
        <section className="s-section" aria-labelledby="continue-title">
          <div className="s-section-title">
            <h2 id="continue-title">{w("Lanjutkan", "Continue")}</h2>
          </div>
          <Link
            className="s-support-card"
            to={"/talk/" + continueRoom.id}
            style={{ textDecoration: "none", color: "inherit" }}
          >
            <strong>
              {continueRoom.save
                ? w("Percakapan tersimpan", "Saved conversation")
                : w("Percakapan yang sedang berjalan (tidak disimpan)", "Conversation in progress (not saved)")}
            </strong>
            <span className="s-muted s-small">
              {w("Terakhir aktif ", "Last active ")}
              {loc.dateTime(
                continueRoom.turns.at(-1)?.createdAt ?? new Date().toISOString(),
              )}
              {continueRoom.turns.length
                ? ` · ${continueRoom.turns.length} ${w("pesan", "messages")}`
                : ""}
            </span>
          </Link>
        </section>
      )}

      {!continueRoom && continueSaved && (
        <section className="s-section" aria-labelledby="continue-title">
          <div className="s-section-title">
            <h2 id="continue-title">{w("Lanjutkan", "Continue")}</h2>
          </div>
          <Link
            to={"/talk?record=" + continueSaved.id}
            className="s-support-card"
            style={{ textDecoration: "none", color: "inherit" }}
          >
            <strong>
              {prefs.hidePreviews
                ? w("Percakapan", "Conversation")
                : continueSaved.title || w("Percakapan", "Conversation")}
            </strong>
            <span className="s-muted s-small">
              {w("Terakhir aktif ", "Last active ")}
              {loc.dateTime(continueSaved.updatedAt)}
            </span>
          </Link>
        </section>
      )}

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
                onClick={async () => {
                  try {
                    await api<Profile>(
                      "/me",
                      { expectedVersion: user.version, ageBand: "18_plus" },
                      "PATCH",
                    );
                    await refresh();
                  } catch {
                    // The banner stays; the person can try again.
                  }
                }}
              >
                {w("Ya, aku 18 tahun ke atas", "Yes, I'm 18 or older")}
              </button>
              <LinkButton to="/settings/account" variant="quiet">
                {w("Ubah usia di Akun", "Change age in Account")}
              </LinkButton>
            </div>
          </div>
        </section>
      )}

      <section className="s-section" aria-labelledby="latest-title">
        <div className="s-section-title">
          <h2 id="latest-title">{w("Terbaru di Catatanku", "Latest in My notes")}</h2>
          <Link to="/notes">{w("Lihat semua", "See all")}</Link>
        </div>
        {entries === null ? (
          error ? (
            <p className="s-muted">
              {w(
                "Belum bisa memuat yang tersimpan. Aksi di atas tetap bisa dipakai.",
                "Couldn't load what you've saved. The actions above still work.",
              )}
            </p>
          ) : (
            <Loading label={w("Memuat yang tersimpan…", "Loading what you've saved…")} />
          )
        ) : latest.length === 0 ? (
          <EmptyState
            title={w("Belum ada yang tersimpan.", "Nothing saved yet.")}
            body={w(
              "Apa pun yang kamu simpan akan muncul di Catatanku. Tidak ada target atau kewajiban.",
              "Anything you save will appear in My notes. No targets. No obligations.",
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
          <div className="s-entry-list">
            {latest.map((r) => (
              <Link
                key={r.id}
                to={
                  r.kind === "conversation"
                    ? "/talk?record=" + r.id
                    : "/notes/" + r.id
                }
              >
                <span className="s-entry-date">{loc.shortDate(r.updatedAt)}</span>
                <span>
                  <strong>
                    {r.kind === "conversation"
                      ? w("Percakapan", "Conversation")
                      : r.kind === "checkin"
                        ? w("Cek perasaan", "Check-in")
                        : w("Catatan", "Note")}
                  </strong>
                  {!prefs.hidePreviews && (
                    <small>
                      {r.kind === "conversation"
                        ? r.title || w("Percakapan tersimpan", "Saved conversation")
                        : r.text.slice(0, 120) || w("Tanpa teks", "No text")}
                    </small>
                  )}
                </span>
                <Icon name="arrow-right" />
              </Link>
            ))}
          </div>
        )}
      </section>

      {user.kind === "guest" && user.guest && (
        <p className="s-retention">
          {w("Mode tamu · dihapus otomatis jika ", "Guest mode · deleted automatically after ")}
          {Math.floor(user.guest.inactiveTtlHours / 24)}
          {w(" hari tidak aktif, paling lambat ", " inactive days, at the latest ")}
          {loc.date(user.guest.maxAgePurgeAt)} ·{" "}
          <Link to="/settings/account/upgrade">{w("Simpan ke akun", "Save to an account")}</Link>
        </p>
      )}
    </div>
  );
}
