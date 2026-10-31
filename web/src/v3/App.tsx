import { lazy, Suspense, useEffect, useState, type ReactNode } from "react";
import { useI18n } from "../i18n";
import { useRouter } from "../lib/router";
import { AnnounceProvider, useWords } from "./shared";
import { PrefsProvider } from "./prefs";
import { SessionProvider, useSession } from "./session";
import { AppShell } from "./shell";
import "./styles.css";
import { Loading } from "./ui";
import { HomePage } from "./screens/Home";
import { DashboardPage } from "./screens/Dashboard";
import { TalkPage } from "./screens/Talk";
import { NotesPage } from "./screens/Notes";
import { AboutPage } from "./screens/About";
const SupportPage = lazy(() =>
  import("./screens/Support").then((m) => ({ default: m.SupportPage })),
);
const SummariesPage = lazy(() => import("./screens/Continuity").then(m => ({ default: m.SummariesPage })));
const SupportFeaturePage = lazy(() =>
  import("./screens/Support").then((m) => ({ default: m.SupportFeaturePage })),
);
const HelpPage = lazy(() =>
  import("./screens/Help").then((m) => ({ default: m.HelpPage })),
);
const ReadingPage = lazy(() =>
  import("./screens/Reading").then((m) => ({ default: m.ReadingPage })),
);
const ReadingItemPage = lazy(() =>
  import("./screens/Reading").then((m) => ({ default: m.ReadingItemPage })),
);
const SharesPage = lazy(() =>
  import("./screens/Shares").then((m) => ({ default: m.SharesPage })),
);
const RecipientPage = lazy(() =>
  import("./screens/Shares").then((m) => ({ default: m.RecipientPage })),
);
const PoliciesPage = lazy(() =>
  import("./screens/Policies").then((m) => ({ default: m.PoliciesPage })),
);
const PrivacyPage = lazy(() =>
  import("./screens/Settings").then((m) => ({ default: m.PrivacyPage })),
);
const DisplayPage = lazy(() =>
  import("./screens/Settings").then((m) => ({ default: m.DisplayPage })),
);
const AccountPage = lazy(() =>
  import("./screens/Settings").then((m) => ({ default: m.AccountPage })),
);
const SignInPage = lazy(() =>
  import("./screens/Auth").then((m) => ({ default: m.SignInPage })),
);
const RecoverPage = lazy(() =>
  import("./screens/Auth").then((m) => ({ default: m.RecoverPage })),
);
const UpgradePage = lazy(() =>
  import("./screens/Auth").then((m) => ({ default: m.UpgradePage })),
);
const PlannedCarePage = lazy(() =>
  import("./screens/PlannedCare").then((m) => ({ default: m.PlannedCarePage })),
);
const OnboardingPage = lazy(() =>
  import("./screens/Onboarding").then((m) => ({ default: m.OnboardingPage })),
);

type OnboardingDraft = {
  ageBand: string;
  conversationDefault: "save" | "dont_save" | null;
  codes: string[] | null;
};
const emptyDraft: OnboardingDraft = {
  ageBand: "",
  conversationDefault: null,
  codes: null,
};
import {
  AgeBlockedPage,
  DataDeletedPage,
  GuardianGatePage,
  NotFoundPage,
  PolicyGatePage,
  SessionEndedPage,
} from "./screens/System";

export function V3App() {
  return (
    <PrefsProvider>
      <AnnounceProvider>
        <SessionProvider>
          <AppRoutes />
        </SessionProvider>
      </AnnounceProvider>
    </PrefsProvider>
  );
}

const OWNER_PREFIXES = ["/home", "/talk", "/notes", "/support", "/settings/privacy", "/settings/account"];

function titleFor(path: string, lang: string): string {
  const t = (id: string, en: string) => (lang === "en" ? en : id);
  if (path === "/" || path === "") return t("Dashboard Sela", "Sela dashboard");
  if (path === "/home") return t("Beranda", "Home");
  if (path === "/dashboard") return t("Dashboard Sela", "Sela dashboard");
  if (path === "/talk") return t("Ruang cerita", "Talk");
  if (path.startsWith("/talk/")) return t("Percakapan", "Conversation");
  if (path === "/notes") return t("Catatan pribadiku", "My personal notes");
  if (path === "/notes/write") return t("Tulis catatan", "Write a note");
  if (path === "/notes/check-in") return t("Cek perasaan", "Check-in");
  if (path === "/notes/patterns") return t("Pola dari cek perasaanmu", "Patterns in your check-ins");
  if (path === "/notes/memory") return t("Yang Sela ingat", "What Sela remembers");
  if (path.startsWith("/notes/")) return t("Catatan", "Note");
  if (path === "/support") return t("Dukungan", "Support");
  if (path === "/support/directory") return t("Bantuan terencana", "Planned support");
  if (path.startsWith("/support/summaries")) return t("Ringkasan untuk konsultasi", "Summary for a consultation");
  if (path.startsWith("/support/shares")) return t("Status berbagi", "Sharing status");
  if (path === "/help") return t("Bantuan sekarang", "Help now");
  if (path === "/reading") return t("Bacaan & latihan kecil", "Reading & small exercises");
  if (path.startsWith("/reading/")) return t("Bacaan", "Reading");
  if (path === "/about") return t("Tentang Sela", "About Sela");
  if (path === "/policies" || path.startsWith("/policies/")) return t("Kebijakan", "Policies");
  if (path === "/signin") return t("Masuk", "Sign in");
  if (path === "/recover") return t("Pulihkan akun", "Recover your account");
  if (path === "/start/age") return t("Usia", "Age");
  if (path === "/start/teen") return t("Batas kerahasiaan", "Limits of confidentiality");
  if (path === "/start/storage") return t("Penyimpanan", "Storage");
  if (path === "/start/account") return t("Buat akun", "Create an account");
  if (path === "/start/codes") return t("Kode pemulihan", "Recovery codes");
  if (path === "/start/guardian") return t("Persetujuan wali", "Guardian consent");
  if (path === "/start/ai") return t("Balasan AI", "AI replies");
  if (path === "/start/blocked") return t("Belum tersedia", "Not available");
  if (path === "/settings/privacy") return t("Privasi & data", "Privacy & data");
  if (path === "/settings/display") return t("Tampilan & aksesibilitas", "Display & accessibility");
  if (path === "/settings/account" || path.startsWith("/settings/account/")) return t("Akun", "Account");
  if (path === "/recipient") return t("Halaman penerima", "Recipient page");
  return t("Halaman tidak ditemukan", "Page not found");
}

function isOwnerPath(path: string): boolean {
  // Planned support is public: it needs no session and stores no choice.
  if (path === "/support/directory") return false;
  return OWNER_PREFIXES.some((p) => path === p || path.startsWith(p + "/"));
}

function readingSlug(path: string): string {
  try {
    return decodeURIComponent(path.slice("/reading/".length));
  } catch {
    return "";
  }
}

function isGuardianExempt(path: string): boolean {
  return (
    path.startsWith("/settings/privacy") ||
    path.startsWith("/settings/display") ||
    path.startsWith("/settings/account") ||
    path === "/help" ||
    path.startsWith("/reading") ||
    path.startsWith("/policies") ||
    path.startsWith("/start/guardian")
  );
}

function AppRoutes() {
  const w = useWords();
  const { path, navigate, query } = useRouter();
  const { lang } = useI18n();
  const { user, loading, policies, options, expired, refresh, setUser } = useSession();
  const [draft, setDraft] = useState<OnboardingDraft>(emptyDraft);
  const [deleted, setDeleted] = useState(false);

  useEffect(() => {
    if (!loading && user && (path === "/" || path === "/signin" || path === "/recover")) {
      navigate("/home", { replace: true });
    }
  }, [loading, navigate, path, user]);

  useEffect(() => {
    if (!loading && !user && isOwnerPath(path) && !expired) {
      navigate("/signin?next=" + encodeURIComponent(path), { replace: true });
    }
  }, [expired, loading, navigate, path, user]);

  useEffect(() => {
    if (!user && (path === "/start/codes" || path === "/start/guardian" || path === "/start/ai")) {
      navigate("/start/age", { replace: true });
    }
  }, [navigate, path, user]);

  useEffect(() => {
    if (deleted && path === "/") {
      // The deleted state is a real end screen at "/", not a trap for every
      // route. Navigating to Help or a policy must still work.
      return;
    }
    if (path === "/checkin" || path === "/timeline") {
      navigate(path === "/checkin" ? "/notes/check-in" : "/notes", { replace: true });
    }
    if (path === "/settings") {
      navigate(user ? "/settings/privacy" : "/signin?next=%2Fsettings%2Fprivacy", { replace: true });
    }
  }, [navigate, path, user]);

  useEffect(() => {
    if (deleted && path.startsWith("/start/")) setDeleted(false);
  }, [deleted, path]);

  const owner = isOwnerPath(path);
  const title = titleFor(path, lang);
  const focused = path.startsWith("/talk/") && path !== "/talk";
  const room = Boolean(focused && path !== "/talk/new");

  let screen: ReactNode;

  if (loading && !policies?.terms_privacy) {
    screen = <Loading label={w("Menyiapkan ruangmu…", "Getting your space ready…")} />;
  } else if (deleted && path === "/") {
    screen = <DataDeletedPage onRestart={() => setDeleted(false)} />;
  } else if (path === "/" || path === "") {
    screen = <DashboardPage signedIn={Boolean(user)} />;
  } else if (path === "/dashboard") {
    screen = <DashboardPage signedIn={Boolean(user)} />;
  } else if (path === "/about") {
    screen = <AboutPage />;
  } else if (path === "/help") {
    screen = <HelpPage />;
  } else if (path === "/reading") {
    screen = <ReadingPage />;
  } else if (path.startsWith("/reading/")) {
    screen = <ReadingItemPage slug={readingSlug(path)} />;
  } else if (path === "/policies") {
    screen = <PoliciesPage />;
  } else if (path.startsWith("/policies/")) {
    screen = <PoliciesPage purpose={path.slice("/policies/".length)} />;
  } else if (path === "/signin") {
    screen = <SignInPage />;
  } else if (path === "/recover") {
    screen = <RecoverPage />;
  } else if (path === "/settings/display") {
    screen = <DisplayPage />;
  } else if (path.startsWith("/start/")) {
    if (path === "/start/blocked") {
      screen = <AgeBlockedPage reason={query.get("reason") ?? "under15"} />;
    } else if (!user && path !== "/start/age" && path !== "/start/teen" && path !== "/start/storage" && path !== "/start/account") {
      screen = <AgeBlockedPage reason="band" />;
    } else {
      const step = path.slice("/start/".length);
      screen = (
        <OnboardingPage
          step={step}
          draft={draft}
          setDraft={setDraft}
          options={options}
          policies={policies}
          onCreated={refresh}
        />
      );
    }
  } else if (path === "/recipient") {
    screen = <RecipientPage />;
  } else if (owner && !user) {
    screen = expired ? (
      <SessionEndedPage guest={sessionGuest()} />
    ) : (
      <Loading label={w("Membuka…", "Opening…")} />
    );
  } else if (user && user.ageBand === "15_17" && user.guardianConsent?.required && user.guardianConsent.status !== "active" && !isGuardianExempt(path)) {
    screen = <GuardianGatePage user={user} />;
  } else if (user?.actionsRequired?.some((a) => a.startsWith("accept_policy:")) && !isPolicyExempt(path)) {
    screen = <PolicyGatePage user={user} />;
  } else if (user && path === "/home") {
    screen = <HomePage user={user} />;
  } else if (user && (path === "/talk" || path.startsWith("/talk/"))) {
    screen = <TalkPage user={user} />;
  } else if (user && (path === "/notes" || path.startsWith("/notes/"))) {
    screen = <NotesPage user={user} path={path} />;
  } else if (path === "/support/directory") {
    screen = <PlannedCarePage />;
  } else if (user && path === "/support") {
    screen = <SupportPage />;
  } else if (user && (path.startsWith("/support/summaries") || path === "/support/prepare" || path === "/support/after")) {
    screen = <SummariesPage path={path.startsWith("/support/summaries") ? path : "/support/summaries"} />;
  } else if (user && path.startsWith("/support/shares")) {
    screen = <SharesPage />;
  } else if (user && path.startsWith("/support/")) {
    const feature = path.includes("/prepare")
      ? "prepare"
      : path.includes("/share")
        ? "share"
        : path.includes("/after")
          ? "after"
          : path.startsWith("/support/shares/")
            ? "status"
            : "summary";
    screen = <SupportFeaturePage feature={feature} />;
  } else if (user && path === "/settings/privacy") {
    screen = (
      <PrivacyPage
        user={user}
        policies={policies}
        onDeleted={() => {
          setUser(null);
          setDeleted(true);
          navigate("/", { replace: true });
        }}
      />
    );
  } else if (user && path === "/settings/account/upgrade") {
    screen =
      user.kind === "guest" ? (
        <UpgradePage />
      ) : (
        <AccountPage
          user={user}
          options={options}
          onDeleted={() => {
            setUser(null);
            setDeleted(true);
            navigate("/", { replace: true });
          }}
        />
      );
  } else if (user && path === "/settings/account") {
    screen = (
      <AccountPage
        user={user}
        options={options}
        onDeleted={() => {
          setUser(null);
          setDeleted(true);
          navigate("/", { replace: true });
        }}
      />
    );
  } else {
    screen = <NotFoundPage />;
  }

  return (
    <AppShell title={title} focused={focused} room={room}>
      <Suspense
        fallback={<Loading label={w("Memuat halaman…", "Loading the page…")} />}
      >
        {screen}
      </Suspense>
    </AppShell>
  );
}

function isPolicyExempt(path: string): boolean {
  return (
    path.startsWith("/settings/privacy") ||
    path.startsWith("/settings/display") ||
    path.startsWith("/settings/account") ||
    path === "/help" ||
    path.startsWith("/reading") ||
    path.startsWith("/policies")
  );
}

function sessionGuest(): boolean {
  try {
    return window.sessionStorage.getItem("sela.userKind") === "guest";
  } catch {
    return false;
  }
}
