import { useI18n } from '../i18n';
import { Link } from '../lib/router';
import { useSession } from '../lib/session';

export function Welcome() {
  const { t } = useI18n();
  const { session } = useSession();
  return (
    <div className="stack welcome">
      <header className="hero">
        <h1>{t('welcome.title')}</h1>
        <p className="tagline">{t('app.tagline')}</p>
        <p className="lead">{t('welcome.lead')}</p>
        <div className="row">
          <Link to={session ? (session.stage === 'crisis' ? '/help' : '/story') : '/start'} className="btn btn-primary">
            {t('welcome.start')}
          </Link>
          <Link to="/help" className="btn btn-quiet">
            {t('welcome.help')}
          </Link>
        </div>
      </header>
      <div className="grid-2">
        <section className="card" aria-labelledby="is-title">
          <h2 id="is-title">{t('welcome.isTitle')}</h2>
          <ul className="checklist">
            <li>{t('welcome.is1')}</li>
            <li>{t('welcome.is2')}</li>
            <li>{t('welcome.is3')}</li>
          </ul>
        </section>
        <section className="card" aria-labelledby="isnot-title">
          <h2 id="isnot-title">{t('welcome.isNotTitle')}</h2>
          <ul className="crosslist">
            <li>{t('welcome.isNot1')}</li>
            <li>{t('welcome.isNot2')}</li>
            <li>{t('welcome.isNot3')}</li>
            <li>{t('welcome.isNot4')}</li>
          </ul>
        </section>
      </div>
      <p>
        <Link to="/research">{t('welcome.research')}</Link>
      </p>
    </div>
  );
}
