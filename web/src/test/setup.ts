import '@testing-library/jest-dom/vitest';

// jsdom has no layout: the router's scroll-to-top is a no-op in tests.
window.scrollTo = () => undefined;
