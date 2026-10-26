import '@testing-library/jest-dom/vitest';
import { render, screen, within } from '@testing-library/react';
import { I18nProvider } from '../i18n';
import { CatalogProvider } from '../lib/catalog';
import { ProjectionView, itemsProvenance, openingIsUsers, openingOrigin } from '../components/Projection';
import { QualifierChips } from '../components/ui';
import type { Projection } from '../lib/types';
import { App } from '../App';
import { Providers, approvedSession, catalogFixture, mockApi, setPath } from './helpers';

const base: Projection = {
  synthetic: true,
  language: 'en',
  briefVersion: 2,
  items: [{ kind: 'feeling', text: 'I feel on edge.', qualifiers: { negated: false, subject: 'self', time: 'present', certainty: 'certain' } }],
  opening: ['I would like to talk about this.', 'I would like to know the next step.'],
  slotId: 's1',
  provenance: 'manual',
};

function show(p: Projection) {
  return render(
    <I18nProvider initial="en">
      <CatalogProvider initial={catalogFixture()}>
        <ProjectionView p={p} />
      </CatalogProvider>
    </I18nProvider>,
  );
}

describe('ProjectionView opening provenance', () => {
  it('labels test-double opening lines as a draft, never as the user own lines', () => {
    show({ ...base, openingProvenance: 'test_double', openingUserEdited: false });
    expect(screen.queryByRole('heading', { name: /The user's own opening lines/ })).toBeNull();
    const heading = screen.getByRole('heading', { name: /Suggested opening lines the user approved/ });
    // The badge right after the heading names the test double.
    expect(heading.nextElementSibling).toHaveTextContent('TEST DOUBLE — not Meta');
    // The items keep their own (manual) provenance.
    expect(screen.getByText('Manual, no AI')).toBeInTheDocument();
  });
  it('titles only lines the user wrote or edited as the user own', () => {
    const { unmount } = show({ ...base, openingProvenance: 'manual', openingUserEdited: true });
    const heading = screen.getByRole('heading', { name: "The user's own opening lines" });
    expect(within(heading.nextElementSibling as HTMLElement).getByText('Manual, no AI')).toBeInTheDocument();
    unmount();
    expect(openingIsUsers({ ...base, openingProvenance: 'live_meta', openingUserEdited: false })).toBe(false);
    expect(openingIsUsers({ ...base, openingProvenance: 'live_meta', openingUserEdited: true })).toBe(true);
    expect(openingIsUsers({ ...base, openingProvenance: 'manual', openingUserEdited: true })).toBe(true);
    // "Draft without AI" (manual, not edited) is Sela's template, not the user's writing.
    expect(openingIsUsers({ ...base, openingProvenance: 'manual', openingUserEdited: false })).toBe(false);
    expect(openingIsUsers({ ...base, openingProvenance: 'manual' })).toBe(false);
    // An older server without the field never gets the user's-own title.
    expect(openingIsUsers({ ...base })).toBe(false);
  });

  it('labels a no-AI template over AI-drafted items as Sela-built, with the items provenance', () => {
    show({ ...base, provenance: 'test_double', openingProvenance: 'manual', openingUserEdited: false });
    expect(screen.queryByRole('heading', { name: "The user's own opening lines" })).toBeNull();
    const heading = screen.getByRole('heading', { name: /Opening lines Sela built from the wording of the approved note/ });
    // The template strings together the test double's wording: never "Manual, no AI".
    expect(heading.nextElementSibling).toHaveTextContent('TEST DOUBLE — not Meta');
    expect(screen.queryByText('Manual, no AI')).toBeNull();
  });

  it('keeps "Manual, no AI" on a template over hand-made items', () => {
    show({ ...base, provenance: 'manual', openingProvenance: 'manual', openingUserEdited: false });
    const heading = screen.getByRole('heading', { name: /Opening lines Sela built from the wording of the approved note/ });
    expect(within(heading.nextElementSibling as HTMLElement).getByText('Manual, no AI')).toBeInTheDocument();
  });
});

describe('opening origin helpers', () => {
  it('derive the wording provenance from the items', () => {
    expect(itemsProvenance([])).toBe('manual');
    expect(itemsProvenance(['manual', 'test_double', 'manual'])).toBe('test_double');
    expect(openingOrigin('manual', false, 'live_meta')).toEqual({ origin: 'template', provenance: 'live_meta' });
    expect(openingOrigin('test_double', false, 'manual')).toEqual({ origin: 'draft', provenance: 'test_double' });
    expect(openingOrigin('manual', true, 'test_double')).toEqual({ origin: 'user', provenance: 'manual' });
  });
});

describe('QualifierChips', () => {
  it('names the chip list with its own label, not the kind selector label', () => {
    render(
      <I18nProvider initial="en">
        <QualifierChips q={{ negated: true, subject: 'self', time: 'present', certainty: 'certain' }} />
      </I18nProvider>,
    );
    expect(screen.getByRole('list', { name: 'Meaning notes' })).toBeInTheDocument();
    expect(screen.queryByRole('list', { name: 'Label' })).toBeNull();
  });
});

describe('Practice opening badge', () => {
  afterEach(() => vi.unstubAllGlobals());
  it('a "Draft without AI" opening over test-double items is not badged "Manual, no AI"', async () => {
    setPath('/practice');
    const s = approvedSession();
    const items = s.ledger!.items.map((i) => ({ ...i, provenance: 'test_double' as const }));
    const view = {
      ...s,
      ledger: { ...s.ledger!, items, provenance: 'test_double' as const },
      opening: { lines: [{ text: 'I would like to talk about this: I feel on edge.', basedOn: ['intent', 'i1'] }], provenance: 'manual' as const, briefVersion: 2, userEdited: false },
    };
    mockApi({ 'GET /session': () => ({ data: view }), 'GET /ai/status': () => ({ data: view.ai }) });
    render(
      <Providers>
        <App />
      </Providers>,
    );
    const card = await screen.findByRole('region', { name: 'Opening lines' });
    expect(within(card).getByText('TEST DOUBLE — not Meta, for testing only')).toBeInTheDocument();
    expect(within(card).queryByText('Manual, no AI')).toBeNull();
  });
});
