import { test as setup, expect } from '@playwright/test';

const authFile = 'e2e/.auth/admin.json';

setup('authenticate', async ({ page }) => {
  // Use full URL to avoid baseURL path resolution issues
  await page.goto('http://127.0.0.1/logmon/login');
  await page.waitForLoadState('networkidle');
  
  await page.screenshot({ path: 'e2e/screenshots/setup-login-page.png', fullPage: true });
  
  // Find the login form - LogMonitor uses Element Plus
  const form = page.locator('.el-form').first();
  await expect(form).toBeVisible({ timeout: 10000 });
  
  const usernameInput = form.locator('input:not([type="password"])').first();
  const passwordInput = form.locator('input[type="password"]').first();
  
  await usernameInput.fill('admin');
  await passwordInput.fill('admin123');
  
  // Click login button
  await form.locator('button').filter({ hasText: /登录|login/i }).first().click();
  
  // Wait for redirect away from login
  await page.waitForURL(/\/logmon\/(?!login)/, { timeout: 10000 });
  
  // Verify token
  const token = await page.evaluate(() => localStorage.getItem('logmon_token'));
  expect(token).toBeTruthy();
  
  await page.context().storageState({ path: authFile });
});
