import { test, expect } from '@playwright/test';

const BASE = 'http://127.0.0.1/logmon';

test.describe('Overview Page', () => {
  test('displays stat cards', async ({ page }) => {
    await page.goto(`${BASE}/`);
    await page.waitForLoadState('networkidle');
    
    const statCards = page.locator('.stat-card, [class*="stat"]');
    await expect(statCards.first()).toBeVisible({ timeout: 10000 });
  });

  test('displays app table', async ({ page }) => {
    await page.goto(`${BASE}/`);
    await page.waitForLoadState('networkidle');
    
    const table = page.locator('.el-table, table');
    await expect(table.first()).toBeVisible({ timeout: 10000 });
  });
});
