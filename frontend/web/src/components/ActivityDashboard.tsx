import { useContext, useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { AuthContext } from '../context/AuthContext';
import { ActivityTimeline } from './ActivityTimeline';
import { getActivityMetrics, ActivityMetricsResponse } from '../api/activity';

/**
 * Props accepted by {@link ActivityDashboard}.
 */
interface ActivityDashboardProps {
  /**
   * Identifier for the user whose metrics and timeline should be rendered.
   */
  userId: string;
}

const TIMELINE_LIMIT = 6;

/**
 * Presents high-level metrics and a trimmed timeline for a single user's recent activities,
 * combining aggregation data with the {@link ActivityTimeline} component.
 *
 * @param props - Configuration for the dashboard, including the target `userId`.
 */
export function ActivityDashboard({ userId }: ActivityDashboardProps) {
  const { token } = useContext(AuthContext);
  const { data, isLoading } = useQuery<ActivityMetricsResponse | undefined>({
    queryKey: ['activity-metrics', token, userId, TIMELINE_LIMIT],
    enabled: !!token,
    queryFn: () => getActivityMetrics(token!, userId, { timelineLimit: TIMELINE_LIMIT, windowHours: 24 }),
  });

  const metrics = useMemo(() => {
    const summary = data?.summary;
    const items = data?.timeline ?? [];
    if (!summary || summary.total === 0) {
      return {
        total: 0,
        pending: 0,
        synced: 0,
        failed: 0,
        avgDuration: '—',
        successRate: '—',
        avgProcessing: '—',
        oldestPending: '—',
        lastActivity: '—',
      };
    }

    const avgDuration = summary.average_duration_minutes > 0
      ? `${Math.round(summary.average_duration_minutes)} min`
      : '—';
    const successRate = summary.success_rate > 0
      ? `${Math.round(summary.success_rate * 100)}%`
      : '—';
    const avgProcessing = summary.average_processing_seconds > 0
      ? formatDurationFromMs(summary.average_processing_seconds * 1000)
      : '—';
    const oldestPending = summary.oldest_pending_age_seconds > 0
      ? formatRelativeFromMs(summary.oldest_pending_age_seconds * 1000)
      : '—';
    const last = summary.last_activity_at ? new Date(summary.last_activity_at).toLocaleString() : '—';

    return {
      total: summary.total,
      pending: summary.pending,
      synced: summary.synced,
      failed: summary.failed,
      avgDuration,
      successRate,
      avgProcessing,
      oldestPending,
      lastActivity: last,
    };
  }, [data]);

  return (
    <section className="panel dashboard">
      <div className="dashboard__header">
        <h2>Activity Overview</h2>
      </div>
      <div className="dashboard__metrics">
        <MetricTile label="Total Activities" value={metrics.total} loading={isLoading} hint="Count of records returned by latest fetch" />
        <MetricTile label="Pending" value={metrics.pending} loading={isLoading} hint="Awaiting pipeline reconciliation" />
        <MetricTile label="Synced" value={metrics.synced} loading={isLoading} hint="Persisted and reconciled records" />
        <MetricTile label="Failed" value={metrics.failed} loading={isLoading} hint="Requires DLQ replay" />
        <MetricTile label="Avg Duration" value={metrics.avgDuration} loading={isLoading} hint="Mean user session length" />
        <MetricTile label="Success Rate" value={metrics.successRate} loading={isLoading} hint="Synced vs total activities" />
        <MetricTile label="Avg Processing Lag" value={metrics.avgProcessing} loading={isLoading} hint="Started vs latest update delta" />
        <MetricTile label="Oldest Pending" value={metrics.oldestPending} loading={isLoading} hint="Age of the longest-waiting pending activity" />
        <MetricTile label="Last Activity" value={metrics.lastActivity} loading={isLoading} hint="Most recent start timestamp" />
      </div>
      <div className="dashboard__timeline">
        <div className="dashboard__timeline-header">
          <h3>Recent timeline</h3>
          <p>Latest six activities with optimistic status markers.</p>
        </div>
        <ActivityTimeline
          items={data?.timeline ?? []}
          isLoading={isLoading}
          limit={TIMELINE_LIMIT}
          emptyMessage={token ? 'No activities recorded for this user yet.' : 'Provide a token to load activities.'}
        />
      </div>
    </section>
  );
}

interface MetricTileProps {
  label: string;
  value: string | number;
  loading?: boolean;
  hint?: string;
}

function MetricTile({ label, value, loading, hint }: MetricTileProps) {
  return (
    <div className="dashboard__tile">
      <span className="dashboard__tile-label">{label}</span>
      <span className="dashboard__tile-value">{loading ? '…' : value}</span>
      {hint && <span className="dashboard__tile-hint">{hint}</span>}
    </div>
  );
}

function formatDurationFromMs(ms: number) {
  if (!Number.isFinite(ms) || ms <= 0) {
    return '—';
  }
  const minutes = Math.round(ms / 60000);
  if (minutes < 1) {
    return '<1 min';
  }
  if (minutes < 60) {
    return `${minutes} min`;
  }
  const hours = Math.floor(minutes / 60);
  const mins = minutes % 60;
  if (mins === 0) {
    return `${hours} hr${hours === 1 ? '' : 's'}`;
  }
  return `${hours} hr${hours === 1 ? '' : 's'} ${mins} min`;
}

function formatRelativeFromMs(ms: number) {
  if (!Number.isFinite(ms) || ms <= 0) {
    return '—';
  }
  const minutes = Math.round(ms / 60000);
  if (minutes < 1) return '<1 min';
  if (minutes < 60) return `${minutes} min`;
  const hours = Math.round(minutes / 60);
  if (hours < 24) return `${hours} hr${hours === 1 ? '' : 's'}`;
  const days = Math.round(hours / 24);
  if (days < 7) return `${days} day${days === 1 ? '' : 's'}`;
  const weeks = Math.round(days / 7);
  if (weeks < 5) return `${weeks} wk${weeks === 1 ? '' : 's'}`;
  const months = Math.round(days / 30);
  if (months < 12) return `${months} mo${months === 1 ? '' : 's'}`;
  const years = Math.round(days / 365);
  return `${years} yr${years === 1 ? '' : 's'}`;
}
