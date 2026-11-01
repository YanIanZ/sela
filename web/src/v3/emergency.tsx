import { Link } from "../lib/router";
import { useI18n } from "../i18n";
import { useWords } from "./shared";
import { Icon } from "./icons";
import { helpData, channelHref } from "./helpdata";
import { Dialog, buttonClass } from "./ui";

/**
 * EmergencySheet is the quick-help mode. It opens by itself the moment the
 * safety check reads danger in what the person wrote (a threat to themselves
 * or someone else, or being hurt or threatened), so verified help is the
 * first thing on screen instead of a line further down the page. It contacts
 * no one: every action is the person's own choice, and closing it returns to
 * exactly where they were.
 */
export function EmergencySheet({
  open,
  onClose,
  context = "talk",
}: {
  open: boolean;
  onClose: () => void;
  context?: "talk" | "note" | "community" | "map";
}) {
  const w = useWords();
  const { lang } = useI18n();
  // The services marked urgent: the emergency number, the ambulance and
  // the 24-hour mental-health line, in the catalogue order.
  const services = helpData.help.filter((s) => s.urgent);
  return (
    <Dialog
      open={open}
      onClose={onClose}
      alert
      className="s-emergency-sheet"
      title={w("Kamu tidak sendirian", "You're not alone")}
    >
      <div className="s-emergency-body">
        <p className="s-emergency-lead">
          {w(
            "Kalau kamu atau orang lain sedang dalam bahaya sekarang, hubungi layanan darurat setempat atau minta orang terdekat menemanimu.",
            "If you or someone else is in danger right now, contact local emergency services or ask someone nearby to stay with you.",
          )}
        </p>
        <p className="s-hint">
          {context === "talk"
            ? w(
                "Balasan Sela dijeda di percakapan ini. Sela tidak menghubungi siapa pun atas namamu.",
                "Sela's replies are paused in this conversation. Sela doesn't contact anyone on your behalf.",
              )
            : context === "community"
            ? w(
                "Tulisan ini tidak disimpan dan tidak ditampilkan ke siapa pun. Sela tidak menghubungi siapa pun atas namamu.",
                "This text was not saved or shown to anyone. Sela doesn't contact anyone on your behalf.",
              )
            : context === "map"
            ? w(
                "Peta kondisimu tidak disimpan dan tidak dikirim ke siapa pun. Sela tidak menghubungi siapa pun atas namamu.",
                "Your condition map isn't stored or sent to anyone. Sela doesn't contact anyone on your behalf.",
              )
            : w(
                "Tulisanmu tetap tersimpan seperti pilihanmu. Sela tidak menghubungi siapa pun atas namamu.",
                "Your writing stays saved as you chose. Sela doesn't contact anyone on your behalf.",
              )}
        </p>
        <ul className="s-emergency-services">
          {services.map((s) => (
            <li key={s.id}>
              <div className="s-emergency-service-head">
                <strong>{s.name}</strong>
                <span className="s-hint">{s.hours[lang]}</span>
              </div>
              <div className="s-emergency-channels">
                {s.channels
                  .filter((c) => c.kind !== "web")
                  .map((c) => (
                    <a
                      key={c.kind + c.value}
                      className={buttonClass(c.kind === "phone" ? "help" : "secondary")}
                      href={channelHref(c)}
                    >
                      <Icon name={c.kind === "phone" ? "phone" : "whatsapp"} />
                      <span>
                        {c.kind === "phone" ? w("Telepon ", "Call ") : "WhatsApp "}
                        {c.value}
                      </span>
                    </a>
                  ))}
              </div>
            </li>
          ))}
        </ul>
        <div className="s-emergency-actions">
          <Link to="/help" className={buttonClass("primary")} onClick={onClose}>
            <Icon name="help" />
            <span>{w("Lihat semua bantuan", "See all help")}</span>
          </Link>
          <button
            type="button"
            className={buttonClass("secondary")}
            onClick={() => window.dispatchEvent(new Event("sela:request-quick-exit"))}
          >
            <Icon name="quick-exit" />
            <span>{w("Keluar cepat", "Quick exit")}</span>
          </button>
          <button type="button" className={buttonClass("quiet")} onClick={onClose}>
            {context === "talk"
              ? w("Kembali ke percakapan", "Back to the conversation")
              : context === "community"
              ? w("Kembali ke ruang berbagi", "Back to the sharing space")
              : context === "map"
              ? w("Kembali ke peta kondisi", "Back to the condition map")
              : w("Kembali ke catatan", "Back to the note")}
          </button>
        </div>
      </div>
    </Dialog>
  );
}
