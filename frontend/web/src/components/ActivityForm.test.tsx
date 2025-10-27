import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { vi } from 'vitest';
import { ActivityForm } from './ActivityForm';
import { AuthContext } from '../context/AuthContext';

vi.mock('../api/activity', () => ({
  createActivity: vi.fn().mockResolvedValue({ activity_id: 'abc', status: 'pending', idempotent_replay: false })
}));

const queryClient = new QueryClient();

function renderWithProviders(token = 'token') {
  return render(
    <AuthContext.Provider value={{ token, tenantId: 'tenant' }}>
      <QueryClientProvider client={queryClient}>
        <ActivityForm userId="user-123" />
      </QueryClientProvider>
    </AuthContext.Provider>
  );
}

describe('ActivityForm', () => {
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
});
