import { useContext, useState } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import { AuthContext } from '../context/AuthContext';
import { searchExercises, upsertExercise } from '../api/ontology';

/**
 * Composite panel for querying the ontology service and quickly adding new exercises.
 * Provides read and write flows side-by-side so operator workflows stay in one view.
 */
export function OntologySearch() {
  const { token } = useContext(AuthContext);
  const [query, setQuery] = useState('');
  const [newExercise, setNewExercise] = useState({ name: '', difficulty: '', targets: '' });

  const { data, isFetching, refetch } = useQuery({
    queryKey: ['ontology', query],
    enabled: !!token,
    queryFn: () => searchExercises(token, query),
  });

  const upsertMutation = useMutation({
    mutationFn: () =>
      upsertExercise(token, {
        name: newExercise.name,
        difficulty: newExercise.difficulty,
        targets: newExercise.targets.split(',').map((s) => s.trim()).filter(Boolean),
      }),
    onSuccess: () => {
      setNewExercise({ name: '', difficulty: '', targets: '' });
      refetch();
    },
  });

  return (
    <section className="panel">
      <h2>Exercise Ontology</h2>
      <div className="panel-grid">
        <div>
          <label>
            Search term
            <input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="e.g. squat"
              disabled={!token}
            />
          </label>
          <button onClick={() => refetch()} disabled={!token || isFetching}>
            {isFetching ? 'Searching…' : 'Search'}
          </button>
          <ul className="list">
            {data?.items.map((ex) => (
              <li key={ex.id}>
                <strong>{ex.name}</strong> — {ex.difficulty || 'n/a'}
                {ex.targets && ex.targets.length > 0 && (
                  <div className="meta">targets: {ex.targets.join(', ')}</div>
                )}
              </li>
            ))}
          </ul>
        </div>
        <div>
          <h3>Add Exercise</h3>
          <form
            onSubmit={(e) => {
              e.preventDefault();
              upsertMutation.mutate();
            }}
          >
            <label>
              Name
              <input
                value={newExercise.name}
                onChange={(e) => setNewExercise((prev) => ({ ...prev, name: e.target.value }))}
                required
              />
            </label>
            <label>
              Difficulty
              <input
                value={newExercise.difficulty}
                onChange={(e) => setNewExercise((prev) => ({ ...prev, difficulty: e.target.value }))}
              />
            </label>
            <label>
              Targets (comma-separated)
              <input
                value={newExercise.targets}
                onChange={(e) => setNewExercise((prev) => ({ ...prev, targets: e.target.value }))}
              />
            </label>
            <button type="submit" disabled={!token || upsertMutation.isPending}>
              {upsertMutation.isPending ? 'Saving…' : 'Save Exercise'}
            </button>
          </form>
          {upsertMutation.isError && <p className="error">{String(upsertMutation.error)}</p>}
          {upsertMutation.isSuccess && <p className="success">Exercise saved.</p>}
        </div>
      </div>
    </section>
  );
}
