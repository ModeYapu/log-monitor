
        
        interface StepResult { step: string; passed: boolean; details: string; }

async function run(page: any, baseUrl: string): Promise<StepResult[]> {
  const results: StepResult[] = [];
  const pages = [
    '/',
    '/logs',
    '/performance',
    '/alerts',
    '/live',
    '/recordings',
    '/settings',
    '/users'
  ];

  for (const path of pages) {
    const step = `Navigate to ${path}`;
    try {
      const response = await page.goto(`${baseUrl}${path}`, { waitUntil: 'domcontentloaded', timeout: 15000 });
      const status = response ? response.status() : 'no response';
      if (status === 200) {
        results.push({ step, passed: true, details: `Status ${status}` });
      } else {
        results.push({ step, passed: false, details: `Expected 200, got ${status}` });
      }
    } catch (err: any) {
      results.push({ step, passed: false, details: err.message || String(err) });
    }
  }

  return results;
}

        module.exports = { run };
      