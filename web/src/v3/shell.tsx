import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { Link, useRouter } from "../lib/router";
import { useI18n } from "../i18n";
import { Icon, Mark, type IconName } from "./icons";
import { usePrefs } from "./prefs";
import { useSession } from "./session";
import { api, type Profile } from "./api";
import { useAnnounce, useOnline, useWords } from "./shared";
import { buttonClass } from "./ui";

const places: Array<{ to: string; label: [string, string]; icon: IconName }> = [
  { to: "/home", label: ["Beranda", "Home"], icon: "home" },
  { to: "/talk", label: ["Cerita", "Talk"], icon: "talk" },
  { to: "/notes", label: ["Catatanku", "My notes"], icon: "notes" },
  { to: "/support", label: ["Dukungan", "Support"], icon: "support" },
];

export function isPlace(path: string, place: string): boolean {
  if (place === "/home") return path === "/home";
  if (place === "/talk") return path === "/talk" || path.startsWith("/talk/");
  if (place === "/notes") return path === "/notes" || path.startsWith("/notes/");
  if (place === "/support")
    return path === "/support" || path.startsWith("/support/");
  return path === place;
}

export function LanguageSwitch({ compact = false }: { compact?: boolean }) {
  const { lang, setLang } = useI18n();
  const { user, setUser } = useSession();
  const { announce } = useAnnounce();
  const w = useWords();
  const choose = async (next: "id" | "en") => {
    if (next === lang) return;
    setLang(next);
    announce("status", next === "id" ? "Bahasa diganti." : "Language changed.");
    if (user) {
      try {
        const updated = await api<Profile>(
          "/me",
          { expectedVersion: user.version, locale: next },
          "PATCH",
        );
        setUser(updated);
      } catch {
        // The interface language changed for this tab; the profile keeps the
        // previous value and the next change can retry.
      }
    }
  };
  return (
    <div
      className={`s-lang${compact ? " s-lang--compact" : ""}`}
      role="group"
      aria-label={w("Bahasa", "Language")}
    >
      <button
        type="button"
        lang="id"
        aria-pressed={lang === "id"}
        className="s-lang-btn"
        onClick={() => void choose("id")}
      >
        {"Bahasa Indonesia"}
      </button>
      <button
        type="button"
        lang="en"
        aria-pressed={lang === "en"}
        className="s-lang-btn"
        onClick={() => void choose("en")}
      >
        {"English"}
      </button>
    </div>
  );
}

function useQuickExit() {
  const { user } = useSession();
  const { prefs } = usePrefs();
  const [armed, setArmed] = useState(false);
  const armedUntil = useRef(0);
  const suppressUntil = useRef(0);
  const timer = useRef<number | null>(null);
  const { announce } = useAnnounce();
  const w = useWords();

  const exit = useCallback(() => {
    try {
      window.dispatchEvent(new Event("sela:quick-exit"));
    } catch {
      // state is discarded by the navigation anyway
    }
    const shouldSignOut =
      user?.kind === "account" &&
      (prefs.quickExitSignOut === "always" ||
        (prefs.quickExitSignOut === "trusted" &&
          window.sessionStorage.getItem("sela.trusted") !== "true"));
    if (shouldSignOut) {
      try {
        void fetch("/api/v3/auth/logout", {
          method: "POST",
          credentials: "same-origin",
          keepalive: true,
          headers: {
            "X-Sela-Request": "1",
            "X-Sela-Language": document.documentElement.lang === "en" ? "en" : "id",
            "Content-Type": "application/json",
          },
          body: "{}",
        });
      } catch {
        // leaving still wins; the server session expires on its own
      }
    }
    window.location.replace("https://www.google.com/");
  }, [prefs.quickExitSignOut, user]);

  useEffect(() => {
    const onClose = () => {
      suppressUntil.current = Date.now() + 1500;
    };
    document.addEventListener("close", onClose, true);
    return () => document.removeEventListener("close", onClose, true);
  }, []);

  useEffect(() => {
    if (!prefs.quickExitShortcut) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== "Escape" || e.repeat || e.defaultPrevented || e.isComposing) return;
      if (document.querySelector("dialog[open]")) return;
      if (Date.now() < suppressUntil.current) return;
      if (Date.now() <= armedUntil.current) {
        armedUntil.current = 0;
        setArmed(false);
        exit();
        return;
      }
      armedUntil.current = Date.now() + 1500;
      setArmed(true);
      announce(
        "status",
        w("Tekan Esc sekali lagi untuk keluar cepat.", "Press Esc once more for Quick exit."),
      );
      if (timer.current) window.clearTimeout(timer.current);
      timer.current = window.setTimeout(() => {
        setArmed(false);
        armedUntil.current = 0;
      }, 1500);
    };
    const onAnyKey = (e: KeyboardEvent) => {
      if (e.key !== "Escape") {
        armedUntil.current = 0;
        setArmed(false);
      }
    };
    document.addEventListener("keydown", onKey);
    document.addEventListener("keydown", onAnyKey);
    return () => {
      document.removeEventListener("keydown", onKey);
      document.removeEventListener("keydown", onAnyKey);
    };
  }, [announce, exit, prefs.quickExitShortcut, w]);

  return { exit, armed };
}

function MenuSheet({
  open,
  onClose,
  onExit,
}: {
  open: boolean;
  onClose: () => void;
  onExit: () => void;
}) {
  const { user, signOut } = useSession();
  const w = useWords();
  const ref = useRef<HTMLDialogElement>(null);
  const heading = useRef<HTMLHeadingElement>(null);
  const returnTo = useRef<HTMLElement | null>(null);
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    if (open && !el.open) {
      returnTo.current =
        document.activeElement instanceof HTMLElement ? document.activeElement : null;
      el.showModal();
      requestAnimationFrame(() => document.getElementById("menu-help")?.focus());
    } else if (!open && el.open) {
      el.close();
      returnTo.current?.focus();
    }
  }, [open]);
  return (
    <dialog
      ref={ref}
      className="s-menu-sheet"
      aria-labelledby="menu-title"
      onCancel={(e) => {
        e.preventDefault();
        onClose();
      }}
      onClose={() => {
        if (open) onClose();
      }}
    >
      <div className="s-menu-inner">
        <div className="s-menu-safety">
          <Link
            to="/help"
            id="menu-help"
            className={buttonClass("help-quiet")}
            onClick={onClose}
          >
            <Icon name="help" />
            <span>{w("Butuh bantuan sekarang", "Need help now")}</span>
          </Link>
          <button type="button" className={buttonClass("quiet")} onClick={onExit}>
            <Icon name="quick-exit" />
            <span>{w("Keluar cepat", "Quick exit")}</span>
          </button>
          <button type="button" className={buttonClass("quiet")} onClick={onClose}>
            <Icon name="close" />
            <span>{w("Tutup", "Close")}</span>
          </button>
        </div>
        <h2 id="menu-title" ref={heading} tabIndex={-1}>
          {w("Menu", "Menu")}
        </h2>
        {user && (
          <nav aria-label={w("Navigasi utama", "Main navigation")} className="s-menu-group">
            <p className="s-menu-label">{w("Ruangmu", "Your space")}</p>
            {places.map((p) => (
              <Link key={p.to} to={p.to} onClick={onClose}>
                <Icon name={p.icon} />
                {w(p.label[0], p.label[1])}
              </Link>
            ))}
          </nav>
        )}
        <nav aria-label={w("Pengaturan", "Settings")} className="s-menu-group">
          <p className="s-menu-label">{w("Pengaturan", "Settings")}</p>
          {user && (
            <>
              <Link to="/settings/privacy" onClick={onClose}>
                {w("Privasi & data", "Privacy & data")}
              </Link>
              <Link to="/settings/account" onClick={onClose}>
                {w("Akun", "Account")}
              </Link>
            </>
          )}
          <Link to="/settings/display" onClick={onClose}>
            {w("Tampilan & aksesibilitas", "Display & accessibility")}
          </Link>
          {!user && (
            <>
              <Link to="/signin" onClick={onClose}>
                {w("Masuk", "Sign in")}
              </Link>
              <Link to="/start/age" onClick={onClose}>
                {w("Mulai", "Get started")}
              </Link>
              <Link to="/reading" onClick={onClose}>
                {w("Bacaan & latihan kecil", "Reading & small exercises")}
              </Link>
            </>
          )}
        </nav>
        <div className="s-menu-group">
          <p className="s-menu-label">{w("Bahasa", "Language")}</p>
          <LanguageSwitch />
        </div>
        <nav aria-label={w("Tentang", "About")} className="s-menu-group">
          <Link to="/about" onClick={onClose}>
            {w("Tentang Sela", "About Sela")}
          </Link>
          <Link to="/policies" onClick={onClose}>
            {w("Kebijakan", "Policies")}
          </Link>
          <Link to="/help" onClick={onClose}>
            {w("Bantuan sekarang", "Help now")}
          </Link>
        </nav>
        {user?.kind === "account" && (
          <button
            type="button"
            className={buttonClass("quiet", "s-menu-signout")}
            onClick={() => void signOut()}
          >
            {w("Keluar dari akun", "Sign out")}
          </button>
        )}
        {user?.kind === "guest" && (
          <p className="s-menu-note">
            {w(
              "Mode tamu tidak punya tombol keluar: tanpa akun, keluar berarti datamu tidak bisa dibuka lagi.",
              "Guest mode has no sign-out: without an account, signing out would lock you out of your data.",
            )}
          </p>
        )}
      </div>
    </dialog>
  );
}

export function AppShell({
  title,
  children,
  focused = false,
  room = false,
}: {
  title: string;
  children: ReactNode;
  focused?: boolean;
  room?: boolean;
}) {
  const w = useWords();
  const { path } = useRouter();
  const { user, expired } = useSession();
  const { prefs } = usePrefs();
  const { exit, armed } = useQuickExit();
  const { clear } = useAnnounce();
  const online = useOnline();
  const [menu, setMenu] = useState(false);
  const [crisisDismissed, setCrisisDismissed] = useState(false);
  const [guestDismissed, setGuestDismissed] = useState(false);
  const firstPath = useRef(true);

  useEffect(() => {
    document.title =
      prefs.neutralTitles || (prefs.hidePreviews && user)
        ? "Sela"
        : `${title} – Sela`;
    clear();
    setMenu(false);
  }, [clear, path, prefs.hidePreviews, prefs.neutralTitles, title, user]);

  useEffect(() => {
    // The router moves focus on its own navigations; this covers browser Back
    // and direct entry. The first paint keeps the skip link as the first Tab
    // stop instead of jumping straight into <main>.
    if (firstPath.current) {
      firstPath.current = false;
      return;
    }
    requestAnimationFrame(() => {
      const main = document.getElementById("main");
      if (main && document.activeElement !== main) main.focus();
    });
  }, [path]);

  useEffect(() => {
    const root = document.documentElement;
    let frame = 0;
    const measure = () => {
      window.cancelAnimationFrame(frame);
      frame = window.requestAnimationFrame(() => {
        const header = document.querySelector<HTMLElement>(".s-header");
        const nav = document.querySelector<HTMLElement>(".s-place-nav");
        const composer = document.getElementById("composer");
        const navStyle = nav ? getComputedStyle(nav) : null;
        const navVisible =
          Boolean(nav) &&
          navStyle?.display !== "none" &&
          navStyle?.position === "fixed" &&
          (nav?.getBoundingClientRect().height ?? 0) > 0;
        root.style.setProperty(
          "--top-bar-h",
          `${Math.round(header?.getBoundingClientRect().height ?? 0)}px`,
        );
        root.style.setProperty(
          "--bottom-bar-h",
          navVisible ? `${Math.round(nav?.getBoundingClientRect().height ?? 0)}px` : "0px",
        );
        root.style.setProperty(
          "--composer-h",
          composer ? `${Math.round(composer.getBoundingClientRect().height)}px` : "0px",
        );
      });
    };
    measure();
    const ro =
      typeof ResizeObserver === "undefined" ? null : new ResizeObserver(measure);
    const shell = document.querySelector(".s-shell");
    if (ro && shell) ro.observe(shell);
    const header = document.querySelector(".s-header");
    const nav = document.querySelector(".s-place-nav");
    if (ro && header) ro.observe(header);
    if (ro && nav) ro.observe(nav);
    const mo = new MutationObserver(measure);
    mo.observe(document.body, { childList: true, subtree: true });
    window.addEventListener("resize", measure, { passive: true });
    window.addEventListener("orientationchange", measure, { passive: true });
    return () => {
      window.cancelAnimationFrame(frame);
      ro?.disconnect();
      mo.disconnect();
      window.removeEventListener("resize", measure);
      window.removeEventListener("orientationchange", measure);
    };
  }, [path, user, focused]);

  useEffect(() => {
    const onQuick = () => {
      setMenu(false);
    };
    const onRequest = () => exit();
    window.addEventListener("sela:quick-exit", onQuick);
    window.addEventListener("sela:request-quick-exit", onRequest);
    return () => {
      window.removeEventListener("sela:quick-exit", onQuick);
      window.removeEventListener("sela:request-quick-exit", onRequest);
    };
  }, [exit]);

  const showCrisisBanner =
    Boolean(user) &&
    !prefs.hidePreviews &&
    !crisisDismissed &&
    (path === "/home" || path.startsWith("/talk"));
  const crisis =
    typeof window !== "undefined" && window.sessionStorage.getItem("sela.crisis") === "1";
  const guestNearExpiry =
    user?.kind === "guest" &&
    user.guest &&
    new Date(user.guest.maxAgePurgeAt).getTime() - Date.now() < 7 * 86_400_000 &&
    !guestDismissed;

  return (
    <div className={`s-shell${focused ? " s-shell--focused" : ""}${room ? " s-shell--room" : ""}`}>
      <div className="s-shell-inner">
      <a href="#main" className="s-skip">
        {w("Lewati ke konten", "Skip to content")}
      </a>
      {room && (
        <a href="#composer" className="s-skip s-skip--second">
          {w("Lewati ke kotak tulis", "Skip to the writing box")}
        </a>
      )}
      <header className="s-header">
        <Link to={user ? "/home" : "/"} className="s-brand">
          <Mark size={30} />
          <span>{"Sela"}</span>
        </Link>
        <div className="s-header-actions">
          <Link
            to="/help"
            className={buttonClass("help-quiet")}
            aria-current={path === "/help" ? "page" : undefined}
          >
            <Icon name="help" />
            <span>{w("Butuh bantuan sekarang", "Need help now")}</span>
          </Link>
          <div className="s-exit-wrap">
            <button type="button" className={buttonClass("quiet")} onClick={() => exit()}>
              <Icon name="quick-exit" />
              <span>{w("Keluar cepat", "Quick exit")}</span>
            </button>
            {armed && (
              <span className="s-exit-hint" role="status">
                {w("Tekan Esc sekali lagi.", "Press Esc once more.")}
              </span>
            )}
          </div>
          <button
            type="button"
            className={buttonClass("quiet", "s-menu-button")}
            aria-haspopup="dialog"
            onClick={() => setMenu(true)}
          >
            <Icon name="menu" />
            <span>{w("Menu", "Menu")}</span>
          </button>
        </div>
      </header>

      {user && !focused && (
        <nav className="s-place-nav" aria-label={w("Navigasi utama", "Main navigation")}>
          <ul>
            {places.map((p) => (
              <li key={p.to}>
                <Link
                  to={p.to}
                  aria-current={isPlace(path, p.to) ? "page" : undefined}
                >
                  <Icon name={p.icon} size={22} />
                  <span>{w(p.label[0], p.label[1])}</span>
                </Link>
              </li>
            ))}
          </ul>
        </nav>
      )}

      {user && !focused && (
        <aside className="s-sidebar">
          <nav aria-label={w("Navigasi utama", "Main navigation")}>
            <ul className="s-sidebar-places">
              {places.map((p) => (
                <li key={p.to}>
                  <Link to={p.to} aria-current={isPlace(path, p.to) ? "page" : undefined}>
                    <Icon name={p.icon} />
                    <span>{w(p.label[0], p.label[1])}</span>
                  </Link>
                </li>
              ))}
            </ul>
          </nav>
          <nav aria-label={w("Pengaturan", "Settings")} className="s-sidebar-settings">
            <p className="s-sidebar-label">{w("Pengaturan", "Settings")}</p>
            <Link
              to="/settings/privacy"
              aria-current={path === "/settings/privacy" ? "page" : undefined}
            >
              {w("Privasi & data", "Privacy & data")}
            </Link>
            <Link
              to="/settings/display"
              aria-current={path === "/settings/display" ? "page" : undefined}
            >
              {w("Tampilan & aksesibilitas", "Display & accessibility")}
            </Link>
            <Link
              to="/settings/account"
              aria-current={path === "/settings/account" ? "page" : undefined}
            >
              {w("Akun", "Account")}
            </Link>
          </nav>
          <LanguageSwitch compact />
          <nav aria-label={w("Tentang", "About")} className="s-sidebar-about">
            <Link to="/about">{w("Tentang Sela", "About Sela")}</Link>
            <Link to="/policies">{w("Kebijakan", "Policies")}</Link>
          </nav>
        </aside>
      )}

      <section className="s-notices" aria-label={w("Pemberitahuan", "Notices")}>
        {expired && (
          <div className="s-banner s-banner--warning">
            <Icon name="lock" />
            <div>
              <p>
                {w(
                  "Sesimu sudah berakhir. Masuk lagi untuk melanjutkan; tulisan yang belum terkirim masih di layar ini.",
                  "Your session has ended. Sign in again to continue; anything you hadn't sent is still on this screen.",
                )}
              </p>
              <Link to="/signin">{w("Masuk", "Sign in")}</Link>
            </div>
          </div>
        )}
        {!online && (
          <div className="s-banner s-banner--warning">
            <Icon name="wifi-off" />
            <div>
              <p>
                {w(
                  "Kamu sedang offline. Tulisanmu tetap di layar.",
                  "You're offline. Your writing stays on screen.",
                )}
              </p>
              <p className="s-small">
                {w(
                  "Bantuan sekarang tetap bisa dibuka dari data bawaan.",
                  "Help now still opens from built-in data.",
                )}
              </p>
            </div>
          </div>
        )}
        {user?.synthetic && (
          <div className="s-banner s-banner--demo">
            <span className="s-chip">{w("Demo", "Demo")}</span>
            <p>
              {w(
                "Semua data di sini sintetis. Fitur berjalan sungguhan.",
                "All data here is synthetic. Features really run.",
              )}
            </p>
          </div>
        )}
        {user?.actionsRequired?.includes("acknowledge_recovery_codes") && (
          <div className="s-banner s-banner--warning">
            <Icon name="key" />
            <div>
              <p>
                {w(
                  "Kode pemulihanmu belum dikonfirmasi tersimpan.",
                  "You haven't confirmed that your recovery codes are saved.",
                )}
              </p>
              <Link to="/settings/account">
                {w("Simpan kode pemulihan", "Save your recovery codes")}
              </Link>
            </div>
          </div>
        )}
        {user?.actionsRequired?.some((a) => a.startsWith("accept_policy:")) && (
          <div className="s-banner s-banner--info">
            <Icon name="info" />
            <div>
              <p>
                {w(
                  "Kebijakan privasi diperbarui. Lihat yang berubah.",
                  "The privacy policy changed. See what's new.",
                )}
              </p>
              <Link to="/settings/privacy#persetujuan">
                {w("Lihat persetujuan", "See your consents")}
              </Link>
            </div>
          </div>
        )}
        {guestNearExpiry && user?.guest && (
          <div className="s-banner s-banner--info">
            <Icon name="clock" />
            <div>
              <p>
                {w("Mode tamu: datamu dihapus otomatis pada ", "Guest mode: your data is deleted on ")}
                {new Intl.DateTimeFormat(
                  document.documentElement.lang === "en" ? "en-GB" : "id-ID",
                  { dateStyle: "long" },
                ).format(new Date(user.guest.maxAgePurgeAt))}
                .
              </p>
              <div className="s-actions">
                <Link to="/settings/account/upgrade">
                  {w("Simpan ke akun", "Save to an account")}
                </Link>
                <button type="button" className={buttonClass("quiet")} onClick={() => setGuestDismissed(true)}>
                  {w("Tutup", "Dismiss")}
                </button>
              </div>
            </div>
          </div>
        )}
        {showCrisisBanner && crisis && (
          <div className="s-banner s-banner--help">
            <Icon name="help" />
            <div>
              <p>
                {w(
                  "Bantuan tetap ada di sini kalau kamu butuh.",
                  "Help is still here if you need it.",
                )}
              </p>
              <div className="s-actions">
                <Link to="/help" className={buttonClass("help")}>
                  <Icon name="help" />
                  <span>{w("Lihat semua bantuan", "See all help")}</span>
                </Link>
                <button type="button" className={buttonClass("quiet")} onClick={() => setCrisisDismissed(true)}>
                  {w("Tutup", "Dismiss")}
                </button>
              </div>
            </div>
          </div>
        )}
      </section>

      <main id="main" tabIndex={-1} aria-labelledby="page-title">
        {children}
      </main>

      {!focused && (
        <footer className="s-footer">
          <p>
            {w(
              "Sela bukan layanan darurat dan tidak memberi diagnosis.",
              "Sela is not an emergency service and gives no diagnosis.",
            )}
          </p>
          <nav aria-label={w("Tautan kaki", "Footer links")}>
            <Link to="/about">{w("Tentang Sela", "About Sela")}</Link>
            <Link to="/policies">{w("Kebijakan", "Policies")}</Link>
            <Link to="/help">{w("Butuh bantuan sekarang", "Need help now")}</Link>
          </nav>
          <p className="s-attribution">
            {w("Diteruskan dari Dengar.in.", "Continues Dengar.in.")}
          </p>
        </footer>
      )}

      <MenuSheet open={menu} onClose={() => setMenu(false)} onExit={() => exit()} />
      </div>
    </div>
  );
}
