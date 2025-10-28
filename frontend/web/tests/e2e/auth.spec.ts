import { test, expect } from '@playwright/test';

const STORAGE_KEY = 'i5e-auth';

const initialToken = {
  access_token: 'access-one',
  expires_in: 65,
  token_type: 'bearer',
  refresh_token: 'refresh-one',
  refresh_expires_in: 3600,
  tenant_id: 'tenant-demo',
};

const refreshedToken = {
  access_token: 'access-two',
  expires_in: 300,
  token_type: 'bearer',
  refresh_token: 'refresh-two',
  refresh_expires_in: 3600,
  tenant_id: 'tenant-demo',
};

const defaultMetricsResponse = {
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
};

test.describe('Authentication workflow', () => {
  test.beforeEach(async ({ page }) => {
    await page.route('**/v1/activities/metrics?**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(defaultMetricsResponse),
      });
    });
  });

  test('requests and refreshes tokens automatically', async ({ page }) => {
    await page.addInitScript(({ key }) => {
      window.localStorage.removeItem(key);
    }, { key: STORAGE_KEY });

    let issueCallCount = 0;
    await page.route('**/v1/token', async (route) => {
      issueCallCount += 1;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(initialToken),
      });
    });

    let refreshCallCount = 0;
    await page.route('**/v1/token/refresh', async (route) => {
      refreshCallCount += 1;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(refreshedToken),
      });
    });

    await page.goto('/');

    await page.getByLabel('Tenant ID').fill('tenant-demo');
    await page.getByLabel('Account ID').fill('account-demo');
    await page.getByRole('button', { name: 'Request Token' }).click();

    await expect(page.getByText('Token issued.')).toBeVisible();
    const textarea = page.locator('textarea').first();
    await expect(textarea).toHaveValue(/access-one/);

    const refreshButton = page.locator('.session-banner__button', { hasText: /Refresh/ }).first();
    await expect(refreshButton).toBeVisible();

    const refreshRequest = page.waitForRequest('**/v1/token/refresh');
    await refreshRequest;

    await expect(textarea).toHaveValue(/access-two/);
    expect(issueCallCount).toBe(1);
    expect(refreshCallCount).toBeGreaterThanOrEqual(1);

    const storage = await page.evaluate((key) => window.localStorage.getItem(key), STORAGE_KEY);
    expect(storage).toContain('refresh-two');
  });

  test('shows error when refresh fails and clears session', async ({ page }) => {
    await page.addInitScript(({ key, payload }) => {
      window.localStorage.setItem(key, payload);
    }, {
      key: STORAGE_KEY,
      payload: JSON.stringify({
        token: 'stale-token',
        tenantId: 'tenant-demo',
        accountId: 'account-demo',
        userId: 'user-1',
        remember: true,
        refreshToken: 'stale-refresh',
        accessExpiresAt: Date.now() + 500,
        refreshExpiresAt: Date.now() + 60_000,
        scopes: ['activities:read'],
      }),
    });

    await page.route('**/v1/token/refresh', async (route) => {
      await route.fulfill({
        status: 400,
        contentType: 'application/json',
        body: JSON.stringify({ detail: 'invalid refresh token' }),
      });
    });

    await page.goto('/');

    await page.waitForTimeout(1500);

    await expect(page.locator('text=Session expired. Please reauthenticate.').first()).toBeVisible();
    await expect(page.locator('textarea').first()).toHaveValue('');
    const reauthButton = page.getByRole('button', { name: /Reauthenticate/i });
    await expect(reauthButton).toBeVisible();
    await reauthButton.click();
    await expect(page.getByText('Session expired. Please reauthenticate.')).toHaveCount(0);
  });
});
