import { render, screen, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { BrowserRouter } from 'react-router-dom';
import Enroll from './Enroll';
import { I18nextProvider } from 'react-i18next';
import i18n from '../i18n';

// Mock the @privacybydesign/yivi-frontend module
vi.mock('@privacybydesign/yivi-frontend', () => ({
  newPopup: vi.fn(() => ({
    start: vi.fn(() => Promise.resolve()),
  })),
}));

const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

// Mock the AppContext module
const mockSetPhoneNumber = vi.fn();
let mockPhoneNumber = '+31612345678';

vi.mock('../AppContext', () => ({
  useAppContext: () => ({
    phoneNumber: mockPhoneNumber,
    setPhoneNumber: mockSetPhoneNumber,
  }),
}));

const renderEnrollPage = (phoneNumber: string = '+31612345678') => {
  mockPhoneNumber = phoneNumber;

  return render(
    <I18nextProvider i18n={i18n}>
      <BrowserRouter>
        <Enroll />
      </BrowserRouter>
    </I18nextProvider>
  );
};

describe('Enroll Page - Error Handling for 401 cannot-validate-token', () => {
  beforeEach(async () => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
    window.location.hash = '';
    // Set language to English for tests
    await i18n.changeLanguage('en');
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('should display error message when deeplink has wrong token (401 cannot-validate-token)', async () => {
    // Set the URL hash to simulate SMS deeplink with wrong token
    window.location.hash = '#!verify:+31612345678:WRONG1';

    // Mock fetch to return 401 error from backend
    (global.fetch as any).mockResolvedValueOnce({
      ok: false,
      status: 401,
      text: async () => 'error:cannot-validate-token',
    });

    renderEnrollPage();

    // Wait for fetch to be called automatically (useEffect triggers on hash change)
    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith(
        '/verify',
        expect.objectContaining({
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: expect.stringContaining('WRONG1'),
        })
      );
    }, { timeout: 3000 });

    // Wait for error message to appear
    await waitFor(
      () => {
        const errorMessage = screen.getByText(/code cannot be verified/i);
        expect(errorMessage).toBeInTheDocument();
      },
      { timeout: 3000 }
    );

    // Verify error is displayed in alert-danger div
    const errorAlert = screen.getByRole('alert');
    expect(errorAlert).toHaveClass('alert-danger');

    // Should not navigate away from the page
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it('should display Dutch error message when deeplink has wrong token', async () => {
    // Change language to Dutch
    await i18n.changeLanguage('nl');

    // Set the URL hash to simulate SMS deeplink
    window.location.hash = '#!verify:+31612345678:WRONG2';

    // Mock fetch to return 401 error
    (global.fetch as any).mockResolvedValueOnce({
      ok: false,
      status: 401,
      text: async () => 'error:cannot-validate-token',
    });

    renderEnrollPage();

    // Wait for fetch to be called
    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalled();
    }, { timeout: 3000 });

    // Wait for Dutch error message to appear
    await waitFor(
      () => {
        const errorMessage = screen.getByText(/code kon niet worden geverifieerd/i);
        expect(errorMessage).toBeInTheDocument();
      },
      { timeout: 3000 }
    );
  });

  it('should properly convert backend error code format from colons/dashes to underscores', async () => {
    // This test verifies that "error:cannot-validate-token" gets converted to "error_cannot_validate_token"
    window.location.hash = '#!verify:+31612345678:ABC123';

    // Mock fetch to return error with backend format (colons and dashes)
    (global.fetch as any).mockResolvedValueOnce({
      ok: false,
      status: 401,
      text: async () => 'error:cannot-validate-token', // Backend sends this format
    });

    renderEnrollPage();

    // If the error code wasn't converted properly, the translation wouldn't work
    // and we wouldn't see the correct error message
    await waitFor(
      () => {
        const errorMessage = screen.getByText(/code cannot be verified/i);
        expect(errorMessage).toBeInTheDocument();
      },
      { timeout: 3000 }
    );
  });

  it('should navigate to error page when backend returns empty error code', async () => {
    window.location.hash = '#!verify:+31612345678:ABC123';

    // Mock fetch to return error with empty body
    (global.fetch as any).mockResolvedValueOnce({
      ok: false,
      status: 500,
      text: async () => '', // Empty error code
    });

    renderEnrollPage();

    // Should navigate to generic error page when no specific error code is provided
    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/en/error');
    }, { timeout: 3000 });
  });

  it('should navigate to error page when network error occurs', async () => {
    window.location.hash = '#!verify:+31612345678:ABC123';

    // Mock fetch to throw a network error
    (global.fetch as any).mockRejectedValueOnce(new Error('Network error'));

    renderEnrollPage();

    // Should navigate to generic error page on network failure
    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/en/error');
    }, { timeout: 3000 });
  });

  it('should handle other backend error codes correctly', async () => {
    window.location.hash = '#!verify:+31612345678:ABC123';

    // Mock fetch to return a different error type
    (global.fetch as any).mockResolvedValueOnce({
      ok: false,
      status: 400,
      text: async () => 'error:phone-number-format',
    });

    renderEnrollPage();

    // Wait for error alert to appear
    await waitFor(
      () => {
        const errorDiv = screen.getByRole('alert');
        expect(errorDiv).toBeInTheDocument();
        expect(errorDiv).toHaveClass('alert-danger');
        // This error message exists in translations so it should display
        expect(screen.getByText(/did not enter a valid telephone number|geen geldig telefoonnummer/i)).toBeInTheDocument();
      },
      { timeout: 3000 }
    );
  });
});
