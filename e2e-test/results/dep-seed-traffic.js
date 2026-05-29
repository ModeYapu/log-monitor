/**
 * seed-traffic.js — 生成真实流量，触发 LogMonitor SDK 上报
 * 纯 JS 避免 TS 编译依赖问题
 */
const { chromium } = require('playwright');

const VAULT_URL = 'http://127.0.0.1/vault/';

async function main() {
  console.log('[SeedTraffic] Starting...');
  const browser = await chromium.launch({ headless: true });
  const ctx = await browser.newContext({
    viewport: { width: 1920, height: 1080 },
    userAgent: 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36',
  });
  const page = await ctx.newPage();

  const msgs = [];
  page.on('console', msg => {
    if (msg.text().includes('[LogMonitor]') || msg.text().includes('[rrweb]'))
      msgs.push(msg.text());
  });

  try {
    console.log(`[SeedTraffic] Visiting: ${VAULT_URL}`);
    await page.goto(VAULT_URL, { waitUntil: 'networkidle', timeout: 30000 });
    await sleep(2000);

    // Click links
    const links = await page.locator('a[href]').all();
    console.log(`[SeedTraffic] Found ${links.length} links`);
    for (let i = 0; i < Math.min(links.length, 3); i++) {
      try {
        const href = await links[i].getAttribute('href');
        if (href && !href.startsWith('javascript')) {
          console.log(`[SeedTraffic] Clicking: ${href.slice(0, 40)}`);
          await links[i].click();
          await sleep(1500);
        }
      } catch (_) {}
    }

    // Search
    try {
      const s = page.locator('input[type="search"], input[placeholder*="搜索"]').first();
      if (await s.isVisible({ timeout: 2000 })) {
        await s.fill('e2e test query');
        await s.press('Enter');
        await sleep(2000);
      }
    } catch (_) {}

    // Scroll
    await page.evaluate(() => window.scrollTo(0, document.body.scrollHeight / 2));
    await sleep(1000);

    // Wait for SDK buffer
    console.log('[SeedTraffic] Waiting 5s for SDK buffer...');
    await sleep(5000);

    // Manual flush
    await page.evaluate(() => {
      if (window.LogMonitor) { window.LogMonitor.flush(); console.log('[SeedTraffic] Flushed'); }
    }).catch(() => {});
    await sleep(1000);

    // Close (beforeunload → recording end)
    console.log('[SeedTraffic] Closing...');
    await ctx.close();
    await browser.close();
    await sleep(3000);

    console.log(`[SeedTraffic] Done! Messages: ${msgs.length}`);
  } catch (e) {
    console.error(`[SeedTraffic] Error: ${e.message}`);
    await browser.close().catch(() => {});
  }
}

function sleep(ms) { return new Promise(r => setTimeout(r, ms)); }

main();
