import { useEffect, useId, useRef, useState, type FormEvent } from "react";
import { useWords } from "../v3/shared";
import { AdminError, adminApi, type Activation, type Admin } from "./api";
import { LanguageToggle, roleLabel } from "./chrome";

type Words = (id: string, en: string) => string;

function activationError(err: unknown, w: Words): string {
  if (err instanceof AdminError) {
    if (err.status === 410 || err.code === "invite_invalid")
      return w(
        "Tautan ini sudah dipakai, dibatalkan, atau kedaluwarsa. Minta tautan baru ke admin klinikmu atau tim Sela.",
        "This link was already used, revoked or has expired. Ask your clinic administrator or the Sela team for a new one.",
      );
    if (err.code === "password_weak")
      return w(
        "Kata sandi terlalu lemah. Pakai minimal 15 karakter, tidak mengandung nama pengguna, dan tidak mudah ditebak.",
        "The password is too weak. Use at least 15 characters, not containing the username, and not easy to guess.",
      );
    if (err.status === 401)
      return w("Kode dari aplikasi authenticator tidak cocok. Tunggu kode berikutnya lalu coba lagi.", "The authenticator code doesn't match. Wait for the next code and try again.");
    if (err.status === 429) return w("Terlalu banyak percobaan. Tunggu sebentar.", "Too many attempts. Wait a moment.");
    if (err.code === "network") return w("Tidak dapat menghubungi server.", "Can't reach the server.");
  }
  return w("Aktivasi gagal. Coba lagi.", "Activation failed. Try again.");
}

/**
 * ActivationPage lets an invited partner set their own password and
 * authenticator (D-079). The token comes from the URL fragment, which is
 * dropped from the address bar at once and never sent in a URL.
 */
export function ActivationPage({ onDone }: { onDone: (admin: Admin) => void }) {
  const w = useWords();
  const [token] = useState(() => window.location.hash.replace(/^#/, ""));
  const [activation, setActivation] = useState<Activation | null>(null);
  const [loadError, setLoadError] = useState("");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [code, setCode] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [copied, setCopied] = useState(false);
  const titleRef = useRef<HTMLHeadingElement>(null);
  const pwId = useId();
  const confirmId = useId();
  const codeId = useId();
  const pwHint = useId();

  useEffect(() => {
    if (window.location.hash) {
      window.history.replaceState(null, "", window.location.pathname + window.location.search);
    }
    if (!token) {
      setLoadError(activationError(new AdminError(410, "invite_invalid"), w));
      return;
    }
    let live = true;
    adminApi
      .activationStart(token)
      .then((a) => live && setActivation(a))
      .catch((err: unknown) => live && setLoadError(activationError(err, w)));
    return () => {
      live = false;
    };
    // the token is read once on mount
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    titleRef.current?.focus({ preventScroll: true });
  }, [activation, loadError]);

  const mismatch = confirm !== "" && confirm !== password;
  const ready = password.length >= 15 && password === confirm && /^\d{6}$/.test(code);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (!ready || busy) return;
    setBusy(true);
    setError("");
    try {
      const admin = await adminApi.activationComplete(token, password, code);
      window.history.replaceState(null, "", "/operator");
      onDone(admin);
    } catch (err) {
      if (err instanceof AdminError && err.status === 410) setLoadError(activationError(err, w));
      else setError(activationError(err, w));
    } finally {
      setBusy(false);
    }
  };

  const copy = async () => {
    if (!activation) return;
    try {
      await navigator.clipboard.writeText(activation.secret);
      setCopied(true);
    } catch {
      setCopied(false);
    }
  };

  const grouped = activation ? activation.secret.match(/.{1,4}/g)?.join(" ") ?? activation.secret : "";

  return (
    <div className="op op-login">
      <header className="op-login-top">
        <span className="op-brand-word">{w("Sela", "Sela")}</span>
        <span className="op-badge">{w("Operator", "Operator")}</span>
        <LanguageToggle />
      </header>
      <main className="op-login-card op-activate">
        <h1 ref={titleRef} tabIndex={-1} className="op-login-title op-page-title">
          {w("Aktifkan akunmu", "Activate your account")}
        </h1>
        {loadError ? (
          <p className="op-alert" role="alert">
            {loadError}
          </p>
        ) : !activation ? (
          <p className="op-hint" role="status">
            {w("Memeriksa undangan…", "Checking the invitation…")}
          </p>
        ) : (
          <>
            <p className="op-login-lead">
              {w(
                `Kamu diundang sebagai ${roleLabel(activation.role, w)} di ${activation.orgName}. Nama pengguna: ${activation.username}.`,
                `You are invited as ${roleLabel(activation.role, w)} at ${activation.orgName}. Username: ${activation.username}.`,
              )}
            </p>
            <section className="op-section" aria-labelledby="op-act-mfa">
              <h2 id="op-act-mfa">{w("1. Tambahkan ke aplikasi authenticator", "1. Add it to your authenticator app")}</h2>
              <p>
                {w(
                  "Pindai atau masukkan kunci ini di Google Authenticator, Microsoft Authenticator, 1Password, atau aplikasi serupa. Jangan bagikan ke siapa pun.",
                  "Scan or enter this key in Google Authenticator, Microsoft Authenticator, 1Password or a similar app. Don't share it with anyone.",
                )}
              </p>
              <code className="op-secret" aria-label={w("Kunci authenticator", "Authenticator key")}>
                {grouped}
              </code>
              <div className="op-partner-row">
                <button type="button" className="op-btn op-btn--secondary" onClick={() => void copy()}>
                  {copied ? w("Tersalin", "Copied") : w("Salin kunci", "Copy key")}
                </button>
                <a className="op-btn op-btn--quiet" href={activation.otpauthUrl}>
                  {w("Buka di aplikasi authenticator", "Open in an authenticator app")}
                </a>
              </div>
            </section>
            <form className="op-form" onSubmit={(e) => void submit(e)} aria-labelledby="op-act-pw">
              <h2 id="op-act-pw">{w("2. Buat kata sandi dan masukkan kode", "2. Create a password and enter the code")}</h2>
              {error && (
                <p className="op-alert" role="alert">
                  {error}
                </p>
              )}
              <div className="op-field">
                <label htmlFor={pwId}>{w("Kata sandi baru", "New password")}</label>
                <span className="op-hint" id={pwHint}>
                  {w("Minimal 15 karakter, misalnya beberapa kata. Hanya kamu yang tahu; tim Sela tidak bisa melihatnya.", "At least 15 characters, for example a few words. Only you know it; the Sela team can't see it.")}
                </span>
                <input
                  id={pwId}
                  className="op-input"
                  type="password"
                  autoComplete="new-password"
                  required
                  minLength={15}
                  aria-describedby={pwHint}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                />
              </div>
              <div className="op-field">
                <label htmlFor={confirmId}>{w("Ulangi kata sandi", "Repeat the password")}</label>
                <input
                  id={confirmId}
                  className="op-input"
                  type="password"
                  autoComplete="new-password"
                  required
                  aria-invalid={mismatch || undefined}
                  value={confirm}
                  onChange={(e) => setConfirm(e.target.value)}
                />
                {mismatch && <span className="op-hint">{w("Kata sandi belum sama.", "The passwords don't match yet.")}</span>}
              </div>
              <div className="op-field">
                <label htmlFor={codeId}>{w("Kode dari aplikasi authenticator", "Code from the authenticator app")}</label>
                <input
                  id={codeId}
                  className="op-input op-input--code"
                  inputMode="numeric"
                  autoComplete="one-time-code"
                  pattern="[0-9]{6}"
                  maxLength={6}
                  required
                  value={code}
                  onChange={(e) => setCode(e.target.value.replace(/\D/g, ""))}
                />
              </div>
              <button className="op-btn op-btn--primary" type="submit" disabled={busy || !ready} aria-busy={busy || undefined}>
                {busy ? w("Mengaktifkan…", "Activating…") : w("Aktifkan dan masuk", "Activate and sign in")}
              </button>
            </form>
          </>
        )}
        <p className="op-login-foot">
          {w(
            "Sela tidak pernah meminta kata sandi atau kode MFA lewat pesan. Tautan ini hanya bisa dipakai sekali.",
            "Sela never asks for your password or MFA code by message. This link works only once.",
          )}
        </p>
      </main>
    </div>
  );
}
