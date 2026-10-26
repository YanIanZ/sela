import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { App } from './App';
import { I18nProvider } from './i18n';
import { RouterProvider } from './lib/router';
import { SessionProvider } from './lib/session';
import './styles.css';

const root = document.getElementById('root');
if (root) {
  createRoot(root).render(
    <StrictMode>
      <I18nProvider>
        <RouterProvider>
          <SessionProvider>
            <App />
          </SessionProvider>
        </RouterProvider>
      </I18nProvider>
    </StrictMode>,
  );
}
