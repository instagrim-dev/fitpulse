import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { vi } from 'vitest';
import * as activityApi from '../api/activity';
import { AuthContext } from '../context/AuthContext';
import { ActivityDashboard } from './ActivityDashboard';

describe('ActivityDashboard', () => {
  let queryClient: QueryClient;
  let nowSpy: ReturnType<typeof vi.spyOn> | null = null;

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    vi.restoreAllMocks();
    nowSpy = vi.spyOn(Date, 'now').mockReturnValue(new Date('2025-10-28T12:00:00Z').getTime());
  });

  afterEach(() => {
    queryClient.clear();
    nowSpy?.mockRestore();
  });

  function renderWithToken(token: string) {
    return render(
      <AuthContext.Provider value={{ token, tenantId: 'tenant' }}>
        <QueryClientProvider client={queryClient}>
          <ActivityDashboard userId="user-1" />
        </QueryClientProvider>
      </AuthContext.Provider>
    );
  }

  it('renders summary metrics from activity data', async () => {
    vi.spyOn(activityApi, 'getActivityMetrics').mockResolvedValue({
      summary: {
        total: 2,
        pending: 1,
        synced: 1,
        failed: 0,
        average_duration_minutes: 37.5,
        average_processing_seconds: 1200,
        oldest_pending_age_seconds: 172800,
        success_rate: 0.5,
        last_activity_at: '2025-10-27T12:00:00Z',
      },
      timeline: [
        {
          activity_id: 'a1',
          user_id: 'user-1',
          tenant_id: 'tenant',
          activity_type: 'Ride',
          duration_min: 45,
          source: 'fitbit',
          status: 'synced',
          started_at: '2025-10-27T12:00:00Z',
          created_at: '2025-10-27T12:00:00Z',
          updated_at: '2025-10-27T12:20:00Z',
          version: '2',
        },
        {
          activity_id: 'a2',
          user_id: 'user-1',
          tenant_id: 'tenant',
          activity_type: 'Run',
          duration_min: 30,
          source: 'web-ui',
          status: 'pending',
          started_at: '2025-10-26T10:00:00Z',
          created_at: '2025-10-26T10:00:00Z',
          updated_at: '2025-10-26T10:00:00Z',
          version: '1',
        },
      ],
      timeline_limit: 6,
      window_seconds: 86400,
    });

    renderWithToken('token');

    const totalLabel = await screen.findByText('Total Activities');
    await waitFor(() => expect(totalLabel.nextElementSibling?.textContent).toBe('2'));
    expect(await screen.findByText('38 min')).toBeInTheDocument();
    expect(await screen.findByText('50%')).toBeInTheDocument();
    expect(await screen.findByText('20 min')).toBeInTheDocument();
    expect(await screen.findByText('2 days')).toBeInTheDocument();
    expect(await screen.findByText('Recent timeline')).toBeInTheDocument();
    expect(await screen.findByText('Ride')).toBeInTheDocument();
  });

  it('shows placeholders when loading', () => {
    vi.spyOn(activityApi, 'getActivityMetrics').mockImplementation(
      () => new Promise(() => {})
    );

    renderWithToken('token');
    expect(screen.getAllByText('…').length).toBeGreaterThan(0);
  });

  it('shows empty state when no data', async () => {
    vi.spyOn(activityApi, 'getActivityMetrics').mockResolvedValue({
      summary: {
        total: 0,
        pending: 0,
        synced: 0,
        failed: 0,
        average_duration_minutes: 0,
        average_processing_seconds: 0,
        oldest_pending_age_seconds: 0,
        success_rate: 0,
      },
      timeline: [],
      timeline_limit: 6,
      window_seconds: 86400,
    });

    renderWithToken('token');
    expect(await screen.findByText('No activities recorded for this user yet.')).toBeInTheDocument();
  });
});
