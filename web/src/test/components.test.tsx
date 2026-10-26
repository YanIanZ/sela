import { render, screen } from '@testing-library/react';
import { I18nProvider } from '../i18n';
import { ErrorNotice, ProvenanceBadge, QualifierChips } from '../components/ui';
import { ApiError } from '../lib/api';

describe('components', () => {
  it('ErrorNotice localizes stable codes in both languages', () => {
    const { unmount } = render(
      <I18nProvider initial="id">
        <ErrorNotice error={new ApiError('stale_version', 409)} />
      </I18nProvider>,
    );
    expect(screen.getByRole('alert')).toHaveTextContent('Data sudah berubah');
    unmount();
    render(
      <I18nProvider initial="en">
        <ErrorNotice code="access_ended" />
      </I18nProvider>,
    );
    expect(screen.getByRole('alert')).toHaveTextContent('Access is not available');
  });
  it('ErrorNotice falls back for unknown codes', () => {
    render(
      <I18nProvider initial="en">
        <ErrorNotice code="totally_unknown" />
      </I18nProvider>,
    );
    expect(screen.getByRole('alert')).toHaveTextContent('Something went wrong');
  });
  it('labels test-double output as not Meta', () => {
    render(
      <I18nProvider initial="en">
        <ProvenanceBadge provenance="test_double" />
      </I18nProvider>,
    );
    expect(screen.getByText(/TEST DOUBLE — not Meta/)).toBeInTheDocument();
  });
  it('shows meaning-bearing qualifiers', () => {
    render(
      <I18nProvider initial="id">
        <QualifierChips q={{ negated: true, subject: 'other', time: 'past', certainty: 'uncertain' }} />
      </I18nProvider>,
    );
    for (const label of ['Kalimat negatif', 'Tentang/menurut orang lain', 'Waktu lampau', 'Belum pasti']) {
      expect(screen.getByText(label)).toBeInTheDocument();
    }
  });
});
