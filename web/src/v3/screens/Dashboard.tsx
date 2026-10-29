import { Link } from "../../lib/router";
import { Icon, type IconName } from "../icons";
import { useWords } from "../shared";
import { LinkButton } from "../ui";

// An information portal, separate from the personal Home workspace.
// No personal data is loaded and no provider is called on this page.
export function DashboardPage({ signedIn = false }: { signedIn?: boolean }) {
  const w = useWords();
  const groups: { title: string; items: { to: string; icon: IconName; title: string; body: string }[] }[] = [
    { title: w("Untuk yang kamu rasakan", "For what you feel"), items: [
      { to: "/talk", icon: "talk", title: w("Ruang cerita", "Talk"), body: w("Mulai dari satu kalimat. Pilih sendiri apakah kamu ingin balasan AI.", "Start with a sentence. Choose whether you want AI replies.") },
      { to: "/notes/write", icon: "pencil", title: w("Jurnal pribadi", "Personal journal"), body: w("Tulis pengalaman, pikiran, atau hal kecil yang ingin kamu simpan.", "Write about an experience, a thought, or something small worth keeping.") },
      { to: "/notes/check-in", icon: "interpretation", title: w("Cek perasaan", "Check in"), body: w("Catat suasana hati dan energimu tanpa penilaian atau skor.", "Note your mood and energy without judgement or scores.") },
    ] },
    { title: w("Untuk melanjutkan ceritamu", "For carrying your story forward"), items: [
      { to: "/notes/weekly", icon: "calendar", title: w("Refleksi mingguan", "Weekly reflection"), body: w("Lihat kembali tulisan dan cek perasaan dalam satu minggu.", "Look back at your writing and check-ins over a week.") },
      { to: "/notes/memory", icon: "bookmark", title: w("Memori pilihanmu", "Memory you choose"), body: w("Simpan konteks secara manual. Perbaiki atau hapus kapan saja.", "Keep context manually. Edit or delete it whenever you choose.") },
      { to: "/support/summaries", icon: "notes", title: w("Ringkasan & persiapan", "Summaries & preparation"), body: w("Pilih catatan, susun ringkasan, dan catat rencana tindak lanjut.", "Choose notes, build a summary, and record your follow-up plans.") },
    ] },
  ];
  return <div className="s-info-dashboard">
    <header className="s-info-heading">
      <div><h1 id="page-title">{w("Dashboard Sela", "Sela dashboard")}</h1><p>{w("Kenali ruangnya, lihat yang tersedia, lalu lanjutkan dengan caramu.", "Get to know the space, see what is available, then continue your way.")}</p></div>
      <LinkButton to={signedIn ? "/home" : "/start/age"} variant="primary" icon="arrow-right">{w("Masuk ke Beranda", "Continue to Home")}</LinkButton>
    </header>

    <div className="s-info-facts" aria-label={w("Sekilas tentang Sela", "Sela at a glance")}>
      <span><Icon name="user" />{w("Bisa mulai sebagai tamu", "Start as a guest")}</span>
      <span><Icon name="shield" />{w("Izin simpan & AI terpisah", "Separate storage & AI choices")}</span>
      <span><Icon name="bookmark" />{w("Memori tidak otomatis", "Memory is never automatic")}</span>
      <span><Icon name="settings" />{w("Tampilan bisa disesuaikan", "Adjustable display")}</span>
    </div>

    <div className="s-info-layout">
      <div className="s-info-main">
        <section className="s-info-welcome" aria-labelledby="dashboard-intro">
          <div><h2 id="dashboard-intro">{w("Ada lebih dari satu cara untuk mengambil jeda.", "There is more than one way to pause.")}</h2><p>{w("Sela menghubungkan ruang bercerita, jurnal, refleksi, dan persiapan bantuan. Dashboard ini mengenalkan semuanya; ruang pribadimu ada di Beranda.", "Sela brings together talking, journaling, reflection, and preparation for support. This dashboard introduces them; your personal space starts at Home.")}</p></div>
          <Link to="/about">{w("Tentang Sela", "About Sela")}<Icon name="arrow-right" /></Link>
        </section>

        <section className="s-info-features" aria-labelledby="dashboard-features">
          <div className="s-info-section-title"><h2 id="dashboard-features">{w("Jelajahi fitur", "Explore the features")}</h2><span>{w("Pilih sesuai kebutuhanmu", "Choose what you need")}</span></div>
          <div className="s-info-feature-groups">{groups.map(group => <section key={group.title}>
            <h3>{group.title}</h3><ul>{group.items.map(item => <li key={item.to}><Link to={signedIn ? item.to : "/start/age"}>
              <Icon name={item.icon} /><span><strong>{item.title}</strong><small>{item.body}</small></span><Icon name="arrow-right" />
            </Link></li>)}</ul>
          </section>)}</div>
          {!signedIn && <p className="s-info-footnote">{w("Fitur pribadi dibuka setelah kamu memilih mode tamu atau akun dan izin yang sesuai.", "Personal features open after you choose guest or account mode and the relevant permissions.")}</p>}
        </section>

        <section className="s-info-reading" aria-labelledby="dashboard-reading">
          <div className="s-info-section-title"><h2 id="dashboard-reading">{w("Bacaan untuk jeda", "Reading for a pause")}</h2><Link to="/reading">{w("Lihat bacaan", "Browse reading")}</Link></div>
          <div className="s-info-reading-topics">
            <article><h3>{w("Mengenal stres", "Understanding stress")}</h3><span>{"WHO"}</span><p>{w("Tanya jawab dan panduan dari penerbit resmi.", "Questions, answers, and guides from the official publisher.")}</p></article>
            <article><h3>{w("Kesejahteraan sehari-hari", "Everyday wellbeing")}</h3><span>{"NHS"}</span><p>{w("Sumber untuk menjelajahi kesejahteraan mental.", "Resources for exploring mental wellbeing.")}</p></article>
            <article><h3>{w("Ruang untuk istirahat", "Room for rest")}</h3><span>{"NHS"}</span><p>{w("Temukan panduan tidur langsung di situs penerbit.", "Find sleep guidance on the publisher's own website.")}</p></article>
          </div>
        </section>
      </div>

      <aside className="s-info-aside">
        <section className="s-info-path" aria-labelledby="dashboard-path"><h2 id="dashboard-path">{w("Dari sini ke Beranda", "From here to Home")}</h2>
          <ol><li><strong>{w("Kenali yang tersedia", "Explore what is here")}</strong><p>{w("Baca fitur dan batas layanan Sela.", "Read about Sela's features and limits.")}</p></li><li><strong>{w("Tentukan pilihanmu", "Make your choices")}</strong><p>{w("Mulai sebagai tamu atau akun. Izin simpan dan AI kamu pilih terpisah.", "Choose guest or account mode, then choose storage and AI permissions separately.")}</p></li><li><strong>{w("Buka ruang pribadimu", "Open your personal space")}</strong><p>{w("Di Beranda, mulai menulis atau lanjutkan yang kamu simpan.", "At Home, start writing or return to what you saved.")}</p></li></ol>
          <LinkButton to={signedIn ? "/home" : "/start/age"} variant="secondary">{w("Lanjut ke Beranda", "Go to Home")}</LinkButton>
        </section>
        <section className="s-info-control" aria-labelledby="dashboard-control"><h2 id="dashboard-control">{w("Tetap dalam kendalimu", "You stay in control")}</h2><dl>
          <div><dt>{w("Percakapan & jurnal", "Conversations & journal")}</dt><dd>{w("Penyimpanan mengikuti izinmu. Pilihan tidak menyimpan tetap tersedia untuk percakapan.", "Storage follows your permissions. Conversations also offer a do-not-save option.")}</dd></div>
          <div><dt>{w("Ringkasan", "Summaries")}</dt><dd>{w("Ditulis dan diperiksa olehmu. Unduh setelah menyetujui versinya.", "Written and reviewed by you. Download after approving its version.")}</dd></div>
          <div><dt>{w("Berbagi ke penerima", "Recipient sharing")}</dt><dd>{w("Tautan berbagi dan kode akses belum tersedia.", "Share links and recipient access codes are not available yet.")}</dd></div>
        </dl><Link to="/policies">{w("Baca kebijakan", "Read the policies")}</Link></section>
        <section className="s-info-help" aria-labelledby="dashboard-help"><Icon name="help" /><h2 id="dashboard-help">{w("Butuh bantuan manusia?", "Need human support?")}</h2><p>{w("Lihat informasi layanan bantuan. Sela bukan layanan darurat dan tidak memberi diagnosis.", "Explore support service information. Sela is not an emergency service and gives no diagnosis.")}</p><Link to="/help">{w("Lihat layanan bantuan", "View support services")}<Icon name="arrow-right" /></Link></section>
      </aside>
    </div>
    <nav className="s-info-bottom" aria-label={w("Informasi lainnya", "More information")}><Link to="/settings/display">{w("Atur tampilan", "Adjust display")}</Link><Link to="/about">{w("Tentang & batas Sela", "About Sela and its limits")}</Link>{!signedIn && <Link to="/signin">{w("Sudah punya akun? Masuk", "Already have an account? Sign in")}</Link>}</nav>
  </div>;
}
