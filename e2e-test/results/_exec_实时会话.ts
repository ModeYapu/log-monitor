
        
        interface StepResult { step: string; passed: boolean; details: string; }

async function run(page: any, baseUrl: string): Promise<StepResult[]> {
  const results: StepResult[] = [];

  // Step 1: Navigate to the live sessions page
  try {
    await page.goto(`${baseUrl}/live`, { waitUntil: 'domcontentloaded', timeout: 15000 });
    await page.waitForTimeout(2000);
    results.push({ step: 'navigate_to_live_page', passed: true, details: 'Successfully navigated to /live page' });
  } catch (e: any) {
    results.push({ step: 'navigate_to_live_page', passed: false, details: `Failed to navigate to /live: ${e.message}` });
  }

  // Step 2: Verify page structure - check for online users section/element
  try {
    const bodyText = await page.evaluate(() => document.body.innerText);
    const hasOnlineUsers = bodyText.toLowerCase().includes('online user') || bodyText.toLowerCase().includes('在线用户') || bodyText.toLowerCase().includes('online');
    // Also check for DOM elements that might represent online users
    const onlineUserElements = await page.evaluate(() => {
      const allElements = document.querySelectorAll('*');
      let found = false;
      for (const el of allElements) {
        const text = el.textContent?.toLowerCase() || '';
        const id = (el.id || '').toLowerCase();
        const cls = (el.className || '').toString().toLowerCase();
        if (text.includes('online user') || text.includes('在线用户') || id.includes('online') || cls.includes('online')) {
          found = true;
          break;
        }
      }
      return found;
    });
    const passed = hasOnlineUsers || onlineUserElements;
    results.push({
      step: 'verify_online_users_presence',
      passed,
      details: passed ? 'Found "online users" related content on the page' : 'Could not find "online users" related content on the page'
    });
  } catch (e: any) {
    results.push({ step: 'verify_online_users_presence', passed: false, details: `Error checking for online users: ${e.message}` });
  }

  // Step 3: Verify page structure - check for viewer section/element
  try {
    const bodyText = await page.evaluate(() => document.body.innerText);
    const hasViewer = bodyText.toLowerCase().includes('viewer') || bodyText.toLowerCase().includes('观看者') || bodyText.toLowerCase().includes('查看');
    // Also check for DOM elements that might represent viewer
    const viewerElements = await page.evaluate(() => {
      const allElements = document.querySelectorAll('*');
      let found = false;
      for (const el of allElements) {
        const text = el.textContent?.toLowerCase() || '';
        const id = (el.id || '').toLowerCase();
        const cls = (el.className || '').toString().toLowerCase();
        if (text.includes('viewer') || text.includes('观看者') || id.includes('viewer') || cls.includes('viewer')) {
          found = true;
          break;
        }
      }
      return found;
    });
    const passed = hasViewer || viewerElements;
    results.push({
      step: 'verify_viewer_presence',
      passed,
      details: passed ? 'Found "viewer" related content on the page' : 'Could not find "viewer" related content on the page'
    });
  } catch (e: any) {
    results.push({ step: 'verify_viewer_presence', passed: false, details: `Error checking for viewer: ${e.message}` });
  }

  // Step 4: Verify overall page structure has key layout elements (header, main content area)
  try {
    const structureInfo = await page.evaluate(() => {
      const header = document.querySelector('header, [class*="header"], [id*="header"]');
      const main = document.querySelector('main, [class*="main"], [id*="main"], [class*="content"], [id*="content"]');
      const sidebar = document.querySelector('aside, [class*="sidebar"], [id*="sidebar"], [class*="panel"], [id*="panel"]');
      return {
        hasHeader: !!header,
        hasMain: !!main,
        hasSidebar: !!sidebar,
        title: document.title
      };
    });
    const hasBasicStructure = structureInfo.hasMain;
    results.push({
      step: 'verify_page_structure',
      passed: hasBasicStructure,
      details: `Page title: "${structureInfo.title}", Header: ${structureInfo.hasHeader}, Main: ${structureInfo.hasMain}, Sidebar: ${structureInfo.hasSidebar}`
    });
  } catch (e: any) {
    results.push({ step: 'verify_page_structure', passed: false, details: `Error verifying page structure: ${e.message}` });
  }

  return results;
}

        module.exports = { run };
      