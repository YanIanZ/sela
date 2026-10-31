import { useEffect, useState } from "react";
import { Link } from "../../lib/router";
import { useI18n } from "../../i18n";
import { api, V3Error, type ReadingItem, type ReadingListItem } from "../api";
import { Icon } from "../icons";
import { useWords } from "../shared";
import {
  ErrorNotice,
  LinkButton,
  Loading,
  Notice,
  PageHeader,
  TextField,
} from "../ui";

const sources = [
  { publisher: "WHO", title: { id: "Doing What Matters in Times of Stress", en: "Doing What Matters in Times of Stress" }, topic: "stress", language: { id: "Panduan dengan pilihan bahasa", en: "Guide with language options" }, url: "https://www.who.int/publications/i/item/9789240003927" },
  { publisher: "WHO", title: { id: "Tanya jawab tentang stres", en: "Questions and answers about stress" }, topic: "stress", language: { id: "Bahasa Inggris", en: "English" }, url: "https://www.who.int/news-room/questions-and-answers/item/stress" },
  { publisher: "NHS", title: { id: "Mengenal kesejahteraan mental", en: "Mental wellbeing tips" }, topic: "wellbeing", language: { id: "Bahasa Inggris", en: "English" }, url: "https://www.nhs.uk/every-mind-matters/mental-wellbeing-tips/" },
  { publisher: "NHS", title: { id: "Panduan tentang tidur", en: "A guide to sleep" }, topic: "sleep", language: { id: "Bahasa Inggris", en: "English" }, url: "https://www.nhs.uk/every-mind-matters/mental-wellbeing-tips/how-to-fall-asleep-faster-and-sleep-better/" },
];

function reviewDate(iso: string, lang: string): string {
  const match = iso.match(/^(\d{4})-(\d{2})-(\d{2})$/);
  if (!match) return iso;
  const date = new Date(Date.UTC(Number(match[1]), Number(match[2]) - 1, Number(match[3])));
  if (Number.isNaN(date.getTime())) return iso;
  return new Intl.DateTimeFormat(lang === "en" ? "en-GB" : "id-ID", {
    day: "numeric",
    month: "long",
    year: "numeric",
    timeZone: "UTC",
  }).format(date);
}

export function ReadingPage() {
  const w = useWords();
  const { lang } = useI18n();
  const [items, setItems] = useState<ReadingListItem[] | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [query, setQuery] = useState("");

  useEffect(() => {
    let active = true;
    api<{ items: ReadingListItem[] }>("/reading")
      .then((d) => {
        if (active) setItems(d.items ?? []);
      })
      .catch((e) => {
        if (active) {
          setError(e);
          setItems([]);
        }
      });
    return () => {
      active = false;
    };
  }, []);

  const filtered = sources.filter((s) =>
    (s.title.id + s.title.en + s.publisher + s.topic)
      .toLowerCase()
      .includes(query.toLowerCase()),
  );

  return (
    <div className="c-reading">
      <PageHeader
        title={w("Jeda untuk membaca", "A pause to read")}
        lead={w(
          "Latihan kecil Sela dan sumber resmi untuk kamu jelajahi dengan ritmemu sendiri.",
          "Sela's small exercises and official sources to explore at your own pace.",
        )}
      />

      <section className="s-section" aria-labelledby="reading-library-title">
        <h2 id="reading-library-title">{w("Latihan kecil dari Sela", "Small exercises from Sela")}</h2>
        <p className="s-hint">
          {w(
            "Hanya bacaan yang sudah disetujui editor manusia yang muncul di sini. Setiap teks dan lisensinya diperiksa sebelum ditampilkan.",
            "Only reading approved by a human editor appears here. Each text and its licence are checked before display.",
          )}
        </p>
        {items === null && !error && <Loading label={w("Memuat bacaan…", "Loading reading…")} />}
        <ErrorNotice error={error} />
        {items?.length === 0 && !error && (
          <Notice tone="info" title={w("Belum ada bacaan yang bisa dibuka", "No reading can be opened yet")}>
            <p>
              {w(
                "Kartu latihan Sela masih menunggu tinjauan editor manusia. Setiap teks, sumber, dan lisensinya harus disetujui dan diverifikasi sebelum bisa tampil di sini.",
                "Sela exercise cards still await human editorial review. Every text, source and licence must be approved and verified before it can appear here.",
              )}
            </p>
            <p>
              {w(
                "Tautan resmi di bawah dapat kamu buka langsung di situs penerbit. Sela tidak mengirim cerita atau catatanmu ke sana.",
                "You can open the official links below directly on the publisher's site. Sela does not send your stories or notes there.",
              )}
            </p>
          </Notice>
        )}
        {items && items.length > 0 && (
          <div className="c-reading-list">
            {items.map((item) => (
              <article className="c-reading-source" key={item.id}>
                <div>
                  <h3>
                    <Link to={`/reading/${encodeURIComponent(item.id)}`}>{item.title[lang]}</Link>
                  </h3>
                  <p>{item.summary[lang]}</p>
                  <p className="s-muted">
                    {item.sources.map((s) => s.publisher).join(" · ")}
                    {item.sources.length > 0 ? " · " : ""}
                    {w("baca", "read")} {item.readingTimeMinutes[lang]} {w("menit", "min")} ·{" "}
                    {w("Ditinjau", "Reviewed")} {reviewDate(item.reviewedOn, lang)}
                  </p>
                </div>
                <Icon name="arrow-right" />
              </article>
            ))}
          </div>
        )}
      </section>

      <section className="s-section" aria-labelledby="official-sources-title">
        <h2 id="official-sources-title">{w("Sumber resmi WHO dan NHS", "Official WHO and NHS sources")}</h2>
        <div className="c-reading-intro">
          <p>
            {w(
              "Tautan ini membuka situs penerbit. Cerita dan catatan Sela tidak ikut dikirim. Isi tersedia dalam bahasa yang disediakan penerbit.",
              "These links open the publisher's website. Your Sela stories and notes are not sent with them. Content is available in the publisher's languages.",
            )}
          </p>
          <TextField
            type="search"
            label={w("Cari judul atau penerbit", "Search titles or publishers")}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        </div>
        <div className="c-reading-list">
          {filtered.map((s) => (
            <article className="c-reading-source" key={s.url}>
              <div>
                <h3>
                  <a href={s.url} target="_blank" rel="noopener noreferrer" referrerPolicy="no-referrer">
                    {w(s.title.id, s.title.en)}
                    <span className="s-sr-only">
                      {w(" (membuka situs penerbit di tab baru)", " (opens publisher website in a new tab)")}
                    </span>
                  </a>
                </h3>
                <p className="s-muted">
                  {s.publisher} · {w(s.language.id, s.language.en)} · {w("Situs penerbit", "Publisher website")}
                </p>
              </div>
              <Icon name="arrow-right" />
            </article>
          ))}
        </div>
        {filtered.length === 0 && (
          <p role="status">{w("Tidak ada judul yang cocok. Coba kata lain.", "No matching titles. Try another word.")}</p>
        )}
      </section>

      <section className="c-editorial-status">
        <h2>{w("Tentang latihan kecil Sela", "About Sela's small exercises")}</h2>
        <p>
          {w(
            "Kartu latihan dari Dengar.in sudah diinventarisasi, tetapi teks dan lisensinya masih menunggu tinjauan editor manusia. Sela hanya menampilkan bacaan yang lolos pemeriksaan itu; katalog di atas menautkan langsung ke penerbit dan bukan kartu latihan Sela yang sudah disetujui.",
            "Dengar.in exercise cards have been inventoried, but their text and licences still await human editorial review. Sela shows only reading that passes that check; the catalogue above links directly to publishers and is not an approved Sela exercise card.",
          )}
        </p>
        <LinkButton to="/notes/write" variant="secondary">
          {w("Tulis yang ingin kamu ingat", "Write what you want to remember")}
        </LinkButton>
      </section>
    </div>
  );
}

export function ReadingItemPage({ slug }: { slug: string }) {
  const w = useWords();
  const { lang } = useI18n();
  const [item, setItem] = useState<ReadingItem | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [missing, setMissing] = useState(false);

  useEffect(() => {
    let active = true;
    setItem(null);
    setError(null);
    setMissing(false);
    api<ReadingItem>(`/reading/${encodeURIComponent(slug)}`)
      .then((d) => {
        if (active) setItem(d);
      })
      .catch((e) => {
        if (!active) return;
        if (e instanceof V3Error && e.code === "not_found") setMissing(true);
        else setError(e);
      });
    return () => {
      active = false;
    };
  }, [slug]);

  const back = { to: "/reading", label: w("Bacaan & latihan kecil", "Reading & small exercises") };

  if (missing) {
    return (
      <div className="s-narrow">
        <PageHeader
          back={back}
          title={w("Bacaan tidak tersedia", "Reading not available")}
          lead={w(
            "Bacaan ini belum lolos tinjauan editor atau tidak ada di pustaka.",
            "This reading has not passed editorial review or is not in the library.",
          )}
        />
        <Notice tone="info" title={w("Belum bisa dibuka", "Not available yet")}>
          <p>
            {w(
              "Sela hanya menampilkan bacaan yang sudah disetujui editor manusia. Kembali ke daftar untuk melihat bacaan lain atau sumber resmi.",
              "Sela shows only reading approved by a human editor. Go back to the list for other reading or the official sources.",
            )}
          </p>
        </Notice>
        <LinkButton to="/reading" variant="secondary">
          {w("Kembali ke bacaan", "Back to reading")}
        </LinkButton>
      </div>
    );
  }

  if (!item) {
    return (
      <div className="s-narrow">
        <PageHeader back={back} title={w("Memuat bacaan…", "Loading reading…")} />
        <ErrorNotice error={error} onRetry={() => window.location.reload()} retryLabel={w("Coba lagi", "Try again")} />
        {!error && <Loading label={w("Membuka bacaan…", "Opening reading…")} />}
      </div>
    );
  }

  const relation =
    ({
      adapted: w("Diadaptasi dari sumber", "Adapted from a source"),
      paraphrased: w("Diparafrasekan dari sumber", "Paraphrased from a source"),
      structure_only: w("Mengikuti struktur sumber", "Follows a source's structure"),
      sela_original: w("Teks orisinal Sela", "Sela's own text"),
    } as Record<string, string>)[item.source.relation] ?? item.source.relation;

  const reviewLabel =
    item.review.status === "human_approved"
      ? w("Disetujui setelah tinjauan editor manusia", "Approved after human editorial review")
      : item.review.status;

  return (
    <article className="c-reading c-reading-detail s-narrow">
      <PageHeader
        back={back}
        title={item.title[lang]}
        lead={w(
          `${item.readingTimeMinutes[lang]} menit baca · ${reviewDate(item.review.reviewedOn, lang)}`,
          `${item.readingTimeMinutes[lang]} min read · ${reviewDate(item.review.reviewedOn, lang)}`,
        )}
      />
      <p className="s-lead">{item.summary[lang]}</p>

      {item.steps && (
        <section className="c-reading-section" aria-labelledby="reading-steps-title">
          <h2 id="reading-steps-title">{w("Langkah kecil", "Small steps")}</h2>
          <ol>
            {item.steps[lang].map((step) => (
              <li key={step}>{step}</li>
            ))}
          </ol>
        </section>
      )}

      {item.sections?.map((section, index) => (
        <section className="c-reading-section" key={`${section.heading.en}-${index}`}>
          <h2>{section.heading[lang]}</h2>
          {section.paragraphs.map((paragraph) => (
            <p key={paragraph.en}>{paragraph[lang]}</p>
          ))}
        </section>
      ))}

      {item.why && (
        <section className="c-reading-section" aria-labelledby="reading-why-title">
          <h2 id="reading-why-title">{w("Kenapa ini membantu", "Why this helps")}</h2>
          <p>{item.why[lang]}</p>
        </section>
      )}

      {item.seekHelpWhen && (
        <section className="c-reading-section" aria-labelledby="reading-help-title">
          <h2 id="reading-help-title">{w("Kapan mencari bantuan", "When to seek help")}</h2>
          <p>{item.seekHelpWhen[lang]}</p>
        </section>
      )}

      {item.cautions && (
        <section className="c-reading-section" aria-labelledby="reading-cautions-title">
          <h2 id="reading-cautions-title">{w("Perlu diingat", "Keep in mind")}</h2>
          <p>{item.cautions[lang]}</p>
        </section>
      )}

      <section className="c-reading-meta" aria-labelledby="reading-review-title">
        <h2 id="reading-review-title">{w("Tinjauan dan batas", "Review and limits")}</h2>
        <p>
          {reviewLabel} · {reviewDate(item.review.reviewedOn, lang)}
        </p>
        <p className="s-hint">
          {w(
            "Sumber dan klaim diperiksa saat penulisan. Bacaan ini bukan nasihat medis, diagnosis, atau terapi, dan bukan pengganti bantuan profesional.",
            "Sources and claims were checked at authoring time. This reading is not medical advice, diagnosis or therapy, and it is not a replacement for professional help.",
          )}
        </p>
        <p className="s-hint">
          {w("Kalau kamu butuh bantuan sekarang, buka ", "If you need help now, open ")}
          <Link to="/help">{w("Bantuan sekarang", "Help now")}</Link>.
        </p>
      </section>

      <section className="c-reading-meta" aria-labelledby="reading-source-title">
        <h2 id="reading-source-title">{w("Sumber dan lisensi", "Source and licence")}</h2>
        <p>{item.attribution[lang]}</p>
        {item.adaptationNotice && <p className="s-hint">{item.adaptationNotice[lang]}</p>}
        <p className="s-hint">
          {relation} · {item.source.locator}
        </p>
        {item.source.sources.length > 0 && (
          <ul>
            {item.source.sources.map((s) => (
              <li key={s.id}>
                <a href={s.url} target="_blank" rel="noopener noreferrer" referrerPolicy="no-referrer">
                  {s.title}
                </a>{" "}
                <span className="s-muted">· {s.publisher}</span>
              </li>
            ))}
          </ul>
        )}
        <p className="s-hint">
          {w("Lisensi teks", "Text licence")}: {item.licence.itemTextLicence}
        </p>
        <p className="s-hint">
          {w("Lisensi sumber", "Source licence")}: {item.licence.sourceLicence}
        </p>
        {item.licence.note && <p className="s-hint">{item.licence.note}</p>}
        {item.adaptationNote && (
          <p className="s-hint">
            {w("Catatan adaptasi", "Adaptation note")}: {item.adaptationNote}
          </p>
        )}
        {item.evidenceNote && (
          <p className="s-hint">
            {w("Catatan bukti", "Evidence note")}: {item.evidenceNote}
          </p>
        )}
      </section>
    </article>
  );
}

export function RecipientUnavailablePage() {
  const w = useWords();
  return (
    <div className="s-narrow">
      <PageHeader
        title={w("Halaman penerima", "Recipient page")}
        lead={w(
          "Halaman ini untuk orang yang diundang membaca ringkasan. Versi Sela v3 belum bisa membuat atau membuka undangan.",
          "This page is for someone invited to read a summary. This build of Sela v3 can't make or open an invitation yet.",
        )}
      />
      <Notice tone="info" title={w("Belum tersedia", "Not available yet")}>
        <p>
          {w(
            "Tautan dan kode akses belum bisa dibuat karena layanan berbagi belum tersambung. Halaman penerima versi lama tetap tersedia hanya di demo sintetis.",
            "Links and access codes can't be made because the sharing service isn't connected. The earlier recipient page remains available only in the synthetic demo.",
          )}
        </p>
      </Notice>
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
