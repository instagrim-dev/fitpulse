import { test, expect } from '@playwright/test';

const STORAGE_KEY = 'i5e-auth';

const failedActivityItem = {
  activity_id: 'act-failed-1',
  tenant_id: 'tenant-demo',
  user_id: 'user-1',
  activity_type: 'Tempo Ride',
  started_at: '2025-10-27T15:00:00.000Z',
  duration_min: 45,
  source: 'wearable-sync',
  status: 'failed',
  version: 'v3',
  created_at: '2025-10-27T15:00:00.000Z',
  updated_at: '2025-10-27T15:05:00.000Z',
  failure_reason: 'Schema registry rejected payload',
  next_retry_at: '2025-10-28T16:00:00.000Z',
  replay_available: true,
} as const;

const recoveredActivityItem = {
  ...failedActivityItem,
  status: 'synced' as const,
  version: 'v4',
  updated_at: '2025-10-28T16:05:00.000Z',
  failure_reason: undefined,
  next_retry_at: undefined,
  replay_available: false,
};

const failedActivities = { items: [failedActivityItem] };
const recoveredActivities = { items: [recoveredActivityItem] };

const failedMetrics = {
  summary: {
    total: 1,
    pending: 0,
    synced: 0,
    failed: 1,
    average_duration_minutes: 45,
    average_processing_seconds: 300,
    oldest_pending_age_seconds: 0,
    success_rate: 0,
    last_activity_at: '2025-10-27T15:05:00.000Z',
  },
  timeline: [failedActivityItem],
  timeline_limit: 6,
  window_seconds: 86_400,
};

const recoveredMetrics = {
  summary: {
    total: 1,
    pending: 0,
    synced: 1,
    failed: 0,
    average_duration_minutes: 45,
    average_processing_seconds: 180,
    oldest_pending_age_seconds: 0,
    success_rate: 1,
    last_activity_at: '2025-10-28T16:05:00.000Z',
  },
  timeline: [recoveredActivityItem],
  timeline_limit: 6,
  window_seconds: 86_400,
};

test.describe('Activity dashboard replay flow', () => {
  test.beforeEach(async ({ page }) => {
    await page.addInitScript(({ key, payload }) => {
      window.localStorage.setItem(key, payload);
    }, {
      key: STORAGE_KEY,
      payload: JSON.stringify({
        token: 'test-token',
        tenantId: 'tenant-demo',
        accountId: 'account-demo',
        userId: 'user-1',
        remember: true,
        ontologyQuery: 'ride',
      }),
    });

    await page.route('**/v1/activities/metrics?**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(recoveredMetrics),
      });
    });
  });

  test('surfaces failure details and clears after replay', async ({ page }) => {
    let servedFailureMetrics = false;
    await page.route('**/v1/activities/metrics?**', async (route) => {
      if (!servedFailureMetrics) {
        servedFailureMetrics = true;
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(failedMetrics),
        });
        return;
      }
      await route.fallback();
    });

    let callCount = 0;
    await page.route('**/v1/activities?**', async (route) => {
      callCount += 1;
      const body = callCount === 1 ? failedActivities : recoveredActivities;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(body),
      });
    });

    await page.goto('/');

    await expect(page.getByRole('heading', { name: 'Activity Overview' })).toBeVisible();
    const failureNote = page.locator('#overview .timeline__status-note', {
      hasText: 'Schema registry rejected payload',
    }).first();
    await expect(failureNote).toBeVisible();
    await expect(failureNote).toContainText(/Replay queued/);

    await page.locator('#history').getByRole('button', { name: 'Refresh' }).click();

    await expect(page.locator('#history .timeline__status-note')).toHaveCount(0);

    await page.reload();

    await expect(page.locator('#overview .timeline__status-note')).toHaveCount(0);
    await expect(page.locator('.timeline__pill--synced').first()).toBeVisible();
  });

  test('submits activity and observes reconciliation', async ({ page }) => {
    let listCalls = 0;
    const pendingActivity = {
      activity_id: 'act-new-1',
      tenant_id: 'tenant-demo',
      user_id: 'user-1',
      activity_type: 'Swim',
      started_at: '2025-10-28T09:00:00.000Z',
      duration_min: 40,
      source: 'web-ui',
      status: 'pending',
      version: 'v1',
      created_at: '2025-10-28T09:00:00.000Z',
      updated_at: '2025-10-28T09:00:00.000Z',
    };
    const syncedActivity = {
      ...pendingActivity,
      status: 'synced' as const,
      version: 'v2',
      updated_at: '2025-10-28T09:05:00.000Z',
    };

    const listResponses = [
      { items: [] },
      { items: [pendingActivity] },
      { items: [syncedActivity] },
      { items: [syncedActivity] },
    ];

    await page.route('**/v1/activities?**', async (route) => {
      const index = Math.min(listCalls, listResponses.length - 1);
      const payload = listResponses[index];
      listCalls += 1;
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(payload) });
    });

    await page.route('**/v1/activities', async (route) => {
      if (route.request().method() !== 'POST') {
        await route.fallback();
        return;
      }
      const body = await route.request().postDataJSON();
      expect(body.user_id).toBe('user-1');
      expect(body.activity_type).toBe('Swim');
      await route.fulfill({
        status: 202,
        contentType: 'application/json',
        body: JSON.stringify({
          activity_id: 'act-new-1',
          status: 'pending',
          idempotent_replay: false,
        }),
      });
    });

    await page.goto('/');

    await expect(page.getByRole('heading', { name: 'Activity Overview' })).toBeVisible();

    await page.getByLabel('Activity type').fill('Swim');
    await page.getByLabel('Duration (minutes)').fill('40');
    const startTimeInput = page.getByLabel('Start time');
    await startTimeInput.fill('2025-10-28T09:00');
    await page.getByLabel('Source').fill('web-ui');

    await page.getByRole('button', { name: 'Submit' }).click();

    await expect(page.getByText('Activity submitted!')).toBeVisible();

    await page.locator('#history').getByRole('button', { name: 'Refresh' }).click();

    await expect(page.locator('#history .timeline__pill--pending')).toHaveCount(0);
    await expect(page.locator('#history .timeline__pill--synced').first()).toBeVisible();
    await expect(page.locator('#history .timeline__title', { hasText: 'Swim' })).toBeVisible();
  });
});
