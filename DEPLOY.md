# fastmd 宝塔面板部署指南

## 前置条件

- VPS 已安装宝塔面板（Linux 版）
- 域名 `fastmd.dev` 已购买，DNS 管理权在手
- 宝塔面板已安装：**Nginx** 或 **Caddy**（推荐 Nginx，宝塔对其支持最好）

> **注意**：宝塔面板自带 Nginx 反代配置 UI，不需要手动写 Nginx conf。以下使用 **Nginx + Let's Encrypt** 方案。

---

## Step 1：DNS 解析

在域名注册商控制台，添加 A 记录：

```
类型: A
主机记录: @
记录值: <你的 VPS IP>
TTL: 600
```

等待解析生效（通常 5-10 分钟）：
```bash
ping fastmd.dev  # 看是否解析到你的 VPS IP
```

---

## Step 2：在 VPS 上安装 Go

SSH 登录 VPS：

```bash
# 下载 Go 1.22（按需替换版本号）
wget https://go.dev/dl/go1.22.0.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.22.0.linux-amd64.tar.gz

# 写入环境变量
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# 验证
go version
```

---

## Step 3：部署服务端二进制

### 方式 A：本地交叉编译后上传（推荐）

在本地开发机执行：

```bash
# 进入项目根目录
cd fastmd

# 交叉编译 Linux amd64 服务端
GOOS=linux GOARCH=amd64 go build \
  -ldflags "-X main.Version=v0.1.0" \
  -o dist/fastmd-server \
  ./cmd/server

# 上传到 VPS
scp dist/fastmd-server root@<VPS_IP>:/usr/local/bin/fastmd-server
```

### 方式 B：在 VPS 上直接编译

```bash
# 克隆代码
git clone https://github.com/<user>/fastmd.git /opt/fastmd
cd /opt/fastmd

# 编译
go build -ldflags "-X main.Version=v0.1.0" -o /usr/local/bin/fastmd-server ./cmd/server
```

---

## Step 4：创建数据目录

```bash
mkdir -p /var/lib/fastmd
chmod 750 /var/lib/fastmd
```

---

## Step 5：配置 Systemd 服务

创建服务文件：

```bash
cat > /etc/systemd/system/fastmd.service << 'EOF'
[Unit]
Description=fastmd server
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/var/lib/fastmd
ExecStart=/usr/local/bin/fastmd-server --port 8080 --db /var/lib/fastmd/fastmd.db
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF
```

启动并设置开机自启：

```bash
systemctl daemon-reload
systemctl enable fastmd
systemctl start fastmd

# 检查状态
systemctl status fastmd
```

查看日志：

```bash
journalctl -u fastmd -f
```

---

## Step 6：宝塔面板配置 Nginx 反代

1. 登录宝塔面板 → **网站** → **添加站点**
   - 域名：`fastmd.dev`
   - 根目录：`/www/wwwroot/fastmd.dev`（占位，实际由反代接管）
   - PHP 版本：纯静态/不选

2. 站点创建后，点击站点名 → **配置文件**，在 `server {}` 块内添加：

```nginx
location / {
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;

    # 支持大文件推送（最大 1MB 正文）
    client_max_body_size 2m;
}
```

3. 点击 **SSL** → **Let's Encrypt** → 申请免费证书（自动配置 HTTPS）

4. 开启 **强制 HTTPS** 选项

---

## Step 7：验证部署

```bash
# 测试 API 是否正常
curl https://fastmd.dev/v1/version

# 推一条测试文档
echo "# Deploy Test" | fastmd

# 浏览器打开返回的链接，确认 HTML 渲染正常
```

---

## Step 8：部署 install.sh

`install.sh` 需要通过 `https://fastmd.dev/install.sh` 访问，有两种方式：

### 方式 A：服务端路由（推荐）

在 Go 服务中注册静态路由，将 `install.sh` 和各平台二进制嵌入：

```go
// cmd/server/main.go
e.Static("/install.sh", "install.sh")
e.Static("/releases", "dist/")  // 存放各平台二进制
```

### 方式 B：Nginx 静态文件

将 `install.sh` 放入站点根目录：
```bash
cp install.sh /www/wwwroot/fastmd.dev/install.sh
```

---

## 日常运维

| 操作 | 命令 |
|---|---|
| 查看服务状态 | `systemctl status fastmd` |
| 查看实时日志 | `journalctl -u fastmd -f` |
| 重启服务 | `systemctl restart fastmd` |
| 更新服务端 | 上传新二进制 → `systemctl restart fastmd` |
| 备份数据库 | `cp /var/lib/fastmd/fastmd.db /backup/fastmd-$(date +%Y%m%d).db` |

---

## 常见问题

**Q: 宝塔面板访问 8080 端口失败？**
A: 检查宝塔防火墙 → 确保 8080 对 `127.0.0.1` 开放（内网访问即可，不需要对外）。

**Q: Let's Encrypt 申请失败？**
A: 确认域名 DNS 已解析到当前 VPS IP，80 端口未被占用。

**Q: Systemd 服务起不来？**
A: 运行 `journalctl -u fastmd -n 50` 查看详细错误日志。
