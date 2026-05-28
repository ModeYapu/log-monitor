import { test, expect } from '@playwright/test';

const BASE = 'http://127.0.0.1/logmon';

test.describe('User Management', () => {
  test('displays user list with admin', async ({ page }) => {
    await page.goto(`${BASE}/users`);
    await page.waitForLoadState('networkidle');
    
    const table = page.locator('.el-table');
    await expect(table).toBeVisible({ timeout: 10000 });
    
    const rows = table.locator('.el-table__body-wrapper .el-table__row');
    await expect(rows.first()).toBeVisible({ timeout: 5000 });
    
    const rowText = await rows.first().innerText();
    expect(rowText).toContain('admin');
    
    await page.screenshot({ path: 'e2e/screenshots/users.png', fullPage: true });
  });
});
