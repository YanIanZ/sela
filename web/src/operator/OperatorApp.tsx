import {
  useCallback,
  useEffect,
  useId,
  useMemo,
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
  type CareOrg,
  type DirectoryEntry,
  type Overview,
  type SandboxRequest,
  type Staff,
  type TaskSpend,
  type Tasks,
} from "./api";
import { ActivationPage } from "./ActivationPage";
import { DirectoryPanel, SandboxPanel } from "./CarePanels";
import { canReviewReading, LanguageToggle, roleLabel } from "./chrome";
import { ReadingPage } from "./ReadingPanel";
import { CommunityPanel } from "./CommunityPanel";
import { NetworkPage } from "./NetworkPanel";
import { PartnerConsole } from "./PartnerRequestsPanel";
import { OpLink, Page, RouterProvider, Shell, useRouter, type NavItem } from "./shell";
import "./operator.css";

type Words = (id: string, en: string) => string;

/** Audit action labels and the category each one belongs to. */
const ACTIONS: Record<string, [string, string, string]> = {
  "admin.create": ["Akun dibuat", "Account created", "access"],
  "admin.login": ["Masuk", "Signed in", "access"],
  "admin.login_failed": ["Gagal masuk", "Failed sign-in", "access"],
  "admin.logout": ["Keluar", "Signed out", "access"],
  "admin.revoke": ["Akses dicabut", "Access revoked", "access"],
  "admin.cache_invalidate": ["Cache dibersihkan", "Cache invalidated", "system"],
  "community.restore": ["Tulisan dipulihkan", "Post restored", "moderation"],
  "community.remove": ["Tulisan dihapus", "Post removed", "moderation"],
  "network.org_create": ["Organisasi dibuat", "Organisation created", "network"],
  "network.org_verify": ["Organisasi diverifikasi", "Organisation verified", "network"],
  "network.org_suspend": ["Organisasi ditangguhkan", "Organisation suspended", "network"],
  "network.license_verify": ["Lisensi diverifikasi", "Licence verified", "network"],
  "network.invite_create": ["Undangan dibuat", "Invitation created", "network"],
  "network.invite_revoke": ["Undangan dibatalkan", "Invitation revoked", "network"],
  "network.invite_accept": ["Undangan diterima", "Invitation accepted", "network"],
  "care_request.open": ["Rangkuman dibuka", "Summary opened", "requests"],
  "care_request.propose": ["Waktu diusulkan", "Times proposed", "requests"],
  "care_request.decline": ["Permintaan ditolak", "Request declined", "requests"],
  "care_request.complete": ["Permintaan selesai", "Request completed", "requests"],
  "directory.create": ["Entri direktori dibuat", "Directory entry created", "content"],
  "directory.update": ["Entri direktori diubah", "Directory entry updated", "content"],
  "directory.submit": ["Entri diajukan", "Entry submitted", "content"],
  "directory.verify": ["Entri diterbitkan", "Entry published", "content"],
  "directory.reject": ["Entri ditolak", "Entry rejected", "content"],
  "directory.unpublish": ["Entri ditarik", "Entry unpublished", "content"],
  "sandbox.create": ["Simulasi dibuat", "Simulation created", "content"],
  "sandbox.transition": ["Status simulasi diubah", "Simulation status changed", "content"],
  "reading.approve": ["Bacaan disetujui", "Reading approved", "content"],
  "reading.revoke": ["Bacaan ditarik", "Reading withdrawn", "content"],
};

const CATEGORIES: [string, string, string][] = [
  ["all", "Semua", "All"],
  ["access", "Akses", "Access"],
  ["network", "Jaringan", "Network"],
  ["moderation", "Moderasi", "Moderation"],
  ["requests", "Permintaan", "Requests"],
  ["content", "Konten", "Content"],
  ["system", "Sistem", "System"],
];

export function actionLabel(action: string, w: Words): string {
  const label = ACTIONS[action];
  return label ? w(label[0], label[1]) : action;
}

function actionCategory(action: string): string {
  return ACTIONS[action]?.[2] ?? "system";
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
  return (
    <RouterProvider>
      <OperatorRoot />
    </RouterProvider>
  );
}

function OperatorRoot() {
  const w = useWords();
  const { lang } = useI18n();
  const { path, navigate } = useRouter();
  const [admin, setAdmin] = useState<Admin | null | undefined>(undefined);
  const [bootError, setBootError] = useState("");
  const [expired, setExpired] = useState(false);
  const activating = path === "/operator/activate";

  useEffect(() => {
    document.title = lang === "en" ? "Sela Operator" : "Operator Sela";
  }, [lang]);

  useEffect(() => {
    if (activating) return;
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
  }, [w, activating]);

  const onExpired = useCallback(() => {
    setExpired(true);
    setAdmin(null);
  }, []);

  if (activating) {
    return (
      <ActivationPage
        onDone={(a) => {
          setAdmin(a);
          navigate("/operator");
        }}
      />
    );
  }
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
  // A partner account (D-076) acts inside one organisation and sees only
  // its care requests: none of the internal console pages apply.
  if (admin.partner) {
    return <PartnerConsole admin={admin} onSignOut={() => setAdmin(null)} onExpired={onExpired} />;
  }
  return <InternalConsole admin={admin} onSignOut={() => setAdmin(null)} onExpired={onExpired} />;
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

type Perms = {
  audit: boolean;
  cacheWrite: boolean;
  staff: boolean;
  directory: boolean;
  directoryWrite: boolean;
  directoryVerify: boolean;
  sandbox: boolean;
  moderate: boolean;
  networkManage: boolean;
  networkVerify: boolean;
  reading: boolean;
};

function permsOf(admin: Admin): Perms {
  const has = (p: string) => admin.permissions.includes(p);
  return {
    audit: has("security/audit/read"),
    cacheWrite: has("ops/cache/write"),
    staff: has("staff/manage"),
    directory: has("directory/write") || has("directory/verify"),
    directoryWrite: has("directory/write"),
    directoryVerify: has("directory/verify"),
    sandbox: has("sandbox/write"),
    moderate: has("community/moderate"),
    networkManage: has("network/manage"),
    networkVerify: has("network/verify"),
    reading: canReviewReading(admin),
  };
}

/**
 * InternalConsole is the staff surface: a sidebar of the pages the role may
 * use. Each page loads its own data when it is shown.
 */
function InternalConsole({
  admin,
  onSignOut,
  onExpired,
}: {
  admin: Admin;
  onSignOut: () => void;
  onExpired: () => void;
}) {
  const w = useWords();
  const { path, navigate } = useRouter();
  const perms = permsOf(admin);
  const [signingOut, setSigningOut] = useState(false);
  const [error, setError] = useState("");
  const [tasks, setTasks] = useState<Tasks>({});

  const items: NavItem[] = [
    { path: "/operator", label: w("Beranda", "Home") },
    ...(perms.networkManage || perms.networkVerify
      ? [
          {
            path: "/operator/network",
            label: w("Jaringan profesional", "Professional network"),
            count: (tasks.network?.pendingOrgs ?? 0) + (tasks.network?.pendingLicenses ?? 0),
          },
        ]
      : []),
    ...(perms.moderate ? [{ path: "/operator/moderation", label: w("Moderasi", "Moderation"), count: tasks.moderation }] : []),
    ...(perms.directory || perms.sandbox
      ? [{ path: "/operator/content", label: w("Konten", "Content"), count: tasks.directoryPending }]
      : []),
    ...(perms.reading ? [{ path: "/operator/reading", label: w("Tinjauan bacaan", "Reading reviews") }] : []),
    { path: "/operator/system", label: w("Sistem", "System") },
  ];
  const allowed = items.some((i) => i.path === path);
  const active = allowed ? path : "/operator";
  useEffect(() => {
    if (!allowed) navigate("/operator");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [allowed]);

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

  const loadTasks = useCallback(async () => {
    try {
      setTasks(await adminApi.tasks());
    } catch (err) {
      if (err instanceof AdminError && err.status === 401) onExpired();
    }
  }, [onExpired]);
  useEffect(() => {
    void loadTasks();
  }, [loadTasks, active]);

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

  return (
    <Shell admin={admin} items={items} active={active} signingOut={signingOut} onSignOut={() => void signOut()}>
      {error && (
        <p className="op-alert" role="alert">
          {error}
        </p>
      )}
      {active === "/operator/network" ? (
        <NetworkPage canManage={perms.networkManage} canVerify={perms.networkVerify} onExpired={onExpired} />
      ) : active === "/operator/moderation" ? (
        <Page
          title={w("Moderasi", "Moderation")}
          lead={w(
            "Tulisan ruang berbagi yang ditahan, disembunyikan, atau dilaporkan. Penulis tidak pernah ditampilkan.",
            "Sharing-space posts that were held, hidden or reported. Authors are never shown.",
          )}
        >
          <CommunityPanel onExpired={onExpired} />
        </Page>
      ) : active === "/operator/content" ? (
        <ContentPage perms={perms} onError={handleError} />
      ) : active === "/operator/reading" ? (
        <ReadingPage admin={admin} onSignOut={onSignOut} onExpired={onExpired} embedded />
      ) : active === "/operator/system" ? (
        <SystemPage admin={admin} perms={perms} onError={handleError} />
      ) : (
        <HomePage perms={perms} tasks={tasks} onError={handleError} />
      )}
    </Shell>
  );
}

function HomePage({ perms, tasks, onError }: { perms: Perms; tasks: Tasks; onError: (err: unknown) => void }) {
  const w = useWords();
  const fmt = useFormatters();
  const [overview, setOverview] = useState<Overview | null>(null);
  const [cache, setCache] = useState<CacheHealth | null>(null);

  const load = useCallback(async () => {
    try {
      const [ov, health] = await Promise.all([adminApi.overview(), adminApi.cacheHealth()]);
      setOverview(ov);
      setCache(health);
    } catch (err) {
      onError(err);
    }
  }, [onError]);
  useEffect(() => {
    void load();
  }, [load]);

  const rows: { count: number; label: string; to: string }[] = [];
  const n = tasks.network;
  if (n && (perms.networkVerify || perms.networkManage)) {
    rows.push({ count: n.pendingOrgs, label: w("organisasi menunggu verifikasi", "organisations waiting for verification"), to: "/operator/network" });
    rows.push({ count: n.pendingLicenses, label: w("lisensi praktisi menunggu dicek", "practitioner licences waiting for a check"), to: "/operator/network" });
    rows.push({ count: n.openInvites, label: w("undangan belum diaktifkan", "invitations not activated yet"), to: "/operator/network" });
  }
  if (tasks.moderation !== undefined) {
    rows.push({ count: tasks.moderation, label: w("tulisan menunggu moderasi", "posts waiting for moderation"), to: "/operator/moderation" });
  }
  if (tasks.directoryPending !== undefined) {
    rows.push({ count: tasks.directoryPending, label: w("entri direktori menunggu tinjauan", "directory entries waiting for review"), to: "/operator/content" });
  }
  const waiting = rows.filter((r) => r.count > 0);

  return (
    <Page
      title={w("Beranda", "Home")}
      lead={w(
        "Yang menunggu tindakanmu dan keadaan server. Cerita, jurnal, dan prompt pengguna tidak pernah ditampilkan di konsol ini.",
        "What is waiting for you and how the server is doing. User stories, journals and prompts are never shown in this console.",
      )}
      actions={
        <button type="button" className="op-btn op-btn--quiet" onClick={() => void load()}>
          {w("Muat ulang", "Refresh")}
        </button>
      }
    >
      <section className="op-section" aria-labelledby="op-tasks">
        <h2 id="op-tasks">{w("Perlu tindakan", "Needs action")}</h2>
        {waiting.length === 0 ? (
          <p className="op-hint">{w("Tidak ada yang menunggu.", "Nothing is waiting.")}</p>
        ) : (
          <ul className="op-tasks">
            {waiting.map((r) => (
              <li key={r.label}>
                <OpLink to={r.to}>
                  <span className="op-task-count">{fmt.count(r.count)}</span>
                  <span className="op-task-label">{r.label}</span>
                </OpLink>
              </li>
            ))}
          </ul>
        )}
      </section>

      <section className="op-section" aria-labelledby="op-summary">
        <h2 id="op-summary">{w("Ringkasan", "Overview")}</h2>
        <div className="op-cards">
          <Kpi label={w("Staf aktif", "Active staff")} value={overview ? fmt.count(overview.activeStaff) : "—"} />
          <Kpi label={w("Sesi operator aktif", "Live operator sessions")} value={overview ? fmt.count(overview.liveSessions) : "—"} />
          <Kpi label={w("Aksi audit 24 jam", "Audit actions, 24 h")} value={overview ? fmt.count(overview.auditLast24h) : "—"} />
          <Kpi label={w("Cache publik", "Public cache")} value={cache ? cacheState(cache, w) : "—"} />
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

      {overview?.aiSpend && <SpendPanel spend={overview.aiSpend} byTask={overview.aiSpendByTask} />}
    </Page>
  );
}

function ContentPage({ perms, onError }: { perms: Perms; onError: (err: unknown) => void }) {
  const w = useWords();
  const [directory, setDirectory] = useState<DirectoryEntry[] | null>(null);
  const [careAudit, setCareAudit] = useState<CareAudit[] | null>(null);
  const [sandbox, setSandbox] = useState<SandboxRequest[] | null>(null);
  const [busy, setBusy] = useState("");
  const [notice, setNotice] = useState("");

  const reload = useCallback(async () => {
    const jobs: Promise<void>[] = [];
    if (perms.directory) {
      jobs.push(adminApi.directory().then(setDirectory));
      jobs.push(adminApi.careAudit().then(setCareAudit));
    }
    if (perms.sandbox) jobs.push(adminApi.sandbox().then(setSandbox));
    await Promise.all(jobs);
  }, [perms.directory, perms.sandbox]);
  useEffect(() => {
    reload().catch(onError);
  }, [reload, onError]);

  const run = async (key: string, job: () => Promise<unknown>, done: string) => {
    setBusy(key);
    setNotice("");
    try {
      await job();
      setNotice(done);
      await reload();
    } catch (err) {
      onError(err);
    } finally {
      setBusy("");
    }
  };

  return (
    <Page
      title={w("Konten", "Content")}
      lead={w(
        "Direktori bantuan terencana yang bersumber dan sandbox simulasinya. Persetujuan bacaan ada di halaman Tinjauan bacaan.",
        "The sourced planned-support directory and its simulation sandbox. Reading approvals are on the Reading reviews page.",
      )}
    >
      <p className="op-notice" role="status" hidden={!notice}>
        {notice}
      </p>
      {perms.directory && (
        <DirectoryPanel
          entries={directory}
          audit={careAudit}
          busy={busy}
          canWrite={perms.directoryWrite}
          canVerify={perms.directoryVerify}
          onCreate={(input) =>
            void run(
              "directory-create",
              () => adminApi.createDirectory(input),
              w("Draf direktori dibuat. Ajukan untuk tinjauan setelah sumbernya lengkap.", "Directory draft created. Submit it after the source is complete."),
            )
          }
          onAction={(id, action, version) =>
            void run(
              `directory:${id}`,
              () => adminApi.directoryAction(id, action, version),
              action === "verify"
                ? w("Entri diterbitkan. Cache publik akan membaca versi ini.", "Entry published. The public view reads this version.")
                : w("Status direktori diperbarui.", "Directory status updated."),
            )
          }
        />
      )}
      {perms.sandbox && (
        <SandboxPanel
          requests={sandbox}
          busy={busy}
          onCreate={(input) =>
            void run(
              "sandbox-create",
              () => adminApi.createSandbox(input),
              w("Permintaan simulasi dibuat. Tidak ada pengiriman ke fasilitas.", "Simulation request created. Nothing was sent to a facility."),
            )
          }
          onTransition={(id, version, status) =>
            void run(
              `sandbox:${id}`,
              () => adminApi.transitionSandbox(id, version, status),
              w("Status simulasi diperbarui. Tidak ada efek eksternal.", "Simulation status updated. No external effect."),
            )
          }
        />
      )}
      {perms.reading && (
        <p className="op-hint">
          <OpLink to="/operator/reading">{w("Buka tinjauan bacaan", "Open reading reviews")}</OpLink>
        </p>
      )}
    </Page>
  );
}

function SystemPage({ admin, perms, onError }: { admin: Admin; perms: Perms; onError: (err: unknown) => void }) {
  const w = useWords();
  const fmt = useFormatters();
  const [overview, setOverview] = useState<Overview | null>(null);
  const [cache, setCache] = useState<CacheHealth | null>(null);
  const [events, setEvents] = useState<AuditEvent[] | null>(null);
  const [staff, setStaff] = useState<Staff[] | null>(null);
  const [orgs, setOrgs] = useState<CareOrg[]>([]);
  const [revoking, setRevoking] = useState("");
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState("");
  const [category, setCategory] = useState("all");
  const filterId = useId();

  const load = useCallback(async () => {
    try {
      const jobs: Promise<void>[] = [
        adminApi.overview().then(setOverview),
        adminApi.cacheHealth().then(setCache),
      ];
      if (perms.audit) jobs.push(adminApi.audit(100).then(setEvents));
      if (perms.staff) jobs.push(adminApi.staff().then(setStaff));
      if (perms.staff && (perms.networkManage || perms.networkVerify)) {
        jobs.push(adminApi.networkOrgs().then((r) => setOrgs(r.orgs)));
      }
      await Promise.all(jobs);
    } catch (err) {
      onError(err);
    }
  }, [onError, perms.audit, perms.staff, perms.networkManage, perms.networkVerify]);
  useEffect(() => {
    void load();
  }, [load]);

  const revokeStaff = async (username: string) => {
    setRevoking(username);
    setNotice("");
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
      onError(err);
    } finally {
      setRevoking("");
    }
  };

  const invalidate = async () => {
    setBusy(true);
    setNotice("");
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
      onError(err);
    } finally {
      setBusy(false);
    }
  };

  const internal = (staff ?? []).filter((s) => !s.careOrgId && !["partner_admin", "practitioner", "partner_staff"].includes(s.role));
  const partners = (staff ?? []).filter((s) => s.careOrgId || ["partner_admin", "practitioner", "partner_staff"].includes(s.role));
  const shownEvents = (events ?? []).filter((e) => category === "all" || actionCategory(e.action) === category);

  return (
    <Page
      title={w("Sistem", "System")}
      lead={w("Anggaran AI, cache, akun, dan jejak audit.", "AI budget, cache, accounts and the audit trail.")}
      actions={
        <button type="button" className="op-btn op-btn--quiet" onClick={() => void load()}>
          {w("Muat ulang", "Refresh")}
        </button>
      }
    >
      <p className="op-notice" role="status" hidden={!notice}>
        {notice}
      </p>
      <SpendPanel spend={overview?.aiSpend} byTask={overview?.aiSpendByTask} />

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
            <span className={`op-dot${cache?.ok ? " op-dot--ok" : ""}`} aria-hidden="true" />
            {cache
              ? cache.ok
                ? w("Sehat", "Healthy")
                : cache.enabled
                  ? w("Tidak tersedia", "Unavailable")
                  : w("Tidak aktif", "Disabled")
              : w("Memuat…", "Loading…")}
          </span>
          {cache?.latencyMs !== undefined && <span className="op-hint">{`${cache.latencyMs} ms`}</span>}
          {perms.cacheWrite ? (
            <button
              type="button"
              className="op-btn op-btn--secondary"
              onClick={() => void invalidate()}
              disabled={busy || !cache?.enabled}
              aria-busy={busy || undefined}
            >
              {busy ? w("Membersihkan…", "Invalidating…") : w("Bersihkan cache bantuan", "Invalidate help cache")}
            </button>
          ) : (
            <span className="op-hint">{w("Peranmu tidak dapat membersihkan cache.", "Your role can't invalidate the cache.")}</span>
          )}
        </div>
      </section>

      {perms.staff && (
        <StaffPanel admin={admin} staff={staff === null ? null : internal} busy={revoking} onRevoke={(u) => void revokeStaff(u)} />
      )}

      {perms.staff && partners.length > 0 && (
        <section className="op-section" aria-labelledby="op-partner-accounts">
          <h2 id="op-partner-accounts">{w("Akun mitra", "Partner accounts")}</h2>
          <p>
            {w(
              "Akun klinik dan praktisi, dikelompokkan per organisasi. Undangan, lisensi, dan detailnya ada di Jaringan profesional.",
              "Clinic and practitioner accounts, grouped by organisation. Invitations, licences and details are on the Professional network page.",
            )}
          </p>
          {Array.from(new Set(partners.map((p) => p.careOrgId ?? ""))).map((orgId) => (
            <div key={orgId || "none"} className="op-network-members">
              <h3>{orgs.find((o) => o.id === orgId)?.name ?? w("Organisasi", "Organisation")}</h3>
              <ul className="op-rows">
                {partners
                  .filter((p) => (p.careOrgId ?? "") === orgId)
                  .map((p) => (
                    <li key={p.username}>
                      <span className="op-row-name">
                        {p.username} · {roleLabel(p.role, w)}
                      </span>
                      <span className="op-row-value op-hint">
                        {statusLabel(p.status, w)} · {p.lastLoginAt ? fmt.date(p.lastLoginAt) : w("Belum pernah masuk", "Never signed in")}
                      </span>
                    </li>
                  ))}
              </ul>
            </div>
          ))}
          {(perms.networkManage || perms.networkVerify) && (
            <p className="op-hint">
              <OpLink to="/operator/network">{w("Buka Jaringan profesional", "Open the Professional network")}</OpLink>
            </p>
          )}
        </section>
      )}

      <section className="op-section" aria-labelledby="op-audit">
        <h2 id="op-audit">{w("Audit operator", "Operator audit")}</h2>
        {!perms.audit ? (
          <p className="op-hint">{w("Peranmu tidak memiliki izin membaca audit.", "Your role isn't allowed to read the audit trail.")}</p>
        ) : (
          <>
            <div className="op-filter">
              <label htmlFor={filterId}>{w("Kategori", "Category")}</label>
              <select id={filterId} className="op-input" value={category} onChange={(e) => setCategory(e.target.value)}>
                {CATEGORIES.map(([k, id, en]) => (
                  <option key={k} value={k}>
                    {w(id, en)}
                  </option>
                ))}
              </select>
            </div>
            {events === null ? (
              <p className="op-hint">{w("Memuat audit…", "Loading audit…")}</p>
            ) : shownEvents.length === 0 ? (
              <p className="op-hint">{w("Belum ada tindakan di kategori ini.", "No actions in this category yet.")}</p>
            ) : (
              <ol className="op-events">
                {shownEvents.map((e) => (
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
          </>
        )}
      </section>
    </Page>
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
      <h2 id="op-staff">{w("Staf internal", "Internal staff")}</h2>
      <p>
        {w(
          "Hanya identitas staf internal Sela. Tidak ada kata sandi, kode MFA, atau data pengguna di daftar ini. Akun staf internal dibuat lewat CLI di server.",
          "Sela's internal staff identities only. No passwords, MFA secrets or user data appear in this list. Internal staff accounts are created with the CLI on the server.",
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
