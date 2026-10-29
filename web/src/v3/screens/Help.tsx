import { useEffect, useState } from "react";
import { Link, useRouter } from "../../lib/router";
import { useI18n } from "../../i18n";
import { api } from "../api";
import { useOnline, useWords } from "../shared";
import { useSession } from "../session";
import { Icon } from "../icons";
import { channelHref, helpData, isStale } from "../helpdata";
import {
  LinkButton,
  Notice,
  PageHeader,
  buttonClass,
} from "../ui";

export function HelpPage() {
  const w = useWords();
  const { user } = useSession();
  const { query } = useRouter();
  const online = useOnline();
  const [channel, setChannel] = useState<string>("");
  const services = helpData.help;
  const urgent = services.slice(0, 2);
  // The directory lists the remaining services; the two above are not repeated.
  const others = services.slice(2);
  const filtered = channel
    ? others.filter((s) => s.channels.some((c) => c.kind === channel))
    : others;
  const fromRoom = query.get("from") === "talk";

  return (
    <>
      <section className="s-emergency" aria-labelledby="page-title">
        <PageHeader
          title={w("Bantuan sekarang", "Help now")}
          lead={w(
            "Kalau kamu dalam bahaya sekarang, hubungi layanan darurat setempat atau orang terdekat yang bisa menemanimu.",
            "If you're in danger right now, contact local emergency services or someone nearby who can stay with you.",
          )}
        />
        <p>
          <strong>
            {w(
              "Sela tidak bisa menghubungi siapa pun atas namamu.",
              "Sela can't contact anyone on your behalf.",
            )}
          </strong>
        </p>
        <p className="s-hint">
          {w(
            "Daftar ini hanya untuk Indonesia. Bahasa yang kamu pilih tidak menentukan negara atau layanan yang tersedia untukmu.",
            "This list is for Indonesia only. Your chosen language doesn't decide your country or which services are available.",
          )}
        </p>
      </section>

      {!online && (
        <Notice tone="warning" title={w("Kamu sedang offline", "You're offline")}>
          <p>
            {w(
              "Data layanan di bawah dimuat dari aplikasi, diperiksa 26 Okt 2026. Tautan telepon dan situs tetap bisa dibuka jika perangkatmu mendukungnya.",
              "The service data below comes from the app, checked 26 Oct 2026. Phone and website links still open if your device supports them.",
            )}
          </p>
        </Notice>
      )}

      <section className="s-section" aria-labelledby="urgent-title">
        <h2 id="urgent-title">{w("Layanan yang bisa dihubungi sekarang", "Services you can contact now")}</h2>
        <div className="s-help-list">
          {urgent.map((s) => (
            <ServiceCard key={s.id} resource={s} />
          ))}
        </div>
      </section>

      {fromRoom && (
        <p style={{ marginTop: "var(--space-4)" }}>
          <Link to="/talk" className={buttonClass("secondary")}>
            <Icon name="arrow-left" />
            {w("Kembali ke percakapan", "Back to the conversation")}
          </Link>
        </p>
      )}

      <section className="s-section" id="layanan" aria-labelledby="directory-title">
        <h2 id="directory-title">{w("Layanan bantuan lainnya", "Other support services")}</h2>
        <div className="s-filters" role="group" aria-label={w("Saring layanan", "Filter services")}>
          {[
            { kind: "phone", id: "Telepon", en: "Phone" },
            { kind: "whatsapp", id: "WhatsApp", en: "WhatsApp" },
            { kind: "web", id: "Situs", en: "Website" },
          ].map((f) => (
            <button
              key={f.kind}
              type="button"
              className="s-filter"
              aria-pressed={channel === f.kind}
              onClick={() => setChannel(channel === f.kind ? "" : f.kind)}
            >
              {channel === f.kind && <Icon name="check" size={16} />}
              {w(f.id, f.en)}
            </button>
          ))}
          {channel && (
            <button type="button" className={buttonClass("quiet")} onClick={() => setChannel("")}>
              {w("Hapus filter", "Clear filter")}
            </button>
          )}
        </div>
        {filtered.length === 0 ? (
          <p className="s-hint">
            {w(
              "Tidak ada layanan lain dengan saluran ini. Layanan di atas tetap bisa dihubungi.",
              "No other services use this channel. The services above can still be contacted.",
            )}
          </p>
        ) : (
          <div className="s-help-list">
            {filtered.map((s) => (
              <ServiceCard key={s.id} resource={s} />
            ))}
          </div>
        )}
      </section>

      <SelfHelpLibrary />

      <section className="s-section">
        <h2>{w("Bacaan yang mungkin membantu", "Reading that may help")}</h2>
        <p className="s-hint">
          {w(
            "Bacaan dan latihan kecil belum bisa dibuka; setiap bacaan menunggu tinjauan editor sebelum ditampilkan.",
            "Reading and small exercises can't be opened yet; every item awaits an editor's review before it is shown.",
          )}
        </p>
        <LinkButton to="/reading" variant="quiet">
          {w("Tentang bacaan & latihan kecil", "About reading & small exercises")}
        </LinkButton>
      </section>

      <section className="s-section">
        <h2>{w("Bantuan terencana", "Planned support")}</h2>
        <p className="s-hint">
          {w(
            "Cari organisasi, fasilitas, atau praktisi dari sumber publik. Kamu yang menghubungi; tidak ada permintaan atau pemesanan otomatis.",
            "Find organizations, facilities or practitioners from public sources. You make contact; there is no automatic request or booking.",
          )}
        </p>
        <LinkButton to="/support/directory" variant="quiet">
          {w("Cari bantuan terencana", "Find planned support")}
        </LinkButton>
      </section>

      {user?.ageBand === "15_17" || user?.ageBand === "unknown" ? (
        <p className="s-hint">
          {w(
            "Layanan khusus remaja hanya ditampilkan jika sudah diverifikasi. Daftar di atas adalah layanan umum yang terverifikasi.",
            "Teen-specific services appear only when verified. The list above is the verified general list.",
          )}
        </p>
      ) : null}
    </>
  );
}

type SelfHelpCard = {
  id: string;
  title: { id: string; en: string };
  summary: { id: string; en: string };
  steps: { id: string[]; en: string[] };
  why: { id: string; en: string };
  seekHelpWhen: { id: string; en: string };
  cautions: { id: string; en: string };
  topics: string[];
};

/** The reviewed v1 self-help library, now served by the Go runtime. */
function SelfHelpLibrary() {
  const w = useWords();
  const { lang } = useI18n();
  const [cards, setCards] = useState<SelfHelpCard[] | null>(null);
  const [error, setError] = useState<unknown>(null);

  useEffect(() => {
    let active = true;
    api<{ selfHelpCards: SelfHelpCard[] }>("/help-content")
      .then((d) => {
        if (active) setCards(d.selfHelpCards);
      })
      .catch((e) => {
        if (active) setError(e);
      });
    return () => {
      active = false;
    };
  }, []);

  if (error || !cards?.length) return null;

  return (
    <section className="s-section" aria-labelledby="self-help-title">
      <h2 id="self-help-title">{w("Latihan kecil", "Small exercises")}</h2>
      <p className="s-hint">
        {w(
          "Dipilih dari pustaka yang sudah ditinjau, dengan catatan sumbernya. Ini bukan pengganti bantuan profesional.",
          "Chosen from a reviewed library, with its source notes. This is not a replacement for professional help.",
        )}
      </p>
      <div className="s-help-list">
        {cards.slice(0, 6).map((card) => (
          <details className="s-service" key={card.id}>
            <summary>
              <strong>{card.title[lang]}</strong>
            </summary>
            <p>{card.summary[lang]}</p>
            <ol>
              {card.steps[lang].map((step) => (
                <li key={step}>{step}</li>
              ))}
            </ol>
            <p className="s-small s-muted">{card.why[lang]}</p>
            <p className="s-hint">{card.cautions[lang]}</p>
            <p className="s-hint">{card.seekHelpWhen[lang]}</p>
          </details>
        ))}
      </div>
    </section>
  );
}

function ServiceCard({ resource }: { resource: (typeof helpData.help)[number] }) {
  const w = useWords();
  const { lang } = useI18n();
  const stale = isStale(resource);
  return (
    <article className="s-service">
      <h3>{resource.name}</h3>
      <p>{resource.description[lang]}</p>
      <p className="s-small s-muted">
        {resource.hours[lang]} · {resource.cost[lang]}
      </p>
      {stale && (
        <p className="s-status-chip s-status-chip--corrected">
          <Icon name="clock" size={16} />
          {w(
            "Lewat tanggal periksa ulang — cek situs resminya",
            "Past its re-check date — check the official site",
          )}
        </p>
      )}
      <div className="s-actions">
        {resource.channels.map((c) => (
          <a
            key={c.kind + c.value}
            className={buttonClass("secondary")}
            href={channelHref(c)}
            target={c.kind === "web" ? "_blank" : undefined}
            rel={c.kind === "web" ? "noreferrer noopener" : undefined}
          >
            <Icon name={c.kind === "phone" ? "phone" : c.kind === "whatsapp" ? "whatsapp" : "external"} />
            {c.kind === "phone"
              ? w("Telepon ", "Call ") + resource.name
              : c.kind === "whatsapp"
                ? "WhatsApp · " + resource.name
                : w("Buka situs ", "Open website ") + resource.name}
          </a>
        ))}
      </div>
      <p className="s-source">
        {w("Sumber: ", "Source: ")}
        {resource.sources.map((s, i) => (
          <span key={s.url}>
            {i > 0 ? " · " : ""}
            <a href={s.url} target="_blank" rel="noreferrer noopener">
              {s.label}
            </a>
          </span>
        ))}
        {" · "}
        {w("diverifikasi ", "verified ")}
        {resource.lastVerified}
        {" · "}
        {w("periksa ulang sebelum ", "re-check before ")}
        {resource.reverifyBy}
      </p>
      <p className="s-source">
        {w(
          "Sela tidak memantau percakapan dan tidak dapat menghubungi siapa pun atas namamu.",
          "Sela does not monitor conversations and cannot contact anyone on your behalf.",
        )}
      </p>
    </article>
  );
}
