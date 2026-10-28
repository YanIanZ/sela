import { StrictMode, Suspense, lazy } from "react";
import { createRoot } from "react-dom/client";
const LegacyRoot = lazy(() =>
  import("./App").then((module) => ({ default: module.Root })),
);
import { V3App } from "./v3/App";
import { I18nProvider } from "./i18n";
import { RouterProvider } from "./lib/router";
import { CatalogProvider } from "./lib/catalog";
import "./styles.css";

const root = document.getElementById("root");
if (root) {
  createRoot(root).render(
    <StrictMode>
      <I18nProvider>
        <RouterProvider>
          {window.location.pathname === "/r" ||
          window.location.pathname === "/demo" ||
          window.location.pathname.startsWith("/demo/") ? (
            <CatalogProvider>
              <Suspense fallback={null}>
                <LegacyRoot />
              </Suspense>
            </CatalogProvider>
          ) : (
            <V3App />
          )}
        </RouterProvider>
      </I18nProvider>
    </StrictMode>,
  );
}
