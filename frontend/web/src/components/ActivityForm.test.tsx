import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { vi } from 'vitest';
import { ActivityForm } from './ActivityForm';
import { AuthContext } from '../context/AuthContext';

const createActivityMock = vi.hoisted(() => vi.fn());

vi.mock('../api/activity', () => ({
  createActivity: createActivityMock,
}));

const createClient = () => new QueryClient();

function renderWithProviders(token = 'token') {
  const qc = createClient();
  return {
    user: userEvent.setup(),
    ...render(
    <AuthContext.Provider value={{ token, tenantId: 'tenant' }}>
      <QueryClientProvider client={qc}>
        <ActivityForm userId="user-123" />
      </QueryClientProvider>
    </AuthContext.Provider>
    ),
  };
}

describe('ActivityForm', () => {
  beforeEach(() => {
    createActivityMock.mockResolvedValue({
      activity_id: 'abc',
      tenant_id: 'tenant',
      user_id: 'user-123',
      activity_type: 'Run',
      started_at: new Date().toISOString(),
      duration_min: 30,
      source: 'web-ui',
      status: 'pending',
    });
  });

  afterEach(() => {
    createActivityMock.mockClear();
  });

  it('disables submit button without token', () => {
    renderWithProviders('');
    const button = screen.getByRole('button', { name: /submit/i });
    expect(button).toBeDisabled();
  });

  it('enables submit button with token', () => {
    renderWithProviders('valid-token');
    const button = screen.getByRole('button', { name: /submit/i });
    expect(button).toBeEnabled();
  });

  it('calls createActivity on submit', async () => {
    const { user } = renderWithProviders('valid-token');
    await user.click(screen.getByRole('button', { name: /submit/i }));
    expect(createActivityMock).toHaveBeenCalled();
  });
});
