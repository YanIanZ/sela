import { useI18n } from '../i18n';
import { useSession } from '../lib/session';
import type { PolicyNotice } from '../lib/types';
import { PageHeader } from '../components/ui';

export function PolicyText({ policy, hideTitle }: { policy: PolicyNotice; hideTitle?: boolean }) {
  const { tx } = useI18n();
  return (
    <div className="policy">
      {hideTitle ? null : <h2>{tx(policy.title)}</h2>}
      <ul>
        {policy.body.map((b, i) => (
          <li key={i}>{tx(b)}</li>
        ))}
      </ul>
    </div>
  );
}

export function PrivacyPage() {
  const { t } = useI18n();
  const { catalog } = useSession();
  if (!catalog) return null;
  const keys = ['privacy', 'ai_processing', 'storage', 'sharing'] as const;
  return (
    <div className="stack">
      <PageHeader title={t('privacy.title')} />
      {keys.map((k) => (
        <section className="card" key={k}>
          <PolicyText policy={catalog.policies[k]} />
          <p className="muted">{catalog.policies[k].version}</p>
        </section>
      ))}
    </div>
  );
}
