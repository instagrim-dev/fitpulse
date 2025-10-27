const ONTOLOGY_API_URL = import.meta.env.VITE_ONTOLOGY_API_URL || 'http://localhost:8090';

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

export interface UpsertExercisePayload {
  id?: string;
  name: string;
  difficulty?: string;
  targets?: string[];
  requires?: string[];
}

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
