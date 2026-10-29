# Sela v3 owner acceptance guide

Updated: 2026-10-28. Branch under review: `feat/v3-experience`. This guide is
for the owner's manual acceptance of the v3 frontend. It is **not** evidence
that the product is pilot-ready. Use synthetic accounts, notes and messages
only. Do not enter real health information or another person's real data.

## 1. Before you start

### Environment

1. Build and start the local delivery image:
   `bash scripts/run-local-v3.sh`
2. Open `http://localhost:8080`. Use a private browser profile for each run.
3. The Demo banner is visible because this is the development environment; all
   content is synthetic. The `/demo` path is still the separate v2 synthetic
   demonstration and is not part of this acceptance.
4. Open the browser console and network panel. A single `401 /api/v3/me` on a
   fresh public visit is expected (no session yet); there must be no
   uncaught exception.

### Devices and widths

Check each primary journey on:

- 320 × 740 CSS px (small phone, 100% and 150% text size);
- 390 × 844 (common phone);
- 768 × 1024 (tablet);
- 1365 × 900 (desktop);
- browser zoom 200% and 400% at 1280 × 1024;
- the four display schemes: light, dark, contrast-light, contrast-dark.

Reduce browser motion to "reduce" once and confirm nothing depends on an
animation. Turn on a screen reader for at least the Introduction, the age step,
the room composer, the privacy page and a dialog; record what was announced.

### Synthetic test data

Use `pelangi.senja` as a username only in a disposable local database, with a
long synthetic password. Never reuse a real password. For the guest pass, start
a guest and delete it at the end. Useful synthetic messages:

- `Aku merasa lelah akhir-akhir ini, tetapi aku ingin mencoba menulisnya.`
- `Aku sudah tidur kurang dari lima jam selama beberapa hari.`
- `Aku ingin menyiapkan pertanyaan untuk pertemuan besok.`

## 2. Acceptance checklist by screen group

Mark each as Pass / Fail / Blocked. "Blocked" must name the missing API; do not
mark a blocked screen as passing.

### S01 Introduction and About

- [ ] `/` shows "Ruang sejenak untuk didengar.", the "Mulai" action, the four
      "what you need to know" points, and the safety sentence.
- [ ] The language buttons change the whole page at once and the status is
      announced; each button label stays in its own language.
- [ ] Text size 100/115/130/150 changes immediately and survives a reload.
- [ ] "Butuh bantuan sekarang" and "Keluar cepat" are present, in that order.
- [ ] `/about` names Meta/Modal, Claude/Anthropic and the DeepSeek fallback, and
      says that not every provider is live.
- [ ] No invented testimonial, partnership or professional claim.

### S02 Onboarding

- [ ] `/start/age` asks for a band, not a birth date. Under 15 and "15 tahun"
      reach a blocked screen with verified help and create nothing.
- [ ] 18+ guest: `/start/storage` requires a storage mode, a conversation
      default and both confirmations; the guest is created only after the
      primary action. The network panel shows `POST /auth/guest` once.
- [ ] 18+ account: `Lanjut membuat akun` opens `/start/account`; creating an
      account shows ten one-time codes; the continue button stays unavailable
      until "Aku sudah menyimpan kode ini" is checked.
- [ ] 16–17: the confidentiality step appears; only the guest option is offered;
      the guardian step appears after guest creation. Agreeing records consent;
      declining deletes the new guest and shows the teen help path.
- [ ] `/start/ai` shows a separate Meta card and Claude card, and the DeepSeek
      card only for 18+. The primary action is unavailable until each applicable
      card has an answer. "Tidak sekarang" records a choice without blocking the
      room.
- [ ] Back works on every step; nothing is stored before the storage step.
- [ ] API limits: a missing `termsPolicyVersion` shows the policy error, not a
      generic crash; a full guest pool shows `server_busy` copy and an account
      alternative.

**API:** `GET /auth/options`, `GET /policies`, `POST /auth/guest`,
`POST /auth/register`, `PUT /me/consents/{purpose}`, `PUT /me/guardian-consent`,
`POST /me/recovery-codes/acknowledge`, `DELETE /me`.

### S03 Sign in

- [ ] `/signin` has a visible username label, password label, "Tampilkan",
      remember-device choice, and both recovery/get-started links.
- [ ] A wrong password shows the uniform form-level message, keeps both values,
      marks both fields invalid and moves focus to the username.
- [ ] Rate limiting shows a plain retry message; no CAPTCHA or character puzzle.
- [ ] Success goes to `/home`, or to a same-origin `next` path only.

**API:** `POST /auth/login`.

### S04 Create account and codes

- [ ] Username rules are visible before typing; password rules are visible and
      a "n of 15" progress line updates on blur.
- [ ] A taken username shows the API message and a suggestion.
- [ ] Codes are shown once, in a monospace list, with copy, download and print.
- [ ] Leaving the codes step without confirming is recoverable from Account.
- [ ] Account creation never asks for email, phone or real name.

**API:** `POST /auth/register`, `POST /me/recovery-codes/acknowledge`.

### S05 Recovery

- [ ] `/recover` asks for username, one-time code and a new password, with the
      tolerant-code hint.
- [ ] A wrong code shows one uniform message, keeps the values and does not say
      which part failed.
- [ ] Success reports the remaining code count; three or fewer offers new codes.

**API:** `POST /auth/recovery`.

### S06 Guest upgrade

- [ ] `/settings/account/upgrade` explains that existing guest data stays and
      that the account enables sign-out and other devices.
- [ ] The same username/password rules apply; the resulting account keeps the
      same data and shows recovery codes once.
- [ ] A lost response tells the person to sign in with the new credentials.

**API:** `POST /auth/guest/upgrade`, recovery-code acknowledgement.

### S07 Home

- [ ] `/home` has one h1 "Beranda", a greeting, and "Mulai bercerita" as the
      clear primary action.
- [ ] "Tulis catatan" and "Cek perasaan" open their full screens; Home has no
      inline check-in form.
- [ ] "Terbaru di Catatanku" shows at most three saved entries and a "Lihat
      semua" link; an empty account shows the "Belum ada yang tersimpan" state.
- [ ] No day counts, streaks, scores, charts or guilt copy.
- [ ] A guest sees the retention line and "Simpan ke akun".
- [ ] `/talk?record=…` from a recent saved conversation opens the room.
- [ ] If the records or conversations request fails, each card fails on its own
      with "Coba lagi"; the primary actions still work.

**API:** `GET /me`, `GET /records`, `GET /conversations`.

### S08 Conversation list

- [ ] `/talk` lists saved conversations newest first with date, title (unless
      previews are hidden) and message count.
- [ ] A live unsaved conversation is shown at the top as "tidak disimpan".
- [ ] Empty, loading and error states match the design; "Mulai bercerita" keeps
      working during an error.
- [ ] No "unfinished homework" style status is shown.

**API:** `GET /records`, `GET /conversations`.

### S09 Conversation room

- [ ] `/talk/new` requires a save/don't-save choice if there is no default; the
      group error appears in the legend and focus moves to the first option.
- [ ] Sending a message keeps the person's words, shows "Sela sedang menulis…",
      and never displays unverified AI text before release.
- [ ] A released reply names its generator and checker from stored provenance.
      "Tentang balasan ini" shows provider, model, check and time.
- [ ] With AI consent declined, the message is kept and the room shows "Balasan
      AI tidak diizinkan" with room to write; it does not invent a reply.
- [ ] Stop (while pending) retains the words, reports the stop, and returns focus
      to the text area.
- [ ] "Akhiri percakapan" ends in one action; saved and unsaved closing panels
      say what happened. A saved conversation remains in Catatanku.
- [ ] Changing storage from saved to not saved asks before deleting the copy;
      changing back saves the conversation.
- [ ] A crisis match keeps the message, shows the static verified support panel,
      pauses replies, announces help once, and offers resume/end. Closing the
      panel leaves the pause line. Reopening a saved paused conversation shows
      the pause line.
- [ ] "Ke balasan terakhir" moves focus to the last Sela article; "Butuh bantuan
      sekarang" and "Keluar cepat" remain available.
- [ ] Quick exit (button or Esc twice) leaves the app; Esc twice does not fire
      after closing a dialog or while a dialog is open.
- [ ] **Known gaps:** correction messages, reflection cards, memory proposals,
      hold-timeout warning and derived-item deletion are not implemented.

**API:** `/conversations*`, `/records` for saved snapshots.

### S10 Note and reflection

- [ ] `/notes/write` focuses the note field; title is optional; prompt chips
      insert editable text; Save is unavailable until the storage choice is made.
- [ ] A saved note opens at `/notes/:id`; editing creates a version and keeps the
      original in "Lihat versi sebelumnya".
- [ ] Delete uses an inline confirm; after deletion focus goes to the timeline.
- [ ] A crisis match saves the note and shows the static support panel.
- [ ] Leaving with unsaved text triggers the browser's leave warning; Quick exit
      deliberately skips it.
- [ ] "Minta Sela menangkap…" is **not** offered as working; reflection is
      blocked by the missing route.

**API:** `/records`, `/records/{id}/versions`, `DELETE /records/{id}`.

### S11 Check-in

- [ ] `/notes/check-in` offers six feelings including "Campur aduk / tidak
      yakin", optional energy, ten optional tags, and a 500-character note.
- [ ] Saving without a feeling shows the group error in the legend.
- [ ] Saving requires the storage choice; success opens the detail.
- [ ] A second check-in on the same local day shows the "already checked in
      today" state instead of creating a duplicate.
- [ ] The detail shows feeling, energy, tags, note, provenance and date, and can
      delete with an inline confirm.

**API:** `POST /records`, `GET/DELETE /records/{id}`.

### S12 Timeline and patterns

- [ ] `/notes` groups saved entries by day, newest first; type, time and preview
      are visible; previews disappear with "Sembunyikan pratinjau isi".
- [ ] Filter chips persist in the URL (`?types=`); clearing filters restores all.
- [ ] A conversation row opens the saved room; a note/check-in row opens detail.
- [ ] Empty and filtered-empty states are distinct.
- [ ] `/notes/patterns` asks for opt-in before showing anything, then shows only
      simple counts for 14/30 days in a real table, with no score or trend.
- [ ] **Known gap:** summary and share events are absent because their APIs are
      absent.

**API:** `GET /records`.

### S13 What Sela remembers

- [ ] `/notes/memory` states that memory is not available and shows no items,
      proposals or hidden profile.
- [ ] The page explains the missing `personal_memory` consent/memory API and
      links to Privacy & data.

**API:** none (blocked).

### S14 Support hub

- [ ] `/support` offers summaries, reading, services and preparation/follow-up.
- [ ] The services link opens `/help`; reading and summary cards state their
      blockers in plain language and do not show sample data.
- [ ] The fixed line says seeking professional help is optional.

**API:** static; `/help` bundle.

### S15 Summary for a consultation

- [ ] `/support/summaries/new` (and any summary route) is an honest unavailable
      state: no sample summary, no fake approval, no share link.
- [ ] It links back to notes and help.
- [ ] The blocker names `/consultation-summaries`.

**API:** blocked.

### S16 Preparation

- [ ] `/support/summaries/:id/prepare` is an honest unavailable state; it does
      not claim a checklist, booking, partnership or verified professional.

**API:** blocked.

### S17 Sharing

- [ ] `/support/summaries/:id/share` is an honest unavailable state; no token,
      access code or recipient projection is invented.

**API:** blocked.

### S18 Share status, audit and revoke

- [ ] `/support/shares/:id` is an honest unavailable state; no audit event or
      revoke result is fabricated.

**API:** blocked.

### S19 After the consultation

- [ ] `/support/summaries/:id/after` is an honest unavailable state; ordinary
      notes remain available.
- [ ] No professional note is implied.

**API:** blocked.

### S20 Reading and small exercises

- [ ] `/reading` says no reading is published yet because every item awaits
      human editorial review.
- [ ] No unreviewed WHO/CDC/Kemenkes text or exercise is displayed as ready.
- [ ] Help and notes remain available.

**API/editorial:** blocked (content review).

### S21 Immediate help

- [ ] `/help` renders from the bundle without a network call; the urgent
      services come first and the full list is under `#layanan`.
- [ ] Every phone link dials the displayed main number, not the extension
      digits; WhatsApp and website links open as labelled.
- [ ] Source, verification date and re-check date are visible; stale entries
      show the warning.
- [ ] The page says it is for Indonesia, that Sela cannot contact anyone, and
      that language does not decide the country.
- [ ] Filters for phone/WhatsApp/website work offline.
- [ ] Teen/unknown age shows the general verified list and says teen-specific
      entries appear only when verified.

**API:** bundled directory; no live dependency.

### S22 Privacy and data

- [ ] `/settings/privacy` has in-page links for storage, memory, names, AI,
      export, sessions, audit, consents and deletion.
- [ ] Storage shows the server, encryption, region (or honest "not decided"),
      the new-conversation default and preview hiding.
- [ ] The guest notice shows the automatic deletion date and the shared-device
      warning; "Simpan ke akun" works.
- [ ] Memory and names sections state their blockers without fake controls.
- [ ] Each AI provider has its own card, provider, place, status and
      allow/withdraw action; withdrawing is immediate and does not affect the
      others.
- [ ] Export offers HTML/Markdown/JSON and ID/EN; an account is asked for a
      password only when required; the download works.
- [ ] Sessions: accounts can revoke others and sign out everywhere; guests see a
      read-only session and no sign-out.
- [ ] Audit shows only content-free events; consent history is available.
- [ ] Delete requires the understanding checkbox and password for accounts,
      states the 7/30-day backup window and the non-recallable copies, then
      shows the "data deleted" screen.

**API:** `/me/consents*`, `/me/sessions*`, `/me/audit`, `/me/export`,
`/auth/reauth`, `DELETE /me`, `/auth/logout-all`.

### S23 Display and accessibility

- [ ] Text size 100/115/130/150 applies at once and survives reload; blocked
      storage still applies until the tab closes and says so.
- [ ] Theme and high contrast produce four schemes; the preview updates.
- [ ] Relaxed spacing, reduced motion, show replies at once, Enter-to-send,
      reply announcements, Quick exit shortcut, Quick exit sign-out policy,
      hidden previews, neutral tab titles and language are all present and
      reversible. No voice/TTS control is present (D-060).
- [ ] "Kembalikan ke bawaan" resets everything.
- [ ] The preview shows a user turn, a Sela turn and a provenance badge/status.

**API:** none; local preferences.

### S24 Account

- [ ] Accounts see username, recovery-code count, change password, new codes,
      age band, guardian consent (16–17), sign out and sign out everywhere.
- [ ] Changing age to under 15 shows the block and does not save the choice.
- [ ] Changing the password signs out other sessions and says how many.
- [ ] Recovery codes are shown once; the acknowledgement is confirmed.
- [ ] Guests see the retention dates, "Simpan ke akun" and delete; **no sign-out
      button**, with the explanation.
- [ ] Withdrawing guardian consent sends the person to the guardian gate and
      keeps privacy/help/display reachable.

**API:** `/me`, `/auth/password`, `/auth/recovery-codes`, guardian consent,
`/auth/logout*`, `DELETE /me`.

### S25 Recipient page

- [ ] `/recipient` explains that v3 sharing is not connected and shows no
      invented projection or code step.
- [ ] `/r` remains the v2 synthetic recipient demo with an explicit demo label.

**API:** blocked in v3; v2 demo retained.

### S26 Minors

- [ ] 15 and under 15 are blocked in every environment and create nothing.
- [ ] 16–17 read confidentiality before storage; only guest start is offered;
      guardian consent is recorded after creation and grants no content access.
- [ ] `unknown` gets the confidentiality step without the guardian sentence and
      no guardian step.
- [ ] Production configuration still allows only 18+ until legal review.
- [ ] No hidden monitoring, guardian access, or default sharing.

**API:** age band, guardian consent; legal gate remains.

### S27 System screens

- [ ] Unknown route: not-found h1, Home/Introduction and Help actions.
- [ ] `/start/blocked?reason=under15|age15|band`: correct copy, verified help,
      nothing stored.
- [ ] Session expiry: a banner keeps the current screen; sign-in is available;
      the guest variant explains that the cookie is gone.
- [ ] Guardian gate and policy gate: privacy/display/account/help/reading remain
      reachable; the gate itself gives a way forward.
- [ ] After deletion: the "Datamu sudah dihapus" screen with the 7/30-day facts.
- [ ] Storage unavailable/busy: inline retry and help remain available.

### S28 Policies

- [ ] `/policies` lists every active policy with title and current version.
- [ ] A policy shows the version, the person's consent state if signed in,
      "Singkatnya", the full text in both languages and a link to manage consent.
- [ ] An unknown purpose shows not-found.
- [ ] Earlier versions are explicitly not available yet; no old text is faked.
- [ ] Links from consent cards return to the same choice without changing it.

**API:** `/policies`, `/me/consents`.

## 3. Cross-cutting manual checks

### Keyboard and focus

- [ ] First Tab shows "Lewati ke konten"; in the room a second skip link reaches
      the writing box.
- [ ] Every action works without a pointer; no keyboard trap.
- [ ] Radios use arrows; dialogs close with Esc and return focus to the trigger;
      the menu sheet starts with help and Quick exit.
- [ ] Route changes set a unique title and move focus to the main content.
- [ ] Focus is never fully hidden by the sticky header, bottom bar or banner.

### Screen reader

- [ ] The page name is announced after a route change.
- [ ] Each field announces its label, hint, required state and error.
- [ ] A released reply is announced once, with "Sela:" and the reply text, only
      when the preference is on; the transcript itself does not double-announce.
- [ ] The crisis panel is announced as "Bantuan ditampilkan di bawah pesanmu."
      once; its contents are reachable by heading/Tab.
- [ ] Status and success messages do not steal focus.

### Visual and responsive

- [ ] 320px: no horizontal scroll; buttons and labels wrap; the 2×2 bottom bar
      does not cover the last content when scrolled to the end.
- [ ] 150% text plus 200% browser zoom: still no clipping or overlap.
- [ ] Text spacing override (line height 1.5, paragraph 2em, letter 0.12em,
      word 0.16em) causes no loss.
- [ ] High contrast keeps every control edge visible; no information by colour.
- [ ] Reduced motion removes animation; no content depends on it.

### Privacy

- [ ] `localStorage` contains only `sela.prefs` (display settings) and
      `sela.patterns` (the opt-in pattern flag); no note, message, token,
      password or recovery code.
- [ ] `sessionStorage` contains only `sela.trusted`, `sela.userKind` and
      `sela.crisis` flags; no content.
- [ ] Export contains records and revisions; deletion removes the account,
      stops access and shows the backup window.
- [ ] Console and network logs contain no note text, message text or tokens.

## 4. How to report a result

Create one row per failed check: screen, step, expected, observed, browser and
width, language, scheme, whether a screen reader/zoom was on, and a synthetic
reproduction. Do not paste personal data, recovery codes or tokens. Attach a
screenshot only if it contains synthetic data. A blocked screen is not a failed
screen; name the API or editorial gate.

The implementation evidence for this guide is in
[V3_FRONTEND_VALIDATION.md](docs/V3_FRONTEND_VALIDATION.md). Status words are defined
in [SPEC §G](SPEC.md#g-goal); nothing here is a clinical, live or pilot claim.
