# fastmd 宝塔面板部署指南

> 核心流程：push 到 GitHub → GitHub Webhook → 宝塔 Webhook → VPS 自动 git pull + 编译 + 重启

---

## 一、VPS 准备工作（仅首次）

SSH 登录 VPS，依次执行：

### 1.1 安装 Go

```bash
wget https://go.dev/dl/go1.22.0.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.22.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
go version   # 验证安装成功
```

### 1.2 克隆仓库

```bash
cd /www/wwwroot
git clone https://github.com/wonfull888/fastmd.git
cd fastmd
```

### 1.3 配置 git 拉取凭据（Token 方式）

```bash
# <TOKEN> 替换为你的 GitHub Personal Access Token
git remote set-url origin https://<TOKEN>@github.com/wonfull888/fastmd.git
```

### 1.4 首次编译

```bash
cd /www/wwwroot/fastmd
mkdir -p dist data

export PATH=$PATH:/usr/local/go/bin
go build -ldflags "-X main.Version=v0.1.0" -o dist/fastmd-server ./cmd/server

# 验证二进制能运行
./dist/fastmd-server --version
```

---

## 二、宝塔面板添加 Go 项目

宝塔面板左侧菜单 → **项目** → **Go项目** → **添加Go项目**，按下表填写：

| 字段 | 填写内容 |
|---|---|
| **项目执行文件** | 点击文件夹图标，选择 `/www/wwwroot/fastmd/dist/fastmd-server` |
| **项目名称** | `fastmd` |
| **项目端口** | `8080`（**不要**勾选"放行端口"，端口对外通过 Nginx 反代，不直接暴露） |
| **执行命令** | `fastmd-server --port 8080 --db /www/wwwroot/fastmd/data/fastmd.db` |
| **环境变量** | 默认"无"即可 |
| **运行用户** | `www`（默认） |
| **开机启动** | ✅ 勾上 |
| **绑定域名** | `fastmd.dev` |

点击 **确定**，宝塔会自动启动并守护进程（每 120 秒检测一次）。

---

## 三、Nginx 反代配置

宝塔面板 → **网站** → **添加站点** → 域名填 `fastmd.dev` → 创建后：

1. 点击网站名旁边的 **设置**
2. 选择 **反向代理** → **添加反向代理**：
   - 代理名称：`fastmd`
   - 目标 URL：`http://127.0.0.1:8080`
   - 点击 **保存**

3. 选择 **SSL** → **Let's Encrypt** → 申请证书 → 开启**强制 HTTPS**

---

## 四、自动部署配置

### 4.1 编写部署脚本

在 VPS 上创建：

```bash
cat > /www/wwwroot/fastmd/deploy.sh << 'EOF'
#!/bin/bash
set -e
export PATH=$PATH:/usr/local/go/bin

PROJECT="/www/wwwroot/fastmd"
cd $PROJECT

echo "[$(date '+%Y-%m-%d %H:%M:%S')] Deploy started"

# 拉取最新代码
git pull origin main

# 重新编译
go build -ldflags "-X main.Version=$(git describe --tags --always 2>/dev/null || echo 'dev')" \
  -o dist/fastmd-server ./cmd/server

# 重启：杀掉旧进程，宝塔守护进程会自动在 120 秒内重启
kill $(cat /tmp/fastmd.pid 2>/dev/null) 2>/dev/null || pkill -f "fastmd-server" || true

echo "[$(date '+%Y-%m-%d %H:%M:%S')] Deploy complete"
EOF

chmod +x /www/wwwroot/fastmd/deploy.sh
```

手动测试一次确认没问题：

```bash
bash /www/wwwroot/fastmd/deploy.sh
```

### 4.2 宝塔 Webhook 配置

宝塔面板 → **软件商店** → 确认已安装 **宝塔WebHook** → 打开 → **添加**：

- **名称**：`fastmd-deploy`
- **执行脚本**：
  ```bash
  bash /www/wwwroot/fastmd/deploy.sh >> /var/log/fastmd-deploy.log 2>&1
  ```

保存后，复制生成的 **Webhook URL**（格式如 `http://<VPS_IP>:8888/hook?access_key=xxx&param=fastmd-deploy`）

### 4.3 GitHub Webhook 配置

打开 [github.com/wonfull888/fastmd/settings/hooks](https://github.com/wonfull888/fastmd/settings/hooks) → **Add webhook**：

| 字段 | 值 |
|---|---|
| Payload URL | 粘贴宝塔生成的 Webhook URL |
| Content type | `application/json` |
| Which events | Just the **push** event |
| Active | ✅ |

点击 **Add webhook**，然后查看 **Recent Deliveries** 确认有 ✅。

---

## 五、验证完整流程

```bash
# 本地触发一次 push
git commit --allow-empty -m "test: trigger webhook" && git push

# VPS 上查看部署日志（等待约 10-20 秒）
tail -f /var/log/fastmd-deploy.log

# 验证 API 是否正常
curl https://fastmd.dev/v1/version
```

---

## 日常运维

| 操作 | 方式 |
|---|---|
| 查看运行状态 | 宝塔面板 → 项目 → Go项目 → 查看 fastmd 状态 |
| 手动重启 | 宝塔面板 → Go项目 → fastmd → 重启 |
| 查看部署日志 | `tail -f /var/log/fastmd-deploy.log` |
| 查看运行日志 | 宝塔面板 → Go项目 → fastmd → 日志 |
| 备份数据库 | `cp /www/wwwroot/fastmd/data/fastmd.db /backup/fastmd-$(date +%Y%m%d).db` |

---

## 常见问题

**Q: Webhook 收到但 deploy.sh 报错 `go: command not found`**
A: deploy.sh 开头已加 `export PATH=$PATH:/usr/local/go/bin`，确认 Go 安装在 `/usr/local/go/`。

**Q: 编译成功但宝塔守护进程没有重启服务**
A: 宝塔守护进程默认每 120 秒检测一次。可进宝塔面板 → Go项目 → 手动点击"重启"。

**Q: GitHub Webhook 显示红色 ✕**
A: 宝塔 8888 端口需要在**宝塔安全** → 放行该端口，或在服务器防火墙开放。
