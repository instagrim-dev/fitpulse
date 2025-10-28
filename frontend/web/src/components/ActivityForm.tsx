import { useContext, useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { AuthContext } from '../context/AuthContext';
import {
  ActivityItem,
  ActivityListResponse,
  createActivity,
  CreateActivityPayload,
} from '../api/activity';

export interface ActivityFormProps {
  userId: string;
}

/**
 * Form for submitting new activities to the backend for a specific user. The component
 * invalidates the cached activity query on success so the list refreshes automatically.
 */
export function ActivityForm({ userId }: ActivityFormProps) {
  const { token } = useContext(AuthContext);
  const queryClient = useQueryClient();
  const [activityType, setActivityType] = useState('Run');
  const [duration, setDuration] = useState(30);
  const [startTime, setStartTime] = useState(new Date().toISOString().slice(0, 16));
  const [source, setSource] = useState('web-ui');

  const mutation = useMutation({
    mutationFn: (): Promise<ActivityItem> => {
      const payload: CreateActivityPayload = {
        user_id: userId,
        activity_type: activityType,
        started_at: new Date(startTime).toISOString(),
        duration_min: duration,
        source,
      };
      return createActivity(token, payload).then((resp) => ({
        activity_id: resp.activity_id,
        tenant_id: '',
        user_id: userId,
        activity_type: payload.activity_type,
        started_at: payload.started_at,
        duration_min: payload.duration_min,
        source: payload.source,
        status: resp.status,
        version: 'v1',
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
        failure_reason: undefined,
        next_retry_at: undefined,
        quarantined_at: undefined,
        replay_available: false,
      }));
    },
    onMutate: async () => {
      if (!token) return;
      await queryClient.cancelQueries({ queryKey: ['activities', token, userId] });
      const previous = queryClient.getQueryData<ActivityListResponse>(['activities', token, userId]);
      const optimisticItem: ActivityItem = {
        activity_id: `temp-${Date.now()}`,
        tenant_id: '',
        user_id: userId,
        activity_type: activityType,
        started_at: new Date(startTime).toISOString(),
        duration_min: duration,
        source,
        status: 'pending',
        version: 'draft',
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
        failure_reason: undefined,
        next_retry_at: undefined,
        quarantined_at: undefined,
        replay_available: false,
      };
      queryClient.setQueryData<ActivityListResponse>(['activities', token, userId], (old) => {
        if (!old) {
          return { items: [optimisticItem] };
        }
        return {
          ...old,
          items: [optimisticItem, ...old.items],
        };
      });
      return { previous };
    },
    onError: (_error, _variables, context) => {
      if (context?.previous) {
        queryClient.setQueryData(['activities', token, userId], context.previous);
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['activities', token, userId] });
    },
  });

  const disabled = !token || mutation.isPending;

  return (
    <section className="panel">
      <h2>Log Activity</h2>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          mutation.mutate();
        }}
      >
        <label>
          Activity type
          <input value={activityType} onChange={(e) => setActivityType(e.target.value)} required />
        </label>
        <label>
          Duration (minutes)
          <input
            type="number"
            min={1}
            value={duration}
            onChange={(e) => setDuration(Number(e.target.value))}
            required
          />
        </label>
        <label>
          Start time
          <input
            type="datetime-local"
            value={startTime}
            onChange={(e) => setStartTime(e.target.value)}
            required
          />
        </label>
        <label>
          Source
          <input value={source} onChange={(e) => setSource(e.target.value)} />
        </label>
        <button type="submit" disabled={disabled}>
          {mutation.isPending ? 'Saving…' : 'Submit'}
        </button>
      </form>
      {mutation.isError && <p className="error">{String(mutation.error)}</p>}
      {mutation.isSuccess && !mutation.isError && <p className="success">Activity submitted!</p>}
    </section>
  );
}
