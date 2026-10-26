import { useI18n, type MessageKey } from '../i18n';
import type { Projection } from '../lib/types';
import { Badge, ProvenanceBadge, QualifierChips } from './ui';
import { useSession } from '../lib/session';

/** ProjectionView renders exactly the fields of a share projection. Used for
 * the owner's preview and the recipient's view, so both show the same thing. */
export function ProjectionView({ p }: { p: Projection }) {
  const { t, dateTime } = useI18n();
  const { catalog } = useSession();
  const slot = catalog?.professionals.flatMap((x) => x.slots).find((s) => s.id === p.slotId);
  return (
    <div className="projection stack-sm">
      <p>
        <Badge tone="demo">{t('app.demoBadge')}</Badge> <ProvenanceBadge provenance={p.provenance} />
      </p>
      {slot ? (
        <p>
          <strong>{t('recipient.slot')}:</strong> {dateTime(slot.start)} ({t(`next.slot.${slot.format}` as MessageKey)})
        </p>
      ) : null}
      {p.displayName !== undefined ? (
        <p>
          <strong>{t('recipient.from')}:</strong> {p.displayName === 'anonymous' ? t('recipient.anonymous') : p.displayName}
        </p>
      ) : null}
      {p.ageRange ? <p>{t('recipient.adult')}</p> : null}
      {p.preferredLanguage ? (
        <p>
          <strong>{t('recipient.preferred')}:</strong> {p.preferredLanguage === 'en' ? t('app.lang.en') : t('app.lang.id')}
        </p>
      ) : null}
      <h3>{t('recipient.items', { version: p.briefVersion })}</h3>
      <ul className="projection-items">
        {p.items.map((it, i) => (
          <li key={i}>
            <Badge>{t(`ledger.kind.${it.kind}` as MessageKey)}</Badge> <span lang={p.language}>{it.text}</span>
            <QualifierChips q={it.qualifiers} />
            {it.quote ? (
              <p className="muted small">
                {t('recipient.quote')}: «{it.quote}»
              </p>
            ) : null}
          </li>
        ))}
      </ul>
      {p.unknowns && p.unknowns.length > 0 ? (
        <>
          <h3>{t('recipient.unknowns')}</h3>
          <ul>
            {p.unknowns.map((u) => (
              <li key={u}>{t(`ledger.unknown.${u}` as MessageKey)}</li>
            ))}
          </ul>
        </>
      ) : null}
      {p.opening && p.opening.length > 0 ? (
        <>
          <h3>{t('recipient.opening')}</h3>
          <ol>
            {p.opening.map((l, i) => (
              <li key={i}>{l}</li>
            ))}
          </ol>
        </>
      ) : null}
      {p.preferences ? (
        <>
          <h3>{t('recipient.preferences')}</h3>
          <ul>
            <li>
              {t('next.prefs.language')}: {t(`next.opt.language.${p.preferences.language}` as MessageKey)}
            </li>
            <li>
              {t('next.prefs.format')}: {t(`next.opt.format.${p.preferences.format}` as MessageKey)}
            </li>
            <li>
              {t('next.prefs.budget')}: {t(`next.opt.budget.${p.preferences.budget}` as MessageKey)}
            </li>
            <li>
              {t('next.prefs.availability')}: {t(`next.opt.availability.${p.preferences.availability}` as MessageKey)}
            </li>
          </ul>
        </>
      ) : null}
    </div>
  );
}
