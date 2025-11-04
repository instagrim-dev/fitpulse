import { useQuery } from '@tanstack/react-query';
import { AuthContext } from '../context/AuthContext';
import { useContext } from 'react';
import { searchExercises } from '../api/ontology';

/**
 * Props accepted by {@link OntologyInsights}.
 */
interface OntologyInsightsProps {
  /**
   * Search query to snapshot; used for cache keying and lookups.
   */
  query: string;
}

/**
 * Displays a lightweight panel summarising the top ontology results for the supplied query.
 * Hidden when the user is unauthenticated to avoid misleading empty states.
 *
 * @param props - Configuration for the insights panel, including the current query string.
 */
export function OntologyInsights({ query }: OntologyInsightsProps) {
  const { token } = useContext(AuthContext);
  const { data, isLoading } = useQuery({
    queryKey: ['ontology-insights', token, query],
    enabled: !!token && !!query,
    queryFn: () => searchExercises(token, query),
  });

  if (!token) {
    return null;
  }

  return (
    <section className="panel">
      <div className="dashboard__header">
        <h2>Ontology Insights</h2>
      </div>
      {isLoading && <p>Loading insights…</p>}
      {!isLoading && data && (
        <ul className="insights-list">
          {Array.from(new Map(data.items.map((e) => [e.id, e])).values())
            .slice(0, 5)
            .map((exercise) => (
              <li key={exercise.id}>
                <strong>{exercise.name}</strong>
                <div className="meta">
                  Difficulty: {exercise.difficulty || 'n/a'} | Targets:{' '}
                  {exercise.targets?.join(', ') || 'n/a'}
                </div>
              </li>
            ))}
          {data.items.length === 0 && <li>No matching exercises yet.</li>}
        </ul>
      )}
    </section>
  );
}
