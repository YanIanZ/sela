import { useWords } from "../v3/shared";
import { useI18n } from "../i18n";
import type { Admin } from "./api";

type Words = (id: string, en: string) => string;

const ROLE_LABELS: Record<string, [string, string]> = {
  owner: ["Pemilik platform", "Platform owner"],
  security_admin: ["Admin keamanan", "Security admin"],
  directory_editor: ["Editor direktori", "Directory editor"],
  directory_reviewer: ["Peninjau direktori", "Directory reviewer"],
  research_editor: ["Editor riset", "Research editor"],
  safety_reviewer: ["Peninjau keselamatan", "Safety reviewer"],
  ops: ["Operasional", "Ops"],
};

export function roleLabel(role: string, w: Words): string {
  const label = ROLE_LABELS[role];
  return label ? w(label[0], label[1]) : role;
}

// The reading library is support content: both editorial reviewer roles may
// inspect it. The server re-checks the same two permissions on every call.
export function canReviewReading(admin: Admin): boolean {
  return (
    admin.permissions.includes("research/review") ||
    admin.permissions.includes("safety-content/review")
  );
}

export function LanguageToggle() {
  const { lang, setLang } = useI18n();
  const w = useWords();
  return (
    <div className="op-lang" role="group" aria-label={w("Bahasa", "Language")}>
      <button
        type="button"
        lang="id"
        aria-pressed={lang === "id"}
        onClick={() => setLang("id")}
      >
        {"Indonesia"}
      </button>
      <button
        type="button"
        lang="en"
        aria-pressed={lang === "en"}
        onClick={() => setLang("en")}
      >
        {"English"}
      </button>
    </div>
  );
}

export function OperatorHeader({
  admin,
  active,
  signingOut,
  onSignOut,
}: {
  admin: Admin;
  active: "console" | "reading";
  signingOut?: boolean;
  onSignOut: () => void;
}) {
  const w = useWords();
  return (
    <header className="op-header">
      <div className="op-brand">
        <span className="op-brand-word">{w("Sela", "Sela")}</span>
        <span className="op-badge">{w("Operator", "Operator")}</span>
      </div>
      <nav className="op-nav" aria-label={w("Navigasi operator", "Operator navigation")}>
        <a href="/operator" aria-current={active === "console" ? "page" : undefined}>
          {w("Konsol", "Console")}
        </a>
        {canReviewReading(admin) && (
          <a
            href="/operator/reading"
            aria-current={active === "reading" ? "page" : undefined}
          >
            {w("Tinjauan bacaan", "Reading reviews")}
          </a>
        )}
      </nav>
      <div className="op-header-side">
        <LanguageToggle />
        <span className="op-user">
          <strong>{admin.username}</strong>
          <span className="op-user-role">{roleLabel(admin.role, w)}</span>
        </span>
        <button
          type="button"
          className="op-btn op-btn--quiet"
          onClick={onSignOut}
          disabled={signingOut}
        >
          {signingOut ? w("Keluar…", "Signing out…") : w("Keluar", "Sign out")}
        </button>
      </div>
    </header>
  );
}
