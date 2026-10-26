import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import ts from 'typescript';

// Every user-visible string must come from the i18n dictionaries. This test
// parses all TSX files and fails on JSX text or user-facing string attributes
// that contain words.
const ROOT = join(__dirname, '..');
const USER_ATTRS = new Set(['aria-label', 'title', 'placeholder', 'alt', 'aria-description']);
const ALLOWED_TEXT = new Set(['—', '✓', '✗', '·', '«', '»', 'doi:']);

function files(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((e) =>
    e.isDirectory() ? (e.name === 'test' ? [] : files(join(dir, e.name))) : e.name.endsWith('.tsx') ? [join(dir, e.name)] : [],
  );
}

describe('no hard-coded product strings', () => {
  it('JSX text and user-facing attributes come from i18n', () => {
    const problems: string[] = [];
    for (const file of files(ROOT)) {
      const src = ts.createSourceFile(file, readFileSync(file, 'utf8'), ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
      const visit = (node: ts.Node) => {
        if (ts.isJsxText(node)) {
          const text = node.getText().trim();
          if (text && /[A-Za-z]{2,}/.test(text) && !ALLOWED_TEXT.has(text)) problems.push(`${file}: text "${text}"`);
        }
        if (ts.isJsxAttribute(node) && node.initializer && ts.isStringLiteral(node.initializer)) {
          const name = node.name.getText();
          if (USER_ATTRS.has(name) && /[A-Za-z]{2,}/.test(node.initializer.text)) problems.push(`${file}: ${name}="${node.initializer.text}"`);
        }
        ts.forEachChild(node, visit);
      };
      visit(src);
    }
    expect(problems).toEqual([]);
  });
});
