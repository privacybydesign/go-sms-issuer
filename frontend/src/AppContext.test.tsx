import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen, act } from '@testing-library/react';
import { AppProvider, useAppContext } from './AppContext';

function PhoneNumberProbe() {
  const { phoneNumber, setPhoneNumber } = useAppContext();
  return (
    <div>
      <span data-testid="phone">{phoneNumber ?? ''}</span>
      <button onClick={() => setPhoneNumber('+31612345678')}>set</button>
      <button onClick={() => setPhoneNumber('')}>clear</button>
    </div>
  );
}

describe('AppContext phone number persistence', () => {
  beforeEach(() => {
    sessionStorage.clear();
  });

  it('persists the phone number to sessionStorage', () => {
    render(
      <AppProvider>
        <PhoneNumberProbe />
      </AppProvider>
    );

    act(() => {
      screen.getByText('set').click();
    });

    expect(screen.getByTestId('phone').textContent).toBe('+31612345678');
    expect(sessionStorage.getItem('phoneNumber')).toBe('+31612345678');
  });

  it('restores the phone number from sessionStorage on mount (e.g. after a refresh)', () => {
    sessionStorage.setItem('phoneNumber', '+31612345678');

    render(
      <AppProvider>
        <PhoneNumberProbe />
      </AppProvider>
    );

    expect(screen.getByTestId('phone').textContent).toBe('+31612345678');
  });

  it('clears the stored phone number when set to an empty value', () => {
    sessionStorage.setItem('phoneNumber', '+31612345678');

    render(
      <AppProvider>
        <PhoneNumberProbe />
      </AppProvider>
    );

    act(() => {
      screen.getByText('clear').click();
    });

    expect(screen.getByTestId('phone').textContent).toBe('');
    expect(sessionStorage.getItem('phoneNumber')).toBeNull();
  });
});
