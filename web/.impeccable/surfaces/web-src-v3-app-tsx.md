---
version: 1
slug: "web-src-v3-app-tsx"
primary_target: "web/src/v3/App.tsx"
related_targets: ["web/src/v3/shell.tsx","web/src/v3/screens/Home.tsx","web/src/v3/styles.css"]
---

# Sela replacement interface — Sampul Buku

Mode: Operate (public portal is its own Read-mode brief, inheriting this world). Scope: whole web/src/v3 app. Owner request 2026-10-28: rebuild the whole look, more professional and responsive; "editorial hangat" + "tenang & tepercaya"; keep every function and all copy. Supersedes the cobalt Open Fold world. Preserve owner authorization, consent, records, crisis help, Quick exit, ID/EN, display preferences (scheme, contrast, text size, spacing, motion, colour mood) and every supported API; unsupported APIs stay honest blocked states.

## Direction contract

THESIS: Each place in Sela is a book you own: a flat cover-colour band names where you are, and beneath it a warm, ruled reading page where your own words lead. Refuses the category default of white cards, pastel blobs and a single teal accent.

OWN-WORLD: Warm paper ground (#F5F0E6) with a lighter page (#FBF8F2), ink #231C17, hairline #DCD2C2 and a 2px ink rule opening each section. Four cover colours as wayfinding: brick Beranda, forest Cerita, ochre Catatanku, indigo Dukungan. Literata serif for titles and record titles; Hanken Grotesk for UI and body. Flat print surfaces, 4–8px corners, no shadows at rest, spine marks beside places and shelf rows.

STORY: Arrive, see which book you are in, pick talk/note/check-in from the cover, then read your own saved words in ruled rows; Help and Quick exit always in the same top-right place.

FIRST VIEWPORT: Top bar: Sela spine-mark wordmark left, four place tabs with spine colours centre, Help + Quick exit + Menu right; desktop tool strip beneath. Then a full-bleed cover band in the place colour: serif page title at 3.5–4.75rem with greeting, invitation and primary actions on the cover. Below, a 1.4fr/1fr ruled page: continue + recent records left, shelf of other rooms right. Mobile keeps two-row header and four-place bottom bar with spine colours.

FORM: Indonesian paperback cover system (Pustaka Jaya / Balai Pustaka 1970–80s), my top-ranked grounded candidate, shown as IMPECCABLE'S PICK and chosen by the user over the rolled Surat Balasan direction. seed:5abf77ad9f750cfb. Sketch .impeccable/mocks/decision/sampul-buku.html (capture captures/sampul-buku.png). Code-led.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## Constraints carried forward
- No kicker above headings; no scores, streaks or progress.
- Help/danger/success/warning semantics identical across colour moods; text 4.5:1, controls/focus 3:1 in every scheme; high contrast keeps its own values.
- Focused writing/listening rooms keep a protected writing area; 320px and 130–150% text must reflow without horizontal scroll.
- Dev-only dashboard preview stays synthetic, labelled and API-free.
