import { test, expect } from '@playwright/test';

const STORAGE_KEY = 'i5e-auth';

const rideResults = {
  items: [
    {
      id: 'ride-1',
      name: 'Cadence Ride',
      difficulty: 'intermediate',
      targets: ['cardio', 'endurance'],
    },
  ],
};

const squatResults = {
  items: [
    {
      id: 'squat-1',
      name: 'Air Squat',
      difficulty: 'beginner',
      targets: ['legs'],
    },
  ],
};

test.describe('Ontology insights', () => {
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

    await page.route('**/v1/exercises?**', async (route) => {
      const url = new URL(route.request().url());
      const query = url.searchParams.get('query') ?? '';
      let body = { items: [] as typeof rideResults.items };
      if (query.toLowerCase() === 'ride') body = rideResults;
      else if (query.toLowerCase() === 'squat') body = squatResults;
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) });
    });

    await page.route('**/v1/exercises', async (route) => {
      if (route.request().method() !== 'POST') {
        await route.fallback();
        return;
      }
      const payload = await route.request().postDataJSON();
      squatResults.items.push({
        id: 'squat-2',
        name: payload.name,
        difficulty: payload.difficulty,
        targets: payload.targets,
      });
      await route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({ id: 'squat-2' }),
      });
    });

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
  });

  test('shows insights and search results', async ({ page }) => {
    await page.goto('/');

    const insights = page.locator('.insights-list li');
    await expect(insights.first()).toContainText('Cadence Ride');

    await page.getByLabel('Search term').fill('squat');
    await expect(page.locator('#ontology .list li').first()).toContainText('Air Squat');
    await expect(page.locator('.insights-list li').first()).toContainText('Air Squat');
  });

  test('adds a new exercise and refetches search results', async ({ page }) => {
    await page.goto('/');

    await page.getByLabel('Search term').fill('squat');
    await expect(page.locator('#ontology .list li')).toContainText('Air Squat');

    const ontologyPanel = page.locator('#ontology');
    await ontologyPanel.getByLabel('Name').fill('Explosive Squat');
    await ontologyPanel.getByLabel('Difficulty').fill('advanced');
    await ontologyPanel.getByLabel('Targets (comma-separated)').fill('legs,power');
    await page.getByRole('button', { name: 'Save Exercise' }).click();

    await expect(page.getByText('Exercise saved.')).toBeVisible();
    await expect(page.locator('#ontology .list li', { hasText: 'Explosive Squat' })).toBeVisible();
  });
});
