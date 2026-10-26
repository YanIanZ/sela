# Sela v3 screens

Status: **design decision** by the design lead, 2026-10-26, revised after review rounds 1 and 2 ([DESIGN_DIRECTION.md §11](DESIGN_DIRECTION.md#11-revision-log)). **Not implemented.** Components, tokens and patterns come from [DESIGN_DIRECTION.md](DESIGN_DIRECTION.md). Places and routes come from [INFORMATION_ARCHITECTURE.md](INFORMATION_ARCHITECTURE.md). Copy rules come from [CONTENT_STYLE.md](CONTENT_STYLE.md). Accessibility acceptance criteria come from [ACCESSIBILITY_CRITERIA.md](ACCESSIBILITY_CRITERIA.md).

## How to read a screen section

Every section has the same fields:

- **Purpose:** why the screen exists.
- **Primary action:** at most one per view.
- **Secondary actions.**
- **Content and data:** what is shown, and the provenance of each item. The classes are from D-039: `user_original`, `ai_summary`, `user_correction`, `unconfirmed_interpretation`, `uploaded_document` (reserved) and `verified_professional_note` (reserved). Class and review status are separate; the mapping for every record type is in [DESIGN_DIRECTION.md §6.7.1](DESIGN_DIRECTION.md#671-record-types-class-statuses-and-transitions).
- **States:** empty, loading, error and success.
- **Recovery:** how a person gets out of every failure without losing their words.
- **Keyboard and focus order.**
- **Layout:** at 320 CSS px, and on wide screens (shell 64em or wider).
- **Copy:** Indonesian and English examples. Final strings live in `web/src/i18n/{id,en}.ts` and keep ID/EN parity, as in v2.
- **API:**
  - "v3 spec": specified in [API_SPEC_V3](../API_SPEC_V3.md); the foundation is being implemented.
  - "v2": implemented in v2; development, test and demo only.
  - "planned": a reserved prefix in API_SPEC_V3 §9. The method and path given are a **design proposal**, and the owning milestone writes the contract.

These rules hold on every screen. They are not repeated in each section.

- **Shell:** the top bar holds "Butuh bantuan sekarang" (the Help-quiet form), "Keluar cepat" and Menu in a fixed order, with full labels that wrap (two rows on phones; one full-width strip across the window on wide layouts), the notices region follows, and there is one `h1`. Every full-height sheet (Menu, the room's Detail) starts with "Butuh bantuan sekarang" and "Keluar cepat". The announcement regions `#sr-status`, `#sr-reply` and `#sr-alert` exist from the first paint as the last children of `<body>`, and are cleared on every route change ([DESIGN_DIRECTION.md §6.1](DESIGN_DIRECTION.md#61-app-shell)).
- **Route change:** focus moves to `<main>`, which is named by the page h1 (`aria-labelledby`), so screen readers announce the page name (the v2 router behaviour, with a name added), and `document.title` is set to "{title} – Sela".
- **Required choices:** a required radio group that is left empty shows the group error of [DESIGN_DIRECTION.md §6.3](DESIGN_DIRECTION.md#63-inputs): the message in the `<legend>`, `role="radiogroup"` with `aria-invalid="true"` on the fieldset (not on the radios), and focus on the first option.
- **Errors:**
  - Every API error is shown in plain language from its stable `code` (the v2 `ErrorNotice` pattern).
  - `rate_limited` shows the static text "Coba lagi dalam sekitar N menit." The value comes from `Retry-After`, rounded up, with no live countdown.
  - `server_busy` and `store_unavailable` offer [Coba lagi].
- **Unsaved text:** typed text is never discarded by an error. Drafts are kept in memory until sent or saved. Quick exit discards them.
- **Focus when something disappears:** when a control or a row is removed while it has focus, focus moves to the next row's first control, or to the list heading when the list is empty, and `#sr-status` says what happened ([DESIGN_DIRECTION.md §6.12](DESIGN_DIRECTION.md#612-dialogs-sheets-and-inline-confirmation)).
- **Deleting a source:** deleting a conversation, note or check-in first lists what was built from it and offers to delete those too ([DESIGN_DIRECTION.md §6.12](DESIGN_DIRECTION.md#612-dialogs-sheets-and-inline-confirmation)).
- **Hidden previews:** with "Sembunyikan pratinjau isi" on, lists and cards show only type and time: no preview text, no conversation or summary titles, no recipient labels.
- **Targets and gaps:** every target is at least 44×44px, with gaps of at least 8px. Nothing needs a drag or a long press.

---

## S01 Introduction

**Route** `/` (with `/about` as the longer "Tentang Sela" page) · **Access** public; signed-in owners are redirected to `/home` · **Title** "Ruang sejenak untuk didengar – Sela" / "A little space to feel heard – Sela" (the same "{title} – Sela" pattern as every route)

- **Purpose:** tell a first-time visitor in a few seconds what Sela is, what it is not, and how to start, without asking for anything. It also lets them set the language and text size before reading on, which used to be an onboarding step.
- **Primary action:** "Mulai" / "Get started" leads to S02.
- **Secondary actions:**
  - the language buttons "Bahasa Indonesia" / "English" (§6.19, each label in its own language);
  - "Ukuran teks" / "Text size": a radio group of four sizes, each with an "Aa" sample; it applies at once;
  - "Pengaturan tampilan lainnya" / "More display settings" leads to S23 and returns here;
  - "Masuk" / "Sign in" leads to S03;
  - "Pelajari Sela lebih lanjut" / "Learn more about Sela" leads to `/about`;
  - "Bacaan & latihan kecil" (public).
- **Content and data:** public copy only.
  - The mark, then the h1 (the tagline).
  - A two-sentence lead.
  - "Bahasa dan ukuran teks" / "Language and text size": the two controls in one compact group.
  - "Yang perlu kamu tahu" (four points).
  - A short "Cara Sela menjaga keselamatanmu" (how Sela keeps you safe) note.
  - A footer with the Dengar.in attribution.
  - In demo, the demo banner.
- **About page** (`/about`):
  - how the AI works;
  - the providers and where they process data (D-048), using the live provider line from `GET /api/v2/ai/status` until v3 has its own;
  - what is never done (no diagnosis, no contacting anyone);
  - credits;
  - the link "Untuk peninjau" (for reviewers) to `/research`.
- **States:**
  - empty: not applicable;
  - loading: the page is static, and `GET /api/v3/auth/options` only decides whether "Mulai sebagai tamu" is offered;
  - error: if the options call fails, the page still renders, with "Mulai" kept and the guest availability checked again in S02;
  - success: a language or size change shows the status "Bahasa diganti" or "Ukuran teks: Lebih besar".
- **Recovery:** with no connection, the page and the Immediate help page render from the bundle. Language and size are kept in memory (and in `localStorage` when allowed) and carried into onboarding.
- **Keyboard and focus order:**
  1. skip link;
  2. Butuh bantuan sekarang;
  3. Keluar cepat;
  4. Menu;
  5. h1 (not focusable);
  6. Bahasa Indonesia, English;
  7. the text-size radios (arrow keys);
  8. Pengaturan tampilan lainnya;
  9. Mulai;
  10. Masuk;
  11. Pelajari Sela lebih lanjut;
  12. footer links.
- **Layout:**
  - 320px: one column; the mark above the h1 at `--font-size-7`; the language buttons side by side (they wrap at large text); the four size options in a 2×2 grid; Mulai and Masuk stacked full width; "Yang perlu kamu tahu" as a plain list.
  - Wide: the mark in the header; the h1 at `--font-size-8`, within `--measure`; the buttons inline; the lists in two columns ("Sela bisa" / "Sela tidak").

| Element | ID | EN |
|---|---|---|
| h1 | Ruang sejenak untuk didengar. | A little space to feel heard. |
| Lead | Ceritakan apa yang sedang kamu rasakan. Sela mendengarkan, membantumu melihat kembali ceritamu, dan kalau kamu mau, membantumu bersiap bicara dengan profesional. | Talk about what's on your mind. Sela listens, helps you look back at what you said and, if you want, helps you get ready to talk with a professional. |
| Language and size | Bahasa dan ukuran teks · Kamu bisa mengubahnya kapan saja di Menu. | Language and text size · You can change these any time in the Menu. |
| Points | Sela adalah AI, bukan psikolog atau dokter. · Sela tidak memberi diagnosis atau pengobatan. · Kamu yang memilih apa yang disimpan dan dibagikan. · Sela tidak pernah menghubungi siapa pun atas namamu. | Sela is an AI, not a psychologist or a doctor. · Sela gives no diagnosis or treatment. · You choose what is kept and what is shared. · Sela never contacts anyone on your behalf. |
| Safety note | Kalau kamu dalam bahaya sekarang, tekan "Butuh bantuan sekarang" di bagian atas layar. | If you're in danger right now, press "Need help now" at the top of the screen. |

- **API:** `GET /api/v3/auth/options` (v3 spec); `GET /api/v2/ai/status` (v2) for the About page.

## S02 Onboarding

**Routes** `/start/age` → (16–17 or "unknown": `/start/teen`; see S26) → `/start/storage` → (account: `/start/account`, `/start/codes`; see S04) → (16–17: `/start/guardian`; see S26) → `/start/ai` → the room `/talk/new` · **Access** public · **Title** "Langkah 1 dari 3: Usia – Mulai – Sela"

Onboarding is short:

- **target: at most four screens from "Mulai" to the composer** for an adult guest: three onboarding steps, then the room. Journey J1 measures it. An account adds two screens (account form, recovery codes); 16–17 adds the confidentiality step and the guardian step;
- one plain question per step;
- every step its own URL, so Back works;
- a progress line in text: "Langkah 1 dari 3" / "Step 1 of 3". The total is recomputed when a path adds steps (an account: "dari 5"), and when AI replies are off, so the AI step is skipped ("dari 2").

**No live AI provider is ever called without a granted `ai_processing` consent** (D-029, D-048, RR-2; brief §8). The AI step (step 3) is therefore shown whenever AI replies are on, in every environment, the demo included, and skipped **only when AI replies are off**. Until the `ai_processing` purpose is active (it is reserved for P3/P5 in API_SPEC_V3 §3), AI replies count as off for the person: the room opens in its "AI off" state (write without replies), except in development and test, where the labelled test double may reply, because nothing leaves Sela. Journey J1 checks this.

**Nothing is sent to the server before the storage step** (the v1 defect F01 wrote a check-in before consent). Values chosen earlier (language and text size on S01, the age band) are kept in memory and carried forward, so they are never asked twice (WCAG 3.3.7).

### Step 1: Sebelum mulai & usia (what to expect, and age band) · `/start/age`

- **Purpose:** honest expectations before any data exists, and the right protections, on one screen. The step asks for an age band, not a birth date, and adults are not asked for finer bands (data minimisation; brief §6).
- **Content:**
  - "Sebelum mulai" / "Before you start", a short block of four points: what Sela does (listen, reflect, keep what you choose, help you prepare); what it does not do (no diagnosis, not a professional, not an emergency service, contacts no one); "Sela memeriksa tulisanmu untuk menampilkan bantuan jika ada tanda bahaya. Sela tidak memberi tahu siapa pun." / "Sela checks what you write so it can show help if there are signs of danger. Sela does not tell anyone."; and "Tekan Keluar cepat, atau Esc dua kali, untuk langsung pindah ke halaman lain." / "Press Quick exit, or Esc twice, to leave at once.";
  - the age question.
- **Control:** a radio group, required, legend "Berapa usiamu?" / "How old are you?":
  - "18 tahun ke atas" / "18 or older";
  - "16–17 tahun" / "16–17";
  - "15 tahun" / "15";
  - "Di bawah 15 tahun" / "Under 15";
  - "Aku tidak ingin menyebutkan" / "I'd rather not say". Offered only if `unknown` is allowed by configuration.

  The 15 / 16–17 split is **mandatory now** (D-030: 15-year-olds stay blocked).
- **Hint:** "Supaya Sela menunjukkan informasi dan perlindungan yang sesuai. Kami tidak menanyakan tanggal lahirmu." / "So Sela can show the right information and protections. We don't ask for your date of birth."
- **Outcomes:**
  - "Di bawah 15 tahun": S27 "Under 15". Nothing is created;
  - "15 tahun": S27 "15 tahun", **in every environment**, with teen help. Nothing is created. The block is enforced by the UI today; server enforcement waits for an API sub-band ([DESIGN_DIRECTION.md §10](DESIGN_DIRECTION.md#10-open-points-for-the-pm-and-other-owners) item 10);
  - a band that is not enabled (`ageBands.allowed` from `/auth/options`; production allows only `18_plus` until the legal review): S27 "Belum tersedia untuk kelompok usiamu", with teen help resources;
  - "16–17 tahun" or "Aku tidak ingin menyebutkan", when enabled: `/start/teen`, the confidentiality step (S26). The `unknown` band gets the same explanation and every other minor default (ADR-0002 §3; the list is in S26), but no guardian step (the API records guardian consent for `15_17` only; whether that may stay so is an open point for the legal review, [DESIGN_DIRECTION.md §10](DESIGN_DIRECTION.md#10-open-points-for-the-pm-and-other-owners) item 20);
  - "18 tahun ke atas": step 2.
- **Error:** Lanjut with nothing chosen shows the group error, "Perlu diperbaiki: pilih salah satu." / "Needs fixing: choose one.", in the legend, with `aria-invalid` on the group, and focus moves to the first option.
- **Primary action:** "Lanjut" / "Continue".

### Step 2: Di mana datamu disimpan (where your data is kept) · `/start/storage`

- **Purpose:** an informed choice of where data lives (D-028; ADR-0002 §7), and the default for new conversations, so the room does not ask again.
- **Primary action:** depends on the choice.
  - "Mulai sebagai tamu" / "Start as a guest" calls `POST /api/v3/auth/guest`.
  - "Lanjut membuat akun" / "Continue to create an account" leads to S04.
- **Secondary action:** "Sudah punya akun? Masuk" / "Already have an account? Sign in".
- **Content:**
  1. **"Di mana datamu disimpan?"**: two option panels (a radio group). When an option is selected, its full notice expands in the page. The notice is never hidden behind a link, because the API requires it to have been shown.
     - **Guest:**
       - no username needed;
       - stored on Sela's server, encrypted per person, in the hosting region, named once B-3/B-15 decide it;
       - deleted after 7 days without activity, at the latest 30 days after starting;
       - only this browser can open it, so a private window, clearing browser data or another device loses access;
       - **and anyone who uses this browser can open it**, because guests have no sign-out and Quick exit never signs a guest out: "Siapa pun yang memakai browser ini bisa membuka data tamumu. Di perangkat bersama, pakai akun." / "Anyone who uses this browser can open your guest data. On a shared device, use an account.";
       - "Ini bukan anonimitas mutlak: server tetap melihat alamat koneksimu untuk keamanan, dan operator Sela menjalankan servernya." / "This isn't complete anonymity: the server still sees your connection address for security, and Sela's operators run the servers.";
       - **an optional choice, of equal weight** (a radio pair, nothing selected): "Di perangkat ini, pratinjau isi di Beranda dan Catatanku:" / "On this device, content previews on Home and in My notes:" "Tampilkan" / "Show" · "Sembunyikan (lebih aman di perangkat bersama)" / "Hide (safer on a shared device)". Hint: "Kalau tidak dipilih, pratinjau ditampilkan. Bisa diubah kapan saja di Privasi & data." / "If you don't choose, previews are shown. You can change this any time in Privacy & data." It sets "Sembunyikan pratinjau isi" on this device and sends nothing to the server.
     - **Account:**
       - a username (not your real name) and a long password;
       - 10 recovery codes;
       - no email asked;
       - kept until you delete it;
       - works on any device.
     - **16–17 (interim, until guardian consent can be sent with creation):** only the guest option is offered, with the line "Untuk usia 16–17 tahun, Sela sementara hanya bisa dipakai sebagai tamu. Setelah persetujuan wali tercatat, kamu bisa menyimpannya ke akun." ([DESIGN_DIRECTION.md §10](DESIGN_DIRECTION.md#10-open-points-for-the-pm-and-other-owners) item 11).
  2. **"Percakapan baru"** / "New conversations": the storage choice (§6.9) as the default, with nothing pre-selected: "Simpan di Catatanku" or "Jangan simpan", with the hint "Kamu bisa mengubahnya untuk setiap percakapan." / "You can change it for each conversation."
- **Required confirmations:** two separate checkboxes, one per required consent (UU PDP Pasal 22 asks for distinguishable consents; the final wording is pending legal review B-10):
  - "Aku sudah membaca Ketentuan & Privasi (versi …)" with a link to `/policies/terms_privacy`;
  - for guests only: "Aku mengerti cara penyimpanan mode tamu di atas".
- **States:**
  - busy: "Menyiapkan…" on the button;
  - errors:
    - a missing choice or confirmation: the group or field error pattern (§6.3), with an error summary at the top when more than one is missing;
    - `server_busy` (guest capacity): "Mode tamu sedang penuh. Coba lagi sebentar lagi, atau buat akun.";
    - `policy_outdated`: the page reloads the policy versions and asks again;
    - `age_band_not_enabled`: S27;
  - success: on to step 3 (or S04 for an account; or the guardian step for 16–17).

### Step 3: Balasan AI dan pemeriksa (AI replies and the checker) · `/start/ai`

Shown whenever AI replies are on (see the rule above the steps); skipped only when they are off.

- **Purpose:** the `ai_processing` consent for listening replies and the `ai_provider_anthropic` consent for the checker (D-029, D-048), each disclosed with its provider, place and purpose before it is given. The checker is asked here, not later, because in a wellbeing conversation replies that mention health are common (RR-8 flags them), and without a checker decision those replies would silently not be shown.
- **Controls:** two consent cards, one below the other, each with its own pair of equal-weight buttons (§6.9). Pressing a button records that card's answer at once and changes its status line; nothing is pre-selected.
  1. **"Balasan AI · Meta, diproses di Modal (Amerika Serikat)"** / "AI replies · Meta, processed on Modal (United States)". It lists:
     - what is sent ("hanya tulisan yang diperlukan untuk membalas; tanpa nama pengguna, dan dengan kontak serta nama yang kamu samarkan diganti"; "only the text needed to reply; no username, and with contact details and names you keep private replaced");
     - why;
     - how long the provider keeps it (pending verification);
     - what happens without it ("Kamu tetap bisa menulis dan menyimpan; Sela tidak membalas." / "You can still write and save; Sela won't reply.").

     Buttons: [Izinkan balasan AI] / [Allow AI replies] · [Tidak sekarang] / [Not now].
  2. **"Pemeriksa · Claude dari Anthropic, diproses di Amerika Serikat"** / "Checker · Claude by Anthropic, processed in the United States". In plain words: "Pemeriksa memeriksa balasan yang menyebut hal kesehatan, sebelum kamu melihatnya, dan memeriksa ringkasan serta hal yang diusulkan untuk diingat." / "The checker checks replies that mention health before you see them, and checks summaries and things suggested for Sela to remember." It lists what is sent ("hanya kalimat yang diperiksa dan kata-katamu yang terkait"; "only the sentences being checked and your related words"), the place, the provider's retention (deleted within 30 days by default; longer for content its safety systems flag; ADR-0003 §8.1), and the honest decline path: "Kalau tidak diizinkan: balasan yang menyebut hal kesehatan tidak ditampilkan, dan Sela tidak menyusun ringkasan atau mengusulkan hal untuk diingat dengan AI. Kamu tetap bisa menulis, menyusun sendiri, dan menambahkan sendiri." / "If you don't allow it: replies that mention health aren't shown, and Sela won't draft summaries or suggest things to remember with AI. You can still write, put summaries together yourself and add things yourself."

     Buttons: [Izinkan pemeriksa] / [Allow the checker] · [Tidak sekarang] / [Not now]. While card 1 is unanswered or answered "Tidak sekarang", card 2 says "Hanya dipakai jika balasan AI diizinkan." / "Only used if AI replies are allowed." and needs no answer.
  3. **Optional third card, adults only:** "Model cadangan · DeepSeek, diproses di Tiongkok" / "Backup model · DeepSeek, processed in China", with the full disclosure (ADR-0003 §8.1) and its own equal-weight pair "Izinkan model cadangan" / "Tidak". It is **off unless the person presses it**, and pressing nothing is the expected path. It is not shown for 16–17 or "unknown" (ADR-0003 §8.3: minors get no fallback generator).
- **Primary action:** "Mulai bercerita" / "Start talking" opens the room (`/talk/new`) with the defaults from step 2. It needs an answer on card 1 and, when card 1 is allowed, on card 2. A missing answer shows "Perlu diperbaiki: pilih Izinkan atau Tidak sekarang." in that card's status line (the alert icon, linked to its buttons by `aria-describedby`), and focus moves to the card's first button. A quiet link, "Lihat beranda dulu" / "See Home first", goes to Home without answering; an unanswered checker question is then asked in context later (S09).
- **Test doubles** (development and test, and the demo where D-045 allows them): the cards keep their real text, so tests check the real copy, and add the line "Di lingkungan uji ini, program uji yang membalas dan memeriksa; tidak ada yang dikirim ke Meta atau Claude." / "In this test environment, a test program replies and checks; nothing is sent to Meta or Claude." The same consent rules apply (ADR-0003 §10.1).
- **States:**
  - busy: "Menyimpan pilihanmu…" on the pressed button;
  - **error, the consent `PUT` fails:** the card's status line says "Pilihanmu belum tersimpan. Tulisanmu belum ada yang dikirim ke mana pun." / "Your choice wasn't saved yet. Nothing you've written has been sent anywhere." with [Coba lagi]; "Mulai bercerita" then offers [Lanjut tanpa balasan AI] (the room opens with AI off, and the choice can be made later in S22). `policy_outdated` reloads the card and asks again;
  - success: each card shows "Diizinkan" or "Tidak diizinkan untuk sekarang" with [Ubah]; "Mulai bercerita" opens the room.
- **Not asked here:** memory, which is asked the first time Sela could propose remembering something (adults; never in context for 16–17 or "unknown"). Asking it at its moment of need keeps onboarding short (brief §5: "tanpa profil panjang") and gives it its context. **No consent is ever asked during an outage** (ADR-0003 §8).

The intent picker is not a step: it sits, optional, in the room's conversation strip (S09).

### Whole flow

- **Recovery:**
  - Back always works.
  - Closing the tab before step 2 leaves nothing on the server.
  - A lost guest response leaves an unused guest that the purge job removes (API_SPEC_V3 §5). The UI retries once and then offers to start again.
- **Keyboard and focus order:** on each step,
  1. shell;
  2. back link;
  3. h1;
  4. the step's controls in reading order;
  5. the primary action.

  Radios use arrow keys. Focus moves to `<main>` on each new step, which announces the step's title.
- **Layout:**
  - 320px: one column, the option panels stacked, the primary button full width at the end of the content (not sticky).
  - Wide: a centred column of 36rem; the progress line under the h1.
- **API:**
  - `GET /api/v3/auth/options` and `GET /api/v3/policies` (v3 spec);
  - `POST /api/v3/auth/guest` (v3 spec), with `locale`, `ageBand`, `termsPolicyVersion` and `guestNoticeVersion`;
  - the default for new conversations: a profile field (planned P3; until then kept on this device);
  - `PUT /api/v3/me/consents/ai_processing`, `…/ai_provider_anthropic` and `…/ai_provider_deepseek` (purposes reserved; planned P3/P5);
  - `GET /api/v2/ai/status` (v2) until `/api/v3/ai` exists, to know whether AI replies are on and whether a test double stands in.

| Element | ID | EN |
|---|---|---|
| Step 1 h1 | Sebelum mulai | Before you start |
| Age legend | Berapa usiamu? | How old are you? |
| Step 2 h1 | Di mana datamu disimpan? | Where is your data kept? |
| Conversation default | Percakapan baru: simpan di Catatanku, atau jangan simpan? | New conversations: save to My notes, or don't save? |
| Step 3 h1 | Balasan AI dan pemeriksa | AI replies and the checker |

## S03 Sign in

**Route** `/signin` · **Access** public · **Title** "Masuk – Sela"

- **Purpose:** return to an account.
- **Primary action:** "Masuk" / "Sign in" sends `POST /api/v3/auth/login`.
- **Secondary actions:**
  - "Lupa kata sandi? Pakai kode pemulihan" / "Forgot your password? Use a recovery code" leads to S05;
  - "Belum punya akun? Mulai" / "No account yet? Get started" leads to S02.
- **Content:**
  - Username (`autocomplete="username"`).
  - Password (`autocomplete="current-password"`, with a "Tampilkan" toggle; paste allowed).
  - The checkbox "Ingat perangkat ini" / "Remember this device", with the hint "Tetap masuk sampai 7 hari tanpa aktivitas. Jangan pilih ini di perangkat bersama." / "Stay signed in for up to 7 days of inactivity. Don't choose this on a shared device." (ADR-0002 §8).
  - No CAPTCHA, puzzle or "type character 3 of your password" (WCAG 3.3.8).
- **States:**
  - busy: "Memeriksa…" (hashing can take up to a few seconds);
  - error, `invalid_credentials`: "Nama pengguna atau kata sandi tidak cocok." / "The username or password does not match." The server cannot say which field is wrong, so this uses the **form-level error region** above the fields ([DESIGN_DIRECTION.md §6.3](DESIGN_DIRECTION.md#63-inputs)): it is present empty from page load with `role="alert"`, and receives the alert icon, "Perlu diperbaiki:" and the text. Both fields get `aria-invalid="true"` and list the region in `aria-describedby`. **Both values are kept** (the password stays masked; "Tampilkan" lets the person check it), because the mistake may be the username and a 15–256-character password is costly to retype by hand. **Focus moves to the username field**, so a screen-reader user hears the alert and then the username with the same message, and can Tab on to the password;
  - error, `password_login_disabled`: a notice with a link to S05;
  - error, `rate_limited`: "Terlalu banyak percobaan. Coba lagi dalam sekitar 2 menit.";
  - error, `server_busy`: retry;
  - success: on to `/home`, or to the page the person was sent from (a same-origin path only, never an external URL; the v1 safe-`next` rule).
- **Recovery:**
  - The typed username and password are kept after an error.
  - From S27 "session ended", the person returns to the page they were on, with their in-tab draft intact.
- **Keyboard and focus order:**
  1. shell;
  2. h1;
  3. the form-level error region (not focusable; read through the alert and the fields' descriptions);
  4. Username;
  5. Password;
  6. Tampilkan;
  7. Ingat perangkat ini;
  8. Masuk;
  9. Lupa kata sandi;
  10. Belum punya akun.

  Enter in any field submits.
- **Layout:**
  - 320px: one column, full-width fields and button.
  - Wide: a 28rem centred card.
- **API:** `POST /api/v3/auth/login` (v3 spec).

| Element | ID | EN |
|---|---|---|
| h1 | Masuk ke akunmu | Sign in to your account |
| Password hint | Kamu boleh menempel kata sandi dari pengelola kata sandi. | You can paste your password from a password manager. |

---

## S04 Create an account and recovery codes

**Routes** `/start/account`, `/start/codes` (also reached from S06 as `/settings/account/upgrade`) · **Access** public · **Title** "Buat akun – Sela"

- **Purpose:** an account that works on any device, without email (D-028).
- **Primary action:** "Buat akun" / "Create account" sends `POST /api/v3/auth/register`. Then "Lanjut" after saving the codes.
- **Secondary actions:**
  - "Kembali ke pilihan penyimpanan";
  - on the codes step: [Salin semua] [Unduh .txt] [Cetak].
- **Content:**
  - **Username,** with its rules **shown before typing** (3.3.2): "3–32 karakter: huruf a–z, angka, titik, garis bawah, atau tanda hubung. Jangan pakai nama asli, email, atau nomor telepon." / "3–32 characters: letters a–z, digits, dot, underscore or hyphen. Don't use your real name, email or phone number."
  - **Password** (`autocomplete="new-password"`), with the rule up front: "Minimal 15 karakter. Coba gabungan beberapa kata yang mudah kamu ingat, misalnya 'pohon mangga di depan rumah'." / "At least 15 characters. Try a few words you can remember, like 'mango tree by the front door'." A plain progress line ("11 dari 15 karakter") updates on blur, not on every keystroke.
  - **Remember this device.**
  - The age band and language carried from S02 (shown, with "Ubah"; not asked again).
  - The terms checkbox.
  - **Codes step:**
    - 10 codes as an ordered list;
    - an explanation: "Kode ini satu-satunya cara masuk jika kamu lupa kata sandi. Setiap kode hanya bisa dipakai sekali. Sela tidak bisa menampilkannya lagi." / "These codes are the only way back in if you forget your password. Each works once. Sela can't show them again.";
    - the required checkbox "Aku sudah menyimpan kode ini di tempat aman";
    - [Lanjut].
- **States:**
  - errors, placed under their field:
    - `username_invalid`, `username_taken` ("Nama pengguna ini sudah dipakai. Pilih yang lain."), `password_too_short`, `password_too_long` and `password_weak`. Each has its API text plus a suggestion;
    - with more than one error, an error summary at the top that links to the fields;
  - if the username contains crisis language (the safety screen runs on every free-text field, as in v2), the crisis panel (§6.15) is shown and the person is asked to pick another username; nothing is stored;
  - success: on to the codes step.
- **Recovery:** if the register response is lost, a retry gets `username_taken`. The UI then says "Akunmu mungkin sudah dibuat. Coba masuk dengan kata sandi yang sama; kamu bisa membuat kode pemulihan baru setelahnya." (API_SPEC_V3 §5).
- **Keyboard and focus order:**
  1. h1;
  2. Username;
  3. Password;
  4. Tampilkan;
  5. Ingat perangkat ini;
  6. carried values with Ubah;
  7. the terms checkbox;
  8. Buat akun.

  On the codes step, focus goes to the h1, then the list (not focusable), Salin semua, Unduh, Cetak, the checkbox and Lanjut.
- **Layout:**
  - 320px: the codes in one column in the mono stack at `--font-size-3`.
  - Wide: the codes in two columns.
  - Print stylesheet: the codes only, the date, and the line "Simpan di tempat aman. Setiap kode sekali pakai."
- **API:**
  - `POST /api/v3/auth/register` (v3 spec);
  - `POST /api/v3/me/recovery-codes/acknowledge` (v3 spec), sent when the checkbox is confirmed and Lanjut is pressed.

---

## S05 Recovery

**Route** `/recover` · **Access** public · **Title** "Pulihkan akun – Sela"

- **Purpose:** set a new password with a recovery code (ADR-0002 §6).
- **Primary action:** "Pulihkan dan masuk" / "Recover and sign in".
- **Secondary action:** "Kembali ke Masuk".
- **Content:**
  - Username.
  - Recovery code (`autocomplete="one-time-code"`). Paste is allowed; case, spaces and hyphens are ignored, and the look-alikes O and 0, and I, L and 1, are read as the same (ADR-0002 §6). The hint says so (copy table below).
  - New password, with the rule shown.
  - What happens: "Semua perangkat lain akan dikeluarkan." / "All other devices will be signed out."
- **States:**
  - errors:
    - `recovery_failed` (one uniform text): "Nama pengguna atau kode pemulihan tidak cocok, atau kode itu sudah pernah dipakai." It uses the form-level error region above the fields, as in S03: the username and code fields get `aria-invalid="true"` and reference it; **every typed value is kept** (the code too, so a typo can be fixed), and focus moves to the code field;
    - the password errors are checked first, under the new-password field, and no code is used up;
    - `rate_limited`;
  - success: a panel saying "Kata sandimu sudah diganti. 9 kode pemulihan tersisa." With 3 or fewer left: "Sebaiknya buat kode baru sekarang." [Buat kode baru] [Lanjut ke beranda].
- **Recovery:** if the response is lost, the password already changed, so the UI says "Coba masuk dengan kata sandi baru."
- **Keyboard and focus order:** h1, the form-level error region (not focusable), Username, Kode, Kata sandi baru, Tampilkan, Pulihkan.
- **Layout:** as S03.

| Element | ID | EN |
|---|---|---|
| h1 | Pulihkan akunmu | Recover your account |
| Code hint | Huruf besar/kecil, spasi, dan tanda hubung tidak berpengaruh. Huruf O dan angka 0 dianggap sama, begitu juga huruf I, L, dan angka 1. | Upper or lower case, spaces and hyphens don't matter. The letter O and the digit 0 count as the same, and so do the letters I and L and the digit 1. |
| Uniform error | Nama pengguna atau kode pemulihan tidak cocok, atau kode itu sudah pernah dipakai. | The username or recovery code does not match, or that code was already used. |
| Few codes left | Sisa 3 kode pemulihan. Sebaiknya buat kode baru sekarang. | 3 recovery codes left. It's a good idea to make new ones now. |

- **API:** `POST /api/v3/auth/recovery` (v3 spec).

---

## S06 Guest upgrade

**Route** `/settings/account/upgrade` · **Access** owner (guest) · **Title** "Simpan ke akun – Sela"

- **Purpose:** keep a guest's data beyond the 30-day limit and open it on other devices, with the same data and the same `user.id` (ADR-0002 §7).
- **Primary action:** "Simpan ke akun" / "Save to an account".
- **Secondary action:** "Nanti saja" / "Not now", back to the previous page.
- **Content:**
  - "Semua yang sudah kamu simpan tetap ada. Setelah ini, kamu masuk dengan nama pengguna dan kata sandi, dan datamu tidak lagi dihapus otomatis." / "Everything you've saved stays. After this you sign in with a username and password, and your data is no longer deleted automatically.";
  - the same fields as S04, then the codes step;
  - why it matters on a shared device: "Tanpa akun, siapa pun yang memakai browser ini bisa membuka datamu. Dengan akun, kamu bisa keluar dari akun." / "Without an account, anyone using this browser can open your data. With one, you can sign out."
- **States:**
  - errors: as S04, plus `not_guest` ("Akun ini sudah terdaftar.") and `policy_outdated` (accept the new terms first, inline);
  - success: the codes step, then back to where the person came from, with the banner "Datamu sekarang tersimpan di akun." / "Your data is now saved to your account."
- **Recovery:** if the response is lost, the account exists and the guest cookie was rotated, so the UI says "Masuk dengan nama pengguna dan kata sandi yang baru kamu buat."
- **Keyboard, layout:** as S04.
- **API:** `POST /api/v3/auth/guest/upgrade` (v3 spec); then acknowledge the codes.

---

## S07 Home

**Route** `/home` · **Access** owner · **Title** "Beranda – Sela"

- **Purpose:** a calm place to start. It answers "what can I do now?" and "is anything waiting for me?" It is **not** a dashboard: no counts of days, no scores, no charts, no "you haven't checked in today" (D-036).
- **Primary action:** "Mulai bercerita" / "Start talking" opens S09 (a new conversation) with the person's defaults; nothing is asked first.
- **Secondary actions:**
  - "Tulis catatan" / "Write a note" opens S10;
  - "Cek perasaan" / "Check in" opens S11. **Home has no inline check-in** (design decision, round 2): a partial form on Home would make it a dashboard of inputs, and the full check-in is one tap away;
  - "Lanjutkan percakapan" (continue a conversation), shown for a saved conversation with activity in the last 12 hours that has not ended (conversations end automatically after 12 hours without activity; planned P3), and for the **unsaved conversation still held in this tab** ("Percakapan yang sedang berjalan (tidak disimpan)");
  - links on the active-shares and latest-entries cards.
- **Content and data, in reading order.** Above the h1 is the notices region, as on every page: the crisis follow-up banner when it applies and previews are not hidden (§6.11), the guest-expiry banner and any others. The crisis banner therefore sits in the same place relative to the content here and in the room (3.2.6). Then:
  1. The h1 **"Beranda" / "Home"**, which describes the page for heading navigation (2.4.6).
  2. The lead, a greeting with no name: "Selamat sore. Apa yang ingin kamu lakukan sekarang?" / "Good afternoon. What would you like to do now?" Usernames can identify a person on a shared screen, so they are not shown.
  3. The primary and two secondary actions.
  4. "Lanjutkan" (only if relevant): "Percakapan Sabtu, 24 Okt · Merapikan pikiran" [Lanjutkan percakapan]. With previews hidden: "Percakapan Sabtu, 24 Okt" only.
  5. "Sedang dibagikan" (only if a share is active): each share shows its recipient label, a status chip (§6.8) and the expiry, and links to S18. **With previews hidden**, it shows "1 ringkasan sedang dibagikan" and the expiry, with no recipient label or title.
  6. "Terbaru di Catatanku" / "Latest in My notes": the 3 latest timeline entries (type, time, preview unless previews are hidden) and "Lihat semua".
  7. A retention line for guests: "Mode tamu · dihapus otomatis jika 7 hari tidak aktif, paling lambat 25 Nov 2026 · [Simpan ke akun]".
  8. An optional actions-required card, for example `confirm_age_band` ("Apakah usiamu masih 16–17 tahun?").
- **States:**
  - empty (first visit): items 4 to 6 are absent. Under the actions: "Belum ada yang tersimpan. Apa pun yang kamu simpan akan muncul di Catatanku." / "Nothing saved yet. Anything you save will appear in My notes."
  - loading: the h1, lead and actions render immediately (no data needed), and the cards show "Memuat…" after 400ms;
  - error: each card fails on its own ("Belum bisa memuat yang terbaru. [Coba lagi]"), and the actions keep working;
  - success: not applicable.
- **Recovery:** offline, the actions still open the room (sending will wait), and the help page works.
- **Keyboard and focus order:**
  1. shell (help, Quick exit, Menu, the navigation);
  2. the notices region: the crisis follow-up banner's "Lihat semua bantuan" and "Tutup", then any other banner's links;
  3. h1 "Beranda";
  4. Mulai bercerita;
  5. Tulis catatan;
  6. Cek perasaan;
  7. Lanjutkan percakapan;
  8. the share links;
  9. the latest-entry links;
  10. Lihat semua;
  11. Simpan ke akun.
- **Layout:**
  - 320px: one column. Mulai bercerita is full width. Tulis catatan and Cek perasaan share a row only when both fit (they wrap by available space; about 313px are needed, so at 320px each takes a full row, and on 360px phones at 100% they sit side by side). The cards are stacked.
  - Wide: the main column holds the h1, lead and actions, with "Lanjutkan" and "Terbaru di Catatanku" below; "Sedang dibagikan" is a right-hand column at 75em and over.
- **API:**
  - `GET /api/v3/me` (v3 spec);
  - `GET /api/v3/conversations?limit=1&state=open&activeWithin=12h`, `GET /api/v3/shares?state=active` and `GET /api/v3/timeline?limit=3` (all planned).

## S08 Conversations (Talk)

**Route** `/talk` · **Access** owner · **Title** "Ruang cerita – Sela"

- **Purpose:** start a new conversation or return to a saved one.
- **Primary action:** "Mulai bercerita" / "Start talking" opens `/talk/new`.
- **Secondary actions:** open a conversation; "Lihat di Catatanku" (the same items in the timeline).
- **Content:**
  - the h1 "Ruang cerita" with the lead "Ceritakan apa saja. Kamu yang menentukan apakah percakapan disimpan." / "Talk about anything. You decide whether a conversation is saved.";
  - a list of saved conversations, newest first, each showing:
    - the date of the last activity ("Terakhir aktif Sabtu, 24 Okt 2026" / "Last active Saturday, 24 Oct 2026");
    - the optional title the person gave (hidden with previews);
    - the intent chip, if any;
    - the number of messages.

    No "ended / not ended" state is shown: most people close the tab rather than press "Akhiri percakapan", and "Belum diakhiri" would read like unfinished homework. Conversations end automatically after 12 hours without activity (planned P3), and any saved conversation can be continued;
  - the current unsaved conversation in this tab, if any, at the top: "Percakapan yang sedang berjalan (tidak disimpan)". It stays there while its hold lasts, also after the person opened Help or Privasi & data (S09). Unsaved conversations are never listed otherwise.
- **States:**
  - empty: "Belum ada percakapan tersimpan. Percakapan yang kamu simpan akan muncul di sini." with the primary action;
  - loading;
  - error: "Belum bisa memuat percakapanmu." [Coba lagi]. Mulai bercerita keeps working;
  - success: not applicable.
- **Recovery:** pagination with "Muat lebih banyak"; Back from a conversation keeps the loaded pages and the scroll position ([DESIGN_DIRECTION.md §8](DESIGN_DIRECTION.md#8-implementation-notes-for-the-frontend-planned-owned-by-the-frontend-engineer)). A failed load keeps what was already shown.
- **Keyboard and focus order:** h1, Mulai bercerita, the list links in order, Muat lebih banyak.
- **Layout:**
  - 320px: list rows of at least 64px, title wrapping.
  - Wide: the list within `--measure-wide`.
- **API:** `GET /api/v3/conversations?limit=&cursor=` (planned).

---

## S09 Conversation room

**Routes** `/talk/new`, `/talk/:id` · **Access** owner · **Title** "Percakapan – Sela" (with the date and the optional title for saved conversations; neutral when previews are hidden)

The core of the product (brief §5; D-034). The component specifications are in [DESIGN_DIRECTION.md §6.5–6.6 and §6.15](DESIGN_DIRECTION.md#65-conversation-transcript-messages-composer). Two rules shape every state below:

- **A reply is shown only after release** (ADR-0003 §5): the server runs the guards, at most one same-provider regeneration and, when flagged, the verifier, before the browser receives any text. Nothing is shown and then taken back.
- **The transcript log is silent** (`aria-live="off"`). Every state below has one fixed announcement, listed in [DESIGN_DIRECTION.md §6.5](DESIGN_DIRECTION.md#65-conversation-transcript-messages-composer) and tested by ST-1 to ST-22.

- **Purpose:** let a person tell what is going on and be listened to. Replies fit the context. Nothing is forced on them. They can correct what Sela understood, choose what is kept and whether memory is used, end at any time, and find help inline when it is needed.
- **Primary action:** "Kirim" / "Send" (Ctrl or Cmd+Enter). The label never changes, even when no reply will follow.
- **Secondary actions:**
  - "Akhiri percakapan" / "End conversation";
  - "Hentikan balasan" (Stop reply, while a reply is pending);
  - "Kurang tepat?" (Not quite right?, on each reply);
  - "Apa yang Sela tangkap?" (What did Sela understand?, in Detail);
  - change the intent, the storage choice, or "Pakai yang Sela ingat di percakapan ini";
  - "Ke balasan terakhir" (go to the last reply);
  - "Bacakan" (read aloud, if enabled);
  - mic (if dictation is enabled);
  - Detail: "Beri judul" (give a title), "Hapus percakapan" (delete the conversation), "Yang Sela ingat" (what Sela remembers), "Butuh bantuan sekarang".
- **Content and data** (classes and statuses from [DESIGN_DIRECTION.md §6.7.1](DESIGN_DIRECTION.md#671-record-types-class-statuses-and-transitions)):
  - **The person's messages:** `user_original`; correction messages: `user_correction`.
  - **Sela's replies:** class `ai_summary` (pending confirmation, [DESIGN_DIRECTION.md §10](DESIGN_DIRECTION.md#10-open-points-for-the-pm-and-other-owners) item 17) with status `unreviewed`, or `corrected` after a correction. **In the transcript a reply shows only its author line, and "Dikoreksi" after a correction** (the exception in DESIGN_DIRECTION §6.7): no badge and no "Belum ditinjau" chip under every reply. The visible author line always names the generator, and the verifier when one checked it, from the reply's stored provenance: "Sela · balasan AI (Meta) · diperiksa oleh Claude", or "… · diperiksa oleh program uji (bukan AI)" in a test environment (ADR-0003 §10, §10.1). "Tentang balasan ini" (About this reply) shows the badge "Tulisan AI" with its review status, then the details:
    - the provider (Meta / DeepSeek cadangan / program uji);
    - the model and served version;
    - the route (utama / cadangan);
    - checks ("Diperiksa: tidak ada klaim kesehatan" or "Diperiksa oleh Claude: didukung");
    - memory: "Memakai 2 hal yang Sela ingat · Lihat" or "Tidak memakai yang Sela ingat";
    - the time.
  - **Reflection items:** class `unconfirmed_interpretation` ("Tafsiran Sela") with their own review status (Belum ditinjau → Dikonfirmasi or Ditolak); an edited item becomes a new version of class `user_correction` with status Dikoreksi.
  - **Crisis panel, pause line and support line:** static content and verified directory data.
  - **Storage status chip;** the strip line with intent, storage and memory.

### Room states

| State | What the person sees | Actions |
|---|---|---|
| **New conversation** (the default was chosen in onboarding or S22) | The conversation strip states the defaults as sentences: "Percakapan ini akan disimpan di Catatanku · Ubah" (or "tidak disimpan"), and, when memory is on, "Pakai yang Sela ingat di percakapan ini: Ya · Tidak". The intent chips are optional. Beside it the room intro: "Tulis apa saja yang ada di pikiranmu. Tidak perlu rapi. Sela akan mendengarkan dan membalas dengan singkat." and the small line "Sela adalah AI, bukan psikolog. Kalau kamu dalam bahaya sekarang, tekan Butuh bantuan sekarang di atas." **Nothing is asked before the first message**, and the text area works immediately | Type; change storage, memory or intent |
| **New, no storage default** (an account created before the default existed) | The storage question as two option rows, nothing selected. Kirim with nothing selected keeps the text and shows the **group error** in the legend, "Perlu diperbaiki: pilih dulu, simpan atau jangan simpan." / "Needs fixing: choose first, save or don't save."; the fieldset (`role="radiogroup"`) gets `aria-invalid="true"`, and focus moves to the first option ([DESIGN_DIRECTION.md §6.3](DESIGN_DIRECTION.md#63-inputs)). The answer becomes the default | Choose; send |
| **Ongoing** | The transcript (§6.5). The strip collapsed to one line: "Didengarkan saja · Disimpan · Memori dipakai · Ubah" | Send, correct, end |
| **Sending** | The person's message appears at once with "Mengirim…". It is not read back | — |
| **Safety screening** (every message, deterministic, before storage and before any model, D-035) | Invisible when nothing matches | — |
| **Contact details in the message** | **Sending is not gated.** The message is sent at once; the server masks contact details for AI as always. On the sent message, a non-blocking notice names the kind, never the detail: "Nomor telepon disamarkan untuk AI." / "Phone number hidden from AI." For an ID (NIK) or card number, the stored copy is masked too: "Nomor KTP disamarkan untuk AI dan di Catatanku." Status: the notice's first sentence, once | [Samarkan juga di Catatanku] (saved conversations); for NIK and card numbers [Tampilkan seperti yang kutulis] |
| **Crisis match** (lexicon, RR-1) | The message stays visible and is never erased. Right after it, the **crisis panel** (§6.15): "Kamu tidak sendirian", up to three verified services and "Lihat semua bantuan". `#sr-alert` says once: "Bantuan ditampilkan di bawah pesanmu." AI replies **pause for this conversation**, and **the pause line appears at once** directly above the composer, with [Ke bantuan]. If the reader is not at the bottom, the new-content button says "Bantuan ditampilkan di bawah pesanmu ↓". The rest of the app stays open. The message is stored only if this conversation is saved | [Lanjutkan bercerita dengan Sela] resumes replies; [Akhiri percakapan]; [Tutup bantuan ini]; the help links; [Ke bantuan] on the pause line moves focus to the panel heading |
| **Generator risk `crisis`** (RR-7) | Exactly as a crisis match: the same panel after the person's message and the same pause. The generated output is **not shown and not stored**, and nothing suggests a reply existed | As above |
| **Verifier `safety_signal`** (VC-8) | As a crisis match; the output is withheld | As above |
| **Replies paused, panel closed** | After "Tutup bantuan ini", the panel goes and the **persistent pause line** stays above the composer: "Balasan Sela dijeda di percakapan ini. Kamu tetap bisa menulis." / "Sela's replies are paused in this conversation. You can still write." Its [Ke bantuan] becomes [Lihat semua bantuan]. Focus moves to the text area. The line returns when the conversation is reopened while paused | [Lanjutkan bercerita dengan Sela] [Lihat semua bantuan] |
| **Message sent while paused** | Kept according to the storage choice, with the meta line "tanpa balasan (dijeda)" / "no reply (paused)". Still screened. Status: "Terkirim." | — |
| **A later match** (while paused or after resuming) | The panel again after that message, and replies pause again. If a full panel was shown within the person's last 5 messages, the **condensed form**: "Bantuan tetap ada. Layanan yang tadi ditampilkan bisa kamu hubungi kapan saja." | [Lihat semua bantuan] [Tampilkan layanan lagi] [Lanjutkan bercerita dengan Sela] [Tutup bantuan ini] |
| **Waiting for a reply** | The pending block "Sela sedang menulis…" after the person's message (static text; no animation). Kirim is unavailable with the hint "Tunggu balasan Sela, atau hentikan dulu." | Hentikan balasan |
| **Cold start** (Modal scaling from zero) | The pending block reads "Sela sedang bersiap, bisa sampai sekitar 1 menit." (from `coldStartEstimateMs`). Static text, updated at most once per second; no progress bar | Hentikan balasan |
| **Still waiting** (20 s after the request) | "Masih menulis. Kadang ini butuh lebih lama." | Hentikan balasan; it keeps waiting |
| **Guard rejection, one regeneration** (RR-5) | **Nothing changes:** "Sela sedang menulis…" stays during the one same-provider regeneration | Hentikan balasan |
| **Rejected again** | The no-reply notice: "Sela belum bisa membalas pesan ini dengan baik, jadi tidak menampilkan balasan. Tulisanmu tetap di sini. Sela akan mencoba membalas pesanmu berikutnya." / "Sela couldn't reply to this message well, so it isn't showing a reply. Your writing is still here. Sela will try again with your next message." No fallback provider is tried (RR-5) | Keep writing; [Akhiri percakapan] |
| **Reply checked** (the deterministic health-claim detector flagged it; D-029) | "Sela sedang memeriksa balasan ini…" for up to the verifier timeout | Hentikan balasan |
| **Released** | The reply article (§6.5), with the generator and any checker on its author line. Under full motion its sentences fade in over about 1.5 s; under reduced motion it appears whole | Kurang tepat?, Tentang balasan ini, Bacakan |
| **Released with risk `elevated`** (RR-7) | The reply, then the static support line: "Kalau terasa berat, bantuan bisa kamu hubungi kapan saja." [Lihat semua bantuan]. Announced with the reply as one line | — |
| **Withheld: the check did not support it** (VC-8) | "Sela tidak menampilkan balasan ini, karena pemeriksaan menemukan bagian yang belum tentu sesuai dengan ceritamu. Ceritamu tetap aman di sini. Kamu bisa melanjutkan." / "Sela isn't showing this reply, because the check found parts that may not match what you said. Your words are safe here. You can carry on." | Keep writing |
| **Withheld: checker not allowed** (RR-11, "no consent"; the person answered "Tidak sekarang" or withdrew it) | Neutral wording, never "you haven't allowed it": "Sela tidak menampilkan balasan ini. Balasan yang menyebut hal kesehatan perlu diperiksa dulu, dan pemeriksa (Claude) belum diizinkan. Ceritamu tetap aman di sini." / "Sela isn't showing this reply. Replies that mention health need a check first, and the checker (Claude) isn't allowed yet. Your words are safe here." | [Pengaturan AI] (a neutral link to S22 `#ai`); keep writing |
| **Withheld: checker never answered** (a profile from before the onboarding card, or onboarding left through "Lihat beranda dulu") | The same first sentence, then the **checker consent card in context** inside the notice (§6.9): "Pemeriksa · Claude dari Anthropic, diproses di Amerika Serikat", what it checks, what is sent, and what happens without it. Asked once per conversation | [Izinkan pemeriksa] [Tidak sekarang] [Jangan tanya lagi], of equal weight. After "Izinkan pemeriksa": status "Pemeriksa diizinkan." and a quiet [Minta Sela membalas lagi] (a new reply; the withheld one is never shown). After "Tidak sekarang" or "Jangan tanya lagi": the neutral notice of the row above |
| **Withheld: checker unavailable** (RR-11, "service unavailable") | "Sela tidak menampilkan balasan ini. Balasan yang menyebut hal kesehatan perlu diperiksa dulu, dan pemeriksanya (Claude) sedang tidak tersedia. Ceritamu tetap aman di sini." / "Sela isn't showing this reply. Replies that mention health need a check first, and the checker (Claude) isn't available right now. Your words are safe here." | [Coba lagi]; [Pengaturan AI]; keep writing |
| **Discarded at commit** (RR-13: the conversation, a consent or the crisis state changed while Sela wrote) | "Sela tidak menampilkan balasan ini karena ada yang berubah saat Sela menulis, misalnya izin AI." | Keep writing |
| **Stopped before release** | Nothing of the reply is shown or stored. The pending block becomes the meta line "Balasan dihentikan sebelum ditampilkan" / "Reply stopped before it was shown". Focus moves to the text area | Send another message |
| **Fallback used** (Meta unavailable and DeepSeek consent granted) | The author line: "Sela · balasan AI (DeepSeek, cadangan)". "Tentang balasan ini" adds "diproses di Tiongkok" and why the main model was unavailable | — |
| **AI unavailable, no fallback consent** | A notice in the transcript: "Sela belum bisa membalas sekarang. Tulisanmu tetap di sini (dan tersimpan, kalau kamu memilih menyimpan)." / "Sela can't reply right now. Your writing stays here (and is saved, if you chose to save it)." | [Coba lagi] [Lanjut menulis tanpa balasan] [Pengaturan AI] (a neutral link to S22 `#ai`). The backup model is never offered or promoted inside the outage (ADR-0003 §8) |
| **No AI consent, or AI off on the server** (including any environment where `ai_processing` is not yet active, outside development and test) | The room intro says "Balasan AI sedang mati. Kamu tetap bisa menulis; Sela tidak membalas." **No live provider is called.** The button still reads "Kirim"; each message gets the meta line "tanpa balasan (balasan AI mati)", and the status says "Terkirim." | [Nyalakan balasan AI] (links to S22 `#ai`; absent when AI is off on the server) |
| **Reflection offered** | The "Sela menangkap…" card (§6.6) after a reply, only for the intents that ask for it, or on request. Its line: "Ini tafsiran Sela, belum tentu benar. Kalau mau, periksa satu per satu." Status: "Sela menangkap 2 hal di bawah balasan." | Benar / Ubah / Bukan, hapus; add |
| **Memory proposal** (a **saved** conversation, the `personal_memory` consent, after a verifier pass; adults, or 16–17 and "unknown" only after they turned memory on in S13) | Inside the card: "Boleh Sela mengingat ini untuk percakapan berikutnya?" with the generator line "Ditulis dengan bantuan AI (Meta) · diperiksa oleh Claude". Without the memory consent, the first time (adults only): the consent card with three equal buttons. **Unsaved conversation:** no proposal, and the card says "Sela tidak mengusulkan untuk mengingat apa pun dari percakapan yang tidak disimpan." No permitted checker: "… karena pemeriksa (Claude) belum diizinkan" or "… karena pemeriksanya sedang tidak tersedia" | [Ingat] [Ubah dulu] [Jangan]; consent: [Izinkan Sela mengingat] [Tidak sekarang] [Jangan tanya lagi]. After "Tidak sekarang", not asked again in this conversation |
| **Something new arrives while the reader is scrolled up** (a reply, the crisis panel, a no-reply notice, a card) | The new-content button directly above the composer names it: "Pesan baru dari Sela ↓", "Bantuan ditampilkan di bawah pesanmu ↓", "Sela tidak menampilkan balasan ↓", "Sela menangkap 2 hal ↓" (help first when several arrive) | Pressing it scrolls to the item and moves focus to its heading or article, with the focus ring |
| **Offline** | The offline banner. Send gives "Belum terkirim — tidak ada koneksi" with "Kirim lagi" | Kirim lagi |
| **Session ended mid-conversation** | Dialog: "Sesimu sudah berakhir. Masuk lagi untuk melanjutkan; tulisanmu yang belum terkirim masih di layar ini." For guests: "Data tamu tidak bisa dibuka lagi dari browser ini." | [Masuk] |
| **Absolute session limit near** (accounts) | The dialog of [DESIGN_DIRECTION.md §6.12](DESIGN_DIRECTION.md#612-dialogs-sheets-and-inline-confirmation), 10 minutes before; the draft stays | [Masuk lagi] [Nanti saja] |
| **End, saved conversation** ("Akhiri percakapan") | One action, no "are you sure". AI stops. The closing panel says what happened: "Percakapan selesai. Tersimpan di Catatanku." with the line "Penyedia AI memproses tulisanmu untuk membalas; lihat Privasi & data." Optional, offered once, nothing pre-selected. For the intent "Menyiapkan konsultasi": also [Buat ringkasan dari percakapan ini] | [Lihat apa yang Sela tangkap] [Cek perasaan] [Tulis catatan] [Kembali ke beranda] |
| **End, unsaved conversation** | One action. AI stops. The transcript is **kept in tab memory, and in the server's hold, until the person closes the tab, starts a new conversation or uses Quick exit, or the hold times out after a warning** (§6.9). Opening Help, Privasi & data or any other place does not discard it, and "Lanjutkan percakapan" on Home and in Cerita brings the person back to this panel. The panel says: "Percakapan selesai. Percakapan ini tidak disimpan. Sela membuangnya saat kamu menutup tab ini, memulai percakapan baru, atau memakai Keluar cepat. Kamu bisa membuka bantuan atau Privasi & data dulu tanpa kehilangannya." / "Conversation ended. It isn't saved. Sela discards it when you close this tab, start a new conversation or use Quick exit. You can open help or Privacy & data first without losing it." plus "Penyedia AI memproses tulisanmu untuk membalas dan bisa menyimpannya sementara sesuai kebijakannya; lihat Privasi & data." After a discard, the next page's status says "Percakapan tadi sudah dibuang." | [Lanjutkan percakapan] (nothing lost) [Simpan percakapan ini] (the storage dialog with "Simpan juga pesan sebelumnya", §6.9) [Lihat apa yang Sela tangkap] (the card is labelled "Tidak disimpan") [Cek perasaan] [Tulis catatan] [Kembali ke beranda] [Buang sekarang] |
| **Unsaved hold about to time out** (no activity for the P3 hold time) | A dialog, at least 2 minutes before: "Percakapan yang tidak disimpan ini akan dibuang dalam sekitar 2 menit karena tidak ada aktivitas." with the help link. If it times out anyway, the transcript stays visible in the tab, read only: "Percakapan ini sudah dibuang dari server Sela. Isinya masih terlihat di tab ini sampai kamu meninggalkannya." The unsent draft stays | [Tetap buka] (restarts the hold; any number of times) [Simpan percakapan ini] [Buang sekarang]; after a timeout: [Mulai percakapan baru] |
| **Starting a new conversation while an unsaved one is held** | An inline confirm on "Mulai bercerita": "Percakapan yang tidak disimpan tadi akan dibuang." | [Buang dan mulai baru] [Simpan dulu] [Batal] |
| **Reopening a saved conversation** | The transcript, with [Lanjutkan percakapan ini]. If replies were paused, the pause line. Conversations that ended automatically after 12 hours without activity (planned P3) look the same; nothing says "unfinished" | Continue |
| **Delete** (Detail) | The deletion confirm that lists derived items ([DESIGN_DIRECTION.md §6.12](DESIGN_DIRECTION.md#612-dialogs-sheets-and-inline-confirmation)): "Kutipan dari percakapan ini ada di 2 ringkasan (1 sedang dibagikan ke 'Psikolog Puskesmas') dan 1 hal yang Sela ingat." With nothing derived: "Hapus percakapan ini dan semua yang Sela tangkap darinya?" | [Hapus percakapan dan semua turunannya] [Hapus percakapannya saja] [Batal], the two destructive choices in the same Confirm-destructive variant and size, focus on the confirm sentence first; or [Hapus percakapan] [Batal] |

### Storage and memory changes mid-conversation

The storage dialogs are specified in [DESIGN_DIRECTION.md §6.9](DESIGN_DIRECTION.md#69-consent-storage-memory-and-private-name-controls). The status chip updates, and a status message confirms: "Mulai sekarang percakapan ini tidak disimpan." / "From now on this conversation isn't saved." Changing "Pakai yang Sela ingat di percakapan ini" applies from the next reply: "Mulai balasan berikutnya, Sela tidak memakai yang Sela ingat."

### Recovery

- Every failure keeps the person's text on screen.
- Retrying a failed send reuses the same idempotency key (planned), so a message is never posted twice.
- A dropped operation event stream resumes from `Last-Event-ID` (the v2 SSE runtime). Because the reply arrives whole, a drop never leaves half a reply on screen.
- A failed stop request is retried automatically, because cancelling is idempotent (v2 `DELETE /operations/{id}`).
- An unsaved conversation can be continued or saved from the closing panel until the tab is closed, a new conversation is started, Quick exit is used, or the hold times out after its warning; visiting Help or Privasi & data in between costs nothing.

### Keyboard and focus order

1. Skip links: "Lewati ke konten", then "Lewati ke kotak tulis".
2. Top bar: Butuh bantuan sekarang, Keluar cepat, Menu.
3. Room toolbar: ← Cerita, Akhiri percakapan, Detail (the h1 and the storage chip are not focusable).
4. Conversation strip (when expanded): intent chips, storage "Ubah", the memory radios.
5. Transcript: on wide layouts the region itself (`tabindex="0"`, scrolls with the arrow keys). Then the interactive items in order:
   - each message's notice actions ("Kirim lagi", the contact-detail notice's button);
   - each reply's "Tentang balasan ini", "Kurang tepat?" and "Bacakan";
   - reflection card actions;
   - crisis panel links and actions; the support line's link;
   - the closing panel's actions.
6. Room end bar (page flow): the pause line's buttons ([Ke bantuan] or [Lihat semua bantuan], then [Lanjutkan bercerita dengan Sela]), then "Ke balasan terakhir". Shift+Tab from the text area reaches these first.
7. The new-content button, when shown.
8. Composer: the text area, mic, Hentikan balasan, Kirim.

The Detail sheet (compact) opens with "Butuh bantuan sekarang", "Keluar cepat" and "Tutup" as its first items, then its heading and content ([DESIGN_DIRECTION.md §6.1](DESIGN_DIRECTION.md#61-app-shell)).

Keys and focus rules:

- **Ctrl/Cmd+Enter** sends.
- **Esc twice** is Quick exit, with the arming rules of [DESIGN_DIRECTION.md §6.12](DESIGN_DIRECTION.md#612-dialogs-sheets-and-inline-confirmation).
- There are no single-key shortcuts.
- **Focus stays in the composer** after sending, while Sela writes and on release.
- When "Hentikan balasan" disappears while focused (pressed, or the reply was released), focus moves to the text area.
- After "Tutup bantuan ini", focus moves to the text area.
- "Ke bantuan" and the new-content button move focus to the item's heading (or article), which shows the ring whether the button was pressed by key, click or tap.
- After "Kurang tepat?" focus goes to its field.
- After a reflection action focus stays on that statement.
- After "Akhiri percakapan" focus goes to the closing panel heading: the person asked for it, so this does not steal focus. After "Lanjutkan percakapan" it returns to the text area.
- After a confirmed deletion, focus goes to the Talk list heading (the conversation is gone).

### Layout

- **320px:**
  - the top bar in two rows (help; Quick exit and Menu);
  - the room toolbar wrapping by available space: back link and "Akhiri percakapan" on the first row (about 268px of 288), the h1 on the second, the storage chip and Detail on the third;
  - the transcript full width, with 16px gutters, then the room end bar (pause line, "Ke balasan terakhir") in the page flow;
  - the composer sticky at the bottom (in flow in short viewports; the soft keyboard never switches that mode, §5.8). It holds only its label, the text area and the action group;
  - the text area growing to 40% of the visual viewport; the field and Kirim share a line when they fit (about 94px for Kirim), and "Hentikan balasan" and Kirim move below the field while a reply is pending. Kirim is always last and at least 44px;
  - **while typing** (the text area has focus and the keyboard is open), the typing layout of [DESIGN_DIRECTION.md §6.5](DESIGN_DIRECTION.md#65-conversation-transcript-messages-composer): only "Butuh bantuan sekarang" stays fixed at the top, in the help strip; "Keluar cepat" and Menu return to the page flow; the field is capped at 25% and the composer at 50% of the visual viewport; and when fewer than three transcript lines would remain, the composer joins the page flow until the field loses focus, so the person reads Sela's reply by scrolling, with nothing covered. Estimates: about 132px of transcript (4.5 lines) at 360×300 and 112px at 320×280 with a sticky composer at 100% text; flow mode on a 230px visual viewport and at 150% text.

  The bottom navigation is hidden. Detail opens as a full sheet that starts with help and Quick exit.
- **Wide:**
  - the sidebar stays;
  - the transcript column is `--measure` wide and centred in the main column, with its own scroll and `scroll-padding-block`;
  - from 75em, the right-hand details panel (20rem) shows the intent, storage, memory, "Sela menangkap…" items, "Yang Sela ingat", Delete and "Butuh bantuan sekarang";
  - the composer sticks to the bottom of the transcript column, outside the transcript's scroll area.

### Copy

| Element | ID | EN |
|---|---|---|
| Composer label | Tulis untuk Sela | Write to Sela |
| Intent group | Hari ini kamu ingin… (boleh dilewati) | What would help today? (optional) |
| Storage question | Simpan percakapan ini di Catatanku? | Save this conversation to My notes? |
| Memory choice | Pakai yang Sela ingat di percakapan ini · Ya · Tidak | Use what Sela remembers in this conversation · Yes · No |
| Room intro | Tulis apa saja yang ada di pikiranmu. Tidak perlu rapi. | Write whatever is on your mind. It doesn't need to be tidy. |
| Correction prompt | Apa yang kurang tepat? Tulis dengan kata-katamu. | What wasn't right? Say it in your own words. |
| End | Akhiri percakapan | End conversation |
| Resume replies | Lanjutkan bercerita dengan Sela | Keep talking with Sela |
| Pause line | Balasan Sela dijeda di percakapan ini. Kamu tetap bisa menulis. | Sela's replies are paused in this conversation. You can still write. |
| Stopped | Balasan dihentikan sebelum ditampilkan | Reply stopped before it was shown |
| Unsaved, closing | Percakapan ini tidak disimpan. Sela membuangnya saat kamu menutup tab ini, memulai percakapan baru, atau memakai Keluar cepat. Kamu bisa membuka bantuan atau Privasi & data dulu tanpa kehilangannya. | It isn't saved. Sela discards it when you close this tab, start a new conversation or use Quick exit. You can open help or Privacy & data first without losing it. |
| Hold warning | Percakapan yang tidak disimpan ini akan dibuang dalam sekitar 2 menit karena tidak ada aktivitas. [Tetap buka] | This unsaved conversation will be discarded in about 2 minutes because of inactivity. [Keep it open] |
| Contact detail, sent | Nomor telepon disamarkan untuk AI. [Samarkan juga di Catatanku] | Phone number hidden from AI. [Hide it in My notes too] |
| Pause line, help still shown | Balasan Sela dijeda di percakapan ini. Kamu tetap bisa menulis. [Ke bantuan] | Sela's replies are paused in this conversation. You can still write. [Go to help] |
| Checker not allowed | Balasan yang menyebut hal kesehatan perlu diperiksa dulu, dan pemeriksa (Claude) belum diizinkan. | Replies that mention health need a check first, and the checker (Claude) isn't allowed yet. |

### API (planned unless marked)

- `POST /api/v3/conversations` `{storage: "save" | "dont_save", intent?, useMemory?}`.
- `GET` and `PATCH /api/v3/conversations/{id}` `{storage?, intent?, title?, useMemory?}`.
- `POST /api/v3/conversations/{id}/messages` `{text, kind: "message" | "correction", replyTo?}` returns `{message, safety, redactedKinds[], operation?}`; the message carries the server-set `lang`. `PATCH /api/v3/conversations/{id}/messages/{messageId}` `{storedMasking: "mask" | "as_written"}` applies "Samarkan juga di Catatanku" or "Tampilkan seperti yang kutulis".
- `POST /api/v3/conversations/{id}/hold` (extend the unsaved hold; "Tetap buka") and the hold's expiry time in `GET /api/v3/conversations/{id}`.
- `GET /api/v3/operations/{id}/events` (SSE, post-release: progress events, then one of `released`, `withheld` with a reason, `crisis`, `no_ai`, `stopped`) and `DELETE /api/v3/operations/{id}` (stop), after the v2 pattern of `GET /api/v2/operations/{id}/events` and `DELETE /api/v2/operations/{id}`.
- `POST /api/v3/conversations/{id}/resume` (continue after the crisis panel).
- `POST /api/v3/conversations/{id}/end`; for an unsaved conversation the server holds it until `POST /api/v3/conversations/{id}/discard` (sent on leaving the closing panel, by `sendBeacon` on page hide and by Quick exit) or until its hold time runs out.
- `GET /api/v3/derived?source=conversation:{id}` and `DELETE /api/v3/conversations/{id}?derived=delete|keep`.
- `POST /api/v3/conversations/{id}/reflections`.
- `PATCH /api/v3/reflections/{id}/items/{itemId}` `{action: "confirm" | "edit" | "reject" | "undo", text?}`.
- `POST /api/v3/memory/proposals/{id}/accept|reject`.
- `GET /api/v2/ai/status` (v2) for the cold-start estimate, until `/api/v3/ai` exists.
- `PUT /api/v3/me/consents/personal_memory` (purpose reserved, planned P3) and `PUT /api/v3/me/consents/ai_provider_anthropic` (the in-context checker card; planned P5).
- The `released` event carries the reply text, its `lang` and its provenance (generator and verification provider), so the author line and the announcement never depend on configuration.

## S10 Note (journal) and reflection

**Routes** `/notes/write` (new), `/notes/:id` (detail and edit) · **Access** owner · **Title** "Tulis catatan – Sela" / "Catatan – Sela"

- **Purpose:** write privately, with no AI involved unless asked. Look back at a note, edit it with its history kept, and optionally ask Sela to reflect.
- **Primary action:** "Simpan catatan" / "Save note".
- **Secondary actions:**
  - "Batal" (cancel);
  - "Butuh pancingan?" (need a prompt?): three optional prompt chips, which insert the prompt as the note's first line, in the user's control and editable:
    - "Apa yang paling terasa hari ini?";
    - "Apa yang ingin kamu ingat?";
    - "Apa yang membantu, sekecil apa pun?";
  - on the detail page:
    - "Ubah" (edit);
    - "Hapus" (delete);
    - "Minta Sela menangkap…" (ask Sela to reflect; AI, needs `ai_processing`; the output is class `unconfirmed_interpretation` ("Tafsiran Sela") with its own review status, shown in the §6.6 card with its generator line);
    - "Buat ringkasan dari catatan ini" (starts S15 with this note pre-selected);
    - "Lihat versi sebelumnya" (see earlier versions).
- **Content and data:**
  - an optional title (at most 120 characters);
  - the text (`user_original`, at most 8,000 characters, planned);
  - the date;
  - on the detail page: version ("versi 2 · diubah 25 Okt 2026, 21.10"), the version list (each version read only, with time), and reflection items with their badge and status chip.
  - Names on the person's "Nama yang disamarkan" list are masked before "Minta Sela menangkap…" sends anything to AI (ADR-0003 DM-2). Contact details are always masked for AI; when "Minta Sela menangkap…" runs, the same non-blocking notice as in the room (S09) appears under the note. Saving a note never waits on it.
- **States:**
  - empty: a new note opens with the field focused and the hint "Hanya kamu yang bisa membacanya, kecuali kamu memilih membagikannya." / "Only you can read this, unless you choose to share it.";
  - loading (detail): "Memuat catatan…";
  - error, on save: "Catatan belum tersimpan. Tulisanmu masih di sini." [Coba simpan lagi];
  - success: "Tersimpan · 20.14", and the URL becomes `/notes/:id`;
  - **crisis match on save** (D-035): the note **is saved**, if the person is saving at all. The crisis panel appears below with: "Tersimpan. Karena ada kata-kata dalam catatanmu yang terdengar sangat berat, Sela menunjukkan bantuan di bawah ini. Kamu tidak harus memakainya." Nothing is refused. This fixes v1 finding 4, where heavy feelings could not be saved.
- **Leaving with unsaved text:**
  - in-app navigation shows a dialog, "Catatan belum disimpan." [Simpan dulu] [Tinggalkan tanpa menyimpan] [Batal];
  - Quick exit skips the dialog by design.
- **Recovery:**
  - the text is kept on every error;
  - editing creates a new version;
  - deleting uses the confirm that lists derived items ([DESIGN_DIRECTION.md §6.12](DESIGN_DIRECTION.md#612-dialogs-sheets-and-inline-confirmation)). With nothing derived: "Hapus catatan ini? Semua versinya ikut terhapus." [Hapus catatan] [Batal]. With derived items: "Kutipan dari catatan ini ada di 1 ringkasan yang sudah disetujui." [Hapus catatan dan semua turunannya] [Hapus catatannya saja] [Batal], both destructive choices in the same Confirm-destructive variant and size. After deletion, focus goes to the timeline heading.
- **Keyboard and focus order:**
  1. h1;
  2. Judul (optional);
  3. Catatanmu (the text area, focused on arrival for new notes; the person came here to write);
  4. the prompt chips;
  5. Simpan;
  6. Batal.

  On the detail page: h1, the text (not focusable), Ubah, Minta Sela menangkap, Buat ringkasan, Hapus, Lihat versi sebelumnya. Ctrl/Cmd+S is **not** intercepted, so the browser's behaviour stays.
- **Layout:**
  - 320px: full-width fields; the text area at least 10 lines tall; the buttons at the end of the form (not sticky; in flow with the soft keyboard).
  - Wide: within `--measure`; on the detail page, the version list in a disclosure.
- **API (planned):**
  - `POST /api/v3/journal` `{title?, text}`;
  - `GET`, `PATCH` and `DELETE /api/v3/journal/{id}` (`?derived=delete|keep`), with `GET /api/v3/derived?source=journal:{id}`;
  - `GET /api/v3/journal/{id}/versions`;
  - `POST /api/v3/journal/{id}/reflections`.

| Element | ID | EN |
|---|---|---|
| h1 (new) | Tulis catatan | Write a note |
| Field label | Catatanmu | Your note |
| Saved | Tersimpan di Catatanku · 20.14 | Saved to My notes · 20:14 |

---

## S11 Check-in

**Routes** `/notes/check-in` (new); `/notes/:id` for a saved check-in (the check-in detail, below) · **Access** owner · **Title** "Cek perasaan – Sela"; detail "Cek perasaan, {tanggal} – Sela" (neutral "Entri – Sela" when previews are hidden)

- **Purpose:** a quick, private snapshot of how someone feels. There are no streaks, reminders, praise or scores (D-036).
- **Primary action:** "Simpan" / "Save".
- **Secondary actions:**
  - "Batal";
  - after saving, offered once: "Mau cerita lebih banyak?" [Mulai bercerita].
- **Content and data:**
  - feeling: required, from six options (§6.17);
  - "Tenaga" / "Energy": optional, Rendah / Sedang / Tinggi (Low / Medium / High);
  - "Yang sedang memengaruhi" / "What's affecting you": optional checkbox chips:
    - Tidur;
    - Sekolah/kuliah;
    - Pekerjaan;
    - Keluarga;
    - Teman;
    - Pasangan;
    - Uang;
    - Kesehatan;
    - Kesepian;
    - Lainnya;
  - "Catatan singkat" / "Short note": optional, 500 characters.

  Everything is `user_original`.
- **States:**
  - empty: the form;
  - error: "Belum tersimpan." [Coba lagi], with the choices kept;
  - success: "Tersimpan di Catatanku · 20.14." No "great job", no counts;
  - crisis match in the note: saved, plus the crisis panel (as S10).
- **Recovery:** the choices are kept on errors. A saved check-in can be deleted from its detail page (below), with the same derived-items confirm as a note (a check-in can be a summary source).
- **Keyboard and focus order:**
  1. h1;
  2. the feeling radios (arrow keys);
  3. the energy radios;
  4. the tag checkboxes;
  5. the note;
  6. Simpan.
- **Layout:** the tile grid follows the **width of its fieldset** (a container query in em on the fieldset), never the text-size setting (DESIGN_DIRECTION §5.4 rule 5):
  - fieldset 36em or wider: the six tiles in one row;
  - 18em to 36em: a 2×3 grid (320px phones at 100–115% text);
  - below 18em: one column (for example 320px at 130% text or more, or under a 1.4.12 spacing stylesheet).
  - The tags wrap by available space.
- **API (planned):**
  - `POST /api/v3/check-ins`;
  - `GET` and `DELETE /api/v3/check-ins/{id}` (`?derived=delete|keep`), with `GET /api/v3/derived?source=check_in:{id}`.

### Check-in detail · `/notes/:id`

- **Purpose:** look back at one saved check-in, see what was built from it, and delete it. Reached from its timeline row (S12) and from the pattern view's table.
- **Primary action:** none; the page is read only. Check-ins are not edited (a new check-in is the honest way to record a change).
- **Secondary actions:** "Hapus cek perasaan ini" / "Delete this check-in" (Danger, with the derived-items confirm); "Cek perasaan lagi" (opens S11 for a new one); "← Catatanku".
- **Content and data:** the date and time; the feeling (its label and the dots row, as text); energy and tags if given; the short note (`user_original`, `lang` when set); "Dipakai di: Ringkasan untuk konsultasi, versi 3" when a summary quotes it (a link to S15), or nothing.
- **States:** loading "Memuat cek perasaan…" after 400ms; error "Belum bisa memuat cek perasaan ini." [Coba lagi]; not found (deleted, or someone else's id): S27 Not found; with previews hidden the h1 is "Cek perasaan" and the note text is still shown, because the person opened it on purpose.
- **Delete:** nothing derived: "Hapus cek perasaan ini?" [Hapus cek perasaan] [Batal]. With derived items: "Isi cek perasaan ini ada di 1 ringkasan (sedang dibagikan ke 'Psikolog Puskesmas')." [Hapus cek perasaan dan semua turunannya] [Hapus cek perasaannya saja] [Batal], both destructive choices in the same Confirm-destructive variant and size, focus on the confirm sentence first ([DESIGN_DIRECTION.md §6.12](DESIGN_DIRECTION.md#612-dialogs-sheets-and-inline-confirmation)). After deletion: the timeline, with focus on its heading and the status "Cek perasaan dihapus."
- **Keyboard and focus order:** shell; ← Catatanku; h1; the "Dipakai di" link; Cek perasaan lagi; Hapus cek perasaan ini (and its confirm).
- **Layout:** 320px: one column, the facts as a definition list, the actions stacked at the end. Wide: within `--measure`.
- **API (planned):** `GET` and `DELETE /api/v3/check-ins/{id}` (`?derived=delete|keep`); `GET /api/v3/derived?source=check_in:{id}`.

| Element | ID | EN |
|---|---|---|
| Legend | Bagaimana perasaanmu sekarang? | How are you feeling right now? |
| Options | Berat · Kurang baik · Biasa saja · Cukup baik · Baik · Campur aduk / tidak yakin | Heavy · Not great · Okay · Fairly good · Good · Mixed / not sure |

---

## S12 Timeline

**Route** `/notes` (default section of Catatanku), `/notes/patterns` (opt-in) · **Access** owner · **Title** "Catatan pribadiku – Sela"

- **Purpose:** one calm history across sessions of everything kept (brief §7: "Timeline lintas sesi").
- **Primary action:** "Tulis catatan" / "Write a note".
- **Secondary actions:**
  - "Cek perasaan";
  - the filter chips;
  - open an entry;
  - "Buat ringkasan untuk konsultasi" (go to S15);
  - "Yang Sela ingat" (the section link);
  - "Lihat pola cek perasaan" (see check-in patterns);
  - "Muat lebih banyak" (load more).
- **Content and data:**
  - the sub-navigation: "Linimasa" | "Yang Sela ingat";
  - filter chips (checkboxes): Percakapan, Catatan (journal entries only), Cek perasaan, Ringkasan, Berbagi. None selected means all. The general word for anything kept is "entri" or "yang tersimpan", never "catatan";
  - entries grouped by day under `h2` ("Hari ini", "Kemarin", "Sabtu, 24 Oktober 2026"), each one link (§6.14) with:
    - the type;
    - the time;
    - the preview, which is `user_original` text and is hidden when "Sembunyikan pratinjau isi" is on;
    - badges where relevant, such as "Ringkasan · versi 3 · Disiapkan";
  - share events, which are content-free ("Ringkasan dibagikan ke 'Psikolog Puskesmas'", "Dibuka penerima", "Selesai dibagikan · penerima tidak dapat menerima"). With previews hidden they read "Ringkasan dibagikan", with no recipient label or summary title;
  - each row opens its entry: a conversation in S09, a note in S10, a check-in in the check-in detail (S11), a summary in S15, a share in S18;
  - a retention line for guests.
- **Pattern view** (`/notes/patterns`, opt-in):
  - The first visit shows a consent card: "Pola dibuat dari cek perasaanmu sendiri dengan hitungan sederhana, tanpa AI. Kamu bisa mematikannya kapan saja." [Tampilkan pola] [Tidak sekarang].
  - Opt-in only, for every age. For 16–17 this follows the profiling rule of PP 17/2025 Pasal 19 under D-030.
  - It shows plain sentences and a table for the last 14 or 30 days: the counts per feeling, and the tags that appear most with "Berat".
  - There is no score, trend arrow or prediction. It is labelled "Ini hitungan sederhana, bukan penilaian." / "These are simple counts, not an assessment."
  - An optional bar chart has the table as its text equivalent (1.1.1).
- **States:**
  - empty: "Belum ada yang tersimpan. Yang kamu simpan akan muncul di sini, dari yang terbaru." [Mulai bercerita] [Tulis catatan];
  - empty with filters: "Tidak ada entri untuk pilihan ini." [Hapus filter];
  - loading;
  - error: [Coba lagi], keeping what was already loaded;
  - success: the load-more status ("20 entri lagi dimuat").
- **Recovery:** filters persist in the URL query, and the loaded pages and scroll position are kept in the tab, so Back from an entry returns to the same place ([DESIGN_DIRECTION.md §8](DESIGN_DIRECTION.md#8-implementation-notes-for-the-frontend-planned-owned-by-the-frontend-engineer)).
- **Keyboard and focus order:**
  1. h1;
  2. the sub-navigation;
  3. Tulis catatan;
  4. Cek perasaan;
  5. the filter chips;
  6. entries in order;
  7. Muat lebih banyak;
  8. Buat ringkasan;
  9. Lihat pola.

  After "Muat lebih banyak", focus goes to the first new entry.
- **Layout:**
  - 320px: day headings with full-width entry rows (type icon and label on the first line, preview on the second, time and badge on the third); the filter chips wrap by available space. Focusing a filter chip never hides the bottom bar (only text-entry fields do).
  - Wide: within `--measure-wide`; the filters in one row; the actions in the page header.
- **API (planned):**
  - `GET /api/v3/timeline?types=&limit=&cursor=`;
  - `GET /api/v3/check-ins/patterns?days=14|30`.

| Element | ID | EN |
|---|---|---|
| h1 | Catatan pribadiku | My personal notes |
| Lead | Semua yang kamu simpan, dari yang terbaru. Hanya kamu yang bisa melihatnya. | Everything you've saved, newest first. Only you can see it. |

---

## S13 What Sela remembers

**Route** `/notes/memory` · **Access** owner · **Title** "Yang Sela ingat – Sela"

- **Purpose:** transparent, correctable memory (brief §7: "Memory yang transparan dan dapat dikoreksi"). The person sees everything Sela may use in later conversations, and can change it, forget it or pause it.
- **Primary action:**
  - without consent: "Izinkan Sela mengingat" / "Let Sela remember";
  - with consent: "Tambahkan sesuatu" / "Add something".
- **Secondary actions:**
  - per item: "Ubah" (change) and "Lupakan" (forget);
  - "Jeda memori" (pause memory: Sela stops using it but keeps it);
  - "Lupakan semuanya" (forget everything);
  - proposals: "Ingat" / "Jangan";
  - "Cabut izin mengingat" (withdraw the memory consent).

  Each memory control states its scope in one line next to it ([DESIGN_DIRECTION.md §6.9](DESIGN_DIRECTION.md#69-consent-storage-memory-and-private-name-controls)): "Jeda memori" — "Berlaku untuk semua percakapan mulai sekarang, termasuk yang sedang terbuka. Yang Sela ingat tetap tersimpan."; the consent — "Izin supaya Sela boleh menyimpan dan memakai yang kamu setujui. Mencabutnya melupakan semuanya."; the link to the S22 default — "Berlaku untuk percakapan yang kamu mulai setelah ini."
- **Content and data:**
  - the explanation: "Sela hanya mengingat hal yang kamu setujui. Sela memakainya untuk menyesuaikan balasan. Catatanmu tidak dipakai untuk melatih model." / "Sela only remembers what you agree to. It uses it to fit its replies to you. Your notes are not used to train models." (D-047);
  - memory status: on, paused or off;
  - items, each with ([DESIGN_DIRECTION.md §6.7.1](DESIGN_DIRECTION.md#671-record-types-class-statuses-and-transitions)):
    - the statement (with `lang` when the item's `lang` differs from the interface);
    - the badge and the status chip, side by side: "Kata-katamu" for an item the person added; "Tafsiran Sela · Dikonfirmasi" for a proposal they accepted; "Koreksimu · Dikoreksi" after they changed it;
    - for an accepted proposal, the generator line from its stored provenance: "Ditulis dengan bantuan AI (Meta) · diperiksa oleh Claude" (or the fallback, alternative-checker and test-program forms);
    - "Dipakai di 1 ringkasan" when a summary includes it (a link to S15);
    - the source ("dari percakapan 24 Okt", linked; "dari percakapan yang sudah dihapus" when it was deleted on its own);
    - version;
    - "terakhir dipakai 25 Okt" (last used; planned, and shown only if the API records use);
  - "Usulan dari Sela" (proposals): pending items, class "Tafsiran Sela", status "Belum ditinjau", each with the generator line and "Diperiksa oleh Claude: cocok dengan kata-katamu" (or "Diperiksa oleh program uji (bukan AI)" in a test environment, from the stored provenance). Proposals that failed the check are never shown. Proposals come **only from saved entries** (AT-3, DM-4);
  - a link to the default "Pakai yang Sela ingat di percakapan baru" in S22.
- **States:**
  - empty without consent: the consent card with what memory does, where it is stored and how to undo it;
  - empty with consent: "Belum ada yang Sela ingat. Kamu bisa menambahkannya, atau menyetujui usulan di percakapan.";
  - paused: a warning-tinted line, "Memori dijeda. Sela tidak memakainya di percakapan." [Lanjutkan memori];
  - loading;
  - error;
  - success: per action ("Diperbarui", "Dilupakan").
- **Forget:** pressing "Lupakan" first asks the server whether a summary includes the item (the derived-items check of [DESIGN_DIRECTION.md §6.12](DESIGN_DIRECTION.md#612-dialogs-sheets-and-inline-confirmation); "Memeriksa…" meanwhile).
  - Nothing derived: an inline confirm, "Lupakan ini? Sela tidak akan memakainya lagi. Percakapan asalnya tetap ada." [Lupakan] [Batal].
  - A summary includes it: "Hal ini ada di 1 ringkasan (sedang dibagikan ke 'Psikolog Puskesmas')." / "This is in 1 summary (shared with 'Psikolog Puskesmas')." [Lupakan dan keluarkan dari ringkasan] (the summary gets a new version without it, its approval is cleared, and an active share of the affected version is revoked, with the fixed revocation line) [Lupakan saja] (the summary keeps its copy) [Batal]; both destructive choices in the same Confirm-destructive variant and size.

  After forgetting, focus moves to the next item's first control (or the list heading if none is left) and the status says "Dilupakan." (or "Dilupakan. 1 ringkasan diperbarui, 1 berbagi dicabut."). For "Lupakan semuanya": a dialog, "Lupakan semua yang Sela ingat? Ini tidak menghapus percakapan atau catatanmu." [Lupakan semuanya] [Batal], with the same two choices when summaries include any item.
- **Withdrawing the consent** ("Cabut izin mengingat", here and in S22): a dialog, "Cabut izin mengingat? Sela akan melupakan semua yang Sela ingat (12 hal). Percakapan dan catatanmu tetap ada." [Cabut izin dan lupakan semuanya] [Batal] (with the derived-items choices when summaries include items). Forgetting is the default and the only choice for now: keeping items paused without the consent is offered only if the legal review (B-10) confirms a basis ([DESIGN_DIRECTION.md §10](DESIGN_DIRECTION.md#10-open-points-for-the-pm-and-other-owners) item 21). Withdrawing never needs reauthentication.
- **Recovery:** edits keep the earlier version, shown in the item's "dikoreksi dari…" disclosure.
- **Keyboard and focus order:**
  1. h1;
  2. the sub-navigation;
  3. the status and its action;
  4. Tambahkan sesuatu;
  5. the proposals (Ingat / Jangan);
  6. each item's Ubah and Lupakan (and its "Dipakai di" link);
  7. Lupakan semuanya;
  8. Cabut izin mengingat.
- **Layout:**
  - 320px: item cards stacked, actions in one row (they wrap at large text).
  - Wide: within `--measure-wide`; the proposals in a side column from 75em.
- **Minors and `unknown`:** memory is off by default and opt-in here only (D-030), with teen copy; it is never asked in context (S26).
- **API (planned):**
  - `GET /api/v3/memory`;
  - `POST /api/v3/memory` `{text}`;
  - `PATCH /api/v3/memory/{id}` `{text}`;
  - `DELETE /api/v3/memory/{id}`;
  - `PATCH /api/v3/memory/settings` `{paused}`;
  - `DELETE /api/v3/memory` (forget everything); both deletes take `?derived=delete|keep`, after `GET /api/v3/derived?source=memory:{id}` (or `source=memory:*`);
  - `POST /api/v3/memory/proposals/{id}/accept|reject`;
  - `PUT /api/v3/me/consents/personal_memory`.

---

## S14 Support hub

**Route** `/support` · **Access** owner · **Title** "Dukungan – Sela"

- **Purpose:** gather what helps beyond the conversation, without pushing anyone toward a professional (brief §1: "Pengguna tidak wajib mencari profesional").
- **Primary action:** none is forced. Each section has its own clear action.
- **Secondary actions:**
  - "Buat ringkasan" (create a summary);
  - open a summary or a share;
  - "Lihat semua bacaan" (see all reading);
  - "Lihat layanan bantuan" (see support services).
- **Content and data:** the lead "Hal-hal yang bisa membantu di luar percakapan. Semuanya pilihan." / "Things that can help beyond talking. All of it is optional." Then three sections, each with an `h2`:
  1. **Ringkasan untuk konsultasi** (Summary for a consultation).
     - Explanation: "Kalau kamu ingin bicara dengan profesional, misalnya di Puskesmas atau dengan psikolog, Sela bisa membantumu menyiapkan ringkasan dari yang kamu pilih di Catatanku."
     - [Buat ringkasan].
     - The list of summaries, each with its state: "Draf"; "Disiapkan · versi 3" for an approved version that has not been shared (D-040 `prepared`); once shared, the share's latest recorded status chip ("Dibagikan", "Dibuka penerima" …; an end state always with its reason line, such as "Selesai dibagikan · penerima tidak dapat menerima"), with the recipient label unless previews are hidden.
     - Active shares.
  2. **Bacaan & latihan kecil** (Reading & small exercises).
     - Three items chosen by editors. They are not personalised and there is no "recommended for you".
     - "Lihat semua".
  3. **Layanan bantuan** (Support services).
     - "Daftar layanan yang sudah diverifikasi, dengan sumber dan tanggal pemeriksaan."
     - [Lihat layanan bantuan] goes to `/help#layanan`.
- **States:**
  - empty summaries: "Belum ada ringkasan. Tidak wajib — buat hanya kalau kamu membutuhkannya." / "No summaries yet. Not required — make one only if you need it.";
  - loading: each section shows "Memuat…" after 400ms;
  - error: shown per section;
  - success: not applicable.
- **Recovery:** each section fails on its own with [Coba lagi]; the other sections, "Buat ringkasan" and the help link keep working. Reading and services fall back to bundled data when the network is down.
- **Keyboard and focus order:** h1, then each section's links and buttons in order.
- **Layout:**
  - 320px: the three sections stacked.
  - Wide: summaries in the main column; reading and services side by side below.
- **API (planned):**
  - `GET /api/v3/consultation-summaries`;
  - `GET /api/v3/shares`;
  - `GET /api/v3/reading?featured=1`.

---

## S15 Summary for a consultation: build, review, approve, view

**Routes** `/support/summaries/new` (starts a draft) → `/support/summaries/:id/edit/choose` → `…/edit/compose` → `…/edit/review` → `…/edit/approve`; `/support/summaries/:id` (view) · **Access** owner · **Title** "Ringkasan untuk konsultasi – Sela"

The summary is honest by construction. It contains only what the person chose (brief §7). Every item shows its source words. AI wording is checked before the person sees it (D-029). The approval is bound to an exact version and hash (v2). The summary is never called a medical record (D-038).

On every step and in the view, a header line reads "Ini catatan pribadimu. Bukan rekam medis atau hasil pemeriksaan." / "These are your own notes. Not a medical record or an assessment.", followed by **one** of these, depending on what the summary really contains (D-038: "where applicable"):

- it contains AI wording or a Sela interpretation: "Sebagian kalimatnya ditulis dengan bantuan AI (Meta) dan diperiksa oleh Claude." / "Some of the wording was written with AI help (Meta) and checked by Claude." (with DeepSeek, the alternative checker, or "program uji (bukan AI)" named when they were used);
- it does not: "Disusun sendiri, tanpa AI." / "Put together by you, without AI."

The line is built from the **stored provenance of the items in this version**, never from the environment's configuration, so a summary checked by a test program never says "Claude".

The progress line reads "Langkah 2 dari 4: Cara menyusun".

### Step 1: Pilih yang ingin dimasukkan (choose what to include)

- **Purpose:** choose the sources.
- **Primary action:** "Lanjut" (shows "3 dipilih").
- **Content:**
  - a selectable list (§6.14) of saved conversations, notes, check-ins and memory items, with filters by type and by time ("30 hari terakhir", "Semua");
  - **memory items keep their class** ([DESIGN_DIRECTION.md §6.7.1](DESIGN_DIRECTION.md#671-record-types-class-statuses-and-transitions)). Each memory row shows its badge: an item Sela proposed reads "Tafsiran Sela · Dikonfirmasi" with its generator line and the hint "Masuk sebagai tafsiran Sela, bukan kata-katamu. Untuk memakai kata-katamu sendiri, pilih percakapan asalnya." / "Goes in as Sela's interpretation, not your words. To use your own words, choose the conversation it came from." (with a link to that conversation when it still exists); an item the person wrote reads "Kata-katamu";
  - nothing is pre-selected, except an entry the person started from (S10 "Buat ringkasan dari catatan ini").
- **Empty:** "Belum ada yang tersimpan untuk dipilih. Ringkasan dibuat dari yang kamu simpan." [Mulai bercerita] [Tulis catatan].
- **Error:** [Coba lagi]. The selection is kept.

### Step 2: Cara menyusun (how to compose)

- **Purpose:** choose between AI help and doing it by hand. The manual path always exists.
- **Control:** a radio group.
  - "Bantu aku menyusun" / "Help me put it together", with the line: "Sela (Meta, diproses di Amerika Serikat) menyusun poin-poin dari kata-katamu, lalu pemeriksa (Claude, Amerika Serikat) mengecek bahwa setiap poin sesuai dengan kata-katamu. Nama yang kamu samarkan dan kontak diganti dulu sebelum dikirim." When the person has granted the backup model, it adds: "Kalau model utama sedang tidak tersedia, model cadangan DeepSeek (diproses di Tiongkok) yang menyusun, dan Claude tetap memeriksa." / "If the main model is unavailable, the backup model DeepSeek (processed in China) writes it, and Claude still checks it." (AT-4). This describes what *will* happen, so it comes from the live AI status; in a test environment it reads "program uji (bukan AI)" for whichever part a test program does. Only conversations, notes and check-ins are sent to AI drafting; memory items enter the summary as they are, with their own class.
  - "Susun sendiri" / "I'll put it together myself": pick quotes from the chosen notes and write the wording.
- **Consents:** missing ones (`ai_processing`, `ai_provider_anthropic`) appear inline as consent cards (§6.9). Without the verifier consent, the AI option explains, in neutral words: "Pemeriksa (Claude) belum diizinkan, jadi Sela tidak menyusun dengan AI. Kamu tetap bisa menyusun sendiri." / "The checker (Claude) isn't allowed yet, so Sela won't draft with AI. You can still put it together yourself." D-029 makes the verifier mandatory for summary wording.
- **AI running:**
  - The status reads "Sela sedang menyusun dari 3 catatan…", then "Sedang diperiksa…", with Stop.
  - **Result:**
    - items that passed;
    - "N kalimat tidak dipakai karena tidak cocok dengan kata-katamu" / "N sentences weren't used because they didn't match your words", with a disclosure listing them, labelled "Tidak dipakai".
  - **Verifier unavailable:** "Pemeriksa sedang tidak tersedia, jadi Sela tidak memakai kalimat AI. Kata-katamu sudah disiapkan sebagai poin; kamu bisa menyusunnya sendiri." The system abstains and keeps the person's own words (D-029).

### Step 3: Periksa (review)

- **Purpose:** the person checks and owns every item.
- **Content:** items grouped by kind, each group with an `h2`:
  - Perasaan (feelings);
  - Pengalaman (experiences);
  - Dampak sehari-hari (daily impact);
  - Sudah berapa lama (how long);
  - Konteks (context);
  - Kekhawatiran (worries);
  - Harapan (hopes);
  - Pertanyaan (questions);
  - Hal praktis (practical things).

  These are the v2 kinds, renamed for plain language.
- **Each item card:**
  - the quote block: `user_original`, "Kata-katamu · percakapan 24 Okt", linking to the source. A name on the person's "Nama yang disamarkan" list is shown with a dotted underline, the eye-off icon and the visually hidden text "(disamarkan)", so the person sees which names a recipient will not see;
  - the wording, with its badge and status chip side by side ([DESIGN_DIRECTION.md §6.7.1](DESIGN_DIRECTION.md#671-record-types-class-statuses-and-transitions)): "Tulisan AI · Belum ditinjau" with the generator line "Ditulis dengan bantuan AI (Meta) · diperiksa oleh Claude" (shown once at the top of each group that has AI wording, and in each item's detail); "Koreksimu · Dikoreksi"; or "Kata-katamu" when the quote is used as the wording;
  - **an item from a memory item that Sela proposed** has no quote block: it shows "Tafsiran Sela" with its status chip (starting at "Belum ditinjau" in this summary), its generator line, and "Dari Yang Sela ingat · asalnya percakapan 24 Okt" with [Lihat asalnya]. Its actions are Benar, Ubah, Ganti jenis and Hapus; there is no "Pakai kata-kataku saja", because there are no words of the person to use. It is never styled with the kunyit rule;
  - the qualifier chips from v2's Meaning Lock (for example "tidak terjadi", "tentang orang lain", "dulu", "belum pasti"), shown as text;
  - the review status;
  - actions:
    - "Benar" (confirm);
    - "Pakai kata-kataku saja" (use my words);
    - "Ubah" (edit);
    - "Ganti jenis" (change kind): a native select **plus a "Pindahkan" / "Move" button** that applies it. The select alone never moves the item, because some browsers fire `change` on every arrow key in a closed select. After "Pindahkan", the item appears under its new group's `h2`, focus stays on the same item (its "Ganti jenis" select), and the status says "Dipindahkan ke Pengalaman." / "Moved to Experiences.";
    - "Hapus" (remove).
- **Extra sections the person writes (optional):**
  - "Yang belum kuketahui" (things not known yet; the v2 unknowns);
  - "Pertanyaan yang ingin kutanyakan" (questions to ask);
  - "Hal praktis" (practical preferences: language, format, cost, schedule, what makes starting hard; the v2 preferences).
- **Progress:** "4 dari 7 poin sudah kamu periksa." (status, polite).
- **Primary action:** "Lanjut ke persetujuan" (continue to approval). It is `aria-disabled` until every item is confirmed (v2 rule), with the reason in the hint.
- **Recovery:**
  - every change creates a new version and clears the approval (v2 rule);
  - edits pass the safety screen;
  - a failed save keeps the edit open with its text.

### Step 4: Setujui (approve)

- **Purpose:** an explicit, reviewable decision (WCAG 3.3.4).
- **Content:** the full summary exactly as it will be shown or shared (the same component as the view).
- **Primary action:** "Setujui versi 3" / "Approve version 3".
- **Secondary action:** "Kembali memeriksa".
- **Errors:**
  - `stale_version`: "Ringkasan berubah di tab lain. Periksa versi terbaru." It reloads at step 3.
  - other failures: "Persetujuan belum tersimpan. Ringkasanmu tidak berubah." [Coba lagi].
- **Success:** the summary view `/support/summaries/:id` opens with the status line "Versi 3 disetujui dan disiapkan untuk dibagikan." / "Version 3 approved and ready to share." at the top of the page, the state "Disiapkan · versi 3", and focus on `<main>` as for any route change. Its next actions (Siapkan konsultasi, Bagikan, Tunjukkan di layar) are the first buttons.

### View `/support/summaries/:id`

- **Header:** "Ringkasan untuk konsultasi · versi 3 · disetujui 26 Okt 2026 · Disiapkan" (or the share's latest status once shared).
- **Actions:**
  - [Siapkan konsultasi] (S16);
  - [Bagikan] (S17);
  - [Tunjukkan di layar] (show on screen);
  - [Cetak atau unduh] (print or download);
  - [Ubah] (creates version 4);
  - [Setelah konsultasi] (S19);
  - [Hapus ringkasan] (inline confirm: "Akses yang sedang dibagikan juga akan berhenti.").
- **Shares of this summary:** each with its status chip.
- **Show on screen:** a quiet full-screen mode for showing a professional in person, with no link needed.
  - Text size goes up one step.
  - Navigation is removed, except "Tutup" (close) and the top bar with help and exit.
  - Provenance stays visible as text.
- **Print and download:**
  - A print stylesheet: black on white, provenance as text, no IDs or URLs.
  - The footer line, matching the summary's content and built from its stored provenance: "Catatan pribadi, sebagian kalimatnya ditulis dengan bantuan AI (Meta) dan diperiksa oleh Claude. Bukan rekam medis. Dicetak 26 Okt 2026." or "Catatan pribadi, disusun sendiri tanpa AI. Bukan rekam medis. Dicetak 26 Okt 2026."
  - Each AI-worded item and each Sela interpretation keeps its badge, status and the group's generator line in print, and every item keeps its `lang`.
  - Download as HTML or Markdown (planned; the same renderer as the account export).

### Whole flow

- **Keyboard and focus order:** each step is h1, then the controls in reading order, then the primary action. On step 3, Tab moves through each item's actions in order. After an action, focus stays on the item.
- **Layout:**
  - 320px: item cards stacked; the quote block above the wording; actions wrapping to two rows.
  - Wide: within `--measure-wide`; the quote and wording side by side at 75em and over.
- **API:**
  - planned: `POST /api/v3/consultation-summaries` `{sourceIds[]}` (conversations, notes, check-ins and memory items; a memory item is copied with its class and version); `POST …/{id}/draft` (an AI operation, with SSE events); `POST …/{id}/items`; `PATCH …/{id}/items/{itemId}` `{action: confirm | use_quote | edit | set_kind | remove}`; `POST …/{id}/approval` `{version}`; `GET` and `DELETE …/{id}`; `GET …/{id}/export?format=html|md`. Items carry `lang` and the masked-name spans (planned);
  - v2 as the reference: `/api/v2/ledger/draft`, `/api/v2/ledger/items/{id}`, `/api/v2/ledger/clarifications`, `/api/v2/ledger/approval`.

| Element | ID | EN |
|---|---|---|
| h1 | Ringkasan untuk konsultasi | Summary for a consultation |
| Unsupported AI line | Tidak dipakai: tidak cocok dengan kata-katamu | Not used: didn't match your words |
| Group generator line | Ditulis dengan bantuan AI (Meta) · diperiksa oleh Claude | Written with AI help (Meta) · checked by Claude |
| Moved | Dipindahkan ke Pengalaman. | Moved to Experiences. |
| Approve | Setujui versi 3 | Approve version 3 |

---

## S16 Preparation

**Route** `/support/summaries/:id/prepare` · **Access** owner · **Title** "Persiapan konsultasi – Sela"

- **Purpose:** practical readiness: where to go, what to bring, how to begin. It claims no booking, partnership or verified professional (D-041).
- **Primary action:** "Simpan persiapan" / "Save preparation".
- **Secondary actions:**
  - choose a destination;
  - "Buat kalimat dari ringkasan" (make lines from the summary; deterministic template);
  - "Minta Sela menyarankan" (ask Sela to suggest lines; AI);
  - "Tulis sendiri" (write my own);
  - "Latihan mengucapkan" (practise saying it).
- **Content and data:** three sections, each with an `h2`.
  1. **"Ke mana kamu akan membawanya?"** (where will you take it?) A radio group:
     - Puskesmas atau klinik (FKTP);
     - Psikolog;
     - Healing119;
     - Layanan kampus atau sekolah;
     - Orang yang kupercaya (someone I trust);
     - Belum tahu (not sure yet).

     Choosing one shows its **sourced checklist** (v1 F25; public content). Each item has its source and date. Items not verified are labelled "belum diverifikasi". Example: "Bawa KTP dan kartu BPJS jika ada" (Kemenkes, …). "Belum tahu" shows a short comparison of the options.
  2. **"Kalimat pembuka"** (opening lines), one to three, with provenance from [DESIGN_DIRECTION.md §6.7.1](DESIGN_DIRECTION.md#671-record-types-class-statuses-and-transitions):
     - from a template ("Buat kalimat dari ringkasan"): generator `manual`, with the **class of the content it quotes** and one combined generator line that never contradicts the badge:
       - quoting only the person's words: "Kata-katamu" and "Dari templat Sela, tanpa AI" / "From a Sela template, no AI";
       - quoting AI wording the person confirmed: "Tulisan AI" and "Dari templat Sela, memakai kalimat AI yang sudah kamu konfirmasi (Meta · diperiksa oleh Claude)" / "From a Sela template, using AI wording you confirmed (Meta · checked by Claude)", with the original generator and checker from the quoted item's provenance;
       - quoting a correction the person made: "Koreksimu" and "Dari templat Sela, memakai kalimat yang kamu koreksi";
     - suggested by Sela ("Minta Sela menyarankan"): class `ai_summary` ("Tulisan AI · Belum ditinjau") with the generator line, shown only after the deterministic guards and, if flagged, the verifier (AT-5); "Pakai kalimat ini" confirms it. The line carries `lang` when it differs from the interface;
     - written by the person: "Kata-katamu";
     - editable;
     - the opening-line validator and pressure lexicon from v2 apply.
  3. **"Latihan mengucapkan"** (practise):
     - type, or dictate if enabled, how you would start;
     - the feedback lists what was mentioned and what was not, from the summary items;
     - "Latihan ini tidak disimpan dan tidak dinilai." / "Practice isn't saved or scored." (v2 Opening Practice).
- **States:**
  - empty: no destination chosen yet ("Pilih tujuan untuk melihat daftar persiapan");
  - loading: the checklist shows "Memuat daftar…" after 400ms; bundled checklists render offline;
  - AI suggestion running or failed: the template and own-words options stay available;
  - **save failed:** "Persiapan belum tersimpan. Pilihan dan kalimatmu masih di sini." / "Your preparation wasn't saved yet. Your choices and lines are still here." [Coba simpan lagi]; leaving with unsaved changes asks as in S10;
  - success: "Persiapan tersimpan · 20.14".
- **Recovery:** typed lines are kept on errors. Stale checklist sources show their date and "periksa ulang di situs resmi".
- **Keyboard and focus order:**
  1. h1;
  2. the destination radios;
  3. checklist items (checkboxes the person can tick for themselves);
  4. the line options;
  5. the line fields;
  6. the practice field;
  7. "Periksa latihanku" (check my practice);
  8. Simpan.
- **Layout:**
  - 320px: sections stacked.
  - Wide: the checklist and the lines side by side.

| Element | ID | EN |
|---|---|---|
| h1 | Persiapan konsultasi | Preparing for a consultation |
| Destination question | Ke mana kamu akan membawanya? | Where will you take it? |
| Unverified item | Belum diverifikasi — cek di tempat tujuan. | Not verified — check with the place you're going. |
| Practice note | Latihan ini tidak disimpan dan tidak dinilai. | Practice isn't saved or scored. |
| Coverage feedback | Yang sudah kamu sebut: susah tidur, sudah 3 minggu. Belum: apa yang ingin kamu tanyakan. | You've mentioned: trouble sleeping, for 3 weeks. Not yet: what you want to ask. |

- **API:**
  - planned: `GET /api/v3/reading/destinations` (public content, P6); `POST /api/v3/consultation-summaries/{id}/opening/template|draft`; `PUT …/opening`; `POST …/opening/practice`;
  - v2 as the reference: `/api/v2/opening/template`, `/api/v2/opening/draft`, `PUT /api/v2/opening`, `/api/v2/opening/practice`.

---

## S17 Sharing

**Route** `/support/summaries/:id/share` (steps `…/share/content`, `…/share/recipient`, `…/share/duration`, `…/share/review`, then `…/share/done`) · **Access** owner · **Title** "Bagikan ringkasan – Sela"

- **Purpose:** share chosen content with a chosen recipient, for a stated purpose and a limited time, after an exact preview (brief §7).
- **Primary action:** "Bagikan" / "Share", on the review step.
- **Secondary actions:** "Kembali" on each step; "Batal" (back to the summary).
- **Content and data:** each step shows a summary of the earlier choices with "Ubah" (WCAG 3.3.7).
  1. **Isi** (content). Checkboxes:
     - "Poin ringkasan (wajib)" / "Summary points (required)";
     - "Kata-kata asliku di samping tiap poin" / "My original words next to each point";
     - "Yang belum kuketahui";
     - "Pertanyaan yang ingin kutanyakan";
     - "Kalimat pembuka";
     - "Hal praktis";
     - "Kelompok usia" / "Age group" (shows "18 tahun ke atas" or "16–17 tahun");
     - "Bahasa pilihanku" / "My preferred language";
     - "Nama yang boleh dilihat penerima (opsional)" / "A name the recipient may see (optional)", a text field screened for contact details.
     - **"Nama yang disamarkan"** / "Names kept private", shown when the chosen parts contain names from the person's list: "Di ringkasan ini ada nama yang kamu samarkan: 'Rina' (2 poin), 'Pak Budi' (1 poin). Penerima melihat [nama disamarkan]." Then one checkbox per name, unchecked: "Tampilkan 'Rina' kepada penerima" / "Show 'Rina' to the recipient". The choice is per name for this share, not per item, because showing a name in one item and hiding it in another would still reveal it. The masking also applies to "Kata-kata asliku".
  2. **Penerima & tujuan** (recipient and purpose).
     - "Untuk siapa?" / "Who is it for?", up to 80 characters, with the hint "Contoh: 'Psikolog di Puskesmas Kecamatan'. Sela tidak memeriksa identitas penerima." / "For example 'Psychologist at the district Puskesmas'. Sela doesn't check who the recipient is." (D-041).
     - "Untuk apa?" / "What for?", a radio group:
       - "Bahan untuk konsultasi pertama" (material for a first consultation);
       - "Bahan untuk konsultasi lanjutan" (material for a follow-up consultation);
       - "Lainnya" with a text field of up to 80 characters.
  3. **Lama akses** (access duration). A radio group: 24 jam · 3 hari · 7 hari · 14 hari. This is a proposal; v2 has 1, 24 and 72 hours, and the options are an open point for P4. **No option is ever shorter than 20 hours** (v2's 1 hour is dropped), so the share's own expiry stays within the WCAG 2.2.1 20-hour exception for the recipient. Hint: "Setelah itu, tautan berhenti bekerja."
  4. **Pratinjau & persetujuan** (preview and consent).
     - The exact projection the recipient will see (the v2 `ProjectionView`, "Ini persis yang akan dilihat penerima."), with every masked name shown as "[nama disamarkan]" unless the person chose to show it, the generator line on AI-worded items, and Sela's interpretations labelled as such ("Tafsiran Sela · dikonfirmasi pengirim"), never as the person's words.
     - Then the consent checkbox, built from the choices: "Aku mengizinkan Sela membagikan ringkasan versi 3, dengan bagian yang kupilih dan nama yang kusamarkan, kepada 'Psikolog di Puskesmas Kecamatan' untuk 'Bahan untuk konsultasi pertama' selama 7 hari." with the policy version.
     - Then the two fixed notes:
       - "Mencabut akses menghentikan akses berikutnya. Yang sudah dilihat, disalin, atau difoto penerima tidak bisa ditarik kembali.";
       - "Dibagikan atau dibuka tidak berarti kamu sudah mendapat perawatan."
- **Done step:** the share link and the access code (§6.16), the delivery advice ("tautan lewat WhatsApp dan kode lewat SMS atau aplikasi pesan lain"; reading the code out is offered only as the alternative, because a pasted code needs no transcription, WCAG 3.3.8), the acknowledgement checkbox and [Lihat status berbagi].
- **States:**
  - preview loading: "Menyiapkan pratinjau…";
  - preview failed: "Pratinjau belum bisa dibuat. [Coba lagi]". It retries when the connection or the tab returns (the v2 behaviour). Sharing needs a current preview;
  - errors:
    - the summary changed or is not approved: "Ringkasan berubah. Setujui versi terbaru dulu." with a link to S15 step 4;
    - `policy_outdated`: show the new policy and ask again;
  - replay of the same request: "Ini permintaan yang sama; ini tanda terimanya." (the v2 idempotency);
  - success: the done step.
- **Recovery:**
  - The consent checkbox resets whenever any choice changes (v2 rule).
  - One idempotency key per exact payload, so retrying a click never creates two shares.
- **Starting from an earlier share** ("Buat tautan baru" or "Buat versi baru dan bagikan ulang" in S18; WCAG 3.3.7): every step is **pre-filled from the old share**: the parts, the names to show, the recipient label, the purpose and the duration. Each step opens with those choices already selected and shown in the summary line with "Ubah". Only the consent checkbox starts empty, because it is a new consent; the review step says "Tautan lama ke penerima ini akan dicabut saat tautan baru dibuat." A choice that no longer applies (a name no longer in the summary, a part the new version lacks) is dropped and named: "'Pak Budi' tidak ada lagi di ringkasan ini."
- **Keyboard and focus order:** per step, h1, then the summary of earlier choices with Ubah, then the step's controls, then Lanjut. On the review step: the preview region (heading-navigable, not focusable), the consent checkbox, Bagikan. On the done step: Salin tautan, Salin kode, the checkbox, Lihat status.
- **Layout:**
  - 320px: one column; the preview in a bordered region.
  - Wide: on the review step, the preview on the left and the consent on the right, from 75em.
- **API:**
  - planned: `POST /api/v3/shares/preview`; `POST /api/v3/shares` (header `Idempotency-Key`; body with summary version, fields, the names to show, recipient label, purpose, duration, consent and sharing policy version). The access code keeps the v2 format and normalisation ([DESIGN_DIRECTION.md §6.16](DESIGN_DIRECTION.md#616-secrets-recovery-codes-share-link-and-access-code));
  - v2 as the reference: `POST /api/v2/handoffs/preview`, `POST /api/v2/handoffs`.

---

## S18 Share status, audit and revoke

**Route** `/support/shares/:id` · **Access** owner · **Title** "Status berbagi – Sela"

- **Purpose:** know exactly what happened to a share, stop it at any time, and record that a consultation began or ended (D-040). Only recorded events are shown as having happened.
- **Primary action:** depends on the state.
  - Active: "Cabut akses sekarang" / "Revoke access now" (Danger, inline confirm with the Confirm-destructive button).
  - Ended: none.
- **Secondary actions:**
  - "Aku sudah mulai konsultasi" (I've started the consultation; records `consultation_in_progress`, shown as "Kamu menandai: konsultasi berlangsung");
  - "Tandai selesai dan hentikan akses" / "Mark finished and stop access" (records `ended`; the label names the consequence, and an inline confirm says it again: "Penerima tidak bisa membuka ringkasan ini lagi. Kalau penerima masih perlu membacanya untuk tindak lanjut, biarkan saja; aksesnya berakhir sendiri pada 2 Nov 2026, 21.00." [Tandai selesai dan hentikan akses] [Batal]). Writing about the consultation does **not** need this: "Setelah konsultasi" (S19) never changes access;
  - "Lihat yang dibagikan" (see what was shared; the projection);
  - "Buat tautan baru" (make a new link, if the link or code was lost; revokes the old one and needs a new consent; S17 opens pre-filled from this share);
  - "Buat versi baru dan bagikan ulang" (when the recipient asked for clarification; below);
  - "Setelah konsultasi" (S19).
- **Content and data:**
  - the **event list** ([DESIGN_DIRECTION.md §6.8](DESIGN_DIRECTION.md#68-handoff-status-chips-and-the-shares-event-list)), built only from recorded events, each step with its own time or "belum" / "tidak tercatat". For example, after the summary was shown on screen in person and the person then marked the consultation:
    1. "Disiapkan · versi 3 · 26 Okt 2026, 19.40"
    2. "Dibagikan · 26 Okt 2026, 19.45"
    3. "Dibuka penerima · tidak tercatat"
    4. "Diakui penerima · tidak tercatat"
    5. "Kamu menandai: konsultasi berlangsung · 26 Okt 2026, 21.00"

    In an end state the list stays, headed by the end-state chip with its time and **its reason line**, which says who ended it: "Selesai dibagikan · kamu menandai konsultasi selesai", "Selesai dibagikan · penerima tidak dapat menerima", "Akses dicabut · olehmu" or "Kedaluwarsa · 2 Nov 2026, 21.00" ([DESIGN_DIRECTION.md §6.8](DESIGN_DIRECTION.md#68-handoff-status-chips-and-the-shares-event-list));
  - the facts:
    - Untuk (the recipient label, with "identitas tidak diverifikasi");
    - Tujuan (purpose);
    - Isi (the parts shared, and the names kept hidden);
    - Versi ringkasan (summary version);
    - Berakhir (expiry, as a date and time; there is no live countdown);
  - **"Riwayat akses"** (access history), the audit: every recipient access and action, content-free, newest first. For example:
    - "Dibuka · 26 Okt 2026, 21.03";
    - "Diakui penerima · 21.10";
    - "Penerima meminta klarifikasi: 'Ada bagian yang kurang jelas' · 21.20";
    - "Kode akses salah dimasukkan · 20.58 (2 kali)";
    - "Akses dicabut olehmu · 22.00";
  - the fixed sentence "Dibagikan atau dibuka tidak berarti kamu sudah mendapat perawatan."
- **States:**
  - loading: "Memuat status berbagi…" after 400ms;
  - error: "Belum bisa memuat status terbaru." with the last loaded state and its time kept on screen, and [Coba lagi];
  - **clarification requested:** a notice above the event list: "Penerima meminta klarifikasi: 'Ada bagian yang kurang jelas' · 26 Okt 2026, 21.20." / "The recipient asked for clarification: 'Some parts are unclear' · 26 Oct 2026, 21:20." with [Buat versi baru dan bagikan ulang]. That opens S15 to make version 4, then S17 pre-filled from this share (parts, names to show, recipient label, purpose and duration, each with "Ubah"); the review step says "Tautan lama ke penerima ini akan dicabut saat tautan baru dibuat." Doing nothing is also fine: the notice stays until the share ends;
  - **recipient cannot accept:** the end-state chip with its reason, "Selesai dibagikan · penerima tidak dapat menerima · 21.25" / "Sharing ended · the recipient couldn't accept it · 21:25", and a calm next step: "Kalau mau, kamu bisa membagikannya ke orang lain, atau menyimpannya untuk nanti." [Bagikan ke penerima lain] (S17, pre-filled except the recipient label);
  - **marked finished:** "Selesai dibagikan · kamu menandai konsultasi selesai · 21.40", then [Setelah konsultasi];
  - **revoke success:** status "Akses dicabut. Tautan dan kode tidak bisa dipakai lagi." / "Access revoked. The link and code no longer work." with the fixed line about copies already seen;
  - locked after too many wrong codes: "Akses dikunci karena kode salah dimasukkan terlalu sering." [Buat tautan baru].
- **Recovery:** revoking is idempotent, so a repeated tap or a retry is safe (v2). A lost link or code leads to "Buat tautan baru". A failed status change ("Aku sudah mulai konsultasi") keeps the page as it was and says "Belum tersimpan. [Coba lagi]".
- **Keyboard and focus order:**
  1. h1;
  2. the clarification notice and its button, when present;
  3. the event list (not focusable);
  4. the facts;
  5. the action buttons in the order above;
  6. the audit list;
  7. Muat lebih banyak.

  After revoking or marking finished, focus moves to the new status line (`tabindex="-1"`), which shows the ring on any focus, so a touch or magnifier user sees where focus went too ([DESIGN_DIRECTION.md §5.7](DESIGN_DIRECTION.md#57-focus-indicator)).
- **Layout:**
  - 320px: the event list as a vertical list; the facts as a definition list; the actions stacked.
  - Wide: the event list horizontal when it fits (it falls back to vertical at large text); the audit in a side column from 75em.
- **API:**
  - planned: `GET /api/v3/shares/{id}` (with audit events, recipient actions, clarification reasons, and for an end state its actor and reason: `ended_by: user | recipient`, `revoked_reason: user | source_deleted`); `DELETE /api/v3/shares/{id}/access` (revoke); `PATCH /api/v3/shares/{id}` `{status: consultation_in_progress | ended}`; `POST /api/v3/shares/{id}/reshare-draft` (the pre-filled choices for S17);
  - v2 as the reference: `DELETE /api/v2/handoffs/{id}/share`.

## S19 After the consultation

**Route** `/support/summaries/:id/after` · **Access** owner · **Title** "Setelah konsultasi – Sela"

- **Purpose:** carry on reflecting after a consultation, in the person's own words (brief §1 and §15: "Melanjutkan refleksi setelah konsultasi").
- **Primary action:** "Simpan catatan" / "Save note".
- **Secondary actions:**
  - "Cek perasaan" (inline);
  - "Siapkan ringkasan berikutnya" (prepare the next summary; a new version that carries the open questions over);
  - "Mulai bercerita".
- **Content and data:** the lead "Catat dengan kata-katamu sendiri apa yang terjadi. Ini catatanmu, bukan catatan resmi dari profesional." / "Write down, in your own words, what happened. These are your notes, not an official note from the professional."

  The fields are all optional (`user_original`; saved as a note linked to the summary):
  - "Bagaimana rasanya pertemuan itu?" (how did it feel?);
  - "Apa yang ingin kamu ingat?" (what do you want to remember?);
  - "Apa yang disarankan atau disepakati? Tulis dengan kata-katamu." (what was suggested or agreed?);
  - "Apa yang ingin kamu tanyakan lain kali?" (what do you want to ask next time?).

  Also: a link to the summary and its shares, and "Konsultasinya belum terjadi atau batal?" (didn't happen, or cancelled?), which answers "Tidak apa-apa. Ringkasanmu tetap tersimpan." / "That's fine. Your summary stays saved."
- **States:**
  - empty: the fields;
  - loading: the summary link and its shares show "Memuat…" after 400ms; the fields work at once;
  - error: "Belum tersimpan. Tulisanmu masih di sini." [Coba simpan lagi];
  - success: "Tersimpan di Catatanku";
  - crisis match: saved, plus the crisis panel.
- **Recovery:** the text is kept on every error. Leaving with unsaved text asks as in S10. If the linked summary was deleted meanwhile, the note is saved on its own and says "Ringkasan terkait sudah dihapus."
- **Rules:**
  - What a professional said is recorded only as the person's own words (`user_original`), never as `verified_professional_note`.
  - Sela gives no advice on medication the person mentions.
- **Keyboard and focus order:** h1, the four fields, Cek perasaan, Simpan, then the next-step links.
- **Layout:**
  - 320px: stacked.
  - Wide: within `--measure`.
- **API (planned):**
  - `POST /api/v3/journal` `{kind: "follow_up", summaryId, sections}`;
  - `POST /api/v3/check-ins`.

---

## S20 Reading and small exercises

**Routes** `/reading`, `/reading/:slug` · **Access** public; bookmarks for owners · **Title** "Bacaan & latihan kecil – Sela"

- **Purpose:** short, sourced reading and optional small exercises, with no day counters (D-037). The content is public; it is never copied into anyone's notes (D-032).
- **Primary action:** open an item.
- **Secondary actions:**
  - the topic filter chips;
  - "Simpan" (bookmark);
  - on exercises: "Mulai latihan" (start), "Jeda" (pause), "Berhenti" (stop).
- **Content and data:**
  - **List:** reading cards (§6.18) with:
    - title;
    - a one-line description;
    - topics;
    - reading time;
    - source;
    - a "Cocok untuk remaja" (suitable for teens) tag where reviewed.
  - **Item page:**
    - the text, at 18px within `--measure`;
    - "Sumber & lisensi" (source and licence). For the WHO cards: attribution, "Diadaptasi dari …", the licence CC BY-NC-SA 3.0 IGO and the adaptation notice (D-044);
    - "Ditinjau pada 20 Okt 2026" (reviewed on), with the verifier check done at authoring time (D-029);
    - the language (`lang` attribute if it differs from the interface).
  - **Exercises:** numbered steps.
    - An optional timer starts only on "Mulai latihan", with Pause and Stop. It never starts by itself.
    - Under reduced motion it shows "Napas ke-3 dari 5" (breath 3 of 5) as text, with no animation.
    - No sound plays unless the person turns on read-aloud.
- **States:**
  - empty filter: "Belum ada bacaan untuk topik ini." [Hapus filter];
  - loading: "Memuat bacaan…" after 400ms;
  - error: cached items are shown if available, labelled "Ditampilkan dari saat terakhir dimuat";
  - bookmark: "Disimpan ke Bacaan tersimpan" (saved to your reading list). A signed-out person pressing it sees "Masuk atau mulai untuk menyimpan."
- **Recovery:** a list that fails keeps the cached items and offers [Coba lagi]. An item page that fails offers [Coba lagi] and "Kembali ke daftar bacaan". A failed bookmark keeps the button's previous state and says "Belum tersimpan. [Coba lagi]". A running exercise is never stopped by a network error, because its steps are already on the page.
- **Keyboard and focus order:**
  - list: h1, the filter chips, the cards in order;
  - item: h1, the text, Simpan, Sumber & lisensi, then the exercise controls.
- **Layout:**
  - 320px: the cards stacked.
  - Wide: the cards in two columns within `--measure-wide`; the item page at `--measure`.

| Element | ID | EN |
|---|---|---|
| h1 | Bacaan & latihan kecil | Reading & small exercises |
| Lead | Bacaan singkat dan latihan kecil yang bisa kamu coba kapan saja. Semuanya pilihan, tanpa hitungan hari. | Short reads and small exercises you can try any time. All optional, no day counts. |
| Source line | Diadaptasi dari WHO · CC BY-NC-SA 3.0 IGO | Adapted from WHO · CC BY-NC-SA 3.0 IGO |
| Exercise start | Mulai latihan · Jeda · Berhenti | Start · Pause · Stop |
| Bookmark (signed out) | Masuk atau mulai untuk menyimpan. | Sign in or get started to save this. |

- **API (planned):**
  - `GET /api/v3/reading?topic=&audience=&cursor=`;
  - `GET /api/v3/reading/{slug}`;
  - `PUT` and `DELETE /api/v3/reading/{slug}/bookmark`.

---

## S21 Immediate help

**Route** `/help` (directory at `/help#layanan`) · **Access** public · **Title** "Bantuan sekarang – Sela"

- **Purpose:** the fastest path to real, verified help. It never waits for a model, the account or the network (D-035, D-043).
- **Primary action:** contact a service. Each channel is a real link: phone (`tel:`), WhatsApp, or the service's website.
- **Secondary actions:**
  - "Kembali ke percakapan" (back to the conversation; shown when the person came from the room; an unsaved conversation is still held, S09);
  - the directory filters: Untuk remaja, Telepon, WhatsApp, Gratis (only where cost is verified);
  - "Bacaan yang mungkin membantu" (reading that may help).
- **Content and data:**
  1. A static h1 and lead: "Kalau kamu dalam bahaya sekarang, hubungi layanan darurat setempat atau orang terdekat yang bisa menemanimu." / "If you're in danger right now, contact local emergency services or someone nearby who can stay with you." (the v2 `helpNotice`).
  2. "Sela tidak bisa menghubungi siapa pun atas namamu." / "Sela can't contact anyone on your behalf."
  3. "Layanan yang bisa dihubungi sekarang": the urgent services, as service cards (§6.15).
  4. "Langkah kecil untuk beberapa menit ke depan" (small steps for the next few minutes). Static content, shown **only after review** (P6). Until then this section is absent.
  5. "Layanan bantuan lainnya" (`#layanan`): the full verified directory.
  6. "Daftar ini hanya untuk Indonesia. Bahasa yang kamu pilih tidak menentukan negara atau layanan yang tersedia untukmu."

  Every entry carries its source, verification date and re-verify-by date. Stale entries carry the warning chip (D-043). Teen entries appear only if verified, and the unverified KPAI WhatsApp entry from v1 is never shown. For age band 16–17 or "unknown", the teen services are listed first. When no verified teen entry exists, the list falls back to the static lead (item 1) and the verified general entries; it is never empty.
- **States:**
  - **offline or API down:** rendered from the bundled data (as the v2 `help.fallback.json`), with "Data bawaan aplikasi, diperiksa 26 Okt 2026." / "Built-in data, checked 26 Oct 2026.";
  - loading: none, because the content renders from the bundle at once and refreshes silently;
  - error: none visible, because the bundle stays.
- **Keyboard and focus order:**
  1. shell (the top bar's help link has `aria-current="page"`);
  2. h1;
  3. each service's channel links in order;
  4. Kembali ke percakapan;
  5. the directory filters;
  6. the directory entries.
- **Layout:**
  - 320px: service cards stacked; channel links as full-width buttons, at least 44px and labelled with the service name ("Telepon Healing119", not "Telepon").
  - Wide: the urgent services in two columns; the directory below.
- **API:**
  - the bundled data;
  - `GET /api/v2/catalog` (help field; v2) until `GET /api/v3/reading/help-services` exists (planned P6).

---

## S22 Privacy and data settings

**Route** `/settings/privacy` (the policy texts are S28) · **Access** owner · **Title** "Privasi & data – Sela"

- **Purpose:** see and control every privacy decision in one place (brief §7 and §15; UU PDP rights summarised in the facts report).
- **Primary action:** depends on the section.
- **Content and data:** an in-page list of links (Penyimpanan · Memori · Nama yang disamarkan · Pemrosesan AI · Ekspor · Sesi & perangkat · Riwayat akses · Persetujuan & kebijakan · Hapus data), then the sections, each with an `h2` and an `id` for deep links (`#penyimpanan`, `#memori`, `#nama`, `#ai`, `#ekspor`, `#sesi`, `#riwayat`, `#persetujuan`, `#hapus`):
  1. **Penyimpanan** (storage).
     - Where data lives: "Server Sela (database), wilayah: [label] · terenkripsi per orang". The region is pending B-3/B-15.
     - The default for new conversations (a storage choice, §6.9), with the "Jangan simpan" hint that names provider retention.
     - "Sembunyikan pratinjau isi" (hide content previews). It also hides summary titles, recipient labels and the crisis follow-up banner.
     - For guests: the dates of automatic deletion, and the shared-device line: "Siapa pun yang memakai browser ini bisa membuka data tamumu. Di perangkat bersama, pakai akun atau sembunyikan pratinjau isi." [Simpan ke akun].
  2. **Memori** (memory), each control with its scope line ([DESIGN_DIRECTION.md §6.9](DESIGN_DIRECTION.md#69-consent-storage-memory-and-private-name-controls)):
     - the `personal_memory` consent card ("Izin supaya Sela boleh menyimpan dan memakai yang kamu setujui. Mencabutnya melupakan semuanya."); withdrawing opens the S13 withdrawal dialog, which forgets everything;
     - "Jeda memori" ("Berlaku untuk semua percakapan mulai sekarang, termasuk yang sedang terbuka. Yang Sela ingat tetap tersimpan.");
     - "Pakai yang Sela ingat di percakapan baru" / "Use what Sela remembers in new conversations", Ya / Tidak ("Berlaku untuk percakapan yang kamu mulai setelah ini. Kamu tetap bisa mengubahnya di setiap percakapan.");
     - a link to S13.
  3. **Nama yang disamarkan** (names kept private, `#nama`): the list of names and words the person keeps private, with [Tambahkan] and [Hapus dari daftar] per row, and the explanation of what masking does ([DESIGN_DIRECTION.md §6.9](DESIGN_DIRECTION.md#69-consent-storage-memory-and-private-name-controls)). Empty: "Belum ada nama di daftar ini. Nama yang kamu tambahkan diganti dengan [nama disamarkan] sebelum dikirim ke AI dan di ringkasan yang kamu bagikan." Contact details are always masked for AI and need no list.
  4. **Pemrosesan AI** (AI processing, `#ai`): three consent cards (§6.9), each naming provider, place and purpose (D-048):
     - "Balasan & penyusunan · Meta, di Modal (Amerika Serikat)";
     - "Pemeriksa · Claude dari Anthropic (Amerika Serikat)", with the same plain line as onboarding: "memeriksa balasan yang menyebut hal kesehatan, ringkasan, dan hal yang diusulkan untuk diingat";
     - "Cadangan · DeepSeek (Tiongkok)". It explains: "Dipakai hanya jika model utama tidak tersedia, tidak pernah untuk melewati penolakan keamanan."

     And the fixed lines "Mencabut izin menghentikan pemakaian penyedia itu mulai sekarang. Yang sudah terkirim ke penyedia tidak bisa ditarik kembali; penyedia menyimpannya sesuai kebijakannya." / "Withdrawing stops that provider's use from now on. What was already sent can't be recalled from the provider, which keeps it under its own policy." (ADR-0003 §8.2).

     Below the cards: "Pertanyaan izin di tengah fitur" / "Permission questions inside features": when the person chose "Jangan tanya lagi" somewhere, a line lists it with [Tanyakan lagi saat diperlukan] / [Ask again when needed].
  5. **Ekspor** (export).
     - Format radio:
       - "Mudah dibaca (HTML)";
       - "Teks (Markdown)";
       - "Untuk aplikasi lain (JSON)".
     - Language radio.
     - [Unduh data] (download).
     - The note: "File ini berisi semua yang kamu simpan di Sela. Simpan di tempat aman; siapa pun yang membuka file ini bisa membacanya."
     - Accounts are asked for their password in a dialog first (reauthentication within 10 minutes; `reauth_required`). The dialog holds a read-only "Masuk sebagai pelangi.senja" field (`autocomplete="username"`) and the password field (`autocomplete="current-password"`), so a password manager can fill it ([DESIGN_DIRECTION.md §6.3](DESIGN_DIRECTION.md#63-inputs)).
  6. **Sesi & perangkat** (sessions and devices).
     - **Accounts:**
       - the live sessions: "Firefox · Android · sesi ini", the created and last-seen times (rounded to the hour) and the expiry;
       - [Keluarkan] (sign out) per session; after it, focus moves to the next session's button or to the section heading;
       - [Keluarkan semua perangkat lain] (sign out of all other devices);
       - [Keluar dari semua perangkat] (sign out everywhere, this one included; a dialog).
     - **Guests:** the one session, **read only** (browser, created time, the guest dates), with the explanation from S24: "Tamu tidak punya tombol keluar, karena tanpa akun, keluar berarti datamu tidak bisa dibuka lagi." The only actions are [Simpan ke akun] (S06) and [Hapus data tamu sekarang] (section 9). The API allows guests to revoke sessions and log out (API_SPEC_V3 §7), but for a guest either one locks them out of their own data for good, so the UI does not offer it.
  7. **Riwayat akses** (access history).
     - The account audit list with plain labels for each event type, for example "Masuk berhasil", "Persetujuan diberikan: …", "Ekspor dibuat (HTML)", "Kode pemulihan dipakai".
     - Share accesses are listed per share in S18 and summarised here.
     - Pagination.
  8. **Persetujuan & kebijakan** (consents and policies): every consent with its state and version, the history ("Riwayat persetujuan") and links to the policy texts.
  9. **Hapus data** (delete data).
     - What will be deleted: everything, including summaries, shares (recipient access stops) and memory.
     - The timeline: "Sekarang: hilang dari Sela · Dalam 7 hari: tidak ada cadangan yang bisa membukanya · Dalam 30 hari: hilang dari semua cadangan." (ADR-0002 §11).
     - What cannot be recalled: "salinan yang sudah kamu unduh atau sudah dilihat penerima".
     - The suggestion: "Ekspor dulu kalau kamu ingin menyimpan salinan."
     - The checkbox "Aku mengerti ini tidak bisa dibatalkan".
     - A password prompt for accounts, with the read-only username field (the uniform credential-error pattern).
     - [Hapus akun dan semua data] or, for guests, [Hapus data tamu sekarang] (Danger; the dialog's final button is Confirm-destructive).
- **States:**
  - each section loads and fails on its own;
  - success messages are inline statuses;
  - after deletion: S27 "Datamu sudah dihapus", with the dates.
- **Recovery:**
  - `reauth_required` shows the password dialog again;
  - export `rate_limited` shows "Kamu sudah mengekspor beberapa kali dalam satu jam. Coba lagi nanti.";
  - deletion retried after success gives `401`, which the UI treats as done (API_SPEC_V3 §6).
- **Keyboard and focus order:**
  1. h1;
  2. the in-page links (each moves focus to its section `h2`);
  3. the sections in order.

  Dialogs return focus to their trigger.
- **Layout:**
  - 320px: the sections stacked; the in-page links as a wrapped list.
  - Wide: the in-page links as a sticky side list in the main column from 75em (sticky inside the content, not fixed to the viewport).

| Element | ID | EN |
|---|---|---|
| h1 | Privasi & data | Privacy & data |
| Lead | Semua pilihan tentang datamu ada di sini. Kamu bisa mengubahnya kapan saja. | Every choice about your data is here. You can change any of them at any time. |
| Export note | File ini berisi semua yang kamu simpan di Sela. Simpan di tempat aman; siapa pun yang membuka file ini bisa membacanya. | This file contains everything you've saved in Sela. Keep it somewhere safe; anyone who opens it can read it. |
| Delete timeline | Sekarang: hilang dari Sela · Dalam 7 hari: tidak ada cadangan yang bisa membukanya · Dalam 30 hari: hilang dari semua cadangan. | Now: gone from Sela · Within 7 days: no backup can open it · Within 30 days: gone from every backup. |
| Delete button | Hapus akun dan semua data | Delete my account and all data |
| Session row | Firefox · Android · sesi ini | Firefox · Android · this session |
| Names section | Nama yang disamarkan | Names kept private |
| Memory default | Pakai yang Sela ingat di percakapan baru | Use what Sela remembers in new conversations |

- **API:** `GET` and `PUT /api/v3/me/consents[/{purpose}]`, `GET /api/v3/me/consents/history`, `GET /api/v3/me/export?format=&lang=`, `POST /api/v3/auth/reauth`, `GET /api/v3/me/sessions`, `DELETE /api/v3/me/sessions/{id}` and `POST /api/v3/auth/logout-all` (accounts only in this UI), `GET /api/v3/me/audit`, `DELETE /api/v3/me` and `GET /api/v3/policies` (all v3 spec). The AI and memory purposes are reserved (planned P3/P5). The names list, the memory default, the "ask again" flags and the "answered the checker question" flag are planned (P3/P5).

---

## S23 Display and accessibility settings

**Route** `/settings/display` · **Access** public · **Title** "Tampilan & aksesibilitas – Sela"

- **Purpose:** let each person set the interface to their needs, with no assumptions by age (brief §6). Changes apply immediately and are stored on this device.
- **Primary action:** none. Each control applies at once, with a status such as "Ukuran teks: Lebih besar".
- **Secondary action:** "Kembalikan ke bawaan" (reset to defaults).
- **Content and data:** the preferences in [DESIGN_DIRECTION.md §5.9](DESIGN_DIRECTION.md#59-display-preferences), grouped under `h2` headings. Control types follow §6.3: three-way choices are radio groups, two-state preferences are switches, and anything that sends data elsewhere uses a button after its disclosure.
  - **Teks** (text): size (radio group) and relaxed spacing (switch);
  - **Warna** (colour): theme (radio group: Ikuti sistem / Terang / Gelap) and high contrast (radio group: Ikuti sistem / Ya / Tidak);
  - **Gerakan** (motion): reduce motion (radio group: Ikuti sistem / Ya / Tidak) and show replies at once (switch);
  - **Percakapan** (conversation): "Enter untuk mengirim" (switch);
  - **Pembaca layar** (screen readers): "Umumkan balasan ke pembaca layar" / "Announce replies to screen readers" (switch, on by default), with the hint "Untuk pembaca layar. Ini tidak memutar suara; untuk suara, pakai Bacakan dengan suara." It controls only whether a released reply is sent to the reply live region; it is kept apart from Suara so nobody turns it on expecting audio, or off without knowing it silences their screen reader;
  - **Suara** (voice): "Bacakan dengan suara" / "Read aloud" (switch; local voices only; speech synthesis) and dictation (the "Nyalakan dikte" button, below);
  - **Keselamatan** (safety):
    - the Quick exit shortcut (switch), with "Tekan Esc dua kali dengan cepat. Esc pertama menampilkan petunjuk; menutup dialog dengan Esc tidak ikut dihitung." / "Press Esc twice quickly. The first press shows a hint; closing a dialog with Esc doesn't count.";
    - **"Keluar cepat juga mengeluarkan akun"** / "Quick exit also signs you out" (accounts only; radio group): "Kalau aku tidak memilih 'Ingat perangkat ini'" (default) / "Selalu" / "Tidak pernah". Hint: "Di perangkat bersama, sebaiknya akunmu ikut dikeluarkan. Mode tamu tidak pernah dikeluarkan, karena tanpa akun kamu tidak bisa masuk lagi.";
    - the Quick exit destination (planned), neutral tab titles (planned) and hide previews (which also appears in S22);
  - **Bahasa** (language).

  A live preview shows a sample message pair ("Kamu" / "Sela") and a provenance badge with its status chip at the current settings. The note: "Pengaturan ini disimpan di perangkat ini saja." / "These settings are saved on this device only."
- **Dictation:** the disclosure is shown in full, followed by the button **"Nyalakan dikte" / "Turn on dictation"**, which states the outcome: when pressed, the status says "Dikte aktif. Mikrofon baru diminta saat kamu menekan tombol mikrofon di kotak tulis." and the button becomes "Matikan dikte" / "Turn off dictation". There is no switch, because dictation may send audio to the browser vendor.
- **Read aloud:** lists the local voices only, or says "Tidak ada suara lokal untuk bahasa ini di perangkatmu."
- **States:**
  - storage blocked: "Pengaturan berlaku sampai tab ini ditutup." / "Settings apply until this tab is closed.";
  - success: the inline status.
- **Recovery:** "Kembalikan ke bawaan" restores every default at once and says so; a setting that makes the page hard to use (for example a very large text size) can be undone from the same place, because the controls reflow with the text.
- **Keyboard and focus order:** h1, then each group's controls in order, then the reset button. Radios use arrow keys; switches and buttons use Space or Enter.
- **Layout:**
  - 320px: stacked; the preview after the text group.
  - Wide: the controls on the left; the sticky preview on the right, from 75em.
- **API:** none (`localStorage`). Sync to the account is planned.

## S24 Account

**Route** `/settings/account` · **Access** owner · **Title** "Akun – Sela"

- **Purpose:** manage sign-in, recovery, age band and guardian consent. For guests: upgrade or delete.
- **Primary action:**
  - accounts: none fixed;
  - guests: "Simpan ke akun" (S06).
- **Content and data:**
  - **Account:**
    - the username;
    - "Ganti kata sandi" (change password: a read-only username field, then current (`current-password`) and new (`new-password`); other sessions are signed out);
    - "Kode pemulihan: 9 tersisa · dibuat 20 Okt 2026" with [Buat kode baru] (needs the password, asked with a read-only username field as in S22; the old codes stop working);
    - "Usia: 18 tahun ke atas" with [Ubah]. Choosing under 15 shows the S27 explanation and offers export and deletion; nothing changes;
    - "Persetujuan wali" (guardian consent; for 16–17, S26);
    - "Keluar dari akun" / "Sign out" (never shortened to "Keluar", which is never used for signing out);
    - "Keluar dari semua perangkat" (sign out everywhere).
  - **Guest:**
    - "Mode tamu" with the dates and where the data is stored;
    - [Simpan ke akun];
    - [Hapus data tamu sekarang] (the S22 delete flow).
    - **No sign-out button**, because it would lock the guest out of their own data. The explanation: "Tamu tidak punya tombol keluar, karena tanpa akun, keluar berarti datamu tidak bisa dibuka lagi."
- **States:**
  - loading: "Memuat akunmu…" after 400ms; the rows keep their space so nothing jumps;
  - errors: `current_password_incorrect` ("Kata sandi saat ini tidak cocok.") under the current-password field, and the password rules under the new one;
  - success:
    - password changed: "Kata sandi diganti. 2 perangkat lain dikeluarkan.";
    - new codes: shown once (§6.16).
- **Recovery:** typed values are kept after an error, passwords included (masked, with "Tampilkan"); after `current_password_incorrect` the current-password field is focused with its text selected, so it can be retyped at once. A lost password-change response means the new password is already active: "Coba masuk dengan kata sandi baru." A lost recovery-code response: regenerate again (the earlier batch is already invalid). A failed load offers [Coba lagi]; "Keluar dari akun" and help keep working.
- **Keyboard and focus order:** h1, then each row's action in order.
- **Layout:**
  - 320px: rows stacked.
  - Wide: within `--measure-wide`.

| Element | ID | EN |
|---|---|---|
| h1 | Akun | Account |
| Sign out | Keluar dari akun | Sign out |
| Recovery row | Kode pemulihan: 9 tersisa · dibuat 20 Okt 2026 | Recovery codes: 9 left · made 20 Oct 2026 |
| Password changed | Kata sandi diganti. 2 perangkat lain dikeluarkan. | Password changed. 2 other devices were signed out. |
| Guest, no sign-out | Tamu tidak punya tombol keluar, karena tanpa akun, keluar berarti datamu tidak bisa dibuka lagi. | Guests have no sign-out button, because without an account, signing out would lock you out of your data. |
| Age change | Usia: 18 tahun ke atas · Ubah | Age: 18 or older · Change |

- **API:** `GET` and `PATCH /api/v3/me`, `POST /api/v3/auth/password`, `POST /api/v3/auth/recovery-codes`, `POST /api/v3/auth/logout`, `POST /api/v3/auth/logout-all` and `GET`/`PUT`/`DELETE /api/v3/me/guardian-consent` (all v3 spec).

---

## S25 Recipient page

**Route** `/r#t=<token>` · **Access** recipient (separate principal; [INFORMATION_ARCHITECTURE.md §5.3](INFORMATION_ARCHITECTURE.md#53-the-recipient-is-separated)) · **Title** "Halaman penerima – Sela"

- **Purpose:** let the person the user invited read exactly what was shared, and acknowledge it. Nothing more, and with honest labels.
- **Primary action:**
  - code step: "Buka ringkasan" / "Open the summary";
  - view: "Konfirmasi sudah diterima" / "Confirm receipt".
- **Secondary actions:**
  - "Minta klarifikasi" (ask for clarification): a radio group of fixed reasons, with no free text, so no recipient text ever looks like a professional note (D-039 reserves `verified_professional_note`). The reasons (pending P4):
    1. "Ada bagian yang kurang jelas" / "Some parts are unclear";
    2. "Mohon dalam bahasa lain" / "Please share it in the other language";
    3. "Mohon sertakan pertanyaan yang ingin Anda bicarakan" / "Please include the questions you want to discuss";
    4. "Mohon bagikan lagi dengan waktu akses lebih lama" / "Please share again with a longer access time".

    [Kirim permintaan] sends it; the status says "Permintaan terkirim. Pengirim akan melihatnya di Sela." The sender sees it in S18;
  - "Tidak dapat menerima" (cannot accept): a Danger button with an **inline confirm**, because it ends the sender's share for good: "Ini mengakhiri akses Anda ke ringkasan ini, dan pengirim akan diberi tahu bahwa Anda tidak dapat menerimanya." / "This ends your access to this summary, and the sender will be told that you can't accept it." [Ya, saya tidak dapat menerima] [Batal]. Batal returns focus to the trigger;
  - "Keluar dari halaman ini" (leave this page; ends the recipient session);
  - the language switch (local only).
- **Content and data:**
  - **Banner** (always), matching the summary's content (D-038 "where applicable"): "Anda membuka ringkasan yang dibagikan seseorang melalui Sela. Ini catatan pribadi pengirim. Bukan rekam medis. Sela tidak memverifikasi identitas Anda atau pengirim." / "You are viewing a summary someone shared through Sela. These are the sender's personal notes. Not a medical record. Sela does not verify your identity or the sender's." Then one of: "Sebagian kalimatnya ditulis dengan bantuan AI (Meta) dan diperiksa oleh Claude, lalu dikonfirmasi pengirim." / "Some of the wording was written with AI help (Meta) and checked by Claude, then confirmed by the sender." or "Disusun sendiri oleh pengirim, tanpa AI." / "Put together by the sender, without AI." The line is built from the frozen projection's stored provenance: a part checked by a test program reads "… diperiksa oleh program uji (bukan AI) …", never "Claude", and the "Mode uji" banner is shown above it.
  - **The projection:**
    - the summary points, each with its provenance as text and, for AI wording, the generator line: "Kata-kata pengirim"; "Tulisan AI · Ditulis dengan bantuan AI (Meta) · diperiksa oleh Claude · dikonfirmasi pengirim" (with DeepSeek, the alternative checker or the test program named when used); "Koreksi pengirim";
    - **a Sela interpretation the sender confirmed** (an item that came from "Yang Sela ingat") is never shown as the sender's words: "Tafsiran Sela · dikonfirmasi pengirim" / "Sela's interpretation · confirmed by the sender", with its generator line and no quote styling;
    - opening lines from a template that quote AI wording: "Tulisan AI · dari templat Sela, memakai kalimat AI (Meta · diperiksa oleh Claude) · dikonfirmasi pengirim";
    - masked names as "[nama disamarkan]" / "[name hidden]", unless the sender chose to show them;
    - only the parts chosen, each with `lang` when it differs from the page language;
    - the purpose ("Tujuan: Bahan untuk konsultasi pertama");
    - the age group if shared ("Usia: 16–17 tahun");
    - the expiry ("Akses berakhir 2 Nov 2026, 21.00");
    - the recipient label the user wrote ("Ditujukan untuk: Psikolog di Puskesmas Kecamatan").
  - **Footer:** "Tentang halaman ini" (about this page).
- **States:**
  - no token: "Tautan tidak lengkap. Buka tautan lengkap yang dikirim kepada Anda.";
  - **code step:**
    - "Masukkan kode akses yang dikirim terpisah." / "Enter the access code sent to you separately." The field uses `autocomplete="one-time-code"`, `autocapitalize="characters"`, `spellcheck="false"` and `inputmode="text"`; paste is allowed. Hint: "Kode terdiri dari 8 huruf dan angka. Huruf besar/kecil, spasi, dan tanda hubung tidak berpengaruh. Kode tidak memakai angka 0 dan 1, atau huruf O, I, L, dan U." / "The code has 8 letters and digits. Upper or lower case, spaces and hyphens don't matter. It never uses 0, 1, O, I, L or U." (the v2 normalisation and alphabet);
    - **format check first, with no attempt used** ([DESIGN_DIRECTION.md §6.16](DESIGN_DIRECTION.md#616-secrets-recovery-codes-share-link-and-access-code)): after normalisation, a code that is not 8 characters long or contains 0, 1, O, I, L or U gets a field error under the code field ("Perlu diperbaiki: kode terdiri dari 8 huruf dan angka. Angka 0 dan 1, dan huruf O, I, L, dan U tidak dipakai."), `aria-invalid="true"` and focus on the field, and nothing is sent. Five wrong codes lock the share for good (v2), so attempts are only spent on well-formed codes;
    - every server-side failure, whether a wrong code, a lock, revocation or expiry, shows the **same** text: "Akses ke ringkasan ini tidak tersedia. Tautan atau kodenya salah, atau aksesnya sudah berakhir." (the v2 uniform `access_ended`), in the form-level error region. **The typed code is kept** and focus moves to the code field, so a typo can be fixed;
  - view loading: "Memuat ringkasan…" after 400ms;
  - **access ends soon** (less than about 15 minutes left): a static line above the projection, "Akses ke ringkasan ini berakhir pukul 21.00." / "Access to this summary ends at 21:00." It appears once, at that point or on load, and never counts down. The expiry is not extendable here, because extending it would go beyond the sender's consent (the 2.2.1 essential exception);
  - after acknowledging: "Terima kasih. Pengirim akan melihat bahwa Anda sudah menerimanya.";
  - after "Tidak dapat menerima": "Akses Anda sudah diakhiri. Pengirim akan diberi tahu." and the projection is removed;
  - **ended mid-view** (revoked or expired): the next request shows the uniform ended text, and the projection is removed from the screen;
  - **the recipient session itself** (proposal for P4; [DESIGN_DIRECTION.md §10](DESIGN_DIRECTION.md#10-open-points-for-the-pm-and-other-owners) item 8): it has an idle limit but no absolute limit shorter than the share. At least 2 minutes before the idle limit, a dialog says "Sesi halaman ini akan berakhir dalam sekitar 2 menit karena tidak ada aktivitas." / "This page's session will end in about 2 minutes because of inactivity." [Tetap buka / Keep it open] (any number of times) [Keluar dari halaman ini] (WCAG 2.2.1). If it ends anyway, the page does **not** show the uniform ended text: it says "Sesi halaman ini berakhir karena tidak ada aktivitas. Untuk membuka lagi, masukkan kode akses yang sama." / "This page's session ended because of inactivity. To open it again, enter the same access code." with the code field, using the link token still held in this tab (it was scrubbed from the address bar, but not from memory). If the tab was reloaded, it says "Buka lagi tautan yang dikirim kepada Anda, lalu masukkan kode yang sama." If the share was revoked or expired meanwhile, re-entering the code gives the uniform text, so this page reveals nothing about the share's state.
- **Recovery:** the uniform error page says what the recipient can do: "Kalau Anda masih memerlukannya, mintalah tautan dan kode baru kepada pengirim." / "If you still need it, ask the sender for a new link and code." A wrong code can be retried until the lock; the field keeps the typed code so a typo can be fixed. A failed acknowledgement or clarification keeps the page and offers [Coba lagi].
- **Keyboard and focus order:**
  - code step: h1, the error region (not focusable), the code field, Buka ringkasan;
  - view: h1, the banner (not focusable), the near-expiry line (not focusable), the projection (heading-navigable), Konfirmasi, Minta klarifikasi, Tidak dapat menerima, Keluar dari halaman ini.
- **Layout:**
  - 320px: one column; the banner at the top in flow.
  - Wide: within `--measure`.
  - A print stylesheet is **not** offered as a button. The page says: "Sela tidak menyediakan unduhan. Mohon tidak menyimpan salinan di luar kebutuhan konsultasi." This cannot prevent copying, and the sender has already been told so in S17.
- **Copy:** formal "Anda" register ([CONTENT_STYLE.md §9](CONTENT_STYLE.md#9-recipient-copy)).
- **API:**
  - planned: `POST /api/v3/recipient/sessions` `{token, code}`; `GET /api/v3/recipient/share` (with the session's idle expiry); `POST /api/v3/recipient/session/extend` ("Tetap buka"); `POST /api/v3/recipient/share/actions` `{action: acknowledge | clarify | cannot_accept, reasonId?}`; `DELETE /api/v3/recipient/session`. v2's 2-hour absolute `RecipientTTL` is not carried over;
  - v2 as the reference: `/api/v2/recipient/*`, with `randomCode` and `normalizeCode` kept for the code format.

## S26 Minors variants (15-17)

**PM decision D-030:** implemented, but disabled in production by configuration until a legal review (B-10). Enabled in development, demo and staging. **15-year-olds stay blocked** until Sela is assessed as low risk under PP 17/2025 Pasal 21(1)(b).

### Age split (mandatory now)

The age step (S02 step 1) asks "15 tahun" and "16–17 tahun" separately.

- "15 tahun" goes to S27 "15 tahun" **in every environment**, with teen help. Nothing is created. The UI enforces this today.
- The open dependency is only the API: `ageBand` has `15_17` alone, so the server cannot enforce the block yet ([DESIGN_DIRECTION.md §10](DESIGN_DIRECTION.md#10-open-points-for-the-pm-and-other-owners) item 10). Until it can, "16–17 tahun" is sent as `15_17`.
- "Aku tidak ingin menyebutkan" (`unknown`) goes through the confidentiality step too (ADR-0002 §3), with no guardian step, and gets **every other minor default** (the table below). Whether `unknown` may skip guardian consent, and whether it must be disabled wherever `15_17` is enabled for real users, is open for the legal review ([DESIGN_DIRECTION.md §10](DESIGN_DIRECTION.md#10-open-points-for-the-pm-and-other-owners) item 20). In every environment where `unknown` is enabled it is otherwise a way around both the guardian step and the 15 block.

### Teen onboarding additions

The confidentiality step comes before the storage step, so nothing is stored before the teen has read it.

**Guardian consent and creation (decision, with an API dependency).** Nothing should exist if the guardian says no (PP 17/2025 Pasal 9(4)–(5): the service is not provided and the child's data is deleted).

- **Target:** the guardian step runs **before** the storage step, its result is kept in memory, and it is sent together with guest start or registration (proposed API change; [DESIGN_DIRECTION.md §10](DESIGN_DIRECTION.md#10-open-points-for-the-pm-and-other-owners) item 11). A decline then creates nothing.
- **Interim, until that API exists:** 16–17 may start **only as a guest** (S02 step 2). The guardian step runs right after the guest is created and before any content can be written. A decline calls `DELETE /api/v3/me` (guests need no reauthentication) and says "Data yang baru dibuat sudah dihapus." An account can be made later through "Simpan ke akun" (S06), once guardian consent is recorded.

**`/start/teen`: Batas kerahasiaan (limits of confidentiality).**

- **Purpose:** explain plainly what stays private and where the limits are, before anything is stored (brief §6).
- **Content (teen copy):**
  - "Apa yang kamu tulis di Sela tidak dikirim ke orang tuamu, walimu, atau sekolah." / "What you write in Sela isn't sent to your parents, guardian or school."
  - "Orang tua atau walimu perlu memberi persetujuan supaya kamu bisa memakai Sela. Persetujuan itu tidak membuat mereka bisa membaca ceritamu." / "Your parent or guardian needs to agree to you using Sela. That doesn't let them read what you write." (Not shown for "unknown".)
  - "Sela tidak akan menghubungi siapa pun, termasuk saat kamu dalam bahaya. Karena itu Sela menunjukkan layanan bantuan yang bisa kamu hubungi sendiri." / "Sela won't contact anyone, even if you're in danger. That's why it shows you services you can contact yourself."
  - "Batasnya: orang yang memegang HP-mu bisa membuka Sela jika kamu tetap masuk. Orang yang menjalankan server Sela secara teknis bisa mengakses data, dan hukum bisa mewajibkan data diberikan dalam keadaan tertentu." / "The limits: someone holding your phone can open Sela if you stay signed in. The people who run Sela's servers can technically access data, and the law can require data to be handed over in some situations." The legal wording is pending legal review (B-10).
  - "Kalau kamu membagikan ringkasan, penerimanya bisa membaca apa yang kamu pilih." / "If you share a summary, the recipient can read what you chose."
- **Primary action:** "Aku mengerti, lanjut" / "I understand, continue". There is no quiz (3.3.8 spirit: no cognitive test).
- **Recovery:** Back returns to the age step; nothing has been sent.

**`/start/guardian`: Persetujuan orang tua atau wali (parent or guardian consent).**

- **Purpose:** the guardian consent record. It **grants no access** (API_SPEC_V3 §6; D-030).
- **Flow:**
  1. The teen sees "Minta orang tua atau walimu membaca layar berikutnya bersamamu." / "Ask your parent or guardian to read the next screen with you." [Lanjut].
  2. **A screen addressed to the guardian**, in formal "Anda" register, from the `guardian_information` policy:
     - what Sela is;
     - that the guardian **will not** see conversations, notes or summaries;
     - that Sela never contacts anyone;
     - where the data is stored;
     - how to withdraw.
  3. Controls:
     - the relationship radio ("Orang tua" / "Wali yang sah");
     - the optional name ("Nama Anda (opsional)", at most 80 characters, no contact details);
     - the checkbox "Saya orang tua/wali anak ini. Saya setuju dia memakai Sela, dan saya mengerti saya tidak mendapat akses ke isi ceritanya.";
     - [Saya setuju] and [Saya tidak setuju], of equal weight.
  4. The honest line: "Sela tidak dapat memastikan siapa yang menekan tombol ini." / "Sela can't confirm who presses this button." The method is `guardian_present_attestation`. Whether it is enough under PP 17/2025 Pasal 9 and 22 is pending legal review (B-10).
  5. Hand back to the teen: "Terima kasih. Sekarang serahkan kembali kepada anak Anda." [Lanjut].
- **States:**
  - **the guardian declines** ("Saya tidak setuju"): in the target flow nothing was created; in the interim flow the client calls `DELETE /api/v3/me` first. The screen says "Tanpa persetujuan orang tua atau wali, Sela belum bisa dipakai. Data yang baru dibuat sudah dihapus. Bantuan sekarang tetap bisa kamu buka kapan saja." / "Without a parent's or guardian's agreement, Sela can't be used yet. The data that was just created has been deleted. You can still open Help now at any time." with the teen help list. If the deletion request fails, it is retried; if it still fails, the screen says so and the purge job removes the guest by its schedule;
  - `policy_outdated`: reload;
  - `server_busy`: [Coba lagi], with the choices kept.
- **API:** `PUT /api/v3/me/guardian-consent` (v3 spec); `DELETE /api/v3/me` on decline (interim). Sending the attestation with `POST /api/v3/auth/guest` and `/auth/register` is a proposal to the API team.

### Differences on other screens for 16–17 and for `unknown`

`unknown` ("Aku tidak ingin menyebutkan") gets **the protective defaults of `15_17`** (ADR-0002 §3), so every row applies to it too, **except the guardian rows** (the API records guardian consent for `15_17` only). The last column says where `unknown` differs.

| Screen | Difference for 16–17 | `unknown` |
|---|---|---|
| S02 Onboarding | The confidentiality step; the guardian step; guest only (interim); no DeepSeek card | The confidentiality step without the guardian sentence; **no guardian step**; guest or account as for adults (no guardian consent to wait for); no DeepSeek card |
| S07 Home | A yearly "Apakah usiamu masih 16–17 tahun?" card (`confirm_age_band`). When `guardian_consent` is required, an interstitial covers the places (privacy, display, account, help and reading stay open) | No yearly card and no guardian interstitial. Account offers "Ubah usia" as for everyone |
| S09 Room | The room intro uses teen copy. **No in-context memory question:** proposals are not offered until memory is turned on in S13. Crisis panel services list teen services first (verified only; the general verified list when none is verified) | The same: teen copy, no in-context memory question, teen services first |
| S12 Timeline | The pattern view is opt-in only (PP 17/2025 Pasal 19, profiling) | The same |
| S13 Memory | Off by default. The consent card explains in teen copy: "Sela bisa mengingat hal yang kamu setujui. Ini mati sampai kamu menyalakannya." | The same |
| S15, S16 AI help | As for adults, but never with DeepSeek (no fallback generator and no alternative checker, ADR-0003 §8.3) | The same |
| S17 Sharing | The "Kelompok usia" field shows "16–17 tahun". A note: "Penerima akan tahu usiamu 16–17 tahun jika kamu memilih bagian ini." | The field shows "Tidak disebutkan" / "Not stated", with the note "Penerima akan tahu kamu tidak menyebutkan usiamu jika kamu memilih bagian ini." |
| S20 Reading | Items flagged "Cocok untuk remaja" first. Items not reviewed for teens are labelled "Untuk 18 tahun ke atas" and can still be opened (no hidden content) | The same |
| S21 Help | Teen services first (verified only) | The same |
| S22 Privacy & data | The DeepSeek card is not shown. In its place: "Model cadangan tidak dipakai untuk usia di bawah 18 tahun." / "The backup model isn't used under 18." (ADR-0003 §8.3). The Claude and Meta cards appear only where the band is enabled | The DeepSeek card is not shown, with "Model cadangan tidak dipakai jika usia tidak disebutkan." / "The backup model isn't used when age isn't stated." Claude and Meta only where the band is enabled |
| S24 Account | The guardian consent row: status, "Persetujuan dicatat 26 Okt 2026 oleh: Orang tua", [Catat ulang] and [Cabut]. Withdrawal → gate (see below) | No guardian row |

**Guardian consent withdrawn or missing:**

- Every place shows: "Persetujuan wali belum ada atau sudah dicabut. Sela tidak bisa dipakai sampai persetujuan dicatat lagi. Kamu tetap bisa mengekspor atau menghapus datamu, dan membuka Bantuan sekarang."
- [Catat persetujuan wali] [Ekspor data] [Hapus data].
- Whether the data must then be deleted (PP 17/2025 Pasal 9(4)–(5)) is pending legal review.

### Production, while 16–17 is disabled

S27 "Belum tersedia untuk kelompok usiamu": "Sela belum tersedia untuk usia 16–17 tahun di sini. Kalau kamu butuh bantuan sekarang, layanan di bawah bisa kamu hubungi." It shows the verified teen services, or the static help lead and the verified general services when none is verified. Nothing is stored.

## S27 System screens

All of these use the normal shell, with help and exit, and an h1.

| Screen | When | ID | EN | Actions |
|---|---|---|---|---|
| Not found | Unknown route | Halaman ini tidak ditemukan. | This page wasn't found. | [Ke beranda] or [Ke pengenalan] |
| Session ended (account) | `401 not_signed_in` | Sesimu sudah berakhir. Masuk lagi untuk melanjutkan. Tulisan yang belum terkirim masih ada di tab ini. | Your session has ended. Sign in again to continue. Anything you hadn't sent is still in this tab. | [Masuk] |
| Session ended (guest) | `401` with no account | Data tamu tidak bisa dibuka lagi dari browser ini. Data itu akan dihapus otomatis sesuai jadwal. | Guest data can't be opened from this browser any more. It will be deleted automatically on schedule. | [Mulai lagi] [Bantuan sekarang] |
| Storage unavailable | `503 store_unavailable` | Penyimpanan sedang tidak tersedia. Coba lagi sebentar lagi. Bantuan sekarang tetap bisa dibuka. | Storage is unavailable right now. Try again shortly. Help now still works. | [Coba lagi] |
| Busy | `503 server_busy` | Server sedang sibuk. Coba lagi sebentar lagi. | The server is busy. Try again shortly. | [Coba lagi] |
| Under 15 | Age band `under_15` | Sela belum bisa dipakai di usiamu. Kalau kamu butuh bantuan sekarang, layanan di bawah bisa kamu hubungi. | Sela can't be used at your age yet. If you need help now, you can contact the services below. | Help list: the verified teen entries; when none is verified, the static S21 lead ("Kalau kamu dalam bahaya sekarang, hubungi layanan darurat setempat atau orang terdekat yang bisa menemanimu.") plus the verified general entries. Nothing is stored |
| 15 tahun | "15 tahun" chosen in S02 (D-030 block, every environment) | Sela belum bisa dipakai di usia 15 tahun. Kalau kamu butuh bantuan sekarang, layanan di bawah bisa kamu hubungi. | Sela can't be used at age 15 yet. If you need help now, you can contact the services below. | The same help list and fallback. Nothing is stored |
| Age band not enabled | `age_band_not_enabled` | Sela belum tersedia untuk kelompok usia ini. | Sela isn't available for this age group yet. | The same help list and fallback |
| Policy updated | `actionsRequired: accept_policy:*` | Kebijakan privasi diperbarui. Yang berubah: … | The privacy policy was updated. What changed: … | [Setuju] [Baca lengkap] [Ekspor dan hapus data]. Never blocks privacy actions |
| Data deleted | After `DELETE /api/v3/me` | Datamu sudah dihapus dari Sela. Dalam 7 hari tidak ada cadangan yang bisa membukanya, dan dalam 30 hari semua cadangan ikut terhapus. | Your data has been deleted from Sela. Within 7 days no backup can open it, and within 30 days it's gone from every backup. | [Ke pengenalan] |
| Demo reset | Demo environment, data reset on schedule | Demo sudah diatur ulang. Semua data demo sintetis. | The demo was reset. All demo data is synthetic. | [Mulai] |

- **Keyboard:** focus on `<main>`, then the h1, then the actions.
- **Layout:** one column at every width, within `--measure`.

## S28 Policy pages

**Routes** `/policies` (index), `/policies/:purpose` (one policy; `?version=` for an earlier version) · **Access** public · **Title** "Kebijakan – Sela" / "Policies – Sela"; one policy: "{judul kebijakan} – Sela", for example "Ketentuan & Privasi – Sela"

- **Purpose:** let anyone read each policy in plain language before and after agreeing to it, see which version applies, and see what changed (UU PDP Pasal 21 information; D-048 for the AI providers; [CONTENT_STYLE.md §3.2](CONTENT_STYLE.md#32-reading-level-guidance): a plain summary first).
- **Primary action:** none; these are pages to read. When reached from a consent card or onboarding, a back link returns there ("← Kembali ke pilihanmu"), with the choice untouched.
- **Secondary actions:** "Lihat versi sebelumnya" (earlier versions); "Yang berubah" (what changed); the language switch; "Cetak" (the browser's print, with a print stylesheet); for signed-in owners, "Kelola persetujuan" (S22 `#persetujuan`).
- **Content and data:**
  - **Index:** an h1 "Kebijakan" and one row per policy, each with its title, one line on what it covers, the current version and its date: "Ketentuan & Privasi", "Penyimpanan mode tamu", "Informasi untuk orang tua atau wali", and, once active, one page per AI provider and purpose ("Balasan AI · Meta, di Modal (Amerika Serikat)", "Pemeriksa · Claude dari Anthropic (Amerika Serikat)", "Model cadangan · DeepSeek (Tiongkok)"), memory and sharing. Reserved purposes are not listed until they are active.
  - **One policy:**
    1. the h1 (the policy title);
    2. the version line: "Versi 2026-10-26.1 · berlaku sejak 26 Okt 2026" / "Version 2026-10-26.1 · in force since 26 Oct 2026";
    3. for signed-in owners, the person's own state: "Kamu menyetujui versi ini pada 26 Okt 2026." or "Kamu belum menyetujui versi terbaru." with [Lihat dan setujui] (the policy-updated screen of S27). For a provider policy: "Diizinkan sejak …" or "Belum diizinkan";
    4. **"Singkatnya"** / "In short": three to five plain sentences, first on the page and never contradicting the full text. For an AI provider page it names the provider, the place, the purpose, what is sent and the provider's retention (or "belum diverifikasi");
    5. the full text, in sections with `h2` headings and an in-page list of links to them;
    6. "Yang berubah dari versi sebelumnya" / "What changed since the previous version": a short list, when there is a previous version;
    7. "Versi sebelumnya": earlier versions with their dates, each a link to `?version=`.
  - Texts come from `GET /api/v3/policies` in both languages; a part only in one language carries `lang`.
- **States:**
  - loading: the h1 and "Memuat kebijakan…" after 400ms;
  - error: "Belum bisa memuat kebijakan ini. [Coba lagi]"; when a version was loaded before in this tab, it stays on screen with "Ditampilkan dari saat terakhir dimuat, 20.14";
  - an earlier version: a notice above the h1, "Ini versi lama (2026-09-01.2), tidak berlaku lagi. [Lihat versi yang berlaku]";
  - unknown purpose or version: S27 Not found;
  - empty: not applicable (the index always lists the required policies).
- **Recovery:** reading never changes a consent; the back link returns to the exact onboarding step or consent card the person came from, with everything they had chosen.
- **Keyboard and focus order:** shell; back link (when present); h1; the version line and own state (the state's button); "Singkatnya" (not focusable); the in-page section links; the sections' links in order; "Yang berubah"; the earlier versions. Each in-page link moves focus to its section `h2`.
- **Layout:**
  - 320px: one column within `--measure`, 18px reading text; the in-page links as a wrapped list after "Singkatnya".
  - Wide: within `--measure`; the in-page links as a sticky side list from 75em (sticky inside the content, never fixed to the viewport).
  - Print: black on white, the version line on every page, no navigation.

| Element | ID | EN |
|---|---|---|
| Index h1 | Kebijakan | Policies |
| Summary heading | Singkatnya | In short |
| Version line | Versi 2026-10-26.1 · berlaku sejak 26 Okt 2026 | Version 2026-10-26.1 · in force since 26 Oct 2026 |
| Own state | Kamu menyetujui versi ini pada 26 Okt 2026. | You agreed to this version on 26 Oct 2026. |
| Old version | Ini versi lama, tidak berlaku lagi. Lihat versi yang berlaku. | This is an old version and no longer applies. See the version in force. |
| What changed | Yang berubah dari versi sebelumnya | What changed since the previous version |

- **API:** `GET /api/v3/policies` (v3 spec; current versions, ID and EN); `GET /api/v3/me/consents` (v3 spec) for the person's own state; earlier versions and their change lists: `GET /api/v3/policies/{purpose}/versions` and `…/versions/{version}` (planned).
