import { useEffect, useState } from "react";
import { useI18n } from "../../i18n";
import { api, type Consent, type Profile } from "../api";
import { useSession } from "../session";
import { useAnnounce, useWords } from "../shared";
import { helpData, channelHref } from "../helpdata";
import { Icon } from "../icons";
import {
  Button,
  ErrorNotice,
  LinkButton,
  PageHeader,
  buttonClass,
} from "../ui";

export function NotFoundPage() {
  const w = useWords();
  const { user } = useSession();
  return (
    <div className="s-system">
      <PageHeader title={w("Halaman ini tidak ditemukan.", "This page wasn't found.")} />
      <p className="s-muted">
        {w(
          "Mungkin tautannya salah, atau halaman ini sudah dipindahkan. Tidak ada yang hilang dari catatanmu.",
          "The link may be wrong, or this page has moved. Nothing is missing from your notes.",
        )}
      </p>
      <div className="s-actions">
        <LinkButton to={user ? "/home" : "/"} variant="primary">
          {user ? w("Ke beranda", "Go to Home") : w("Ke pengenalan", "Go to Introduction")}
        </LinkButton>
        <LinkButton to="/help" variant="help" icon="help">
          {w("Bantuan sekarang", "Help now")}
        </LinkButton>
      </div>
    </div>
  );
}

export function SessionEndedPage({ guest }: { guest: boolean }) {
  const w = useWords();
  return (
    <div className="s-system">
      <PageHeader
        title={
          guest
            ? w("Data tamu tidak bisa dibuka lagi dari browser ini.", "Guest data can't be opened from this browser any more.")
            : w("Sesimu sudah berakhir.", "Your session has ended.")
        }
      />
      <p>
        {guest
          ? w(
              "Data itu akan dihapus otomatis sesuai jadwal. Kamu bisa mulai lagi atau membuka bantuan.",
              "That data will be deleted automatically on schedule. You can start again or open help.",
            )
          : w(
              "Masuk lagi untuk melanjutkan. Tulisan yang belum terkirim masih ada di tab ini.",
              "Sign in again to continue. Anything you hadn't sent is still in this tab.",
            )}
      </p>
      <div className="s-actions">
        {!guest && (
          <LinkButton to="/signin?next=/home" variant="primary">
            {w("Masuk", "Sign in")}
          </LinkButton>
        )}
        {guest && (
          <LinkButton to="/start/age" variant="primary">
            {w("Mulai lagi", "Start again")}
          </LinkButton>
        )}
        <LinkButton to="/help" variant="help" icon="help">
          {w("Bantuan sekarang", "Help now")}
        </LinkButton>
        <LinkButton to="/" variant="quiet">
          {w("Kembali ke pengenalan", "Back to the Introduction")}
        </LinkButton>
      </div>
    </div>
  );
}

export function AgeBlockedPage({ reason }: { reason: string }) {
  const w = useWords();
  const { lang } = useI18n();
  const title =
    reason === "age15"
      ? w("Sela belum bisa dipakai di usia 15 tahun.", "Sela can't be used at age 15 yet.")
      : reason === "band"
        ? w("Sela belum tersedia untuk kelompok usia ini.", "Sela isn't available for this age group yet.")
        : w("Sela belum bisa dipakai di usiamu.", "Sela can't be used at your age yet.");
  const help = helpData.help.slice(0, 2);
  return (
    <div className="s-system">
      <PageHeader
        title={title}
        lead={w(
          "Kalau kamu butuh bantuan sekarang, layanan di bawah bisa kamu hubungi.",
          "If you need help now, you can contact the services below.",
        )}
      />
      <p>
        {w(
          "Tidak ada data yang dibuat. Kamu bisa menutup halaman ini kapan saja.",
          "No data was created. You can close this page at any time.",
        )}
      </p>
      <div className="s-help-list">
        {help.map((s) => (
          <article className="s-service" key={s.id}>
            <h2>{s.name}</h2>
            <p>{s.description[lang]}</p>
            <div className="s-actions">
              {s.channels.map((c) => (
                <a
                  key={c.kind + c.value}
                  className={buttonClass("secondary")}
                  href={channelHref(c)}
                  target={c.kind === "web" ? "_blank" : undefined}
                  rel={c.kind === "web" ? "noreferrer noopener" : undefined}
                >
                  <Icon name={c.kind === "phone" ? "phone" : c.kind === "whatsapp" ? "whatsapp" : "external"} />
                  {c.kind === "phone" ? w("Telepon ", "Call ") + s.name : s.name}
                </a>
              ))}
            </div>
          </article>
        ))}
      </div>
      <div className="s-actions">
        <LinkButton to="/help" variant="help" icon="help">
          {w("Semua layanan bantuan", "All support services")}
        </LinkButton>
        <LinkButton to="/" variant="quiet">
          {w("Ke pengenalan", "Go to Introduction")}
        </LinkButton>
      </div>
    </div>
  );
}

export function GuardianGatePage({ user }: { user: Profile }) {
  const w = useWords();
  return (
    <div className="s-system">
      <PageHeader
        title={w("Persetujuan wali belum ada.", "Guardian consent is missing.")}
        lead={w(
          "Sela tidak bisa dipakai sampai persetujuan dicatat lagi. Kamu tetap bisa mengekspor atau menghapus datamu, dan membuka Bantuan sekarang.",
          "Sela can't be used until consent is recorded again. You can still export or delete your data, and open Help now.",
        )}
      />
      <div className="s-actions">
        <LinkButton to="/start/guardian" variant="primary">
          {w("Catat persetujuan wali", "Record guardian consent")}
        </LinkButton>
        <LinkButton to="/settings/privacy#ekspor" variant="secondary">
          {w("Ekspor data", "Export data")}
        </LinkButton>
        <LinkButton to="/settings/privacy#hapus" variant="danger" icon="trash">
          {w("Hapus data", "Delete data")}
        </LinkButton>
        <LinkButton to="/help" variant="help" icon="help">
          {w("Bantuan sekarang", "Help now")}
        </LinkButton>
      </div>
      <p className="s-hint">
        {w(
          "Akun: ",
          "Account: ",
        )}
        {user.username ?? w("tamu", "guest")}
      </p>
      <LinkButton to="/settings/display" variant="quiet">
        {w("Tampilan & aksesibilitas", "Display & accessibility")}
      </LinkButton>
    </div>
  );
}

export function DataDeletedPage({ onRestart }: { onRestart: () => void }) {
  const w = useWords();
  return (
    <div className="s-system">
      <PageHeader
        title={w("Datamu sudah dihapus dari Sela.", "Your data has been deleted from Sela.")}
        lead={w(
          "Dalam 7 hari tidak ada cadangan yang bisa membukanya, dan dalam 30 hari semua cadangan ikut terhapus.",
          "Within 7 days no backup can open it, and within 30 days it's gone from every backup.",
        )}
      />
      <p className="s-hint">
        {w(
          "Salinan yang sudah kamu unduh tidak bisa ditarik kembali.",
          "Copies you downloaded can't be recalled.",
        )}
      </p>
      <div className="s-actions">
        <Button variant="primary" onClick={onRestart}>
          {w("Mulai lagi", "Start again")}
        </Button>
        <LinkButton to="/help" variant="help" icon="help">
          {w("Bantuan sekarang", "Help now")}
        </LinkButton>
      </div>
    </div>
  );
}

export function PolicyGatePage({ user }: { user: Profile }) {
  const w = useWords();
  const { lang } = useI18n();
  const { policies, refresh } = useSession();
  const { announce } = useAnnounce();
  const [consents, setConsents] = useState<Consent[] | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const action = user.actionsRequired.find((a) => a.startsWith("accept_policy:"));
  const purpose = action?.slice("accept_policy:".length) ?? "";
  const policy = policies[purpose];

  useEffect(() => {
    api<{ consents: Consent[] }>("/me/consents")
      .then((d) => setConsents(d.consents))
      .catch(setError);
  }, []);

  const agree = async () => {
    const c = consents?.find((x) => x.purpose === purpose);
    if (!c) return;
    setBusy(true);
    setError(null);
    try {
      await api(
        "/me/consents/" + purpose,
        { granted: true, policyVersion: c.currentPolicyVersion },
        "PUT",
      );
      await refresh();
      announce("status", w("Kebijakan disetujui.", "Policy accepted."));
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="s-system">
      <PageHeader
        title={w("Kebijakan privasi diperbarui.", "The privacy policy was updated.")}
        lead={w(
          "Ada versi baru yang perlu kamu baca. Pilihan privasi tetap bisa dibuka sebelum kamu menyetujui.",
          "There is a new version to read. Privacy choices stay open before you agree.",
        )}
      />
      <ErrorNotice error={error} />
      <div className="s-actions">
        <Button variant="primary" busy={busy} onClick={() => void agree()} disabled={!consents}>
          {w("Setuju", "Agree")}
        </Button>
        <LinkButton to={"/policies/" + purpose} variant="secondary">
          {w("Baca lengkap", "Read it in full")}
        </LinkButton>
        <LinkButton to="/settings/privacy" variant="quiet">
          {w("Buka Privasi & data", "Open Privacy & data")}
        </LinkButton>
      </div>
      <p className="s-hint">
        {policy?.title[lang] ?? purpose}
      </p>
    </div>
  );
}
