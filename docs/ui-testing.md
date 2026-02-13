# UI Automated Testing

This document describes the frontend automated testing setup for KubeOpenCode's Web UI, including the technology stack, test architecture, how to run tests, and how to maintain them.

## Overview

The KubeOpenCode Web UI has a comprehensive automated test suite with **220 tests** across 21 test files, achieving **82.5% line coverage**. Tests run in ~3 seconds with zero external dependencies — no browser, no cluster, no network access required.

### Test Pyramid

```
┌─────────────────────────────┐
│   Page Integration Tests    │  ← 7 page test suites (101 tests)
│   (API mocking with MSW)    │    Full page rendering + user interactions
├─────────────────────────────┤
│   Component Tests           │  ← 7 component test suites (75 tests)
│   (Isolated rendering)      │    Individual component behavior
├─────────────────────────────┤
│   Unit Tests                │  ← 4 util/hook test suites (34 tests)
│   (Pure functions)          │    Utils, hooks
├─────────────────────────────┤
│   Context Tests             │  ← 1 context test suite (7 tests)
│   (State management)        │    ToastContext
└─────────────────────────────┘
```

## Technology Stack

| Tool | Version | Purpose |
|------|---------|---------|
| [Vitest](https://vitest.dev/) | ^4.0 | Test runner (fast, native TypeScript/ESM support) |
| [React Testing Library](https://testing-library.com/docs/react-testing-library/intro/) | ^16.3 | Component rendering and DOM queries |
| [@testing-library/user-event](https://testing-library.com/docs/user-event/intro/) | ^14.6 | Simulating realistic user interactions |
| [@testing-library/jest-dom](https://github.com/testing-library/jest-dom) | ^6.9 | Custom DOM matchers (`toBeInTheDocument`, etc.) |
| [MSW](https://mswjs.io/) (Mock Service Worker) | ^2.12 | API request interception and mocking |
| [jsdom](https://github.com/jsdom/jsdom) | ^28.0 | Browser environment simulation |
| [@vitest/coverage-v8](https://vitest.dev/guide/coverage) | ^4.0 | Code coverage via V8 engine |

### Why Vitest over Jest?

- Native TypeScript support without `ts-jest` configuration
- Compatible with Webpack path aliases (`@/`) out of the box
- Faster startup and execution (shared config with Vite/Webpack)
- Same API as Jest (`describe`, `it`, `expect`, `vi.mock`) — minimal learning curve

## Running Tests

### Makefile Targets

```bash
# Run all 220 tests (~3 seconds)
make ui-test

# Run tests with line coverage report
make ui-test-coverage
```

### npm Scripts (from `ui/` directory)

```bash
cd ui

# Run all tests (single run, CI mode)
npm test

# Watch mode — re-runs on file save (useful during development)
npm run test:watch

# Run with coverage report
npm run test:coverage
```

### Running a Single Test File

```bash
cd ui
npx vitest run src/components/__tests__/LogViewer.test.tsx
```

### Running Tests Matching a Pattern

```bash
cd ui
npx vitest run --reporter=verbose -t "shows error state"
```

## CI Integration

UI tests run in both PR and push workflows (`.github/workflows/pr.yaml` and `push.yaml`). The test step executes in the existing `unit-test` job, after `npm install` and before `npm run build`:

```yaml
- name: Run UI tests
  run: make ui-test
```

Since tests run in ~3 seconds with zero external dependencies, CI impact is negligible.

## File Structure

```
ui/
├── vitest.config.ts              # Vitest configuration
├── vitest.setup.ts               # Global setup (MSW lifecycle, jest-dom)
├── src/
│   ├── mocks/                    # API mocking layer
│   │   ├── data.ts               # Mock data fixtures (Tasks, Agents, Templates)
│   │   ├── handlers.ts           # MSW request handlers for all API endpoints
│   │   └── server.ts             # MSW server instance
│   ├── test/
│   │   └── utils.tsx             # renderWithProviders helper
│   ├── utils/__tests__/          # Pure function tests
│   │   ├── cookies.test.ts       # Cookie get/set/encode (7 tests)
│   │   ├── time.test.ts          # Relative/full time formatting (12 tests)
│   │   └── agent.test.ts         # Glob matching, namespace availability (11 tests)
│   ├── hooks/__tests__/
│   │   └── useFilterState.test.tsx  # URL-based filter persistence (4 tests)
│   ├── components/__tests__/     # Component tests
│   │   ├── StatusBadge.test.tsx   # Phase colors, animated dots (11 tests)
│   │   ├── Labels.test.tsx        # Label rendering, maxDisplay (8 tests)
│   │   ├── ConfirmDialog.test.tsx # Open/close, variants, keyboard (11 tests)
│   │   ├── ResourceFilter.test.tsx # Input, filtering, clear (9 tests)
│   │   ├── LogViewer.test.tsx     # SSE mock, log display, search (21 tests)
│   │   ├── YamlViewer.test.tsx    # Expand/collapse, loading, error (7 tests)
│   │   ├── Breadcrumbs.test.tsx   # Links, separators, aria labels (6 tests)
│   │   └── TimeAgo.test.tsx       # Relative time, custom className (5 tests)
│   ├── contexts/__tests__/
│   │   └── ToastContext.test.tsx  # Add/remove, types, auto-dismiss (7 tests)
│   └── pages/__tests__/          # Page integration tests
│       ├── DashboardPage.test.tsx      # Stats, recent tasks/agents (12 tests)
│       ├── TasksPage.test.tsx          # List, filter, namespace, pagination (12 tests)
│       ├── TaskCreatePage.test.tsx     # Form, agent/template, submit (12 tests)
│       ├── TaskDetailPage.test.tsx     # Detail, stop, delete, rerun (17 tests)
│       ├── AgentsPage.test.tsx         # Cards, filter, namespace, error (11 tests)
│       ├── AgentDetailPage.test.tsx    # Config, credentials, contexts (16 tests)
│       ├── TemplatesPage.test.tsx      # Cards, description, filter (11 tests)
│       └── TemplateDetailPage.test.tsx # Detail, agent link, contexts (10 tests)
```

## Test Architecture

### Configuration (`vitest.config.ts`)

```typescript
export default defineConfig({
  resolve: {
    alias: { '@': path.resolve(__dirname, 'src') },
  },
  test: {
    globals: true,           // No need to import describe/it/expect
    environment: 'jsdom',    // Browser-like environment
    setupFiles: ['./vitest.setup.ts'],
    css: false,              // Skip CSS processing (Tailwind)
    coverage: {
      provider: 'v8',
      reporter: ['text', 'lcov'],
      include: ['src/**/*.{ts,tsx}'],
      exclude: ['src/**/*.test.{ts,tsx}', 'src/mocks/**', 'src/index.tsx'],
    },
  },
});
```

Key decisions:
- `css: false` — Disables CSS processing. Tailwind CSS imports would fail in jsdom without a full PostCSS pipeline, and CSS is irrelevant for behavioral testing.
- `globals: true` — Allows using `describe`, `it`, `expect` without imports (matching Jest convention).
- `environment: 'jsdom'` — Simulates a browser DOM for React component rendering.

### Global Setup (`vitest.setup.ts`)

```typescript
import '@testing-library/jest-dom/vitest';
import { cleanup } from '@testing-library/react';
import { server } from './src/mocks/server';

beforeAll(() => server.listen({ onUnhandledRequest: 'warn' }));
afterEach(() => { cleanup(); server.resetHandlers(); });
afterAll(() => server.close());
```

This ensures:
1. jest-dom matchers (e.g., `toBeInTheDocument`) are available globally.
2. MSW intercepts all HTTP requests during tests and warns about unhandled ones.
3. React components are cleaned up after each test.
4. MSW handlers are reset after each test (prevents handler leaks between tests).

### API Mocking with MSW

[MSW (Mock Service Worker)](https://mswjs.io/) intercepts HTTP requests at the network level, so the application code (fetch calls, React Query hooks) runs exactly as it does in production — only the server responses are mocked.

**Mock data** (`src/mocks/data.ts`) — Typed fixtures that mirror real API responses:

```typescript
export const mockTasks: Task[] = [
  {
    name: 'fix-bug-123',
    namespace: 'default',
    phase: 'Running',
    description: 'Fix authentication bug in login flow',
    agentRef: { name: 'opencode-agent', namespace: 'default' },
    // ... full Task object
  },
  // ... more tasks
];
```

**Request handlers** (`src/mocks/handlers.ts`) — One handler per API endpoint:

```typescript
export const handlers = [
  http.get('/api/v1/tasks', () => {
    return HttpResponse.json(mockTaskListResponse);
  }),
  http.get('/api/v1/namespaces/:namespace/tasks/:name', ({ params, request }) => {
    const { namespace, name } = params;
    const task = mockTasks.find(t => t.namespace === namespace && t.name === name);
    if (!task) return HttpResponse.json({ error: 'not found' }, { status: 404 });
    return HttpResponse.json(task);
  }),
  // ... handlers for all 18+ endpoints
];
```

The handlers cover all API endpoints: `info`, `namespaces`, tasks (list/get/create/delete/stop), agents (list/get), templates (list/get), and YAML output variants.

**Overriding handlers in specific tests** — Use `server.use()` to temporarily replace a handler:

```typescript
it('shows error state when API fails', async () => {
  server.use(
    http.get('/api/v1/agents', () => {
      return HttpResponse.json({ message: 'Internal error' }, { status: 500 });
    })
  );
  renderWithProviders(<AgentsPage />);
  await waitFor(() => {
    expect(screen.getByText(/Error loading agents/)).toBeInTheDocument();
  });
});
```

### Test Wrapper (`src/test/utils.tsx`)

React components in KubeOpenCode require several context providers (React Query, Toast, Router). The `renderWithProviders` helper wraps components with all required providers:

```typescript
export function renderWithProviders(
  ui: React.ReactElement,
  options?: { initialEntries?: string[] }
) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },   // No retries in tests
      mutations: { retry: false },
    },
  });

  return render(ui, {
    wrapper: ({ children }) => (
      <QueryClientProvider client={queryClient}>
        <ToastProvider>
          <MemoryRouter initialEntries={options?.initialEntries || ['/']}>
            {children}
          </MemoryRouter>
        </ToastProvider>
      </QueryClientProvider>
    ),
  });
}
```

Key settings:
- `retry: false` — Tests should fail immediately on API errors, not retry.
- `gcTime: 0` — Garbage collect query cache immediately to prevent test pollution.
- `MemoryRouter` — In-memory routing (no real browser URL needed).

### SSE Testing Pattern (LogViewer)

The LogViewer component uses Server-Sent Events (SSE) via `EventSource` for real-time log streaming. Since jsdom doesn't provide `EventSource`, tests use a custom `MockEventSource` class:

```typescript
class MockEventSource {
  static instances: MockEventSource[] = [];
  url: string;
  onmessage: ((ev: MessageEvent) => void) | null = null;
  close = vi.fn();

  constructor(url: string) {
    this.url = url;
    MockEventSource.instances.push(this);
  }

  simulateOpen() {
    this.readyState = 1;
    this.onopen?.(new Event('open'));
  }

  simulateMessage(data: object) {
    this.onmessage?.(new MessageEvent('message', {
      data: JSON.stringify(data)
    }));
  }

  simulateError() {
    this.readyState = 2;
    this.onerror?.(new Event('error'));
  }
}

// Register as global
vi.stubGlobal('EventSource', MockEventSource);
```

This allows tests to control the SSE connection lifecycle:

```typescript
it('displays log lines from SSE messages', () => {
  render(<LogViewer namespace="default" taskName="test" podName="pod" isRunning={true} />);
  const es = MockEventSource.instances[0];

  act(() => {
    es.simulateOpen();
    es.simulateMessage({ type: 'log', content: 'Building project...' });
  });

  expect(screen.getByText('Building project...')).toBeInTheDocument();
});
```

## Test Coverage

Current coverage report (as of 220 tests):

| Category | Files | Stmts | Branch | Funcs | Lines |
|----------|-------|-------|--------|-------|-------|
| **Utils** (cookies, time, agent) | 3 | 100% | 100% | 100% | 100% |
| **Hooks** (useFilterState) | 1 | 100% | 100% | 100% | 100% |
| **Test utils** | 1 | 100% | 100% | 100% | 100% |
| **Contexts** (ToastContext) | 1 | 94% | 67% | 100% | 93% |
| **Components** | 10 | 77% | 80% | 70% | 78% |
| **Pages** | 8 | 78% | 76% | 71% | 81% |
| **API client** | 1 | 72% | 57% | 70% | 86% |
| **Overall** | **25** | **79%** | **76%** | **73%** | **83%** |

Untested files (by design):
- `App.tsx` — Root component with route definitions (covered by page tests indirectly)
- `Layout.tsx` — Static layout shell
- `ToastContainer.tsx` — Presentational toast rendering (ToastContext logic is tested)
- `Skeleton.tsx` — Simple loading placeholders

## What Each Test Suite Covers

### Unit Tests

| Suite | Tests | What It Verifies |
|-------|-------|------------------|
| `cookies.test.ts` | 7 | `getCookie`, `setCookie` with encoding, expiration, defaults |
| `time.test.ts` | 12 | `formatTimeAgo` (just now, minutes, hours, days, months, years), `formatFullTime` |
| `agent.test.ts` | 11 | `matchGlob` (exact, wildcard, prefix), `isAgentAvailableForNamespace` (allowed/denied/global) |
| `useFilterState.test.tsx` | 4 | URL search param persistence, filter updates, clear behavior |

### Component Tests

| Suite | Tests | What It Verifies |
|-------|-------|------------------|
| `StatusBadge.test.tsx` | 11 | Color mapping per phase (Running/Completed/Failed/Pending/Queued/Stopped), animated dots for active states |
| `Labels.test.tsx` | 8 | Key=value rendering, `maxDisplay` truncation, "+N more" badge, empty state |
| `ConfirmDialog.test.tsx` | 11 | Open/close, confirm/cancel callbacks, destructive variant styling, Escape key, click-outside |
| `ResourceFilter.test.tsx` | 9 | Text input, debounced filtering, clear button, Enter key submit |
| `LogViewer.test.tsx` | 21 | SSE connection, log line display, line numbers, search/filter, fullscreen toggle, clear, reconnection, cleanup on unmount |
| `YamlViewer.test.tsx` | 7 | Expand/collapse toggle, YAML content display, loading state, error state, copy button |
| `Breadcrumbs.test.tsx` | 6 | Link rendering, plain text for current item, SVG separators, aria labels |
| `TimeAgo.test.tsx` | 5 | Relative time display (fake timers), title attribute, custom className, Date object input |

### Context Tests

| Suite | Tests | What It Verifies |
|-------|-------|------------------|
| `ToastContext.test.tsx` | 7 | `addToast`/`removeToast`, toast types (success/error/info), unique IDs, auto-dismiss after 4s (fake timers), multiple simultaneous toasts |

### Page Integration Tests

| Suite | Tests | What It Verifies |
|-------|-------|------------------|
| `DashboardPage.test.tsx` | 12 | Stats cards (task/agent/template counts), recent tasks list, recent agents list, "New Task" link, empty states, error states |
| `TasksPage.test.tsx` | 12 | Task list rendering, phase filter buttons, namespace selector, name filter, pagination, empty state, error state with retry |
| `TaskCreatePage.test.tsx` | 12 | Form fields, agent dropdown, template dropdown, namespace selector, form submission, API error handling, navigation after create |
| `TaskDetailPage.test.tsx` | 17 | Task metadata, agent reference link, status badge, stop/delete/rerun actions, confirm dialog, breadcrumbs, conditions display, error/loading/not-found states |
| `AgentsPage.test.tsx` | 11 | Agent cards, context/credential counts, maxConcurrentTasks badge, allowed namespaces, card links, namespace filter, error/empty states |
| `AgentDetailPage.test.tsx` | 16 | Agent name/namespace, executor/agent images, workspaceDir, credentials list, contexts list, server status, conditions, allowed namespaces, labels, error/loading states |
| `TemplatesPage.test.tsx` | 11 | Template cards, description display, agent reference, contexts count, card links, namespace filter, error/empty states |
| `TemplateDetailPage.test.tsx` | 10 | Template name/namespace, description, agent reference link, contexts list, "Create Task" CTA, error/loading states |

## Maintenance Guide

### When to Update Tests

| Scenario | What to Change |
|----------|---------------|
| Changed API response shape (Go server) | Update `src/mocks/data.ts` (mock data) and potentially `src/mocks/handlers.ts` |
| Added a new API endpoint | Add handler in `src/mocks/handlers.ts` |
| Changed component behavior | Update the corresponding `__tests__/*.test.tsx` |
| Added a new page | Create `src/pages/__tests__/NewPage.test.tsx` |
| Added a new component | Create `src/components/__tests__/NewComponent.test.tsx` |
| Added a new utility function | Create `src/utils/__tests__/newUtil.test.ts` |
| Added a new React context | Create `src/contexts/__tests__/NewContext.test.tsx` |

### Writing a New Component Test

```typescript
import { describe, it, expect } from 'vitest';
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { renderWithProviders } from '../../test/utils';
import MyComponent from '../MyComponent';

describe('MyComponent', () => {
  it('renders the title', () => {
    renderWithProviders(<MyComponent title="Hello" />);
    expect(screen.getByText('Hello')).toBeInTheDocument();
  });

  it('calls onClick when button is clicked', async () => {
    const user = userEvent.setup();
    const onClick = vi.fn();
    renderWithProviders(<MyComponent onClick={onClick} />);

    await user.click(screen.getByRole('button'));
    expect(onClick).toHaveBeenCalledOnce();
  });
});
```

### Writing a New Page Test

```typescript
import { describe, it, expect } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { server } from '../../mocks/server';
import { renderWithProviders } from '../../test/utils';
import MyPage from '../MyPage';

describe('MyPage', () => {
  it('renders data from API', async () => {
    renderWithProviders(<MyPage />, { initialEntries: ['/my-page'] });

    await waitFor(() => {
      expect(screen.getByText('Expected Content')).toBeInTheDocument();
    });
  });

  it('shows error state when API fails', async () => {
    server.use(
      http.get('/api/v1/my-endpoint', () => {
        return HttpResponse.json({ message: 'error' }, { status: 500 });
      })
    );

    renderWithProviders(<MyPage />, { initialEntries: ['/my-page'] });

    await waitFor(() => {
      expect(screen.getByText(/Error/)).toBeInTheDocument();
    });
  });
});
```

### Common Patterns and Pitfalls

**1. Use `waitFor` for async data**

Page components fetch data via React Query. The initial render shows loading state; data appears after the API responds.

```typescript
// Wrong — data isn't rendered yet
renderWithProviders(<AgentsPage />);
expect(screen.getByText('opencode-agent')).toBeInTheDocument(); // Fails!

// Correct — wait for async data
renderWithProviders(<AgentsPage />);
await waitFor(() => {
  expect(screen.getByText('opencode-agent')).toBeInTheDocument();
});
```

**2. Handle duplicate text on the page**

When multiple elements share the same text (e.g., breadcrumb + heading both say "Agents"), use more specific queries:

```typescript
// Wrong — matches breadcrumb and heading
screen.getByText('Create Task'); // Error: found multiple elements

// Correct — use role-based query
screen.getByRole('heading', { name: 'Create Task' });
```

**3. Use `getAllByText` for repeated elements**

When multiple cards/rows display the same label:

```typescript
// Wrong — multiple agents show "Contexts"
screen.getByText('Contexts'); // Error: found multiple elements

// Correct
const labels = screen.getAllByText('Contexts');
expect(labels.length).toBeGreaterThan(0);
```

**4. Testing content inside `<pre>` tags**

Multi-line text in `<pre>` elements can't be matched with `getByText`. Use DOM queries:

```typescript
// Wrong
screen.getByText('apiVersion: kubeopencode.io/v1alpha1\nkind: Task'); // Fails

// Correct
const pre = container.querySelector('pre');
expect(pre!.textContent).toContain('apiVersion: kubeopencode.io/v1alpha1');
```

**5. Fake timers for time-dependent behavior**

```typescript
beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(new Date('2026-02-13T12:00:00Z'));
});

afterEach(() => {
  vi.useRealTimers();
});
```

## Troubleshooting

### Tests fail locally but pass in CI (or vice versa)

Tests are deterministic and environment-independent. If you see differences:
- Check Node.js version (CI uses Node 22)
- Run `cd ui && npm install` to ensure dependencies are up to date
- Check that `vitest.setup.ts` has not been modified

### "Cannot find module" errors

Ensure the path alias is configured in `vitest.config.ts`:

```typescript
resolve: {
  alias: { '@': path.resolve(__dirname, 'src') },
},
```

### MSW warns about unhandled requests

This means a component is calling an API endpoint that has no handler in `src/mocks/handlers.ts`. Add a handler for the missing endpoint.

### Tests timing out

Increase the timeout for specific tests:

```typescript
it('handles slow operation', async () => {
  // test code
}, 10000); // 10 second timeout
```

Or check if `waitFor` is waiting for something that never appears (wrong mock data, typo in expected text).
