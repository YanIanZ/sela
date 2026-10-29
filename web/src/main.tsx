import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { V3App } from "./v3/App";
import { OperatorApp } from "./operator/OperatorApp";
import { I18nProvider } from "./i18n";
import { RouterProvider } from "./lib/router";

// The operator console is a separate trust domain with its own layout and
// CSS; it never mounts the consumer shell or session provider.
const operator =
  window.location.pathname === "/operator" ||
  window.location.pathname.startsWith("/operator/");

const root = document.getElementById("root");
if (root) {
  createRoot(root).render(
    <StrictMode>
      <I18nProvider>
        {operator ? (
          <OperatorApp />
        ) : (
          <RouterProvider>
            <V3App />
          </RouterProvider>
        )}
      </I18nProvider>
    </StrictMode>,
  );
}
