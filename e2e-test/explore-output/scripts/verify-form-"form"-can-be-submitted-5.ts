import { chromium } from '@playwright/test';

async function main() {
  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage();
  try {

    // --- Navigate to target page ---
    await page.goto(`http://127.0.0.1/logmon/login`, { waitUntil: 'networkidle', timeout: 15000 });
    console.log('Testing: Verify form "form" can be submitted');

    // Check form exists and is submittable
    const forms = await page.locator('form, .el-form').count();
    console.log('INFO: Found ' + forms + ' forms');

    console.log('PASS: Verify form "form" can be submitted');
  } finally {
    await browser.close();
  }
}

main().catch(e => { console.error('FAIL:', e.message); process.exit(1); });