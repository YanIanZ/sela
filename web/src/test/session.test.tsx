import '@testing-library/jest-dom/vitest';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from '../App';
import { I18nProvider } from '../i18n';
import { ConfirmButton } from '../components/ui';
import type { Handoff, SessionView } from '../lib/types';
import { Providers, approvedSession, catalogFixture, mockApi, sessionFixture, setPath } from './helpers';

function renderApp(catalog: Parameters<typeof Providers>[0]['catalog'] = undefined) {
  return render(
    <Providers catalog={catalog}>
      <App />
    </Providers>,
  );
}

function handoffFixture(over: Partial<Handoff> = {}): Handoff {
  const s = approvedSession();
  return {
    id: 'h1',
    version: 1,
    professionalId: 'pro-a',
    slotId: 's1',
    briefVersion: 2,
    fields: ['items'],
    projection: { synthetic: true, language: 'en', briefVersion: 2, items: [], slotId: 's1', provenance: 'manual' },
    status: 'requested',
    share: 'active',
    createdAt: s.createdAt,
    expiresAt: s.expiresAt,
    timeline: [],
    opened: false,
    careReceived: false,
    ...over,
  };
}

const interrupted = (s: SessionView): SessionView => ({ ...s, version: s.version + 1, safety: { level: 'crisis', interrupted: true }, stage: 'crisis', story: undefined, ledger: undefined });

afterEach(() => vi.unstubAllGlobals());

describe('owner session view', () => {
  it('reloads the session on in-app navigation, so recipient actions show without a page reload', async () => {
    setPath('/settings');
    let current = approvedSession({ handoffs: [handoffFixture()] });
    const api = mockApi({
      'GET /session': () => ({ data: current }),
      'GET /ai/status': () => ({ data: current.ai }),
    });
    renderApp();
    await screen.findByRole('heading', { name: 'Settings' });
    current = approvedSession({ version: 9, handoffs: [handoffFixture({ status: 'acknowledged', opened: true, version: 2 })] });
    fireEvent.click(screen.getByRole('link', { name: 'My journey' }));
    expect(await screen.findByText('Acknowledged by the recipient (simulated)')).toBeInTheDocument();
    expect(screen.getByText('The recipient has opened the note')).toBeInTheDocument();
    expect(api.count('GET /session')).toBeGreaterThanOrEqual(2);
  });

  it('a session replaced in another tab replaces the view on refresh, even at a lower version', async () => {
    setPath('/ledger');
    let current = approvedSession({ version: 7, createdAt: '2026-10-26T07:23:46.376143981Z' });
    const api = mockApi({
      'GET /session': () => ({ data: current }),
      'GET /ai/status': () => ({ data: current.ai }),
    });
    renderApp();
    expect(await screen.findByRole('button', { name: 'Remove' })).toBeInTheDocument();
    // Tab B deleted everything and started again: a new session at version 1.
    current = sessionFixture({ version: 1, createdAt: '2026-10-26T07:23:48.240099501Z', stage: 'consent' });
    const before = api.count('GET /session');
    await act(async () => {
      window.dispatchEvent(new Event('focus'));
    });
    await waitFor(() => expect(api.count('GET /session')).toBeGreaterThan(before));
    expect(await screen.findByText('There is no story yet. Choose or write one first.')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Remove' })).toBeNull();
    expect(screen.queryByText('I feel on edge.')).toBeNull();
  });

  it('nothing a page held for a replaced session carries over to the new one', async () => {
    setPath('/story');
    let current = sessionFixture({ version: 4, createdAt: '2026-10-26T07:23:46.376143981Z', stage: 'story' });
    const api = mockApi({
      'GET /session': () => ({ data: current }),
      'GET /ai/status': () => ({ data: current.ai }),
    });
    renderApp();
    fireEvent.change(await screen.findByLabelText('Your story (practice)'), { target: { value: 'A draft typed for the old session.' } });
    current = sessionFixture({ version: 1, createdAt: '2026-10-26T07:23:48.240099501Z', stage: 'consent' });
    const before = api.count('GET /session');
    await act(async () => {
      window.dispatchEvent(new Event('focus'));
    });
    await waitFor(() => expect(api.count('GET /session')).toBeGreaterThan(before));
    await waitFor(() => expect(screen.getByLabelText('Your story (practice)')).toHaveValue(''));
  });

  it('a stale version caused by a replaced session loads the new session instead of failing for good', async () => {
    setPath('/ledger');
    let current = approvedSession({ version: 7, createdAt: '2026-10-26T07:23:46.376143981Z' });
    const api = mockApi({
      'GET /session': () => ({ data: current }),
      'GET /ai/status': () => ({ data: current.ai }),
      'PATCH /ledger/items/i1': () => ({ status: 409, error: 'stale_version' }),
    });
    renderApp();
    const remove = await screen.findByRole('button', { name: 'Remove' });
    current = sessionFixture({ version: 1, createdAt: '2026-10-26T07:23:48.240099501Z', stage: 'consent' });
    fireEvent.click(remove);
    expect(await screen.findByText('There is no story yet. Choose or write one first.')).toBeInTheDocument();
    expect(api.count('PATCH /ledger/items/i1')).toBe(1);
  });

  it('still drops an older view of the same session that answers after a newer one', async () => {
    setPath('/ledger');
    const createdAt = '2026-10-26T07:23:46.376143981Z';
    let current = approvedSession({ version: 7, createdAt });
    const api = mockApi({
      'GET /session': () => ({ data: current }),
      'GET /ai/status': () => ({ data: current.ai }),
    });
    renderApp();
    expect(await screen.findByRole('button', { name: 'Remove' })).toBeInTheDocument();
    current = sessionFixture({ version: 5, createdAt, stage: 'consent' });
    const before = api.count('GET /session');
    await act(async () => {
      window.dispatchEvent(new Event('focus'));
    });
    await waitFor(() => expect(api.count('GET /session')).toBeGreaterThan(before));
    expect(screen.getByRole('button', { name: 'Remove' })).toBeInTheDocument();
    expect(screen.queryByText('There is no story yet. Choose or write one first.')).toBeNull();
  });

  it('a crisis from a ledger wording edit routes to Immediate help', async () => {
    setPath('/ledger');
    const s = approvedSession();
    mockApi({
      'GET /session': () => ({ data: s }),
      'GET /ai/status': () => ({ data: s.ai }),
      'PATCH /ledger/items/i1': () => ({ data: interrupted(s) }),
    });
    renderApp();
    fireEvent.click(await screen.findByRole('button', { name: 'Change wording' }));
    fireEvent.change(screen.getByLabelText('Your wording for this item'), { target: { value: 'crisis wording' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(window.location.pathname).toBe('/help'));
    expect(await screen.findByRole('heading', { level: 1, name: 'Immediate help' })).toBeInTheDocument();
  });

  it('shows the interrupted state instead of journey controls on an interrupted session', async () => {
    setPath('/story');
    const s = interrupted(sessionFixture());
    mockApi({ 'GET /session': () => ({ data: s }), 'GET /ai/status': () => ({ data: s.ai }) });
    renderApp();
    expect(await screen.findByRole('heading', { name: 'The ordinary journey has stopped for this session' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Settings: delete the session and start again' })).toHaveAttribute('href', '/settings');
    expect(screen.queryByRole('button', { name: /Use this story/ })).toBeNull();
    expect(screen.queryByLabelText(/Your story/)).toBeNull();
    // The navigation keeps only what stays usable.
    expect(screen.queryByRole('link', { name: 'My note' })).toBeNull();
  });

  it('a crisis pseudonym at session creation goes to Immediate help, not the consent step', async () => {
    setPath('/start');
    mockApi({
      'GET /session': () => ({ status: 401, error: 'session_missing' }),
      'GET /ai/status': () => ({ data: sessionFixture().ai }),
      'POST /sessions': () => ({ data: { ...interrupted(sessionFixture()), version: 1 } }),
    });
    renderApp();
    fireEvent.click(await screen.findByLabelText(/I am 18 or older/));
    fireEvent.click(screen.getByRole('button', { name: 'I understand, continue' }));
    await waitFor(() => expect(window.location.pathname).toBe('/help'));
  });

  it('a crisis pseudonym in Settings goes to Immediate help', async () => {
    setPath('/settings');
    const s = sessionFixture();
    mockApi({
      'GET /session': () => ({ data: s }),
      'GET /ai/status': () => ({ data: s.ai }),
      'PATCH /session': () => ({ data: interrupted(s) }),
    });
    renderApp();
    fireEvent.click(await screen.findByLabelText(/Pseudonym/, { selector: 'input[type=radio]' }));
    fireEvent.change(screen.getByLabelText('Pseudonym', { selector: 'input:not([type=radio])' }), { target: { value: 'crisis name' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(window.location.pathname).toBe('/help'));
  });

  it('keeps the chosen language when the first save meets a stale version', async () => {
    setPath('/settings');
    let s = sessionFixture({ language: 'en', version: 3 });
    let patches = 0;
    const api = mockApi({
      'GET /session': () => ({ data: s }),
      'GET /ai/status': () => ({ data: s.ai }),
      'PATCH /session': (body) => {
        patches++;
        const b = body as { expectedVersion: number; language: 'id' | 'en' };
        if (patches === 1) {
          s = { ...s, version: 4 }; // the recipient acted meanwhile
          return { status: 409, error: 'stale_version' };
        }
        expect(b.expectedVersion).toBe(4);
        s = { ...s, version: 5, language: b.language };
        return { data: s };
      },
    });
    renderApp();
    await screen.findByRole('heading', { name: 'Settings' });
    fireEvent.click(screen.getByRole('button', { name: 'Indonesia' }));
    await waitFor(() => expect(api.count('PATCH /session')).toBe(2));
    await waitFor(() => expect(document.documentElement.lang).toBe('id'));
    expect(await screen.findByRole('heading', { name: 'Pengaturan' })).toBeInTheDocument();
  });

  it('never reverts a language choice that could not be saved when the view reloads', async () => {
    setPath('/settings');
    const s = sessionFixture({ language: 'en', version: 3 });
    const api = mockApi({
      'GET /session': () => ({ data: s }),
      'GET /ai/status': () => ({ data: s.ai }),
      'PATCH /session': () => ({ status: 500, error: 'internal' }),
    });
    renderApp();
    await screen.findByRole('heading', { name: 'Settings' });
    fireEvent.click(screen.getByRole('button', { name: 'Indonesia' }));
    expect(await screen.findByText(/Bahasa hanya berubah di perangkat ini/)).toBeInTheDocument();
    // A background reload (navigation) keeps the visitor's choice.
    const before = api.count('GET /session');
    fireEvent.click(screen.getByRole('link', { name: 'Perjalananku' }));
    await waitFor(() => expect(api.count('GET /session')).toBeGreaterThan(before));
    expect(await screen.findByRole('heading', { level: 1, name: 'Perjalananku' })).toBeInTheDocument();
    expect(document.documentElement.lang).toBe('id');
  });
});

describe('catalog failures', () => {
  it('Immediate help shows the bundled verified list when the catalog fails, with a retry', async () => {
    setPath('/help');
    const api = mockApi({
      'GET /catalog': () => ({ status: 503, error: 'capacity' }),
      'GET /session': () => ({ status: 401, error: 'session_missing' }),
      'GET /ai/status': () => ({ status: 503, error: 'capacity' }),
    });
    renderApp(null);
    expect(await screen.findByRole('heading', { name: 'Healing119 (Kementerian Kesehatan RI)' })).toBeInTheDocument();
    // The link dials exactly the number shown; the extension stays text (119 ext 8 once dialled 1198).
    expect(screen.getByRole('link', { name: '119 ext 8' })).toHaveAttribute('href', 'tel:119');
    const before = api.count('GET /catalog');
    fireEvent.click(screen.getByRole('button', { name: 'Try again' }));
    await waitFor(() => expect(api.count('GET /catalog')).toBe(before + 1));
  });

  it('other pages show the failure and a retry instead of loading forever', async () => {
    setPath('/privacy');
    let fail = true;
    const api = mockApi({
      'GET /catalog': () => (fail ? { status: 503, error: 'capacity' } : { data: catalogFixture() }),
      'GET /session': () => ({ status: 401, error: 'session_missing' }),
      'GET /ai/status': () => ({ status: 503, error: 'capacity' }),
    });
    renderApp(null);
    expect(await screen.findByRole('heading', { name: 'Sela could not load the demo content' })).toBeInTheDocument();
    expect(screen.getByRole('alert')).toHaveTextContent('The demo is full');
    fail = false;
    fireEvent.click(screen.getByRole('button', { name: 'Try again' }));
    await waitFor(() => expect(api.count('GET /catalog')).toBe(2));
    expect(await screen.findByRole('heading', { level: 1, name: 'Privacy and service boundaries' })).toBeInTheDocument();
  });
});

describe('ConfirmButton', () => {
  it('returns focus to the trigger on Cancel and after confirming', async () => {
    const onConfirm = vi.fn();
    render(
      <I18nProvider initial="en">
        <ConfirmButton label="Delete all data now" confirmText="Sure?" onConfirm={onConfirm} />
      </I18nProvider>,
    );
    fireEvent.click(screen.getByRole('button', { name: 'Delete all data now' }));
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    await waitFor(() => expect(screen.getByRole('button', { name: 'Delete all data now' })).toHaveFocus());
    fireEvent.click(screen.getByRole('button', { name: 'Delete all data now' }));
    await act(async () => {
      fireEvent.click(screen.getAllByRole('button', { name: 'Delete all data now' })[0]!);
    });
    expect(onConfirm).toHaveBeenCalledTimes(1);
    await waitFor(() => expect(screen.getByRole('button', { name: 'Delete all data now' })).toHaveFocus());
  });
});
