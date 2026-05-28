import { test, expect } from '@playwright/test';

const BASE = 'http://127.0.0.1/logmon';

test.describe('Logs Page', () => {
  test('displays log table with data', async ({ page }) => {
    await page.goto(`${BASE}/logs`);
    await page.waitForLoadState('networkidle');
    
    const table = page.locator('.el-table');
    await expect(table).toBeVisible({ timeout: 10000 });
    
    const selects = page.locator('.el-select');
    expect(await selects.count()).toBeGreaterThanOrEqual(1);
    
    await expect(page.getByRole('button', { name: /搜索|查询/i })).toBeVisible();
    await expect(page.getByRole('button', { name: /重置/i })).toBeVisible();
  });
});
