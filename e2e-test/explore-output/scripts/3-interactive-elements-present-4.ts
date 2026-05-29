import { chromium } from '@playwright/test';

async function main() {
  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage();
  try {

    // --- Navigate to target page ---
    await page.goto(`http://127.0.0.1/logmon/login`, { waitUntil: 'networkidle', timeout: 15000 });
    console.log('Testing: 3 interactive elements present');

    // Check interactive elements count
    const buttons = await page.locator('button').count();
    const inputs = await page.locator('input, select, textarea').count();
    const links = await page.locator('a').count();
    const total = buttons + inputs + links;
    console.log('INFO: Buttons=' + buttons + ' Inputs=' + inputs + ' Links=' + links + ' Total=' + total);
    if (total < 1) {
      console.log('FAIL: Expected at least 1 interactive elements, found ' + total);
      process.exit(1);
    }

    console.log('PASS: 3 interactive elements present');
  } finally {
    await browser.close();
  }
}

main().catch(e => { console.error('FAIL:', e.message); process.exit(1); });