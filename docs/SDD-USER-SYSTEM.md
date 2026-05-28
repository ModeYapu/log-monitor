# LogMonitor 用户系统 & 登录页面 — SDD

## 1. 目标

为 LogMonitor Dashboard 添加完整的用户认证系统：
- 登录页面（JWT token 认证）
- 用户管理（CRUD，仅 admin）
- 路由守卫（未登录跳转登录页）
- 安装时自动创建 admin 用户
- 移除 nginx Basic Auth，改用应用层认证

## 2. 架构设计

```
浏览器 → Vue3 Login → POST /api/auth/login → Go 后端验证 → 返回 JWT
浏览器 → Vue3 Router Guard → 检查 localStorage token → 无则跳转 /login
浏览器 → API 请求 → Authorization: Bearer <jwt> → Go 中间件验证
```

## 3. 数据库

### 新表 `users`

```sql
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    display_name TEXT DEFAULT '',
    role TEXT NOT NULL DEFAULT 'user',  -- 'admin' | 'user'
    enabled INTEGER NOT NULL DEFAULT 1,
    last_login_at INTEGER DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users(username);
```

### 安装时种子数据

后端启动时检查 `users` 表是否为空，如果为空则创建默认 admin 用户：
- username: `admin`
- password: 配置文件 `auth.default_password` 指定，默认 `admin123`
- role: `admin`

## 4. Go 后端改动

### 4.1 新增 handler: `collector/handler/auth.go`

```
POST /api/auth/login     — 登录，返回 JWT token + 用户信息
GET  /api/auth/me        — 获取当前用户信息（需 token）
PUT  /api/auth/password  — 修改自己的密码（需 token）
GET  /api/users          — 列出所有用户（需 admin）
POST /api/users          — 创建用户（需 admin）
PUT  /api/users/:id      — 更新用户（需 admin）
DELETE /api/users/:id    — 删除用户（需 admin，不能删自己）
```

### 4.2 JWT 实现

使用 `golang-jwt/jwt/v5` 库：
- 签名密钥从配置文件 `auth.jwt_secret` 读取，如果为空则自动生成（启动时日志输出）
- Token 有效期 24 小时
- Claims 包含：`user_id`, `username`, `role`, `exp`

```go
// TokenClaims
type TokenClaims struct {
    UserID   int64  `json:"user_id"`
    Username string `json:"username"`
    Role     string `json:"role"`
    jwt.RegisteredClaims
}

// GenerateToken — 生成 JWT
// ValidateToken — 验证 JWT
```

### 4.3 密码存储

使用 `golang.org/x/crypto/bcrypt`，cost=10。

### 4.4 认证中间件

在 main.go 中为需要认证的路由组添加 JWT 中间件：
- `/api/report` — 不需要认证（SDK 上报）
- `/api/events` — 不需要认证（SDK 上报）
- `/api/report/screenshot` — 不需要认证（SDK 上报）
- `/api/auth/login` — 不需要认证
- `/api/auth/me` — 需要认证（任何角色）
- `/api/auth/password` — 需要认证（任何角色）
- `/api/query/*` — 需要认证（任何角色）
- `/api/query/stats` — 需要认证
- `/api/alerts/*` — 需要认证
- `/ws/*` — 需要认证（token 通过 query param）
- `/api/users/*` — 需要认证 + admin 角色
- `/api/health` — 不需要认证

### 4.5 配置扩展

`config.yaml` 添加：
```yaml
auth:
  enabled: true
  jwt_secret: ""           # 空=自动生成
  default_password: "admin123"  # 仅首次安装使用
  token_expire_hours: 24
```

### 4.6 CORS 更新

所有 handler 的 CORS 头部统一由中间件处理，不再在各个 handler 里单独设置。

## 5. 前端改动

### 5.1 新增页面: `views/Login.vue`

- Element Plus 表单：用户名 + 密码 + 登录按钮
- 登录成功 → 存 JWT 到 localStorage → 跳转 /
- 登录失败 → Element Plus 错误提示
- 深色主题，与现有 Dashboard 风格一致
- 居中卡片布局，上方显示 LogMonitor logo/标题

### 5.2 新增页面: `views/UserManagement.vue`

- 仅 admin 可见
- Element Plus 表格展示用户列表
- 新增用户对话框
- 编辑用户对话框（修改角色、启用/禁用）
- 删除用户（二次确认）
- 重置密码

### 5.3 路由更新

```typescript
// router/index.ts
{ path: '/login', name: 'Login', component: Login, meta: { public: true } }
{ path: '/users', name: 'UserManagement', component: UserManagement, meta: { requiresAdmin: true } }
// 现有路由 meta: { requiresAuth: true }
```

### 5.4 路由守卫

```typescript
router.beforeEach((to, from, next) => {
  const token = localStorage.getItem('logmon_token')
  if (to.meta.public) return next()
  if (!token) return next('/login')
  // 可选：检查 token 是否过期
  next()
})
```

### 5.5 API 层更新

```typescript
// api/index.ts — axios 拦截器添加 token
api.interceptors.request.use(config => {
  const token = localStorage.getItem('logmon_token')
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

// 401 响应自动跳转登录
api.interceptors.response.use(
  res => res,
  error => {
    if (error.response?.status === 401) {
      localStorage.removeItem('logmon_token')
      router.push('/login')
    }
    return Promise.reject(error)
  }
)
```

### 5.6 侧边栏更新

在 App.vue 侧边栏底部添加：
- admin 用户可见「用户管理」菜单项
- 所有用户可见当前登录用户名 + 退出按钮

### 5.7 Settings.vue 扩展

添加「修改密码」区域：
- 旧密码 + 新密码 + 确认密码
- Element Plus 表单验证

## 6. nginx 改动

部署完成后移除 nginx Basic Auth（改由应用层处理）：

```nginx
location /logmon/ {
    # 移除 auth_basic 两行
    alias /opt/logmonitor/dashboard/;
    # ... 其余不变
}
```

## 7. 文件清单

### Go 后端（新增/修改）
| 文件 | 操作 | 说明 |
|------|------|------|
| `handler/auth.go` | 新增 | 登录/用户管理 API |
| `middleware/jwt.go` | 新增 | JWT 验证中间件 |
| `middleware/cors.go` | 新增 | 统一 CORS 中间件 |
| `storage/users.go` | 新增 | 用户 CRUD 数据库操作 |
| `model/user.go` | 新增 | User 模型 |
| `config/config.go` | 修改 | 添加 Auth 配置 |
| `main.go` | 修改 | 路由分组 + 中间件 |
| `handler/report.go` | 修改 | 移除 CORS 代码 |
| `handler/query.go` | 修改 | 移除 CORS 代码 |
| `handler/alerts.go` | 修改 | 移除 CORS 代码 |
| `go.mod` | 修改 | 添加 jwt, bcrypt 依赖 |

### 前端（新增/修改）
| 文件 | 操作 | 说明 |
|------|------|------|
| `views/Login.vue` | 新增 | 登录页面 |
| `views/UserManagement.vue` | 新增 | 用户管理页面 |
| `api/index.ts` | 修改 | token 拦截器 + auth API |
| `router/index.ts` | 修改 | 添加路由 + 守卫 |
| `App.vue` | 修改 | 侧边栏用户信息 + 退出 |
| `views/Settings.vue` | 修改 | 添加修改密码 |
| `types/index.ts` | 修改 | 添加 User 类型 |

## 8. 安全注意事项

1. JWT secret 必须足够长（≥32字符），生产环境应手动配置
2. 密码 bcrypt cost=10，不能低于 8
3. admin 用户不能被删除或禁用
4. 不能删自己
5. 登录失败不透露是用户名错还是密码错（统一「用户名或密码错误」）
6. JWT 过期后前端自动跳转登录页
7. WebSocket 连接支持 query param 传 token

## 9. 执行顺序

1. Go 后端：model → storage → config → middleware → handler → main → go build
2. 前端：types → api → Login.vue → router guard → App.vue → UserManagement.vue → Settings.vue
3. 部署：编译 → 停服务 → 替换二进制 → 刷新前端 → 移除 nginx Basic Auth
4. 验证：登录 → 查看日志 → 创建用户 → 退出 → 重新登录
