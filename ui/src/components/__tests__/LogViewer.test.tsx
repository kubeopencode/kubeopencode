import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, act, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import LogViewer from '../LogViewer';

// Mock EventSource for SSE testing
class MockEventSource {
  url: string;
  onopen: ((ev: Event) => void) | null = null;
  onmessage: ((ev: MessageEvent) => void) | null = null;
  onerror: ((ev: Event) => void) | null = null;
  readyState = 0;
  close = vi.fn();

  static instances: MockEventSource[] = [];

  constructor(url: string) {
    this.url = url;
    MockEventSource.instances.push(this);
  }

  simulateOpen() {
    this.readyState = 1;
    this.onopen?.(new Event('open'));
  }

  simulateMessage(data: object) {
    this.onmessage?.(new MessageEvent('message', { data: JSON.stringify(data) }));
  }

  simulateError() {
    this.readyState = 2;
    this.onerror?.(new Event('error'));
  }
}

describe('LogViewer', () => {
  beforeEach(() => {
    MockEventSource.instances = [];
    vi.stubGlobal('EventSource', MockEventSource);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  const defaultProps = {
    namespace: 'default',
    taskName: 'test-task',
    podName: 'test-pod',
    isRunning: true,
  };

  it('renders the Logs header', () => {
    render(<LogViewer {...defaultProps} />);
    expect(screen.getByText('Logs')).toBeInTheDocument();
  });

  it('shows "Waiting for pod..." when no podName', () => {
    render(<LogViewer {...defaultProps} podName={undefined} />);
    expect(screen.getByText('Waiting for pod...')).toBeInTheDocument();
  });

  it('shows "Pod not yet created" in log area when no podName', () => {
    render(<LogViewer {...defaultProps} podName={undefined} />);
    expect(screen.getByText('Pod not yet created')).toBeInTheDocument();
  });

  it('creates EventSource with correct URL when podName is provided', () => {
    render(<LogViewer {...defaultProps} />);
    expect(MockEventSource.instances.length).toBe(1);
    expect(MockEventSource.instances[0].url).toContain('/api/v1/namespaces/default/tasks/test-task/logs');
  });

  it('shows "Connected" status after SSE connection opens', () => {
    render(<LogViewer {...defaultProps} />);
    const es = MockEventSource.instances[0];

    act(() => {
      es.simulateOpen();
    });

    expect(screen.getByText('Connected')).toBeInTheDocument();
  });

  it('displays log lines from SSE messages', () => {
    render(<LogViewer {...defaultProps} />);
    const es = MockEventSource.instances[0];

    act(() => {
      es.simulateOpen();
      es.simulateMessage({ type: 'log', content: 'Building project...' });
      es.simulateMessage({ type: 'log', content: 'Tests passed!' });
    });

    expect(screen.getByText('Building project...')).toBeInTheDocument();
    expect(screen.getByText('Tests passed!')).toBeInTheDocument();
  });

  it('shows line numbers for log entries', () => {
    render(<LogViewer {...defaultProps} />);
    const es = MockEventSource.instances[0];

    act(() => {
      es.simulateOpen();
      es.simulateMessage({ type: 'log', content: 'Line 1' });
      es.simulateMessage({ type: 'log', content: 'Line 2' });
    });

    expect(screen.getByText('1')).toBeInTheDocument();
    expect(screen.getByText('2')).toBeInTheDocument();
  });

  it('shows line count in footer', () => {
    render(<LogViewer {...defaultProps} />);
    const es = MockEventSource.instances[0];

    act(() => {
      es.simulateOpen();
      es.simulateMessage({ type: 'log', content: 'Line 1' });
      es.simulateMessage({ type: 'log', content: 'Line 2' });
    });

    expect(screen.getByText('2 lines')).toBeInTheDocument();
  });

  it('updates status from SSE status messages', () => {
    render(<LogViewer {...defaultProps} />);
    const es = MockEventSource.instances[0];

    act(() => {
      es.simulateOpen();
      es.simulateMessage({ type: 'status', podPhase: 'Running' });
    });

    expect(screen.getByText('Pod: Running')).toBeInTheDocument();
  });

  it('shows error message from SSE error events', () => {
    render(<LogViewer {...defaultProps} />);
    const es = MockEventSource.instances[0];

    act(() => {
      es.simulateOpen();
      es.simulateMessage({ type: 'error', message: 'Container crashed' });
    });

    expect(screen.getByText('Container crashed')).toBeInTheDocument();
  });

  it('updates status and closes on complete event', () => {
    render(<LogViewer {...defaultProps} />);
    const es = MockEventSource.instances[0];

    act(() => {
      es.simulateOpen();
      es.simulateMessage({ type: 'complete', phase: 'Completed' });
    });

    expect(screen.getByText('Completed (Completed)')).toBeInTheDocument();
    expect(es.close).toHaveBeenCalled();
  });

  it('shows reconnecting message on connection error when running', () => {
    render(<LogViewer {...defaultProps} isRunning={true} />);
    const es = MockEventSource.instances[0];

    act(() => {
      es.simulateOpen();
      es.simulateError();
    });

    expect(screen.getByText('Connection lost, reconnecting...')).toBeInTheDocument();
  });

  it('shows stream ended message on error when not running', () => {
    render(<LogViewer {...defaultProps} isRunning={false} />);
    const es = MockEventSource.instances[0];

    act(() => {
      es.simulateOpen();
      es.simulateError();
    });

    expect(screen.getByText('Stream ended')).toBeInTheDocument();
  });

  it('clears logs when Clear button is clicked', async () => {
    const user = userEvent.setup();
    render(<LogViewer {...defaultProps} />);
    const es = MockEventSource.instances[0];

    act(() => {
      es.simulateOpen();
      es.simulateMessage({ type: 'log', content: 'Some log' });
    });

    expect(screen.getByText('Some log')).toBeInTheDocument();

    await user.click(screen.getByText('Clear'));

    expect(screen.queryByText('Some log')).not.toBeInTheDocument();
    expect(screen.getByText('0 lines')).toBeInTheDocument();
  });

  it('toggles search bar with search button', async () => {
    const user = userEvent.setup();
    render(<LogViewer {...defaultProps} />);

    // Search bar not visible initially
    expect(screen.queryByPlaceholderText('Search logs...')).not.toBeInTheDocument();

    // Click search button
    await user.click(screen.getByTitle('Search (Ctrl+F)'));

    // Search bar should appear
    expect(screen.getByPlaceholderText('Search logs...')).toBeInTheDocument();
  });

  it('filters logs by search query', async () => {
    const user = userEvent.setup();
    render(<LogViewer {...defaultProps} />);
    const es = MockEventSource.instances[0];

    act(() => {
      es.simulateOpen();
      es.simulateMessage({ type: 'log', content: 'Error: something failed' });
      es.simulateMessage({ type: 'log', content: 'Info: all good' });
      es.simulateMessage({ type: 'log', content: 'Error: another failure' });
    });

    // Open search
    await user.click(screen.getByTitle('Search (Ctrl+F)'));
    const searchInput = screen.getByPlaceholderText('Search logs...');
    await user.type(searchInput, 'Error');

    // Match count should show
    expect(screen.getByText('2 matches')).toBeInTheDocument();
  });

  it('closes search with Close button', async () => {
    const user = userEvent.setup();
    render(<LogViewer {...defaultProps} />);

    await user.click(screen.getByTitle('Search (Ctrl+F)'));
    expect(screen.getByPlaceholderText('Search logs...')).toBeInTheDocument();

    await user.click(screen.getByText('Close'));
    expect(screen.queryByPlaceholderText('Search logs...')).not.toBeInTheDocument();
  });

  it('toggles fullscreen', async () => {
    const user = userEvent.setup();
    const { container } = render(<LogViewer {...defaultProps} />);

    // Not fullscreen initially
    expect(container.firstChild).not.toHaveClass('fixed');

    await user.click(screen.getByTitle('Fullscreen'));

    // Should be fullscreen
    expect(container.firstChild).toHaveClass('fixed');

    await user.click(screen.getByTitle('Exit fullscreen'));

    // Should exit fullscreen
    expect(container.firstChild).not.toHaveClass('fixed');
  });

  it('shows "Waiting for logs..." when pod exists but no logs yet', () => {
    render(<LogViewer {...defaultProps} />);
    expect(screen.getByText('Waiting for logs...')).toBeInTheDocument();
  });

  it('closes EventSource on unmount', () => {
    const { unmount } = render(<LogViewer {...defaultProps} />);
    const es = MockEventSource.instances[0];

    unmount();

    expect(es.close).toHaveBeenCalled();
  });

  it('does not create EventSource when podName is undefined', () => {
    render(<LogViewer {...defaultProps} podName={undefined} />);
    expect(MockEventSource.instances.length).toBe(0);
  });
});
