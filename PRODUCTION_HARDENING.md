# LogMonitor Phase 1: 生产环境安全加固

## 项目位置
- 源码：`/home/coder/log-monitor/`
- 运行时：`/opt/logmonitor/`
- Nginx：`/etc/nginx/sites-enabled/sanfacheng.cyou.conf`

## 当前状态
- 进程以 root 运行：`/usr/local/bin/logmonitor-collector -config /opt/logmonitor/config.yaml`
- 无 systemd service（手动启动）
- Dashboard `/logmon/` 无认证，公网可访问
- CORS 为 `*`
- 数据库 108MB 在 `/opt/logmonitor/data/logmonitor.db`
- 已有 systemd service 模板：`/home/coder/log-monitor/deploy/logmonitor-collector.service`

## 任务清单（按顺序执行）

### 1. 创建 logmonitor 系统用户
```bash
useradd -r -s /bin/false logmonitor
```

### 2. 准备目录结构
```bash
mkdir -p /opt/logmonitor/collector /opt/logmonitor/data /opt/logmonitor/data/backup
cp /usr/local/bin/logmonitor-collector /opt/logmonitor/collector/
```

### 3. 创建生产配置文件 `/opt/logmonitor/collector/config.yaml`
```yaml
server:
  port: 9200
  cors: false

database:
  path: /opt/logmonitor/data/logmonitor.db
  retention_days: 30

buffer:
  size: 10000
  flush_interval_ms: 2000
  flush_batch_size: 500

alert:
  check_interval_ms: 60000
```

### 4. 设置文件权限
```bash
chown -R logmonitor:logmonitor /opt/logmonitor
chmod 750 /opt/logmonitor
chmod 750 /opt/logmonitor/data
chmod 640 /opt/logmonitor/data/logmonitor.db
```

### 5. 部署 systemd service
创建 `/etc/systemd/system/logmonitor-collector.service`：
```ini
[Unit]
Description=LogMonitor Collector Service
After=network.target

[Service]
Type=simple
User=logmonitor
Group=logmonitor
WorkingDirectory=/opt/logmonitor/collector
ExecStart=/opt/logmonitor/collector/logmonitor-collector -config /opt/logmonitor/collector/config.yaml
Restart=always
RestartSec=5

NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/logmonitor/data

LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

### 6. 停止旧进程，启动新 service
```bash
# 停止当前 root 进程
pkill -f "logmonitor-collector"

# 启动 systemd service
systemctl daemon-reload
systemctl enable logmonitor-collector
systemctl start logmonitor-collector

# 验证
systemctl status logmonitor-collector
ss -tlnp | grep 9200
curl -s http://127.0.0.1:9200/api/health
```

### 7. Dashboard 加 Basic Auth
```bash
# 安装 apache2-utils（如果没有 htpasswd）
apt-get install -y apache2-utils

# 创建密码文件（用户名 admin，密码自动生成一个强密码）
htpasswd -cb /etc/nginx/.htpasswd-logmon admin "$(openssl rand -base64 16)"
```

### 8. 修改 nginx 配置
编辑 `/etc/nginx/sites-enabled/sanfacheng.cyou.conf`：

在 `location /logmon/` 块中添加 auth：
```nginx
location /logmon/ {
    auth_basic "LogMonitor Dashboard";
    auth_basic_user_file /etc/nginx/.htpasswd-logmon;
    alias /opt/logmonitor/dashboard/;
    # ... 其余保持不变
}
```

注意：`/logmon-api/` 路径不加认证（SDK 需要上报）。

然后：
```bash
nginx -t && systemctl reload nginx
```

### 9. 收紧 CORS — 修改 Go 代码
编辑 `/home/coder/log-monitor/collector/handler/report.go`：

将 `Access-Control-Allow-Origin: *` 改为从配置读取白名单。

在 `/home/coder/log-monitor/collector/config/config.go` 中添加 `AllowedOrigins` 字段。

在 report handler 中使用配置的 origins 替代 `*`。如果请求 Origin 在白名单中，返回该 Origin；否则不返回 CORS 头。

### 10. 编译部署新二进制
```bash
cd /home/coder/log-monitor/collector
GOPROXY=https://goproxy.cn go build -buildvcs=false -o /opt/logmonitor/collector/logmonitor-collector .
chown logmonitor:logmonitor /opt/logmonitor/collector/logmonitor-collector
systemctl restart logmonitor-collector
```

### 11. 最终验证
```bash
# 服务运行正常
systemctl status logmonitor-collector
curl -s http://127.0.0.1:9200/api/health

# Dashboard 需要认证
curl -s -o /dev/null -w "%{http_code}" http://127.0.0.1/logmon/
# 期望 401

# Dashboard 认证后可访问
curl -s -o /dev/null -w "%{http_code}" -u admin:密码 http://127.0.0.1/logmon/
# 期望 200

# SDK 上报不需要认证
curl -s -X POST http://127.0.0.1:9200/api/report -H "Content-Type: application/json" -d '{"appId":"test","events":[]}'
# 期望返回错误但不是 401

# 进程不是 root
ps -o user,pid,cmd -C logmonitor-coll
# 期望 USER 为 logmonitor
```

## 重要约束
- 不要修改 nginx 的 `/logmon-api/` 路径（SDK 上报需要）
- 不要修改 nginx 的 `/ws/` WebSocket 路径
- 数据库文件不要删除或清空
- htpasswd 的用户名密码记录下来输出给我
- CORS 白名单包含：`https://sanfacheng.cyou`, `https://www.sanfacheng.cyou`
- Go 编译用 `GOPROXY=https://goproxy.cn`
- 编译新二进制替换时先 `systemctl stop` 再 `cp` 再 `systemctl start`
