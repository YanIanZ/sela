# Sound design plan and legal sourcing

## What is in the film

The whole score is **synthesised from code** in `scripts/score.py` (NumPy + SciPy): additive sine/saw oscillators, filtered white noise and a generated reverb impulse. There are **no samples, loops, presets, stock libraries or third-party recordings**. No MSI audio was referenced, sampled or imitated note-for-note; only the *production energy* of tech launch films informed the plan (sparse hits, silence before reveals, a rising pulse under the technical sequence).

Ownership: the score is original work created for SELA and can be released under the repository's MIT licence with the rest of `video/`. Re-running `python3 scripts/score.py` regenerates it bit-for-bit (seeded RNG).

Outputs: `public/audio/score-master.wav` (180 s) and `public/audio/score-teaser.wav` (30 s), 48 kHz, 24-bit stereo, two-pass EBU R128 normalised to **−16 LUFS, true peak ≤ −1.5 dBTP** (measured −15.9 LUFS / −1.7 dBTP).

## Palette

| Element | How it is made | Used for |
| --- | --- | --- |
| Pad | 3 detuned band-limited saws per note, 2-pole roll-off, slow LFO | Chapter harmony (A minor → C major arc) |
| Hit | Pitch-swept sine (90→36 Hz) + filtered noise transient + air band | Every kinetic headline |
| Heartbeat | Two decaying 52 Hz thumps | Chapter 01 pulse, 1/4 notes under the MIE graph |
| Tick | Short 1–2.7 kHz sine blip | UI highlight rings, graph state changes |
| Pluck | 6-partial decaying tone | 16th-note pulse under the MIE graph (112 BPM), timeline markers |
| Bell | Inharmonic partials (1, 2, 2.76, 4.07, 5.4) | Correction moment, approval, final signature |
| Whoosh / riser | Time-varying low-pass noise, rising tone | Scene transitions, logo reveal |

## Cue map (driven by `src/timeline.json`)

| Chapter | Harmony | Key moments |
| --- | --- | --- |
| 01 Every story matters | Am(add9) low, dark | Two heartbeat pulses → hit on EVERY STORY / MATTERS → **intentional silence** (~1.4 s) → riser → hit on UNDERSTANDS? |
| 02 Beyond listening | Fmaj7, opening | Soft hit, hit on UNDERSTAND?, ticks on the reply and the review buttons |
| 03 Meaning Integrity Engine | Dm9 → Am → Fmaj7(#11) | Hit ONE WORD, falling whoosh on kadang→selalu, double tick on the flag, hit ONE CORRECTION, riser into the graph, 16th pluck pulse, **bell + hit on the correction**, descending ticks as summary/link/request change |
| 04 Context over time | Cmaj9, calm | Plucks E4/G4/B4 on the three dated records, soft hit on NO SCORES |
| 05 Personal action bridge | G6 | Hits, tick on save, single pluck on OPTIONAL |
| 06 Care passport | Fmaj7 → C/E | Three rising hits WORDS/CHOICE/APPROVAL, ticks on each source pick, bell on approval |
| 07 Support, on your terms | Am7 → Fmaj7 | Ticks on send/propose, bell on confirm, soft low thud on cancel |
| 08 Final reveal | C major (add9) | Riser → sub + bell on the sprout mark, plucks on the tagline, two-note "sprout" signature (C5–G5) |

## Mixing guidance for a narrated version

- Duck music ~6 dB under narration; keep hits but pull them ~3 dB.
- Do not add alarm, siren, heartbeat-monitor or "horror" sounds; keep the mental-health context calm.
- If a licensed track replaces the score, keep proof of licence (track, author, licence ID, date) in this file and make sure the licence covers online competition use and editing.

## If you prefer human-made music

Acceptable sources: a composer commissioned with a written work-for-hire or licence; or tracks under CC0 / CC BY (with attribution in the description). Avoid "free" YouTube audio of unclear origin and anything from existing product films.
