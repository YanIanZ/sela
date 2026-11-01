import fallback from "../data/help.fallback.json";

type Text = { id: string; en: string };

export type HelpResource = {
  id: string;
  name: string;
  region: string;
  regionName: Text;
  channels: { kind: "phone" | "whatsapp" | "web"; value: string }[];
  hours: Text;
  cost: Text;
  description: Text;
  sources: { label: string; url: string; accessedOn: string }[];
  verificationStatus: string;
  lastVerified: string;
  verifiedBy: string;
  reverifyBy: string;
  /** urgent services lead quick-help mode (D-077) */
  urgent?: boolean;
};

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

/** waNumber is the international form wa.me needs: an Indonesian number
 * written with a leading 0 ("0811-843-6633") becomes 62… ("628118436633"). */
export function waNumber(value: string): string {
  const digits = value.replace(/\D/g, "");
  return digits.startsWith("0") ? "62" + digits.slice(1) : digits;
}

export function channelHref(channel: { kind: string; value: string }): string {
  if (channel.kind === "phone") return telHref(channel.value);
  if (channel.kind === "whatsapp") return "https://wa.me/" + waNumber(channel.value);
  return channel.value;
}

export function isStale(resource: HelpResource, now = new Date()): boolean {
  const due = Date.parse(resource.reverifyBy);
  return Number.isFinite(due) && due < now.getTime();
}
