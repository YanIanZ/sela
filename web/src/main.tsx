import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { V3App } from "./v3/App";
import { I18nProvider } from "./i18n";
import { RouterProvider } from "./lib/router";

const root = document.getElementById("root");
if (root) {
  createRoot(root).render(
    <StrictMode>
      <I18nProvider>
        <RouterProvider>
          <V3App />
        </RouterProvider>
      </I18nProvider>
    </StrictMode>,
  );
}
