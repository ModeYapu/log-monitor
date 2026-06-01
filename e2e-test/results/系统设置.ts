interface StepResult { step: string; passed: boolean; details: string; }

async function run(page: any, baseUrl: string): Promise<StepResult[]> {
  const results: StepResult[] = [];
  const settingsUrl = `${baseUrl}/settings`;

  // Step 1: verify_forms_present - expect settings forms to exist
  try {
    await page.goto(settingsUrl, { waitUntil: 'networkidle' });
    await page.waitForTimeout(1000);

    const formCount = await page.locator('form').count();
    const inputCount = await page.locator('input, select, textarea').count();
    const formGroups = await page.locator('.form-group, .form-item, .el-form-item, .ant-form-item, [class*="form"]').count();

    if (formCount > 0 || inputCount > 0 || formGroups > 0) {
      results.push({
        step: 'verify_forms_present',
        passed: true,
        details: `设置表单存在 - found ${formCount} form(s), ${inputCount} input(s)/select(s)/textarea(s), ${formGroups} form group(s)`
      });
    } else {
      // Check for any interactive elements that might represent settings
      const checkboxes = await page.locator('input[type="checkbox"], [role="switch"], [class*="switch"]').count();
      const sliders = await page.locator('input[type="range"], [role="slider"]').count();
      if (checkboxes > 0 || sliders > 0) {
        results.push({
          step: 'verify_forms_present',
          passed: true,
          details: `设置表单存在 - found ${checkboxes} toggle(s)/switch(es), ${sliders} slider(s)`
        });
      } else {
        results.push({
          step: 'verify_forms_present',
          passed: false,
          details: `设置表单存在 - no forms or form elements found on ${settingsUrl}`
        });
      }
    }
  } catch (e: any) {
    results.push({
      step: 'verify_forms_present',
      passed: false,
      details: `设置表单存在 - error: ${e.message}`
    });
  }

  // Step 2: verify_save_button - expect save button to be visible
  try {
    const saveButtonSelectors = [
      'button:has-text("保存")',
      'button:has-text("保 存")',
      'button:has-text("Save")',
      'button:has-text("保存设置")',
      'button:has-text("确定")',
      'button:has-text("提 交")',
      'button:has-text("提交")',
      'input[type="submit"]',
      'button[type="submit"]',
      '.save-btn',
      '#save-btn',
      '.btn-save',
      '[class*="save"]',
      'button.el-button--primary',
      'button.ant-btn-primary'
    ];

    let saveButtonFound = false;
    let foundSelector = '';

    for (const selector of saveButtonSelectors) {
      const btn = page.locator(selector);
      const count = await btn.count();
      if (count > 0) {
        const firstBtn = btn.first();
        const isVisible = await firstBtn.isVisible().catch(() => false);
        if (isVisible) {
          saveButtonFound = true;
          foundSelector = selector;
          break;
        }
      }
    }

    if (saveButtonFound) {
      results.push({
        step: 'verify_save_button',
        passed: true,
        details: `保存按钮可见 - found using selector: ${foundSelector}`
      });
    } else {
      // Fallback: get all buttons text for debugging
      const allButtons = page.locator('button');
      const buttonCount = await allButtons.count();
      const buttonTexts: string[] = [];
      for (let i = 0; i < Math.min(buttonCount, 10); i++) {
        const text = await allButtons.nth(i).textContent().catch(() => '');
        if (text) buttonTexts.push(text.trim());
      }
      results.push({
        step: 'verify_save_button',
        passed: false,
        details: `保存按钮可见 - no save button found. Page has ${buttonCount} button(s): [${buttonTexts.join(', ')}]`
      });
    }
  } catch (e: any) {
    results.push({
      step: 'verify_save_button',
      passed: false,
      details: `保存按钮可见 - error: ${e.message}`
    });
  }

  return results;
}
