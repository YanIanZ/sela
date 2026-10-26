import { readFileSync } from 'node:fs';
import { join } from 'node:path';

// Immediate help must render even when the catalog call fails, so the page
// bundles its own copy of the verified help resources. The copy may never
// drift from the server's fixture.
describe('bundled help fallback', () => {
  it('is byte-identical to internal/fixtures/data/help.json', () => {
    const server = readFileSync(join(__dirname, '..', '..', '..', 'internal', 'fixtures', 'data', 'help.json'), 'utf8');
    const bundled = readFileSync(join(__dirname, '..', 'data', 'help.fallback.json'), 'utf8');
    expect(bundled).toBe(server);
    const parsed = JSON.parse(bundled) as { help: unknown[]; helpNotice: { id: string; en: string } };
    expect(parsed.help.length).toBeGreaterThan(0);
    expect(parsed.helpNotice.id && parsed.helpNotice.en).toBeTruthy();
  });
});
