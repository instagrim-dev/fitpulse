import { useMemo } from 'react';
import type { ActivityItem } from '../api/activity';

type TimelineStatus = 'pending' | 'synced' | 'failed' | 'other';

/**
 * Props accepted by {@link ActivityTimeline}.
 */
interface ActivityTimelineProps {
  /** Activities to display in the timeline, ordered newest-first. */
  items?: ActivityItem[];
  /** When true a loading placeholder is rendered. */
  isLoading?: boolean;
  /** Message rendered when there are no activities. */
  emptyMessage?: string;
  /** Optional limit applied before rendering to keep the view focused. */
  limit?: number;
}

const STATUS_META: Record<TimelineStatus, { label: string; subtitle: string }> = {
  pending: { label: 'Pending', subtitle: 'Awaiting pipeline reconciliation' },
  synced: { label: 'Synced', subtitle: 'Persisted in activity store' },
  failed: { label: 'Failed', subtitle: 'Needs operator attention' },
  other: { label: 'Recorded', subtitle: 'Status reported by pipeline' },
};

function coerceStatus(status: string): TimelineStatus {
  switch (status.toLowerCase()) {
    case 'pending':
      return 'pending';
    case 'synced':
    case 'completed':
    case 'processed':
      return 'synced';
    case 'failed':
    case 'error':
      return 'failed';
    default:
      return 'other';
  }
}

function formatDuration(mins: number | undefined) {
  if (!mins || Number.isNaN(mins) || mins <= 0) {
    return 'Unknown duration';
  }
  if (mins < 60) {
    return `${mins} min`;
  }
  const hours = Math.floor(mins / 60);
  const minutes = mins % 60;
  if (minutes === 0) {
    return `${hours} hr${hours === 1 ? '' : 's'}`;
  }
  return `${hours} hr${hours === 1 ? '' : 's'} ${minutes} min`;
}

function formatTimestamp(iso?: string) {
  if (!iso) {
    return { absolute: 'Unknown start time', relative: '' };
  }
  const date = new Date(iso);
  const absolute = date.toLocaleString();
  const relative = formatRelative(date);
  return { absolute, relative };
}

function formatRelative(date: Date) {
  const delta = Date.now() - date.getTime();
  const minutes = Math.round(delta / 60000);
  if (Number.isNaN(minutes)) return '';
  if (minutes < 1) return 'just now';
  if (minutes < 60) return `${minutes} min ago`;
  const hours = Math.round(minutes / 60);
  if (hours < 24) return `${hours} hr${hours === 1 ? '' : 's'} ago`;
  const days = Math.round(hours / 24);
  if (days < 7) return `${days} day${days === 1 ? '' : 's'} ago`;
  const weeks = Math.round(days / 7);
  return `${weeks} wk${weeks === 1 ? '' : 's'} ago`;
}

function normaliseVersion(version: string) {
  const trimmed = version.trim();
  if (!trimmed) return null;
  return trimmed.toLowerCase().startsWith('v') ? trimmed : `v${trimmed}`;
}

/**
 * Renders a vertical timeline of activity items with derived metadata such as status badges,
 * optimistic markers, and relative timestamps.
 *
 * @param props - Rendering configuration, including activity items and loading state.
 */
export function ActivityTimeline({
  items = [],
  isLoading = false,
  emptyMessage = 'No activities recorded yet.',
  limit,
}: ActivityTimelineProps) {
  const displayItems = useMemo(() => {
    if (!items) return [] as ActivityItem[];
    return typeof limit === 'number' ? items.slice(0, limit) : items;
  }, [items, limit]);

  if (isLoading) {
    return (
      <div className="timeline timeline--loading" aria-busy="true" aria-live="polite">
        <div className="timeline__skeleton" />
        <div className="timeline__skeleton" />
        <div className="timeline__skeleton" />
      </div>
    );
  }

  if (!displayItems.length) {
    return <p className="timeline__empty">{emptyMessage}</p>;
  }

  return (
    <ul className="timeline" aria-live="polite">
      {displayItems.map((item, index) => {
        const status = coerceStatus(item.status);
        const meta = STATUS_META[status];
        const { absolute, relative } = formatTimestamp(item.started_at);
        const identifier = item.activity_id || `activity-${index}`;
        const optimistic = (item.activity_id?.startsWith('temp-') ?? false) || status === 'pending';
        const versionLabel = item.version ? normaliseVersion(item.version) : null;
        const created = item.created_at ? formatTimestamp(item.created_at) : null;
        const updated = item.updated_at ? formatTimestamp(item.updated_at) : null;
        const failureMessages = buildFailureMessages(item);
        return (
          <li key={identifier} className={`timeline__item timeline__item--${status}`}>
            <div className={`timeline__marker${optimistic ? ' timeline__marker--pending' : ''}`}
              aria-hidden="true"
            />
            <div className="timeline__content">
              <div className="timeline__header">
                <div className="timeline__title">{item.activity_type || 'Activity'}</div>
                <span className={`timeline__pill timeline__pill--${status}`}>
                  {meta.label}
                </span>
              </div>
              <div className="timeline__timestamp">
                <time dateTime={item.started_at}>{absolute}</time>
                {relative && <span className="timeline__relative">{relative}</span>}
              </div>
              <div className="timeline__meta">
                <span>{formatDuration(item.duration_min)}</span>
                {item.source && <span>Source · {item.source}</span>}
                <span>{meta.subtitle}</span>
                {versionLabel && <span>Version · {versionLabel}</span>}
              </div>
              <div className="timeline__meta timeline__meta--secondary">
                {created && (
                  <span>
                    Created <time dateTime={item.created_at}>{created.relative || created.absolute}</time>
                  </span>
                )}
                {updated && (
                  <span>
                    Updated <time dateTime={item.updated_at}>{updated.relative || updated.absolute}</time>
                  </span>
                )}
                <span>ID · {identifier}</span>
              </div>
              {failureMessages.length > 0 && (
                <div className="timeline__status-note">
                  {failureMessages.map((message, idx) => (
                    <span key={`${identifier}-failure-${idx}`}>{message}</span>
                  ))}
                </div>
              )}
            </div>
          </li>
        );
      })}
    </ul>
  );
}

function buildFailureMessages(item: ActivityItem): string[] {
  const messages: string[] = [];
  if (item.failure_reason) {
    messages.push(item.failure_reason);
  }

  if (item.replay_available) {
    if (item.next_retry_at) {
      const { absolute, relative } = formatTimestamp(item.next_retry_at);
      messages.push(`Replay queued for ${relative || absolute}.`);
    } else {
      messages.push('Replay queued.');
    }
  }

  if (item.quarantined_at) {
    const { absolute, relative } = formatTimestamp(item.quarantined_at);
    messages.push(`Quarantined ${relative || absolute}.`);
  }

  if (!messages.length && item.status?.toLowerCase() === 'failed') {
    messages.push('Investigate DLQ entry for this activity; replay when resolved.');
  }

  return messages;
}
