import { useEffect, useState, type FormEvent } from "react";
import { useRouter } from "../../lib/router";
import { useI18n } from "../../i18n";
import { api, V3Error, type AuthResult } from "../api";
import { useSession } from "../session";
import { useAnnounce, useWords } from "../shared";
import {
  Button,
  CheckboxRow,
  ErrorNotice,
  LinkButton,
  PageHeader,
  PasswordField,
  StatusLine,
  TextField,
} from "../ui";

const SECURE_NEXT = /^\/(?!\/)[\w\-./?=&%#]*$/;

export function SignInPage() {
  const w = useWords();
  const { query, navigate } = useRouter();
  const { refresh } = useSession();
  const { announce } = useAnnounce();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [remember, setRemember] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const next = query.get("next") ?? "";
  const nextPath = SECURE_NEXT.test(next) ? next : "/home";

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const r = await api<AuthResult>("/auth/login", {
        username,
        password,
        rememberDevice: remember,
      });
      window.sessionStorage.setItem("sela.trusted", String(Boolean(r.session?.trustedDevice)));
      await refresh();
      announce("status", w("Masuk berhasil.", "Signed in."));
      navigate(nextPath);
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  };

  const invalid =
    error instanceof V3Error && error.code === "invalid_credentials";

  useEffect(() => {
    if (invalid) document.getElementById("signin-username")?.focus();
  }, [invalid]);

  return (
    <div className="s-auth-card">
      <PageHeader
        back={{ to: "/", label: w("Pengenalan", "Introduction") }}
        title={w("Masuk ke akunmu", "Sign in to your account")}
      />
      <ErrorNotice error={error} />
      <form onSubmit={(e) => void submit(e)}>
        <TextField
          id="signin-username"
          label={w("Nama pengguna", "Username")}
          value={username}
          autoComplete="username"
          required
          error={invalid ? w("Periksa nama pengguna dan kata sandi.", "Check the username and password.") : undefined}
          onChange={(e) => setUsername(e.target.value)}
        />
        <PasswordField
          label={w("Kata sandi", "Password")}
          hint={w(
            "Kamu boleh menempel kata sandi dari pengelola kata sandi.",
            "You can paste your password from a password manager.",
          )}
          value={password}
          autoComplete="current-password"
          required
          error={invalid ? w("Periksa nama pengguna dan kata sandi.", "Check the username and password.") : undefined}
          onChange={(e) => setPassword(e.target.value)}
        />
        <CheckboxRow
          checked={remember}
          onChange={setRemember}
          hint={w(
            "Tetap masuk sampai 7 hari tanpa aktivitas. Jangan pilih ini di perangkat bersama.",
            "Stay signed in for up to 7 days of inactivity. Don't choose this on a shared device.",
          )}
        >
          {w("Ingat perangkat ini", "Remember this device")}
        </CheckboxRow>
        <div className="s-actions">
          <Button type="submit" variant="primary" busy={busy}>
            {busy ? w("Memeriksa…", "Checking…") : w("Masuk", "Sign in")}
          </Button>
        </div>
      </form>
      <div className="s-actions s-mt-4">
        <LinkButton to="/recover" variant="quiet">
          {w("Lupa kata sandi? Pakai kode pemulihan", "Forgot your password? Use a recovery code")}
        </LinkButton>
        <LinkButton to="/start/age" variant="quiet">
          {w("Belum punya akun? Mulai", "No account yet? Get started")}
        </LinkButton>
      </div>
    </div>
  );
}

export function RecoverPage() {
  const w = useWords();
  const { refresh } = useSession();
  const { announce } = useAnnounce();
  const [username, setUsername] = useState("");
  const [code, setCode] = useState("");
  const [password, setPassword] = useState("");
  const [remember, setRemember] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [remaining, setRemaining] = useState<number | null>(null);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const r = await api<AuthResult & { remainingRecoveryCodes: number }>(
        "/auth/recovery",
        {
          username,
          recoveryCode: code,
          newPassword: password,
          rememberDevice: remember,
        },
      );
      setRemaining(r.remainingRecoveryCodes);
      window.sessionStorage.setItem("sela.trusted", String(Boolean(r.session?.trustedDevice)));
      await refresh();
      announce("status", w("Kata sandi sudah diganti.", "Your password has been changed."));
    } catch (e) {
      setError(e);
      if ((e as V3Error).code === "recovery_failed") {
        requestAnimationFrame(() => document.getElementById("recover-code")?.focus());
      }
    } finally {
      setBusy(false);
    }
  };

  if (remaining !== null) {
    return (
      <div className="s-auth-card">
        <PageHeader title={w("Kata sandimu sudah diganti.", "Your password has been changed.")} />
        <p>
          {w(
            `${remaining} kode pemulihan tersisa.`,
            `${remaining} recovery codes left.`,
          )}
        </p>
        {remaining <= 3 && (
          <StatusLine tone="warning">
            {w(
              "Sebaiknya buat kode baru sekarang.",
              "It's a good idea to make new codes now.",
            )}
          </StatusLine>
        )}
        <div className="s-actions">
          <LinkButton to="/settings/account" variant="primary">
            {w("Buat kode baru", "Make new codes")}
          </LinkButton>
          <LinkButton to="/home" variant="secondary">
            {w("Lanjut ke beranda", "Go to Home")}
          </LinkButton>
        </div>
      </div>
    );
  }

  return (
    <div className="s-auth-card">
      <PageHeader
        back={{ to: "/signin", label: w("Masuk", "Sign in") }}
        title={w("Pulihkan akunmu", "Recover your account")}
      />
      <ErrorNotice error={error} />
      <form onSubmit={(e) => void submit(e)}>
        <TextField
          id="signin-username"
          label={w("Nama pengguna", "Username")}
          value={username}
          autoComplete="username"
          required
          onChange={(e) => setUsername(e.target.value)}
        />
        <TextField
          id="recover-code"
          label={w("Kode pemulihan", "Recovery code")}
          hint={w(
            "Huruf besar/kecil, spasi, dan tanda hubung tidak berpengaruh.",
            "Upper or lower case, spaces and hyphens don't matter.",
          )}
          value={code}
          autoComplete="one-time-code"
          required
          onChange={(e) => setCode(e.target.value)}
        />
        <PasswordField
          label={w("Kata sandi baru", "New password")}
          hint={w("Minimal 15 karakter.", "At least 15 characters.")}
          value={password}
          minLength={15}
          autoComplete="new-password"
          required
          onChange={(e) => setPassword(e.target.value)}
        />
        <CheckboxRow
          checked={remember}
          onChange={setRemember}
          hint={w(
            "Tetap masuk sampai 7 hari tanpa aktivitas. Jangan pilih ini di perangkat bersama.",
            "Stay signed in for up to 7 days of inactivity. Don't choose this on a shared device.",
          )}
        >
          {w("Ingat perangkat ini", "Remember this device")}
        </CheckboxRow>
        <p className="s-hint">
          {w("Semua perangkat lain akan dikeluarkan.", "All other devices will be signed out.")}
        </p>
        <Button type="submit" variant="primary" busy={busy}>
          {w("Pulihkan dan masuk", "Recover and sign in")}
        </Button>
      </form>
    </div>
  );
}

export function CodesStep({
  codes,
  onDone,
  onAcknowledge,
}: {
  codes: string[] | null;
  onDone: () => void;
  onAcknowledge: () => Promise<unknown>;
}) {
  const w = useWords();
  const { announce } = useAnnounce();
  const [saved, setSaved] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);

  const continueNext = async () => {
    setBusy(true);
    setError(null);
    try {
      await onAcknowledge();
      announce("status", w("Kode pemulihan dikonfirmasi.", "Recovery codes confirmed."));
      onDone();
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="s-narrow">
      <PageHeader
        title={w("Simpan jalan pulangmu.", "Keep a way back in.")}
        lead={w(
          "Kode ini satu-satunya cara masuk jika kamu lupa kata sandi. Setiap kode hanya bisa dipakai sekali. Sela tidak bisa menampilkannya lagi.",
          "These codes are the only way back in if you forget your password. Each works once. Sela can't show them again.",
        )}
      />
      <ErrorNotice error={error} />
      {codes ? (
        <>
          <div className="s-codes">
            <div className="s-codes-grid">
              {codes.map((c) => (
                <code key={c}>{c}</code>
              ))}
            </div>
          </div>
          <div className="s-actions">
            <Button
              variant="secondary"
              icon="copy"
              onClick={() => {
                void navigator.clipboard
                  ?.writeText(codes.join("\n"))
                  .then(() => announce("status", w("Kode disalin.", "Codes copied.")))
                  .catch(() => setError(w("Salin kode secara manual.", "Copy the codes manually.")));
              }}
            >
              {w("Salin semua", "Copy all")}
            </Button>
            <Button
              variant="secondary"
              icon="download"
              onClick={() => {
                const url = URL.createObjectURL(
                  new Blob([codes.join("\n")], { type: "text/plain" }),
                );
                const a = document.createElement("a");
                a.href = url;
                a.download = "sela-recovery-codes.txt";
                a.click();
                window.setTimeout(() => URL.revokeObjectURL(url), 1000);
              }}
            >
              {w("Unduh .txt", "Download .txt")}
            </Button>
            <Button variant="secondary" icon="print" onClick={() => window.print()}>
              {w("Cetak", "Print")}
            </Button>
          </div>
          <CheckboxRow checked={saved} onChange={setSaved}>
            {w(
              "Aku sudah menyimpan kode ini di tempat aman",
              "I have saved these codes somewhere safe",
            )}
          </CheckboxRow>
          <div className="s-onboard-actions">
            <Button
              variant="primary"
              disabled={!saved}
              busy={busy}
              onClick={() => void continueNext()}
            >
              {w("Lanjut", "Continue")}
            </Button>
          </div>
        </>
      ) : (
        <>
          <StatusLine tone="warning">
            {w(
              "Kode pemulihan tidak bisa ditampilkan lagi. Buat kode baru dari halaman Akun setelah masuk.",
              "Recovery codes can't be shown again. Make new codes from the Account page after signing in.",
            )}
          </StatusLine>
          <div className="s-onboard-actions">
            <Button variant="primary" onClick={onDone}>
              {w("Lanjut", "Continue")}
            </Button>
          </div>
        </>
      )}
    </div>
  );
}

export function UpgradePage() {
  const w = useWords();
  const { navigate } = useRouter();
  const { setUser, refresh } = useSession();
  const { announce } = useAnnounce();
  const { lang } = useI18n();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [remember, setRemember] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [codes, setCodes] = useState<string[] | null>(null);
  const [done, setDone] = useState(false);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const r = await api<AuthResult>("/auth/guest/upgrade", {
        username,
        password,
        rememberDevice: remember,
      });
      if (r.user) setUser(r.user);
      if (r.recoveryCodes) setCodes(r.recoveryCodes);
      window.sessionStorage.setItem("sela.trusted", String(Boolean(r.session?.trustedDevice)));
      announce("status", w("Data tamu disimpan ke akun.", "Guest data saved to an account."));
      await refresh();
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  };

  if (codes) {
    return (
      <CodesStep
        codes={codes}
        onDone={() => {
          setDone(true);
          navigate("/settings/account");
        }}
        onAcknowledge={() => api("/me/recovery-codes/acknowledge", {})}
      />
    );
  }
  if (done) return null;

  return (
    <div className="s-auth-card">
      <PageHeader
        back={{ to: "/settings/account", label: w("Akun", "Account") }}
        title={w("Simpan ke akun", "Save to an account")}
        lead={w(
          "Semua yang sudah kamu simpan tetap ada. Setelah ini, kamu masuk dengan nama pengguna dan kata sandi, dan datamu tidak lagi dihapus otomatis.",
          "Everything you've saved stays. After this you sign in with a username and password, and your data is no longer deleted automatically.",
        )}
      />
      <ErrorNotice error={error} />
      <form onSubmit={(e) => void submit(e)}>
        <TextField
          label={w("Nama pengguna", "Username")}
          hint={w(
            "3–32 karakter: huruf a–z, angka, titik, garis bawah, atau tanda hubung. Jangan pakai nama asli, email, atau nomor telepon.",
            "3–32 characters: letters a–z, digits, dot, underscore or hyphen. Don't use your real name, email or phone number.",
          )}
          value={username}
          minLength={3}
          maxLength={32}
          autoComplete="username"
          required
          onChange={(e) => setUsername(e.target.value)}
        />
        <PasswordField
          label={w("Kata sandi", "Password")}
          hint={w(
            "Minimal 15 karakter. Coba gabungan beberapa kata yang mudah kamu ingat.",
            "At least 15 characters. Try a few words you can remember.",
          )}
          value={password}
          minLength={15}
          autoComplete="new-password"
          required
          onChange={(e) => setPassword(e.target.value)}
        />
        <CheckboxRow checked={remember} onChange={setRemember}>
          {w("Ingat perangkat ini", "Remember this device")}
        </CheckboxRow>
        <p className="s-hint">
          {w(
            "Tanpa akun, siapa pun yang memakai browser ini bisa membuka datamu. Dengan akun, kamu bisa keluar dari akun.",
            "Without an account, anyone using this browser can open your data. With one, you can sign out.",
          )}
        </p>
        <div className="s-actions">
          <Button type="submit" variant="primary" busy={busy}>
            {w("Simpan ke akun", "Save to an account")}
          </Button>
          <LinkButton to="/settings/account" variant="quiet">
            {w("Nanti saja", "Not now")}
          </LinkButton>
        </div>
      </form>
      <p className="s-hint">
        {w(
          `Bahasa akunmu: ${lang === "id" ? "Bahasa Indonesia" : "English"}.`,
          `Your account language: ${lang === "id" ? "Bahasa Indonesia" : "English"}.`,
        )}
      </p>
    </div>
  );
}
