import { test, expect } from '@playwright/test';
import type { Page } from '@playwright/test';

const STORAGE_KEY = 'i5e-auth';

async function seedStorage(page: Page, overrides: Record<string, unknown> = {}) {
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
      ...overrides,
    }),
  });
}

test.describe('Error handling', () => {
  test('recovers from activity list failure', async ({ page }) => {
    await seedStorage(page);

    await page.route('**/v1/activities/metrics?**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          summary: {
            total: 0,
            pending: 0,
            synced: 0,
            failed: 0,
            average_duration_minutes: 0,
            average_processing_seconds: 0,
            oldest_pending_age_seconds: 0,
            success_rate: 0,
          },
          timeline: [],
          timeline_limit: 6,
          window_seconds: 86_400,
        }),
      });
    });

    let callCount = 0;
    await page.route('**/v1/activities?**', async (route) => {
      callCount += 1;
      if (callCount === 1) {
        await route.fulfill({
          status: 500,
          contentType: 'application/json',
          body: JSON.stringify({ error: 'boom' }),
        });
      } else {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ items: [] }),
        });
      }
    });

    await page.goto('/');

    await expect(page.locator('.empty-state--error', { hasText: 'Unable to load recent activities.' })).toBeVisible();
    await page.getByRole('button', { name: /Try again/i }).click();
    await expect(page.getByText('No activities recorded yet. Use the form above to create one.')).toBeVisible();
  });

  test('recovers from ontology search failure', async ({ page }) => {
    await seedStorage(page);

    await page.route('**/v1/activities/metrics?**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          summary: {
            total: 0,
            pending: 0,
            synced: 0,
            failed: 0,
            average_duration_minutes: 0,
            average_processing_seconds: 0,
            oldest_pending_age_seconds: 0,
            success_rate: 0,
          },
          timeline: [],
          timeline_limit: 6,
          window_seconds: 86_400,
        }),
      });
    });

    let searchCalls = 0;
    await page.route('**/v1/exercises?**', async (route) => {
      searchCalls += 1;
      if (searchCalls <= 2) {
        await route.fulfill({
          status: 503,
          contentType: 'application/json',
          body: JSON.stringify({ error: 'unavailable' }),
        });
      } else {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            items: [
              { id: 'ride-1', name: 'Cadence Ride', difficulty: 'intermediate', targets: ['cardio'] },
            ],
          }),
        });
      }
    });

    await page.goto('/');

    await expect(page.locator('.empty-state--error', { hasText: 'Unable to fetch ontology results.' })).toBeVisible();
    await page.getByRole('button', { name: /Retry search/i }).click();
    await expect(page.getByText('Cadence Ride')).toBeVisible();
  });
});
