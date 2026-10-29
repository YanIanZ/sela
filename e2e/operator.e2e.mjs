// Local browser acceptance for the operator console. Synthetic staff only.
//
// Provision an operator first (RUNBOOK §10), then:
//   E2E_BASE_URL=http://127.0.0.1:8081 \
//   OPERATOR_USERNAME=ops.ui OPERATOR_PASSWORD='…' \
//   OPERATOR_TOTP_SECRET='…' node e2e/operator.e2e.mjs
//
// The script never prints the password or the TOTP secret.
import { chromium } from "playwright";
import { readFileSync, mkdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { createHmac } from "node:crypto";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "..");
const OUT = join(ROOT, "e2e/results/operator");
mkdirSync(OUT, { recursive: true });
const axeSource = readFileSync(join(ROOT, "web/node_modules/axe-core/axe.min.js"), "utf8");
const base = process.env.E2E_BASE_URL ?? "http://127.0.0.1:8080";
const username = process.env.OPERATOR_USERNAME;
const password = process.env.OPERATOR_PASSWORD;
const secret = process.env.OPERATOR_TOTP_SECRET;

if (!username || !password || !secret) {
  console.error(
    "operator.e2e: set OPERATOR_USERNAME, OPERATOR_PASSWORD and OPERATOR_TOTP_SECRET (RUNBOOK §10)",
  );
  process.exit(2);
}

function base32(text) {
  const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";
  let bits = 0;
  let value = 0;
  const out = [];
  for (const char of text.replace(/=+$/, "").replace(/[\s-]/g, "").toUpperCase()) {
    const index = alphabet.indexOf(char);
    if (index < 0) throw new Error("invalid base32 secret");
    value = (value << 5) | index;
    bits += 5;
    if (bits >= 8) {
      out.push((value >>> (bits - 8)) & 0xff);
      bits -= 8;
    }
  }
  return Buffer.from(out);
}

function totp(now = Date.now()) {
  const counter = Math.floor(now / 1000 / 30);
  const buf = Buffer.alloc(8);
  buf.writeBigUInt64BE(BigInt(counter));
  const digest = createHmac("sha1", base32(secret)).update(buf).digest();
  const offset = digest[digest.length - 1] & 0x0f;
  const code = (digest.readUInt32BE(offset) & 0x7fffffff) % 1_000_000;
  return String(code).padStart(6, "0");
}

const results = [];
const errors = [];

function check(name, ok, detail = "") {
  results.push({ name, ok, detail });
  console.log(`${ok ? "PASS" : "FAIL"} ${name}${detail ? " — " + detail : ""}`);
}

async function audit(page, name) {
  await page.evaluate(async () => {
    try {
      await document.fonts.ready;
    } catch {
      /* older engines */
    }
    await new Promise((resolve) =>
      requestAnimationFrame(() => requestAnimationFrame(resolve)),
    );
  });
  await page.evaluate(axeSource);
  const result = await page.evaluate(async () =>
    await axe.run(document, {
      runOnly: {
        type: "tag",
        values: ["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"],
      },
    }),
  );
  const bad = result.violations.filter((v) =>
    ["serious", "critical"].includes(v.impact ?? ""),
  );
  check(
    name,
    bad.length === 0,
    bad.map((v) => `${v.id} (${v.nodes.length})`).join(", "),
  );
}

const browser = await chromium.launch({ headless: true });
const context = await browser.newContext({
  locale: "en-US",
  viewport: { width: 1366, height: 900 },
});
const page = await context.newPage();
page.on("console", (message) => {
  // The anonymous probe and the sign-out are expected 401s: there is no
  // staff session yet or any more. Everything else is a real error.
  if (message.type() === "error" && !/401 \(Unauthorized\)/.test(message.text())) {
    errors.push(message.text());
  }
});
page.on("pageerror", (error) => errors.push(String(error)));

try {
  await page.goto(`${base}/operator`, { waitUntil: "networkidle" });
  await page
    .getByRole("heading", { name: "Sign in to the operator console", exact: true })
    .waitFor();
  check("login screen loads at /operator", true);
  check("consumer shell is not mounted", (await page.locator(".s-shell").count()) === 0);
  await audit(page, "login screen has no serious/critical WCAG 2.2 AA findings");

  await page.getByLabel("Username").fill(username);
  await page.getByLabel("Password").fill(password);
  await page.getByLabel("MFA code").fill(totp());
  await page.getByRole("button", { name: "Sign in" }).click();

  await page.getByRole("heading", { name: "Operator console", exact: true }).waitFor();
  await page.getByText("Active staff").waitFor();
  await page.getByText(/Database ready/).waitFor();
  await page.getByText("Healthy", { exact: true }).first().waitFor();
  check("TOTP sign-in reaches the dashboard", true);
  check(
    "overview shows the staff count",
    (await page.locator(".op-card", { hasText: "Active staff" }).innerText()).match(/\d/) !== null,
  );
  check(
    "public cache reports healthy",
    (await page.getByText("Public cache", { exact: true }).count()) > 0 &&
      (await page.getByText("Healthy", { exact: true }).count()) > 0,
  );
  check(
    "audit trail is visible to the owner",
    (await page.getByRole("heading", { name: "Operator audit", exact: true }).count()) === 1,
  );
  await audit(page, "dashboard has no serious/critical WCAG 2.2 AA findings");

  await page.screenshot({
    path: join(OUT, "operator-desktop.png"),
    fullPage: true,
  });

  await page.emulateMedia({ colorScheme: "dark" });
  await page.waitForTimeout(150);
  await audit(page, "dashboard in dark mode has no serious/critical WCAG 2.2 AA findings");
  await page.screenshot({
    path: join(OUT, "operator-desktop-dark.png"),
    fullPage: true,
  });
  await page.emulateMedia({ colorScheme: "light" });

  await page.setViewportSize({ width: 320, height: 720 });
  await page.waitForTimeout(250);
  const overflow = await page.evaluate(
    () => document.documentElement.scrollWidth - window.innerWidth,
  );
  check("320 px layout does not scroll horizontally", overflow <= 1, `${overflow}px`);
  await page.screenshot({
    path: join(OUT, "operator-320.png"),
    fullPage: true,
  });

  await page.setViewportSize({ width: 1366, height: 900 });
  await page.getByRole("button", { name: "Sign out" }).click();
  await page
    .getByRole("heading", { name: "Sign in to the operator console", exact: true })
    .waitFor();
  check("sign out ends the operator session", true);
} catch (error) {
  check("journey completed without an exception", false, String(error));
} finally {
  await browser.close();
}

check("no console or page errors", errors.length === 0, errors.slice(0, 3).join(" | "));

const failed = results.filter((r) => !r.ok);
console.log(
  `\noperator e2e: ${results.length - failed.length}/${results.length} checks passed`,
);
process.exit(failed.length === 0 ? 0 : 1);
