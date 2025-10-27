/**
 * Base URL for the activity service. Defaults to the local compose stack when the
 * `VITE_ACTIVITY_API_URL` environment variable is not provided.
 */
const ACTIVITY_API_URL = import.meta.env.VITE_ACTIVITY_API_URL || 'http://localhost:8080';

/**
 * Shape of the request body sent to `POST /v1/activities`.
 */
export interface CreateActivityPayload {
  user_id: string;
  activity_type: string;
  started_at: string;
  duration_min: number;
  source: string;
}

/**
 * Response returned by the activity service after creating a new activity.
 */
export interface ActivityResponse {
  activity_id: string;
  status: string;
  idempotent_replay: boolean;
}

/**
 * Submit a new activity for the authenticated user.
 *
 * @param token - JWT containing the tenant scope for authorization.
 * @param payload - Activity attributes to persist.
 * @returns The created activity descriptor from the service.
 * @throws Error when the HTTP request fails or returns a non-2xx status.
 */
export async function createActivity(token: string, payload: CreateActivityPayload) {
  const resp = await fetch(`${ACTIVITY_API_URL}/v1/activities`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify(payload),
  });

  if (!resp.ok) {
    throw new Error(`Create failed: ${resp.status}`);
  }
  return (await resp.json()) as ActivityResponse;
}

/**
 * Individual item returned from the activity listing endpoint.
 */
export interface ActivityItem {
  activity_id: string;
  tenant_id: string;
  user_id: string;
  activity_type: string;
  started_at: string;
  duration_min: number;
  source: string;
  status: string;
}

export interface ActivityListResponse {
  items: ActivityItem[];
  next_cursor?: string;
}

/**
 * Fetch the paginated activity history for a user.
 *
 * @param token - JWT containing the tenant scope for authorization.
 * @param userId - Identifier for the user whose activities should be returned.
 * @param cursor - Optional pagination cursor supplied by previous responses.
 * @returns The activity list response including optional pagination metadata.
 * @throws Error when the HTTP request fails or returns a non-2xx status.
 */
export async function listActivities(token: string, userId: string, cursor?: string) {
  const params = new URLSearchParams({ user_id: userId });
  if (cursor) {
    params.set('cursor', cursor);
  }
  const resp = await fetch(`${ACTIVITY_API_URL}/v1/activities?${params.toString()}`, {
    headers: {
      Authorization: `Bearer ${token}`,
    },
  });
  if (!resp.ok) {
    throw new Error(`List failed: ${resp.status}`);
  }
  return (await resp.json()) as ActivityListResponse;
}
