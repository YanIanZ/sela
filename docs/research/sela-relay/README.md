# Sela — A little space to feel heard.

Paket refining produk dan riset • 26 Oktober 2026 • Bahasa utama Indonesia, rancangan produk ID/EN.

**Rekomendasi: bangun Sela sebagai pendamping untuk mempersiapkan dan menyelesaikan perjalanan menuju bantuan profesional.** Fokus demonstrasi adalah menjaga makna cerita, kendali pengguna atas informasi, dan kepastian status serah terima.

Nama kerja kapabilitas utama: **Sela Relay**. Nama ini adalah usulan produk; keunikan merek, paten, dan kebaruan global belum diverifikasi.

Dokumen ini menambah rancangan dan bahan keputusan. Ia tidak mengubah SPEC.md repo, tidak mengimplementasikan runtime Go, dan tidak menunjukkan integrasi Meta yang sudah aktif. Semua fitur tambahan berstatus **PROPOSED**. Batas demo sintetis pada SPEC.md tetap berlaku.

## Urutan membaca

1. [Audit repo dan keputusan](01_REPO_AUDIT.md)
2. [Produk dan Sela Relay](02_PRODUCT_STRATEGY.md)
3. [Alur dan frontend ID/EN](03_JOURNEY_AND_UX.md)
4. [Rencana Go dan kontrak data](04_GO_REFACTOR.md)
5. [Meta, penjurian, dan demo](05_META_AND_JUDGING.md)
6. [Metode dan batas riset](06_RESEARCH_METHOD.md)
7. [Peta bukti ke fitur](07_EVIDENCE_TO_FEATURE.md)
8. [Roadmap dan validasi owner](08_ROADMAP_AND_VALIDATION.md)
9. [100 publikasi beranotasi](09_RESEARCH_REGISTER.md)
10. [Evidence appraisal, integritas register, dan batas klaim](10_EVIDENCE_APPRAISAL_AND_CLAIMS.md)
11. [Sela Relay end-to-end, Meaning Lock dan Return Bridge](11_PRODUCT_VERTICAL_SLICE.md)
12. [Kontrak implementasi Go + Meta dan migrasi](12_GO_META_IMPLEMENTATION_CONTRACT.md)
13. [Runbook penjurian, evaluasi dan demo](13_JUDGING_AND_DEMO_PACKAGE.md)
14. [Protokol penyaringan metadata 7.580 kandidat dan antrean telaah](14_METADATA_SCREENING_PROTOCOL.md)

Data sitasi tersedia dalam [JSON](research_100.json) dan [BibTeX](research_100.bib). Teks lengkap artikel tidak disalin ke paket ini. Buka DOI atau tautan indeks untuk membaca sumber.

## Kesimpulan kerja

Sela sudah memiliki arah v2 pada dokumen repo, tetapi kode yang berjalan masih v1. Fondasi Care Passport yang diwarisi sudah mencakup kutipan sumber, klarifikasi, consent AI, validasi dan kalimat pembuka konsultasi. Refaktor harus mempertahankan perilaku yang bermanfaat sambil membangun kesinambungan menuju tenaga profesional.

100 publikasi pada paket ini telah diperiksa pada tingkat judul, metadata dan abstrak yang tersedia. **Ini evidence map awal, bukan systematic review, bukan 100 telaah naskah lengkap, dan bukan validasi klinis Sela.** Riset mencakup temuan positif, hasil nihil, kegagalan implementasi dan risiko.

## Pitch

**ID:** Sela membantu orang yang belum tahu cara memulai konsultasi mengubah ceritanya menjadi catatan yang mereka pahami dan setujui, menemukan pilihan bantuan yang sesuai kendala praktis, lalu mengikuti serah terima sampai ada respons dari penerima.

**EN:** Sela helps people prepare for professional mental-health support by turning their story into a source-linked, user-approved brief and carrying it through a consent-controlled handoff with visible next steps. The hackathon demonstrates this journey with fictional people and simulated care.

## Sumber proyek

Repo yang diperiksa: https://github.com/YanIanZ/sela pada commit `c51807ccdc5c3bc0df308b5f40d3d68ae8a6e1fd`. Repo dapat berubah setelah audit. Dokumen pada direktori ini dipublikasikan sebagai bahan riset dan proposal; status implementasi mengikuti SPEC.md.

## Authority and spec traceability

This research/proposal package is non-normative. [SPEC.md](../../../SPEC.md) remains authoritative. Publishing these documents does not adopt proposed features or complete runtime tasks. Preserve C3/C5/C8/C9 and V1–V15; amend the spec before implementation changes scope.

| Document | Relevant spec references |
| --- | --- |
| Repository audit | C5,C10,C11,I.docs,V14,V15 |
| Product strategy and journey | C2,C3,C8,C9,I.web,V1–V8,V13 |
| Go refactor | C1,C6,C7,I.api,I.data,I.meta,V3–V12,V14 |
| Meta and judging | C4,C10,I.meta,V7,V9,V10,V15 |
| Research method, register and evidence map | C10,I.docs,V15 |
| Roadmap and owner validation | I.ops,V1–V15,T2–T9 |
| Evidence appraisal and 100-study claim gates | C10,I.docs,V7,V15 |
| Relay vertical slice and feature proposals | C2,C3,C8,C9,I.web,V1–V8,V13 |
| Go + Meta implementation contract | C1,C4,C6,C7,I.api,I.meta,V3–V15 |
| Judging, experiment and demo runbook | C10,I.ops,V1–V15,T2–T9 |
