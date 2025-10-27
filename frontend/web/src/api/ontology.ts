/**
 * Base URL for the ontology service. Falls back to the local compose port when
 * `VITE_ONTOLOGY_API_URL` is not supplied.
 */
const ONTOLOGY_API_URL = import.meta.env.VITE_ONTOLOGY_API_URL || 'http://localhost:8090';

/**
 * Exercise node returned from the ontology service.
 */
export interface ExerciseNode {
  id: string;
  name: string;
  difficulty?: string;
  targets?: string[];
  requires?: string[];
  contraindicated_with?: string[];
  complementary_to?: string[];
  last_updated?: string;
}

export interface ExerciseListResponse {
  items: ExerciseNode[];
}

/**
 * Query the ontology service for exercises matching a search string.
 *
 * @param token - JWT with `ontology:read` scope.
 * @param query - Free-text query string.
 * @returns A collection of exercise nodes matching the criteria.
 * @throws Error when the HTTP request fails or returns a non-2xx status.
 */
export async function searchExercises(token: string, query: string) {
  const params = new URLSearchParams();
  if (query) params.set('query', query);

  const resp = await fetch(`${ONTOLOGY_API_URL}/v1/exercises?${params.toString()}`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  if (!resp.ok) {
    throw new Error(`Search failed: ${resp.status}`);
  }
  return (await resp.json()) as ExerciseListResponse;
}

/**
 * Payload accepted by the ontology service when creating or updating an exercise.
 */
export interface UpsertExercisePayload {
  id?: string;
  name: string;
  difficulty?: string;
  targets?: string[];
  requires?: string[];
}

/**
 * Create or update an exercise definition.
 *
 * @param token - JWT with permissions to mutate ontology data.
 * @param payload - Exercise attributes to persist.
 * @returns The ontology service response payload.
 * @throws Error when the HTTP request fails or returns a non-2xx status.
 */
export async function upsertExercise(token: string, payload: UpsertExercisePayload) {
  const resp = await fetch(`${ONTOLOGY_API_URL}/v1/exercises`, {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${token}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  });
  if (!resp.ok) {
    throw new Error(`Upsert failed: ${resp.status}`);
  }
  return await resp.json();
}
