# TECH.md — 文档详情页增强 + 宽度调整

> 版本：v0.4 · 日期：2026-05-11 · 作者：RD

---

## Goal Anchor（原文复述，不修改）

| 项目 | 内容 |
|------|------|
| **北极星** | 让分享出去的文档链接，对接收者来说"一眼专业、信息完整、阅读舒适" |
| **必做** | ① 文档 meta 栏展示创建时间 + 文档 ID；② HTML 内容渲染；③ 宽度上限改为 120rem（≈1200px） |
| **不做** | 新增后端 API、修改存储结构、多主题/夜间切换、编辑功能 |
| **成功判据** | 打开任意 `/:id` 页面，meta 栏能看到创建时间和 ID；页面最大宽度视觉上明显宽于现在的 88rem |

---

## 第一性原理拆解

| 问题 | 本质约束 | 最小可行解 |
|------|---------|-----------|
| CreatedAt 未传给模板 | handler 漏传字段 | 在 `renderPage` 调用处加 `"CreatedAt"` key |
| 时间是 int64 Unix 时间戳 | 模板无法直接格式化 int64 | **在 Go 侧格式化为字符串**，模板只做展示 |
| 旧 4 位 ID 文档 created_at 可能为 0 | 历史数据问题 | Go 侧判断：`== 0` → 传空字符串，模板展示 `—` |
| `.doc-page` 宽度 88rem | 一行 CSS | 改为 `min(120rem, calc(100% - 3.2rem))` |
| meta 栏两栏变三栏 | flex 布局 | 中间加 `.doc-time` span，保持 space-between |

---

## 改动清单

### 文件：`cmd/server/main.go`

**改动位置：第 462-475 行**（`renderPage` 调用处）

新增两个字段传给 doc 模板：

```go
"CreatedAt": formatCreatedAt(doc.CreatedAt),
```

同时在文件顶部或 handler 附近增加辅助函数：

```go
// formatCreatedAt converts a Unix timestamp to "2006-01-02 15:04 UTC".
// Returns "" if ts is 0 (old documents with no recorded time).
func formatCreatedAt(ts int64) string {
    if ts == 0 {
        return ""
    }
    return time.Unix(ts, 0).UTC().Format("2006-01-02 15:04 UTC")
}
```

### 文件：`web/templates/doc.html`

**改动位置：第 4 行后**，在 `.doc-meta` 内加 `.doc-time` span：

```html
<span class="doc-time">{{if .CreatedAt}}{{.CreatedAt}}{{else}}—{{end}}</span>
```

布局变为：`doc-time | doc-id | doc-actions`（左中右，space-between）

### 文件：`web/static/style.css`

| 位置 | 改动 |
|------|------|
| 第 661 行 `.doc-page` width | `min(88rem, ...)` → `min(120rem, ...)` |
| 新增 `.doc-time` 样式 | `color: var(--muted); font-size: 1.3rem; white-space: nowrap;` |
| `@media (max-width: 780px)` 块末尾 | 追加 `.doc-meta { flex-direction: column; align-items: flex-start; }` |

---

## 数据流

```
SQLite: created_at INTEGER (Unix 时间戳)
  ↓  store.GetByID()
doc.CreatedAt int64
  ↓  formatCreatedAt(doc.CreatedAt)
"2026-05-10 14:32 UTC" (string, 或 "" 当 ts==0)
  ↓  renderPage() map[string]interface{}
"CreatedAt": "2026-05-10 14:32 UTC"
  ↓  Go html/template
doc.html: {{if .CreatedAt}}{{.CreatedAt}}{{else}}—{{end}}
  ↓  HTTP response
<span class="doc-time">2026-05-10 14:32 UTC</span>
```

---

## 测试钩子总表

| BDD 场景 | 测试钩子 |
|---------|---------|
| 场景 1：meta 栏显示创建时间 | `data-testid="doc-time"` 或 CSS class `.doc-time` 内容非空且格式匹配 |
| 场景 2：文档 ID 保留 | `.doc-id` 内容 = `fastmd.dev/{id}` |
| 场景 3：Markdown 渲染 | `.markdown-body` 内有 `<h1>`, `<strong>`, `<code>` |
| 场景 4：宽度 120rem | `.doc-page` computed width = 1200px（viewport ≥ 1400px 时） |
| 场景 5：旧 4 位 ID 兼容 | `.doc-time` 内容 = `—`（当 created_at=0 时） |
| 场景 6：404 | HTTP 404 + `.error-page` 存在 |
| 场景 7：响应式 | viewport ≤ 780px 时 `.doc-meta` flex-direction = column |

---

## 风险与预案

| 风险 | 概率 | 预案 |
|------|------|------|
| 旧 4 位 ID 文档 `created_at = 0` | 高（历史数据确认有此情况） | `formatCreatedAt` 返回空串，模板显示 `—` |
| 宽度改为 120rem 后横向滚动 | 低（`calc(100% - 3.2rem)` 保底） | CSS 已有 `min()` 限制，移动端无影响 |
| `time` 包已 import，无需新增 | — | 确认第 12 行 `"time"` 已在 import 列表 |
| `doc.CreatedAt` 字段名冲突 | 无（`store.Document.CreatedAt int64` 已确认） | — |

---

## 被砍掉的诱惑

- **`data-testid` 属性**：PRD 不要求，加了反而 HTML 冗余，暂不加，靠 class 测试即可。
- **移动端 meta 栏折叠动画**：PRD 明确标注"暂缓"，不做。
- **Go 时区转换为用户本地时区**：后端不知道用户时区，固定 UTC 是最安全的选择。

---

## Anchor 自查

| 自查项 | 结果 |
|--------|------|
| 所有改动都能映射回 Goal Anchor"必做"？ | ✅ 全部对应：①时间 ②ID保留/渲染 ③宽度 |
| 没有悄悄新增 PRD 没要求的功能？ | ✅ 无新路由、无新 API、无新存储字段 |
| 每个 BDD 场景都有测试钩子？ | ✅ 7 个场景全覆盖 |
| "被砍掉的诱惑"≥ 1 条？ | ✅ 3 条 |
