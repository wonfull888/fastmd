# fastmd v0.1 原子任务清单

> 每个任务独立可交付，按顺序执行。完成一个打勾一个。

---

## T1 — 项目初始化

- [ ] **T1-1** 创建 GitHub 仓库（Public, MIT, Go .gitignore）
- [ ] **T1-2** 本地 `git clone`，初始化目录结构：
  ```
  cmd/server/main.go
  cmd/cli/main.go
  internal/store/store.go
  internal/render/render.go
  web/templates/
  web/static/
  ```
- [ ] **T1-3** `go mod init github.com/<user>/fastmd`
- [ ] **T1-4** 安装依赖：
  ```bash
  go get github.com/labstack/echo/v4
  go get modernc.org/sqlite
  go get github.com/yuin/goldmark
  ```
- [ ] **T1-5** 提交 init commit，推送到 `main` 分支

---

## T2 — 数据库层

- [ ] **T2-1** 实现 `internal/store/store.go`：`New()` 函数，打开 SQLite（WAL 模式）
- [ ] **T2-2** 实现 `migrate()`：自动建表（documents + 2个索引）
- [ ] **T2-3** 实现 `store.Create(id, content, tokenHash, ipHash string) error`
- [ ] **T2-4** 实现 `store.GetByID(id string) (*Document, error)`（含过期过滤）
- [ ] **T2-5** 实现 `store.Delete(id, tokenHash string) (bool, error)`
- [ ] **T2-6** 实现 `generateID(length int) string`（Base62，4位，冲突重试）
- [ ] **T2-7** 单元测试：Create / GetByID / Delete 基本路径

---

## T3 — Markdown 渲染层

- [ ] **T3-1** 实现 `internal/render/render.go`：`ToHTML(markdown string) string`
- [ ] **T3-2** 配置 goldmark：开启 GFM（表格、任务列表、删除线）、代码高亮
- [ ] **T3-3** 验证输出：输入标准 Markdown，检查 HTML 结构正确

---

## T4 — 后端 API

- [ ] **T4-1** 搭建 Echo 框架，注册路由，绑定 Store 和 Render
- [ ] **T4-2** 实现 `POST /v1/push`
  - 读取 body 中 content 和 token
  - SHA-256 哈希 token
  - 生成短 ID，写入数据库
  - 返回 `{ "id": "x7y2", "url": "https://fastmd.dev/x7y2" }`
- [ ] **T4-3** 实现 `GET /:id`（双面路由）
  - 检查 URL 是否以 `.md` 结尾，或 Header `Accept: text/plain`
  - 机器模式：返回 raw Markdown，`Content-Type: text/plain`
  - 人类模式：渲染 HTML，用 `doc.html` 模板包裹
- [ ] **T4-4** 实现 `DELETE /v1/:id`
  - 从 `Authorization: Bearer <token>` 提取 token
  - SHA-256 哈希后与数据库比对
  - 匹配则删除，返回 `{ "ok": true }`；不匹配返回 403
- [ ] **T4-5** 实现 `GET /v1/version`
  - 版本号通过 `ldflags` 构建时注入
  - 返回 `{ "version": "0.1.0", "install_url": "https://fastmd.dev/install.sh" }`
- [ ] **T4-6** 实现 404 路由：找不到文档时返回 `404.html`
- [ ] **T4-7** 请求大小限制：body 最大 1MB（Echo middleware）

---

## T5 — 网站页面

- [ ] **T5-1** 创建 `web/templates/base.html`：公共 Header、导航、Footer
- [ ] **T5-2** 创建 `web/static/style.css`：设计 token、字体（Inter）、代码样式
- [ ] **T5-3** 实现首页 `index.html`：
  - Hero：产品定位一句话 + curl 安装命令（带一键复制按钮）
  - 使用示例：代码块展示 push → 返回链接
  - 场景卡片：AI Agent / 开发者分享
  - 底部 GitHub 链接
- [ ] **T5-4** 实现文档渲染页 `doc.html`：Markdown 渲染区 + 代码高亮 (highlight.js)
- [ ] **T5-5** 实现 API 文档页 `docs.html`
- [ ] **T5-6** 实现帮助页 `help.html`（FAQ 列表）
- [ ] **T5-7** 实现 404 页 `404.html`
- [ ] **T5-8** 实现 `web/static/app.js`：复制按钮交互

---

## T6 — CLI 工具

- [ ] **T6-1** 搭建 CLI 框架（使用标准库 `flag` 或 `cobra`），支持子命令
- [ ] **T6-2** 实现 Token 管理：
  - 首次运行：生成 `fmd_live_xxxx`，写入 `~/.config/fastmd/token`
  - 后续运行：读取已有 Token
- [ ] **T6-3** 实现 `push` 子命令：
  - 检测 stdin 是否有管道输入（`os.Stdin` isTerminal 判断）
  - 支持 `fastmd push <file>` 和 `cat file.md | fastmd`
  - 调用 `POST /v1/push`，打印返回的 URL
- [ ] **T6-4** 实现 `get` 子命令：
  - 调用 `GET /:id.md`，获取 raw Markdown
  - 提取首个 H1 标题，Slug 化作为文件名
  - 无标题则用 `<id>.md`
  - 写入本地文件
- [ ] **T6-5** 实现 `delete` 子命令：
  - 读取本地 Token
  - 调用 `DELETE /v1/:id`，携带 Bearer Token
  - 打印结果
- [ ] **T6-6** 实现 `upgrade` 子命令：
  - 执行 `curl -fsSL https://fastmd.dev/install.sh | sh`
- [ ] **T6-7** 统一错误处理：网络错误、403、404 的友好提示

---

## T7 — 构建与发布

- [ ] **T7-1** 编写 `Makefile`：`make build`、`make release`
- [ ] **T7-2** 交叉编译目标：`linux/amd64`、`linux/arm64`、`darwin/amd64`、`darwin/arm64`
- [ ] **T7-3** 构建时注入版本号：`-ldflags "-X main.Version=v0.1.0"`
- [ ] **T7-4** 编写 `install.sh`：检测平台 → 下载对应二进制 → 写入 `/usr/local/bin/fastmd`
- [ ] **T7-5** GitHub Release：打 `v0.1.0` tag，上传各平台二进制

---

## T8 — 部署

- [ ] **T8-1** VPS 上编译或上传服务端二进制（`linux/amd64`）
- [ ] **T8-2** 配置 Systemd 守护进程（`fastmd.service`）
- [ ] **T8-3** 配置 Caddy 反代（`fastmd.dev` → `localhost:8080`，自动 HTTPS）
- [ ] **T8-4** 域名 DNS：`fastmd.dev` A 记录指向 VPS IP
- [ ] **T8-5** 上传 `install.sh` 至服务器，通过 `https://fastmd.dev/install.sh` 可访问
- [ ] **T8-6** 端到端冒烟测试（见 TEST_CASES.md）

---

## 分支策略

```
main          ← 生产分支，打 tag 触发发布
dev           ← 开发集成分支
feature/T2   ← 每个功能块一个分支，完成后 PR 合并到 dev
```
