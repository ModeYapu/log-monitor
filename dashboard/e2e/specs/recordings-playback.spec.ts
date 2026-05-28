import { test, expect } from '@playwright/test';

const BASE = 'http://127.0.0.1/logmon';
const API_BASE = 'http://127.0.0.1/logmon-api';

test.describe('Recordings Playback', () => {
  test('can open recording player', async ({ page }) => {
    await page.goto(`${BASE}/recordings`);
    await page.waitForLoadState('networkidle');
    
    const table = page.locator('.el-table');
    await expect(table).toBeVisible({ timeout: 15000 });
    const rows = table.locator('.el-table__body-wrapper .el-table__row');
    await expect(rows.first()).toBeVisible({ timeout: 10000 });
    
    // Click play/view button on first row
    const playBtn = rows.first().locator('button').filter({ hasText: /播放|回放|查看|play|view/i }).first();
    if (await playBtn.count() > 0) {
      await playBtn.click();
    } else {
      // Try any button in the row
      const anyBtn = rows.first().locator('button').first();
      if (await anyBtn.count() > 0) {
        await anyBtn.click();
      }
    }
    
    await page.waitForTimeout(3000);
    await page.screenshot({ path: 'e2e/screenshots/recordings-playback.png', fullPage: true });
  });

  test('recording API returns correct data format', async ({ page }) => {
    // Navigate to recordings page first to access localStorage
    await page.goto('http://127.0.0.1/logmon/recordings');
    await page.waitForLoadState('networkidle');
    
    // Get token from localStorage
    const token = await page.evaluate(() => localStorage.getItem('logmon_token'));
    expect(token).toBeTruthy();
    
    // Fetch recordings list
    const listResp = await page.request.get(`${API_BASE}/query/recordings?limit=1`, {
      headers: { Authorization: `Bearer ${token}` }
    });
    expect(listResp.ok()).toBe(true);
    
    const listData = await listResp.json();
    expect(listData.data).toBeDefined();
    expect(listData.data.length).toBeGreaterThan(0);
    
    const recording = listData.data[0];
    const sessionId = recording.sessionId;
    
    console.log(`Testing recording: ${sessionId}, events: ${recording.eventCount}`);
    
    // Verify camelCase field names
    expect(recording).toHaveProperty('sessionId');
    expect(recording).toHaveProperty('startTime');
    expect(recording).toHaveProperty('durationMs');
    expect(recording).toHaveProperty('appId');
    expect(recording).toHaveProperty('status');
    
    // Fetch events
    const eventsResp = await page.request.get(
      `${API_BASE}/query/recordings/${sessionId}?events=true&limit=10`,
      { headers: { Authorization: `Bearer ${token}` } }
    );
    expect(eventsResp.ok()).toBe(true);
    
    const eventsData = await eventsResp.json();
    expect(eventsData.events).toBeDefined();
    expect(eventsData.events.length).toBeGreaterThan(0);
    
    // Verify event data has camelCase fields
    const event = eventsData.events[0];
    expect(event).toHaveProperty('sessionId');
    expect(event).toHaveProperty('eventData');
    expect(event).toHaveProperty('timestamp');
    expect(event).toHaveProperty('seq');
    
    // eventData should be non-empty valid JSON
    expect(typeof event.eventData).toBe('string');
    expect(event.eventData.length).toBeGreaterThan(0);
    
    const parsed = JSON.parse(event.eventData);
    expect(parsed).toHaveProperty('type');
    
    console.log(`✓ Event data valid: type=${parsed.type}, data_keys=${Object.keys(parsed.data || {}).slice(0, 5).join(',')}`);
  });
});
