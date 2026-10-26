import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { Root } from './App';
import { I18nProvider } from './i18n';
import { RouterProvider } from './lib/router';
import { CatalogProvider } from './lib/catalog';
import './styles.css';

const root = document.getElementById('root');
if (root) {
  createRoot(root).render(
    <StrictMode>
      <I18nProvider>
        <RouterProvider>
          <CatalogProvider>
            <Root />
          </CatalogProvider>
        </RouterProvider>
      </I18nProvider>
    </StrictMode>,
  );
}
