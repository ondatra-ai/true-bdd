import { expect, test } from '@playwright/test';

// The heading test fails on the planted defect in services/web/server.js;
// the link test passes. Both assertions read the page once rather than
// retrying, so a failure reports the same text on every run.
test.describe('home page', () => {
  test('shows the store name as its heading', async ({ page }) => {
    await page.goto('/');
    expect(await page.locator('h1').textContent()).toBe('Corner Shop');
  });

  test('links to the opening hours', async ({ page }) => {
    await page.goto('/');
    expect(await page.locator('a').getAttribute('href')).toBe('/hours');
  });
});
