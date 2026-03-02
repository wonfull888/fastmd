# fastmd 宝塔面板自动部署指南（Webhook）

> 核心流程：本地 push 到 GitHub → GitHub 触发 Webhook → 宝塔执行脚本 → VPS 自动 git pull + 编译 + 重启服务

---

## 整体架构

```
本地开发  →  git push main  →  GitHub Webhook  →  宝塔 Webhook  →  VPS 自动部署
```

---

## Step 1：VPS 上克隆仓库

SSH 登录 VPS，克隆代码到服务目录：

```bash
cd /www/wwwroot
git clone https://github.com/wonfull888/fastmd.git fastmd
cd fastmd
```

配置 git 拉取凭据（用 Personal Access Token）：

```bash
git remote set-url origin https://<TOKEN>@github.com/wonfull888/fastmd.git
```

---

## Step 2：安装 Go（VPS 上）

```bash
wget https://go.dev/dl/go1.22.0.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.22.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
go version
```

---

## Step 3：首次手动编译 + 启动

```bash
cd /www/wwwroot/fastmd

# 编译服务端
go build -ldflags "-X main.Version=v0.1.0" -o dist/fastmd-server ./cmd/server

# 创建数据目录
mkdir -p /var/lib/fastmd
```

配置 Systemd 服务：

```bash
cat > /etc/systemd/system/fastmd.service << 'EOF'
[Unit]
Description=fastmd server
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/var/lib/fastmd
ExecStart=/www/wwwroot/fastmd/dist/fastmd-server --port 8080 --db /var/lib/fastmd/fastmd.db
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable fastmd
systemctl start fastmd
```

---

## Step 4：编写自动部署脚本

在 VPS 上创建部署脚本（宝塔 Webhook 会调用它）：

```bash
cat > /www/wwwroot/fastmd/deploy.sh << 'EOF'
#!/bin/bash
set -e

PROJECT_DIR="/www/wwwroot/fastmd"
cd $PROJECT_DIR

echo "[$(date)] Starting deploy..."

# 拉取最新代码
git pull origin main

# 重新编译
export PATH=$PATH:/usr/local/go/bin
go build -ldflags "-X main.Version=$(git describe --tags --always)" \
  -o dist/fastmd-server ./cmd/server

# 重启服务
systemctl restart fastmd

echo "[$(date)] Deploy complete!"
EOF

chmod +x /www/wwwroot/fastmd/deploy.sh
```

验证脚本本身能手动运行成功：
```bash
bash /www/wwwroot/fastmd/deploy.sh
```

---

## Step 5：配置宝塔 Webhook

1. 登录宝塔面板 → 左侧菜单找 **软件商店** → 搜索 **宝塔WebHook** → 确认已安装

2. 打开 **宝塔WebHook** → 点击 **添加**：
   - **名称**：fastmd-deploy
   - **脚本**：
     ```bash
     bash /www/wwwroot/fastmd/deploy.sh >> /var/log/fastmd-deploy.log 2>&1
     ```

3. 添加后，复制生成的 **Webhook URL**，格式类似：
   ```
   http://<VPS_IP>:8888/hook?access_key=xxxxxxxx&param=fastmd-deploy
   ```

---

## Step 6：配置 GitHub Webhook

1. 打开 GitHub 仓库 → **Settings** → **Webhooks** → **Add webhook**

2. 填写：
   - **Payload URL**：粘贴 Step 5 中宝塔生成的 URL
   - **Content type**：`application/json`
   - **Which events**：选 **Just the push event**
   - **Active**：勾上

3. 点 **Add webhook** 保存

4. 在 GitHub Webhooks 页面，点击刚添加的 webhook → **Recent Deliveries**，确认有绿色 ✓（如果是 ✕ 说明 VPS 端口未开放）

---

## Step 7：宝塔配置 Nginx 反代

宝塔面板 → **网站** → **添加站点** → 域名填 `fastmd.dev` → 创建后点击站点 → **配置文件**，在 `server {}` 内添加：

```nginx
location / {
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    client_max_body_size 2m;
}
```

申请 SSL：站点 → **SSL** → **Let's Encrypt** → 一键申请并开启强制 HTTPS。

---

## 验证自动部署

```bash
# 本地随便改一个文件，push 到 main
git add . && git commit -m "test: trigger deploy" && git push

# 等待约 10-20 秒，查看 VPS 上的部署日志
tail -f /var/log/fastmd-deploy.log

# 验证服务版本是否更新
curl https://fastmd.dev/v1/version
```

---

## 日常运维

| 操作 | 命令 |
|---|---|
| 查看服务状态 | `systemctl status fastmd` |
| 查看实时日志 | `journalctl -u fastmd -f` |
| 查看部署日志 | `tail -f /var/log/fastmd-deploy.log` |
| 手动触发部署 | `bash /www/wwwroot/fastmd/deploy.sh` |
| 备份数据库 | `cp /var/lib/fastmd/fastmd.db /backup/fastmd-$(date +%Y%m%d).db` |

---

## 常见问题

**Q: GitHub Webhook 发送失败（红色 ✕）？**
A: 检查宝塔防火墙是否开放了 8888 端口（宝塔默认端口），或改用 80/443 端口的 Nginx 转发。

**Q: deploy.sh 执行成功但服务没更新？**
A: 检查编译是否出错：`tail -50 /var/log/fastmd-deploy.log`

**Q: go command not found 报错？**
A: deploy.sh 中已加 `export PATH=$PATH:/usr/local/go/bin`，确认 Go 安装路径正确。
