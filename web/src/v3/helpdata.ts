import fallback from "../data/help.fallback.json";
import type { HelpResource } from "../lib/types";

export const helpData = fallback as unknown as {
  helpNotice: { id: string; en: string };
  help: HelpResource[];
};

/** telHref dials the main number only. An extension stays visible in the
 * label: stripping the separator would dial a different number. */
export function telHref(value: string): string {
  const main = value.split(/\s*(?:ext\.?|ekstensi|x)\s*\d+\s*$/i)[0] ?? value;
  return `tel:${main.replace(/[^0-9+]/g, "")}`;
}

export function channelHref(channel: { kind: string; value: string }): string {
  if (channel.kind === "phone") return telHref(channel.value);
  if (channel.kind === "whatsapp") return "https://wa.me/" + channel.value.replace(/\D/g, "");
  return channel.value;
}

export function isStale(resource: HelpResource, now = new Date()): boolean {
  const due = Date.parse(resource.reverifyBy);
  return Number.isFinite(due) && due < now.getTime();
}
