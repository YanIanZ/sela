import '@testing-library/jest-dom/vitest';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { Root } from '../App';
import type { RecipientHandoff } from '../lib/types';
import { Providers, catalogFixture, mockApi, setPath } from './helpers';

const TOKEN1 = 'AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA1';
const TOKEN2 = 'BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB2';

function handoff(id: string): RecipientHandoff {
  return {
    id,
    version: 1,
    professionalId: 'pro-a',
    status: 'requested',
    projection: { synthetic: true, language: 'en', briefVersion: 2, items: [], slotId: 's1', provenance: 'manual' },
    expiresAt: new Date(Date.now() + 3600_000).toISOString(),
    timeline: [],
    plans: [],
    synthetic: true,
  };
}

function renderRoot() {
  return render(
    <Providers session={false}>
      <Root />
    </Providers>,
  );
}

afterEach(() => vi.unstubAllGlobals());

describe('recipient perspective', () => {
  it('asks for the code of the link it was opened with, even when signed in for another share', async () => {
    setPath(`/r#t=${TOKEN2}`);
    const api = mockApi({
      'GET /recipient/handoff': () => ({ data: handoff('h-old') }),
      'DELETE /recipient/session': () => ({ data: null }),
    });
    renderRoot();
    expect(await screen.findByRole('heading', { name: 'Enter the access code' })).toBeInTheDocument();
    expect(window.location.hash).toBe('');
    expect(api.count('DELETE /recipient/session')).toBeGreaterThan(0);
    expect(screen.queryByText(/Request for/)).toBeNull();
  });

  it('treats a second link pasted into the tab as a new link and signs in with its token', async () => {
    setPath(`/r#t=${TOKEN1}`);
    const api = mockApi({
      'DELETE /recipient/session': () => ({ data: null }),
      'GET /recipient/handoff': () => ({ error: 'access_ended', status: 403 }),
      'POST /recipient/sessions': (body) => ({ data: handoff((body as { token: string }).token === TOKEN1 ? 'h1' : 'h2') }),
    });
    renderRoot();
    fireEvent.change(await screen.findByLabelText('Access code'), { target: { value: 'AAAA-AAAA' } });
    fireEvent.click(screen.getByRole('button', { name: 'Open the note' }));
    expect(await screen.findByText(/Request for Fictional Counsellor A/)).toBeInTheDocument();

    // Paste link 2 into the same tab: a same-document fragment navigation.
    await act(async () => {
      window.history.pushState(null, '', `/r#t=${TOKEN2}`);
      window.dispatchEvent(new HashChangeEvent('hashchange'));
    });
    expect(await screen.findByRole('heading', { name: 'Enter the access code' })).toBeInTheDocument();
    expect(window.location.hash).toBe('');
    fireEvent.change(screen.getByLabelText('Access code'), { target: { value: 'BBBB-BBBB' } });
    fireEvent.click(screen.getByRole('button', { name: 'Open the note' }));
    await waitFor(() => expect(api.calls.filter((c) => c.path === '/recipient/sessions').length).toBe(2));
    const second = api.calls.filter((c) => c.path === '/recipient/sessions')[1]!;
    expect((second.body as { token: string }).token).toBe(TOKEN2);
  });

  it('renders outside the owner session: no session request, no expiry, language switch stays local', async () => {
    setPath(`/r#t=${TOKEN1}`);
    const api = mockApi({
      'GET /session': () => ({ data: null }),
      'DELETE /recipient/session': () => ({ data: null }),
    });
    renderRoot();
    await screen.findByRole('heading', { name: 'Enter the access code' });
    fireEvent.click(screen.getByRole('button', { name: 'Indonesia' }));
    expect(await screen.findByRole('heading', { name: 'Masukkan kode akses' })).toBeInTheDocument();
    expect(api.count('GET /session')).toBe(0);
    expect(api.count('PATCH /session')).toBe(0);
    expect(screen.queryByText(/Sesi berakhir pada/)).toBeNull();
  });
});

describe('recipient perspective without the catalog', () => {
  it('says the content could not be loaded and offers a retry instead of a blank page', async () => {
    setPath(`/r#t=${TOKEN1}`);
    let fail = true;
    const api = mockApi({
      'GET /catalog': () => (fail ? { status: 503, error: 'capacity' } : { data: catalogFixture() }),
      'DELETE /recipient/session': () => ({ data: null }),
      'POST /recipient/sessions': () => ({ data: handoff('h1') }),
    });
    render(
      <Providers session={false} catalog={null}>
        <Root />
      </Providers>,
    );
    fireEvent.change(await screen.findByLabelText('Access code'), { target: { value: 'AAAA-AAAA' } });
    fireEvent.click(screen.getByRole('button', { name: 'Open the note' }));
    expect(await screen.findByRole('heading', { name: 'Sela could not load the demo content' })).toBeInTheDocument();
    expect(screen.getByRole('alert')).toHaveTextContent('The demo is full');
    expect(screen.getByRole('button', { name: 'Leave recipient mode' })).toBeInTheDocument();
    fail = false;
    const before = api.count('GET /catalog');
    fireEvent.click(screen.getByRole('button', { name: 'Try again' }));
    await waitFor(() => expect(api.count('GET /catalog')).toBe(before + 1));
    expect(await screen.findByText(/Request for Fictional Counsellor A/)).toBeInTheDocument();
  });
});
