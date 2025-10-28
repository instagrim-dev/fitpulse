import { useContext, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { AuthContext } from '../context/AuthContext';
import { listActivities } from '../api/activity';
import { ActivityTimeline } from './ActivityTimeline';
import { EmptyState } from './EmptyState';

/** Props consumed by the `ActivityList` component. */
export interface ActivityListProps {
  /** Identifier of the user whose activities should be listed. */
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

  const { data, isLoading, isError, isFetching, refetch } = useQuery({
    queryKey: ['activities', token, userId, cursor],
    enabled: !!token,
    queryFn: () => listActivities(token, userId, cursor),
    retry: false,
  });

  const refreshDisabled = !token || isFetching || isLoading;

  const renderBody = () => {
    if (!token) {
      return (
        <EmptyState
          title="Authentication required"
          description="Provide a JWT token with activities permissions to load recent history."
        />
      );
    }
    if (isError) {
      return (
        <EmptyState
          variant="error"
          title="Unable to load recent activities."
          description="The activity service is temporarily unreachable. Try again in a moment."
          actionLabel={isFetching ? 'Retrying…' : 'Try again'}
          onAction={() => refetch()}
          actionDisabled={isFetching}
        />
      );
    }
    return (
      <>
        <ActivityTimeline
          items={data?.items ?? []}
          isLoading={isLoading}
          emptyMessage="No activities recorded yet. Use the form above to create one."
        />
        <div className="list-footer">
          <button
            onClick={() => setCursor(data?.next_cursor)}
            disabled={!data?.next_cursor || isFetching}
          >
            Next page
          </button>
        </div>
      </>
    );
  };

  return (
    <section className="panel">
      <div className="panel-header">
        <h2>Recent Activities</h2>
        <button onClick={() => refetch()} disabled={refreshDisabled}>
          {isFetching ? 'Refreshing…' : 'Refresh'}
        </button>
      </div>
      {renderBody()}
    </section>
  );
}
