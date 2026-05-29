/**
 * seed-traffic.js — 生成真实流量，触发 LogMonitor SDK 上报
 * 最小化版本：只访问页面、等待、关闭
 */
const { chromium } = require('playwright');

async function main() {
  console.log('[SeedTraffic] Starting...');
  const browser = await chromium.launch({ headless: true });
  const ctx = await browser.newContext({
    viewport: { width: 1920, height: 1080 },
  });
  const page = await ctx.newPage();

  try {
    // Visit Vault Reader (has LogMonitor SDK integrated)
    console.log('[SeedTraffic] Visiting Vault Reader...');
    await page.goto('http://127.0.0.1/vault/', { waitUntil: 'domcontentloaded', timeout: 15000 });
    await new Promise(r => setTimeout(r, 3000));

    // Trigger console.error to generate error-level log
    await page.evaluate(() => {
      console.error('[LogMonitor-Test] Simulated error from seed-traffic');
      console.warn('[LogMonitor-Test] Simulated warning');
    });

    // Click first real link
    const links = await page.locator('a[href]:not([href="#"])').all();
    for (let i = 0; i < Math.min(links.length, 2); i++) {
      try {
        const href = await links[i].getAttribute('href');
        if (href && !href.startsWith('#') && !href.startsWith('javascript')) {
          console.log(`[SeedTraffic] Navigating to: ${href}`);
          await page.goto('http://127.0.0.1' + href, { timeout: 10000, waitUntil: 'domcontentloaded' });
          await new Promise(r => setTimeout(r, 2000));
        }
      } catch (_) {}
    }

    // Flush SDK
    await page.evaluate(() => {
      if (window.LogMonitor) window.LogMonitor.flush();
    }).catch(() => {});

    // Wait for buffer
    console.log('[SeedTraffic] Waiting 6s for SDK flush...');
    await new Promise(r => setTimeout(r, 6000));

    // Close triggers beforeunload → recording end
    await ctx.close();
    await browser.close();
    console.log('[SeedTraffic] Done!');
  } catch (e) {
    console.error(`[SeedTraffic] Error: ${e.message}`);
    await browser.close().catch(() => {});
  }
}

main();
