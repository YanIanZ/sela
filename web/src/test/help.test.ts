import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { telHref } from '../pages/Help';

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

// A tel: link must dial exactly the number shown. "119 ext 8" once became
// tel:1198, a different number.
describe('help phone links', () => {
  it('dial the main number and keep an extension out of the link', () => {
    expect(telHref('119 ext 8')).toBe('tel:119');
    expect(telHref('119 ext. 8')).toBe('tel:119');
    expect(telHref('119 ekstensi 8')).toBe('tel:119');
    expect(telHref('021-7884-2580')).toBe('tel:02178842580');
    expect(telHref('+62 21 7884 2580')).toBe('tel:+622178842580');
  });
});
