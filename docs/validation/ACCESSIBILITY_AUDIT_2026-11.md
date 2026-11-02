# Accessibility audit, November 2026 (beyond axe)

Date: 2026-11-02. Base commit `198b134` (operator console overhaul, D-079) plus the uncommitted changes listed in §4. Criteria: [ACCESSIBILITY_CRITERIA.md](../design/ACCESSIBILITY_CRITERIA.md) (WCAG 2.2 AA). Roadmap items: S-192 (manual accessibility review), S-195 (browser review), S-196.

**Status: automated and scripted browser audit done locally; findings fixed where marked. No screen-reader user test happened.** Nothing in this document is a claim that a person using NVDA, JAWS, VoiceOver or TalkBack has used Sela. §6 lists what still needs a human with a real screen reader.

## 1. Scope and method

**Surfaces.**

- User app (`web/src/v3`): public dashboard, Bantuan sekarang, Bacaan & latihan kecil, Tentang Sela, Kebijakan, Masuk, Tampilan & aksesibilitas; after a synthetic guest onboarding: Balasan AI consent, Beranda, Ruang cerita (`/talk`), Catatanku, Tulis catatan, Cek perasaan, Dukungan, Privasi & data, Akun.
- Operator console (`web/src/operator`): sign-in, Beranda, Jaringan profesional, Moderasi, Konten, **Tinjauan bacaan** (`ReadingPanel.tsx`, with the list and with an item open), Sistem. Synthetic `owner` operator only.

**Environment.** `sela serve` built from this tree, `SELA_ENV=development`, port 8094, generated KEK, random `SELA_ADMIN_MFA_KEY`, throwaway `postgres:16-alpine` on port 55442, LLM disabled, web built to a scratch dist. Playwright 1.64.1 with Chromium (headless shell), `locale: "id-ID"`, axe-core 4.10.3. All data synthetic. No reading item was approved.

**Script.** [`e2e/a11y-audit.e2e.mjs`](../../e2e/a11y-audit.e2e.mjs) (new). Final run: **356 of 356 checks pass** (after the fixes in §4). For every page it checks:

| Area | What the script does | Criteria |
| --- | --- | --- |
| Landmarks and headings | exactly one `main`, exactly one visible `h1`, no skipped heading level, every `nav` labelled when there is more than one, `html[lang]` set, non-empty `document.title` | 1.3.1, 2.4.2, 2.4.6, 3.1.1 |
| axe, light and dark | WCAG 2.0/2.1/2.2 A+AA tags; serious/critical findings fail; run with `colorScheme: light` and `dark` separately (contrast in both themes) | 1.4.3, 1.4.11, 4.1.2 and the axe set |
| Keyboard | up to 30 Tab stops (60 on the open reading preview): every stop must show an outline or box-shadow, and its centre must not be covered by another element (sticky header) | 2.1.1, 2.4.3, 2.4.7, 2.4.11 |
| Reflow | 320 × 720 viewport: no horizontal page scroll | 1.4.10 |
| Text resize | root font size 200 % at a 640 px layout (same CSS pixels as 200 % zoom at 1280 px): no horizontal scroll, no text box clipped by `overflow: hidden` | 1.4.4, 1.4.12 |
| Reduced motion | `reducedMotion: reduce`: no element with a running animation or a non-zero transition | 2.3.3 (AAA, product criterion), 2.2.2 |
| Route focus | client-side navigation by keyboard: focus lands on `main` (user app) or the page `h1` (operator), never on the old link or `body` | 2.4.3 |
| Skip link | first Tab stop on the public dashboard is "Lewati ke konten" and moves focus to `main` | 2.4.1 |
| Dialog | Menu: focus moves into the dialog, Escape closes it and returns focus to the trigger | 2.1.2, 2.4.3 |
| Forms | sign-in with synthetic wrong credentials: fields get `aria-invalid` and an `aria-describedby` that resolves to the error text; Tulis catatan: the unavailable save button is focusable with `aria-disabled` and a reason | 3.3.1, 3.3.2, 4.1.2 |
| Tinjauan bacaan | opening an item by keyboard moves focus to the item heading; automatic checks are a named region; the manual checklist is a named group of three checkboxes; the override reason is a labelled, described textarea; approve stays unavailable before the checklist | 1.3.1, 2.4.3, 3.3.2, 4.1.2 |

Component tests (Vitest + Testing Library, jsdom) pin the fixes (§4). Screenshots of every page in light, dark and 320 px land in `e2e/results/a11y-audit/` (git-ignored).

**Code review by hand** of `ReadingPanel.tsx`, the operator panels' live regions, the v3 shell route-focus effect, `Reading.tsx` and the note editor, against ACCESSIBILITY_CRITERIA §3 (notably 4.1.2 "unavailable ones use `aria-disabled` plus a reason in `aria-describedby`" and 4.1.3 "regions exist, empty, from the first paint").

## 2. What passed

Every page above passed the landmark/heading, axe (light and dark), keyboard focus visibility, focus-not-obscured, 320 px reflow, 200 % text and reduced-motion probes. Route changes move focus correctly in both apps; the skip link works; the Menu dialog contains and returns focus; the sign-in error path associates errors with fields. The shell's three announcement regions exist at load (already pinned by existing tests).

## 3. Findings

Severity: **High** blocks a task for some users; **Medium** makes a task hard or error-prone; **Low** is friction or a best-practice gap; **Info** is a note.

| # | Surface | Finding | Criterion | Severity | State |
| --- | --- | --- | --- | --- | --- |
| A-01 | Tinjauan bacaan | Opening an item did not move focus. The preview renders after the list of 38 row buttons, so a keyboard or screen-reader user had to pass every remaining row to reach the text they opened. | 2.4.3 | Medium | **Fixed**: focus moves to the item heading (`h3`, `tabIndex=-1`) |
| A-02 | Tinjauan bacaan | "Akhiri publikasi" replaced itself with the confirmation, so focus fell to `body`; "Batal" and a successful revoke dropped focus too. | 2.4.3 | Medium | **Fixed**: focus moves to the reason field, back to the trigger on Batal, and to the item heading after the revoke |
| A-03 | Tinjauan bacaan | Approve errors appeared only at the top of the page, far from the form, with no link to the field; the busy state disabled the submit button, which also dropped focus. | 3.3.1, 1.3.1, 2.4.3 | Medium | **Fixed**: refusal shown in a `role="alert"` inside the form, focused; the named field gets `aria-invalid` and `aria-describedby`; focus returns to the item heading after success |
| A-04 | Tinjauan bacaan, Tulis catatan | Unavailable submit buttons used `disabled`: not focusable, no reason. The criteria require `aria-disabled` plus a reason. | 4.1.2 (criteria wording) | Medium | **Fixed for these two forms**: `aria-disabled` + visible "Masih perlu: …" / "Tulis catatanmu dulu…" via `aria-describedby`; handlers guard the submit |
| A-05 | Both apps | The same `disabled`-without-reason pattern remains on 87 `disabled={…}` uses in `web/src/v3` and 29 in `web/src/operator` (some are inputs, not buttons) (e.g. Continuity, Settings, Talk, Onboarding, NetworkPanel). | 4.1.2 (criteria wording) | Medium | **Open**: needs a sweep with the UX owner; some controls (true no-ops while busy) may keep `disabled` |
| A-06 | Tinjauan bacaan | Success message used a conditionally inserted `role="status"`; insertion with text is not reliably announced. | 4.1.3 | Low | **Fixed**: status and alert regions stay mounted and empty until used |
| A-07 | Operator panels | `NetworkPanel`, `PartnerRequestsPanel` and `OperatorApp` toggle `hidden` on their `role="status"` notices; a hidden region is outside the accessibility tree, so some screen readers may skip the first message. | 4.1.3 | Low | **Open**: verify with a screen reader (§6); switch to an always-present region if it is missed |
| A-08 | Tinjauan bacaan; public reading detail | Source links open a new tab without saying so (the official-sources list in `Reading.tsx` already did). | 3.2.5 (AAA, advisory G201) | Low | **Fixed**: visually hidden "(membuka tab baru)" / "(membuka situs sumber di tab baru)" |
| A-09 | Operator date input | Chromium focuses the date field's day/month/year segments and calendar button inside the control; the host lost `:focus-visible`, so the focus ring vanished on some stops. | 2.4.7 | Low | **Fixed**: ring on `input[type=date]:focus, :focus-within` |
| A-10 | Masuk (user) | An empty submit relies on native `required` validation; the browser's bubble is in the browser's language, not the page's (Indonesian). The server error path is correct. | 3.3.1, 3.1.2 | Low | **Open** (recommendation: same in-page error as the credential error) |
| A-11 | Public reading detail | Attribution text carries licence URLs that a screen reader reads character by character; English source titles on an Indonesian page carry no `lang="en"`. | 2.4.4 (context), 3.1.2 | Low | **Open**: the CC BY-NC-SA licence requires the URI; options are a linked "licence" label plus the URI, and a `lang` field on sources |
| A-12 | Tinjauan bacaan | The "ID"/"EN" chips are `aria-hidden`; the paragraphs carry `lang`, so a screen reader switches voice but does not say which language is which. | 3.1.2 | Info | **Open**: confirm with a screen reader whether a spoken label is needed |
| A-13 | Operator console | The page `h1` takes focus on first load (by design in `Page`), so the skip link is not the first Tab stop on a fresh load, unlike the user app. Not a failure (focus is already past the navigation). | 2.4.1 | Info | Open (consistency choice for the UX owner) |
| A-14 | `e2e/operator.e2e.mjs` | The existing operator script still waits for the old "Operator console" heading removed in the D-079 overhaul, so it stops after sign-in. Unrelated to accessibility; its reading assertions were updated for the new checklist and `aria-disabled`. | — | Info | Open (script owner) |

No High findings were found by these methods. That is a statement about what the probes and the review covered, not about what a screen-reader user would experience.

## 4. Fixes and the tests that pin them

| Fix | Files | Test |
| --- | --- | --- |
| A-01, A-02, A-03, A-04, A-06, A-08 (operator) | `web/src/operator/ReadingPanel.tsx`, `reading.css`, `operator.css`, `api.ts` (`AdminError.field`) | `web/src/test/operatorReading.test.tsx`: focus to opened item; revoke step focus in/out; live regions mounted before any message; in-form refusal focused with `aria-invalid` and description; approve button focusable with `aria-disabled` and a "Still needed" description, sends nothing; source link name says "opens in a new tab" |
| A-04 (user) | `web/src/v3/screens/Notes.tsx` | `web/src/test/v3.test.tsx` "keeps the unavailable save button focusable and says why" |
| A-08 (user) | `web/src/v3/screens/Reading.tsx` | `web/src/test/reading.test.tsx` (source link name and `target`) |
| A-09 | `web/src/operator/operator.css` | browser probe in `e2e/a11y-audit.e2e.mjs` (Tab stops on the open reading preview) |

The reading approval accessibility gate (task 2) adds its own UI, documented in [API_SPEC_ADMIN.md §2.1](../api/API_SPEC_ADMIN.md#21-reading-accessibility-gate-sela-reading-a11y-v1): automatic checks as a named region with a text status per check (never colour alone), the manual checklist as a `fieldset`/`legend` of real checkboxes, and the override reason as a labelled textarea with its hint and character count in `aria-describedby`. Those are pinned by the same test file ("accessibility gate" block).

## 5. Limits of this audit

- Chromium only, desktop and emulated widths; no Firefox, Safari, iOS or Android run.
- 200 % text was emulated with the root font size, not browser page zoom or OS text scaling.
- Forced colours / Windows High Contrast and the app's own high-contrast preference were not probed.
- The keyboard probe follows Tab order and checks visibility; it does not judge whether the order is meaningful. The order was read by hand only on the pages listed.
- The talk room was audited empty (LLM disabled): reply announcements, the crisis panel and the reflection card were not exercised in the browser.
- Contrast is axe's computation on rendered pages; images and gradients axe cannot resolve are not covered.

## 6. Still needs a human with a real screen reader

Following ACCESSIBILITY_CRITERIA §5.1 and §7.1 (NVDA + Firefox, JAWS or NVDA + Chrome, VoiceOver + Safari iOS, TalkBack + Chrome Android; Android first):

1. Every ST row of §5.1 in the talk room with a real or fake model: reply announcement through `#sr-reply`, "Sela sedang menulis…", the crisis line through `#sr-alert`, reflection card arrival.
2. Tinjauan bacaan end to end: the opened-item heading is announced; the automatic-check list reads check name then status; the checklist group name is read with each checkbox; the override hint and character count are read; an in-form refusal (A-03) is announced once, not twice; the "Masih perlu" description is read on the unavailable approve button.
3. A-07: whether the first notice in Jaringan profesional, Permintaan bantuan and Beranda is announced when its region is un-hidden.
4. A-12: whether bilingual paragraphs are distinguishable by voice alone.
5. Heading and landmark lists per screen (criteria 1.3.1 manual part) and the reading order of the dashboard and the operator Beranda.
6. Voice control (2.5.3): saying the visible labels of the approve and revoke buttons, "Butuh bantuan sekarang" and "Keluar cepat".
7. Owner testing with readers aged 50+ and teens for plain language (CONTENT_STYLE 3.2); the automatic readability checks are not a substitute.

## 7. Reproduce

```sh
# throwaway database and server (never the compose stack on :8080)
docker run -d --rm --name sela-a11y-pg -e POSTGRES_PASSWORD=pw -e POSTGRES_USER=sela \
  -e POSTGRES_DB=sela_e2e -p 127.0.0.1:55442:5432 postgres:16-alpine
(cd web && npx vite build --outDir /tmp/sela-dist)
go build -o /tmp/sela ./cmd/sela
export SELA_ENV=development SELA_ADDR=127.0.0.1:8094 SELA_WEB_DIR=/tmp/sela-dist \
  SELA_DATABASE_URL='postgres://sela:pw@127.0.0.1:55442/sela_e2e?sslmode=disable' \
  SELA_ADMIN_MFA_KEY="$(openssl rand -base64 32)"   # plus SELA_DATA_KEK/_ID from `sela keys generate-kek`
/tmp/sela migrate up
printf '%s\n' "$OPERATOR_PASSWORD" | /tmp/sela admin create --username a11y.audit --role owner
/tmp/sela serve &
cd e2e && E2E_BASE_URL=http://127.0.0.1:8094 OPERATOR_USERNAME=a11y.audit \
  OPERATOR_PASSWORD=… OPERATOR_TOTP_SECRET=… node a11y-audit.e2e.mjs
```
