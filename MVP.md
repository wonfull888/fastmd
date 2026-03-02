# fastmd.dev MVP 产品需求文档

## 1. 产品定位 (Positioning)

* **定义**：面向 AI Agent 的 Markdown 极速中转管道。
* **核心用户路径**：AI Agent 通过 API 自动创建/更新/删除文档 → 生成短链接 → 人类通过浏览器审阅 HTML 渲染结果。
* **HTML 渲染**：仅用于人类只读审阅，不是主界面；Agent 通过 `.md` 接口消费结构化数据。
* **产品哲学**：**极简**（免注册）、**极快**（Go+SQLite）、**极客**（CLI 优先）。

---

## 2. MVP 功能清单 (Feature List)

### A. 身份系统 (Identity)

* **匿名令牌**：自动生成 `fmd_live_xxxx` 格式的 Token。
* **凭证校验**：Token 哈希值作为文档所属权的唯一凭证，无需密码。

### B. 核心 API (Backend)

* **创建 (Push)**：`POST /v1/push`。接收 Markdown，返回短 ID（如 `x7y2`）。
* **双面路由 (View)**：`GET /:id`。
  * **人类模式**：浏览器访问，返回美化的 HTML 渲染页。
  * **机器模式**：URL 以 `.md` 结尾或携带 `Accept: text/plain` Header，返回 Raw Markdown。
* **删除 (Delete)**：`DELETE /v1/:id`。凭 Token 物理销毁。
* **版本查询**：`GET /v1/version`。返回最新 CLI 版本及安装路径。

### C. CLI 命令行工具 (Client)

* **Push**：支持管道流，如 `cat file.md | fastmd`。
* **Get**：`fastmd get <ID>`。拉取文档内容到本地，智能提取 H1 作为文件名。
* **Upgrade**：`fastmd upgrade`。通过重新执行安装脚本实现。

---

## 3. 产品流程与场景 (Flow & Scenarios)

### 场景一：AI Agent（主要场景）

1. **静默发布**：Agent 完成任务后，通过 `POST /v1/push` 将报告推送到云端。
2. **轻量交付**：Agent 在对话中只返回一个短链接，保持上下文整洁。
3. **数据消费**：后续 Agent 通过 `.md` 接口直接获取结构化数据，无需解析 HTML。

### 场景二：人类开发者（辅助场景）

1. **一键分享**：终端输入 `cat report.md | fastmd`，瞬间生成可阅读链接。
2. **内容获取**：`fastmd get <ID>`，拉取云端文档到本地。

---

## 4. 技术方案 (Technical Scheme)

### A. 后端架构

* **技术栈**：Go (Golang) + Echo 框架 + SQLite（纯 Go 驱动）。
* **部署**：VPS + Caddy（自动 HTTPS）。
* **存储**：MVP 阶段默认永久存储；TTL 功能（24h / 7d / 30d）为后续版本。

### B. 本地保存与命名规则

执行 `fastmd get <ID>` 且未指定文件名时：

1. **智能提取**：提取 Markdown 首个 H1 标题并 Slug 化（如 `my-report.md`）。
2. **ID 兜底**：若无标题，使用 `<ID>.md`。

### C. 升级机制

* **Upgrade 逻辑**：调用系统 Shell 重新运行 `curl -fsSL https://fastmd.dev/install.sh | sh`。

---

## 5. 执行清单 (Action Plan)

* [ ] **域名**：fastmd.dev A 记录解析。
* [ ] **环境**：VPS 安装 Go 1.21+ 与 Caddy。
* [ ] **开发**：实现后端 API（Push / View / Delete / Version）。
* [ ] **开发**：实现 CLI 工具（push / get / delete / upgrade）。
* [ ] **上线**：配置 Caddy 反代，设置 Systemd 守护进程。