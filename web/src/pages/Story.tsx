import { useState } from 'react';
import { useI18n } from '../i18n';
import { useRouter } from '../lib/router';
import { useSession } from '../lib/session';
import { request } from '../lib/api';
import type { SessionView } from '../lib/types';
import { Button, Card, ErrorNotice, Notice, PageHeader } from '../components/ui';

const MAX = 3000;
const MIN = 20;

type StoryResult = { safety: 'none' | 'elevated' | 'crisis'; session: SessionView };

export function StoryPage() {
  const { t, tx } = useI18n();
  const { navigate } = useRouter();
  const { session, catalog, run, setSession } = useSession();
  const [text, setText] = useState('');
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [notice, setNotice] = useState<'elevated' | 'thirdParty' | null>(null);
  if (!session || !catalog) return null;

  const submit = async (body: { mode: 'scenario'; scenarioId: string } | { mode: 'practice'; text: string }, key: string) => {
    setBusy(key);
    setError(null);
    setNotice(null);
    try {
      const res = await run((version) => request<StoryResult>('/story', { method: 'PUT', body: { expectedVersion: version, ...body } }));
      setSession(res.session);
      if (res.safety === 'crisis' || res.session.safety.interrupted) navigate('/help');
      else if (res.safety === 'elevated') setNotice(res.session.safety.thirdParty ? 'thirdParty' : 'elevated');
      else navigate('/ledger');
    } catch (err) {
      setError(err);
    } finally {
      setBusy(null);
    }
  };

  const current = session.story;
  const currentScenario = current?.scenarioId ? catalog.scenarios.find((s) => s.id === current.scenarioId) : undefined;
  const ordered = [...catalog.scenarios].sort((a, b) => (a.kind === b.kind ? 0 : a.kind === 'crisis' ? 1 : -1));
  return (
    <div className="stack">
      <PageHeader title={t('story.title')} lead={t('story.lead')} />
      <ErrorNotice error={error} />
      {notice ? (
        <Notice tone="warning">
          {notice === 'thirdParty' ? t('story.thirdParty') : t('story.elevated')}{' '}
          <Button variant="quiet" onClick={() => navigate('/ledger')}>
            {t('app.continue')}
          </Button>
        </Notice>
      ) : null}
      {current ? (
        <Card labelledBy="current-story">
          <h2 id="current-story">{t('story.current')}</h2>
          {currentScenario ? <p className="muted">{tx(currentScenario.title)}</p> : null}
          <blockquote className="story-text">{current.segments.filter((s) => s.origin === 'story').map((s) => s.text).join(' ')}</blockquote>
          <p className="muted">{t('story.replaceWarning')}</p>
        </Card>
      ) : null}
      <section aria-labelledby="samples-title" className="stack">
        <h2 id="samples-title">{t('story.samples')}</h2>
        <div className="grid-2">
          {ordered.map((sc) => (
            <article className={`card scenario${sc.kind === 'crisis' ? ' scenario-crisis' : ''}`} key={sc.id} aria-labelledby={`sc-${sc.id}`}>
              <h3 id={`sc-${sc.id}`}>{tx(sc.title)}</h3>
              <p>
                <strong>{t('story.persona')}:</strong> {tx(sc.persona)}
              </p>
              <p className="muted">{tx(sc.summary)}</p>
              {sc.contentWarning ? (
                <p className="warning-text">
                  <strong>{t('story.contentWarning')}:</strong> {tx(sc.contentWarning)}
                </p>
              ) : null}
              <details>
                <summary>{t('story.showStory')}</summary>
                <blockquote className="story-text" lang={sc.language === 'en' ? 'en' : 'id'}>
                  {sc.story}
                </blockquote>
              </details>
              <Button variant="secondary" busy={busy === sc.id} onClick={() => submit({ mode: 'scenario', scenarioId: sc.id }, sc.id)}>
                {t('story.useSample')}
              </Button>
            </article>
          ))}
        </div>
      </section>
      <Card labelledBy="practice-title">
        <h2 id="practice-title">{t('story.practice.title')}</h2>
        <Notice tone="warning">{t('story.practice.warning')}</Notice>
        <div className="field">
          <label htmlFor="practice-text">{t('story.practice.label')}</label>
          <textarea id="practice-text" rows={7} maxLength={MAX} value={text} onChange={(e) => setText(e.target.value)} aria-describedby="practice-count" />
          <p id="practice-count" className="muted">
            {t('story.practice.count', { count: text.length, max: MAX })}
          </p>
        </div>
        <Button busy={busy === 'practice'} disabled={text.trim().length < MIN} onClick={() => submit({ mode: 'practice', text }, 'practice')}>
          {t('story.practice.submit')}
        </Button>
      </Card>
    </div>
  );
}
