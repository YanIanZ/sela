import { useEffect, type ReactNode } from 'react';
import { useRouter } from './lib/router';
import { useSession } from './lib/session';
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
import { NotFound, SessionEnded } from './pages/Misc';

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

const PUBLIC_PAGES: Record<string, () => ReactNode> = {
  '/': () => <Welcome />,
  '/start': () => <Start />,
  '/privacy': () => <PrivacyPage />,
  '/help': () => <HelpPage />,
  '/research': () => <ResearchPage />,
};

export function App() {
  const { path, navigate } = useRouter();
  const { status, session, catalog } = useSession();
  const { t } = useI18n();

  const isSessionPage = path in SESSION_PAGES;
  useEffect(() => {
    if (isSessionPage && status === 'none') navigate('/start', { replace: true });
  }, [isSessionPage, status, navigate]);

  if (path === '/r') {
    return (
      <Layout bare>
        <RecipientPage />
      </Layout>
    );
  }
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
  else if ((isSessionPage && (status === 'loading' || !session)) || !catalog) body = <Spinner label={t('app.loading')} />;
  else body = page();
  return <Layout>{body}</Layout>;
}
