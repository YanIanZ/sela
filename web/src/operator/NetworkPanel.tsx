import { useCallback, useEffect, useState, type FormEvent } from "react";
import { useI18n } from "../i18n";
import { useWords } from "../v3/shared";
import { AdminError, adminApi, type CareOrg, type CareOrgMember, type Invite } from "./api";
import { roleLabel } from "./chrome";
import { Page } from "./shell";
import "./care-panels.css";
import "./network.css";

type Words = (id: string, en: string) => string;

export const ORG_KIND: Record<string, [string, string]> = {
  clinic: ["Klinik", "Clinic"],
  hospital: ["Rumah sakit", "Hospital"],
  puskesmas: ["Puskesmas", "Community health centre"],
  practice: ["Praktik mandiri", "Private practice"],
  organization: ["Organisasi", "Organization"],
};
export const ORG_STATUS: Record<string, [string, string]> = {
  pending: ["Menunggu verifikasi", "Pending verification"],
  verified: ["Terverifikasi", "Verified"],
  suspended: ["Ditangguhkan", "Suspended"],
};
export const STAGES: [string, string, string][] = [
  ["everyday", "Sehari-hari", "Everyday"],
  ["attention", "Perlu perhatian", "Needs attention"],
  ["support", "Butuh dukungan", "Needs support"],
  ["urgent", "Butuh bantuan segera", "Needs help now"],
];
export const MODES: Record<string, [string, string]> = {
  online: ["Daring", "Online"],
  in_person: ["Tatap muka", "In person"],
};
export const PROFESSIONAL_TYPE: Record<string, [string, string]> = {
  psychologist: ["Psikolog", "Psychologist"],
  psychiatrist: ["Psikiater", "Psychiatrist"],
  doctor: ["Dokter", "Doctor"],
  counsellor: ["Konselor", "Counsellor"],
  nurse: ["Perawat", "Nurse"],
  social_worker: ["Pekerja sosial", "Social worker"],
};
const INVITE_STATE: Record<string, [string, string]> = {
  open: ["Menunggu aktivasi", "Waiting for activation"],
  accepted: ["Sudah aktif", "Activated"],
  revoked: ["Dibatalkan", "Revoked"],
  expired: ["Kedaluwarsa", "Expired"],
};
const PARTNER_ROLES: [string, string, string, string, string][] = [
  [
    "practitioner",
    "Tenaga profesional",
    "Practitioner",
    "Membaca rangkuman, mengusulkan waktu, dan tampil ke pengguna dengan jenis profesinya.",
    "Reads summaries, proposes times and is listed to people with their profession.",
  ],
  [
    "partner_admin",
    "Admin klinik",
    "Clinic administrator",
    "Membaca dan menjawab semua permintaan klinik serta melihat lisensi tim.",
    "Reads and answers all of the clinic's requests and sees the team's licences.",
  ],
  [
    "partner_staff",
    "Staf klinik",
    "Clinic staff",
    "Hanya melihat status dan jadwal, tidak pernah isi rangkuman.",
    "Sees status and times only, never the summary.",
  ],
];

export function mapLabel(map: Record<string, [string, string]>, key: string | undefined, w: Words): string {
  if (!key) return "";
  const v = map[key];
  return v ? w(v[0], v[1]) : key;
}

function errorText(err: unknown, w: Words): string {
  if (err instanceof AdminError) {
    if (err.code === "username_taken")
      return w("Nama pengguna itu sudah dipakai atau masih punya undangan aktif.", "That username is taken or still has an open invitation.");
    if (err.status === 403)
      return w(
        "Tindakan ini tidak diizinkan untuk peranmu, atau pembuat tidak boleh memverifikasi organisasi nyata miliknya sendiri.",
        "Your role may not do this, or the creator may not verify a real organisation they created.",
      );
    if (err.status === 409) return w("Status sudah berubah. Muat ulang lalu coba lagi.", "The status already changed. Refresh and try again.");
    if (err.status === 410) return w("Undangan itu sudah tidak aktif.", "That invitation is no longer open.");
    if (err.status === 400) return w("Periksa isian formulir.", "Check the form fields.");
  }
  return w("Permintaan gagal. Coba lagi.", "The request failed. Try again.");
}

function useDate() {
  const { lang } = useI18n();
  const f = new Intl.DateTimeFormat(lang === "en" ? "en-GB" : "id-ID", { dateStyle: "medium", timeStyle: "short" });
  return (iso?: string) => {
    if (!iso) return "";
    const d = new Date(iso);
    return Number.isNaN(d.getTime()) ? iso : f.format(d);
  };
}

/**
 * NetworkPage is the internal view of the professional care network
 * (D-076, D-079): partner organisations, their accounts, licence checks and
 * console invitations.
 */
export function NetworkPage({
  canManage,
  canVerify,
  onExpired,
}: {
  canManage: boolean;
  canVerify: boolean;
  onExpired: () => void;
}) {
  const w = useWords();
  const [orgs, setOrgs] = useState<CareOrg[] | null>(null);
  const [realPartners, setRealPartners] = useState(false);
  const [selected, setSelected] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState("");
  const [form, setForm] = useState(false);

  const fail = useCallback(
    (err: unknown) => {
      if (err instanceof AdminError && err.status === 401) onExpired();
      else setError(errorText(err, w));
    },
    [onExpired, w],
  );

  const load = useCallback(async () => {
    try {
      const r = await adminApi.networkOrgs();
      setOrgs(r.orgs);
      setRealPartners(r.realPartners);
      setSelected((s) => (s && r.orgs.some((o) => o.id === s) ? s : (r.orgs[0]?.id ?? "")));
    } catch (err) {
      fail(err);
    }
  }, [fail]);
  useEffect(() => {
    void load();
  }, [load]);

  const run = async (key: string, job: () => Promise<void>, done: string) => {
    if (busy) return;
    setBusy(key);
    setError("");
    setNotice("");
    try {
      await job();
      setNotice(done);
    } catch (err) {
      fail(err);
    } finally {
      setBusy("");
    }
  };

  const org = orgs?.find((o) => o.id === selected);

  return (
    <Page
      title={w("Jaringan profesional", "Professional network")}
      lead={w(
        "Klinik, praktik, rumah sakit, dan puskesmas mitra beserta akunnya. Akun mitra hanya melihat permintaan organisasinya sendiri. Pembuat organisasi nyata tidak boleh memverifikasinya sendiri.",
        "Partner clinics, practices, hospitals and community health centres with their accounts. A partner account only sees its own organisation's requests. The creator of a real organisation may not verify it.",
      )}
      actions={
        canManage && (
          <button type="button" className="op-btn op-btn--secondary" onClick={() => setForm((v) => !v)} aria-expanded={form}>
            {form ? w("Tutup formulir", "Close form") : w("Tambah organisasi", "Add organisation")}
          </button>
        )
      }
    >
      {!realPartners && (
        <p className="op-hint op-network-sim">
          {w(
            "Deployment ini hanya mengizinkan organisasi simulasi sampai owner memberi persetujuan SELA_CARE_NETWORK_REAL_ACK (B-10 masih terbuka).",
            "This deployment allows simulated organisations only until the owner sets SELA_CARE_NETWORK_REAL_ACK (B-10 is still open).",
          )}
        </p>
      )}
      {error && <p className="op-alert" role="alert">{error}</p>}
      <p className="op-notice" role="status" hidden={!notice}>
        {notice}
      </p>

      {form && canManage && (
        <section className="op-section" aria-labelledby="op-org-new">
          <h2 id="op-org-new">{w("Organisasi baru", "New organisation")}</h2>
          <OrgForm
            realPartners={realPartners}
            busy={busy === "create"}
            onSubmit={(input) =>
              void run(
                "create",
                async () => {
                  const created = await adminApi.createNetworkOrg(input);
                  setForm(false);
                  setSelected(created.id);
                  await load();
                },
                w("Organisasi dibuat dan menunggu verifikasi.", "Organisation created and waiting for verification."),
              )
            }
          />
        </section>
      )}

      {orgs === null ? (
        <p className="op-hint" role="status">{w("Memuat jaringan…", "Loading the network…")}</p>
      ) : orgs.length === 0 ? (
        <section className="op-section">
          <h2>{w("Belum ada organisasi", "No organisations yet")}</h2>
          <p>{w("Tambahkan klinik simulasi untuk mencoba alur bantuan terencana.", "Add a simulated clinic to try the planned support path.")}</p>
        </section>
      ) : (
        <div className="op-two">
          <section className="op-section" aria-labelledby="op-org-list">
            <h2 id="op-org-list">{w("Organisasi", "Organisations")}</h2>
            <ul className="op-org-pick">
              {orgs.map((o) => (
                <li key={o.id}>
                  <button type="button" aria-current={o.id === selected ? "true" : undefined} onClick={() => setSelected(o.id)}>
                    <strong>{o.name}</strong>
                    <span className="op-care-tags">
                      <span className={`op-care-status op-care-status--${o.status === "suspended" ? "revoked" : o.status}`}>
                        {mapLabel(ORG_STATUS, o.status, w)}
                      </span>
                      {o.synthetic && <span className="op-badge op-badge--demo">{w("Simulasi", "Simulation")}</span>}
                      <span className="op-hint">
                        {o.members} {w("akun", "accounts")}
                      </span>
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          </section>
          {org && (
            <OrgDetail
              key={org.id}
              org={org}
              canManage={canManage}
              canVerify={canVerify}
              busy={busy}
              run={run}
              fail={fail}
              onChanged={load}
            />
          )}
        </div>
      )}
    </Page>
  );
}

function OrgDetail({
  org,
  canManage,
  canVerify,
  busy,
  run,
  fail,
  onChanged,
}: {
  org: CareOrg;
  canManage: boolean;
  canVerify: boolean;
  busy: string;
  run: (key: string, job: () => Promise<void>, done: string) => Promise<void>;
  fail: (err: unknown) => void;
  onChanged: () => Promise<void>;
}) {
  const w = useWords();
  const date = useDate();
  const [members, setMembers] = useState<CareOrgMember[] | null>(null);
  const [invites, setInvites] = useState<Invite[] | null>(null);
  const [link, setLink] = useState("");

  const loadMembers = useCallback(async () => {
    try {
      setMembers(await adminApi.networkMembers(org.id));
    } catch (err) {
      fail(err);
    }
  }, [fail, org.id]);
  const loadInvites = useCallback(async () => {
    try {
      setInvites(await adminApi.invites(org.id));
    } catch (err) {
      fail(err);
    }
  }, [fail, org.id]);
  useEffect(() => {
    void loadMembers();
    void loadInvites();
  }, [loadMembers, loadInvites]);

  return (
    <article className="op-section" aria-labelledby="op-org-detail">
      <div className="op-section-head">
        <div>
          <h2 id="op-org-detail">{org.name}</h2>
          <p className="op-care-tags">
            <span className="op-badge op-badge--soft">{mapLabel(ORG_KIND, org.kind, w)}</span>
            <span className={`op-care-status op-care-status--${org.status === "suspended" ? "revoked" : org.status}`}>
              {mapLabel(ORG_STATUS, org.status, w)}
            </span>
            {org.synthetic && <span className="op-badge op-badge--demo">{w("Simulasi", "Simulation")}</span>}
            {org.city && <span className="op-hint">{org.city}</span>}
          </p>
        </div>
        {canVerify && (
          <div className="op-care-actions">
            {org.status !== "verified" && (
              <button
                type="button"
                className="op-btn op-btn--secondary"
                disabled={busy !== ""}
                onClick={() =>
                  void run(
                    `status:${org.id}`,
                    async () => {
                      await adminApi.setNetworkOrgStatus(org.id, "verified");
                      await onChanged();
                    },
                    w("Organisasi terverifikasi dan bisa menerima permintaan.", "Organisation verified and can receive requests."),
                  )
                }
              >
                {w("Verifikasi", "Verify")}
              </button>
            )}
            {org.status !== "suspended" && (
              <button
                type="button"
                className="op-btn op-btn--quiet op-btn--danger-text"
                disabled={busy !== ""}
                onClick={() =>
                  void run(
                    `status:${org.id}`,
                    async () => {
                      await adminApi.setNetworkOrgStatus(org.id, "suspended");
                      await onChanged();
                    },
                    w("Organisasi ditangguhkan. Permintaan baru tidak bisa dikirim.", "Organisation suspended. No new requests can be sent."),
                  )
                }
              >
                {w("Tangguhkan", "Suspend")}
              </button>
            )}
          </div>
        )}
      </div>
      {org.description && <p className="op-care-desc">{org.description}</p>}
      <p className="op-hint">
        {w("Cara layanan", "Service modes")}: {org.modes.map((m) => mapLabel(MODES, m, w)).join(", ") || "—"} ·{" "}
        {w("Cocok untuk tahap", "Fits stages")}:{" "}
        {org.stages.map((s) => {
          const st = STAGES.find(([k]) => k === s);
          return st ? w(st[1], st[2]) : s;
        }).join(", ") || "—"}
      </p>

      <section className="op-network-members" aria-labelledby="op-org-members">
        <h3 id="op-org-members">{w("Akun", "Accounts")}</h3>
        {members === null ? (
          <p className="op-hint">{w("Memuat akun…", "Loading accounts…")}</p>
        ) : members.length === 0 ? (
          <p className="op-hint">{w("Belum ada akun untuk organisasi ini.", "No accounts for this organisation yet.")}</p>
        ) : (
          <ul className="op-network-member-list">
            {members.map((m) => (
              <li key={m.username}>
                <div className="op-care-main">
                  <strong>{m.displayName || m.username}</strong>
                  <span className="op-care-tags">
                    <span className="op-badge op-badge--soft">{roleLabel(m.role, w)}</span>
                    {m.professionalType && <span className="op-hint">{mapLabel(PROFESSIONAL_TYPE, m.professionalType, w)}</span>}
                    <span className="op-hint">@{m.username}</span>
                    {m.status !== "active" && <span className="op-care-status op-care-status--revoked">{w("Dicabut", "Revoked")}</span>}
                  </span>
                  {m.role === "practitioner" && (
                    <p className="op-hint">
                      {w("Lisensi", "Licence")}: {m.licenseNo || w("belum diisi", "not provided")} ·{" "}
                      {m.licenseVerified ? w("terverifikasi", "verified") : w("belum diverifikasi", "not verified")}
                    </p>
                  )}
                </div>
                {canVerify && m.role === "practitioner" && m.licenseNo && !m.licenseVerified && (
                  <button
                    type="button"
                    className="op-btn op-btn--secondary"
                    disabled={busy !== ""}
                    onClick={() =>
                      void run(
                        `license:${m.username}`,
                        async () => {
                          await adminApi.verifyLicense(m.username);
                          await loadMembers();
                        },
                        w(`Lisensi ${m.username} ditandai terverifikasi.`, `${m.username}'s licence is marked verified.`),
                      )
                    }
                  >
                    {w("Verifikasi lisensi", "Verify licence")}
                  </button>
                )}
              </li>
            ))}
          </ul>
        )}
        {canVerify && (
          <p className="op-hint">
            {w(
              "Verifikasi lisensi berarti kamu sudah mencocokkan nomor STR/SIP dengan register resmi (KKI, dinas kesehatan). Kamu tidak bisa memverifikasi lisensimu sendiri.",
              "Verifying a licence means you checked the STR/SIP number against the official register (KKI, health office). You cannot verify your own.",
            )}
          </p>
        )}
      </section>

      <section className="op-network-members" aria-labelledby="op-org-invites">
        <h3 id="op-org-invites">{w("Undangan", "Invitations")}</h3>
        {invites === null ? (
          <p className="op-hint">{w("Memuat undangan…", "Loading invitations…")}</p>
        ) : invites.length === 0 ? (
          <p className="op-hint">{w("Belum ada undangan.", "No invitations yet.")}</p>
        ) : (
          <ul className="op-network-member-list">
            {invites.map((inv) => (
              <li key={inv.id}>
                <div className="op-care-main">
                  <strong>{inv.displayName || inv.username}</strong>
                  <span className="op-care-tags">
                    <span className="op-badge op-badge--soft">{roleLabel(inv.role, w)}</span>
                    <span className="op-hint">@{inv.username}</span>
                    <span
                      className={`op-care-status op-care-status--${inv.state === "open" ? "pending" : inv.state === "accepted" ? "verified" : "revoked"}`}
                    >
                      {mapLabel(INVITE_STATE, inv.state, w)}
                    </span>
                  </span>
                  <p className="op-hint">
                    {inv.state === "open"
                      ? w(`Berlaku sampai ${date(inv.expiresAt)}`, `Valid until ${date(inv.expiresAt)}`)
                      : w(`Dibuat ${date(inv.createdAt)}`, `Created ${date(inv.createdAt)}`)}
                  </p>
                </div>
                {canManage && inv.state === "open" && (
                  <button
                    type="button"
                    className="op-btn op-btn--quiet op-btn--danger-text"
                    disabled={busy !== ""}
                    onClick={() =>
                      void run(
                        `invite:${inv.id}`,
                        async () => {
                          await adminApi.revokeInvite(inv.id);
                          await loadInvites();
                        },
                        w(`Undangan untuk ${inv.username} dibatalkan.`, `The invitation for ${inv.username} was revoked.`),
                      )
                    }
                  >
                    {w("Batalkan undangan", "Revoke invitation")}
                  </button>
                )}
              </li>
            ))}
          </ul>
        )}
      </section>

      {canManage && (
        <section className="op-network-members" aria-labelledby="op-org-invite">
          <h3 id="op-org-invite">{w("Undang akun", "Invite an account")}</h3>
          {link && <ActivationLink url={link} onClose={() => setLink("")} />}
          {org.status === "suspended" ? (
            <p className="op-hint">{w("Organisasi yang ditangguhkan tidak bisa mengundang akun.", "A suspended organisation cannot invite accounts.")}</p>
          ) : (
            <InviteForm
              busy={busy === "invite"}
              onSubmit={(input) =>
                void run(
                  "invite",
                  async () => {
                    const r = await adminApi.createInvite({ ...input, careOrgId: org.id });
                    setLink(r.activationUrl);
                    await loadInvites();
                  },
                  w(`Undangan untuk ${input.username} dibuat.`, `Invitation for ${input.username} created.`),
                )
              }
            />
          )}
          <CliFallback orgId={org.id} />
        </section>
      )}
    </article>
  );
}

function ActivationLink({ url, onClose }: { url: string; onClose: () => void }) {
  const w = useWords();
  const [copied, setCopied] = useState(false);
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(url);
      setCopied(true);
    } catch {
      setCopied(false);
    }
  };
  return (
    <div className="op-once" role="status">
      <strong>{w("Tautan aktivasi", "Activation link")}</strong>
      <p>
        {w(
          "Tautan ini hanya tampil sekali dan berlaku 72 jam. Kirim lewat kanal yang aman. Sela tidak pernah mengetahui kata sandi atau kode MFA mereka.",
          "This link is shown once and is valid for 72 hours. Send it over a safe channel. Sela never learns their password or MFA code.",
        )}
      </p>
      <code>{url}</code>
      <div className="op-partner-row">
        <button type="button" className="op-btn op-btn--secondary" onClick={() => void copy()}>
          {copied ? w("Tersalin", "Copied") : w("Salin tautan", "Copy link")}
        </button>
        <button type="button" className="op-btn op-btn--quiet" onClick={onClose}>
          {w("Sudah saya simpan", "I've saved it")}
        </button>
      </div>
    </div>
  );
}

function InviteForm({
  busy,
  onSubmit,
}: {
  busy: boolean;
  onSubmit: (input: { username: string; role: string; displayName: string; professionalType: string; licenseNo: string }) => void;
}) {
  const w = useWords();
  const [role, setRole] = useState("practitioner");
  const [username, setUsername] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [professionalType, setProfessionalType] = useState("psychologist");
  const [licenseNo, setLicenseNo] = useState("");
  const practitioner = role === "practitioner";
  const usernameOk = /^[a-z0-9][a-z0-9._-]{2,31}$/.test(username);
  const ok = usernameOk && (!practitioner || displayName.trim().length >= 2);

  const submit = (e: FormEvent) => {
    e.preventDefault();
    if (!ok || busy) return;
    onSubmit({
      username,
      role,
      displayName: displayName.trim(),
      professionalType: practitioner ? professionalType : "",
      licenseNo: practitioner ? licenseNo.trim() : "",
    });
    setUsername("");
    setDisplayName("");
    setLicenseNo("");
  };

  return (
    <form className="op-care-form" onSubmit={submit} aria-label={w("Undang akun", "Invite an account")}>
      <fieldset className="op-field op-field--wide op-network-set">
        <legend>{w("Peran", "Role")}</legend>
        {PARTNER_ROLES.map(([v, id, en, hid, hen]) => (
          <label key={v} className="op-check">
            <input type="radio" name="op-invite-role" value={v} checked={role === v} onChange={() => setRole(v)} />
            <span>
              <strong>{w(id, en)}</strong> — <span className="op-hint">{w(hid, hen)}</span>
            </span>
          </label>
        ))}
      </fieldset>
      <label className="op-field">
        <span>{w("Nama pengguna", "Username")}</span>
        <input
          className="op-input"
          required
          value={username}
          autoComplete="off"
          aria-describedby="op-invite-username-hint"
          onChange={(e) => setUsername(e.target.value.toLowerCase())}
        />
        <span className="op-hint" id="op-invite-username-hint">
          {w("3–32 karakter: huruf kecil, angka, titik, garis bawah, atau tanda hubung.", "3–32 characters: lower-case letters, digits, dot, underscore or hyphen.")}
        </span>
      </label>
      <label className="op-field">
        <span>{practitioner ? w("Nama tampilan", "Display name") : w("Nama tampilan (opsional)", "Display name (optional)")}</span>
        <input className="op-input" maxLength={120} value={displayName} onChange={(e) => setDisplayName(e.target.value)} />
      </label>
      {practitioner && (
        <>
          <label className="op-field">
            <span>{w("Jenis profesi", "Profession")}</span>
            <select className="op-input" value={professionalType} onChange={(e) => setProfessionalType(e.target.value)}>
              {Object.entries(PROFESSIONAL_TYPE).map(([k, v]) => (
                <option key={k} value={k}>
                  {w(v[0], v[1])}
                </option>
              ))}
            </select>
          </label>
          <label className="op-field">
            <span>{w("Nomor lisensi STR/SIP (opsional)", "Licence number STR/SIP (optional)")}</span>
            <input className="op-input" maxLength={64} value={licenseNo} onChange={(e) => setLicenseNo(e.target.value)} />
          </label>
        </>
      )}
      <div className="op-actions">
        <button className="op-btn op-btn--primary" type="submit" disabled={busy || !ok}>
          {busy ? w("Membuat…", "Creating…") : w("Buat undangan", "Create invitation")}
        </button>
      </div>
    </form>
  );
}

function CliFallback({ orgId }: { orgId: string }) {
  const w = useWords();
  const [copied, setCopied] = useState(false);
  const cli = `sela admin create-partner --username … --role practitioner --org ${orgId} --display-name … --professional-type psychologist --license …`;
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(cli);
      setCopied(true);
    } catch {
      setCopied(false);
    }
  };
  return (
    <details className="op-network-cli">
      <summary>{w("Cara manual (SSH)", "Manual route (SSH)")}</summary>
      <p className="op-hint">
        {w(
          "Untuk keadaan darurat: jalankan di server (kata sandi dibaca dari stdin; rahasia TOTP ditampilkan sekali).",
          "For emergencies: run on the server (the password is read from stdin; the TOTP secret is shown once).",
        )}
      </p>
      <code>{cli}</code>
      <button type="button" className="op-btn op-btn--quiet" onClick={() => void copy()}>
        {copied ? w("Tersalin", "Copied") : w("Salin perintah", "Copy command")}
      </button>
    </details>
  );
}

function OrgForm({
  realPartners,
  busy,
  onSubmit,
}: {
  realPartners: boolean;
  busy: boolean;
  onSubmit: (input: Record<string, unknown>) => void;
}) {
  const w = useWords();
  const [kind, setKind] = useState("clinic");
  const [name, setName] = useState("");
  const [city, setCity] = useState("");
  const [description, setDescription] = useState("");
  const [modes, setModes] = useState<string[]>(["online"]);
  const [stages, setStages] = useState<string[]>([]);
  const [synthetic, setSynthetic] = useState(true);
  const flip = (list: string[], v: string) => (list.includes(v) ? list.filter((x) => x !== v) : [...list, v]);

  const submit = (e: FormEvent) => {
    e.preventDefault();
    onSubmit({ kind, name, city, description, modes, stages, synthetic: realPartners ? synthetic : true });
  };

  return (
    <form className="op-care-form" onSubmit={submit} aria-label={w("Organisasi baru", "New organisation")}>
      <label className="op-field">
        <span>{w("Nama organisasi", "Organisation name")}</span>
        <input className="op-input" required minLength={2} maxLength={160} value={name} onChange={(e) => setName(e.target.value)} />
      </label>
      <label className="op-field">
        <span>{w("Jenis", "Kind")}</span>
        <select className="op-input" value={kind} onChange={(e) => setKind(e.target.value)}>
          {Object.entries(ORG_KIND).map(([k, v]) => (
            <option key={k} value={k}>
              {w(v[0], v[1])}
            </option>
          ))}
        </select>
      </label>
      <label className="op-field">
        <span>{w("Kota", "City")}</span>
        <input className="op-input" maxLength={120} value={city} onChange={(e) => setCity(e.target.value)} />
      </label>
      <label className="op-field op-field--wide">
        <span>{w("Deskripsi untuk pengguna", "Description for people")}</span>
        <textarea className="op-input" rows={2} maxLength={1000} value={description} onChange={(e) => setDescription(e.target.value)} />
      </label>
      <fieldset className="op-field op-network-set">
        <legend>{w("Cara layanan", "Service modes")}</legend>
        {Object.entries(MODES).map(([v, l]) => (
          <label key={v} className="op-check">
            <input type="checkbox" checked={modes.includes(v)} onChange={() => setModes((m) => flip(m, v))} />
            <span>{w(l[0], l[1])}</span>
          </label>
        ))}
      </fieldset>
      <fieldset className="op-field op-network-set">
        <legend>{w("Cocok untuk tahap", "Fits stages")}</legend>
        {STAGES.map(([v, id, en]) => (
          <label key={v} className="op-check">
            <input type="checkbox" checked={stages.includes(v)} onChange={() => setStages((s) => flip(s, v))} />
            <span>{w(id, en)}</span>
          </label>
        ))}
      </fieldset>
      <div className="op-field op-field--wide">
        <label className="op-check">
          <input
            type="checkbox"
            checked={realPartners ? synthetic : true}
            disabled={!realPartners}
            onChange={(e) => setSynthetic(e.target.checked)}
          />
          <span>{w("Organisasi simulasi (berlabel Simulasi untuk pengguna)", "Simulated organisation (labelled Simulation for people)")}</span>
        </label>
        {!realPartners && (
          <p className="op-hint">
            {w("Wajib simulasi: organisasi nyata belum diizinkan di deployment ini.", "Simulation is required: real organisations are not allowed on this deployment yet.")}
          </p>
        )}
      </div>
      <div className="op-actions">
        <button className="op-btn op-btn--primary" type="submit" disabled={busy || modes.length === 0}>
          {busy ? w("Menyimpan…", "Saving…") : w("Buat organisasi", "Create organisation")}
        </button>
      </div>
    </form>
  );
}
