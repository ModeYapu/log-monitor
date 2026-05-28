# SDD: E2E Testing Framework for LogMonitor

## Overview
为 LogMonitor 项目建立完整的 E2E 测试框架，使用 Playwright 自动化浏览器测试，验证：
1. 登录/认证流程
2. 日志查询/筛选
3. 录制列表展示（数据/时间格式）
4. 录制回放功能
5. 用户管理

## Tech Stack
- **Playwright** - 浏览器自动化
- **Node.js** - 测试运行器
- **TypeScript** - 测试代码

## 测试目标 URL
- Dashboard: `https://sanfacheng.cyou/logmon/`
- API Proxy: `https://sanfacheng.cyou/logmon-api/`

## 测试用例

### 1. Login Flow (`login.spec.ts`)
- 访问 /logmon/，未登录自动跳转 /logmon/login
- 输入 admin/admin123，点击登录
- 登录成功跳转到首页（Overview）
- 验证 token 存储在 localStorage
- 错误密码提示"用户名或密码错误"

### 2. Overview Page (`overview.spec.ts`)
- 登录后访问首页
- 验证统计卡片显示（总事件数、错误数、警告数、信息数）
- 验证应用列表表格存在

### 3. Logs Page (`logs.spec.ts`)
- 访问 /logmon/logs
- 验证筛选表单（应用、级别、类型、关键词）
- 验证日志表格加载
- 验证搜索/重置按钮

### 4. Recordings Page - List (`recordings-list.spec.ts`)
- 访问 /logmon/recordings
- 验证录制列表表格加载
- 验证时间显示格式正确（不是 undefined 或原始时间戳）
- 验证时长显示（不是 undefined）
- 验证数据字段：sessionId, appId, url, status, eventCount, duration

### 5. Recordings Page - Playback (`recordings-playback.spec.ts`)
- 点击第一条录制的"播放"按钮
- 等待播放器容器出现
- 验证 rrweb-player iframe 渲染
- 截图验证播放器画面不为空白

### 6. User Management (`users.spec.ts`)
- 访问 /logmon/users
- 验证用户列表表格
- 验证管理员用户存在

## 项目结构
```
dashboard/
  e2e/
    playwright.config.ts
    fixtures/
      auth.ts          # 登录 fixture
    specs/
      login.spec.ts
      overview.spec.ts
      logs.spec.ts
      recordings-list.spec.ts
      recordings-playback.spec.ts
      users.spec.ts
    utils/
      api.ts            # API 辅助方法
```

## Playwright Config
- baseURL: `https://sanfacheng.cyou/logmon/`
- 浏览器: Chromium only（headless）
- 超时: 30s per test
- 截图: only on failure
- 视频: retain on failure

## Auth Fixture
- 提供 `loginAs(username, password)` 方法
- 登录成功后保存 storageState，后续测试复用
- 默认使用 admin 账号

## 验证标准
- 所有测试通过（绿色）
- 录制列表时间和时长正确显示（非 undefined/NaN）
- 播放器渲染成功（截图不为空白）
- 截图保存到 `e2e/screenshots/` 用于视觉确认

## 执行
```bash
cd /home/coder/log-monitor/dashboard
npx playwright install chromium
npx playwright test
```

## 后续扩展
- CI 集成（GitHub Actions）
- 性能测试（API 响应时间）
- 移动端响应式测试
