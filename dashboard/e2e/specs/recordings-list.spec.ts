import { test, expect } from '@playwright/test';

const BASE = 'http://127.0.0.1/logmon';

test.describe('Recordings List', () => {
  test('displays recordings table with correct data', async ({ page }) => {
    await page.goto(`${BASE}/recordings`);
    await page.waitForLoadState('networkidle');
    
    const table = page.locator('.el-table');
    await expect(table).toBeVisible({ timeout: 15000 });
    
    const rows = table.locator('.el-table__body-wrapper .el-table__row');
    await expect(rows.first()).toBeVisible({ timeout: 10000 });
    
    const rowCount = await rows.count();
    console.log(`Found ${rowCount} recording rows`);
    expect(rowCount).toBeGreaterThan(0);
    
    await page.screenshot({ path: 'e2e/screenshots/recordings-list.png', fullPage: true });
  });

  test('time fields display correctly (not undefined/raw timestamp)', async ({ page }) => {
    await page.goto(`${BASE}/recordings`);
    await page.waitForLoadState('networkidle');
    
    const table = page.locator('.el-table');
    await expect(table).toBeVisible({ timeout: 15000 });
    
    const rows = table.locator('.el-table__body-wrapper .el-table__row');
    await expect(rows.first()).toBeVisible({ timeout: 10000 });
    
    const bodyText = await page.locator('body').innerText();
    
    // Should not show "undefined" or "NaN"
    expect(bodyText).not.toContain('undefined');
    expect(bodyText).not.toContain('NaN');
    
    // Should have readable date/time
    const hasReadableTime = /\d{4}[-\/]\d{2}[-\/]\d{2}/.test(bodyText) || 
                           /\d{1,2}:\d{2}/.test(bodyText);
    expect(hasReadableTime).toBe(true);
  });

  test('recording data fields are populated', async ({ page }) => {
    await page.goto(`${BASE}/recordings`);
    await page.waitForLoadState('networkidle');
    
    const table = page.locator('.el-table');
    await expect(table).toBeVisible({ timeout: 15000 });
    
    const rows = table.locator('.el-table__body-wrapper .el-table__row');
    await expect(rows.first()).toBeVisible({ timeout: 10000 });
    
    const firstRowText = await rows.first().innerText();
    
    // Should have sessionId (UUID format)
    expect(firstRowText).toMatch(/[a-f0-9-]{36}/);
    
    // Should have status (Chinese or English)
    expect(firstRowText).toMatch(/completed|recording|error|已完成|录制中|错误/i);
  });
});
