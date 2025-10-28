import { render, screen } from '@testing-library/react';
import { ActivityTimeline } from './ActivityTimeline';
import type { ActivityItem } from '../api/activity';

describe('ActivityTimeline', () => {
  const baseItem: ActivityItem = {
    activity_id: 'a-1',
    tenant_id: 'tenant',
    user_id: 'user',
    activity_type: 'Ride',
    started_at: '2025-10-27T12:00:00Z',
    duration_min: 45,
    source: 'fitbit',
    status: 'synced',
    created_at: '2025-10-27T12:00:00Z',
    updated_at: '2025-10-27T12:10:00Z',
    version: '3',
  };

  it('renders activities with status pills', () => {
    render(<ActivityTimeline items={[baseItem]} />);
    expect(screen.getByText('Ride')).toBeInTheDocument();
    expect(screen.getByText('Synced')).toBeInTheDocument();
    expect(screen.getByText('Version · v3')).toBeInTheDocument();
  });

  it('renders empty state when no items', () => {
    render(<ActivityTimeline items={[]} emptyMessage="No data" />);
    expect(screen.getByText('No data')).toBeInTheDocument();
  });

  it('shows loading skeleton when requested', () => {
    const { container } = render(<ActivityTimeline isLoading />);
    expect(container.querySelectorAll('.timeline__skeleton').length).toBeGreaterThan(0);
  });

  it('displays failure guidance for failed items', () => {
    const failed: ActivityItem = {
      ...baseItem,
      activity_id: 'a-2',
      status: 'failed',
      failure_reason: 'Schema registry rejected payload',
      replay_available: true,
      next_retry_at: '2025-10-28T15:00:00Z',
    };
    render(<ActivityTimeline items={[failed]} />);
    expect(screen.getByText('Schema registry rejected payload')).toBeInTheDocument();
    expect(screen.getByText(/Replay queued for/)).toBeInTheDocument();
  });

  it('falls back to generic guidance when failure details missing', () => {
    const failed: ActivityItem = {
      ...baseItem,
      activity_id: 'a-3',
      status: 'failed',
      failure_reason: undefined,
      replay_available: false,
      next_retry_at: undefined,
      quarantined_at: undefined,
    };
    render(<ActivityTimeline items={[failed]} />);
    expect(screen.getByText('Investigate DLQ entry for this activity; replay when resolved.')).toBeInTheDocument();
  });
});
