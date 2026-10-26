import { useState, type ReactNode } from 'react';
import { useI18n } from '../i18n';
import { Link, useRouter } from '../lib/router';
import { useOptionalSession } from '../lib/session';
import { useOnline } from '../lib/online';
import type { Lang } from '../lib/types';

// LanguageSwitch changes the interface language. For the owner it is also
// saved to the session; on the recipient perspective (no owner session
// context) it is local only and never touches the owner's session.
function LanguageSwitch({ onNotSaved }: { onNotSaved: (failed: boolean) => void }) {
  const { lang, setLang, t } = useI18n();
  const owner = useOptionalSession();
  const change = async (l: Lang) => {
    if (!owner) {
      setLang(l);
      return;
    }
    onNotSaved(!(await owner.changeLanguage(l)));
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

const NAV: { to: string; key: Parameters<ReturnType<typeof useI18n>['t']>[0]; whileInterrupted?: boolean }[] = [
  { to: '/story', key: 'app.nav.story' },
  { to: '/ledger', key: 'app.nav.ledger' },
  { to: '/next', key: 'app.nav.next' },
  { to: '/journey', key: 'app.nav.journey', whileInterrupted: true },
  { to: '/settings', key: 'app.nav.settings', whileInterrupted: true },
];

/** Layout is the page frame. `bare` hides the owner navigation; `recipient`
 * renders the fictional recipient's perspective, which shows nothing derived
 * from an owner session (no expiry, no crisis banner, no navigation). */
export function Layout({ children, bare, recipient }: { children: ReactNode; bare?: boolean; recipient?: boolean }) {
  const { t, dateTime } = useI18n();
  const { path } = useRouter();
  const owner = useOptionalSession();
  const online = useOnline();
  const [langNotSaved, setLangNotSaved] = useState(false);
  const session = recipient ? null : (owner?.session ?? null);
  const ai = recipient ? null : (owner?.ai ?? null);
  const interrupted = session?.safety.interrupted === true;
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
          <LanguageSwitch onNotSaved={setLangNotSaved} />
        </div>
        {session && !bare && !recipient ? (
          <nav aria-label={t('app.nav.label')} className="mainnav">
            <ul>
              {NAV.filter((n) => !interrupted || n.whileInterrupted).map((n) => (
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
      <div className="banners" role="region" aria-label={t('app.notices')}>
        <p className="banner banner-demo">{t('app.demoBanner')}</p>
        {!online ? (
          <p className="banner banner-warn" role="status">
            {t('app.offline')}
          </p>
        ) : null}
        {langNotSaved ? (
          <p className="banner banner-warn" role="status">
            {t('app.language.notSaved')}
          </p>
        ) : null}
        {interrupted && path !== '/help' ? (
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
        {recipient ? null : <p>{aiLine}</p>}
        <p>{t('app.footer.provenance')}</p>
        {session ? <p>{t('app.expires', { time: dateTime(session.expiresAt) })}</p> : null}
        <p className="footer-links">
          <Link to="/privacy">{t('app.nav.privacy')}</Link> · <Link to="/help">{t('app.nav.help')}</Link> · <Link to="/research">{t('app.nav.research')}</Link>
        </p>
      </footer>
    </div>
  );
}
