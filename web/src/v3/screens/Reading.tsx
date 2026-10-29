import { useState } from "react";
import { Icon } from "../icons";
import { useWords } from "../shared";
import { LinkButton, Notice, PageHeader, TextField } from "../ui";

const sources = [
  { publisher: "WHO", title: { id: "Doing What Matters in Times of Stress", en: "Doing What Matters in Times of Stress" }, topic: "stress", language: { id: "Panduan dengan pilihan bahasa", en: "Guide with language options" }, url: "https://www.who.int/publications/i/item/9789240003927" },
  { publisher: "WHO", title: { id: "Tanya jawab tentang stres", en: "Questions and answers about stress" }, topic: "stress", language: { id: "Bahasa Inggris", en: "English" }, url: "https://www.who.int/news-room/questions-and-answers/item/stress" },
  { publisher: "NHS", title: { id: "Mengenal kesejahteraan mental", en: "Mental wellbeing tips" }, topic: "wellbeing", language: { id: "Bahasa Inggris", en: "English" }, url: "https://www.nhs.uk/every-mind-matters/mental-wellbeing-tips/" },
  { publisher: "NHS", title: { id: "Panduan tentang tidur", en: "A guide to sleep" }, topic: "sleep", language: { id: "Bahasa Inggris", en: "English" }, url: "https://www.nhs.uk/every-mind-matters/mental-wellbeing-tips/how-to-fall-asleep-faster-and-sleep-better/" },
];
export function ReadingPage() {
  const w = useWords(); const [query, setQuery] = useState("");
  const filtered = sources.filter(s => (s.title.id + s.title.en + s.publisher + s.topic).toLowerCase().includes(query.toLowerCase()));
  return <div className="c-reading"><PageHeader title={w("Jeda untuk membaca", "A pause to read")} lead={w("Sumber resmi untuk kamu jelajahi dengan ritmemu sendiri.", "Official sources to explore at your own pace.")} />
    <div className="c-reading-intro"><p>{w("Tautan ini membuka situs penerbit. Cerita dan catatan Sela tidak ikut dikirim. Isi tersedia dalam bahasa yang disediakan penerbit.", "These links open the publisher's website. Your Sela stories and notes are not sent with them. Content is available in the publisher's languages.")}</p><TextField type="search" label={w("Cari judul atau penerbit", "Search titles or publishers")} value={query} onChange={e => setQuery(e.target.value)} /></div>
    <div className="c-reading-list">{filtered.map(s => <article className="c-reading-source" key={s.url}><div><h2><a href={s.url} target="_blank" rel="noopener noreferrer" referrerPolicy="no-referrer">{w(s.title.id, s.title.en)}<span className="s-sr-only">{w(" (membuka situs penerbit di tab baru)", " (opens publisher website in a new tab)")}</span></a></h2><p className="s-muted">{s.publisher} · {w(s.language.id, s.language.en)} · {w("Situs penerbit", "Publisher website")}</p></div><Icon name="arrow-right" /></article>)}</div>
    {filtered.length === 0 && <p role="status">{w("Tidak ada judul yang cocok. Coba kata lain.", "No matching titles. Try another word.")}</p>}
    <section className="c-editorial-status"><h2>{w("Tentang latihan kecil Sela", "About Sela's small exercises")}</h2><p>{w("Kartu latihan dari Dengar.in sudah diinventarisasi, tetapi teks dan lisensinya masih menunggu tinjauan editor manusia. Katalog di atas menautkan langsung ke penerbit; bukan kartu latihan Sela yang sudah disetujui.", "Dengar.in exercise cards have been inventoried, but their text and licences still await human editorial review. The catalogue above links directly to publishers; these are not approved Sela exercise cards.")}</p><LinkButton to="/notes/write" variant="secondary">{w("Tulis yang ingin kamu ingat", "Write what you want to remember")}</LinkButton></section>
  </div>;
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
