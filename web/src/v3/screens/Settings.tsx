import { useEffect, useState, type FormEvent } from "react";
import { Link } from "../../lib/router";
import { useI18n } from "../../i18n";
import {
  api,
  download,
  V3Error,
  type AuditEvent,
  type Consent,
  type ConsentEvent,
  type Options,
  type Policy,
  type Profile,
  type SessionView,
} from "../api";
import { usePrefs } from "../prefs";
import { useAnnounce, useLocale, useWords } from "../shared";
import { useSession } from "../session";
import { Icon } from "../icons";
import { LanguageSwitch } from "../shell";
import {
  Button,
  CheckboxRow,
  ChoiceGroup,
  DefinitionList,
  ErrorNotice,
  InlineConfirm,
  LinkButton,
  Loading,
  Notice,
  PageHeader,
  PasswordField,
  StatusLine,
  Switch,
  TextField,
  UnavailableFeature,
  buttonClass,
} from "../ui";

const purposePlace: Record<string, { id: string; en: string }> = {
  ai_processing: { id: "Meta: balasan dan pemeriksa (Amerika Serikat)", en: "Meta: replies and checker (United States)" },
  personal_memory: { id: "Disimpan di server Sela", en: "Stored on Sela's server" },
};

export function PrivacyPage({
  user,
  policies,
  onDeleted,
}: {
  user: Profile;
  policies: Record<string, Policy>;
  onDeleted: () => void;
}) {
  const w = useWords();
  const loc = useLocale();
  const { lang } = useI18n();
  const { prefs, set } = usePrefs();
  const { setUser, signOut } = useSession();
  const { announce } = useAnnounce();
  const [consents, setConsents] = useState<Consent[] | null>(null);
  const [sessions, setSessions] = useState<SessionView[] | null>(null);
  const [audit, setAudit] = useState<AuditEvent[] | null>(null);
  const [auditCursor, setAuditCursor] = useState<string | null>(null);
  const [history, setHistory] = useState<ConsentEvent[] | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const [exportFormat, setExportFormat] = useState("html");
  const [exportLang, setExportLang] = useState(lang);
  const [needsProof, setNeedsProof] = useState(false);
  const [password, setPassword] = useState("");
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [understand, setUnderstand] = useState(false);
  const [deletePassword, setDeletePassword] = useState("");

  const load = async () => {
    setError(null);
    try {
      const [c, s, a, h] = await Promise.all([
        api<{ consents: Consent[] }>("/me/consents"),
        api<{ sessions: SessionView[] }>("/me/sessions"),
        api<{ events: AuditEvent[]; nextCursor: string | null }>("/me/audit?limit=20"),
        api<{ events: ConsentEvent[] }>("/me/consents/history?limit=20"),
      ]);
      setConsents(c.consents);
      setSessions(s.sessions);
      setAudit(a.events);
      setAuditCursor(a.nextCursor);
      setHistory(h.events);
    } catch (e) {
      setError(e);
    }
  };
  useEffect(() => {
    void load();
  }, []);

  const loadMoreAudit = async () => {
    if (!auditCursor) return;
    try {
      const a = await api<{ events: AuditEvent[]; nextCursor: string | null }>(
        "/me/audit?limit=20&cursor=" + encodeURIComponent(auditCursor),
      );
      setAudit((list) => [...(list ?? []), ...a.events]);
      setAuditCursor(a.nextCursor);
    } catch (e) {
      setError(e);
    }
  };

  const updateConsent = async (c: Consent, granted: boolean) => {
    setBusy(true);
    setError(null);
    try {
      const r = await api<{ consent: Consent; changed: boolean }>(
        "/me/consents/" + c.purpose,
        { granted, policyVersion: c.currentPolicyVersion },
        "PUT",
      );
      setConsents((list) =>
        (list ?? []).map((x) => (x.purpose === c.purpose ? r.consent : x)),
      );
      setNotice(
        granted
          ? w("Izin diberikan. Sela memakai penyedia ini mulai sekarang.", "Permission granted. Sela uses this provider from now on.")
          : w("Izin dicabut. Sela berhenti memakai penyedia ini mulai sekarang.", "Permission withdrawn. Sela stops using this provider from now on."),
      );
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  };

  const doExport = async () => {
    setBusy(true);
    setError(null);
    setNotice("");
    try {
      const ext = exportFormat === "md" ? "md" : exportFormat;
      await download(
        `/me/export?format=${exportFormat}&lang=${exportLang}`,
        `sela-export.${ext}`,
      );
      setNeedsProof(false);
      setPassword("");
      setNotice(
        w(
          "Ekspor sudah diunduh. Simpan di tempat yang aman.",
          "Export downloaded. Keep it somewhere safe.",
        ),
      );
    } catch (e) {
      if (e instanceof V3Error && e.code === "reauth_required") {
        setNeedsProof(true);
        setError(
          w(
            "Masukkan kata sandimu sekali lagi untuk membuat ekspor.",
            "Enter your password once more to create the export.",
          ),
        );
      } else {
        setError(e);
      }
    } finally {
      setBusy(false);
    }
  };

  const reauth = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await api("/auth/reauth", { password });
      setPassword("");
      setNotice(w("Identitas dikonfirmasi.", "Identity confirmed."));
      setNeedsProof(false);
      await doExport();
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };

  const revoke = async (s: SessionView) => {
    setBusy(true);
    try {
      const r = await api<{ revoked: boolean; current: boolean }>(
        "/me/sessions/" + s.id,
        undefined,
        "DELETE",
      );
      if (r.current) {
        setUser(null);
        return;
      }
      setSessions((list) => (list ?? []).filter((x) => x.id !== s.id));
      announce("status", w("Sesi dikeluarkan.", "Session signed out."));
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  };

  const logoutOthers = async () => {
    setBusy(true);
    try {
      const r = await api<{ revoked: number }>("/auth/logout-all", {
        keepCurrent: true,
      });
      setSessions((list) => (list ?? []).filter((x) => x.current));
      setNotice(
        w(`${r.revoked} perangkat lain dikeluarkan.`, `${r.revoked} other devices were signed out.`),
      );
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  };

  const logoutAll = async () => {
    setBusy(true);
    try {
      await api("/auth/logout-all", { keepCurrent: false });
      await signOut();
    } catch (e) {
      setError(e);
      setBusy(false);
    }
  };

  const removeAccount = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      if (user.kind === "account") {
        await api("/auth/reauth", { password: deletePassword });
      }
      await api("/me", { confirm: true }, "DELETE");
      onDeleted();
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  };

  const aiPurposes = (consents ?? []).filter(
    (c) =>
      c.purpose !== "asr_meta" &&
      c.purpose !== "ai_provider_aws_voice" &&
      (c.purpose === "ai_processing" ||
        c.purpose.startsWith("ai_provider_") ||
        c.purpose === "personal_memory"),
  );
  const region = user.guest?.storage.region;

  return (
    <>
      <PageHeader
        title={w("Privasi & data", "Privacy & data")}
        lead={w(
          "Semua pilihan tentang datamu ada di sini. Kamu bisa mengubahnya kapan saja.",
          "Every choice about your data is here. You can change any of them at any time.",
        )}
      />
      <dl className="s-privacy-glance">
        <div>
          <dt>{w("Yang disimpan", "What is kept")}</dt>
          <dd>
            {w(
              "Hanya catatan dan percakapan yang kamu pilih untuk disimpan.",
              "Only the notes and conversations you choose to save.",
            )}
          </dd>
        </div>
        <div>
          <dt>{w("Terlindungi", "Protected")}</dt>
          <dd>
            {w(
              "Terenkripsi per orang di server Sela. Sela dapat membukanya untuk menampilkan fitur yang kamu pilih.",
              "Encrypted per person on Sela's server. Sela can open it to show the features you choose.",
            )}
          </dd>
        </div>
        <div>
          <dt>{w("Kendalimu", "Your control")}</dt>
          <dd>
            {w(
              "Ubah izin, ekspor, atau hapus data kapan saja.",
              "Change permissions, export or delete your data at any time.",
            )}
          </dd>
        </div>
      </dl>
      <ErrorNotice error={error} onRetry={load} />
      {notice && <StatusLine tone="success">{notice}</StatusLine>}
      <div className="s-settings-layout">
        <nav className="s-settings-nav" aria-label={w("Bagian privasi", "Privacy sections")}>
          {([
            ["#penyimpanan", "Penyimpanan", "Storage"],
            ["#memori", "Memori", "Memory"],
            ["#nama", "Nama yang disamarkan", "Names kept private"],
            ["#ai", "Pemrosesan AI", "AI processing"],
            ["#ekspor", "Ekspor", "Export"],
            ["#sesi", "Sesi & perangkat", "Sessions & devices"],
            ["#riwayat", "Riwayat akses", "Access history"],
            ["#persetujuan", "Persetujuan & kebijakan", "Consents & policies"],
            ["#hapus", "Hapus data", "Delete data"],
          ] as const).map(([href, id, en]) => (
            <a key={href} href={href}>
              {w(id, en)}
            </a>
          ))}
        </nav>
        <div>
          <section className="s-settings-section" id="penyimpanan" aria-labelledby="storage-title">
            <h2 id="storage-title">{w("Penyimpanan", "Storage")}</h2>
            <DefinitionList
              items={[
                {
                  term: w("Tempat data", "Where data is"),
                  detail: w(
                    "Server Sela (database PostgreSQL), terenkripsi per orang.",
                    "Sela's server (PostgreSQL database), encrypted per person.",
                  ),
                },
                {
                  term: w("Wilayah", "Region"),
                  detail: region ?? w("Belum ditentukan (menunggu keputusan hosting).", "Not decided yet (pending a hosting decision)."),
                },
                {
                  term: w("Kunci", "Key"),
                  detail:
                    user.kind === "guest"
                      ? w("Cookie browser ini.", "This browser cookie.")
                      : w("Nama pengguna dan kata sandimu.", "Your username and password."),
                },
              ]}
            />
            <h3>{w("Percakapan baru", "New conversations")}</h3>
            <ChoiceGroup
              legend={w("Default untuk percakapan baru", "Default for new conversations")}
              name="default-storage"
              value={prefs.defaultStorage}
              onChange={(v) => set("defaultStorage", v as "save" | "dont_save")}
              options={[
                { value: "save", label: w("Simpan di Catatanku", "Save to My notes") },
                { value: "dont_save", label: w("Jangan simpan", "Don't save") },
              ]}
            />
            <p className="s-hint">
              {w(
                "Berlaku untuk percakapan yang kamu mulai setelah ini. Kamu tetap bisa mengubahnya di setiap percakapan.",
                "Applies to conversations you start after this. You can still change it in each conversation.",
              )}
            </p>
            <Switch
              checked={prefs.hidePreviews}
              onChange={(v) => set("hidePreviews", v)}
              label={w("Sembunyikan pratinjau isi", "Hide content previews")}
              hint={w(
                "Daftar dan kartu hanya menampilkan jenis dan waktu. Berguna di perangkat bersama.",
                "Lists and cards show only type and time. Useful on a shared device.",
              )}
            />
            {user.kind === "guest" && user.guest && (
              <Notice tone="warning" title={w("Mode tamu", "Guest mode")}>
                <p>
                  {w("Data tamu dihapus otomatis pada ", "Guest data is deleted on ")}
                  {loc.date(user.guest.maxAgePurgeAt)}
                  {w(" jika tidak aktif, paling lambat ", " after inactivity, at the latest ")}
                  {loc.date(user.guest.maxAgePurgeAt)}.
                </p>
                <p>
                  {w(
                    "Siapa pun yang memakai browser ini bisa membuka data tamumu. Di perangkat bersama, pakai akun atau sembunyikan pratinjau isi.",
                    "Anyone using this browser can open your guest data. On a shared device, use an account or hide content previews.",
                  )}
                </p>
                <div className="s-actions">
                  <LinkButton to="/settings/account/upgrade" variant="secondary">
                    {w("Simpan ke akun", "Save to an account")}
                  </LinkButton>
                </div>
              </Notice>
            )}
          </section>

          <section className="s-settings-section" id="memori" aria-labelledby="memory-title">
            <h2 id="memory-title">{w("Memori", "Memory")}</h2>
            <p>{w("Memori hanya menyimpan konteks yang kamu tulis sendiri setelah izin terpisah. Tidak ada pengambilan cerita otomatis dan memori tidak dikirim ke AI. Aktifkan, perbaiki, matikan, atau hapus dari halaman Yang Sela ingat.", "Memory only stores context you write yourself after separate permission. Stories are not collected automatically and memory is not sent to AI. Enable, edit, turn off, or delete it from What Sela remembers.")}</p>
            <div className="s-actions">
              <LinkButton to="/notes/memory" variant="secondary">
                {w("Buka Yang Sela ingat", "Open What Sela remembers")}
              </LinkButton>
            </div>
          </section>

          <section className="s-settings-section" id="nama" aria-labelledby="names-title">
            <h2 id="names-title">{w("Nama yang disamarkan", "Names kept private")}</h2>
            <UnavailableFeature
              title={w("Daftar nama belum tersedia", "The name list isn't available yet")}
              body={w(
                "Sela selalu menyamarkan kontak sebelum mengirim apa pun ke AI. Daftar nama pilihanmu menunggu layanan penyimpanan yang terpisah.",
                "Sela always masks contact details before sending anything to AI. Your own name list is waiting for its separate storage service.",
              )}
              dependency={w("API nama yang disamarkan belum ada.", "The names API doesn't exist yet.")}
            />
          </section>

          <section className="s-settings-section" id="ai" aria-labelledby="ai-title">
            <h2 id="ai-title">{w("Pemrosesan AI", "AI processing")}</h2>
            <p>
              {w(
                "Setiap penyedia memerlukan izin terpisah. Menolak satu penyedia tidak menutup Sela; tanpa balasan AI kamu tetap bisa menulis dan menyimpan.",
                "Each provider needs separate permission. Declining one provider doesn't close Sela; without AI replies you can still write and save.",
              )}
            </p>
            {consents === null ? (
              <Loading label={w("Memuat izin…", "Loading permissions…")} />
            ) : (
              aiPurposes.map((c) => {
                const policy = policies[c.purpose];
                const granted = c.granted && !c.outdated;
                return (
                  <section className="s-consent" key={c.purpose}>
                    <h3>{policy?.title[lang] ?? c.purpose}</h3>
                    <p className="s-hint">{purposePlace[c.purpose]?.[lang] ?? ""}</p>
                    {policy?.body.slice(0, 2).map((p, i) => (
                      <p key={i}>{p[lang]}</p>
                    ))}
                    {policy && policy.body.length > 2 && (
                      <details>
                        <summary>{w("Baca kebijakan lengkap", "Read the full policy")}</summary>
                        {policy.body.slice(2).map((p, i) => (
                          <p key={i}>{p[lang]}</p>
                        ))}
                      </details>
                    )}
                    <p className={"s-consent-status" + (granted ? " is-granted" : "")}>
                      <Icon name={granted ? "check" : "info"} size={18} />
                      {granted
                        ? w("Diizinkan", "Allowed") +
                          (c.decidedAt ? " · " + loc.date(c.decidedAt) : "")
                        : w("Tidak diizinkan", "Not allowed")}
                    </p>
                    <div className="s-actions">
                      <Button
                        variant="secondary"
                        disabled={busy}
                        onClick={() => void updateConsent(c, !granted)}
                      >
                        {granted ? w("Cabut izin", "Withdraw") : w("Izinkan", "Allow")}
                      </Button>
                      <Link to={"/policies/" + c.purpose} className={buttonClass("quiet")}>
                        {w("Baca kebijakan", "Read the policy")}
                      </Link>
                    </div>
                  </section>
                );
              })
            )}
            <p className="s-hint">
              {w(
                "Mencabut izin menghentikan pemakaian penyedia itu mulai sekarang. Yang sudah terkirim tidak bisa ditarik kembali; penyedia menyimpannya sesuai kebijakannya.",
                "Withdrawing stops that provider's use from now on. What was already sent can't be recalled; the provider keeps it under its own policy.",
              )}
            </p>
          </section>

          <section className="s-settings-section" id="ekspor" aria-labelledby="export-title">
            <h2 id="export-title">{w("Ekspor", "Export")}</h2>
            <ChoiceGroup
              legend={w("Format", "Format")}
              name="export-format"
              value={exportFormat}
              onChange={setExportFormat}
              variant="chips"
              options={[
                { value: "html", label: w("Mudah dibaca (HTML)", "Easy to read (HTML)") },
                { value: "md", label: w("Teks (Markdown)", "Text (Markdown)") },
                { value: "json", label: w("Untuk aplikasi lain (JSON)", "For other apps (JSON)") },
              ]}
            />
            <ChoiceGroup
              legend={w("Bahasa", "Language")}
              name="export-lang"
              value={exportLang}
              onChange={(v) => setExportLang(v as "id" | "en")}
              variant="chips"
              options={[
                { value: "id", label: "Bahasa Indonesia" },
                { value: "en", label: "English" },
              ]}
            />
            <p className="s-hint">
              {w(
                "File ini berisi semua yang kamu simpan di Sela. Simpan di tempat aman; siapa pun yang membuka file ini bisa membacanya.",
                "This file contains everything you've saved in Sela. Keep it somewhere safe; anyone who opens it can read it.",
              )}
            </p>
            <div className="s-actions">
              <Button variant="primary" icon="download" busy={busy} onClick={() => void doExport()}>
                {w("Unduh data", "Download my data")}
              </Button>
            </div>
            {needsProof && (
              <form onSubmit={(e) => void reauth(e)} className="s-narrow-sm">
                <TextField
                  label={w("Masuk sebagai", "Signed in as")}
                  value={user.username ?? ""}
                  readOnly
                  autoComplete="username"
                />
                <PasswordField
                  label={w("Kata sandi", "Password")}
                  value={password}
                  autoComplete="current-password"
                  required
                  onChange={(e) => setPassword(e.target.value)}
                />
                <Button type="submit" variant="secondary" busy={busy}>
                  {w("Konfirmasi dan unduh", "Confirm and download")}
                </Button>
              </form>
            )}
          </section>

          <section className="s-settings-section" id="sesi" aria-labelledby="sessions-title">
            <h2 id="sessions-title">{w("Sesi & perangkat", "Sessions & devices")}</h2>
            {sessions === null ? (
              <Loading label={w("Memuat sesi…", "Loading sessions…")} />
            ) : user.kind === "guest" ? (
              <>
                <p>
                  {w(
                    "Tamu tidak punya tombol keluar, karena tanpa akun, keluar berarti datamu tidak bisa dibuka lagi.",
                    "Guests have no sign-out button, because without an account, signing out would lock you out of your data.",
                  )}
                </p>
                {sessions.map((s) => (
                  <div className="s-session-row" key={s.id}>
                    <div>
                      <strong>
                        {s.device.browser} · {s.device.os} ·{" "}
                        {w("sesi ini", "this session")}
                      </strong>
                      <p>
                        {w("Dibuat ", "Created ")}
                        {loc.dateTime(s.createdAt)}
                      </p>
                    </div>
                  </div>
                ))}
                <div className="s-actions">
                  <LinkButton to="/settings/account/upgrade" variant="secondary">
                    {w("Simpan ke akun", "Save to an account")}
                  </LinkButton>
                  <Link to="#hapus" className={buttonClass("danger")}>
                    {w("Hapus data tamu sekarang", "Delete guest data now")}
                  </Link>
                </div>
              </>
            ) : (
              <>
                {sessions.map((s) => (
                  <div className="s-session-row" key={s.id}>
                    <div>
                      <strong>
                        {s.device.browser} · {s.device.os}
                        {s.current ? " · " + w("sesi ini", "this session") : ""}
                      </strong>
                      <p>
                        {w("Terakhir terlihat ", "Last seen ")}
                        {loc.dateTime(s.lastSeenAt)} · {w("berakhir ", "ends ")}
                        {loc.dateTime(s.absoluteExpiresAt)}
                      </p>
                    </div>
                    {!s.current && (
                      <Button variant="secondary" disabled={busy} onClick={() => void revoke(s)}>
                        {w("Keluarkan", "Sign out")}
                      </Button>
                    )}
                  </div>
                ))}
                <div className="s-actions">
                  <Button variant="secondary" disabled={busy} onClick={() => void logoutOthers()}>
                    {w("Keluarkan semua perangkat lain", "Sign out all other devices")}
                  </Button>
                  <InlineConfirm
                    label={w("Keluar dari semua perangkat", "Sign out everywhere")}
                    question={w(
                      "Semua perangkat, termasuk yang ini, akan keluar dari akun.",
                      "Every device, including this one, will be signed out.",
                    )}
                    confirmLabel={w("Keluar dari semua perangkat", "Sign out everywhere")}
                    onConfirm={() => void logoutAll()}
                    icon="quick-exit"
                  />
                </div>
              </>
            )}
          </section>

          <section className="s-settings-section" id="riwayat" aria-labelledby="audit-title">
            <h2 id="audit-title">{w("Riwayat akses", "Access history")}</h2>
            <p className="s-hint">
              {w(
                "Hanya detail tanpa isi: waktu, jenis tindakan, dan hasilnya. Tidak ada tulisan, token, atau alamat IP.",
                "Content-free details only: time, kind of action and outcome. No writing, tokens or IP addresses.",
              )}
            </p>
            {audit === null ? (
              <Loading label={w("Memuat riwayat…", "Loading history…")} />
            ) : audit.length === 0 ? (
              <p className="s-muted">{w("Belum ada peristiwa.", "No events yet.")}</p>
            ) : (
              <>
                <ol className="s-audit">
                  {audit.map((event) => (
                    <li key={event.id}>
                      <time dateTime={event.at}>{loc.dateTime(event.at)}</time>
                      <span>{auditLabel(event, lang)}</span>
                    </li>
                  ))}
                </ol>
                {auditCursor && (
                  <Button variant="secondary" onClick={() => void loadMoreAudit()}>
                    {w("Muat lebih banyak", "Load more")}
                  </Button>
                )}
              </>
            )}
          </section>

          <section className="s-settings-section" id="persetujuan" aria-labelledby="consents-title">
            <h2 id="consents-title">{w("Persetujuan & kebijakan", "Consents & policies")}</h2>
            {consents === null ? (
              <Loading label={w("Memuat persetujuan…", "Loading consents…")} />
            ) : (
              <ul className="s-consent-list">
                {consents.map((c) => (
                  <li key={c.purpose}>
                    <strong>{policies[c.purpose]?.title[lang] ?? c.purpose}</strong>
                    <span className="s-muted s-small">
                      {c.granted && !c.outdated
                        ? w("Diizinkan", "Allowed")
                        : w("Tidak diizinkan", "Not allowed")}
                      {c.policyVersion ? ` · ${c.policyVersion}` : ""}
                    </span>
                  </li>
                ))}
              </ul>
            )}
            {history && history.length > 0 && (
              <details>
                <summary>{w("Riwayat persetujuan", "Consent history")}</summary>
                <ol className="s-audit">
                  {history.map((e) => (
                    <li key={e.id}>
                      <time dateTime={e.decidedAt}>{loc.dateTime(e.decidedAt)}</time>
                      <span>
                        {(e.decision === "granted"
                          ? w("Diberikan: ", "Granted: ")
                          : w("Dicabut: ", "Withdrawn: ")) +
                          (policies[e.purpose]?.title[lang] ?? e.purpose)}
                      </span>
                    </li>
                  ))}
                </ol>
              </details>
            )}
            <div className="s-actions">
              <LinkButton to="/policies" variant="quiet">
                {w("Baca semua kebijakan", "Read every policy")}
              </LinkButton>
            </div>
          </section>

          <section className="s-settings-section" id="hapus" aria-labelledby="delete-title">
            <h2 id="delete-title">{w("Hapus data", "Delete data")}</h2>
            <p>
              {w(
                "Yang dihapus: semua catatan, percakapan, cek perasaan, sesi, persetujuan dan riwayat akunmu. Memori, ringkasan, persiapan dan tindak lanjut yang tersimpan ikut dihapus.",
                "What is deleted: every note, conversation, check-in, session, consent and account event. Saved memories, summaries, preparation and follow-up notes are also deleted.",
              )}
            </p>
            <p className="s-hint">
              {w(
                "Sekarang: hilang dari Sela · Dalam 7 hari: tidak ada cadangan yang bisa membukanya · Dalam 30 hari: hilang dari semua cadangan.",
                "Now: gone from Sela · Within 7 days: no backup can open it · Within 30 days: gone from every backup.",
              )}
            </p>
            <p className="s-hint">
              {w(
                "Salinan yang sudah kamu unduh tidak bisa ditarik kembali. Ekspor dulu kalau kamu ingin menyimpan salinan.",
                "Copies you've downloaded can't be recalled. Export first if you want to keep a copy.",
              )}
            </p>
            {!confirmDelete ? (
              <Button variant="danger" icon="trash" onClick={() => setConfirmDelete(true)}>
                {user.kind === "guest"
                  ? w("Hapus data tamu sekarang", "Delete guest data now")
                  : w("Hapus akun dan semua data", "Delete my account and all data")}
              </Button>
            ) : (
              <form onSubmit={(e) => void removeAccount(e)}>
                <CheckboxRow
                  checked={understand}
                  onChange={setUnderstand}
                  hint={w("Tindakan ini tidak bisa dibatalkan.", "This cannot be undone.")}
                >
                  {w("Aku mengerti ini tidak bisa dibatalkan", "I understand this cannot be undone")}
                </CheckboxRow>
                {user.kind === "account" && (
                  <>
                    <TextField
                      label={w("Masuk sebagai", "Signed in as")}
                      value={user.username ?? ""}
                      readOnly
                      autoComplete="username"
                    />
                    <PasswordField
                      label={w("Kata sandi", "Password")}
                      value={deletePassword}
                      required
                      autoComplete="current-password"
                      onChange={(e) => setDeletePassword(e.target.value)}
                    />
                  </>
                )}
                <div className="s-actions">
                  <Button
                    type="submit"
                    variant="confirm"
                    icon="trash"
                    disabled={!understand || busy}
                  >
                    {user.kind === "guest"
                      ? w("Ya, hapus data tamu", "Yes, delete guest data")
                      : w("Ya, hapus akun dan semua data", "Yes, delete my account and all data")}
                  </Button>
                  <Button type="button" variant="quiet" onClick={() => setConfirmDelete(false)}>
                    {w("Batal", "Cancel")}
                  </Button>
                </div>
              </form>
            )}
          </section>
        </div>
      </div>
    </>
  );
}

function auditLabel(event: AuditEvent, lang: string): string {
  const map: Record<string, [string, string]> = {
    "account.created": ["Akun dibuat", "Account created"],
    "account.upgraded": ["Data tamu disimpan ke akun", "Guest data saved to an account"],
    "auth.login_succeeded": ["Masuk berhasil", "Signed in"],
    "auth.login_failed": ["Percobaan masuk gagal", "Failed sign-in"],
    "auth.reauthenticated": ["Identitas dikonfirmasi", "Identity confirmed"],
    "auth.logout": ["Keluar dari akun", "Signed out"],
    "auth.logout_all": ["Keluar dari semua perangkat", "Signed out everywhere"],
    "session.revoked": ["Sesi dikeluarkan", "Session signed out"],
    "password.changed": ["Kata sandi diganti", "Password changed"],
    "password_login.disabled": ["Masuk dengan kata sandi dinonaktifkan", "Password sign-in disabled"],
    "recovery.used": ["Kode pemulihan dipakai", "Recovery code used"],
    "recovery.regenerated": ["Kode pemulihan baru dibuat", "New recovery codes made"],
    "recovery.acknowledged": ["Kode pemulihan dikonfirmasi tersimpan", "Recovery codes confirmed saved"],
    "profile.locale_changed": ["Bahasa diubah", "Language changed"],
    "profile.age_band_changed": ["Kelompok usia diubah", "Age band changed"],
    "profile.age_band_confirmed": ["Kelompok usia dikonfirmasi", "Age band confirmed"],
    "consent.granted": ["Persetujuan diberikan", "Consent granted"],
    "consent.withdrawn": ["Persetujuan dicabut", "Consent withdrawn"],
    "guardian_consent.recorded": ["Persetujuan wali dicatat", "Guardian consent recorded"],
    "guardian_consent.withdrawn": ["Persetujuan wali dicabut", "Guardian consent withdrawn"],
    "export.created": ["Ekspor dibuat", "Export created"],
  };
  const entry = map[event.type];
  let label = entry ? (lang === "en" ? entry[1] : entry[0]) : event.type;
  if (event.detail?.format) label += ` (${String(event.detail.format)})`;
  if (event.detail?.purpose) label += `: ${String(event.detail.purpose)}`;
  if (event.detail?.count) label += ` × ${String(event.detail.count)}`;
  return label;
}

/* ------------------------------------------------------------------ */
/* Display & accessibility                                             */
/* ------------------------------------------------------------------ */

export function DisplayPage() {
  const w = useWords();
  const { user } = useSession();
  const { prefs, set, reset, persisted, scheme } = usePrefs();
  const { announce } = useAnnounce();
  const [notice, setNotice] = useState("");
  const update = <K extends keyof typeof prefs>(key: K, value: (typeof prefs)[K], message: string) => {
    set(key, value);
    setNotice(message);
  };

  return (
    <>
      <PageHeader
        back={{ to: "/", label: w("Kembali", "Back") }}
        title={w("Tampilan & aksesibilitas", "Display & accessibility")}
        lead={w(
          "Atur tampilan sesuai kebutuhanmu. Perubahan berlaku langsung dan disimpan di perangkat ini.",
          "Set the display to what you need. Changes apply at once and are saved on this device.",
        )}
      />
      {notice && <StatusLine tone="success">{notice}</StatusLine>}
      {!persisted && (
        <Notice tone="warning" title={w("Penyimpanan diblokir", "Storage is blocked")}>
          <p>
            {w(
              "Pengaturan berlaku sampai tab ini ditutup.",
              "Settings apply until this tab is closed.",
            )}
          </p>
        </Notice>
      )}
      <div className="s-settings-layout s-settings-layout--display">
        <div>
          <section className="s-display-group" aria-labelledby="text-title">
            <h2 id="text-title">{w("Tampilan teks", "Text display")}</h2>
            <ChoiceGroup
              legend={w("Ukuran teks", "Text size")}
              name="text-scale"
              value={prefs.textScale}
              onChange={(v) =>
                update("textScale", v, w("Ukuran teks diubah.", "Text size changed."))
              }
              variant="chips"
              options={[
                { value: "100", label: "Aa 100%" },
                { value: "115", label: "Aa 115%" },
                { value: "130", label: "Aa 130%" },
                { value: "150", label: "Aa 150%" },
              ]}
            />
            <Switch
              checked={prefs.spacing === "relaxed"}
              onChange={(v) =>
                update(
                  "spacing",
                  v ? "relaxed" : "normal",
                  w("Jarak teks diubah.", "Text spacing changed."),
                )
              }
              label={w("Jarak teks lebih lega", "Relaxed text spacing")}
            />
          </section>
          <section className="s-display-group" aria-labelledby="colour-title">
            <h2 id="colour-title">{w("Warna & kontras", "Colour & contrast")}</h2>
            <ChoiceGroup
              legend={w("Tema", "Theme")}
              name="theme"
              value={prefs.theme}
              onChange={(v) =>
                update("theme", v as "system" | "light" | "dark", w("Tema diubah.", "Theme changed."))
              }
              variant="chips"
              options={[
                { value: "system", label: w("Ikuti sistem", "Follow system") },
                { value: "light", label: w("Terang", "Light") },
                { value: "dark", label: w("Gelap", "Dark") },
              ]}
            />
            <ChoiceGroup
              legend={w("Kontras tinggi", "High contrast")}
              name="contrast"
              value={prefs.contrast}
              onChange={(v) =>
                update(
                  "contrast",
                  v as "system" | "more" | "less",
                  w("Kontras diubah.", "Contrast changed."),
                )
              }
              variant="chips"
              options={[
                { value: "system", label: w("Ikuti sistem", "Follow system") },
                { value: "more", label: w("Ya", "Yes") },
                { value: "less", label: w("Tidak", "No") },
              ]}
            />
          </section>
          <section className="s-display-group" aria-labelledby="motion-title">
            <h2 id="motion-title">{w("Gerakan & balasan", "Motion & replies")}</h2>
            <ChoiceGroup
              legend={w("Kurangi gerakan", "Reduce motion")}
              name="motion"
              value={prefs.motion}
              onChange={(v) =>
                update(
                  "motion",
                  v as "system" | "reduced" | "full",
                  w("Gerakan diubah.", "Motion changed."),
                )
              }
              variant="chips"
              options={[
                { value: "system", label: w("Ikuti sistem", "Follow system") },
                { value: "reduced", label: w("Ya", "Yes") },
                { value: "full", label: w("Tidak", "No") },
              ]}
            />
            <Switch
              checked={prefs.showRepliesAtOnce}
              onChange={(v) => update("showRepliesAtOnce", v, w("Pilihan balasan disimpan.", "Reply setting saved."))}
              label={w("Tampilkan balasan sekaligus", "Show replies at once")}
            />
          </section>
          <section className="s-display-group" aria-labelledby="reader-title">
            <h2 id="reader-title">{w("Pembaca layar", "Screen readers")}</h2>
            <Switch
              checked={prefs.announceReplies}
              onChange={(v) => update("announceReplies", v, w("Pengumuman balasan diubah.", "Reply announcements changed."))}
              label={w("Umumkan balasan ke pembaca layar", "Announce replies to screen readers")}
              hint={w(
                "Untuk pembaca layar. Tidak ada audio yang diputar.",
                "For screen readers. No audio is played.",
              )}
            />
          </section>
          <section className="s-display-group" aria-labelledby="safety-title">
            <h2 id="safety-title">{w("Keselamatan", "Safety")}</h2>
            <Switch
              checked={prefs.quickExitShortcut}
              onChange={(v) => update("quickExitShortcut", v, w("Pintasan keluar cepat diubah.", "Quick exit shortcut changed."))}
              label={w("Pintasan keluar cepat (Esc dua kali)", "Quick exit shortcut (Esc twice)")}
              hint={w(
                "Tekan Esc dua kali dengan cepat. Esc pertama menampilkan petunjuk.",
                "Press Esc twice quickly. The first press shows a hint.",
              )}
            />
            {user?.kind === "account" && (
              <ChoiceGroup
                legend={w("Keluar cepat juga mengeluarkan akun", "Quick exit also signs you out")}
                name="quick-exit-signout"
                value={prefs.quickExitSignOut}
                onChange={(v) =>
                  update(
                    "quickExitSignOut",
                    v as "trusted" | "always" | "never",
                    w("Pilihan keluar cepat diubah.", "Quick exit setting changed."),
                  )
                }
                options={[
                  {
                    value: "trusted",
                    label: w("Kalau aku tidak memilih 'Ingat perangkat ini'", "If I didn't choose 'Remember this device'"),
                  },
                  { value: "always", label: w("Selalu", "Always") },
                  { value: "never", label: w("Tidak pernah", "Never") },
                ]}
              />
            )}
            <Switch
              checked={prefs.neutralTitles}
              onChange={(v) => update("neutralTitles", v, w("Judul tab diubah.", "Tab titles changed."))}
              label={w("Judul tab netral", "Neutral tab titles")}
              hint={w(
                "Nama halaman tidak muncul di bilah tab. Berguna di perangkat bersama.",
                "Page names don't appear in the tab bar. Useful on a shared device.",
              )}
            />
          </section>
          <section className="s-display-group" aria-labelledby="language-title">
            <h2 id="language-title">{w("Bahasa", "Language")}</h2>
            <LanguageSwitch />
          </section>
          <div className="s-actions">
            <Button
              variant="secondary"
              onClick={() => {
                reset();
                setNotice(w("Semua pengaturan kembali ke bawaan.", "Every setting is back to its default."));
                announce("status", w("Pengaturan dikembalikan.", "Settings reset."));
              }}
            >
              {w("Kembalikan ke bawaan", "Reset to defaults")}
            </Button>
          </div>
        </div>
        <aside className="s-preview-box" aria-label={w("Pratinjau", "Preview")}>
          <p className="s-label">{w("Pratinjau", "Preview")}</p>
          <div className="s-preview-turn">
            <span className="s-turn-author">{w("Kamu", "You")}</span>
            <p className="s-turn-body">
              {w("Aku ingin mencoba menuliskan ini pelan-pelan.", "I want to try writing this down slowly.")}
            </p>
          </div>
          <div className="s-preview-turn">
            <span className="s-turn-author">{"Sela"}</span>
            <p className="s-turn-body">
              {w(
                "Ini contoh balasan. Teks asli muncul di ruang cerita, bukan di sini.",
                "This is a sample reply. Your real conversation stays in the talking room, not here.",
              )}
            </p>
          </div>
          <div className="s-preview-badge">
            <span className="s-badge s-badge--ai_summary">{w("Tulisan AI", "AI wording")}</span>
            <span className="s-status-chip s-status-chip--unreviewed">{w("Belum ditinjau", "Not reviewed")}</span>
          </div>
          <p className="s-hint">
            {w("Tema aktif: ", "Active scheme: ")}
            {scheme === "dark"
              ? w("Gelap", "Dark")
              : scheme === "contrast-dark"
                ? w("Gelap, kontras tinggi", "Dark, high contrast")
                : scheme === "contrast-light"
                  ? w("Terang, kontras tinggi", "Light, high contrast")
                  : w("Terang", "Light")}
          </p>
        </aside>
      </div>
    </>
  );
}

/* ------------------------------------------------------------------ */
/* Account                                                             */
/* ------------------------------------------------------------------ */

export function AccountPage({
  user,
  options,
  onDeleted,
}: {
  user: Profile;
  options: Options | null;
  onDeleted: () => void;
}) {
  const w = useWords();
  const loc = useLocale();
  const { setUser, refresh, signOut } = useSession();
  const { announce } = useAnnounce();
  const [error, setError] = useState<unknown>(null);
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [codes, setCodes] = useState<string[] | null>(null);
  const [codesSaved, setCodesSaved] = useState(false);
  const [codePassword, setCodePassword] = useState("");
  const [age, setAge] = useState(user.ageBand);
  const [guardian, setGuardian] = useState<{
    required: boolean;
    status: string;
    active: { guardianName: string | null; relationship: string; grantedAt: string } | null;
  } | null>(null);
  const [guardianLoading, setGuardianLoading] = useState(true);
  const [guardianError, setGuardianError] = useState<unknown>(null);

  const loadGuardian = async () => {
    setGuardianLoading(true);
    setGuardianError(null);
    try {
      const v = await api<{
        required: boolean;
        status: string;
        active: { guardianName: string | null; relationship: string; grantedAt: string } | null;
      }>("/me/guardian-consent");
      setGuardian(v);
    } catch (e) {
      setGuardianError(e);
    } finally {
      setGuardianLoading(false);
    }
  };

  useEffect(() => {
    void loadGuardian();
  }, []);

  const changePassword = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    setNotice("");
    try {
      const r = await api<{ revokedSessions: number }>("/auth/password", {
        currentPassword: current,
        newPassword: next,
      });
      setCurrent("");
      setNext("");
      setNotice(
        r.revokedSessions
          ? w(
              `Kata sandi diganti. ${r.revokedSessions} perangkat lain dikeluarkan.`,
              `Password changed. ${r.revokedSessions} other devices were signed out.`,
            )
          : w("Kata sandi diganti.", "Password changed."),
      );
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };

  const regenerate = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const r = await api<{ recoveryCodes: string[]; generatedAt: string }>(
        "/auth/recovery-codes",
        { password: codePassword },
      );
      setCodes(r.recoveryCodes);
      setCodePassword("");
      setNotice(w("Kode baru dibuat. Kode lama tidak berlaku lagi.", "New codes made. The old ones no longer work."));
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };

  const acknowledge = async () => {
    try {
      await api("/me/recovery-codes/acknowledge", {});
      setCodes(null);
      await refresh();
      announce("status", w("Kode pemulihan dikonfirmasi.", "Recovery codes confirmed."));
    } catch (err) {
      setError(err);
    }
  };

  const changeAge = async (value: string) => {
    setAge(value);
    if (value === "under_15") {
      setNotice(
        w(
          "Sela belum bisa dipakai di usia di bawah 15 tahun. Pilihan tidak disimpan. Kamu bisa mengekspor lalu menghapus datamu.",
          "Sela can't be used under 15. The choice isn't saved. You can export and then delete your data.",
        ),
      );
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const updated = await api<Profile>(
        "/me",
        { expectedVersion: user.version, ageBand: value },
        "PATCH",
      );
      setUser(updated);
      await loadGuardian();
      setNotice(w("Kelompok usia diperbarui.", "Age band updated."));
    } catch (err) {
      setError(err);
      setAge(user.ageBand);
    } finally {
      setBusy(false);
    }
  };

  const withdrawGuardian = async () => {
    setBusy(true);
    try {
      await api("/me/guardian-consent", undefined, "DELETE");
      setGuardian((g) => (g ? { ...g, status: "withdrawn", active: null } : g));
      await refresh();
      setNotice(w("Persetujuan wali dicabut.", "Guardian consent withdrawn."));
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };

  const deleteGuest = async () => {
    setBusy(true);
    try {
      await api("/me", { confirm: true }, "DELETE");
      onDeleted();
    } catch (err) {
      setError(err);
      setBusy(false);
    }
  };

  return (
    <>
      <div className="s-narrow">
        <PageHeader
          title={w("Akun", "Account")}
          lead={w(
            user.kind === "guest"
              ? "Mode tamu dengan data di server Sela."
              : `Masuk sebagai ${user.username}.`,
            user.kind === "guest"
              ? "Guest mode, with your data on Sela's server."
              : `Signed in as ${user.username}.`,
          )}
        />
        <ErrorNotice error={error} />
        {notice && <StatusLine tone="success">{notice}</StatusLine>}
      </div>
      <div className="s-narrow">
        {user.kind === "guest" ? (
          <>
            <Notice tone="info" title={w("Mode tamu", "Guest mode")}>
              <p>
                {w("Data dihapus otomatis jika ", "Data is deleted automatically after ")}
                {Math.floor((user.guest?.inactiveTtlHours ?? 168) / 24)}
                {w(" hari tidak aktif, paling lambat ", " inactive days, at the latest ")}
                {loc.date(user.guest?.maxAgePurgeAt)}
                .
              </p>
              <p>
                {w(
                  "Tamu tidak punya tombol keluar, karena tanpa akun, keluar berarti datamu tidak bisa dibuka lagi.",
                  "Guests have no sign-out button, because without an account, signing out would lock you out of your data.",
                )}
              </p>
              <div className="s-actions">
                <LinkButton to="/settings/account/upgrade" variant="primary">
                  {w("Simpan ke akun", "Save to an account")}
                </LinkButton>
              </div>
            </Notice>
            <div className="s-settings-section">
              <h2>{w("Hapus data tamu", "Delete guest data")}</h2>
              <p>
                {w(
                  "Ini menghapus semua catatan dan percakapan tamu. Tidak bisa dibatalkan.",
                  "This deletes every guest note and conversation. It can't be undone.",
                )}
              </p>
              <InlineConfirm
                label={w("Hapus data tamu sekarang", "Delete guest data now")}
                question={w(
                  "Hapus semua data tamu sekarang? Tindakan ini tidak bisa dibatalkan.",
                  "Delete all guest data now? This cannot be undone.",
                )}
                confirmLabel={w("Ya, hapus semua", "Yes, delete everything")}
                onConfirm={() => void deleteGuest()}
                busy={busy}
              />
            </div>
          </>
        ) : (
          <>
            <section className="s-settings-section">
              <h2>{w("Masuk", "Sign-in")}</h2>
              <DefinitionList
                items={[
                  { term: w("Nama pengguna", "Username"), detail: user.username ?? "" },
                  {
                    term: w("Kode pemulihan", "Recovery codes"),
                    detail: user.recovery
                      ? `${user.recovery.remaining} ${w("tersisa", "left")} · ${w("dibuat", "made")} ${loc.date(user.recovery.generatedAt)}`
                      : "—",
                  },
                ]}
              />
              <div className="s-actions">
                <Button variant="secondary" onClick={() => setCodes([])}>
                  {w("Buat kode pemulihan baru", "Make new recovery codes")}
                </Button>
              </div>
              {codes !== null && codes.length === 0 && (
                <form onSubmit={(e) => void regenerate(e)}>
                  <TextField
                    label={w("Masuk sebagai", "Signed in as")}
                    value={user.username ?? ""}
                    readOnly
                    autoComplete="username"
                  />
                  <PasswordField
                    label={w("Kata sandi saat ini", "Current password")}
                    value={codePassword}
                    required
                    autoComplete="current-password"
                    onChange={(e) => setCodePassword(e.target.value)}
                  />
                  <div className="s-actions">
                    <Button type="submit" variant="secondary" busy={busy}>
                      {w("Buat kode baru", "Make new codes")}
                    </Button>
                    <Button type="button" variant="quiet" onClick={() => setCodes(null)}>
                      {w("Batal", "Cancel")}
                    </Button>
                  </div>
                </form>
              )}
              {codes && codes.length > 0 && (
                <div>
                  <h3>{w("Simpan kode ini sekarang", "Save these codes now")}</h3>
                  <p className="s-hint">
                    {w(
                      "Kode ini hanya ditampilkan sekali. Setiap kode bisa dipakai satu kali.",
                      "These codes are shown once. Each works once.",
                    )}
                  </p>
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
                          .then(() => setNotice(w("Kode disalin.", "Codes copied.")))
                          .catch(() =>
                            setError(w("Salin kode secara manual.", "Copy the codes manually.")),
                          );
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
                  <CheckboxRow checked={codesSaved} onChange={setCodesSaved}>
                    {w(
                      "Aku sudah menyimpan kode ini di tempat aman",
                      "I have saved these codes somewhere safe",
                    )}
                  </CheckboxRow>
                  <Button
                    variant="primary"
                    disabled={!codesSaved}
                    onClick={() => void acknowledge()}
                  >
                    {w("Lanjut", "Continue")}
                  </Button>
                </div>
              )}
            </section>

            <section className="s-settings-section">
              <h2>{w("Ganti kata sandi", "Change password")}</h2>
              <form onSubmit={(e) => void changePassword(e)}>
                <TextField
                  label={w("Masuk sebagai", "Signed in as")}
                  value={user.username ?? ""}
                  readOnly
                  autoComplete="username"
                />
                <PasswordField
                  label={w("Kata sandi saat ini", "Current password")}
                  value={current}
                  required
                  autoComplete="current-password"
                  onChange={(e) => setCurrent(e.target.value)}
                />
                <PasswordField
                  label={w("Kata sandi baru", "New password")}
                  hint={w("Minimal 15 karakter.", "At least 15 characters.")}
                  value={next}
                  required
                  minLength={15}
                  autoComplete="new-password"
                  onChange={(e) => setNext(e.target.value)}
                />
                <Button type="submit" variant="secondary" busy={busy}>
                  {w("Ganti kata sandi", "Change password")}
                </Button>
              </form>
            </section>

            <section className="s-settings-section">
              <h2>{w("Usia", "Age")}</h2>
              <ChoiceGroup
                legend={w("Kelompok usia", "Age band")}
                name="age-band"
                value={age}
                onChange={(v) => void changeAge(v)}
                variant="chips"
                options={[
                  ...(options?.ageBands.allowed ?? ["18_plus"]).map((b) => ({
                    value: b,
                    label:
                      b === "18_plus"
                        ? w("18 tahun ke atas", "18 or older")
                        : b === "15_17"
                          ? "16–17"
                          : b === "unknown"
                            ? w("Tidak disebutkan", "Not stated")
                            : b,
                  })),
                  { value: "under_15", label: w("Di bawah 15 tahun", "Under 15") },
                ]}
              />
              <p className="s-hint">
                {w(
                  "Perubahan usia berlaku untuk perlindungan dan fitur yang sesuai.",
                  "Your age band decides which protections and features apply.",
                )}
              </p>
            </section>

            <section className="s-settings-section">
              <h2>{w("Persetujuan wali", "Guardian consent")}</h2>
              {guardianLoading ? (
                <Loading label={w("Memuat persetujuan wali…", "Loading guardian consent…")} />
              ) : guardianError ? (
                <ErrorNotice
                  error={guardianError}
                  title={w("Belum bisa memuat persetujuan wali", "Couldn't load guardian consent")}
                  onRetry={() => void loadGuardian()}
                />
              ) : guardian?.required ? (
                <>
                  <p>
                    {guardian.status === "active"
                      ? w(
                          `Persetujuan dicatat ${loc.date(guardian.active?.grantedAt)} oleh: ${guardian.active?.relationship === "parent" ? "orang tua" : "wali"}.`,
                          `Consent recorded ${loc.date(guardian.active?.grantedAt)} by: ${guardian.active?.relationship === "parent" ? "parent" : "guardian"}.`,
                        )
                      : w(
                          "Persetujuan wali belum ada atau sudah dicabut.",
                          "Guardian consent is missing or withdrawn.",
                        )}
                  </p>
                  <p className="s-hint">
                    {w(
                      "Persetujuan ini tidak memberi wali akses ke cerita atau catatanmu.",
                      "This consent doesn't give your guardian access to your stories or notes.",
                    )}
                  </p>
                  <div className="s-actions">
                    {guardian.status === "active" ? (
                      <InlineConfirm
                        label={w("Cabut persetujuan wali", "Withdraw guardian consent")}
                        question={w(
                          "Cabut persetujuan wali? Sela tidak bisa dipakai sampai persetujuan dicatat lagi.",
                          "Withdraw guardian consent? Sela can't be used until it is recorded again.",
                        )}
                        confirmLabel={w("Cabut persetujuan", "Withdraw consent")}
                        onConfirm={() => void withdrawGuardian()}
                        icon="ban"
                      />
                    ) : (
                      <LinkButton to="/start/guardian" variant="secondary">
                        {w("Catat persetujuan wali", "Record guardian consent")}
                      </LinkButton>
                    )}
                  </div>
                </>
              ) : (
                <p className="s-muted">
                  {w(
                    "Tidak diperlukan untuk kelompok usiamu.",
                    "Not required for your age band.",
                  )}
                </p>
              )}
            </section>

            <section className="s-settings-section">
              <h2>{w("Keluar", "Sign out")}</h2>
              <div className="s-actions">
                <Button variant="secondary" icon="quick-exit" onClick={() => void signOut()}>
                  {w("Keluar dari akun", "Sign out")}
                </Button>
                <InlineConfirm
                  label={w("Keluar dari semua perangkat", "Sign out everywhere")}
                  question={w(
                    "Semua perangkat, termasuk yang ini, akan keluar dari akun.",
                    "Every device, including this one, will be signed out.",
                  )}
                  confirmLabel={w("Keluar dari semua perangkat", "Sign out everywhere")}
                  onConfirm={async () => {
                    try {
                      await api("/auth/logout-all", { keepCurrent: false });
                      await signOut();
                    } catch (e) {
                      setError(e);
                    }
                  }}
                  icon="quick-exit"
                />
              </div>
            </section>
            <section className="s-settings-section">
              <h2>{w("Hapus akun", "Delete account")}</h2>
              <Link to="/settings/privacy#hapus" className={buttonClass("danger")}>
                <Icon name="trash" />
                {w("Buka penghapusan akun", "Open account deletion")}
              </Link>
            </section>
          </>
        )}
      </div>
    </>
  );
}
