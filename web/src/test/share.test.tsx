import '@testing-library/jest-dom/vitest';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { SharePage } from '../pages/Share';
import type { Projection } from '../lib/types';
import { Providers, approvedSession, deferred, mockApi, setPath, type Reply } from './helpers';

const projection = (fields: string[]): Projection => ({
  synthetic: true,
  language: 'en',
  briefVersion: 2,
  items: [
    {
      kind: 'feeling',
      text: 'I feel on edge.',
      quote: fields.includes('quotes') ? 'QUOTE-SHOWN' : undefined,
      qualifiers: { negated: false, subject: 'self', time: 'present', certainty: 'certain' },
    },
  ],
  slotId: 's1',
  provenance: 'manual',
});

afterEach(() => vi.unstubAllGlobals());

describe('share preview', () => {
  it('never shows a late answer for an earlier field selection, and blocks consent until the preview matches', async () => {
    setPath('/share?pro=pro-a&slot=s1');
    const pending: { fields: string[]; d: ReturnType<typeof deferred<{ data: Projection }>> }[] = [];
    mockApi({
      'GET /session': () => ({ data: approvedSession() }),
      'GET /ai/status': () => ({ data: approvedSession().ai }),
      'POST /handoffs/preview': (body) => {
        const fields = (body as { fields: string[] }).fields;
        const d = deferred<{ data: Projection }>();
        pending.push({ fields, d });
        return d.promise as Reply;
      },
    }, { honorAbort: false });
    render(
      <Providers>
        <SharePage />
      </Providers>,
    );
    const quotes = await screen.findByLabelText('My original words next to each item');
    await waitFor(() => expect(pending.length).toBe(1));
    await act(async () => pending[0]!.d.resolve({ data: projection(pending[0]!.fields) }));
    const consent = screen.getByLabelText(/^I allow Sela to share/);
    expect(consent).toBeEnabled();

    fireEvent.click(quotes); // preview #2: items + quotes (answered late)
    await waitFor(() => expect(pending.length).toBe(2));
    expect(consent).toBeDisabled(); // no preview for the current selection yet
    expect(screen.queryByText('I feel on edge.')).toBeNull();
    fireEvent.click(quotes); // preview #3: items only (answered first)
    await waitFor(() => expect(pending.length).toBe(3));
    await act(async () => pending[2]!.d.resolve({ data: projection(pending[2]!.fields) }));
    await act(async () => pending[1]!.d.resolve({ data: projection(pending[1]!.fields) }));

    expect(quotes).not.toBeChecked();
    expect(screen.queryByText(/QUOTE-SHOWN/)).toBeNull();
    expect(consent).toBeEnabled();
  });
});

describe('share preview failures', () => {
  it('a failed preview can be asked again, and sharing unlocks once it arrives', async () => {
    setPath('/share?pro=pro-a&slot=s1');
    let n = 0;
    const api = mockApi({
      'GET /session': () => ({ data: approvedSession() }),
      'GET /ai/status': () => ({ data: approvedSession().ai }),
      'POST /handoffs/preview': (body) => (++n === 1 ? { status: 429, error: 'rate_limited' } : { data: projection((body as { fields: string[] }).fields) }),
    });
    render(
      <Providers>
        <SharePage />
      </Providers>,
    );
    expect(await screen.findByRole('alert')).toBeInTheDocument();
    expect(screen.getByLabelText(/^I allow Sela to share/)).toBeDisabled();
    fireEvent.click(screen.getByRole('button', { name: 'Try again' }));
    await waitFor(() => expect(api.count('POST /handoffs/preview')).toBe(2));
    expect(await screen.findByText('I feel on edge.')).toBeInTheDocument();
    expect(screen.getByLabelText(/^I allow Sela to share/)).toBeEnabled();
    expect(screen.queryByRole('alert')).toBeNull();
    expect(screen.queryByRole('button', { name: 'Try again' })).toBeNull();
  });

  it('asks again when the connection comes back', async () => {
    setPath('/share?pro=pro-a&slot=s1');
    let n = 0;
    const api = mockApi({
      'GET /session': () => ({ data: approvedSession() }),
      'GET /ai/status': () => ({ data: approvedSession().ai }),
      'POST /handoffs/preview': (body) => (++n === 1 ? { status: 503, error: 'capacity' } : { data: projection((body as { fields: string[] }).fields) }),
    });
    render(
      <Providers>
        <SharePage />
      </Providers>,
    );
    expect(await screen.findByRole('alert')).toBeInTheDocument();
    await act(async () => {
      window.dispatchEvent(new Event('online'));
    });
    await waitFor(() => expect(api.count('POST /handoffs/preview')).toBe(2));
    expect(await screen.findByText('I feel on edge.')).toBeInTheDocument();
    expect(screen.getByLabelText(/^I allow Sela to share/)).toBeEnabled();
  });
});
