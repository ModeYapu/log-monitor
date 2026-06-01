
        
        interface StepResult { step: string; passed: boolean; details: string; }

async function run(page: any, baseUrl: string): Promise<StepResult[]> {
  const results: StepResult[] = [];
  const http = require('http');

  // Step 1: Navigate to recordings page
  try {
    await page.goto(`${baseUrl}/recordings`, { waitUntil: 'networkidle' });
    results.push({ step: 'navigate_to_recordings', passed: true, details: '成功导航到 /recordings 页面' });
  } catch (e: any) {
    results.push({ step: 'navigate_to_recordings', passed: false, details: e.message });
  }

  // Step 2: verify_table_not_empty - target=.el-table expect=录制表格有数据
  try {
    await page.waitForSelector('.el-table', { timeout: 15000 });
    // Wait a bit for data to render
    await page.waitForTimeout(2000);
    const emptyBlock = await page.$('.el-table__empty-block');
    const rows = await page.$$('.el-table__body-wrapper .el-table__row');
    const hasEmptyBlock = emptyBlock !== null;
    const hasRows = rows.length > 0;
    const passed = hasRows && !hasEmptyBlock;
    results.push({
      step: 'verify_table_not_empty',
      passed: passed,
      details: passed ? '录制表格有数据' : `表格行数: ${rows.length}, 空状态块: ${hasEmptyBlock}`
    });
  } catch (e: any) {
    results.push({ step: 'verify_table_not_empty', passed: false, details: e.message });
  }

  // Step 3 & 4: API call to /query/recordings?limit=3 and validate
  let apiData: any = null;
  try {
    apiData = await new Promise<any>((resolve, reject) => {
      const options = {
        hostname: '127.0.0.1',
        port: 9200,
        path: '/api/query/recordings?limit=3',
        method: 'GET',
        timeout: 15000
      };
      const req = http.request(options, (res: any) => {
        let body = '';
        res.on('data', (chunk: string) => { body += chunk; });
        res.on('end', () => {
          try {
            resolve(JSON.parse(body));
          } catch (parseErr: any) {
            reject(new Error('JSON解析失败: ' + parseErr.message));
          }
        });
      });
      req.on('error', reject);
      req.on('timeout', () => { req.destroy(); reject(new Error('API请求超时')); });
      req.end();
    });
  } catch (e: any) {
    results.push({ step: 'verify_api_response', passed: false, details: e.message });
    results.push({ step: 'api_check_recordings_length', passed: false, details: e.message });
    return results;
  }

  // Step 3: verify_api_response - expect=录制有 sessionId
  try {
    const data = apiData.data;
    const hasData = Array.isArray(data) && data.length > 0;
    let allHaveSessionId = false;
    if (hasData) {
      allHaveSessionId = data.every((item: any) => item.sessionId !== undefined && item.sessionId !== null && item.sessionId !== '');
      const sampleIds = data.map((item: any) => item.sessionId).join(', ');
      results.push({
        step: 'verify_api_response',
        passed: allHaveSessionId,
        details: allHaveSessionId ? '录制有 sessionId' : `部分录制缺少sessionId, 样本: ${sampleIds}`
      });
    } else {
      results.push({
        step: 'verify_api_response',
        passed: false,
        details: 'API返回数据为空，无法验证sessionId'
      });
    }
  } catch (e: any) {
    results.push({ step: 'verify_api_response', passed: false, details: e.message });
  }

  // Step 4: api_check - /query/recordings?limit=3 → data.length > 0
  try {
    const data = apiData.data;
    const hasData = Array.isArray(data) && data.length > 0;
    results.push({
      step: 'api_check_recordings_length',
      passed: hasData,
      details: hasData ? `API返回录制数量: ${data.length}，大于0` : `API返回数据长度不大于0，实际: ${Array.isArray(data) ? data.length : '非数组'}`
    });
  } catch (e: any) {
    results.push({ step: 'api_check_recordings_length', passed: false, details: e.message });
  }

  return results;
}

        module.exports = { run };
      