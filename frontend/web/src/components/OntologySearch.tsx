import { useContext, useState } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import { AuthContext } from '../context/AuthContext';
import { searchExercises, upsertExercise } from '../api/ontology';
import { EmptyState } from './EmptyState';

/**
 * Props accepted by {@link OntologySearch}.
 */
interface OntologySearchProps {
  query: string;
  onQueryChange: (value: string) => void;
}

/**
 * Composite panel for querying the ontology service and quickly adding new exercises.
 * Provides read and write flows side-by-side so operator workflows stay in one view.
 *
 * @param props - Configuration for the search form, including the current query and setter.
 */
export function OntologySearch({ query, onQueryChange }: OntologySearchProps) {
  const { token } = useContext(AuthContext);
  const [newExercise, setNewExercise] = useState({ name: '', difficulty: '', targets: '' });

  const { data, isFetching, isError, refetch } = useQuery({
    queryKey: ['ontology', token, query],
    enabled: !!token,
    queryFn: () => searchExercises(token, query),
    retry: false,
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

  const searchDisabled = !token;
  const searching = isFetching && !!token;
  const hasResults = (data?.items?.length ?? 0) > 0;

  return (
    <section className="panel">
      <h2>Exercise Ontology</h2>
      <div className="panel-grid">
        <div>
          <label>
            Search term
            <input
              value={query}
              onChange={(e) => onQueryChange(e.target.value)}
              placeholder="e.g. squat"
              disabled={searchDisabled}
            />
          </label>
          <button onClick={() => refetch()} disabled={searchDisabled || searching}>
            {searching ? 'Searching…' : 'Search'}
          </button>
          {!token && (
            <EmptyState
              title="Authentication required"
              description="Provide an ontology read token to browse exercises."
            />
          )}
          {token && isError && (
            <EmptyState
              variant="error"
              title="Unable to fetch ontology results."
              description="The ontology service may be unavailable. Retry the search shortly."
              actionLabel={searching ? 'Retrying…' : 'Retry search'}
              onAction={() => refetch()}
              actionDisabled={searching}
            />
          )}
          {token && !isError && !hasResults && !searching && (
            <EmptyState
              title="No exercises found"
              description="Try adjusting your search term or create a new exercise using the form."
            />
          )}
          {token && hasResults && (
            <ul className="list">
              {Array.from(new Map((data?.items ?? []).map((ex) => [ex.id, ex])).values()).map((ex) => (
                <li key={ex.id}>
                  <strong>{ex.name}</strong> — {ex.difficulty || 'n/a'}
                  {ex.targets && ex.targets.length > 0 && (
                    <div className="meta">targets: {ex.targets.join(', ')}</div>
                  )}
                </li>
              ))}
            </ul>
          )}
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
                disabled={!token}
              />
            </label>
            <label>
              Difficulty
              <input
                value={newExercise.difficulty}
                onChange={(e) => setNewExercise((prev) => ({ ...prev, difficulty: e.target.value }))}
                disabled={!token}
              />
            </label>
            <label>
              Targets (comma-separated)
              <input
                value={newExercise.targets}
                onChange={(e) => setNewExercise((prev) => ({ ...prev, targets: e.target.value }))}
                disabled={!token}
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
