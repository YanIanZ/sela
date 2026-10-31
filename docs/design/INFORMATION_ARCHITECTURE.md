# Sela v3 information architecture

Status: **design decision** by the design lead, 2026-10-26, revised after review rounds 1 and 2 ([DESIGN_DIRECTION.md §11](DESIGN_DIRECTION.md#11-revision-log)). **Implemented in `web/src/v3/` on 2026-10-28 for the routes whose APIs exist; blocked surfaces are stated in place.** Labels follow [DESIGN_DIRECTION.md](DESIGN_DIRECTION.md) (user-confirmed, PM decision, design decision, externally sourced, planned, pending verification). Screen details are in [SCREENS.md](SCREENS.md).

## 1. The model in one picture

```
                         ┌─────────────────────────────────────────────┐
  On every screen ──────▶│  Butuh bantuan sekarang   ·   Keluar cepat   │  (same place, same order)
                         └─────────────────────────────────────────────┘

  Public (no session)                Signed in (guest or account)                    Separate
  ───────────────────                ─────────────────────────────                   ────────
  Pengenalan  / (+ bahasa, teks)     ┌──────────┬──────────┬────────────┬───────────┐   Penerima  /r
  Mulai       /start/*               │ Beranda  │ Cerita   │ Catatanku  │ Dukungan  │   (recipient:
  Masuk       /signin                │ Home     │ Talk     │ My notes   │ Support   │    own shell,
  Pemulihan   /recover               │ /home    │ /talk    │ /notes     │ /support  │    own cookie,
  Bantuan     /help        ◀─────────┤          │          │            │           │    one share)
  Bacaan      /reading     ◀─────────┤          │          │            │           │
  Tampilan    /settings/display ◀────┤          │          │            │           │
  Tentang     /about                 └──────────┴──────────┴────────────┴───────────┘
  Kebijakan   /policies              Menu: Privasi & data · Tampilan & aksesibilitas · Akun · Bahasa · Keluar dari akun
```

The model is four **places**, two **safety controls** on every screen and one **personal menu**. This is a design decision.

- **Beranda** (Home): a calm start.
  - Its job is to offer the next step: talk, write, or check in.
  - It shows what is active, such as shares and an unfinished conversation.
  - It is not a dashboard. It shows no counts, scores or streaks (D-037).
- **Cerita** (Talk): the listening room, *Ruang cerita* (D-035).
  - It lists the saved conversations.
  - It starts a new conversation and opens existing ones.
- **Catatanku** (My notes): everything that is kept, *Catatan pribadiku* (D-039).
  - One timeline holds every kind of entry.
  - Writing a note and checking in start here.
  - *Yang Sela ingat* ("what Sela remembers") is a section of this place.
- **Dukungan** (Support): help that reaches beyond the conversation.
  - *Ringkasan untuk konsultasi*: preparing, sharing and following up a consultation.
  - *Bacaan & latihan kecil*: reading and small exercises.
  - *Layanan bantuan*: the verified support directory.
  - Seeking a professional is one option among others, never the default path (brief §1).
- **The safety controls:**
  - *Butuh bantuan sekarang* ("need help now") leads to the immediate help page.
  - *Keluar cepat* ("quick exit") leaves the app at once.
  - Both work without a session.
- **The personal menu** holds *Privasi & data*, *Tampilan & aksesibilitas*, *Akun*, *Bahasa*, *Tentang Sela*, *Kebijakan* and, for accounts, *Keluar dari akun*. When the bottom bar has no room (tier C, below), the four places are the first group in the menu.

Why four places: an earlier hub model gave eight equal destinations. The v1 header hid or shortened labels across four width tiers to fit more than a dozen destinations, and that is the failure mode this model avoids ([DESIGN_DIRECTION.md §2](DESIGN_DIRECTION.md#2-directions-explored)). The four place labels are single words that cannot wrap, so the bottom bar changes tier by available space instead of shortening them (§3.2): four across on 360px phones at 100% text, a 2×2 grid on 320px phones and at medium text sizes, and the places inside the menu at the largest sizes.

## 2. Names (ID and EN)

The brief allows renaming (§4). PM decision D-035 fixes "Ruang cerita" / "Talk". D-039 fixes "Catatan pribadiku" and "Ringkasan untuk konsultasi".

| Brief surface (§4) | v3 name, Indonesian | v3 name, English | Where it lives | Why this name |
|---|---|---|---|---|
| Pengenalan dan onboarding | Pengenalan · Mulai | Introduction · Get started | `/`, `/start/*` | Plain verbs |
| Beranda personal | Beranda | Home | Place 1 | Standard; no "dashboard" connotation |
| Ruang percakapan | Cerita (screen title *Ruang cerita*) | Talk | Place 2 | D-035. "Cerita" is the everyday word for telling someone what's going on, without the clinical feel of "konseling" |
| Jurnal serta refleksi | Catatan (write: *Tulis catatan*) | Note (write: *Write a note*) | Catatanku | "Jurnal" sounds like homework to many readers, and "catatan" matches *Catatan pribadiku*. "Catatan" means a journal entry only; anything kept in general is "entri" or "yang tersimpan" ([CONTENT_STYLE.md §4](CONTENT_STYLE.md#4-product-glossary)) |
| Check-in | Cek perasaan | Check-in | Catatanku, Home | Says what it is in Indonesian; "check-in" is not common Indonesian |
| Timeline | Linimasa | Timeline | Catatanku (default section) | Standard Indonesian term |
| Memory | Yang Sela ingat | What Sela remembers | Catatanku section | Describes it honestly; "memori" alone is vague |
| Ringkasan konsultasi | Ringkasan untuk konsultasi | Summary for a consultation | Dukungan | D-039 |
| Persiapan dan tindak lanjut | Persiapan · Setelah konsultasi | Preparation · After the consultation | Inside each summary | Attached to the summary they serve |
| Bacaan dan dukungan | Bacaan & latihan kecil · Layanan bantuan | Reading & small exercises · Support services | Dukungan; `/reading` and `/help` are public | D-038 merges the daily missions into "latihan kecil" |
| Pengaturan privasi serta data | Privasi & data | Privacy & data | Menu | Says both halves |
| (display) | Tampilan & aksesibilitas | Display & accessibility | Menu; public too | Needed before sign-in, for example large text during onboarding |
| Pengalaman penerima profesional | Halaman penerima | Recipient page | `/r` | Recipients are invited by the user and unverified (D-042). "Profesional" is not claimed |
| (always) | Butuh bantuan sekarang (never shortened; wraps) | Need help now | Top bar | D-036 |
| (always) | Keluar cepat (never shortened; wraps) | Quick exit | Top bar | v1 Quick Exit. "Keluar" alone is never a label, because signing out is a different action |
| (accounts) | Keluar dari akun | Sign out | Menu, Akun | Never shortened, so it is never confused with Quick exit (3.2.4) |

Document titles use "{screen title} – Sela", for example "Ruang cerita – Sela" or "Catatan pribadiku – Sela", in the active language (WCAG 2.4.2).

## 3. Navigation model

### 3.1 Landmarks and DOM order (identical on every layout)

1. Skip links. "Lewati ke konten" is always present. The room adds "Lewati ke kotak tulis".
2. `<header>` (banner):
   - the mark and "Sela" (a link to Home, or to Introduction when signed out), from the medium layout up;
   - **Butuh bantuan sekarang** (a link to `/help`, in the calm Help-quiet form);
   - **Keluar cepat**;
   - **Menu** (compact and medium layouts).
3. `<nav aria-label="Navigasi utama">`: the four places. It is a bottom bar on compact and medium layouts and a sidebar on wide ones. It is absent when signed out, and in bottom-bar tier C (the places are then in the menu).
4. `<section aria-label="Pemberitahuan">`: banners, including the crisis follow-up banner on Home and in the room.
5. `<main id="main" tabindex="-1" aria-labelledby="page-title">`, named by its h1, so moving focus to it on a route change announces the page name:
   - the page header (back link, `h1#page-title`, lead);
   - then the content.
6. `<footer>`, at the end of the page content:
   - "Sela bukan layanan darurat dan tidak memberi diagnosis." / "Sela is not an emergency service and gives no diagnosis.";
   - links: Tentang Sela · Kebijakan · Butuh bantuan sekarang;
   - the Dengar.in attribution line (kept from v2).

   The room shows no footer because it is a focused mode. The same note appears in the room's empty state and in its Detail panel.
7. The announcement regions `#sr-status`, `#sr-reply` and `#sr-alert`: the **last children of `<body>`**, visually hidden, empty from the first paint, never re-created, and cleared on every route change and on the person's next input, so stale reply or status text never sits in the reading order ([DESIGN_DIRECTION.md §6.1](DESIGN_DIRECTION.md#61-app-shell)).

The help control and Quick exit are always in the header, in this order, before the navigation. That satisfies WCAG 3.2.6 Consistent Help and 3.2.3 Consistent Navigation.

On compact layouts the bottom bar sits visually at the bottom while the DOM puts it before `<main>`. This is deliberate: screen-reader order is then the same on every device, and the skip link bypasses the bar.

### 3.2 Compact and medium layouts (shell narrower than 64em)

Every bar wraps **by available space**, never by the text-size setting, so the same rules hold at 320px, at 150% text and under a user's 1.4.12 spacing stylesheet. Width estimates and the exact rules are in [DESIGN_DIRECTION.md §6.1](DESIGN_DIRECTION.md#61-app-shell); checks S-1, S-3 and S-5 confirm them.

- **Top bar:** help, Quick exit and Menu, with full labels that wrap inside their controls.
  - One row needs about 530px of viewport at 100% text, so tablets and landscape phones get one row.
  - **Phones in portrait (320–480px): two rows.** "Butuh bantuan sekarang" fills the first row; "Keluar cepat" and Menu share the second. About 104px tall.
  - 150% text on a 320px screen: three rows.
  - It grows with `min-height`, never a fixed height, and is sticky only when the viewport is tall enough (§3.5).
- **Bottom bar:** the four places, in one of three tiers chosen by available space (a container query in em, plus a script that drops a tier when a label overflows, for example under a spacing stylesheet):
  - **A:** four across, icon above label (360px phones and wider at 100% text);
  - **B:** a 2×2 grid, icon beside label (320px phones at 100–115%; 360px phones at 115–130%);
  - **C:** no bottom bar; the places are the first group in the Menu sheet, in the same order (150% text on any phone).
  - It is hidden while a **text-entry** field has focus (chips, check-in tiles and radios do not count), and in the room, a focused mode in which the composer takes the bottom.
- **Menu sheet:**
  - a full-height `<dialog>`; its modal background makes the header inert, so the sheet **starts with "Butuh bantuan sekarang" and "Keluar cepat"**, in the header's order and look, then "Tutup" where the Menu button was, then the heading "Menu" (3.2.6);
  - holds everything in the sidebar plus "Keluar dari akun" (for accounts), and the four places first in tier C.
  - The room's Detail sheet follows the same pattern.
- **Back:**
  - every screen below a place shows a back link to its parent ("← Catatanku");
  - the browser Back button always works, because every step is a URL, and it returns to the same scroll position and loaded list pages ([DESIGN_DIRECTION.md §8](DESIGN_DIRECTION.md#8-implementation-notes-for-the-frontend-planned-owned-by-the-frontend-engineer)).

### 3.3 Wide layout (shell 64em or wider)

- **Header strip across the whole window**, above both columns: the mark, "Butuh bantuan sekarang" and "Keluar cepat", starting at the left edge in that order.
- **Sidebar (16rem)**, below the header strip on the left:
  - the four places;
  - a "Pengaturan" group: Privasi & data, Tampilan & aksesibilitas, Akun;
  - the language switch;
  - Tentang Sela · Kebijakan.
- **Main column**, to the right of the sidebar: the notices at its top, then `<main>`.
- The visual order (header strip left to right, sidebar, notices, content) is the DOM order of §3.1, so focus never jumps from the top right to the sidebar and back to the top left (2.4.3, 3.2.6).
- **The room:** the transcript column stays at `--measure`. From 75em, a right-hand details panel shows intent, storage, memory, "Sela menangkap…" items and "Akhiri percakapan".

### 3.4 Signed-out shell

- **Header:** the mark, help, exit and Menu.
- **Menu contents:**
  - Bahasa;
  - Tampilan & aksesibilitas;
  - Masuk;
  - Mulai;
  - Butuh bantuan sekarang;
  - Bacaan & latihan kecil;
  - Tentang Sela;
  - Kebijakan.
- **No bottom bar.**
- **Onboarding** shows "Langkah 1 dari 3" (recomputed when a path adds steps) and a back link on every step. Language and text size are set on the Introduction itself.

### 3.5 Large text, zoom and short viewports

- **Layout steps:** the shell's container queries are in em, so the layout steps down as text grows. At Sela's 150% text size, a 1366px laptop gets the medium layout.
- **Short viewport mode:** a viewport less than 30 root em tall (480px at 100% text, 720px at 150%), measured by a script **without the soft keyboard** (the measurement is frozen while a text field has focus, so typing never changes the layout).
  - The bottom bar and the composer become part of the page flow, not sticky.
  - "Butuh bantuan sekarang" moves into the fixed **help strip** at the top: a band with its own page background and 6px around the control, so its focus ring is never clipped; the control is `min-height: 44px` and grows with its text (about 56px, or about 73px when its label wraps). That keeps the D-036 "always visible" help control.
  - "Keluar cepat" and Menu join it only when all three fit in one row (landscape phones); otherwise they stay in the page flow directly below it. Esc twice works everywhere.
  - When the strip would take more than a third of the viewport height (for example 320×180 at 150% text), it returns to the page flow as the first control on the page, except while typing in the room (pending PM confirmation).
  - Focus is never hidden behind it: `scroll-padding-top` comes from its measured height (WCAG 2.4.11). The cases checked are 320×256 and 320×180 CSS px (1280×1024 and 1280×720 at 400%), at 100% and 150% text.
  - Details are in [DESIGN_DIRECTION.md §5.8](DESIGN_DIRECTION.md#58-layout-breakpoints-and-short-viewports).
- **Typing in the room on a phone:** while a text field in the room has focus, only the help control stays fixed (the same strip), "Keluar cepat" and Menu return to the page flow, and the composer is capped so Sela's reply stays readable above the keyboard; on very short screens the composer joins the page flow ([DESIGN_DIRECTION.md §6.5](DESIGN_DIRECTION.md#65-conversation-transcript-messages-composer)).

## 4. Route map

**Access** has five values:

- **public**: no session needed.
- **owner**: a signed-in guest or account.
- **account**: accounts only.
- **recipient**: the separate recipient principal.
- **demo**: the demo environment only.

**API** labels:

- "v3 spec": specified in [API_SPEC_V3](../api/API_SPEC_V3.md), with the foundation being implemented now.
- "v2": implemented in v2 and served in development, test and demo only (ADR-0002 §12).
- "planned": a reserved prefix in API_SPEC_V3 §9, with no contract yet.

| Route | Screen ([SCREENS.md](SCREENS.md)) | Title ID / EN | Access | Reached from | API |
|---|---|---|---|---|---|
| `/` | S01 Introduction (with language and text size) | Ruang sejenak untuk didengar – Sela / A little space to feel heard – Sela | public (owners are redirected to `/home`) | Direct, logo | `GET /api/v3/auth/options` (v3 spec) |
| `/start/age`, `/start/teen`, `/start/storage`, `/start/guardian`, `/start/ai` | S02 Onboarding steps (S26 for the teen steps). `/start/ai` (the Meta and Claude consent cards) is shown whenever AI replies are on and skipped only when they are off | "Langkah 1 dari 3: Usia – Mulai – Sela" / "Step 1 of 3: Age – Get started – Sela" | public until the storage step | Intro "Mulai", sign-in "Belum punya akun?" | `POST /api/v3/auth/guest`, `POST /api/v3/auth/register`, `PUT /api/v3/me/guardian-consent` (v3 spec); `PUT /api/v3/me/consents/ai_processing` and `…/ai_provider_anthropic` (planned P3/P5) |
| `/signin` | S03 Sign in | Masuk – Sela / Sign in – Sela | public | Intro, menu, session-ended screen | `POST /api/v3/auth/login` (v3 spec) |
| `/start/account` | S04 Create an account (within onboarding) | Buat akun – Sela / Create an account – Sela | public | Onboarding storage step | `POST /api/v3/auth/register` (v3 spec) |
| `/recover` | S05 Recovery | Pulihkan akun – Sela / Recover your account – Sela | public | Sign in "Lupa kata sandi?" | `POST /api/v3/auth/recovery` (v3 spec) |
| `/settings/account/upgrade` | S06 Guest upgrade | Simpan ke akun – Sela / Save to an account – Sela | owner (guest) | Guest banner, Home, Account, guest-expiry banner | `POST /api/v3/auth/guest/upgrade` (v3 spec) |
| `/home` | S07 Home | Beranda – Sela / Home – Sela | owner | Navigation, logo, the end of onboarding ("Lihat beranda dulu") | `GET /api/v3/me` (v3 spec); timeline, shares and conversations (planned) |
| `/talk` | S08 Conversations | Ruang cerita – Sela / Talk – Sela | owner | Navigation, Home | `/api/v3/conversations` (planned) |
| `/talk/new`, `/talk/:id` | S09 Conversation room | Percakapan – Sela / Conversation – Sela | owner | Home "Mulai bercerita", Talk, timeline entry, the last onboarding step | `/api/v3/conversations` (planned) |
| `/notes` | S12 Timeline | Catatan pribadiku – Sela / My personal notes – Sela | owner | Navigation, Home "Terbaru di Catatanku" | `/api/v3/timeline` (planned) |
| `/notes/write`, `/notes/:id` (a note) | S10 Note (journal) and reflection | Tulis catatan – Sela / Write a note – Sela | owner | Catatanku, Home, the end of a conversation | `/api/v3/journal` (planned) |
| `/notes/check-in`; `/notes/:id` (a check-in) | S11 Check-in, and the check-in detail | Cek perasaan – Sela / Check-in – Sela | owner | Catatanku, Home "Cek perasaan", the end of a conversation; the detail from its timeline row | `/api/v3/check-ins` (planned) |
| `/notes/patterns` | S12 Pattern view (opt-in) | Pola dari cek perasaanmu – Sela / Patterns in your check-ins – Sela | owner | Timeline (only once the person has opted in) | `/api/v3/check-ins` (planned) |
| `/notes/memory` | S13 What Sela remembers | Yang Sela ingat – Sela / What Sela remembers – Sela | owner | Catatanku section link, room Detail, Privacy & data | `/api/v3/memory` (planned) |
| `/support` | S14 Support hub | Dukungan – Sela / Support – Sela | owner | Navigation, Home | Summaries, shares and reading (planned) |
| `/support/summaries/new`, `/support/summaries/:id/edit/:step` | S15 Summary builder | Ringkasan untuk konsultasi – Sela / Summary for a consultation – Sela | owner | Support hub, timeline "Buat ringkasan dari ini", end of a conversation with intent "Menyiapkan konsultasi", a clarification request in S18 | `/api/v3/consultation-summaries` (planned); `/api/v2/ledger/*` (v2) as reference |
| `/support/summaries/:id` | S15 Summary view (show on screen, print) | Ringkasan untuk konsultasi – Sela | owner | Support hub, timeline | planned |
| `/support/summaries/:id/prepare` | S16 Preparation | Persiapan konsultasi – Sela / Preparing for a consultation – Sela | owner | Summary view | planned; `/api/v2/opening/*` (v2) as reference |
| `/support/summaries/:id/share` | S17 Sharing | Bagikan ringkasan – Sela / Share a summary – Sela | owner | Summary view | `/api/v3/shares` (planned); `/api/v2/handoffs/*` (v2) as reference |
| `/support/shares/:id` | S18 Share status, audit and revoke | Status berbagi – Sela / Sharing status – Sela | owner | Support hub, Home "Sedang dibagikan", timeline | planned |
| `/support/summaries/:id/after` | S19 After the consultation | Setelah konsultasi – Sela / After the consultation – Sela | owner | Summary view, share status, Support hub | `/api/v3/journal` (planned) |
| `/reading`, `/reading/:slug` | S20 Reading & small exercises | Bacaan & latihan kecil – Sela / Reading & small exercises – Sela | public (bookmarks: owner) | Support hub, menu, help page, Sela's offer in the room | `/api/v3/reading` (planned) |
| `/help` | S21 Immediate help (with the full directory, `#layanan`) | Bantuan sekarang – Sela / Help now – Sela (the page title; the control that opens it is "Butuh bantuan sekarang") | public | Top bar on every screen, crisis panel, footer, Support hub | Bundled data plus `GET /api/v2/catalog` help (v2) until `/api/v3/reading` help services exist (planned) |
| `/settings/privacy` (sections `#penyimpanan`, `#memori`, `#nama`, `#ai`, `#ekspor`, `#sesi`, `#riwayat`, `#persetujuan`, `#hapus`) | S22 Privacy & data | Privasi & data – Sela / Privacy & data – Sela | owner | Menu, sidebar, consent links everywhere, "Pengaturan AI" in the room | `/api/v3/me/consents`, `/me/export`, `/me/sessions`, `/me/audit`, `DELETE /api/v3/me` (v3 spec); names list and memory default (planned) |
| `/settings/display` | S23 Display & accessibility | Tampilan & aksesibilitas – Sela / Display & accessibility – Sela | public | Menu, sidebar, Introduction ("Pengaturan tampilan lainnya") | None (on this device); sync planned |
| `/settings/account` | S24 Account | Akun – Sela / Account – Sela | owner | Menu, sidebar | `/api/v3/me`, `/auth/password`, `/auth/recovery-codes`, `/me/guardian-consent`, `/auth/logout*` (v3 spec) |
| `/about` | S01 About (part of the Introduction content) | Tentang Sela – Sela / About Sela – Sela | public | Footer, menu | `GET /api/v2/ai/status` (v2) for the provider line |
| `/policies`, `/policies/:purpose` (`?version=`) | S28 Policy pages ("Singkatnya" first, versions, what changed) | Kebijakan – Sela / Policies – Sela; one policy: "{title} – Sela" | public | Footer, consent cards, onboarding, Privacy & data | `GET /api/v3/policies` (v3 spec); earlier versions (planned) |
| `/r` (token in the fragment `#t=…`) | S25 Recipient page | Halaman penerima – Sela / Recipient page – Sela | recipient | The link the user sent | `/api/v3/recipient/*` (planned); `/api/v2/recipient/*` (v2) as reference |
| `/research` | Reviewer research map (v2, unchanged) | Peta riset (untuk peninjau) – Sela | public, dev and demo | About → "Untuk peninjau" | `/api/v2/research/*` (v2) |
| any other | S27 Not found | Halaman tidak ditemukan – Sela / Page not found – Sela | public | — | — |

**Paths and URL history:**

- Paths are short English words. They stay stable when the language changes.
- They are also neutral in browser history: `/talk` and `/notes` say less than "/curhat" or "/krisis" would on a shared device.
- The Quick exit copy still says that browser history is not cleared ([CONTENT_STYLE.md](CONTENT_STYLE.md#77-quick-exit)).

## 5. Public, signed in, recipient

### 5.1 Access matrix

| Capability | Signed out | Guest | Account | Recipient |
|---|---|---|---|---|
| Immediate help, help directory | Yes | Yes | Yes | A link to "Tentang halaman ini" only (see §6) |
| Reading library | Read | Read and bookmark | Read and bookmark | No |
| Display & accessibility settings, language | Yes (this device) | Yes | Yes | Language only (this tab) |
| Talk, notes, check-in, timeline, memory | No | Yes | Yes | No |
| Summary, preparation, sharing, follow-up | No | Yes | Yes | No |
| Export, delete, sessions, audit, consents | No | Yes (no reauthentication; ADR-0002 §8) | Yes (export and delete need a password within 10 minutes) | No |
| Password, recovery codes | No | No (`account_required`) | Yes | No |
| Use on another device | No | No (the cookie is the only key) | Yes | One share, with link and code |
| Automatic deletion | — | After 7 days inactive, at most 30 days after creation (ADR-0002 §7) | Never automatic | Access ends at expiry or revocation |

### 5.2 Guest and account in the IA

- **The places are the same.** Guests see the same four places as accounts. The difference shows up in two spots:
  - a **retention line** wherever stored data is listed ("Mode tamu · dihapus otomatis jika 7 hari tidak aktif");
  - the **guest-expiry banner**, from 7 days before the maximum age.
- **Upgrading** ("Simpan ke akun") is offered on Home, in Account and in the guest-expiry banner. It is never a gate.
- **Guests have no "Keluar dari akun" (sign out), and no session revocation.** Signing out, or revoking the guest's own session, would make their data unreachable, because the cookie is the only key. The API allows both (API_SPEC_V3 §7), so the UI is what protects the guest: Account and "Sesi & perangkat" in Privacy & data show the session read only and offer two actions, "Hapus data tamu sekarang" (delete guest data now) and "Simpan ke akun" (save to an account) ([SCREENS.md S24](SCREENS.md#s24-account), [S22](SCREENS.md#s22-privacy-and-data-settings)). Quick exit never signs a guest out.
- **The cost of that, stated plainly:** anyone who uses the same browser can open a guest's data, and with previews shown, Home's "Terbaru di Catatanku", the share labels and the timeline show what it is about. Guest mode is therefore for a device the person does not share. The design says so and offers the mitigations:
  - the guest storage notice in onboarding says "Siapa pun yang memakai browser ini bisa membuka data tamumu. Di perangkat bersama, pakai akun." (S02 step 2);
  - the same panel offers "Sembunyikan pratinjau isi" as an optional equal-weight choice, and Privacy & data repeats it with the same line;
  - "Simpan ke akun" adds sign-out, and Quick exit can then sign the account out;
  - neutral tab titles (planned) hide the page names in the tab bar.

### 5.3 The recipient is separated

The recipient experience is separate in every dimension: route, shell, principal, cookie, copy register and data. The parts that are PM decisions are D-042 and ADR-0002 §10. The rest are design decisions.

- **Route:** `/r`. The token is in the URL fragment, which is never sent to the server, and it is scrubbed from the address bar on load (the v2 behaviour).
- **Shell:** "Sela · Halaman penerima". It has no owner navigation, no Quick exit (the recipient is not the person at risk), no owner banners and no owner session data.
- **Principal:** a separate recipient cookie (`__Secure-sela_recipient`, `Path=/api/v3/recipient`) bound to exactly one share (API_SPEC_V3 §9).
  - An owner session never opens recipient routes, and a recipient principal never opens owner routes.
  - If the owner is signed in in the same browser and opens their own link, they see the recipient page as a recipient would. The page says so: "Kamu sedang melihat seperti penerima." / "You are viewing as the recipient."
- **Copy register:** formal "Anda", and the sharer is called "pengirim" (sender), never "pasien" ([CONTENT_STYLE.md §9](CONTENT_STYLE.md#9-recipient-copy)).
- **Data:** only the immutable projection the user previewed and consented to. Nothing else from the owner's account is visible.

### 5.4 Minors (15–17; only 16–17 can use Sela) in the IA

PM decision D-031: implemented, but disabled in production until legal review.

- The places are the same. The differences are in onboarding (the 15 / 16–17 split, confidentiality limits, guardian consent that grants no access), defaults (memory and patterns are opt-in only) and copy.
- 15-year-olds are blocked by the UI in every environment, and nothing is created (D-031). Until guardian consent can be sent together with account or guest creation, 16–17 may start only as a guest, and a declined consent deletes the guest at once.
- When guardian consent is missing or withdrawn (`actionsRequired: guardian_consent`), every place shows an interstitial explaining what is needed. **These stay reachable throughout:** Privacy & data, Display, Account, Immediate help and the reading library.
- Details are in [SCREENS.md S26](SCREENS.md#s26-minors-variants-15-17).

## 6. Where help, Quick exit and help resources live

| Shell or screen | "Butuh bantuan sekarang" | "Keluar cepat" | Other help entry points |
|---|---|---|---|
| Public pages (intro, onboarding, sign in, recovery, reading, display, about, policies) | Top bar, 2nd focusable element after the skip link (3rd if the mark is shown) | Top bar, immediately after help | Footer link; onboarding step "Tentang Sela" explains it |
| Owner pages (four places, settings) | Top bar (compact and medium) or top strip (wide), same DOM position | Same, immediately after help | Footer link; Support hub section "Layanan bantuan" (links to `/help#layanan`) |
| Conversation room | Top bar, unchanged; while typing on a phone, the only fixed control (the help strip) | Top bar; while typing on a phone, in the page flow at the top | **Crisis panel** inline after the matching message (D-036); **at the same moment, the pause line** directly above the composer, with "Ke bantuan" (it moves focus to the panel); a new-content button that says "Bantuan ditampilkan di bawah pesanmu" when the reader is scrolled up; the support line beside an `elevated` reply; the room's empty state names the help button; the Detail sheet starts with "Butuh bantuan sekarang" and "Keluar cepat" |
| Journal and check-in after a crisis match | Top bar | Top bar | Crisis panel shown after saving (the entry is kept if storage allows; D-036) |
| Short viewport mode | The fixed help strip at the top (`min-height: 44px` control, grows with its text, 6px of page background around it); in the page flow, as the first control, when the strip would take more than a third of the viewport height | In the fixed strip when help, Quick exit and Menu fit in one row; otherwise in the page flow just below it. Esc twice always works | — |
| Full-height sheets (Menu, the room's Detail) | **The first item of the sheet**, same label and look as the header | **The second item of the sheet**; one activation leaves | — |
| Small dialogs | Behind the modal (inert). Every dialog that could be reached while distressed (session warnings, the unsaved-conversation warning, storage change, delete) includes a "Butuh bantuan sekarang" link in its text | Esc closes the dialog first; the Esc that closes it never counts, and presses within 1.5 s after a dialog, menu or disclosure closes are ignored, so closing something never exits by accident | — |
| Error and system screens (not found, session ended, store unavailable) | Top bar | Top bar | The body text links to help |
| Immediate help page itself | The top bar's help link shows `aria-current="page"` | Top bar | It *is* the help page and works offline from bundled data |
| Recipient page | Not shown. The recipient is not the person at risk, and an owner's help button there would confuse | Not shown | A footer link "Tentang halaman ini" explains what the page is and who to contact (the sender) |

Help resources ("Layanan bantuan") have a single source: the verified directory, with source, verification date and re-verify-by date (D-044). It is shown in three places:

- `/help`: urgent first, then the full list under `#layanan`;
- the crisis panel: up to three entries, chosen by the rule "urgent and matching the age band", falling back to the static lead and the verified general entries when nothing verified matches;
- the Support hub: a link.

The same data is never copied into different texts. Teen entries appear only if verified (D-044), and the unverified KPAI WhatsApp entry from v1 is not shown.

## 7. How the v2 consultation flow is re-homed

v2's single path was story → ledger → next step → opening practice → share → recipient → follow-up. In v3 it becomes a set of places that the person can use in any order. The consultation part is gathered under one object, the **summary**, which owns its preparation, its shares and its follow-up.

```
v2:  /story ─▶ /ledger ─▶ /next ─▶ /practice ─▶ /share ─▶ /r ─▶ /journey (follow-up)

v3:  Cerita & Catatanku (many conversations, notes, check-ins over time)
            │  choose notes
            ▼
     Dukungan ▸ Ringkasan untuk konsultasi
            ├─ Susun & periksa (review, approve version)        ← v2 /ledger
            ├─ Persiapan (destination checklist, opening lines,  ← v2 /next + /practice
            │             practice)
            ├─ Bagikan (preview, consent, expiry, link + code)   ← v2 /share
            │     └─ Status berbagi (stepper, audit, revoke)     ← v2 /journey
            │            └─ Halaman penerima /r                  ← v2 /r
            └─ Setelah konsultasi (own-words follow-up,          ← v2 fixture follow-up
                                   check-in, next summary)
```

| v2 step (route) | v3 home | What changes | Kept from v2 |
|---|---|---|---|
| Welcome `/` | Introduction `/` | Listening-first copy. Starting as guest or with an account. No "first step towards professional support" framing (`welcome.lead`) | Bilingual copy; honest "what Sela is / is not" lists |
| Start `/start` (policy, pseudonym, adult checkbox) | Onboarding `/start/*` | An age band replaces the adult checkbox (D-031). Guest or account (D-029). The pseudonym is dropped: accounts have a username, guests need none | Versioned policy acceptance; safety screening of free-text fields such as the username |
| Consent `/consent` (AI, storage) | Onboarding "Di mana datamu disimpan" (with the default for conversations) and "Balasan AI", the per-conversation storage and memory choices, Privacy & data | Consent per provider with its location (D-049), asked with buttons of equal weight. Storage is chosen per conversation (D-035), with a default set once | Independent, versioned consents with proof |
| Story `/story` (one text area or a sample scenario) | The room `/talk/*` and "Tulis catatan" `/notes/write` | Many conversations and notes; listening replies released only after the guards; sample scenarios only in demo, labelled "Contoh cerita (sintetis)" | Safety gate before storage or inference. v2's **contact-detail refusal becomes a non-blocking notice**: the message is sent at once, AI never receives contact details, and a notice on the sent message offers to mask the stored copy too (ID and card numbers are masked in storage by default, with an undo) ([SCREENS.md S09](SCREENS.md#s09-conversation-room)). Names the person lists are masked before every model call (D-030) |
| Ledger `/ledger` (review, approve) | Summary builder, Review step | Built from **chosen** entries across time, with provenance classes and separate review statuses (D-040), masked private names (the v1 Care Passport redaction preview, F23) and a verifier gate (D-030). Named "Ringkasan untuk konsultasi" (D-039) | Quote kept apart from wording; confirm / edit / use my words / remove; Meaning Lock; approval bound to version and hash; "not known yet" items |
| Next step `/next` (fictional professionals, preferences) | Preparation: "Ke mana kamu akan membawanya?" | Destination **types** (Puskesmas/FKTP, psikolog, Healing119, campus service, someone I trust, not sure yet) with sourced checklists from v1 F25. No professional directory in production (D-042). The fixture professionals stay for the demo configuration only. Practical preferences become an optional summary section | Preference fields; deterministic, explainable rules in demo |
| Practice `/practice` | Preparation: opening lines and practice | Placed beside the checklist | Template, AI or own lines; coverage feedback; attempt not stored; no score |
| Share `/share` | Sharing | Recipient is a label the user types and is marked unverified (D-042). **Purpose** is added. A per-name choice to show masked names. Longer expiry options, none under 20 hours (open point). A new link or a re-share starts pre-filled from the old share. Preview equals the stored projection | Field choice, preview, explicit consent with policy version, idempotency key, revoke |
| Journey `/journey` | Share status page, plus events in the timeline | New status machine (D-041), shown as an event list built only from recorded events; the self-reported step says "Kamu menandai". Every recipient access and clarification request is in the audit list | Timeline event shape; "sent is not care" (`careReceived:false`) |
| Recipient `/r` | Recipient page `/r` | Unverified real recipient. Fixed actions: acknowledge, ask for clarification (four fixed reasons), cannot accept (with an inline confirm). Formal register. Generator and checker named on AI wording | Token in the fragment plus a separate code; the code format (8 characters without look-alike characters) and tolerant entry (case, spaces and hyphens ignored); uniform `access_ended`; lockout; link scrubbing |
| Follow-up (fixture plan and check-ins in Journey) | "Setelah konsultasi" and the standalone check-in | Follow-up is written in the user's own words (provenance `user_original`, never a professional note). Check-in no longer depends on a handoff | Check-in record; no implication of care |
| Settings `/settings` | Privacy & data, Display & accessibility, Account | Export, sessions, audit, provider consents, memory and deletion with the backup timeline added | Delete semantics, now cascading (ADR-0002 §10) |
| Privacy `/privacy` | `/policies` | Policies rewritten: there is a database now, and guest storage is explained | Versioned policy rendering |
| Help `/help` | `/help` | Teen entries (verified only), stale flags, the full directory on the same page | Verified schema; bundled fallback |
| Research `/research` | Unchanged reviewer page, outside user navigation | The corpus stays separate from user data (D-033) | As is |

**Transition** (design decision, pending the PM plan):

- The v3 UI calls `/api/v3` only.
- Until the P3 and P4 contracts exist, the demo environment may run the summary and sharing screens against the v2 endpoints, behind an adapter, so the screens can be reviewed in a browser.
- Production never does this, because the v2 journey store is refused there (ADR-0002 §12).

## 8. Multiple ways to every surface (WCAG 2.4.5)

| Surface | Way 1 | Way 2 | Way 3 |
|---|---|---|---|
| Room | Bottom bar or sidebar: Cerita | Home "Mulai bercerita" | Timeline entry (a saved conversation) |
| Timeline | Navigation: Catatanku (or the Menu in bottom-bar tier C) | Home "Terbaru di Catatanku" → "Lihat semua" | Any entry's back link |
| Note | Catatanku "Tulis catatan" | Home "Tulis catatan" | Closing panel of a conversation |
| Check-in | Catatanku "Cek perasaan" | Home "Cek perasaan" (a link to the full page; Home has no inline check-in) | Closing panel; Setelah konsultasi |
| Check-in detail | Its timeline row | The pattern view's table | A summary's "Lihat asalnya" |
| What Sela remembers | Catatanku section link | Room Detail "Yang Sela ingat" | Privacy & data → Memori |
| Summary | Support hub | Timeline "Buat ringkasan dari catatan ini" | End of a conversation with intent "Menyiapkan konsultasi" |
| Share status | Support hub | Home "Sedang dibagikan" | Timeline share event |
| Reading | Support hub | Menu | Help page "Bacaan yang mungkin membantu"; Sela's offer in a reply |
| Immediate help | Top bar (every screen) | Footer | Crisis panel |
| Privacy & data | Menu or sidebar | Every consent or storage line ("Ubah di Privasi & data"), and "Pengaturan AI" on the room's no-reply notices | Account page |

## 9. State-dependent navigation

| State | What changes | What never changes |
|---|---|---|
| Crisis in a conversation (a lexicon match, generator risk `crisis` or a verifier `safety_signal`; D-036) | That conversation pauses AI replies until the person chooses "Lanjutkan bercerita dengan Sela". From that moment a persistent pause line above the composer says so (with "Ke bantuan" while the panel is shown); messages sent meanwhile are kept (per storage) and marked "tanpa balasan (dijeda)". A later match shows the panel again, condensed if it was shown recently. A dismissible crisis follow-up banner appears **on Home and in the room only**, for the rest of the browser session, and not at all when previews are hidden. **Nothing else is locked.** v2's session-wide interruption (`Safety.Interrupted`) is replaced by this per-conversation pause (gap-v2 §5) | Everything else stays usable. The user's text is never erased |
| Unsaved conversation ended | The closing panel holds it in memory with "Lanjutkan percakapan" and "Simpan percakapan ini". The hold survives in-app navigation (Help, Privacy & data, any place; Home and Cerita offer "Lanjutkan percakapan"), and ends only on Quick exit, tab close, a new conversation, "Buang sekarang", or a hold timeout that warns first with "Tetap buka" | Help, the other places |
| Conversation inactive for 12 hours (planned P3) | It ends automatically; Home stops offering "Lanjutkan percakapan" | It can still be reopened and continued |
| Session about to end (accounts: 2 minutes before the idle limit; 10 minutes before the 12-hour or 30-day absolute limit) | A dialog: "Tetap masuk" for the idle limit; the password for the absolute limit. The page and drafts stay | Everything behind the dialog stays as it was |
| Session ended (`401 not_signed_in`) | S27 "Sesimu sudah berakhir": sign in (accounts) or an explanation (guests). Unsent drafts in the tab are kept and offered after signing in | Help, display, reading |
| Policy outdated (`actionsRequired: accept_policy:*`) | A policy interstitial before the places, showing what changed and offering [Setuju] or [Ekspor dan hapus data] | Privacy actions are never blocked (API_SPEC_V3 §3) |
| Recovery codes not acknowledged | Banner only | Everything |
| Guardian consent missing or withdrawn (16–17) | An interstitial on the four places | Privacy & data, Display, Account, help, reading |
| Age band confirmation due (`confirm_age_band`) | One question on Home ("Apakah usiamu masih 16–17 tahun?") | Everything else |
| Guest close to its maximum age | Banner from 7 days before | Everything |
| AI disabled or no AI consent (including before the `ai_processing` purpose is active) | No live provider is called. The room works as "write without replies". Summary drafting by AI is unavailable and the manual path stays. All of it is explained in place | All places |
| Checker (Claude) not allowed, or never answered | Replies that mention health are not shown, with a neutral notice; a never-answered checker question is asked once, in context, at the first such reply. The backup model (DeepSeek) does not reply either, because Claude checks every backup reply (ADR-0003 RR-4 (i), §17 O-6). Summary drafting by AI and memory proposals are unavailable; the manual paths stay | All places |
| Store unavailable (`503 store_unavailable`) | S27 "Penyimpanan sedang tidak tersedia", with retry | Immediate help (bundled), display, reading (if cached) |
| Offline | Offline banner. Lists show the last loaded view with its time. Sending shows "Belum terkirim" | Immediate help (bundled) |
| Demo environment | Demo banner on every screen. Seeded synthetic personas and "Contoh cerita (sintetis)" starters | All features really run (D-046) |

## 10. Content separation in the IA (PM decision D-033)

| Content type | Where it appears | Never |
|---|---|---|
| Public content (reading, help directory, destination checklists) | `/reading`, `/help`, Preparation | Copied into a person's notes. A bookmark stores only a reference |
| Personal data (conversations, notes, check-ins, memory, summaries, shares, audit) | The four places and settings, for its owner only | Shown to another user. Shown to a recipient beyond the consented projection |
| Community content | None in the pilot (the forum is paused, D-038) | — |
| Demo data | The demo environment only, labelled "sintetis" | Mixed into production |
| Research corpus | The reviewer page `/research` | Linked to any user's history; shown as evidence in Sela's replies |

## 11. Where each record appears and how it leaves

Classes and review statuses are separate fields. The full mapping, with every transition, is [DESIGN_DIRECTION.md §6.7.1](DESIGN_DIRECTION.md#671-record-types-class-statuses-and-transitions); this table only places each record.

| Record (class · statuses) | Appears in | Correct or delete from | Export section (planned, through the API_SPEC_V3 exporter registry) |
|---|---|---|---|
| Conversation and messages. The person's messages: `user_original` · confirmed; corrections: `user_correction` · confirmed. Sela's replies: `ai_summary` · unreviewed or corrected (the class pending confirmation), labelled in the transcript only by their author line naming generator and checker, with the badge and status in "Tentang balasan ini" and the export. Replies are never quoted in a summary: only the person's own words (`user_original`, `user_correction`) are quoted | Talk list, room, timeline (saved only) | Room: delete the conversation (with or without derived items); storage switch | `conversations` |
| "Sela menangkap…" items: `unconfirmed_interpretation` · unreviewed → confirmed or rejected; an edited item is a new version, `user_correction` · corrected | Room, room Detail | The card itself | `conversations.reflections` |
| Memory items: accepted proposals `unconfirmed_interpretation` · confirmed; added by hand `user_original` · confirmed; edited `user_correction` · corrected. Proposals come only from saved entries. A memory item chosen as a summary source keeps its class there: an accepted proposal is "Tafsiran Sela", never a quote, and the recipient sees "Tafsiran Sela · dikonfirmasi pengirim" | Yang Sela ingat; used in replies when memory is on for that conversation; summaries that include them | Yang Sela ingat: change, forget (with the derived-items choice when a summary includes it), pause; forgotten with their source when the person deletes "with derived items"; all forgotten when the memory consent is withdrawn | `memory` |
| Names kept private (the person's list; not a provenance-bearing record) | Privacy & data → Nama yang disamarkan; applied to AI calls, summaries and shares | Privacy & data: add, remove | `privateNames` |
| Notes and follow-up notes (`user_original` · confirmed, versions) | Timeline, note page | Note page: edit (new version), delete (with or without derived items) | `journal` |
| Check-ins (`user_original` · confirmed) | Timeline, pattern view | Check-in detail: delete (with or without derived items) | `checkIns` |
| Summary and versions (quotes `user_original` or `user_correction`; wording `ai_summary`, `user_correction` or `user_original`; items from memory keep the memory item's class, so a Sela interpretation stays `unconfirmed_interpretation`; each with its own status; approval bound to version) | Support hub, summary view, timeline | Builder (new version); delete the summary; a new version without deleted quotes or forgotten memory items when a source is deleted or forgotten "with derived items" | `consultationSummaries` |
| Share and its audit events (content-free; the projection inherits the frozen classes and statuses) | Support hub, share status, timeline, Privacy & data → Riwayat akses | Revoke or end (stops access); revoked automatically when a source is deleted "with derived items". The share record stays in the owner's history until the summary or the account is deleted | `shares` |
| Consents, sessions, account audit | Privacy & data, Account | Withdraw a consent; revoke a session | `consents`, `consentHistory`, `sessions`, `audit` (v3 spec) |
| Bookmarks (references to public reading) | Reading "Disimpan" | Unbookmark | `bookmarks` |

Deleting the account or the guest data removes all of it at once.

- The live data goes immediately.
- No backup holds a usable key after 7 days.
- Backups age out after 30 days (ADR-0002 §11).

The deletion screen shows exactly these dates.
