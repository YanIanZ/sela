import { useEffect, useState } from "react";
import { Link } from "../../lib/router";
import { useI18n } from "../../i18n";
import { api, type Consent } from "../api";
import { useSession } from "../session";
import { useWords } from "../shared";
import { ErrorNotice, LinkButton, Loading, PageHeader } from "../ui";

export function PoliciesPage({ purpose }: { purpose?: string }) {
  const w = useWords();
  const { lang } = useI18n();
  const { policies, user, loading } = useSession();
  const [consents, setConsents] = useState<Consent[] | null>(null);
  const [error, setError] = useState<unknown>(null);

  useEffect(() => {
    if (!user) return;
    api<{ consents: Consent[] }>("/me/consents")
      .then((d) => setConsents(d.consents))
      .catch(setError);
  }, [user]);

  if (purpose) {
    const policy = policies[purpose];
    if (!policy && !loading) {
      return (
        <div className="s-system">
          <PageHeader
            back={{ to: "/policies", label: w("Kebijakan", "Policies") }}
            title={w("Kebijakan ini tidak ditemukan.", "This policy wasn't found.")}
          />
          <LinkButton to="/policies" variant="secondary">
            {w("Lihat semua kebijakan", "See every policy")}
          </LinkButton>
        </div>
      );
    }
    if (!policy) return <Loading label={w("Memuat kebijakan…", "Loading the policy…")} />;
    const consent = consents?.find((c) => c.purpose === purpose);
    return (
      <div className="s-narrow">
        <PageHeader
          back={{ to: "/policies", label: w("Kebijakan", "Policies") }}
          title={policy.title[lang]}
        />
        <p className="s-version-line">
          {w("Versi ", "Version ")}
          {policy.version}
        </p>
        {user && consent && (
          <p className="s-small">
            {consent.granted && !consent.outdated
              ? w("Kamu menyetujui versi ini.", "You agreed to this version.")
              : w("Kamu belum menyetujui versi terbaru.", "You haven't agreed to the latest version yet.")}{" "}
            <Link to="/settings/privacy#persetujuan">{w("Kelola persetujuan", "Manage consents")}</Link>
          </p>
        )}
        <ErrorNotice error={error} />
        <section className="s-policy-section" aria-labelledby="short-title">
          <h2 id="short-title">{w("Singkatnya", "In short")}</h2>
          <div className="s-policy-body">
            {policy.body.slice(0, 3).map((p, i) => (
              <p key={i}>{p[lang]}</p>
            ))}
          </div>
        </section>
        <section className="s-policy-section" aria-labelledby="full-title">
          <h2 id="full-title">{w("Teks lengkap", "Full text")}</h2>
          <div className="s-policy-body">
            {policy.body.map((p, i) => (
              <p key={i}>{p[lang]}</p>
            ))}
          </div>
        </section>
        <p className="s-hint s-mt-4">
          {w(
            `Versi ini: ${policy.version}. Teks ini adalah draf teknis yang menunggu tinjauan hukum.`,
            `This version: ${policy.version}. This text is a technical draft awaiting legal review.`,
          )}
        </p>
        <div className="s-actions">
          <LinkButton to="/settings/privacy#persetujuan" variant="quiet">
            {w("Kembali ke pengaturan", "Back to settings")}
          </LinkButton>
        </div>
      </div>
    );
  }

  const entries = Object.entries(policies);
  return (
    <>
      <PageHeader
        back={{ to: "/", label: w("Pengenalan", "Introduction") }}
        title={w("Kebijakan", "Policies")}
        lead={w(
          "Baca setiap kebijakan dalam bahasa yang jelas, sebelum dan sesudah kamu menyetujuinya.",
          "Read each policy in plain language, before and after you agree to it.",
        )}
      />
      {entries.length === 0 ? (
        <Loading label={w("Memuat kebijakan…", "Loading policies…")} />
      ) : (
        <div className="s-policy-index">
          {entries.map(([key, policy]) => (
            <Link className="s-policy-row" to={"/policies/" + key} key={key}>
              <span>
                <strong>{policy.title[lang]}</strong>
                <br />
                <small>
                  {w("Versi ", "Version ")}
                  {policy.version}
                </small>
              </span>
              <span className="s-muted" aria-hidden="true">→</span>
            </Link>
          ))}
        </div>
      )}
      <p className="s-hint s-mt-4">
        {w(
          "Versi sebelumnya belum tersedia sebagai halaman terpisah. Isi dan tanggalnya dicatat di setiap kebijakan.",
          "Earlier versions aren't available as separate pages yet. Each policy records its text and date.",
        )}
      </p>
    </>
  );
}
