import { useEffect, useState, type FormEvent } from "react";
import { useRouter } from "../../lib/router";
import { useI18n } from "../../i18n";
import {
  api,
  type AuthResult,
  type Consent,
  type Options,
  type Policy,
  type Profile,
} from "../api";
import { usePrefs } from "../prefs";
import { useSession } from "../session";
import { policyVersionLabel, useAnnounce, useWords } from "../shared";
import { Icon } from "../icons";
import { CodesStep } from "./Auth";
import {
  Button,
  CheckboxRow,
  ChoiceGroup,
  ErrorNotice,
  LinkButton,
  Loading,
  Notice,
  PageHeader,
  PasswordField,
  StatusLine,
  TextField,
} from "../ui";

export type OnboardingDraft = {
  ageBand: string;
  conversationDefault: "save" | "dont_save" | null;
  codes: string[] | null;
};

export const emptyDraft: OnboardingDraft = {
  ageBand: "",
  conversationDefault: null,
  codes: null,
};

// under_16 is sent only for a self-declared 15-year-old. It is input-only:
// the server refuses it with age_not_supported and stores nothing, while
// 15_17 remains the stored band for existing 16-17 records.
function apiBand(value: string): string {
  if (value === "18_plus") return "18_plus";
  if (value === "16_17") return "15_17";
  if (value === "15") return "under_16";
  if (value === "unknown") return "unknown";
  return value;
}

export function OnboardingPage({
  step,
  draft,
  setDraft,
  options,
  policies,
  onCreated,
}: {
  step: string;
  draft: OnboardingDraft;
  setDraft: (d: OnboardingDraft) => void;
  options: Options | null;
  policies: Record<string, Policy>;
  onCreated: () => Promise<unknown>;
}) {
  const { navigate } = useRouter();
  if (step === "age") return <AgeStep draft={draft} setDraft={setDraft} options={options} />;
  if (step === "teen") return <TeenStep draft={draft} />;
  if (step === "storage")
    return (
      <StorageStep
        draft={draft}
        setDraft={setDraft}
        options={options}
        policies={policies}
        onCreated={onCreated}
      />
    );
  if (step === "account")
    return <AccountStep draft={draft} setDraft={setDraft} policies={policies} onCreated={onCreated} />;
  if (step === "codes")
    return (
      <CodesStep
        codes={draft.codes}
        onAcknowledge={() => api("/me/recovery-codes/acknowledge", {})}
        onDone={() => {
          navigate(draft.ageBand === "16_17" ? "/start/guardian" : "/start/ai");
        }}
      />
    );
  if (step === "guardian")
    return <GuardianStep policies={policies} onCreated={onCreated} />;
  if (step === "ai") return <AIStep policies={policies} />;
  return <AgeStep draft={draft} setDraft={setDraft} options={options} />;
}

function AgeStep({
  draft,
  setDraft,
  options,
}: {
  draft: OnboardingDraft;
  setDraft: (d: OnboardingDraft) => void;
  options: Options | null;
}) {
  const w = useWords();
  const { navigate } = useRouter();
  const [age, setAge] = useState<string | null>(draft.ageBand || null);
  const [error, setError] = useState("");
  const allowed = options?.ageBands.allowed ?? ["18_plus"];

  const next = () => {
    if (!age) {
      setError(w("Pilih salah satu.", "Choose one."));
      return;
    }
    setError("");
    setDraft({ ...draft, ageBand: age });
    const band = apiBand(age);
    if (age === "under_15") {
      navigate("/start/blocked?reason=under15");
      return;
    }
    if (band === "under_16") {
      navigate("/start/blocked?reason=age15");
      return;
    }
    if (!allowed.includes(band)) {
      navigate("/start/blocked?reason=band");
      return;
    }
    if (age === "16_17" || age === "unknown") {
      navigate("/start/teen");
      return;
    }
    navigate("/start/storage");
  };

  return (
    <div className="s-onboard">
      <p className="s-progress">{w("Langkah 1 dari 3", "Step 1 of 3")}</p>
      <PageHeader
        back={{ to: "/", label: w("Pengenalan", "Introduction") }}
        title={w("Sebelum mulai", "Before you start")}
      />
      <p className="s-onboard-note">
        {w(
          "Sela mendengarkan dan menyimpan hanya yang kamu pilih. Sela bukan psikolog atau dokter, tidak menghubungi siapa pun, dan memeriksa tulisanmu untuk menampilkan bantuan jika ada tanda bahaya.",
          "Sela listens and keeps only what you choose. Sela is not a psychologist or doctor, contacts no one, and checks what you write so it can show help if there are signs of danger.",
        )}
      </p>
      <div style={{ marginTop: "var(--space-5)" }}>
        <ChoiceGroup
          legend={w("Berapa usiamu?", "How old are you?")}
          name="age-band"
          value={age}
          onChange={setAge}
          error={error}
          hint={w(
            "Supaya Sela menunjukkan informasi dan perlindungan yang sesuai. Kami tidak menanyakan tanggal lahirmu.",
            "So Sela can show the right information and protections. We don't ask for your date of birth.",
          )}
          options={[
            { value: "18_plus", label: w("18 tahun ke atas", "18 or older") },
            { value: "16_17", label: "16–17" },
            { value: "15", label: w("15 tahun", "15") },
            { value: "under_15", label: w("Di bawah 15 tahun", "Under 15") },
            ...(allowed.includes("unknown")
              ? [{ value: "unknown", label: w("Aku tidak ingin menyebutkan", "I'd rather not say") }]
              : []),
          ]}
        />
      </div>
      <div className="s-onboard-actions">
        <Button variant="primary" icon="arrow-right" onClick={next}>
          {w("Lanjut", "Continue")}
        </Button>
      </div>
    </div>
  );
}

function TeenStep({ draft }: { draft: OnboardingDraft }) {
  const w = useWords();
  const { navigate } = useRouter();
  return (
    <div className="s-onboard">
      <p className="s-progress">{w("Langkah 2 dari 5", "Step 2 of 5")}</p>
      <PageHeader
        back={{ to: "/start/age", label: w("Usia", "Age") }}
        title={w("Batas kerahasiaan", "Limits of confidentiality")}
      />
      <ul className="s-about-list">
        <li>
          {w(
            "Apa yang kamu tulis di Sela tidak dikirim ke orang tuamu, walimu, atau sekolah.",
            "What you write in Sela isn't sent to your parents, guardian or school.",
          )}
        </li>
        {draft.ageBand === "16_17" && (
          <li>
            {w(
              "Orang tua atau walimu perlu memberi persetujuan supaya kamu bisa memakai Sela. Persetujuan itu tidak membuat mereka bisa membaca ceritamu.",
              "Your parent or guardian needs to agree to you using Sela. That doesn't let them read what you write.",
            )}
          </li>
        )}
        <li>
          {w(
            "Sela tidak akan menghubungi siapa pun, termasuk saat kamu dalam bahaya. Karena itu Sela menunjukkan layanan bantuan yang bisa kamu hubungi sendiri.",
            "Sela won't contact anyone, even if you're in danger. That's why it shows you services you can contact yourself.",
          )}
        </li>
        <li>
          {w(
            "Batasnya: orang yang memegang HP-mu bisa membuka Sela jika kamu tetap masuk. Orang yang menjalankan server Sela secara teknis bisa mengakses data, dan hukum bisa mewajibkan data diberikan dalam keadaan tertentu.",
            "The limits: someone holding your phone can open Sela if you stay signed in. The people who run Sela's servers can technically access data, and the law can require data to be handed over in some situations.",
          )}
        </li>
        <li>
          {w(
            "Kalau kamu membagikan ringkasan, penerimanya bisa membaca apa yang kamu pilih.",
            "If you share a summary, the recipient can read what you chose.",
          )}
        </li>
      </ul>
      <div className="s-onboard-actions">
        <Button
          variant="primary"
          icon="arrow-right"
          onClick={() => navigate("/start/storage")}
        >
          {w("Aku mengerti, lanjut", "I understand, continue")}
        </Button>
      </div>
    </div>
  );
}

function StorageStep({
  draft,
  setDraft,
  options,
  policies,
  onCreated,
}: {
  draft: OnboardingDraft;
  setDraft: (d: OnboardingDraft) => void;
  options: Options | null;
  policies: Record<string, Policy>;
  onCreated: () => Promise<unknown>;
}) {
  const w = useWords();
  const { lang } = useI18n();
  const { setUser, reload } = useSession();
  const { navigate } = useRouter();
  const { set } = usePrefs();
  const { announce } = useAnnounce();
  const [mode, setMode] = useState<"guest" | "account" | null>(null);
  const [save, setSave] = useState<"save" | "dont_save" | null>(draft.conversationDefault);
  const [terms, setTerms] = useState(false);
  const [guestNotice, setGuestNotice] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [groupError, setGroupError] = useState("");
  const minor = draft.ageBand === "16_17";
  const termsVersion = policies.terms_privacy?.version ?? "";
  const guestVersion = policies.guest_storage?.version ?? "";

  const startGuest = async () => {
    if (!mode || !save || !terms || (mode === "guest" && !guestNotice)) {
      setGroupError(
        w(
          "Lengkapi pilihan dan kedua konfirmasi di atas.",
          "Complete the choices and both confirmations above.",
        ),
      );
      return;
    }
    setGroupError("");
    setBusy(true);
    setError(null);
    try {
      const r = await api<{ user: Profile }>("/auth/guest", {
        locale: lang,
        ageBand: apiBand(draft.ageBand),
        termsPolicyVersion: termsVersion,
        guestNoticeVersion: guestVersion,
      });
      if (r.user) setUser(r.user);
      setDraft({ ...draft, conversationDefault: save });
      set("defaultStorage", save);
      announce("status", w("Ruang tamu dibuat.", "Guest space created."));
      await onCreated();
      navigate(draft.ageBand === "16_17" ? "/start/guardian" : "/start/ai");
    } catch (e) {
      setError(e);
      if ((e as { code?: string }).code === "policy_outdated") await reload();
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="s-onboard">
      <p className="s-progress">
        {minor ? w("Langkah 3 dari 5", "Step 3 of 5") : w("Langkah 2 dari 3", "Step 2 of 3")}
      </p>
      <PageHeader
        back={{
          to: draft.ageBand === "16_17" || draft.ageBand === "unknown" ? "/start/teen" : "/start/age",
          label: w("Kembali", "Back"),
        }}
        title={w("Di mana datamu disimpan?", "Where is your data kept?")}
      />
      <ErrorNotice error={error} />
      {groupError && <Notice tone="danger" title={w("Perlu diperbaiki", "Needs fixing")}>{groupError}</Notice>}
      <fieldset className="s-fieldset" role="radiogroup" aria-invalid={groupError ? true : undefined}>
        <legend className="s-legend">{w("Pilih cara memulai", "Choose how to start")}</legend>
        <div className="s-choice-items">
          <label className={`s-option-panel${mode === "guest" ? " is-selected" : ""}`}>
            <span className="s-choice" style={{ border: 0, padding: 0, background: "none" }}>
              <input
                type="radio"
                name="storage-mode"
                checked={mode === "guest"}
                onChange={() => setMode("guest")}
              />
              <span className="s-choice-text">
                <strong>{w("Mulai sebagai tamu", "Start as a guest")}</strong>
                <span className="s-hint">{w("Tanpa nama pengguna.", "No username needed.")}</span>
              </span>
            </span>
            {mode === "guest" && (
              <span className="s-small">
                {w(
                  `Data disimpan di server Sela, terenkripsi. Dihapus setelah ${Math.floor((options?.guest.inactiveTtlHours ?? 168) / 24)} hari tidak aktif, paling lambat ${options?.guest.maxAgeDays ?? 30} hari. Cookie browser ini satu-satunya kuncimu.`,
                  `Stored on Sela's server, encrypted. Deleted after ${Math.floor((options?.guest.inactiveTtlHours ?? 168) / 24)} inactive days, at most ${options?.guest.maxAgeDays ?? 30} days. This browser cookie is your only key.`,
                )}
                <br />
                {w(
                  "Siapa pun yang memakai browser ini bisa membuka data tamumu. Di perangkat bersama, pakai akun.",
                  "Anyone who uses this browser can open your guest data. On a shared device, use an account.",
                )}
                <br />
                {w(
                  "Ini bukan anonimitas mutlak: server tetap melihat alamat koneksimu untuk keamanan.",
                  "This isn't complete anonymity: the server still sees your connection address for security.",
                )}
              </span>
            )}
          </label>
          <label
            className={`s-option-panel${mode === "account" ? " is-selected" : ""}`}
          >
            <span className="s-choice" style={{ border: 0, padding: 0, background: "none" }}>
              <input
                type="radio"
                name="storage-mode"
                checked={mode === "account"}
                disabled={minor}
                onChange={() => setMode("account")}
              />
              <span className="s-choice-text">
                <strong>{w("Buat akun", "Create an account")}</strong>
                <span className="s-hint">
                  {minor
                    ? w(
                        "Untuk usia 16–17 tahun, Sela sementara hanya bisa dipakai sebagai tamu. Setelah persetujuan wali tercatat, kamu bisa menyimpannya ke akun.",
                        "For ages 16–17, Sela can only be used as a guest for now. After guardian consent is recorded, you can save it to an account.",
                      )
                    : w(
                        "Nama pengguna dan kata sandi panjang, tanpa email. Bisa dibuka dari perangkat lain.",
                        "A username and a long password, without email. Open it from another device.",
                      )}
                </span>
              </span>
            </span>
            {mode === "account" && !minor && (
              <span className="s-small">
                {w(
                  "10 kode pemulihan menjadi satu-satunya cara kembali jika kamu lupa kata sandi. Data tetap sampai kamu menghapusnya.",
                  "10 recovery codes are the only way back in if you forget your password. Data stays until you delete it.",
                )}
              </span>
            )}
          </label>
        </div>
      </fieldset>

      <ChoiceGroup
        legend={w("Percakapan baru: simpan di Catatanku, atau jangan simpan?", "New conversations: save to My notes, or don't save?")}
        name="conversation-default"
        value={save}
        onChange={setSave}
        hint={w(
          "Kamu bisa mengubahnya untuk setiap percakapan.",
          "You can change it for each conversation.",
        )}
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
              "Sela membalas tanpa menyimpan; percakapan dibuang saat tab ditutup atau Keluar cepat dipakai.",
              "Sela replies without keeping it; the conversation is discarded when the tab closes or Quick exit is used.",
            ),
          },
        ]}
      />

      <CheckboxRow checked={terms} onChange={setTerms}>
        {w("Aku sudah membaca dan menyetujui Ketentuan & Privasi", "I have read and agree to the Terms & Privacy")}{" "}
        <span className="s-muted">
          ({w("versi ", "version ")}
          {policyVersionLabel(termsVersion, lang)})
        </span>{" "}
        <a href="/policies/terms_privacy" target="_blank" rel="noreferrer">
          {w("Baca", "Read")}
        </a>
      </CheckboxRow>
      {mode === "guest" && (
        <CheckboxRow checked={guestNotice} onChange={setGuestNotice}>
          {w(
            "Aku mengerti cara penyimpanan mode tamu di atas",
            "I understand how guest storage works above",
          )}
        </CheckboxRow>
      )}
      <div className="s-onboard-actions">
        {mode === "account" ? (
          <Button
            variant="primary"
            icon="arrow-right"
            disabled={!terms || !save}
            onClick={() => {
              if (save) {
                setDraft({ ...draft, conversationDefault: save });
                set("defaultStorage", save);
              }
              navigate("/start/account");
            }}
          >
            {w("Lanjut membuat akun", "Continue to create an account")}
          </Button>
        ) : (
          <Button
            variant="primary"
            icon="user"
            busy={busy}
            disabled={!options?.guest.available}
            onClick={() => void startGuest()}
          >
            {busy
              ? w("Menyiapkan…", "Preparing…")
              : w("Mulai sebagai tamu", "Start as a guest")}
          </Button>
        )}
        <LinkButton to="/signin" variant="quiet">
          {w("Sudah punya akun? Masuk", "Already have an account? Sign in")}
        </LinkButton>
      </div>
      {!options?.guest.available && (
        <StatusLine tone="warning">
          {w("Mode tamu sedang penuh. Coba lagi nanti atau buat akun.", "Guest mode is full. Try again later or create an account.")}
        </StatusLine>
      )}
    </div>
  );
}

function AccountStep({
  draft,
  setDraft,
  policies,
  onCreated,
}: {
  draft: OnboardingDraft;
  setDraft: (d: OnboardingDraft) => void;
  policies: Record<string, Policy>;
  onCreated: () => Promise<unknown>;
}) {
  const w = useWords();
  const { lang } = useI18n();
  const { navigate } = useRouter();
  const { setUser } = useSession();
  const { announce } = useAnnounce();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [remember, setRemember] = useState(false);
  const [terms, setTerms] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [progress, setProgress] = useState("");
  const checkLength = () => {
    if (!password) return;
    setProgress(
      password.length >= 15
        ? w("15 karakter atau lebih. Baik.", "15 characters or more. Good.")
        : w(`${password.length} dari 15 karakter.`, `${password.length} of 15 characters.`),
    );
  };

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const r = await api<AuthResult>("/auth/register", {
        username,
        password,
        locale: lang,
        ageBand: apiBand(draft.ageBand),
        termsPolicyVersion: policies.terms_privacy?.version ?? "",
        rememberDevice: remember,
      });
      if (r.user) setUser(r.user);
      window.sessionStorage.setItem("sela.trusted", String(Boolean(r.session?.trustedDevice)));
      setDraft({ ...draft, codes: r.recoveryCodes ?? null });
      announce("status", w("Akun dibuat.", "Account created."));
      await onCreated();
      navigate("/start/codes");
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="s-auth-card">
      <p className="s-progress">{w("Langkah 3 dari 5", "Step 3 of 5")}</p>
      <PageHeader
        back={{ to: "/start/storage", label: w("Penyimpanan", "Storage") }}
        title={w("Buat akun", "Create an account")}
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
            "Minimal 15 karakter. Coba gabungan beberapa kata yang mudah kamu ingat, misalnya 'pohon mangga di depan rumah'.",
            "At least 15 characters. Try a few words you can remember, like 'mango tree by the front door'.",
          )}
          value={password}
          minLength={15}
          maxLength={256}
          autoComplete="new-password"
          required
          onChange={(e) => setPassword(e.target.value)}
          onBlur={checkLength}
        />
        {progress && <p className="s-hint">{progress}</p>}
        <CheckboxRow checked={remember} onChange={setRemember}>
          {w("Ingat perangkat ini", "Remember this device")}
        </CheckboxRow>
        <p className="s-hint">
          {w("Usia: ", "Age: ")}
          {draft.ageBand === "18_plus" ? w("18 tahun ke atas", "18 or older") : "16–17"}
          {" · "}
          {w("Bahasa: ", "Language: ")}
          {lang === "id" ? "Bahasa Indonesia" : "English"}
        </p>
        <CheckboxRow checked={terms} onChange={setTerms}>
          {w("Aku sudah membaca dan menyetujui Ketentuan & Privasi", "I have read and agree to the Terms & Privacy")}{" "}
          <span className="s-muted">
            ({w("versi ", "version ")}
            {policyVersionLabel(policies.terms_privacy?.version ?? "", lang)})
          </span>{" "}
          <a href="/policies/terms_privacy" target="_blank" rel="noreferrer">
            {w("Baca", "Read")}
          </a>
        </CheckboxRow>
        <Button type="submit" variant="primary" busy={busy} disabled={!terms}>
          {w("Buat akun", "Create account")}
        </Button>
      </form>
    </div>
  );
}

function GuardianStep({
  policies,
  onCreated,
}: {
  policies: Record<string, Policy>;
  onCreated: () => Promise<unknown>;
}) {
  const w = useWords();
  const { lang } = useI18n();
  const { navigate } = useRouter();
  const { setUser, refresh } = useSession();
  const { announce } = useAnnounce();
  const [relationship, setRelationship] = useState<"parent" | "legal_guardian" | null>(null);
  const [name, setName] = useState("");
  const [agree, setAgree] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [declined, setDeclined] = useState(false);
  const policy = policies.guardian_information;

  const record = async () => {
    if (!relationship || !agree) {
      setError(w("Pilih hubungan dan centang pernyataan.", "Choose the relationship and tick the statement."));
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await api("/me/guardian-consent", {
        relationship,
        method: "guardian_present_attestation",
        guardianName: name.trim() || null,
        policyVersion: policy?.version ?? "",
      });
      announce("status", w("Persetujuan wali dicatat.", "Guardian consent recorded."));
      await onCreated();
      navigate("/start/ai");
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  };

  const decline = async () => {
    setBusy(true);
    setError(null);
    try {
      await api("/me", { confirm: true }, "DELETE");
      setUser(null);
      setDeclined(true);
      await refresh();
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  };

  if (declined) {
    return (
      <div className="s-narrow">
        <PageHeader title={w("Sela belum bisa dipakai tanpa persetujuan.", "Sela can't be used without consent.")} />
        <p>
          {w(
            "Tanpa persetujuan orang tua atau wali, Sela belum bisa dipakai. Data yang baru dibuat sudah dihapus. Bantuan sekarang tetap bisa kamu buka kapan saja.",
            "Without a parent's or guardian's agreement, Sela can't be used yet. The data that was just created has been deleted. You can still open Help now at any time.",
          )}
        </p>
        <div className="s-actions">
          <LinkButton to="/help" variant="help" icon="help">
            {w("Bantuan sekarang", "Help now")}
          </LinkButton>
          <LinkButton to="/" variant="quiet">
            {w("Ke pengenalan", "Go to Introduction")}
          </LinkButton>
        </div>
      </div>
    );
  }

  return (
    <div className="s-narrow">
      <p className="s-progress">{w("Langkah 4 dari 5", "Step 4 of 5")}</p>
      <PageHeader
        back={{ to: "/start/storage", label: w("Penyimpanan", "Storage") }}
        title={w("Persetujuan orang tua atau wali", "Parent or guardian consent")}
      />
      <p>
        {w(
          "Minta orang tua atau walimu membaca bagian ini bersamamu.",
          "Ask your parent or guardian to read this part with you.",
        )}
      </p>
      <Notice
        tone="info"
        title={
          policy?.title[lang] ??
          w("Informasi untuk orang tua atau wali", "Information for a parent or guardian")
        }
      >
        {policy?.body.map((p, i) => (
          <p key={i}>{p[lang]}</p>
        ))}
        <p>
          {w(
            "Persetujuan ini tidak memberi wali akses ke percakapan, catatan, atau ringkasan.",
            "This consent does not give the guardian access to conversations, notes or summaries.",
          )}
        </p>
      </Notice>
      <ErrorNotice error={error} />
      <ChoiceGroup
        legend={w("Hubungan Anda dengan anak ini", "Your relationship to this child")}
        name="guardian-relationship"
        value={relationship}
        onChange={setRelationship}
        options={[
          { value: "parent", label: w("Orang tua", "Parent") },
          { value: "legal_guardian", label: w("Wali yang sah", "Legal guardian") },
        ]}
      />
      <TextField
        label={w("Nama Anda", "Your name")}
        optional
        maxLength={80}
        value={name}
        onChange={(e) => setName(e.target.value)}
      />
      <CheckboxRow checked={agree} onChange={setAgree}>
        {w(
          "Saya orang tua/wali anak ini. Saya setuju dia memakai Sela, dan saya mengerti saya tidak mendapat akses ke isi ceritanya.",
          "I am this child's parent/guardian. I agree that they may use Sela, and I understand that I get no access to what they write.",
        )}
      </CheckboxRow>
      <p className="s-hint">
        {w(
          "Sela tidak dapat memastikan siapa yang menekan tombol ini.",
          "Sela can't confirm who presses this button.",
        )}
      </p>
      <div className="s-onboard-actions">
        <Button variant="primary" busy={busy} onClick={() => void record()}>
          {w("Saya setuju", "I agree")}
        </Button>
        <Button variant="secondary" busy={busy} onClick={() => void decline()}>
          {w("Saya tidak setuju", "I do not agree")}
        </Button>
      </div>
      <p className="s-hint">
        {w(
          "Terima kasih. Setelah ini, serahkan kembali kepada anak Anda.",
          "Thank you. After this, hand the screen back to the child.",
        )}
      </p>
    </div>
  );
}

function AIStep({
  policies,
}: {
  policies: Record<string, Policy>;
}) {
  const w = useWords();
  const { lang } = useI18n();
  const { navigate } = useRouter();
  const { user } = useSession();
  const [consents, setConsents] = useState<Consent[] | null>(null);
  const [answered, setAnswered] = useState<Record<string, boolean>>({});
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [notice, setNotice] = useState("");
  const [stepError, setStepError] = useState("");

  const load = () => {
    api<{ consents: Consent[] }>("/me/consents")
      .then((d) => {
        setConsents(d.consents);
        const a: Record<string, boolean> = {};
        for (const c of d.consents) if (c.decidedAt) a[c.purpose] = true;
        setAnswered(a);
      })
      .catch(setError);
  };
  useEffect(load, []);

  const meta = consents?.find((c) => c.purpose === "ai_processing");
  const canContinue = meta && answered[meta.purpose];
  const aiOff = consents !== null && !meta;

  const decide = async (c: Consent, granted: boolean) => {
    setBusy(true);
    setError(null);
    try {
      const r = await api<{ consent: Consent }>(
        "/me/consents/" + c.purpose,
        { granted, policyVersion: c.currentPolicyVersion },
        "PUT",
      );
      setConsents((list) =>
        (list ?? []).map((x) => (x.purpose === c.purpose ? r.consent : x)),
      );
      setAnswered((a) => ({ ...a, [c.purpose]: true }));
      setNotice(
        granted
          ? w("Diizinkan.", "Allowed.")
          : w("Tidak diizinkan untuk sekarang.", "Not allowed for now."),
      );
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  };

  const card = (c: Consent, label: { id: string; en: string }) => {
    const policy = policies[c.purpose];
    const granted = c.granted && !c.outdated;
    const decided = answered[c.purpose];
    return (
      <section className="s-consent" key={c.purpose}>
        <h2>{label[lang]}</h2>
        {policy?.body.slice(0, 3).map((p, i) => (
          <p key={i}>{p[lang]}</p>
        ))}
        {policy && policy.body.length > 3 && (
          <details>
            <summary>{w("Baca kebijakan lengkap", "Read the full policy")}</summary>
            {policy.body.slice(3).map((p, i) => (
              <p key={i}>{p[lang]}</p>
            ))}
          </details>
        )}
        <div className="s-consent-status">
          <Icon name={granted ? "check" : "info"} size={18} />
          {decided || granted
            ? granted
              ? w("Diizinkan", "Allowed")
              : w("Tidak diizinkan untuk sekarang", "Not allowed for now")
            : w("Belum dipilih", "Not chosen yet")}
        </div>
        <div className="s-actions">
          <Button
            variant="secondary"
            disabled={busy}
            onClick={() => void decide(c, true)}
            aria-label={w("Izinkan ", "Allow ") + label[lang]}
          >
            {w("Izinkan", "Allow")}
          </Button>
          <Button
            variant="secondary"
            disabled={busy}
            onClick={() => void decide(c, false)}
            aria-label={w("Tidak sekarang untuk ", "Not now for ") + label[lang]}
          >
            {w("Tidak sekarang", "Not now")}
          </Button>
        </div>
      </section>
    );
  };

  return (
    <div className="s-narrow">
      <p className="s-progress">{w("Langkah terakhir", "Last step")}</p>
      <PageHeader
        back={{ to: "/start/storage", label: w("Penyimpanan", "Storage") }}
        title={w("Balasan AI dan pemeriksa", "AI replies and the checker")}
        lead={w(
          "Pilih dengan sadar. Tidak ada yang dikirim ke penyedia AI sebelum kamu mengizinkannya, dan menolak tetap menyisakan ruang untuk menulis.",
          "Choose deliberately. Nothing is sent to an AI provider before you allow it, and declining still leaves you space to write.",
        )}
      />
      <ErrorNotice error={error} />
      {notice && <StatusLine tone="success">{notice}</StatusLine>}
      {consents === null ? (
        <Loading label={w("Memuat pilihan…", "Loading choices…")} />
      ) : aiOff ? (
        <Notice tone="info" title={w("Balasan AI sedang mati", "AI replies are off")}>
          <p>
            {w(
              "Kamu tetap bisa menulis dan menyimpan. Sela tidak akan membalas dengan AI.",
              "You can still write and save. Sela won't reply with AI.",
            )}
          </p>
        </Notice>
      ) : (
        <>
          {meta &&
            card(meta, {
              id: "Balasan AI dan pemeriksa · Meta (Amerika Serikat)",
              en: "AI replies and checker · Meta (United States)",
            })}
        </>
      )}
      <div className="s-onboard-actions">
        <Button
          variant="primary"
          icon="send"
          busy={busy}
          disabled={consents === null}
          onClick={() => {
            if (!aiOff && !canContinue) {
              setStepError(
                w(
                  "Pilih Izinkan atau Tidak sekarang untuk setiap kartu yang berlaku.",
                  "Choose Allow or Not now for each card that applies.",
                ),
              );
              return;
            }
            navigate("/talk/new");
          }}
        >
          {w("Mulai bercerita", "Start talking")}
        </Button>
        <LinkButton to="/home" variant="quiet">
          {w("Lihat beranda dulu", "See Home first")}
        </LinkButton>
      </div>
      {stepError && <StatusLine tone="warning">{stepError}</StatusLine>}
      {user && (
        <p className="s-small s-muted">
          {w(
            "Kamu bisa mengubah pilihan ini kapan saja di Privasi & data.",
            "You can change these choices any time in Privacy & data.",
          )}
        </p>
      )}
    </div>
  );
}
