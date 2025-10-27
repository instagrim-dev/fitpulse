const ACTIVITY_API_URL = import.meta.env.VITE_ACTIVITY_API_URL || 'http://localhost:8080';

export interface CreateActivityPayload {
  user_id: string;
  activity_type: string;
  started_at: string;
  duration_min: number;
  source: string;
}

export interface ActivityResponse {
  activity_id: string;
  status: string;
  idempotent_replay: boolean;
}

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
