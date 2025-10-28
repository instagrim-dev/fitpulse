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
  version?: string;
  created_at?: string;
  updated_at?: string;
  failure_reason?: string;
  next_retry_at?: string;
  quarantined_at?: string;
  replay_available?: boolean;
}

export interface ActivityListResponse {
  items: ActivityItem[];
  next_cursor?: string;
}

export interface ActivityMetricsSummary {
  total: number;
  pending: number;
  synced: number;
  failed: number;
  average_duration_minutes: number;
  average_processing_seconds: number;
  oldest_pending_age_seconds: number;
  success_rate: number;
  last_activity_at?: string;
}

export interface ActivityMetricsResponse {
  summary: ActivityMetricsSummary;
  timeline: ActivityItem[];
  timeline_limit: number;
  window_seconds: number;
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

interface MetricsOptions {
  timelineLimit?: number;
  windowHours?: number;
}

/**
 * Fetch aggregate activity metrics and a recent timeline for the supplied user.
 */
/**
 * Retrieve aggregated metrics and a recent activity timeline for the supplied user.
 *
 * @param token - Bearer token providing tenant scope.
 * @param userId - Identifier of the user whose metrics should be returned.
 * @param options - Optional timeline/window overrides.
 */
export async function getActivityMetrics(token: string, userId: string, options: MetricsOptions = {}) {
  const params = new URLSearchParams({ user_id: userId });
  if (options.timelineLimit && options.timelineLimit > 0) {
    params.set('timeline_limit', String(options.timelineLimit));
  }
  if (options.windowHours !== undefined && options.windowHours >= 0) {
    params.set('window_hours', String(options.windowHours));
  }
  const resp = await fetch(`${ACTIVITY_API_URL}/v1/activities/metrics?${params.toString()}`, {
    headers: {
      Authorization: `Bearer ${token}`,
    },
  });
  if (!resp.ok) {
    throw new Error(`Metrics fetch failed: ${resp.status}`);
  }
  return (await resp.json()) as ActivityMetricsResponse;
}
