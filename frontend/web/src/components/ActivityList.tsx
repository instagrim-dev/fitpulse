import { useContext, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { AuthContext } from '../context/AuthContext';
import { listActivities } from '../api/activity';

export interface ActivityListProps {
  userId: string;
}

/**
 * Displays recent activities for the provided user and exposes basic pagination.
 * Fetches data through React Query, ensuring the UI reacts to cache invalidations
 * triggered elsewhere (for example, after submitting a new activity).
 */
export function ActivityList({ userId }: ActivityListProps) {
  const { token } = useContext(AuthContext);
  const [cursor, setCursor] = useState<string | undefined>();

  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ['activities', userId, cursor],
    enabled: !!token,
    queryFn: () => listActivities(token, userId, cursor),
  });

  return (
    <section className="panel">
      <div className="panel-header">
        <h2>Recent Activities</h2>
        <button onClick={() => refetch()} disabled={!token}>
          Refresh
        </button>
      </div>
      {!token && <p>Provide a JWT token to load activities.</p>}
      {isLoading && <p>Loading…</p>}
      {isError && <p className="error">Failed to load activities.</p>}
      <ul className="list">
        {data?.items.map((item) => (
          <li key={item.activity_id}>
            <strong>{item.activity_type}</strong> — {item.duration_min} min on
            {' '}
            {new Date(item.started_at).toLocaleString()}
            <div className="meta">status: {item.status}</div>
          </li>
        ))}
      </ul>
      <div className="list-footer">
        <button
          onClick={() => setCursor(data?.next_cursor)}
          disabled={!data?.next_cursor}
        >
          Next page
        </button>
      </div>
    </section>
  );
}
