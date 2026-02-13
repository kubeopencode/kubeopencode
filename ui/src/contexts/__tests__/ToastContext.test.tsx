import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, act } from '@testing-library/react';
import { ToastProvider, useToast } from '../ToastContext';

// Test component that exposes toast actions
function TestConsumer() {
  const { toasts, addToast, removeToast } = useToast();
  return (
    <div>
      <button onClick={() => addToast('Success message', 'success')}>Add Success</button>
      <button onClick={() => addToast('Error message', 'error')}>Add Error</button>
      <button onClick={() => addToast('Info message', 'info')}>Add Info</button>
      {toasts.map((t) => (
        <div key={t.id} data-testid={`toast-${t.id}`}>
          <span>{t.message}</span>
          <span data-testid={`type-${t.id}`}>{t.type}</span>
          <button onClick={() => removeToast(t.id)}>Remove {t.id}</button>
        </div>
      ))}
    </div>
  );
}

describe('ToastContext', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('starts with no toasts', () => {
    render(
      <ToastProvider>
        <TestConsumer />
      </ToastProvider>
    );

    expect(screen.queryByText('Success message')).not.toBeInTheDocument();
  });

  it('adds a toast when addToast is called', () => {
    render(
      <ToastProvider>
        <TestConsumer />
      </ToastProvider>
    );

    act(() => {
      screen.getByText('Add Success').click();
    });

    expect(screen.getByText('Success message')).toBeInTheDocument();
  });

  it('adds toasts with correct types', () => {
    render(
      <ToastProvider>
        <TestConsumer />
      </ToastProvider>
    );

    act(() => {
      screen.getByText('Add Success').click();
    });

    // Find the type element for the first toast
    const typeElements = screen.getAllByTestId(/^type-/);
    expect(typeElements[0].textContent).toBe('success');
  });

  it('removes a toast when removeToast is called', () => {
    render(
      <ToastProvider>
        <TestConsumer />
      </ToastProvider>
    );

    act(() => {
      screen.getByText('Add Success').click();
    });

    expect(screen.getByText('Success message')).toBeInTheDocument();

    // Find and click the remove button
    const removeButtons = screen.getAllByText(/^Remove /);
    act(() => {
      removeButtons[0].click();
    });

    expect(screen.queryByText('Success message')).not.toBeInTheDocument();
  });

  it('supports multiple toasts simultaneously', () => {
    render(
      <ToastProvider>
        <TestConsumer />
      </ToastProvider>
    );

    act(() => {
      screen.getByText('Add Success').click();
      screen.getByText('Add Error').click();
      screen.getByText('Add Info').click();
    });

    expect(screen.getByText('Success message')).toBeInTheDocument();
    expect(screen.getByText('Error message')).toBeInTheDocument();
    expect(screen.getByText('Info message')).toBeInTheDocument();
  });

  it('auto-removes toasts after 4 seconds', () => {
    render(
      <ToastProvider>
        <TestConsumer />
      </ToastProvider>
    );

    act(() => {
      screen.getByText('Add Success').click();
    });

    expect(screen.getByText('Success message')).toBeInTheDocument();

    // Advance time past the 4000ms auto-dismiss
    act(() => {
      vi.advanceTimersByTime(4100);
    });

    expect(screen.queryByText('Success message')).not.toBeInTheDocument();
  });

  it('each toast gets a unique ID', () => {
    render(
      <ToastProvider>
        <TestConsumer />
      </ToastProvider>
    );

    act(() => {
      screen.getByText('Add Success').click();
      screen.getByText('Add Error').click();
    });

    const toastElements = screen.getAllByTestId(/^toast-/);
    const ids = toastElements.map((el) => el.getAttribute('data-testid'));
    // All IDs should be unique
    expect(new Set(ids).size).toBe(ids.length);
  });
});
