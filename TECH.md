# TECH.md — fastmd v0.4：分享链接体验升级

> 版本：v0.4 · 日期：2026-07-23 · 作者：RD

---

## Goal Anchor（原文复述，不修改）

- **北极星目标**：让 fastmd 分享出去的链接在任何消费场景下都能正确处理——Agent 拿到原始 Markdown，人类看到可读页面，社交平台展示有意义的预览。
- **本次必做**：
  1. CLI `push` 终端输出链接以 `.md` 结尾
  2. API `POST /v1/push` 返回的 `url` 字段以 `.md` 结尾
  3. 浏览器访问 `/:id.md` 展示 HTML 页面，含提示"查看网页渲染版：去掉 `.md`"和原始 Markdown 内容
  4. Agent 通过 `Accept: text/plain` 访问 `/:id.md` 仍能获取原始 Markdown（向后兼容）
  5. 文档详情页 `og:title` 动态使用文档 H1 标题（无 H1 时退回默认格式）
  6. 文档详情页 `og:description` 动态使用文档内容的开头提炼（约 200 字符）
- **本次不做**：
  1. 修改 `/:id` 页面展示样式或布局（理由：out-of-scope，与本次目标无关）
  2. 自定义 OG 社交预览图（理由：defer-v2，本期先用现有 SVG）
  3. 用户手动设置 OG 标题/描述（理由：defer-v2，fastmd 不做内容编辑）
  4. `GET /v1/docs` API 返回的链接格式变更 → **被补充需求 3 覆盖，现改为 `.md` 后缀**（理由：Dashboard 一致性）
- **成功判据**：本版发布门禁 M-1 ~ M-9 全部验证通过。

---

## 第一性原理拆解

### 需求 1：链接统一 & `.md` 页面增强

| 问题 | 本质约束 | 最小可行解 |
|------|---------|-----------|
| 发布链接不统一 | API 返回的 `url` 字段是唯一真相源，CLI 仅透传 | 改 API handler 一处：`"/"+id+".md"`；CLI 零改动 |
| `.md` 页面人类看不懂 | HTTP 内容协商：Accept 头区分人类/机器 | `.md` + `Accept: text/plain` → raw；`.md` + 默认 Accept → HTML hint 页 |
| 旧文档 CreatedAt=0 = "0001-01-01" | 历史数据 | `formatCreatedAt` 已处理（v0.3 遗留），新模板同样依赖它 |
| Agent 不设 Accept 头时消费失败 | 无法区分，PRD 已评估风险为低 | `<script type="text/markdown">` 嵌入原始内容作为备选路径 |

### 需求 2：动态 OG 标签

| 问题 | 本质约束 | 最小可行解 |
|------|---------|-----------|
| OG 标题千篇一律 | 服务端渲染时文档已加载，可从 Content 实时提取 | `extractTitle()` 已有，传 `OGTitle` / `TwitterTitle` 字段到模板 |
| OG 描述千篇一律 | 无现成函数 | 新函数 `extractDescription(content, 200)`：去 Markdown 标记 → 截断 200 字符 → 扩展到下一个空格 |
| HTML 实体转义 | Go `html/template` 自动转义 `&` `<` `>` `"` `'` 五个字符 | 零额外处理，依赖 Go 模板自动转义 |

---

## 技术选型

| 技术点 | 选择 | 不选 | 原因 |
|--------|------|------|------|
| `.md` hint 模板 | 新建 `web/templates/md-hint.html` | 复用 doc.html 加条件分支 | 职责分离：hint 页只展示 raw + 提示，不拿 doc 页所有数据 |
| hug 语言 | 英文 `💡 To view the rendered page: remove .md from the address bar.` | 中文 | 产品默认英文，base.html lang=en |
| `extractDescription` 截断 | 200 字符后扩展到下一个空格 | 严格 200 字符截断 / 完整段落 | 避免单词中间截断，`strings.LastIndexByte` 找空格 |
| OG HTML 转义 | Go `html/template` 自动转义 | 手动 `html.EscapeString` | Go 模板已对 `{{.OGDescription}}` 等做上下文自动转义 |
| 朗读按钮 | Web Speech API (`speechSynthesis`) | 服务端 TTS（Edge TTS / Piper） | PRD 决定方案 A；零服务端成本 |
| 朗读按钮位置 | doc-meta 区域，与 Raw/Copy link 同行 | 独立浮动栏 | 用户确认方案 A |
| 不支持浏览器 | 静默隐藏朗读按钮 | 显示禁用态 / 替代方案 | `window.speechSynthesis` 检测 |
| tests/ 目录 | Go httptest + `net/http/httptest` | pytest / shell 脚本 | Go 原生测试框架，零额外依赖，可直接测试 handler 逻辑 |

---

## 改动范围

### 新增文件

| 文件 | 用途 |
|------|------|
| `web/templates/md-hint.html` | `.md` 提示页模板 |
| `tests/main_test.go` | httptest 集成测试 |

### 修改文件

| 文件 | 改动内容 |
|------|----------|
| `cmd/server/main.go` | ① `loadTemplates()` 加 `"md-hint"` 页；② 新函数 `extractDescription()`；③ `/:id` handler 重构内容协商逻辑；④ `POST /v1/push` url 加 `.md`；⑤ `GET /v1/docs` url 加 `.md`；⑥ `renderPage` 调用传 `OGTitle`/`OGDescription` 等动态 OG 字段 |
| `web/templates/doc.html` | doc-meta 区域加朗读按钮（位于 Raw 和 Copy link 之间） |
| `web/static/app.js` | ① Dashboard 文档标题改为 `<a>` 链接；② 朗读按钮逻辑（朗读/暂停/继续/停止）；③ 不支持浏览器静默隐藏 |
| `web/static/style.css` | ① `.md-hint-page` 样式；② `.btn-read-aloud` 样式；③ `.dashboard-doc-title` 改为 `<a>` 后的样式适配 |

### 不修改文件

| 文件 | 理由 |
|------|------|
| `cmd/cli/main.go` | CLI 仅透传 API 返回的 `url` 字段，API 改为 `.md` 后缀后 CLI 自动获得，**零改动** |
| `internal/render/render.go` | 不涉及 Markdown 渲染逻辑变更 |
| `internal/store/store.go` | 无存储变更 |
| `web/templates/base.html` | OG 标签字段已预留（`OGTitle`/`OGDescription`），仅 handler 侧多传参数即可 |

---

## 数据流

### `.md` 路由内容协商

```
浏览器请求 GET /abc12345.md (默认 Accept: text/html,...)
  ↓
handler 判断: isMdSuffix=true, acceptPlain=false
  ↓ rawMode=false, mdHintMode=true
  ↓
db.GetByID("abc12345") → doc
  ↓
extractTitle(doc.Content) → "Hello World"
extractDescription(doc.Content, 200) → "This is the doc about..."
  ↓
renderPage(200, "md-hint", data{
  "Title": "Hello World | fastmd.dev",
  "OGTitle": "Hello World",
  "OGDescription": "This is the doc...",
  "RawContent": doc.Content,
  "ID": "abc12345",
  "CreatedAt": "2026-07-23 14:00 UTC",
})

Agent 请求 GET /abc12345.md (Accept: text/plain)
  ↓
handler 判断: isMdSuffix=true, acceptPlain=true
  ↓ rawMode=true
  ↓
return c.String(200, doc.Content)  // text/plain ← 完全不变
```

### OG 标签数据流

```
SQLite: documents.content (TEXT)
  ↓
handler 内
  extractTitle(content)       → string (H1 或 "")
  extractDescription(content)  → string (去标记 + 截断 200 字符)
  ↓
renderPage(200, "doc", data{
  "Title":              title (dynamic),
  "Description":        description (dynamic),
  "OGTitle":            title,
  "OGDescription":      description,
  "TwitterTitle":       title,
  "TwitterDescription": description,
})
  ↓ Go html/template → base.html
<meta property="og:title" content="Hello World">
<meta property="og:description" content="This is the...">
```

### Dashboard API 数据流

```
GET /v1/docs (Authorization: Bearer <token>)
  ↓ db.ListByTokenHash(tokenHash)
[]DocumentSummary{...}
  ↓
for each doc:
  url := absoluteURL(c, "/"+doc.ID+".md")  ← 改动点
  ↓
items = [{id, title, url: "...md", created_at}, ...]
  ↓
return c.JSON(200, {"documents": items})
```

---

## 关键函数签名

### `extractDescription`

```go
// extractDescription returns a plain-text summary of up to maxChars characters
// from the beginning of markdown content. It strips markdown syntax (headers,
// links, images, code fences, formatting markers) and extends to the next space
// so words are never cut in the middle.
// The result is safe for use in HTML attributes — Go's html/template will
// auto-escape &, <, >, ", '.
func extractDescription(content string, maxChars int) string
```

**实现策略**：
1. 逐行处理，跳过代码块（```...```）
2. 去掉 Markdown 标记（`#`, `**`, `_`, `[text](url)`, `![alt](src)`）
3. 合并连续空白为单个空格
4. 截取前 `maxChars` 字符
5. 找到最后一个空格，截断到该位置（避免单词截断）
6. 返回纯文本字符串

---

## 探针总表

| 必做 ID | PRD 效果 | 探针（Probe） | 关联 BDD |
|---------|---------|--------------|----------|
| M-1 | CLI push/pipe 终端输出 `https://fastmd.dev/<id>.md` | CLI 零改动——API 返回 `.md` 后缀后 CLI 自动输出 `.md` 后缀 | S-1, S-2 |
| M-2 | `POST /v1/push` 返回 `url` 以 `.md` 结尾 | `response.url` 匹配正则 `\.md$` | S-3 |
| M-3 | 浏览器访问 `/:id.md` 看到 HTML 提示页 | `Content-Type: text/html` + body 含 "To view the rendered page" + body 含 `<pre>` + 含 `og:title` / `og:description` | S-4, S-7, S-13 |
| M-4 | `Accept: text/plain` 访问 `/:id.md` 返回原始 `text/plain` | `Content-Type: text/plain` + body === 原始内容 | S-5 |
| M-5 | `og:title` 为文档 H1 标题；无 H1 退回 `fastmd/<id>` | `<meta property="og:title">` content 匹配文档 H1 | S-8, S-9, S-13 |
| M-6 | `og:description` 为文档开头摘要约 200 字符 | `<meta property="og:description">` content 长度 ≤ 210 字符且不为空 | S-10, S-11, S-12, S-13 |
| M-7 | Dashboard 文档标题可点击跳转到 `.md` 页面 | `.dashboard-doc-title` 为 `<a>` 元素，href 以 `.md` 结尾 | S-14 |
| M-8 | `GET /v1/docs` 返回 `url` 以 `.md` 结尾 | 每个 `documents[].url` 匹配正则 `\.md$` | S-15 |
| M-9 | 文档渲染页朗读按钮可朗读/暂停/继续/自动停止 | `window.speechSynthesis` 存在时按钮可见 + 点击触发 `speak()` | S-16 |

---

## 测试钩子总表

| BDD 场景 | 测试钩子 |
|---------|---------|
| S-1/S-2 | CLI 输出字符串以 `.md` 结尾（CLI 零改动，API 侧验证） |
| S-3 | `POST /v1/push` 响应 JSON `.url` 字段正则 `\.md$` |
| S-4 | `.md` HTML 响应 `Content-Type: text/html` + body 含 hint text + `<pre>` 含原始内容 |
| S-5 | `Accept: text/plain` → `Content-Type: text/plain` + body === doc.Content |
| S-6 | 不存在的 `.md` doc → HTTP 404 + `X-Robots-Tag` |
| S-7 | 旧 4 位 ID 文档 `.md` 页不崩溃（CreatedAt==0 → `—`） |
| S-8 | `og:title` content === doc H1 |
| S-9 | 无 H1 → `og:title` content === `fastmd/<id>` |
| S-10 | `og:description` 长度 ≤ ~210 字符，内容为文档开头 |
| S-11 | 极短内容（2 字符）→ `og:description` = `"Hi"`，不崩溃 |
| S-12 | `og:description` 中 `"` / `&` 被 html/template 自动转义 |
| S-13 | `.md` 提示页也含 `<meta property="og:title">` 和 `og:description` |
| S-14 | Dashboard `.dashboard-doc-title a` href 以 `.md` 结尾 |
| S-15 | `GET /v1/docs` 每个 `documents[].url` 以 `.md` 结尾 |
| S-16 | 朗读按钮存在（speechSynthesis 支持时）+ 按钮状态变化（朗读→暂停→继续→朗读） |

---

## 测试命令（Repo 回归）

```bash
export PATH="/opt/homebrew/bin:/usr/local/go/bin:$HOME/go/bin:$PATH"
go test ./cmd/server/ ./tests/ -v
```

`tests/` 目录：`tests/main_test.go` — 集成测试（render/store 包）；`cmd/server/main_test.go` — 单元测试（extractDescription/extractTitle 等纯函数 + httptest 端点测试）。

---

## 风险与预案

| 风险 | 概率 | 影响 | 预案 |
|------|------|------|------|
| Agent 不设 `Accept: text/plain`，从 text/plain 切 HTML 后解析失败 | 低 | 中 | ① 文档声明 Agent 加 `Accept: text/plain`；② `<script type="text/markdown">` 嵌入原始内容 |
| 旧 4 位 ID 文档 `CreatedAt = 0` 在 hint 页显示异常 | 高（历史数据） | 低 | `formatCreatedAt` 返回 `""`，模板显示 `—`（与 doc 页相同处理） |
| `extractDescription` 去标记后摘要可读性差 | 中 | 低 | 上线后抽样评估；PRD D-2 已暂缓 AI 摘要方案 |
| Go 模板自动转义破坏社交平台解析 | 极低 | 低 | `html/template` 的 `&quot;` / `&amp;` 是所有 HTML 解析器（含 OG 爬虫）的标准行为 |
| 新 `md-hint.html` 模板未注册 | 低 | 高 | `loadTemplates()` 编译期报错，不会静默上线 |

---

## 被砍掉的诱惑

| 想做的事 | 砍的理由（决策树第几问） | 处置 |
|---------|------------------------|------|
| `.md` 提示页展示渲染后的 Markdown（而非 `<pre>` 原始内容） | Q1：PRD 已明文标注 N-5 cost-too-high，"提示页定位是告诉人类怎么去渲染版" | 砍 |
| CLI 同时输出两个链接（`/:id` 和 `/:id.md`） | Q4：能更小做——只输出 `.md` 链接，人类在 .md 页有提示引导 | 砍 |
| `og:description` 调用 LLM 提炼摘要 | Q3：能更晚做——PRD D-2 暂缓，前 200 字符足够 | 暂缓 |
| 朗读按钮在 Safari/Firefox 做降级方案 | Q4：能更小做——PRD 明确"增强体验"，不支持则静默隐藏 | 砍 |
| 改造 `doc.html` 模板结构以适配朗读按钮 | Q1：PRD 不要求改渲染页布局，按钮嵌入已有 doc-meta 即可 | 砍 |
| 服务端生成 OG 预览图 | Q2：落入 N-2"不做"清单（defer-v2） | 砍 |

---

## 开发验收记录

| 必做 ID | 验收结果 | 探针通过 | 备注 |
|---------|---------|---------|------|
| M-1 | ✅ | CLI 零改动，M-2 驱动 | `result.URL` 来自 API 响应，API 返回 `.md` 后 CLI 自动输出 `.md` 后缀 |
| M-2 | ✅ | `absoluteURL(c, "/"+id+".md")` | curl POST /v1/push → url 以 `.md` 结尾 |
| M-3 | ✅ | `Content-Type: text/html` + body 含 hint text + `<pre>` 含原始内容 | `md-hint.html` 模板渲染 + `renderPage(200, "md-hint", ...)` |
| M-4 | ✅ | `Accept: text/plain` → `text/plain` + 原始内容 | `rawMode := acceptPlain` 保持向后兼容 |
| M-5 | ✅ | `og:title` 动态使用 H1 | `extractTitle(content, "")` → 传 `OGTitle` 到模板；无 H1 退回 `fastmd/<id>` |
| M-6 | ✅ | `og:description` 动态约 200 字符 | `extractDescription(content, 200)` → 传 `OGDescription` 到模板 |
| M-7 | ✅ | `.dashboard-doc-title` 为 `<a>` 链接 | `app.js` 改为 `innerHTML = '<a href="...">'` + `escapeHTML()` |
| M-8 | ✅ | `documents[].url` 以 `.md` 结尾 | `absoluteURL(c, "/"+doc.ID+".md")` |
| M-9 | ✅ | 朗读按钮可见 + 状态循环 | `window.speechSynthesis` 检测 → 显示按钮；`speak()`/`pause()`/`resume()` 状态机 |

---

## Anchor 自查

| 自查项 | 结果 |
|--------|------|
| 所有改动都能映射回 Goal Anchor"必做"？ | ✅ M-1~M-9 全部对应 |
| 没有悄悄新增 PRD 没要求的功能？ | ✅ 无新路由、无新 API、无新存储字段、无新依赖 |
| 每个 BDD 场景都有测试钩子？ | ✅ 16 个场景全覆盖 |
| "被砍掉的诱惑"≥ 1 条？ | ✅ 6 条 |
