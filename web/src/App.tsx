import { Fragment, useEffect, type ReactNode } from 'react';
import { useRouter } from './lib/router';
import { SessionProvider, useSession } from './lib/session';
import { useI18n } from './i18n';
import { Layout } from './components/Layout';
import { Spinner } from './components/ui';
import { Welcome } from './pages/Welcome';
import { Start } from './pages/Start';
import { ConsentPage } from './pages/Consent';
import { StoryPage } from './pages/Story';
import { LedgerPage } from './pages/Ledger';
import { NextStepPage } from './pages/NextStep';
import { PracticePage } from './pages/Practice';
import { SharePage } from './pages/Share';
import { JourneyPage } from './pages/Journey';
import { SettingsPage } from './pages/Settings';
import { PrivacyPage } from './pages/Privacy';
import { HelpPage } from './pages/Help';
import { RecipientPage } from './pages/Recipient';
import { ResearchPage } from './pages/Research';
import { CatalogUnavailable, Interrupted, NotFound, SessionEnded } from './pages/Misc';

const SESSION_PAGES: Record<string, () => ReactNode> = {
  '/consent': () => <ConsentPage />,
  '/story': () => <StoryPage />,
  '/ledger': () => <LedgerPage />,
  '/next': () => <NextStepPage />,
  '/practice': () => <PracticePage />,
  '/share': () => <SharePage />,
  '/journey': () => <JourneyPage />,
  '/settings': () => <SettingsPage />,
};

// Session pages that stay usable on an interrupted session: revoking a share
// (Journey) and deleting the session or changing consent (Settings) are
// privacy actions, never journey steps.
const INTERRUPTED_PAGES = new Set(['/journey', '/settings']);

const PUBLIC_PAGES: Record<string, () => ReactNode> = {
  '/': () => <Welcome />,
  '/start': () => <Start />,
  '/privacy': () => <PrivacyPage />,
  '/help': () => <HelpPage />,
  '/research': () => <ResearchPage />,
};

/** Root picks the perspective: the fictional recipient (/r) renders outside
 * the owner's session context, so nothing there reads or changes the
 * owner's session (expiry, crisis banner, language). */
export function Root() {
  const { path } = useRouter();
  if (path === '/r') {
    return (
      <Layout bare recipient>
        <RecipientPage />
      </Layout>
    );
  }
  return (
    <SessionProvider>
      <App />
    </SessionProvider>
  );
}

export function App() {
  const { path, navigate } = useRouter();
  const { status, session, catalog, catalogError } = useSession();
  const { t } = useI18n();

  const isSessionPage = path in SESSION_PAGES;
  useEffect(() => {
    if (isSessionPage && status === 'none') navigate('/start', { replace: true });
  }, [isSessionPage, status, navigate]);

  if (status === 'ended' && (isSessionPage || path === '/')) {
    return (
      <Layout bare>
        <SessionEnded />
      </Layout>
    );
  }
  const page = PUBLIC_PAGES[path] ?? (isSessionPage ? SESSION_PAGES[path] : undefined);
  let body: ReactNode;
  if (!page) body = <NotFound />;
  else if (isSessionPage && (status === 'loading' || !session)) body = <Spinner label={t('app.loading')} />;
  // Immediate help never waits for the catalog: it has a bundled fallback.
  else if (!catalog && path !== '/help') body = catalogError ? <CatalogUnavailable /> : <Spinner label={t('app.loading')} />;
  else if (isSessionPage && session?.safety.interrupted && !INTERRUPTED_PAGES.has(path)) body = <Interrupted />;
  // A session page is keyed by its session: when the session is replaced
  // (deleted and started again in another tab), nothing the page held for
  // the old one (drafts, a share link, a running step) carries over.
  else body = isSessionPage && session ? <Fragment key={session.createdAt}>{page()}</Fragment> : page();
  return <Layout>{body}</Layout>;
}
