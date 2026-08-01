import { expect, test } from '@playwright/test';

test('home page greets with Hello world', async ({ page }) => {
  await page.goto('/');
  await expect(page.locator('h1')).toHaveText('Hello world');
});
