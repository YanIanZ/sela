// Production check of the sharing space (D-080): crisis texts caught by the
// new lexicon and by Meta's self_risk signal must open quick help and never
// appear in the feed; a few opening questions are posted across topics.
// Usage: BASE=https://sela.vertalt.my.id OUT=<dir> node e2e/prod-community-check.e2e.mjs
import { chromium } from "playwright";
import { mkdirSync } from "node:fs";
import { join } from "node:path";

const base = process.env.BASE ?? "https://sela.vertalt.my.id";
const OUT = process.env.OUT ?? "/tmp/sela-prod-community";
const POST = process.env.POST !== "0";
const CRISIS = process.env.CRISIS !== "0";
mkdirSync(OUT, { recursive: true });

const results = [];
const check = (name, ok, detail = "") => {
  results.push({ name, ok, detail });
  console.log(`${ok ? "PASS" : "FAIL"} ${name}${detail ? " — " + detail : ""}`);
};

const crisisTexts = [
  ["lexicon 2026.11.02-2 (height now)", "aku udah berdiri di atap gedung sekarang"],
  ["lexicon 2026.11.02-2 (pills at once)", "aku udah ngumpulin obat banyak buat diminum sekaligus nanti malam"],
  ["Meta self_risk (not in lexicon)", "aku lagi di stasiun nunggu kereta lewat, maaf ya semuanya"],
  ["Meta self_risk (not in lexicon, en)", "I gave away my things today, nobody will need to worry about me after this weekend"],
];

const posts = [
  ["Kerja", "Pertanyaan pembuka dari tim Sela untuk topik kerja: apa satu hal kecil yang membantumu melewati hari kerja yang berat? Boleh sesederhana jalan sebentar atau minum teh hangat."],
  ["Sekolah & kuliah", "Pertanyaan pembuka dari tim Sela untuk topik sekolah & kuliah: kalau tugas menumpuk, bagaimana caramu memulai dari yang paling kecil?"],
  ["Keluarga", "Pertanyaan pembuka dari tim Sela untuk topik keluarga: bagaimana caramu menjaga jarak yang sehat saat obrolan di rumah mulai memanas?"],
  ["Kesepian", "Pertanyaan pembuka dari tim Sela untuk topik kesepian: kapan terakhir kali kamu merasa ditemani, walau hanya sebentar? Ceritakan kalau mau."],
  ["Sehari-hari", "Pertanyaan pembuka dari tim Sela: apa satu hal baik, sekecil apa pun, yang terjadi minggu ini?"],
];

const browser = await chromium.launch();
const page = await (await browser.newContext({ locale: "id-ID", viewport: { width: 1280, height: 900 } })).newPage();
const apiCalls = [];
page.on("response", (r) => {
  if (r.url().includes("/api/v3/community/posts") && r.request().method() === "POST") apiCalls.push(r.status());
});

try {
  await page.goto(base + "/", { waitUntil: "networkidle" });
  await page.getByRole("link", { name: "Masuk ke Beranda", exact: true }).first().click();
  await page.getByRole("radio", { name: "18 tahun ke atas" }).click();
  await page.getByRole("button", { name: "Lanjut" }).click();
  await page.getByRole("radio", { name: /Mulai sebagai tamu/ }).click();
  await page.getByRole("radio", { name: /Simpan di Catatanku/ }).click();
  await page.getByLabel(/Aku sudah membaca dan menyetujui/).click();
  await page.getByLabel(/Aku mengerti cara penyimpanan/).click();
  await page.getByRole("button", { name: "Mulai sebagai tamu", exact: true }).click();
  await page.waitForURL("**/start/ai");
  const cards = page.locator(".s-consent");
  for (let i = 0; i < 2 && (await cards.count()); i++) {
    await cards.first().getByRole("button", { name: "Tidak sekarang" }).click();
    await page.waitForTimeout(200);
  }

  await page.goto(base + "/support/community", { waitUntil: "networkidle" });
  const agree = page.getByLabel(/Aku setuju dengan aturan ruang berbagi/);
  if (await agree.count()) {
    await agree.click();
    await page.getByRole("button", { name: "Masuk ruang berbagi" }).click();
  }
  await page.getByRole("heading", { name: "Bagikan ceritamu" }).waitFor();
  check("sharing space opens for an adult guest", true);
  await page.screenshot({ path: join(OUT, "01-sharing-space.png"), fullPage: true });

  const box = page.getByLabel("Ceritamu");
  for (const [i, [name, text]] of (CRISIS ? crisisTexts : []).entries()) {
    const before = apiCalls.length;
    await box.fill(text);
    await page.getByRole("button", { name: "Bagikan" }).click();
    const dialog = page.getByRole("alertdialog", { name: /Kamu tidak sendirian/ });
    let opened = true;
    try {
      await dialog.waitFor({ timeout: 45000 });
    } catch {
      opened = false;
    }
    const status = apiCalls[before];
    check(`crisis: ${name} opens quick help`, opened && status === 422, `HTTP ${status}`);
    await page.screenshot({ path: join(OUT, `0${i + 2}-crisis-${i + 1}.png`), fullPage: false });
    if (opened) await dialog.getByText(/Kembali ke ruang berbagi/).click();
    await dialog.waitFor({ state: "hidden" });
    await page.waitForTimeout(300);
  }
  await page.reload({ waitUntil: "networkidle" });
  const feedText = await page.locator(".c-community-feed").innerText();
  check("no crisis text appears in the feed", crisisTexts.every(([, t]) => !feedText.includes(t.slice(0, 30))));

  if (POST) {
    for (const [i, [topic, text]] of posts.entries()) {
      await page.locator(".c-community-compose").getByRole("group", { name: "Topik", exact: true }).getByRole("button", { name: topic, exact: true }).click();
      await page.getByLabel("Ceritamu").fill(text);
      const before = apiCalls.length;
      await page.getByRole("button", { name: "Bagikan" }).click();
      for (let t = 0; t < 90 && apiCalls.length === before; t++) await page.waitForTimeout(500);
      const ok = await page.getByText("Ceritamu sudah dibagikan.").isVisible().catch(() => false);
      check(`post (${topic}) published`, apiCalls[before] === 201 && ok, `HTTP ${apiCalls[before]}`);
      await page.waitForTimeout(1500);
    }
    await page.reload({ waitUntil: "networkidle" });
    const feed = await page.locator(".c-community-feed").innerText();
    check("posts appear in the feed", posts.every(([, t]) => feed.includes(t.slice(0, 40))));
    await page.screenshot({ path: join(OUT, "10-feed-after-posts.png"), fullPage: true });
  }
} catch (e) {
  check("script ran to the end", false, String(e).slice(0, 300));
  await page.screenshot({ path: join(OUT, "99-error.png"), fullPage: true }).catch(() => {});
} finally {
  await browser.close();
}
const failed = results.filter((r) => !r.ok).length;
console.log(`${results.length - failed}/${results.length} checks passed; screenshots in ${OUT}`);
process.exit(failed ? 1 : 0);
