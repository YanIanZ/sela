import { useEffect, useState, type ReactNode } from "react";
import { api } from "./api";
import { useLocale, useWords } from "./shared";
import { Button, Notice } from "./ui";

/** One difference the deterministic Meaning Diff found (D-078). */
export type MeaningDifference = {
  code: string;
  severity: "critical" | "review" | string;
  source?: string;
  wording?: string;
};
export type MeaningCheck = {
  version: string;
  critical: boolean;
  differences: MeaningDifference[];
};
export type StaleInfo = { reason: string; eventId: string; at: string };
export type MeaningNode = { kind: string; id: string; version: number };
export type MeaningImpact = {
  eventId?: string;
  reason?: string;
  origin: MeaningNode;
  memories: number;
  summaries: number;
  shares: number;
  careRequests: number;
  nodes?: MeaningNode[];
  at?: string;
};

type Words = (id: string, en: string) => string;

const q = (s?: string) => (s ? `“${s}”` : "");

/** differenceLabel turns one difference into a plain sentence. */
export function differenceLabel(d: MeaningDifference, w: Words): string {
  switch (d.code) {
    case "frequency":
      return d.source || d.wording
        ? w(
            `Frekuensi berubah: ${q(d.source) || "tidak disebut"} → ${q(d.wording) || "tidak disebut"}`,
            `Frequency changed: ${q(d.source) || "not stated"} → ${q(d.wording) || "not stated"}`,
          )
        : w("Frekuensi berubah", "Frequency changed");
    case "negation":
      return w(
        "Kata “tidak/bukan” hilang atau bertambah, jadi artinya bisa terbalik",
        "A “not/no” was dropped or added, so the meaning may flip",
      );
    case "subject":
      return w(
        "Pelakunya bergeser (orang lain ↔ kamu)",
        "Who it is about shifted (someone else ↔ you)",
      );
    case "certainty":
      return d.severity === "critical"
        ? w(
            `Kepastian naik: ${q(d.source) || "ragu-ragu"} jadi pasti`,
            `Certainty raised: ${q(d.source) || "unsure"} became certain`,
          )
        : w("Tingkat kepastian berubah", "How certain it sounds changed");
    case "generalization":
      return w(
        `Digeneralisasi${d.wording ? `: ${q(d.wording)}` : ""}`,
        `Generalised${d.wording ? `: ${q(d.wording)}` : ""}`,
      );
    case "diagnosis":
      return w("Menambahkan label diagnosis", "Adds a diagnosis label");
    case "medical":
      return w("Menambahkan saran medis", "Adds medical advice");
    case "time":
      return w(
        `Waktunya berbeda${d.source || d.wording ? `: ${q(d.source) || "tidak disebut"} → ${q(d.wording) || "tidak disebut"}` : ""}`,
        `The time differs${d.source || d.wording ? `: ${q(d.source) || "not stated"} → ${q(d.wording) || "not stated"}` : ""}`,
      );
    case "number":
      return w("Ada angka yang tidak kamu tulis", "Has a number you didn't write");
    case "added_content":
      return w("Ada kata yang tidak ada di kata-katamu", "Has words that aren't in yours");
    default:
      return w("Ada perbedaan makna", "Something in the meaning differs");
  }
}

/**
 * MeaningDiff shows the user's own words next to Sela's wording and the
 * differences the deterministic check found. It never claims the check is
 * complete: the user decides what they meant.
 */
export function MeaningDiff({
  quote,
  wording,
  meaning,
  onUseMine,
  busy,
  wordingLabel,
}: {
  quote?: string;
  wording: string;
  meaning?: MeaningCheck | null;
  onUseMine?: () => void;
  busy?: boolean;
  wordingLabel?: string;
}) {
  const w = useWords();
  const diffs = meaning?.differences ?? [];
  const critical = diffs.filter((d) => d.severity === "critical");
  const review = diffs.filter((d) => d.severity !== "critical");
  return (
    <div className="c-meaning">
      <div className="c-meaning-pair">
        {quote && (
          <figure className="c-meaning-side">
            <figcaption>{w("Kata-katamu", "Your words")}</figcaption>
            <blockquote>
              <p>{quote}</p>
            </blockquote>
          </figure>
        )}
        <figure className="c-meaning-side c-meaning-side--sela">
          <figcaption>{wordingLabel ?? w("Rumusan Sela", "Sela's wording")}</figcaption>
          <p className="c-original">{wording}</p>
        </figure>
      </div>
      {meaning && (
        <p className="s-hint" role="status">
          {critical.length > 0
            ? w(
                `Pemeriksaan makna menemukan ${critical.length} perubahan penting.`,
                `The meaning check found ${critical.length} important change${critical.length === 1 ? "" : "s"}.`,
              )
            : review.length > 0
              ? w(
                  "Tidak ada perubahan makna penting yang terdeteksi; ada hal kecil untuk dicek.",
                  "No important change in meaning was detected; a few small things to check.",
                )
              : w(
                  "Tidak ada perubahan makna yang terdeteksi. Tetap kamu yang menentukan.",
                  "No change in meaning was detected. You still decide.",
                )}
        </p>
      )}
      {critical.length > 0 && (
        <Notice tone="warning" title={w("Sela mungkin mengubah maknamu", "Sela may have changed your meaning")}>
          <p>{w("Periksa sebelum menyimpan.", "Check before saving.")}</p>
          <ul className="c-meaning-list">
            {critical.map((d, i) => (
              <li key={d.code + i}>
                <strong>{w("Penting", "Important")}:</strong> {differenceLabel(d, w)}
              </li>
            ))}
          </ul>
          {onUseMine && quote && (
            <div className="s-actions">
              <Button variant="primary" onClick={onUseMine} disabled={busy}>
                {w("Pakai kata-kataku", "Use my words")}
              </Button>
            </div>
          )}
        </Notice>
      )}
      {review.length > 0 && (
        <ul className="c-meaning-list c-meaning-list--review" aria-label={w("Hal kecil untuk dicek", "Small things to check")}>
          {review.map((d, i) => (
            <li key={d.code + i}>
              <span className="c-meaning-chip">{w("Cek", "Check")}</span> {differenceLabel(d, w)}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

/** staleCopy explains why a version stopped being current. */
export function staleCopy(reason: string, w: Words): { title: string; body: string } {
  switch (reason) {
    case "claim_corrected":
      return {
        title: w("Pemahaman yang dirujuk sudah kamu koreksi", "You corrected the meaning this rests on"),
        body: w(
          "Kamu mengoreksi pemahaman yang dirujuk. Tinjau dan simpan versi baru sebelum dipakai lagi.",
          "You corrected the meaning this cites. Review it and save a new version before using it again.",
        ),
      };
    case "claim_rejected":
      return {
        title: w("Pemahaman yang dirujuk sudah kamu tolak", "You rejected the meaning this rests on"),
        body: w(
          "Bagian yang memakai pemahaman itu tidak boleh dipakai lagi. Hapus atau ganti, lalu simpan versi baru.",
          "Statements using that meaning can't be used again. Remove or replace them, then save a new version.",
        ),
      };
    case "claim_deleted":
      return {
        title: w("Pemahaman yang dirujuk sudah dihapus", "The meaning this rests on was deleted"),
        body: w("Susun ulang tanpa bagian itu sebelum dipakai lagi.", "Rebuild without it before using it again."),
      };
    case "source_deleted":
      return {
        title: w("Sumbernya sudah dihapus", "Its source was deleted"),
        body: w(
          "Versi ini tetap tersimpan sebagai riwayatmu, tapi tidak lagi dibagikan atau dipakai.",
          "This version stays as your history, but it is no longer shared or used.",
        ),
      };
    default:
      return {
        title: w("Sumbernya sudah berubah", "Its source has changed"),
        body: w(
          "Kamu mengubah tulisan sumbernya. Tinjau dan simpan versi baru sebelum dipakai lagi.",
          "You changed the source writing. Review it and save a new version before using it again.",
        ),
      };
  }
}

/** StaleNotice says why a record version is no longer current. */
export function StaleNotice({ stale, actions }: { stale: StaleInfo; actions?: ReactNode }) {
  const w = useWords();
  const loc = useLocale();
  const copy = staleCopy(stale.reason, w);
  return (
    <Notice tone="warning" title={copy.title} actions={actions}>
      <p>{copy.body}</p>
      <p className="s-hint">
        {w("Ditandai", "Marked")} {loc.dateTime(stale.at)}
      </p>
    </Notice>
  );
}

/** StaleBadge is the short list label for a stale version. */
export function StaleBadge() {
  const w = useWords();
  return <span className="c-meaning-chip c-meaning-chip--stale">{w("Perlu ditinjau ulang", "Needs review again")}</span>;
}

/** impactText describes what a change will reach; "" for no impact. */
export function impactText(impact: MeaningImpact | null | undefined, w: Words): string {
  if (!impact) return "";
  const parts: string[] = [];
  if (impact.memories > 0) parts.push(w(`menandai ${impact.memories} pemahaman perlu ditinjau`, `mark ${impact.memories} remembered meaning${impact.memories === 1 ? "" : "s"} for review`));
  if (impact.summaries > 0) parts.push(w(`menandai ${impact.summaries} ringkasan perlu ditinjau`, `mark ${impact.summaries} summar${impact.summaries === 1 ? "y" : "ies"} for review`));
  if (impact.shares > 0) parts.push(w(`menghentikan ${impact.shares} tautan berbagi`, `stop ${impact.shares} share link${impact.shares === 1 ? "" : "s"}`));
  if (impact.careRequests > 0) parts.push(w(`membatalkan ${impact.careRequests} permintaan ke tenaga profesional`, `cancel ${impact.careRequests} request${impact.careRequests === 1 ? "" : "s"} to a professional`));
  if (parts.length === 0) return "";
  const list = parts.length === 1 ? parts[0] : `${parts.slice(0, -1).join(", ")}${w(", dan ", ", and ")}${parts[parts.length - 1]}`;
  return w(
    `Ini akan ${list}. Yang sudah dibaca orang lain tidak bisa ditarik kembali.`,
    `This will ${list}. What others already read cannot be recalled.`,
  );
}

/** ImpactSummary renders impactText, or nothing for zero impact. */
export function ImpactSummary({ impact }: { impact: MeaningImpact | null | undefined }) {
  const w = useWords();
  const text = impactText(impact, w);
  if (!text) return null;
  return <p className="c-meaning-impact">{text}</p>;
}

/** useMeaningImpact loads the dry-run impact of changing record id. */
export function useMeaningImpact(id: string | null | undefined, enabled = true, refresh: unknown = 0) {
  const [impact, setImpact] = useState<MeaningImpact | null>(null);
  useEffect(() => {
    if (!id || !enabled) {
      setImpact(null);
      return;
    }
    let live = true;
    api<{ impact: MeaningImpact }>(`/meaning/impact?id=${encodeURIComponent(id)}`)
      .then((d) => live && setImpact(d.impact))
      .catch(() => live && setImpact(null));
    return () => {
      live = false;
    };
  }, [id, enabled, refresh]);
  return impact;
}

/** latestMeaningEvent returns the newest invalidation, if any. */
export async function latestMeaningEvent(): Promise<MeaningImpact | null> {
  try {
    const d = await api<{ events: MeaningImpact[] }>("/meaning/events");
    return d.events[0] ?? null;
  } catch {
    return null;
  }
}

/** resultText describes an invalidation that already happened. */
export function resultText(event: MeaningImpact | null | undefined, w: Words): string {
  if (!event) return "";
  const parts: string[] = [];
  if (event.memories > 0) parts.push(w(`${event.memories} pemahaman`, `${event.memories} meaning${event.memories === 1 ? "" : "s"}`));
  if (event.summaries > 0) parts.push(w(`${event.summaries} ringkasan`, `${event.summaries} summar${event.summaries === 1 ? "y" : "ies"}`));
  if (parts.length === 0 && event.shares === 0 && event.careRequests === 0) return "";
  let out = parts.length > 0 ? w(`Perubahanmu menandai ${parts.join(" dan ")} perlu ditinjau.`, `Your change marked ${parts.join(" and ")} for review.`) : "";
  if (event.shares > 0) out += " " + w(`${event.shares} tautan berbagi dihentikan.`, `${event.shares} share link${event.shares === 1 ? " was" : "s were"} stopped.`);
  if (event.careRequests > 0) out += " " + w(`${event.careRequests} permintaan ke tenaga profesional dibatalkan.`, `${event.careRequests} request${event.careRequests === 1 ? " to a professional was" : "s to professionals were"} cancelled.`);
  return out.trim();
}

/**
 * MeaningChips lists a statement's differences compactly (summary items:
 * the owner wrote them, so this is advice, not a warning).
 */
export function MeaningChips({ meaning }: { meaning?: MeaningCheck | null }) {
  const w = useWords();
  const diffs = meaning?.differences ?? [];
  if (diffs.length === 0) return null;
  return (
    <ul className="c-meaning-list c-meaning-list--review" aria-label={w("Pemeriksaan makna", "Meaning check")}>
      {diffs.map((d, i) => (
        <li key={d.code + i}>
          <span className="c-meaning-chip">{d.severity === "critical" ? w("Penting", "Important") : w("Cek", "Check")}</span>{" "}
          {differenceLabel(d, w)}
        </li>
      ))}
    </ul>
  );
}
