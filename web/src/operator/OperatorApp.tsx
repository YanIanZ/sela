import {
  useCallback,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
  type FormEvent,
} from "react";
import { useI18n } from "../i18n";
import { useWords } from "../v3/shared";
import {
  AdminError,
  adminApi,
  type Admin,
  type AISpend,
  type AuditEvent,
  type CacheHealth,
  type CareAudit,
  type DirectoryEntry,
  type Overview,
  type SandboxRequest,
  type Staff,
  type TaskSpend,
} from "./api";
import { DirectoryPanel, SandboxPanel } from "./CarePanels";
import { LanguageToggle, OperatorHeader, roleLabel } from "./chrome";
import { ReadingPage } from "./ReadingPanel";
import "./operator.css";

type Words = (id: string, en: string) => string;

const ACTION_LABELS: Record<string, [string, string]> = {
  "admin.create": ["Staf dibuat", "Staff created"],
  "admin.login": ["Masuk", "Signed in"],
  "admin.login_failed": ["Percobaan masuk gagal", "Failed sign-in"],
  "admin.logout": ["Keluar", "Signed out"],
  "admin.revoke": ["Staf dicabut", "Staff revoked"],
  "admin.cache_invalidate": ["Cache publik dibersihkan", "Public cache invalidated"],
};

function actionLabel(action: string, w: Words): string {
  const label = ACTION_LABELS[action];
  return label ? w(label[0], label[1]) : action;
}

function authMessage(code: string, w: Words): string {
  switch (code) {
    case "admin_invalid_credentials":
      return w(
        "Nama pengguna, kata sandi, atau kode MFA tidak cocok.",
        "The username, password or MFA code doesn't match.",
      );
    case "admin_throttled":
      return w(
        "Terlalu banyak percobaan. Tunggu sebentar lalu coba lagi.",
        "Too many attempts. Wait a moment and try again.",
      );
    case "admin_unavailable":
      return w(
        "Konsol operator tidak diaktifkan di server ini.",
        "The operator console isn't enabled on this server.",
      );
    case "network":
      return w(
        "Tidak dapat menghubungi server. Periksa koneksimu.",
        "Can't reach the server. Check your connection.",
      );
    default:
      return w(
        "Tidak bisa masuk sekarang. Coba lagi.",
        "Can't sign in right now. Try again.",
      );
  }
}

function dataMessage(err: unknown, w: Words): string {
  if (err instanceof AdminError) {
    if (err.code === "admin_forbidden") {
      return w(
        "Peranmu tidak memiliki izin untuk tindakan itu.",
        "Your role isn't allowed to do that.",
      );
    }
    if (err.code === "cache_disabled") {
      return w(
        "Cache Redis tidak diaktifkan di server ini.",
        "Redis cache isn't enabled on this server.",
      );
    }
    if (err.code === "network") return authMessage(err.code, w);
  }
  return w(
    "Data operator tidak dapat dimuat. Coba lagi.",
    "Operator data couldn't be loaded. Try again.",
  );
}

function useFormatters() {
  const { lang } = useI18n();
  const locale = lang === "en" ? "en-GB" : "id-ID";
  return useMemo(
    () => ({
      count: (n: number) => new Intl.NumberFormat(locale).format(n),
      money: (n: number) =>
        new Intl.NumberFormat(locale === "en-GB" ? "en-US" : "id-ID", {
          style: "currency",
          currency: "USD",
          maximumFractionDigits: 4,
        }).format(n),
      date: (iso: string) => {
        const d = new Date(iso);
        if (Number.isNaN(d.getTime())) return iso;
        return new Intl.DateTimeFormat(locale, {
          dateStyle: "medium",
          timeStyle: "short",
        }).format(d);
      },
    }),
    [locale],
  );
}

export function OperatorApp() {
  const w = useWords();
  const { lang } = useI18n();
  const [admin, setAdmin] = useState<Admin | null | undefined>(undefined);
  const [bootError, setBootError] = useState("");
  const [expired, setExpired] = useState(false);

  useEffect(() => {
    document.title = lang === "en" ? "Sela Operator" : "Operator Sela";
  }, [lang]);

  useEffect(() => {
    let live = true;
    adminApi
      .me()
      .then((a) => {
        if (live) setAdmin(a);
      })
      .catch((err: unknown) => {
        if (!live) return;
        setAdmin(null);
        if (!(err instanceof AdminError && err.status === 401)) {
          setBootError(
            err instanceof AdminError ? authMessage(err.code, w) : authMessage("", w),
          );
        }
      });
    return () => {
      live = false;
    };
  }, [w]);

  const onExpired = useCallback(() => {
    setExpired(true);
    setAdmin(null);
  }, []);

  if (admin === undefined) {
    return (
      <div className="op op-splash" role="status">
        <p>{w("Memeriksa sesi operator…", "Checking operator session…")}</p>
      </div>
    );
  }
  if (admin === null) {
    return (
      <Login
        bootError={expired ? w("Sesi operator berakhir. Masuk lagi.", "The operator session ended. Sign in again.") : bootError}
        onDone={(a) => {
          setExpired(false);
          setBootError("");
          setAdmin(a);
        }}
      />
    );
  }
  const onReading = window.location.pathname.replace(/\/+$/, "") === "/operator/reading";
  if (onReading) {
    return (
      <ReadingPage
        admin={admin}
        onSignOut={() => setAdmin(null)}
        onExpired={onExpired}
      />
    );
  }
  return <Console admin={admin} onSignOut={() => setAdmin(null)} onExpired={onExpired} />;
}

function Login({
  bootError,
  onDone,
}: {
  bootError: string;
  onDone: (admin: Admin) => void;
}) {
  const w = useWords();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [totp, setTotp] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(bootError);
  const usernameId = useId();
  const passwordId = useId();
  const totpId = useId();
  const totpHintId = useId();

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      const admin = await adminApi.login(username.trim(), password, totp.trim());
      onDone(admin);
    } catch (err) {
      setError(
        err instanceof AdminError ? authMessage(err.code, w) : authMessage("", w),
      );
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="op op-login">
      <header className="op-login-top">
        <span className="op-brand-word">{w("Sela", "Sela")}</span>
        <span className="op-badge">{w("Operator", "Operator")}</span>
        <LanguageToggle />
      </header>
      <main className="op-login-card">
        <h1 className="op-login-title">
          {w("Masuk ke konsol operator", "Sign in to the operator console")}
        </h1>
        <p className="op-login-lead">
          {w(
            "Hanya untuk staf. Sesi pengguna anonim tidak bisa masuk di sini.",
            "Staff only. Anonymous user sessions cannot sign in here.",
          )}
        </p>
        {error && (
          <p className="op-alert" role="alert">
            {error}
          </p>
        )}
        <form className="op-form" onSubmit={(e) => void submit(e)}>
          <label className="op-field" htmlFor={usernameId}>
            <span>{w("Nama pengguna", "Username")}</span>
            <input
              id={usernameId}
              className="op-input"
              name="username"
              autoComplete="username"
              autoFocus
              required
              value={username}
              onChange={(e) => setUsername(e.target.value)}
            />
          </label>
          <label className="op-field" htmlFor={passwordId}>
            <span>{w("Kata sandi", "Password")}</span>
            <input
              id={passwordId}
              className="op-input"
              name="password"
              type="password"
              autoComplete="current-password"
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </label>
          <div className="op-field">
            <label htmlFor={totpId}>{w("Kode MFA", "MFA code")}</label>
            <span className="op-hint" id={totpHintId}>
              {w(
                "6 digit dari aplikasi autentikator.",
                "6 digits from your authenticator app.",
              )}
            </span>
            <input
              id={totpId}
              className="op-input op-input--code"
              name="totp"
              inputMode="numeric"
              autoComplete="one-time-code"
              pattern="[0-9]{6}"
              maxLength={6}
              aria-describedby={totpHintId}
              required
              value={totp}
              onChange={(e) => setTotp(e.target.value.replace(/\D/g, ""))}
            />
          </div>
          <button
            className="op-btn op-btn--primary"
            type="submit"
            disabled={busy}
            aria-busy={busy || undefined}
          >
            {busy ? w("Memeriksa…", "Checking…") : w("Masuk", "Sign in")}
          </button>
        </form>
        <p className="op-login-foot">
          {w(
            "Setiap tindakan operator dicatat di audit PostgreSQL. Konsol ini tidak pernah menampilkan cerita, jurnal, atau prompt pengguna.",
            "Every operator action is recorded in the PostgreSQL audit. This console never shows user stories, journals or prompts.",
          )}
        </p>
      </main>
    </div>
  );
}

function Console({
  admin,
  onSignOut,
  onExpired,
}: {
  admin: Admin;
  onSignOut: () => void;
  onExpired: () => void;
}) {
  const w = useWords();
  const fmt = useFormatters();
  const [overview, setOverview] = useState<Overview | null>(null);
  const [cache, setCache] = useState<CacheHealth | null>(null);
  const [events, setEvents] = useState<AuditEvent[] | null>(null);
  const [staff, setStaff] = useState<Staff[] | null>(null);
  const [revoking, setRevoking] = useState("");
  const [directory, setDirectory] = useState<DirectoryEntry[] | null>(null);
  const [sandbox, setSandbox] = useState<SandboxRequest[] | null>(null);
  const [careAudit, setCareAudit] = useState<CareAudit[] | null>(null);
  const [careBusy, setCareBusy] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [actionBusy, setActionBusy] = useState(false);
  const [signingOut, setSigningOut] = useState(false);
  const titleRef = useRef<HTMLHeadingElement>(null);

  const canAudit = admin.permissions.includes("security/audit/read");
  const canWriteCache = admin.permissions.includes("ops/cache/write");
  const canManageStaff = admin.permissions.includes("staff/manage");
  const canDirectory =
    admin.permissions.includes("directory/write") ||
    admin.permissions.includes("directory/verify");
  const canSandbox = admin.permissions.includes("sandbox/write");

  const handleError = useCallback(
    (err: unknown) => {
      if (err instanceof AdminError && err.status === 401) {
        onExpired();
        return;
      }
      setError(dataMessage(err, w));
    },
    [onExpired, w],
  );

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const [ov, health] = await Promise.all([
        adminApi.overview(),
        adminApi.cacheHealth(),
      ]);
      setOverview(ov);
      setCache(health);
      const jobs: Promise<void>[] = [];
      if (canAudit) jobs.push(adminApi.audit(50).then(setEvents));
      if (canManageStaff) jobs.push(adminApi.staff().then(setStaff));
      if (canDirectory) {
        jobs.push(adminApi.directory().then(setDirectory));
        jobs.push(adminApi.careAudit().then(setCareAudit));
      }
      if (canSandbox) jobs.push(adminApi.sandbox().then(setSandbox));
      await Promise.all(jobs);
    } catch (err) {
      handleError(err);
    } finally {
      setLoading(false);
    }
  }, [canAudit, canDirectory, canManageStaff, canSandbox, handleError]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    titleRef.current?.focus();
  }, []);

  const signOut = async () => {
    setSigningOut(true);
    try {
      await adminApi.logout();
      onSignOut();
    } catch (err) {
      if (err instanceof AdminError && err.status === 401) {
        onSignOut();
      } else {
        // Keep the console signed in when the server could not revoke the
        // session; the HttpOnly cookie is still valid.
        setError(dataMessage(err, w));
      }
    } finally {
      setSigningOut(false);
    }
  };

  const revokeStaff = async (username: string) => {
    setRevoking(username);
    setNotice("");
    setError("");
    try {
      await adminApi.revokeStaff(username, "operator console");
      setNotice(
        w(
          `${username} dicabut. Semua sesinya berakhir dan audit mencatat tindakan ini.`,
          `${username} was revoked. All sessions ended and the audit records this action.`,
        ),
      );
      setStaff(await adminApi.staff());
    } catch (err) {
      handleError(err);
    } finally {
      setRevoking("");
    }
  };

  const reloadCare = async () => {
    const jobs: Promise<void>[] = [];
    if (canDirectory) {
      jobs.push(adminApi.directory().then(setDirectory));
      jobs.push(adminApi.careAudit().then(setCareAudit));
    }
    if (canSandbox) jobs.push(adminApi.sandbox().then(setSandbox));
    await Promise.all(jobs);
  };

  const createDirectory = async (input: Record<string, unknown>) => {
    setCareBusy("directory-create");
    setError("");
    try {
      await adminApi.createDirectory(input);
      setNotice(w("Draf direktori dibuat. Ajukan untuk tinjauan setelah sumbernya lengkap.", "Directory draft created. Submit it after the source is complete."));
      await reloadCare();
    } catch (err) {
      handleError(err);
    } finally {
      setCareBusy("");
    }
  };

  const directoryAction = async (
    id: string,
    action: "submit" | "verify" | "reject" | "unpublish",
    version: number,
  ) => {
    setCareBusy(`directory:${id}`);
    setError("");
    try {
      await adminApi.directoryAction(id, action, version);
      setNotice(
        action === "verify"
          ? w("Entri diterbitkan. Cache publik akan membaca versi ini.", "Entry published. The public view reads this version.")
          : w("Status direktori diperbarui.", "Directory status updated."),
      );
      await reloadCare();
    } catch (err) {
      handleError(err);
    } finally {
      setCareBusy("");
    }
  };

  const createSandbox = async (input: Record<string, unknown>) => {
    setCareBusy("sandbox-create");
    setError("");
    try {
      await adminApi.createSandbox(input);
      setNotice(w("Permintaan simulasi dibuat. Tidak ada pengiriman ke fasilitas.", "Simulation request created. Nothing was sent to a facility."));
      await reloadCare();
    } catch (err) {
      handleError(err);
    } finally {
      setCareBusy("");
    }
  };

  const transitionSandbox = async (id: string, version: number, status: string) => {
    setCareBusy(`sandbox:${id}`);
    setError("");
    try {
      await adminApi.transitionSandbox(id, version, status);
      setNotice(w("Status simulasi diperbarui. Tidak ada efek eksternal.", "Simulation status updated. No external effect."));
      await reloadCare();
    } catch (err) {
      handleError(err);
    } finally {
      setCareBusy("");
    }
  };

  const invalidate = async () => {
    setActionBusy(true);
    setNotice("");
    setError("");
    try {
      await adminApi.invalidateCache();
      setNotice(
        w(
          "Cache publik dibersihkan. Permintaan berikutnya membaca sumber asli lalu mengisi ulang cache.",
          "Public cache invalidated. The next request reads the source and refills the cache.",
        ),
      );
      setCache(await adminApi.cacheHealth());
    } catch (err) {
      handleError(err);
    } finally {
      setActionBusy(false);
    }
  };

  return (
    <div className="op">
      <a className="op-skip" href="#op-main">
        {w("Lewati ke konten", "Skip to content")}
      </a>
      <OperatorHeader
        admin={admin}
        active="console"
        signingOut={signingOut}
        onSignOut={() => void signOut()}
      />

      <main className="op-main" id="op-main">
        <div className="op-intro">
          <div>
            <h1 ref={titleRef} tabIndex={-1}>
              {w("Konsol operator", "Operator console")}
            </h1>
            <p>
              {w(
                "Hanya staf. Cerita, jurnal, dan prompt pengguna tidak pernah ditampilkan di sini.",
                "Staff only. User stories, journals and prompts are never shown here.",
              )}
            </p>
          </div>
          <button
            type="button"
            className="op-btn op-btn--quiet"
            onClick={() => void load()}
            disabled={loading}
            aria-busy={loading || undefined}
          >
            {loading ? w("Memuat…", "Loading…") : w("Muat ulang", "Refresh")}
          </button>
        </div>

        {error && (
          <p className="op-alert" role="alert">
            {error}
          </p>
        )}
        {notice && (
          <p className="op-notice" role="status">
            {notice}
          </p>
        )}

        <section className="op-section" aria-labelledby="op-summary">
          <h2 id="op-summary">{w("Ringkasan", "Overview")}</h2>
          <div className="op-cards">
            <Kpi
              label={w("Staf aktif", "Active staff")}
              value={overview ? fmt.count(overview.activeStaff) : "—"}
            />
            <Kpi
              label={w("Sesi operator aktif", "Live operator sessions")}
              value={overview ? fmt.count(overview.liveSessions) : "—"}
            />
            <Kpi
              label={w("Aksi audit 24 jam", "Audit actions, 24 h")}
              value={overview ? fmt.count(overview.auditLast24h) : "—"}
            />
            <Kpi
              label={w("Cache publik", "Public cache")}
              value={cache ? cacheState(cache, w) : "—"}
              tone={cache && !cache.ok ? "warning" : undefined}
            />
          </div>
          <p className="op-meta">
            {overview
              ? w(
                  `Basis data ${overview.databaseReady ? "siap" : "tidak siap"} · versi ${overview.version}`,
                  `Database ${overview.databaseReady ? "ready" : "not ready"} · version ${overview.version}`,
                )
              : w("Memuat status server…", "Loading server status…")}
          </p>
        </section>

        <SpendPanel spend={overview?.aiSpend} byTask={overview?.aiSpendByTask} />

        {canManageStaff && (
          <StaffPanel
            admin={admin}
            staff={staff}
            busy={revoking}
            onRevoke={(username) => void revokeStaff(username)}
          />
        )}

        {canDirectory && (
          <DirectoryPanel
            entries={directory}
            audit={careAudit}
            busy={careBusy}
            canWrite={admin.permissions.includes("directory/write")}
            canVerify={admin.permissions.includes("directory/verify")}
            onCreate={(input) => void createDirectory(input)}
            onAction={(id, action, version) => void directoryAction(id, action, version)}
          />
        )}

        {canSandbox && (
          <SandboxPanel
            requests={sandbox}
            busy={careBusy}
            onCreate={(input) => void createSandbox(input)}
            onTransition={(id, version, status) => void transitionSandbox(id, version, status)}
          />
        )}

        <section className="op-section" aria-labelledby="op-cache">
          <h2 id="op-cache">{w("Cache publik", "Public cache")}</h2>
          <p>
            {w(
              "Hanya katalog bantuan publik yang disimpan di Redis. Tidak ada cerita, token, izin, atau keputusan keselamatan.",
              "Only the public help catalogue is stored in Redis. No stories, tokens, consents or safety decisions.",
            )}
          </p>
          <div className="op-inline">
            <span className="op-status">
              <span
                className={`op-dot${cache?.ok ? " op-dot--ok" : ""}`}
                aria-hidden="true"
              />
              {cache
                ? cache.ok
                  ? w("Sehat", "Healthy")
                  : cache.enabled
                    ? w("Tidak tersedia", "Unavailable")
                    : w("Tidak aktif", "Disabled")
                : w("Memuat…", "Loading…")}
            </span>
            {cache?.latencyMs !== undefined && (
              <span className="op-hint">
                {w(
                  `${cache.latencyMs} ms`,
                  `${cache.latencyMs} ms`,
                )}
              </span>
            )}
            {canWriteCache ? (
              <button
                type="button"
                className="op-btn op-btn--secondary"
                onClick={() => void invalidate()}
                disabled={actionBusy || !cache?.enabled}
                aria-busy={actionBusy || undefined}
              >
                {actionBusy
                  ? w("Membersihkan…", "Invalidating…")
                  : w("Bersihkan cache bantuan", "Invalidate help cache")}
              </button>
            ) : (
              <span className="op-hint">
                {w(
                  "Peranmu tidak dapat membersihkan cache.",
                  "Your role can't invalidate the cache.",
                )}
              </span>
            )}
          </div>
        </section>

        <section className="op-section" aria-labelledby="op-audit">
          <h2 id="op-audit">{w("Audit operator", "Operator audit")}</h2>
          {!canAudit ? (
            <p className="op-hint">
              {w(
                "Peranmu tidak memiliki izin membaca audit.",
                "Your role isn't allowed to read the audit trail.",
              )}
            </p>
          ) : events === null ? (
            <p className="op-hint">{w("Memuat audit…", "Loading audit…")}</p>
          ) : events.length === 0 ? (
            <p className="op-hint">
              {w("Belum ada tindakan operator.", "No operator actions yet.")}
            </p>
          ) : (
            <ol className="op-events">
              {events.map((e) => (
                <li key={e.id}>
                  <div className="op-event-head">
                    <strong>{actionLabel(e.action, w)}</strong>
                    <time dateTime={e.createdAt}>{fmt.date(e.createdAt)}</time>
                  </div>
                  <p className="op-event-meta">
                    {e.actor || w("Sistem", "System")}
                    {e.objectKind ? ` · ${e.objectKind}` : ""}
                    {e.objectId ? ` ${displayObjectId(e)}` : ""}
                    {transition(e) ? ` · ${transition(e)}` : ""}
                  </p>
                  {e.reason && <p className="op-event-reason">{e.reason}</p>}
                </li>
              ))}
            </ol>
          )}
        </section>

        <p className="op-foot">
          {w(
            "Konsol operator Sela. Tindakan istimewa diaudit di PostgreSQL; Redis tidak pernah menjadi sumber izin atau audit.",
            "Sela operator console. Privileged actions are audited in PostgreSQL; Redis is never the source of permission or audit.",
          )}
        </p>
      </main>
    </div>
  );
}

function displayObjectId(event: AuditEvent): string {
  if (event.objectKind === "admin_session" && event.objectId) {
    return `${event.objectId.slice(0, 8)}…`;
  }
  return event.objectId ?? "";
}

function transition(event: AuditEvent): string {
  const parts = [event.oldStatus, event.newStatus].filter(
    (value): value is string => Boolean(value),
  );
  return parts.join(" → ");
}

const STATUS_LABELS: Record<string, [string, string]> = {
  active: ["Aktif", "Active"],
  revoked: ["Dicabut", "Revoked"],
};

function statusLabel(status: string, w: Words): string {
  const label = STATUS_LABELS[status];
  return label ? w(label[0], label[1]) : status;
}

function StaffPanel({
  admin,
  staff,
  busy,
  onRevoke,
}: {
  admin: Admin;
  staff: Staff[] | null;
  busy: string;
  onRevoke: (username: string) => void;
}) {
  const w = useWords();
  const fmt = useFormatters();
  const [confirming, setConfirming] = useState("");
  const activeOwners = (staff ?? []).filter(
    (s) => s.role === "owner" && s.status === "active",
  ).length;

  return (
    <section className="op-section" aria-labelledby="op-staff">
      <h2 id="op-staff">{w("Staf operator", "Operator staff")}</h2>
      <p>
        {w(
          "Hanya identitas staf. Tidak ada kata sandi, kode MFA, atau data pengguna di daftar ini.",
          "Staff identities only. No passwords, MFA secrets or user data appear in this list.",
        )}
      </p>
      {staff === null ? (
        <p className="op-hint">{w("Memuat staf…", "Loading staff…")}</p>
      ) : staff.length === 0 ? (
        <p className="op-hint">{w("Belum ada operator.", "No operators yet.")}</p>
      ) : (
        <ul className="op-staff">
          {staff.map((s) => {
            const self = s.username === admin.username;
            const active = s.status === "active";
            const lastOwner = active && s.role === "owner" && activeOwners <= 1;
            return (
              <li key={s.username}>
                <div className="op-staff-id">
                  <strong>{s.username}</strong>
                  <span className="op-staff-tags">
                    <span className="op-badge op-badge--soft">
                      {roleLabel(s.role, w)}
                    </span>
                    {s.orgId && <span className="op-hint">{s.orgId}</span>}
                  </span>
                </div>
                <p className="op-staff-meta">
                  <span
                    className={`op-dot${active ? " op-dot--ok" : ""}`}
                    aria-hidden="true"
                  />
                  {statusLabel(s.status, w)}
                  {` · ${fmt.count(s.liveSessions)} ${w("sesi", "sessions")}`}
                  {` · ${
                    s.lastLoginAt
                      ? fmt.date(s.lastLoginAt)
                      : w("Belum pernah masuk", "Never signed in")
                  }`}
                </p>
                <div className="op-staff-actions">
                  {!active ? (
                    <span className="op-hint">
                      {w("Akses sudah dicabut", "Access revoked")}
                    </span>
                  ) : self ? (
                    <span className="op-hint">
                      {w(
                        "Tidak bisa mencabut akun sendiri",
                        "You can't revoke your own account",
                      )}
                    </span>
                  ) : lastOwner ? (
                    <span className="op-hint">
                      {w("Pemilik aktif terakhir", "Last active owner")}
                    </span>
                  ) : confirming === s.username ? (
                    <div className="op-confirm">
                      <p>
                        {w(
                          `Cabut akses ${s.username}? Semua sesinya akan berakhir.`,
                          `Revoke ${s.username}? All sessions will end.`,
                        )}
                      </p>
                      <div className="op-confirm-actions">
                        <button
                          type="button"
                          className="op-btn op-btn--danger"
                          disabled={busy === s.username}
                          onClick={() => {
                            setConfirming("");
                            onRevoke(s.username);
                          }}
                        >
                          {busy === s.username
                            ? w("Mencabut…", "Revoking…")
                            : w("Ya, cabut", "Yes, revoke")}
                        </button>
                        <button
                          type="button"
                          className="op-btn op-btn--quiet"
                          onClick={() => setConfirming("")}
                        >
                          {w("Batal", "Cancel")}
                        </button>
                      </div>
                    </div>
                  ) : (
                    <button
                      type="button"
                      className="op-btn op-btn--quiet op-btn--danger-text"
                      onClick={() => setConfirming(s.username)}
                    >
                      {w("Cabut akses", "Revoke access")}
                    </button>
                  )}
                </div>
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}

function cacheState(cache: CacheHealth, w: Words): string {
  if (!cache.enabled) return w("Tidak aktif", "Disabled");
  return cache.ok ? w("Sehat", "Healthy") : w("Tidak tersedia", "Unavailable");
}

function Kpi({
  label,
  value,
  tone,
}: {
  label: string;
  value: string;
  tone?: "warning";
}) {
  return (
    <div className={`op-card${tone ? " op-card--warning" : ""}`}>
      <span className="op-kpi-label">{label}</span>
      <strong className="op-kpi">{value}</strong>
    </div>
  );
}

function SpendPanel({
  spend,
  byTask,
}: {
  spend?: AISpend;
  byTask?: TaskSpend[];
}) {
  const w = useWords();
  const fmt = useFormatters();
  if (!spend) {
    return (
      <section className="op-section" aria-labelledby="op-spend">
        <h2 id="op-spend">{w("Anggaran AI", "AI budget")}</h2>
        <p className="op-hint">
          {w(
            "Anggaran AI bulanan tidak diaktifkan di server ini.",
            "The monthly AI budget isn't enabled on this server.",
          )}
        </p>
      </section>
    );
  }
  const pct =
    spend.monthlyLimitUsd > 0
      ? Math.min(100, Math.round((spend.committedUsd / spend.monthlyLimitUsd) * 100))
      : 0;
  return (
    <section className="op-section" aria-labelledby="op-spend">
      <h2 id="op-spend">{w("Anggaran AI", "AI budget")}</h2>
      <p>
        {w(
          "Pengeluaran API bulan berjalan, dihitung di PostgreSQL sebelum setiap panggilan berbayar.",
          "Current-month API spend, enforced in PostgreSQL before every paid call.",
        )}
      </p>
      <div className="op-cards">
        <Kpi
          label={w("Batas bulanan", "Monthly limit")}
          value={fmt.money(spend.monthlyLimitUsd)}
        />
        <Kpi
          label={w("Komitmen", "Committed")}
          value={fmt.money(spend.committedUsd)}
        />
        <Kpi label={w("Aktual", "Actual")} value={fmt.money(spend.actualUsd)} />
        <Kpi
          label={w("Reservasi terbuka", "Open reservations")}
          value={fmt.count(spend.openReservations)}
        />
      </div>
      <div
        className="op-meter"
        role="progressbar"
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={pct}
        aria-label={w("Pemakaian anggaran", "Budget use")}
      >
        <span style={{ width: `${pct}%` }} />
      </div>
      <p className="op-meta">
        {w(
          `${pct}% dari batas · ${fmt.count(spend.operations)} operasi bulan ini`,
          `${pct}% of the limit · ${fmt.count(spend.operations)} operations this month`,
        )}
      </p>
      {byTask && byTask.length > 0 && (
        <ul className="op-rows">
          {byTask.map((t) => (
            <li key={t.task}>
              <span className="op-row-name">{t.task}</span>
              <span className="op-row-value">
                {fmt.money(t.actualUsd)}
                <span className="op-hint">
                  {` · ${fmt.count(t.operations)} ${w("operasi", "operations")}`}
                </span>
              </span>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
