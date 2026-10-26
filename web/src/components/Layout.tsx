import type { ReactNode } from 'react';
import { useI18n } from '../i18n';
import { Link, useRouter } from '../lib/router';
import { useSession } from '../lib/session';
import { request } from '../lib/api';
import type { Lang, SessionView } from '../lib/types';

function LanguageSwitch() {
  const { lang, setLang, t } = useI18n();
  const { session, run, setSession } = useSession();
  const change = async (l: Lang) => {
    setLang(l);
    if (session && session.language !== l) {
      try {
        const v = await run((version) => request<SessionView>('/session', { method: 'PATCH', body: { expectedVersion: version, language: l } }));
        setSession(v);
      } catch {
        // The interface language already changed locally; the server keeps the previous one.
      }
    }
  };
  return (
    <div className="lang-switch" role="group" aria-label={t('app.language')}>
      {(['id', 'en'] as const).map((l) => (
        <button key={l} type="button" className={l === lang ? 'active' : undefined} aria-pressed={l === lang} onClick={() => change(l)}>
          {l === 'id' ? t('app.lang.id') : t('app.lang.en')}
        </button>
      ))}
    </div>
  );
}

const NAV: { to: string; key: Parameters<ReturnType<typeof useI18n>['t']>[0] }[] = [
  { to: '/story', key: 'app.nav.story' },
  { to: '/ledger', key: 'app.nav.ledger' },
  { to: '/next', key: 'app.nav.next' },
  { to: '/journey', key: 'app.nav.journey' },
  { to: '/settings', key: 'app.nav.settings' },
];

export function Layout({ children, bare }: { children: ReactNode; bare?: boolean }) {
  const { t, dateTime } = useI18n();
  const { path } = useRouter();
  const { session, online, ai } = useSession();
  const mode = ai?.provider.mode ?? 'disabled';
  const aiLine =
    mode === 'modal'
      ? t('app.footer.model.modal', { revision: (ai?.provider.revision ?? '').slice(0, 7) })
      : mode === 'test_double'
        ? t('app.footer.model.test_double')
        : t('app.footer.model.disabled');
  return (
    <div className="shell">
      <a className="skip-link" href="#main">
        {t('app.skip')}
      </a>
      <header className="topbar">
        <div className="topbar-inner">
          <Link to="/" className="brand">
            <span className="brand-mark" aria-hidden="true" />
            <span className="brand-name">{t('app.name')}</span>
          </Link>
          <span className="badge badge-demo">{t('app.demoBadge')}</span>
          <LanguageSwitch />
        </div>
        {session && !bare ? (
          <nav aria-label={t('app.nav.label')} className="mainnav">
            <ul>
              {NAV.map((n) => (
                <li key={n.to}>
                  <Link to={n.to} aria-current={path.startsWith(n.to) ? 'page' : undefined}>
                    {t(n.key)}
                  </Link>
                </li>
              ))}
              <li>
                <Link to="/help" aria-current={path === '/help' ? 'page' : undefined} className="nav-help">
                  {t('app.nav.help')}
                </Link>
              </li>
            </ul>
          </nav>
        ) : null}
      </header>
      <div className="banners">
        <p className="banner banner-demo">{t('app.demoBanner')}</p>
        {!online ? (
          <p className="banner banner-warn" role="status">
            {t('app.offline')}
          </p>
        ) : null}
        {session?.safety.interrupted && path !== '/help' ? (
          <p className="banner banner-alert" role="alert">
            <Link to="/help">{t('app.crisisBanner')}</Link>
          </p>
        ) : null}
      </div>
      <main id="main" tabIndex={-1}>
        {children}
      </main>
      <footer className="footer">
        <p>{t('app.footer.notService')}</p>
        <p>{aiLine}</p>
        <p>{t('app.footer.provenance')}</p>
        {session ? <p>{t('app.expires', { time: dateTime(session.expiresAt) })}</p> : null}
        <p className="footer-links">
          <Link to="/privacy">{t('app.nav.privacy')}</Link> · <Link to="/help">{t('app.nav.help')}</Link> · <Link to="/research">{t('app.nav.research')}</Link>
        </p>
      </footer>
    </div>
  );
}
