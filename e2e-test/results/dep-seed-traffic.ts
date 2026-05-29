/**
 * seed-traffic.ts — 生成真实流量，触发 LogMonitor SDK 上报和 rrweb 录制
 * 被 test-plan.yaml 的 dependencies 引用
 * 
 * 真实流程：
 * 1. Playwright 打开 Vault Reader（已集成 LogMonitor SDK）
 * 2. 执行真实交互（搜索、点击链接、滚动）
 * 3. SDK 自动上报事件到 LogMonitor
 * 4. rrweb 自动录制 DOM 变化
 * 5. 关闭浏览器 → 触发 beforeunload → SDK flush + recording end
 */

import { chromium } from 'playwright';

const VAULT_URL = 'http://127.0.0.1/vault/';

async function main() {
  console.log('[SeedTraffic] Starting traffic generation...');

  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({
    viewport: { width: 1920, height: 1080 },
    userAgent: 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
  });

  const page = await context.newPage();

  // Track SDK messages
  const sdkMessages: string[] = [];
  page.on('console', msg => {
    const text = msg.text();
    if (text.includes('[LogMonitor]') || text.includes('[rrweb]')) {
      sdkMessages.push(text);
      console.log(`  [SDK] ${text}`);
    }
  });

  // Track network requests to LogMonitor
  const apiCalls: string[] = [];
  page.on('request', req => {
    if (req.url().includes('logmon-api') || req.url().includes('9200')) {
      apiCalls.push(`${req.method()} ${req.url().slice(-50)}`);
    }
  });

  try {
    // 1. Visit Vault Reader (triggers SDK init + rrweb recording start)
    console.log(`[SeedTraffic] Visiting: ${VAULT_URL}`);
    await page.goto(VAULT_URL, { waitUntil: 'networkidle', timeout: 30000 });
    await sleep(2000);

    // Check SDK loaded
    const sdkLoaded = await page.evaluate(() => typeof (window as any).LogMonitor !== 'undefined');
    console.log(`[SeedTraffic] SDK loaded: ${sdkLoaded}`);

    // 2. Click some links (generates navigation events + DOM mutations for rrweb)
    const links = await page.locator('a[href]').all();
    console.log(`[SeedTraffic] Found ${links.length} links`);

    for (let i = 0; i < Math.min(links.length, 3); i++) {
      try {
        const href = await links[i].getAttribute('href');
        if (href && !href.startsWith('javascript')) {
          console.log(`[SeedTraffic] Clicking link: ${href.slice(0, 40)}`);
          await links[i].click();
          await sleep(1500);
        }
      } catch (e) {
        // Link might not be clickable, continue
      }
    }

    // 3. Use search if available (generates user interaction events)
    try {
      const searchInput = page.locator('input[type="search"], input[placeholder*="搜索"], input[placeholder*="search"]').first();
      if (await searchInput.isVisible({ timeout: 2000 })) {
        console.log('[SeedTraffic] Typing in search...');
        await searchInput.fill('test query from e2e');
        await searchInput.press('Enter');
        await sleep(2000);
      }
    } catch (e) {
      // No search, skip
    }

    // 4. Scroll (generates scroll events)
    await page.evaluate(() => window.scrollTo(0, document.body.scrollHeight / 2));
    await sleep(1000);
    console.log('[SeedTraffic] Scrolled to middle');

    // 5. Click some buttons
    try {
      const buttons = await page.locator('button:visible').all();
      for (let i = 0; i < Math.min(buttons.length, 2); i++) {
        const text = await buttons[i].innerText().catch(() => '');
        console.log(`[SeedTraffic] Clicking button: "${text.slice(0, 20)}"`);
        await buttons[i].click({ timeout: 2000 }).catch(() => {});
        await sleep(500);
      }
    } catch (e) {
      // Skip
    }

    // 6. Wait for SDK to accumulate events (buffer interval ~5s)
    console.log('[SeedTraffic] Waiting 5s for SDK buffer flush...');
    await sleep(5000);

    // 7. Manually trigger flush
    try {
      await page.evaluate(() => {
        if (typeof (window as any).LogMonitor !== 'undefined') {
          (window as any).LogMonitor.flush();
          console.log('[SeedTraffic] Manual flush triggered');
        }
      });
    } catch (e) {
      // Skip
    }
    await sleep(1000);

    // 8. Close browser (triggers beforeunload → recording end + final flush)
    console.log('[SeedTraffic] Closing browser (triggers beforeunload)...');
    await context.close();
    await browser.close();

    // 9. Wait for backend processing
    console.log('[SeedTraffic] Waiting 3s for backend processing...');
    await sleep(3000);

    // Summary
    console.log(`[SeedTraffic] Done! SDK messages: ${sdkMessages.length}, API calls: ${apiCalls.length}`);
    for (const msg of sdkMessages) {
      console.log(`  SDK: ${msg.slice(0, 80)}`);
    }
    for (const call of apiCalls) {
      console.log(`  API: ${call}`);
    }

  } catch (e) {
    console.error(`[SeedTraffic] Error: ${e}`);
    await browser.close().catch(() => {});
  }
}

function sleep(ms: number): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms));
}

main();
