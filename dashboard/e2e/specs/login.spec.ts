import { test, expect } from '@playwright/test';

// Login tests need fresh browser (no stored auth)
test.use({ storageState: { cookies: [], origins: [] } });

test.describe('Login Page', () => {
  test('shows login form', async ({ page }) => {
    await page.goto('http://127.0.0.1/logmon/login');
    await page.waitForLoadState('networkidle');
    
    // Should have the login form
    const form = page.locator('.el-form').first();
    await expect(form).toBeVisible({ timeout: 10000 });
    
    // Take screenshot
    await page.screenshot({ path: 'e2e/screenshots/login-form.png', fullPage: true });
    
    // Should have text input and password input
    const inputs = form.locator('input');
    expect(await inputs.count()).toBeGreaterThanOrEqual(2);
    
    // Login button should exist
    await expect(form.locator('button').filter({ hasText: /登录/i })).toBeVisible();
  });

  test('shows error on wrong credentials', async ({ page }) => {
    await page.goto('http://127.0.0.1/logmon/login');
    await page.waitForLoadState('networkidle');
    
    const form = page.locator('.el-form').first();
    await form.locator('input:not([type="password"])').first().fill('admin');
    await form.locator('input[type="password"]').first().fill('wrongpassword');
    await form.locator('button').filter({ hasText: /登录/i }).first().click();
    
    await page.waitForTimeout(3000);
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).toMatch(/错误|失败|invalid|incorrect|密码/i);
  });

  test('login page is fullscreen - no sidebar', async ({ page }) => {
    await page.goto('http://127.0.0.1/logmon/login');
    await page.waitForLoadState('networkidle');
    
    // No navigation sidebar visible
    const menuItems = page.locator('.el-menu-item');
    expect(await menuItems.count()).toBe(0);
  });

  test('successful login redirects to dashboard', async ({ page }) => {
    await page.goto('http://127.0.0.1/logmon/login');
    await page.waitForLoadState('networkidle');
    
    const form = page.locator('.el-form').first();
    await form.locator('input:not([type="password"])').first().fill('admin');
    await form.locator('input[type="password"]').first().fill('admin123');
    await form.locator('button').filter({ hasText: /登录/i }).first().click();
    
    await page.waitForURL(/\/logmon\/(?!login)/, { timeout: 10000 });
    
    const token = await page.evaluate(() => localStorage.getItem('logmon_token'));
    expect(token).toBeTruthy();
  });
});
