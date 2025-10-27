import { useContext, useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { AuthContext } from '../context/AuthContext';
import { createActivity } from '../api/activity';

interface Props {
  userId: string;
}

export function ActivityForm({ userId }: Props) {
  const { token } = useContext(AuthContext);
  const queryClient = useQueryClient();
  const [activityType, setActivityType] = useState('Run');
  const [duration, setDuration] = useState(30);
  const [startTime, setStartTime] = useState(new Date().toISOString().slice(0, 16));
  const [source, setSource] = useState('web-ui');

  const mutation = useMutation({
    mutationFn: () =>
      createActivity(token, {
        user_id: userId,
        activity_type: activityType,
        started_at: new Date(startTime).toISOString(),
        duration_min: duration,
        source,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['activities', userId] });
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
      {mutation.isSuccess && <p className="success">Activity submitted!</p>}
    </section>
  );
}
