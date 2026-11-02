# SELA — Meta Build for Your World video storyboard

**Status:** SCRIPT / NOT RECORDED. **Audience:** Indie hackathon judges, product reviewers, accessibility/safety reviewers. **Primary cut target:** 03:00 (editorial duration; verify official portal limit before final edit). **Language:** Indonesian narration with clear English subtitles; prepare fully-English voiceover if needed. **Format:** 16:9 screen capture, legible on mobile, 1080p where possible, no copyrighted background music required. **Data:** 100% synthetic adult personas.

> **The film is a planned demonstration.** Do not present Life Context Timeline, Care Passport preset or Personal Action Bridge as implemented until engineering PRs actually ship and tests pass. The currently implemented MIE correction, summary, share and simulated care request are valid fallbacks, if verified on the exact running build. No fabricated live provider failure, results, partnerships or clinical outcomes.

## 1. Core narrative / angle

**Title:** SELA — Your Story. Your Meaning. Your Next Step.

**Hook:** “AI can listen. But can it remember what you *actually* meant—and stop repeating its own mistake when your story moves into someone else's hands?”

**Unique proof:** A person corrects “I need quiet time **sometimes**” after an AI draft mistakenly states “I avoid **all** friends”; the source-bound MIE graph marks downstream artifacts stale and revokes previously granted access. The user then rebuilds a *user-approved* consultation preparation sheet and—if they choose—hands it to a **fictional** professional workspace.

**Human value:** No need to recite weeks of notes from scratch; the user chooses the parts of their story to carry, can opt to work manually with no AI, and can decline care suggestions altogether.

**Technical value:** Meta-driven, bilingual, source-anchored proposals; Go deterministic Meaning Diff and consent; owner-governed MIE graph with transactional invalidation; versioned encrypted PostgreSQL; safe partner scopes and revocable frozen projection. Where a shot uses a deliberate bad-claim fixture, say so: the goal is deterministic demonstration of handling AI mistakes, *not* to imply Meta genuinely produced that error.

## 2. Primary 03:00 shotlist with exact content

| Timestamp | Shot / screen | User interaction | Narration (draft, Indonesian) | Required onscreen truth |
| --- | --- | --- | --- | --- |
| 00:00–00:15 | Quiet landing / title | Introduce Nadia fictional adult, two short saved-note dates | “Kadang yang paling sulit bukan mencari jawaban, tapi menjelaskan apa yang sebenarnya kita rasakan.” | “Simulasi / Synthetic demo” visible |
| 00:15–00:35 | `/talk` / Talk | Nadia: “Kadang aku butuh waktu sendiri setelah kuliah. Aku tetap suka bertemu teman.” Choose suitable listening mode | “Sela mendengarkan dengan Meta AI. Responsnya bukan diagnosis, dan tidak otomatis menjadi ingatan tentang kita.” | Actual provider provenance, otherwise labelled recorded/test double |
| 00:35–00:57 | Meaning Diff / reflection | Show unapproved proposal anchored to source; separately show a **deliberate bad interpretation fixture** “Nadia selalu menghindari semua teman” | “AI bisa salah menangkap satu kata. ‘Kadang’ berubah jadi ‘selalu’. Itu tidak boleh menjadi fakta tentang seseorang.” | **TEST FIXTURE / Simulated AI error** label, not a forged live output |
| 00:57–01:19 | Review + lineage / impact | Nadia rejects/corrects interpretation; open MIE impact, memory/summary stale | “Di Sela, koreksi bukan cuma edit chat. Meaning Integrity Engine menelusuri sumber dan menghentikan makna yang salah dari menyebar.” | Source version, stale statuses, impacted counts grounded in real API |
| 01:19–01:38 | **Life Context Timeline (proposed)** or fallback `/notes` saved records | Show 3 dated synthetic notes; click source + version and no-AI reading path | “Nadia bisa melihat kembali ceritanya dari waktu ke waktu, dalam kata-katanya sendiri, tanpa skor atau diagnosis.” | If feature not shipped, **use existing notes timeline** and do not call it new product |
| 01:38–01:53 | **Personal Action Bridge (proposed)** or fallback summary preparation questions | Nadia chooses manual step: “Tulis dua pertanyaan yang ingin kubawa saat konsultasi.” | “Kalau ingin melangkah, Nadia sendiri yang memilih satu hal kecil. Tidak ada target pemulihan atau kewajiban.” | Do not show an interactive Action UI unless implemented; fallback uses existing summary `prepared.questions` |
| 01:53–02:17 | **Care Passport preset (proposed)** or fallback `/support/summaries` | Select source quotes, review wording, approve **exact version**, show recipient preview | “Dari catatan yang ia pilih, Nadia menyusun ringkasan persiapan. Yang dibagikan hanya bagian yang ia setujui.” | Personal preparation, **not** medical passport; exact frozen share projection |
| 02:17–02:38 | `/support/plan` + partner operator (synthetic) | Create care request to fictional clinic, partner proposes slot, Nadia confirms (or show owner share→revoke fallback) | “Jika ingin bantuan, Nadia menghubungi layanan simulasi dalam demo. Praktisi nyata tidak dihubungi, dan seluruh kendali tetap di Nadia.” | “Simulated partner / Bukan layanan sungguhan”; no “appointment booked” claim |
| 02:38–02:51 | Revoke and attempted recipient reread | Revoke; request fails after commit; show one MIE lineage statement | “Satu perubahan makna bisa menghentikan akses berikutnya. Cerita yang sudah pernah dibaca tetap tidak bisa ditarik dari salinan penerima.” | Show honest revocation limit and real API denial |
| 02:51–03:00 | Title + source/evaluation one sentence | Display commit and repository; one measured result only if benchmark done | “Sela: your story, your meaning, your next step.” | Exact build SHA, no fabricated 100/100 or clinical outcome |

Timing total: **180 seconds**. Caption phrases must be comfortably legible; avoid narrating faster than a clear understandable pace. If narration cannot fit, keep the shot longer and shorten less important content—don't speed up sensitive privacy notices.

## 3. Narration master script (ID)

“Kadang yang paling sulit bukan mencari jawaban, tapi menjelaskan apa yang sebenarnya kita rasakan.

Ini Nadia, tokoh fiktif yang ingin bercerita setelah hari-hari yang melelahkan. Sela mendengarkan menggunakan model Meta. Ceritanya tetap miliknya; jawaban AI bukan diagnosis atau fakta yang otomatis disimpan.

Bagaimana kalau satu kata berubah makna? Nadia berkata ‘kadang butuh waktu sendiri’, tetapi sebuah contoh interpretasi keliru menyatakan ia selalu menghindari teman. **Contoh kesalahan ini sengaja disimulasikan untuk pengujian.** Nadia menolaknya.

Meaning Integrity Engine menelusuri apa saja yang bergantung pada interpretasi itu. Memori dan ringkasan yang terdampak perlu ditinjau kembali; akses berbagi yang sudah tidak aman dihentikan untuk pembacaan selanjutnya.

Dari catatan yang ia simpan sendiri, Nadia melihat bagaimana ceritanya berubah dari waktu ke waktu. Jika ia mau, Nadia memilih satu langkah kecil: menuliskan pertanyaan yang ingin ia sampaikan.

Ia menyiapkan ringkasan konsultasi dari sumber yang ia pilih, meninjau kutipan aslinya, lalu menyetujui versi final. Untuk demo ini, penerimanya adalah layanan fiktif, bukan rumah sakit atau praktisi sungguhan.

Bila Nadia berubah pikiran, ia dapat menghentikan akses berikutnya. Sela tidak dapat menarik kembali salinan yang sudah pernah dilihat penerima.

Kami membangun Sela karena AI seharusnya membantu orang menjelaskan cerita mereka—bukan mengambil alih maknanya. **Your Story. Your Meaning. Your Next Step.**”

This is a **draft narration**; trim/pause after recording a natural take. Never cut the explicit simulated-error and simulated-partner labels.

## 4. English subtitle / backup voiceover cue sheet

- “Some days, finding the words is harder than finding an answer.”
- “This is Nadia, a fictional adult. She can talk to Sela without turning an AI response into a diagnosis or lasting fact.”
- “A **scripted test fixture** changes ‘sometimes’ into ‘always’. Nadia rejects the mistake.”
- “The Meaning Integrity Engine traces what depended on it, marks related summaries stale, and stops future access to invalid information.”
- “Her own saved stories become an understandable timeline, not a medical risk score.”
- “She chooses a single next step—if she wants one.”
- “She prepares a consultation sheet from exact source quotes, approves the specific version, and chooses whether to share.”
- “In this demo the professional workspace is **fictional**; no real care is booked or delivered.”
- “She can revoke future access. Copies already viewed cannot be erased.”
- “Your story. Your meaning. Your next step.”

Subtitle language and screen locale should be consistent, and screen recording must not show real usernames/emails/phone numbers.

## 5. Camera + editing plan

**Framing:** product-first, no talking-head required. Capture 16:9 desktop; use 125–150% browser zoom if text is hard to read on laptop displays. Crops highlight original vs interpretation without excluding the “Simulated” badge. Use no zoom that hides privacy warnings.

**Motion:** clean cuts, cursor path purposeful, no fake system alert sounds, no flashing or hectic transitions. Preserve narration room tone; include captions with strong contrast. Overlay architecture only for 2–3 seconds if it does not obscure product.

**Audio:** original voiceover or captions-only backup; no scraped copyrighted music. Prefer silence or simple licensed original tone. Avoid dramatic “horror” mental-health framing or imagery.

**Accessibility:** read crucial terms aloud or on screen, not color-only labels. Slow enough to understand a correction and its consequence. Provide downloadable transcript/subtitle file if portal accepts.

**Visual rhythm:** 0:00–0:35 human problem; 0:35–1:19 unique technical “wow” MIE; 1:19–2:17 useful personal outcome; 2:17–2:51 consent-based professional flow; 2:51–3:00 focused ending.

## 6. Demo mode matrix — avoid untruthful shots

| Scene | Code on main as inspected 2026-11-02? | If new features not shipped |
| --- | --- | --- |
| Listening and response mode | YES, subject to deployed version | Run verified local or controlled synthetic environment |
| Source-bound MIE correction/impact | YES, tested locally | Use existing approved synthetic fixtures |
| Full Life Context Timeline | **PROPOSED** | Show existing saved notes and label as current notes timeline |
| Personal Action Bridge | **PROPOSED** | User manually enters questions/goals in approved summary |
| Care Passport specialized UI | **PROPOSED** | Show existing approved consultation summary/export/share |
| Simulated professional request and workspace | YES, locally tested D-076 | Stay in clearly synthetic partner workflow |
| Partner invitations | YES, code D-079, latest CI must be verified | Omit invitation shot, not central to demo |
| Full live Meta ablation benchmark | NOT MEASURED | Show only honest offline results (e.g. current MIE test) with n/limitations |

**Recording decision:** if any of the three proposed P0 features fails acceptance or lacks an end-to-end demo, use current fallback. An honest working story is more ambitious than a mock UI advertised as live.

## 7. The “wow” moment: exact instructions

1. Start with source original visible and immutable.
2. Display deliberately wrong **fixture-labelled** interpretation.
3. Show Nadia's rejection/correction with one click.
4. Reveal source-to-memory-to-summary-to-share-to-care-request dependency count **from server**.
5. Open previously shared snapshot in a separate synthetic recipient tab after correction → denied.
6. Show a new, user-approved summary version that can be shared; old link remains ended.
7. Narrate exactly: “Sela doesn't let a corrected AI guess quietly become the story someone else reads.”

**Never** imply Sela can un-send information recipients already downloaded. This boundary is part of its credibility.

## 8. Deliverable checklist

- [ ] Confirm official duration, aspect, subtitles, attribution and URL access requirements in authenticated submission portal.
- [ ] Capture verified running commit SHA and environment; never assume main == deployed.
- [ ] Synthetic Nadia data loaded only into isolated demo database.
- [ ] Real Meta provenance captured on a permitted, safe route; no raw key in video.
- [ ] Separate *labelled* scripted wrong-interpretation fixture demonstrated.
- [ ] MIE invalidation and denial recorded from backend.
- [ ] UI shots match implemented features; fallback shots used when not shipped.
- [ ] Partner clearly fictional; no booking/clinical efficacy claims.
- [ ] Revocation limitation explained.
- [ ] Captions checked in ID/EN and text size on small screen.
- [ ] Capture avoids personal data, invite tokens, active share links, browser passwords and operator secrets.
- [ ] Benchmark numbers only if actually run, pinned dataset/result.
- [ ] Final artifacts: video, optional SRT, one-page pitch, README link, source/build SHA, evidence index and limitation note.
- [ ] Owner submits and captures submission receipt; PR does not submit anything.

Related: [runbook](META_HACKATHON_2026_RUNBOOK.md), [synthetic Nadia fixture](SYNTHETIC_NADIA_FIXTURE.md), [acceptance](../validation/BWYW_ACCEPTANCE.md).
