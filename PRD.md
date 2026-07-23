# fastmd v0.4 — 分享链接体验升级 PRD

> 由 PM agent v1.8.0 撰写于 2026-07-16

---

## 0. 产品与需求背景

### 0.1 产品形态

- **形态**：Web 工具 + CLI 工具 + Skill
- **部署方式**：独立网站（https://fastmd.dev）
- **备注**：主形态 Web 服务 + CLI 客户端 + Agent Skill，三者共享同一 API

### 0.2 产品定位与价值

- **一句话定位**：为开发者和 AI Agent 提供一个 CLI-first 的 Markdown 发布管道——终端输入内容，即刻获得可分享的链接。
- **核心价值**：零注册、零配置，一行命令把 Markdown 变成可分享的链接。Agent 自动发布长文本，人类一键打开阅读。
- **北极星指标**：月活跃发布数（Monthly Active Pushes），当前 MVP 阶段暂不设硬指标。

### 0.3 本次需求

- **背景**：当前 CLI/API 发布后返回的链接指向渲染版（`/:id`），而 Agent 消费和手动取回（`fastmd get`）使用原始版（`/:id.md`），两种链接并存造成混淆。同时，分享到 Slack/Discord 等平台的文档预览用的是固定文案（"Shared Markdown document on fastmd."），无法反映文档实际内容。
- **本次要做什么**：① 统一所有发布渠道输出 `.md` 后缀链接；② `.md` 页面升级为带提示的 HTML 页面；③ 文档页面 OG 标签动态化为文档的实际标题和内容摘要。
- **本次目标**：
  - 发布链接统一为 `https://fastmd.dev/<id>.md`
  - 人类点击 `.md` 链接能看懂这是什么、知道怎么去渲染版
  - 社交平台分享文档链接能展示文档的实际标题和内容预览
- **不在本次范围**：修改 `/:id` 渲染页面样式、新增编辑功能、自定义 OG 图片生成、修改 `GET /v1/docs` 返回格式

---

## Goal Anchor

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
  4. `GET /v1/docs` API 返回的链接格式变更（理由：out-of-scope，dashboard 链接保持 `/:id` 以便人类直接打开）
- **成功判据**：本版发布门禁全部验证通过。

---

## 发布门禁（Definition of Done）

| 必做 ID | 效果（Outcome） | 验证（Verification） |
|---------|-----------------|----------------------|
| M-1 | CLI `push` / 管道输入后终端输出 `✓ Published → https://fastmd.dev/<id>.md` | 人工：执行 `echo "# test" \| fastmd`，观察输出 URL 以 `.md` 结尾 |
| M-2 | `POST /v1/push` 返回的 `url` 字段值为 `https://fastmd.dev/<id>.md` | 自动化：curl POST 带有效 content/token，检查 `response.url` 匹配正则 `\.md$` |
| M-3 | 浏览器访问 `/:id.md` 看到 HTML 页面，含提示文字"查看网页渲染版：去掉 `.md"和 `<pre>` 包裹的原始 Markdown | 自动化：curl（不设 Accept 头）访问 `/:id.md`，检查 `Content-Type` 为 `text/html`，body 包含提示文字和原始文档内容 |
| M-4 | `curl -H "Accept: text/plain"` 访问 `/:id.md` 仍返回 `text/plain` 原始 Markdown，与当前行为一致 | 自动化：curl `-H "Accept: text/plain"` `/:id.md`，检查 `Content-Type: text/plain`、body 与原始内容完全一致 |
| M-5 | 文档 `og:title` 为该文档首个 H1 标题内容；无 H1 时退回到 `"fastmd/<id>"` | 自动化：发布含 `"# Hello World"` 的文档，curl `/:id`，检查 `<meta property="og:title">` 内容为 `"Hello World"`；发布无 H1 的文档验证退回格式 |
| M-6 | 文档 `og:description` 为文档内容的开头摘要（约 200 字符，去除 Markdown 标记），社交平台预览可看到实际内容 | 自动化：发布含已知开头文本的文档，curl `/:id`，检查 `<meta property="og:description">` 内容包含该文本开头；验证 HTML 实体（如 `"`、`&`）被正确转义 |

**NO-GO 条件**：任一必做项的验证未执行或效果未达。

---

## 一、背景

### 用户原始诉求

**需求 1**：当前 CLI `push` 输出 `https://fastmd.dev/<id>`（渲染版链接），但 Agent 场景中经常需要原始 Markdown。同时，`fastmd get <id>` 内部使用 `/<id>.md`。发布链接和取回链接不一致，造成认知负担。希望统一所有发布渠道输出 `.md` 后缀链接，并在 `.md` 页面上加提示引导人类用户去渲染版。

**需求 2**：当前文档分享到 Slack / Discord / Twitter 时，预览卡片标题是 `"fastmd/<id> | fastmd.dev"`，描述是 `"Shared Markdown document on fastmd."`——无法反映文档实际内容，大量分享链接看起来完全一样。希望 OG 标签能动态展示文档的标题和内容摘要。

### 解决的问题

发布链接统一 + 社交预览不再千篇一律，提升分享体验的"第一眼信息量"。

---

## 二、第一性原理拆解

### 需求 1：链接统一 & `.md` 页面增强

- **真问题**：发布者（人或 Agent）需要一种"万能链接"——Agent 拿到能直接消费原始内容，人类点击不会被 raw text 吓到。
- **现状痛点**：
  - CLI/API 输出 `/:id` → Agent 拿到后要多一步 `curl /:id.md` 才能拿原始内容
  - `/.md` 返回纯文本 → 人类在浏览器打开看到一堆 Markdown 源码，不知所措
  - 强度：中 | 频率：每次发布都在发生
- **真约束**：
  - Agent 消费者必须能拿到 `text/plain` 原始内容（不能破坏现有 Agent 集成）
  - 浏览器默认 Accept 头包含 `text/html`（可据此区分人类/机器）
- **假约束**（标注）：
  - "`.md` 必须是 text/plain" → 假。HTTP 内容协商可根据 Accept 头返回不同格式
  - "统一输出 `.md` 链接会让人类困惑" → 假。加上提示文字即可消除困惑
- **最小可行解**：
  - 改动三处输出（CLI / API / skill）统一加 `.md` 后缀
  - `.md` 路由做内容协商：浏览器（无 `Accept: text/plain`）→ HTML 提示页 + `<pre>` 包裹原始内容；Agent（`Accept: text/plain`）→ 保持现有 `text/plain` 行为
  - 砍掉了：新建独立提示页路由、渲染版内嵌提示、保留 `/id` 和 `/id.md` 双输出（只输出 `.md`）
- **关键假设**：
  - 现有 Agent（Claude Code / OpenCode / Codex）访问 `.md` 时已设置 `Accept: text/plain` ✅ 已验证（handler 代码中 `.md` 后缀直接触发 rawMode，且 `Accept: text/plain` 也是触发条件，两者叠加无问题）
  - 浏览器默认 Accept 头包含 `text/html` ✅ 已验证（所有主流浏览器标准行为）
  - 人类看到提示文字后会理解并操作 ⚠ 未验证——需上线后观察 `.md` 页面访问量 vs `/:id` 页面访问量变化

### 需求 2：动态 OG 标签

- **真问题**：分享出去的链接在社交平台预览完全一样，接收者无法从预览判断"要不要点开"。
- **现状痛点**：所有文档的 OG 标签都是固定文案。Slack 里一堆 fastmd 链接，预览卡片完全无法区分。强度：中 | 频率：每次分享到支持 unfurl 的平台
- **真约束**：OG 标签必须在服务端渲染（社交平台爬虫不执行 JS）
- **假约束**（标注）：
  - "OG 标签需要前端 JS 生成" → 假。Go 模板渲染即可
  - "需要新增数据库字段存 OG 信息" → 假。可从 `Content` 字段实时提取
- **最小可行解**：
  - `og:title` = 从文档内容提取第一个 `# ` H1 标题（已有 `extractTitle()`）
  - `og:description` = 文档内容前约 200 字符，去除 Markdown 标记和换行
  - 所有改动仅在 handler 层增加数据提取 + 模板传参
  - 砍掉了：摘要 AI 生成、用户自定义 OG、独立 OG 图片生成服务
- **关键假设**：
  - 文档通常有 H1 ✅ 已部分验证（`extractTitle()` 已用于 dashboard 列表）
  - 前 200 字符去 Markdown 后仍有可读摘要 ⚠ 未验证——如果是代码块或链接开头，摘要可能不好看

---

## 三、目标用户

- **主用户**：通过 CLI / API / skill 发布内容的开发者和 AI Agent——他们希望发布的链接在各种场景下"该谁用就谁用"（Agent 拿 raw，人类看渲染，社交平台展示预览）。
- **次用户**：文档接收者——通过社交平台或聊天工具点开分享链接的人，他们希望第一眼就知道链接内容是什么。
- **不服务**：需要自定义社交预览卡片样式的品牌运营（fastmd 不做 CMS）。

---

## 四、用户流程

### 流程 A：发布 → 分享 → 被消费（需求 1）

1. 用户（人或 Agent）通过 CLI / API / skill 发布 Markdown 内容
2. 获得链接：`https://fastmd.dev/abc12345.md`
3. **分支 A**：用户把链接贴到聊天窗口 / Issue / 代码注释
   - 另一个 Agent 读取链接 → 请求 `/:id.md` + `Accept: text/plain` → 拿到原始 Markdown 直接消费
   - 代码中的 `curl` 或 `wget` → 同上
4. **分支 B**：人类收到链接，在浏览器打开
   - 浏览器请求 `/:id.md`（默认 Accept 含 `text/html`）
   - 服务端返回 HTML 页面：顶部提示栏"💡 查看网页渲染版：去掉地址栏里的 `.md`"，下方 `<pre>` 区域展示原始 Markdown
   - 人类可：① 手动去掉 URL 末尾的 `.md` 看渲染版 → ② 或直接阅读 `<pre>` 中的原始内容

### 流程 B：社交平台预览（需求 2）

1. 用户在 Slack / Discord / Twitter / iMessage 粘贴 `https://fastmd.dev/abc12345.md`
2. 平台爬虫请求该 URL（使用无 Accept 头的 GET，或 `Accept: */*`）
3. 服务端（`.md` 路由或 `/:id` 路由）返回 HTML，`<head>` 中包含动态 OG 标签：
   - `og:title` = 文档的 H1 标题
   - `og:description` = 文档前 ~200 字符摘要
4. 平台展示带标题和内容摘要的预览卡片，而非千篇一律的 "Shared Markdown document"

> **注意**：社交平台爬虫访问的是 `.md` 链接还是 `/:id` 链接，取决于发布链接格式。本期统一输出 `.md` 链接，因此 OG 标签在 `.md` HTML 响应中也需要包含（见 M-3/M-5/M-6 的交叉覆盖）。

---

## 五、BDD 验收标准

### 需求 1：链接统一 & `.md` 页面增强

```gherkin
Scenario: S-1 CLI 管道发布输出 .md 后缀链接
  Given 用户本地有 Markdown 内容 "hello world"
  When 用户执行 echo "hello world" | fastmd
  Then 终端输出 "✓ Published → https://fastmd.dev/<id>.md"
  And 链接以 .md 结尾

Scenario: S-2 CLI push 文件输出 .md 后缀链接
  Given 用户本地有文件 /tmp/test.md 包含 Markdown 内容
  When 用户执行 fastmd push /tmp/test.md
  Then 终端输出 "✓ Published → https://fastmd.dev/<id>.md"
  And 链接以 .md 结尾

Scenario: S-3 API push 返回 .md 后缀链接
  Given 有效的 content 和 token
  When 客户端 POST /v1/push 发送 {"content":"...", "token":"..."}
  Then 响应 JSON 中 url 字段值为 "https://fastmd.dev/<id>.md"
  And 响应 HTTP 状态码 200

Scenario: S-4 浏览器访问 .md 页面看到提示和原始内容
  Given 存在文档 abc12345，内容为 "# Hello\n\nWorld"
  When 浏览器（默认 Accept 头）请求 GET /abc12345.md
  Then 响应 Content-Type 为 text/html; charset=utf-8
  And 页面 body 中包含提示文字 "查看网页渲染版"
  And 页面 body 中包含 "去掉 .md"（或语义等价的指引）
  And 页面中包含原始 Markdown 内容 "# Hello\n\nWorld"（在 <pre> 标签内）

Scenario: S-5 Agent Accept: text/plain 保持原有行为（向后兼容）
  Given 存在文档 abc12345
  When Agent 请求 GET /abc12345.md，Header 含 Accept: text/plain
  Then 响应 Content-Type 为 text/plain; charset=utf-8
  And 响应 body 为原始 Markdown 内容（与当前行为完全一致）
  And 响应不包含任何 HTML 标签

Scenario: S-6 不存在的 .md 文档返回 404
  Given 文档 ID "zzzzzzzz" 不存在
  When 浏览器访问 GET /zzzzzzzz.md
  Then 响应 HTTP 状态码 404
  And 响应为 HTML 404 页面（使用 base.html + 404.html 模板）
  And X-Robots-Tag 为 "noindex, nofollow, noarchive"

Scenario: S-7 .md 提示页对旧文档（无 CreatedAt）兼容
  Given 存在旧格式 4 位 ID 文档，创建时间未记录
  When 浏览器访问 GET /<old_id>.md
  Then 提示文字正常显示
  And 原始 Markdown 内容正常显示
  And 页面不崩溃、不显示 "0001-01-01"
```

### 需求 2：动态 OG 标签

```gherkin
Scenario: S-8 HTML 渲染页 og:title 使用文档 H1
  Given 文档内容首行 "# 性能优化指南"
  When 客户端（或爬虫）请求 GET /<id>
  Then 响应 HTML 中包含 <meta property="og:title" content="性能优化指南">
  And <title> 标签也包含 "性能优化指南"
  And <meta name="twitter:title"> 也包含 "性能优化指南"

Scenario: S-9 文档无 H1 时 og:title 退回默认格式
  Given 文档内容不含 "# " 开头的行（无 H1）
  When 客户端请求 GET /<id>
  Then 响应 HTML 中 <meta property="og:title" content="fastmd/<id> | fastmd.dev">
  And 退回格式与当前行为一致

Scenario: S-10 og:description 使用文档内容开头摘要
  Given 文档内容为 "本文介绍如何在生产环境中部署 fastmd 服务，包括依赖安装……（后续长内容）"
  When 客户端请求 GET /<id>
  Then 响应 HTML 中 <meta property="og:description"> 的内容包含 "本文介绍如何在生产环境中部署 fastmd 服务"
  And 描述内容长度约 200 字符（不严格截断单词中间）

Scenario: S-11 og:description 处理空内容 / 极短内容
  Given 文档内容为 "Hi"（仅 2 字符）
  When 客户端请求 GET /<id>
  Then og:description 内容为 "Hi"（不会崩溃或输出空白）
  And 长度不超过实际内容长度

Scenario: S-12 og:description HTML 实体转义
  Given 文档内容开头包含双引号 " 和 & 符号
  When 客户端请求 GET /<id>
  Then og:description 中的 " 被转义为 &quot;
  And & 被转义为 &amp;
  And 不影响社交平台正常解析

Scenario: S-13 .md 提示页也包含动态 OG 标签
  Given 文档含 H1 "部署指南"
  When 浏览器（或社交平台爬虫）请求 GET /<id>.md（无 Accept: text/plain）
  Then 返回的 HTML .md 提示页中也包含 <meta property="og:title" content="部署指南">
  And 包含 <meta property="og:description">（内容摘要）
```

---

## 六、MVP 边界（三栏）

### 必做

| ID | 内容 | 关联 BDD |
|----|------|----------|
| M-1 | CLI push/pipe 输出链接以 `.md` 结尾 | S-1, S-2 |
| M-2 | API `POST /v1/push` 返回的 `url` 字段以 `.md` 结尾 | S-3 |
| M-3 | 浏览器访问 `/:id.md` 返回 HTML 提示页（提示文字 + `<pre>` 原始内容 + OG 标签） | S-4, S-7, S-13 |
| M-4 | Agent `Accept: text/plain` 访问 `/:id.md` 仍返回原始 text/plain（向后兼容） | S-5 |
| M-5 | `og:title` 动态使用文档 H1 标题 | S-8, S-9, S-13 |
| M-6 | `og:description` 动态使用文档内容开头摘要（~200 字符） | S-10, S-11, S-12, S-13 |

### 不做

| ID | 内容 | 理由分类 | 详细理由 |
|----|------|----------|----------|
| N-1 | 修改 `/:id` 渲染页面样式、布局或功能 | out-of-scope | 本次聚焦链接统一和 OG 标签，不碰渲染页 UI |
| N-2 | 自定义 OG 社交预览图（如文档标题渲染为图片） | defer-v2 | 涉及额外图片生成服务或前端 Canvas 渲染，需独立评估 |
| N-3 | 允许用户手动设定 OG 标题/描述 | defer-v2 | fastmd 不做内容编辑功能，与产品定位冲突 |
| N-4 | `GET /v1/docs` API 返回链接格式变更 | out-of-scope | Dashboard 是管理工具，保持 `/:id` 格式方便人类管理员直接打开 |
| N-5 | `.md` 提示页展示渲染后的 HTML（而非 `<pre>` 原始内容） | cost-too-high | 提示页目的是让人类知道怎么去渲染版；如果提示页本身就是渲染版，就失去了"原始内容"的定位 |

### 暂缓

| ID | 内容 | 触发条件（什么时候做） |
|----|------|------------------------|
| D-1 | `.md` 提示页加入一键跳转按钮（点击直接跳到渲染版） | 当提示页访问量 / 渲染页访问量 > 30%（说明大量人类在 `.md` 页手动改 URL） |
| D-2 | `og:description` 使用 AI 提炼的摘要（而非截断前 200 字符） | 当用户反馈截断摘要可读性差，且产品决策支持引入 LLM 调用 |

---

## 七、指标

### 北极星指标
- **指标名**：月活跃发布数（Monthly Active Pushes）
- **当前阶段**：MVP，暂不设硬指标
- **测量方式**：统计 `POST /v1/push` 的去重 `token_hash` 数量

### 本轮关注指标（非北极星，观测用）
| 指标 | 说明 | 观测方式 |
|------|------|----------|
| `.md` 页面访问量 vs `/:id` 访问量 | 衡量多少人从 `.md` 跳到了渲染版，验证提示的有效性 | 服务端日志按路由统计 |
| 社交平台 referral 流量变化 | 动态 OG 标签是否带来更多点击 | Referer header 分析 |

### 守护指标（不能下降）
| 指标 | 当前 | 警戒线 |
|------|------|--------|
| `POST /v1/push` 成功率 | ~100% | < 99.9% |
| `GET /:id.md` (Accept: text/plain) 响应时间 | < 50ms | > 200ms |
| Agent 集成可用性（skill 发布成功率） | ~100% | < 99% |

---

## 八、5 维度评估

| 维度 | 评分 (1-5) | 说明 |
|------|-----------|------|
| 问题明确度 | 5 | 两个需求边界清晰，trade-off 明确，有明确的后向兼容方案 |
| 用户匹配度 | 5 | 三类核心用户（开发者、AI Agent、文档接收者）均直接受益 |
| 产品对齐度 | 5 | 强化"发布 → 分享 → 消费"链路，与 CLI-first 定位一致 |
| 用户价值 | 4 | 链接统一减少认知负担；OG 标签提升分享第一印象。但不改变核心使用体验 |
| 实现成本 | 4 | Handler 逻辑调整 + CLI 改一行 + skill 自动跟随 + 新建提示页模板。无存储变更、无新依赖 |

**总分：23 / 25 → P0 立即做**

---

## 九、风险与未验证假设

| 风险/假设 | 可能性 | 影响 | 预案/验证方式 |
|-----------|--------|------|---------------|
| 极少数 Agent 不设置 `Accept: text/plain`，从 `text/plain` 切 HTML 后解析失败 | 低 | 中 | ① 文档明确声明 Agent 应加 `Accept: text/plain`；② 在 `.md` HTML 中用 `<script type="text/markdown" id="raw-content">` 嵌入原始内容作为备选提取路径；③ 线后监控 `.md` 请求的 User-Agent 分布 |
| 文档无 H1 情况比预期多，`og:title` 频繁退回默认格式 | 中 | 低 | `extractTitle()` 已用于 dashboard，可统计历史文档 H1 覆盖率，若 < 60% 则考虑用文档开头第一句话作为备选 |
| 去 Markdown 标记后的前 200 字符摘要可读性差（如以代码块或链接开头） | 中 | 低 | 上线后人工抽样 20 条分享预览截图评估；如需改进，后续迭代增加"跳过代码块/链接取第一段正文"逻辑 |
| `.md` 后缀链接在某些聊天平台（如微信）不被识别为可点击链接 | 低 | 中 | 验证 `.md` TLD 后缀在各主流 IM 平台的可点击性；微信不支持 → 降级方案：同时输出短链接 `fm.dev/<id>`（不在本期范围） |

---

## 十、平台特定检查

### Web
- [x] 响应式断点已确认 — `.md` 提示页沿用 `base.html` 响应式框架
- [x] SEO 元数据已规划 — `/:id` 和 `/:id.md`（HTML 版）均带 `noindex, nofollow, noarchive`
- [x] CWV 目标值 — `.md` 提示页无 JS 交互，LCP 为文字内容，目标 LCP < 1.5s

### CLI
- [x] 向后兼容 — 仅修改输出 URL 格式，CLI 参数和子命令不变
- [x] 管道模式 — `echo "..." | fastmd` 和 `fastmd push <file>` 两种入口均已覆盖

### Skill
- [x] 无需修改 skill 代码 — `publish.sh` 解析 API 响应中的 `url` 字段，API 返回 `.md` 后缀后 skill 自动获得

---

## Anchor 自查

- [x] 本文档所有内容是否都映射到 Goal Anchor 的「必做」？
      M-1 → S-1/S-2 | M-2 → S-3 | M-3 → S-4/S-7/S-13 | M-4 → S-5 | M-5 → S-8/S-9/S-13 | M-6 → S-10/S-11/S-12/S-13
- [x] 本文档是否引入了 Goal Anchor 「不做」清单里的内容？
      否。N-1 到 N-5 均在正文中明确排除。
- [x] 本文档是否新增了未在 Goal Anchor 里的目标？
      否。所有 BDD 场景和必做项均可映射回 Goal Anchor 6 条必做。
- [x] 成功判据是否 = 「发布门禁全部验证通过」？
      是。发布门禁 M-1 到 M-6 全部验证通过 = 本版成功。

---

## 逻辑自洽自检（强制，PM 写完后从头读一遍逐项打勾）

- [x] 0.2 定位与价值能解释 0.3 本次需求为什么值得做？
      是。"CLI-first 发布管道"的核心是"发布 → 拿到链接 → 分享"。本次让链接在分享场景中无缝工作（Agent 拿 raw、人类看提示、社交平台展预览），直接强化核心价值。
- [x] 0.3 本次目标能映射到 Goal Anchor 的"必做"？
      是。目标 1"链接统一"→ M-1/M-2；目标 2"人类看懂 .md"→ M-3/M-4；目标 3"社交预览有意义"→ M-5/M-6。
- [x] Goal Anchor 的每条"必做"在「发布门禁」都有对应行（效果+验证）？
      是。M-1 到 M-6 六条全部对应。
- [x] 每个 BDD 场景都能映射回 0.3 本次目标？
      是。S-1~S-7 → 目标 1+2；S-8~S-13 → 目标 3。
- [x] MVP 三栏的"不做"清单与 0.3 的"不在本次范围"一致？
      是。N-1~N-5 与 0.3 "不在本次范围"四条目一致（N-5 是 0.3 未显式列出的技术边界细化）。
- [x] 全文术语统一（同一概念不出现多个名字）？
      是。核心术语："发布"=push、"原始内容"=raw Markdown、"提示页"=".md HTML 页面、"渲染版"=`/:id` 页面、"OG 标签"=Open Graph 元数据、"摘要"=og:description。
- [x] 发布门禁无「只有探针、没有效果描述」的行？
      是。每条门禁均以产品语言描述用户可观察的效果（如"终端输出以 .md 结尾"、"看到提示文字和原始内容"、"社交平台预览展示文档标题"）。

✅ 自洽自检通过（7/7）

---

## 被砍掉的诱惑

| 想做的事 | 砍的理由（决策树第几问） | 处置 |
|---------|-------------------------|------|
| `.md` 提示页不仅提示、还展示语法高亮的渲染内容（即把 `.md` 页面搞得和 `/:id` 几乎一样） | Q4：能更小做——提示页的定位是"告诉人类怎么去渲染版"，而不是"变成另一个渲染版"。若提示页本身就是完整渲染，就失去了"原始内容入口"的语义 | 砍，仅保留提示 + `<pre>` 原始内容 |
| CLI 输出同时给两个链接：`/:id` 和 `/:id.md` | Q4：能更小做——统一输出 `.md` 即可，人类在 `.md` 页有提示引导去渲染版。同时给两个链接增加终端输出的噪音 | 砍，只输出 `.md` 链接 |
| 文档 `og:description` 使用 AI 提炼摘要 | Q3：能更晚做——前 200 字符截断在大部分场景下足够，AI 摘要需要引入 LLM 调用成本和延迟，当前阶段不必要 | 暂缓到 D-2 |
| 在 CLI 输出中加入彩色 emoji 或格式化 | Q1：与北极星无关——"让链接在各种场景正确呈现"不要求美化终端输出 | 砍 |

---

**PRD 状态：定稿。✅**（v0.4 主需求 + 补充需求 3 + 补充需求 4）

---

## 补充需求 3：Dashboard 文档标题链接化（追加于 2026-07-18）

> 由 PM agent v1.8.0 撰写

### 背景

当前 dashboard 页面文档列表中，文档标题是纯文本（`textContent`），用户无法直接点击跳转。文档 URL 以单独一行展示，可点击打开新标签页。两次点击才能到达文档，体验不直观。

### 需求

1. Dashboard 文档列表中的每个文档标题改为**可点击链接**，点击跳转到文档详情页
2. Dashboard 中的文档链接默认使用 `.md` 后缀（跟随需求 1 的 M-2：API `POST /v1/push` 已返回 `.md` 后缀 URL，dashboard 调用 `GET /v1/docs` API 获取列表，需确认该 API 的 `url` 字段也同步改为 `.md` 后缀）

### 现有实现分析

- Dashboard 文档列表在 `web/static/app.js:200-244` 动态渲染
- 标题渲染：`<h3 class="dashboard-doc-title"></h3>` + `textContent = title`（纯文本，不可点击）
- URL 渲染：`<a class="dashboard-doc-link" href="doc.url">`（单独的链接行）
- 当前 `GET /v1/docs` API 返回的 `url` 字段不带 `.md`（见 `cmd/server/main.go:350`，使用 `absoluteURL(c, "/"+doc.ID)`）

### 改动范围

- **`web/static/app.js`**：把 `.dashboard-doc-title` 的纯文本改为 `<a>` 链接，href 指向 `doc.url`（打开方式由用户控制，不强制新窗口）
- **`cmd/server/main.go`**：`GET /v1/docs` handler 的 `url` 字段也加 `.md`（`absoluteURL(c, "/"+doc.ID+".md")`），与 `POST /v1/push` 保持一致

### 新增 BDD 场景

```gherkin
Scenario: S-14 Dashboard 文档标题可点击跳转
  Given 用户在 Dashboard 已登录并看到文档列表
  When 用户点击某个文档的标题
  Then 浏览器打开该文档（.md 后缀链接，默认当前标签页）
  And 文档标题为 <a> 元素，href 指向文档 .md 链接

Scenario: S-15 Dashboard 文档列表链接使用 .md 后缀
  Given 用户在 Dashboard 查看文档列表
  When 页面渲染文档列表
  Then 每个文档的 URL 链接以 .md 结尾
  And 每个文档标题链接也以 .md 结尾
```

### 新增必做项

| ID | 内容 | 关联 BDD |
|----|------|----------|
| M-7 | Dashboard 文档标题改为可点击链接，跳转到 `.md` 页面 | S-14 |
| M-8 | `GET /v1/docs` API 返回的 `url` 字段以 `.md` 结尾 | S-15 |

### 发布门禁（补充需求 3）

| 必做 ID | 效果（Outcome） | 验证（Verification） |
|---------|-----------------|----------------------|
| M-7 | Dashboard 文档列表中，每个文档标题是可点击的链接，点击后跳转到该文档的 `.md` 页面 | 人工：登录 Dashboard 查看文档列表，点击任意文档标题，确认浏览器跳转到正确的 `.md` 页面 |
| M-8 | `GET /v1/docs` API 返回的 `url` 字段为 `.md` 后缀格式（如 `https://fastmd.dev/abc12345.md`） | 自动化：curl 带有效 token 请求 `GET /v1/docs`，检查每个 `documents[].url` 匹配正则 `\.md$` |

### 风险/注意事项

- M-8 与 v0.4 原 PRD 的 N-4（"GET /v1/docs API 返回链接格式不变"）冲突 —— 此补充需求**覆盖 N-4**，N-4 应标记为废弃。理由：当 CLI/API push 都输出 `.md` 链接后，dashboard 仍给 `/:id` 链接会导致用户从 dashboard 点进去看到渲染版、而分享出去的是 `.md` 版，造成新的不一致。
- Dashboard 标题链接默认在当前标签页打开（`target="_self"`），与下方 URL 链接（`target="_blank"`）行为不同。这符合用户预期：标题是主要导航动作，URL 链接是辅助复制/新窗口打开的入口。

---

## 附录：调研 —— 文档中嵌入视频/音频的技术方案及开源 TTS（2026-07-18）

> 以下调研结果供后续版本规划参考，非当前迭代范围。

### A.1 Markdown 中嵌入视频的技术方案

#### A.1.1 goldmark 现状

fastmd 使用 `goldmark` 作为 Markdown 渲染引擎，当前配置已启用 `html.WithUnsafe()`（`internal/render/render.go:23`）。这意味着 **goldmark 已原生支持 `<video>` 标签的直通渲染** —— 用户在 Markdown 中直接写 HTML `<video>` 标签即可在渲染页播放视频：

```markdown
<video src="https://example.com/video.mp4" controls width="640"></video>
```

- **无需额外依赖**：`html.WithUnsafe()` 允许所有原始 HTML 标签通过
- **注意**：goldmark 默认不渲染原始 HTML，`WithUnsafe()` 是显式开启的安全开关。当前已开启，无额外工作。

#### A.1.2 社区扩展方案

除了直接写 HTML，社区有多种"类 Markdown 语法自动转视频标签"的 goldmark 扩展：

| 扩展 | 方式 | 语法示例 | 成熟度 |
|------|------|----------|--------|
| **[goldmark-enclave](https://github.com/quailyquaily/goldmark-enclave)** | 复用 `![](url)` 图片语法 | `![](https://youtu.be/dQw4w9WgXcQ)` → YouTube 嵌入；`![](https://cdn1.suno.ai/xxx.mp3)` → `<audio>` 标签 | 较成熟（52 commits，MIT 协议），支持 YouTube/Bilibili/Spotify/HTML5 audio |
| **[goldmark-embed](https://github.com/13rac1/goldmark-embed)** | 专用扩展 | YouTube 链接自动转嵌入 | 较简单，仅 YouTube |
| 自定义 `ast.Node` + `renderer.NodeRenderer` | 完全自定义 | 任意语法 → 任意 HTML | 需自己开发 |

#### A.1.3 推荐方案

**短期（零成本）**：告诉用户在 Markdown 中直接写 `<video>` / `<audio>` 标签即可，当前 `WithUnsafe()` 已支持。

**中期（如果用户觉得手写 HTML 太麻烦）**：引入 `goldmark-enclave` 扩展，让 `![](video-url)` 自动变成对应的嵌入组件。需要：
- `go get github.com/quailyquaily/goldmark-enclave`
- 在 `render.go` 中注册 `enclave.New()` 扩展
- 评估安全风险：enclave 会根据 URL 域名匹配第三方嵌入（YouTube/Bilibili 等），需要确认这些第三方 iframe 嵌入是否可接受

### A.2 音频嵌入方案

与视频相同，当前有两个层级：

1. **零成本（已可用）**：用户在 Markdown 中写 `<audio>` 标签
   ```markdown
   <audio src="https://example.com/audio.mp3" controls></audio>
   ```
2. **便捷语法（需开发）**：引入 `goldmark-enclave` 后，`![](https://example.com/audio.mp3)` 自动转为 `<audio>` 标签（enclave 已支持 HTML5 audio）

### A.3 开源免费 TTS / 音频生成方案

#### A.3.1 主流方案对比

| 方案 | 语言 | 开源协议 | 中文支持 | 部署方式 | 质量 | 速度 | 备注 |
|------|------|----------|----------|----------|------|------|------|
| **[Piper](https://github.com/rhasspy/piper)** | C++ | MIT | ✅（多音色） | 本地二进制 / Go binding | ★★★★ | 极快 | 已归档，维护迁移至 [OHF-Voice/piper1-gpl](https://github.com/OHF-Voice/piper1-gpl)（GPL）；Home Assistant 官方 TTS 引擎 |
| **[Edge TTS](https://github.com/rany2/edge-tts)** | Python | GPLv3 | ✅（高质量） | 需网络（调微软接口） | ★★★★★ | 快 | 免费使用微软 Edge 语音服务，无需本地 GPU，中文质量最好 |
| **[Bark](https://github.com/suno-ai/bark)** | Python | MIT | ✅（多语言） | 本地（需 GPU） | ★★★★ | 慢 | Suno AI 出品，可生成非语言声音（笑声、音乐），但推理很慢（10s 音频需数分钟） |
| **[F5-TTS](https://github.com/SWivid/F5-TTS)** | Python | MIT | ✅ | 本地（需 GPU） | ★★★★★ | 中等 | 2024 新秀，Flow Matching 技术，支持语音克隆，质量接近商业产品 |
| **[XTTS (Coqui)](https://github.com/coqui-ai/TTS)** | Python | 非商业受限 | ✅ | 本地（需 GPU） | ★★★★★ | 中等 | Coqui 公司已关闭，代码仍可用但不再维护；支持语音克隆 |
| **[OpenVoice](https://github.com/myshell-ai/OpenVoice)** | Python | MIT | ✅ | 本地（需 GPU） | ★★★★ | 快 | 专注于语音克隆，MIT 协议友好 |
| **[Kokoro](https://github.com/nickvandoorn/kokoro-tts)** | Python | MIT | ❌（英文为主） | 本地（CPU可用） | ★★★★ | 快 | 2025 新方案，82M 参数，CPU 可运行 |

#### A.3.2 推荐排名（综合考虑免费、质量、部署难度、中文支持）

| 排名 | 方案 | 适合场景 |
|------|------|----------|
| **1** | Edge TTS | **快速集成中文 TTS**：调用微软免费接口，质量高，无需 GPU。适合 fastmd 集成（Go 服务调用 Python 脚本或直接 HTTP 调微软接口） |
| **2** | Piper | **本地离线中文 TTS**：MIT → GPL 迁移中，C++ 实现可编译为 Go CGO 扩展，CPU 友好 |
| **3** | F5-TTS | **最高质量 + 开源**：适合需要顶级音质和声音克隆的场景，但需要 GPU |
| **4** | Bark | **创意音频生成**：适合需要非语言声音的场景，但推理太慢不适合实时 |

### A.4 可行性建议

#### A.4.1 视频/音频嵌入在 fastmd 中的实现成本

| 层级 | 方案 | 实现成本 | 说明 |
|------|------|----------|------|
| **Level 0（已可用）** | 文档告知用户可用 `<video>` / `<audio>` HTML 标签 | 0（仅文档） | 当前 `WithUnsafe()` 已支持，只需在文档/README 中说明 |
| **Level 1（低）** | 引入 `goldmark-enclave` | ~2h | 新增 1 个 Go 依赖，~10 行代码变更，支持 `![](url)` 语法自动转视频/音频 |
| **Level 2（中）** | 自定义扩展（特定需求） | ~1-2d | 如果 enclave 的 URL 匹配规则不符合需求，可自写轻量扩展 |
| **Level 3（高）** | 内建音频生成服务器 | 数周 | 集成 TTS 引擎到 fastmd 服务器，用户 push 时自动生成音频版本 |

#### A.4.2 优先级建议

- **v0.5 可做**：Level 0（文档说明） + Level 1（goldmark-enclave 集成），让用户用 `![](audio.mp3)` 语法嵌入音频。这是"低成本高感知"的改进。
- **v0.6+ 可评估**：TTS 集成（如 Edge TTS piping）。是否做取决于用户反馈——有多少人需要在 fastmd 中嵌入音频但自己没有音频文件。
- **不建议现在做 Level 3**：fastmd 的核心定位是"Markdown 发布管道"，而非"内容创作平台"。内建 TTS 会增加运维复杂度（GPU、模型管理），与轻量定位冲突。

#### A.4.3 goldmark-enclave 集成评估

**优点**：
- 纯 Go，MIT 协议，与 fastmd 技术栈一致
- 已支持 HTML5 audio、YouTube、Bilibili 等常见嵌入
- 安装方式简单：`go get` + 一行注册

**风险**：
- 星数较低（9 stars），社区活跃度一般
- 依赖外部服务的 iframe 嵌入（如 YouTube/Twitter），可能影响页面加载速度
- 需要评估 enclave 引入的传递依赖是否干净

**建议**：如果只想要 `![video.mp4]` → `<video>` 的能力，可以只取 enclave 的"object/audio"部分逻辑，或自己写一个更轻量的扩展（goldmark 扩展开发门槛低，自定义扩展约 50-100 行代码）。

---

## 补充需求 4：文档朗读功能（追加于 2026-07-20）

> 由 PM agent v1.8.0 撰写

### 问题拆解

1. **表面诉求**：用户在文档渲染页能"听"文档内容，支持暂停/继续。
2. **本质问题**：通勤、运动、眼睛疲劳场景下，用户需要非视觉的文档消费方式。
3. **触发场景**：用户在浏览器打开文档渲染页 → 不想读 → 想听。频率：中（移动端场景更常见）。
4. **背景原因**：
   - 原因 A：浏览器已内置 Web Speech API（Chrome/Edge 高质量中文），零服务端成本即可实现朗读。
   - 原因 B：调研附录 A.3 评估了 Edge TTS / Piper / F5-TTS 等服务端方案，均需新增 API + 存储 + TTS 调用，与 fastmd 轻量定位冲突。
5. **方案对比**：
   - **方案 A（选定）**：浏览器端 Web Speech API。优点：零服务端改动，即时可用，主流浏览器支持中文朗读。代价：不同浏览器语音质量差异（Chrome 最佳，Firefox 不支持），依赖浏览器标签页不关闭。适用边界：作为"增强体验"功能，不保证所有浏览器均可用。
   - **方案 B（未选）**：服务端 Edge TTS 预生成音频文件。优点：离线可播，跨浏览器一致。代价：需新增 TTS API + 音频存储 + 异步生成流程，运维复杂度显著增加，与 fastmd 轻量发布管道的定位冲突。
   - **推荐：方案 A**。Web Speech API 是当前阶段唯一零成本可交付的方案，且调研附录明确不建议服务端 TTS 集成（"fastmd 核心定位是 Markdown 发布管道，而非内容创作平台"）。
6. **为什么这么做**：方案 A 以零服务端成本让用户在渲染页听到文档内容，与 fastmd"轻量即时分享"定位完全一致。服务端方案 B 新增运维负担，边际收益不足以支撑。

### 需求

在文档渲染页面（`/:id`）添加"朗读"按钮，用户点击后浏览器使用 Web Speech API 朗读文档正文。支持暂停/继续/停止控制。

### 改动范围

- **`web/` 前端**：文档渲染页添加朗读控制 UI（按钮：朗读 → 暂停/继续/停止）+ 朗读 JS 逻辑
- **无需服务端改动**：Go handler、API、模板渲染均不变，Web Speech API 完全在浏览器执行

### 新增 BDD 场景

```gherkin
Scenario: S-16 文档朗读、暂停、继续与自动停止
  Given 用户在浏览器打开文档渲染页面 /:id
  And 文档包含至少一段正文内容（非纯代码块/空文档）
  When 用户点击"朗读"按钮
  Then 浏览器开始朗读文档正文内容（从标题/第一段开始）
  And 按钮文案变为"暂停"
  When 用户点击"暂停"
  Then 朗读暂停，保留当前位置
  And 按钮文案变为"继续"
  When 用户点击"继续"
  Then 从暂停位置恢复朗读
  When 朗读到达文档末尾
  Then 朗读自动停止
  And 按钮恢复为初始"朗读"状态
```

### 新增必做项

| ID | 内容 | 关联 BDD |
|----|------|----------|
| M-9 | 文档渲染页添加朗读按钮，支持朗读/暂停/继续/到达末尾自动停止 | S-16 |

### 发布门禁（补充需求 4）

| 必做 ID | 效果（Outcome） | 验证（Verification） |
|---------|-----------------|----------------------|
| M-9 | 用户在文档渲染页看到"朗读"按钮；点击后浏览器朗读文档正文（非代码块）；朗读中按钮可暂停/继续；朗读到达末尾后按钮自动恢复初始状态 | 人工：Chrome/Edge 桌面版打开任意含正文的文档渲染页，点击朗读按钮 → 确认浏览器发出语音朗读文档文字 → 点击暂停确认语音停止 → 点击继续确认恢复朗读 → 等待朗读完成确认按钮恢复初始状态 |

### 风险/注意事项

- **浏览器兼容性**：Web Speech API 的 `speechSynthesis` 在 Chrome/Edge 桌面版支持良好（中文朗读质量高），Safari 部分支持（中文音色有限），Firefox 不支持。本功能标记为"增强体验"，不保证所有浏览器均可用。可检测 `window.speechSynthesis` 存在性，不支持时隐藏按钮。
- **长文档**：超长文档（>5000 字）朗读耗时较长。浏览器标签页切换或后台运行可能导致朗读中断——当前不做断点续读恢复（SpeechSynthesis 暂停后切换标签页可能丢失状态，属浏览器限制，fastmd 不做 workaround）。
- **代码块跳过**：朗读时跳过代码块内容（代码朗读体验差），仅朗读段落文本和标题。
- **本功能不改变**：文档页面的 SEO、核心渲染逻辑、分享预览、`.md` 提示页。仅影响 `/:id` 渲染页的前端交互层。

---

## 总览更新（v0.4 全量）

### 必做项总表（M-1 ~ M-9）

| 必做 ID | 内容 | 来源 | 关联 BDD |
|---------|------|------|----------|
| M-1 | CLI push/pipe 输出链接以 `.md` 结尾 | v0.4 主需求 | S-1, S-2 |
| M-2 | API `POST /v1/push` 返回的 `url` 字段以 `.md` 结尾 | v0.4 主需求 | S-3 |
| M-3 | 浏览器访问 `/:id.md` 返回 HTML 提示页 | v0.4 主需求 | S-4, S-7, S-13 |
| M-4 | Agent `Accept: text/plain` 访问 `/:id.md` 仍返回 text/plain | v0.4 主需求 | S-5 |
| M-5 | `og:title` 动态使用文档 H1 标题 | v0.4 主需求 | S-8, S-9, S-13 |
| M-6 | `og:description` 动态使用文档内容开头摘要 | v0.4 主需求 | S-10, S-11, S-12, S-13 |
| M-7 | Dashboard 文档标题改为可点击链接 | 补充需求 3 | S-14 |
| M-8 | `GET /v1/docs` API 返回 `.md` 后缀 url | 补充需求 3 | S-15 |
| M-9 | 文档渲染页朗读按钮（朗读/暂停/继续/自动停止） | 补充需求 4 | S-16 |

### BDD 场景总表（S-1 ~ S-16）

| 场景 ID | 场景名称 | 维度 | 来源 |
|---------|---------|------|------|
| S-1 | CLI 管道发布输出 `.md` 后缀链接 | 正常 | v0.4 主需求 |
| S-2 | CLI push 文件输出 `.md` 后缀链接 | 正常 | v0.4 主需求 |
| S-3 | API push 返回 `.md` 后缀链接 | 正常 | v0.4 主需求 |
| S-4 | 浏览器访问 `.md` 页面看到提示和原始内容 | 正常 | v0.4 主需求 |
| S-5 | Agent Accept: text/plain 保持原有行为 | 正常/权限 | v0.4 主需求 |
| S-6 | 不存在的 `.md` 文档返回 404 | 异常 | v0.4 主需求 |
| S-7 | `.md` 提示页对旧文档兼容 | 边界/状态 | v0.4 主需求 |
| S-8 | HTML 渲染页 `og:title` 使用文档 H1 | 正常 | v0.4 主需求 |
| S-9 | 文档无 H1 时 `og:title` 退回默认格式 | 边界 | v0.4 主需求 |
| S-10 | `og:description` 使用文档内容开头摘要 | 正常 | v0.4 主需求 |
| S-11 | `og:description` 处理空内容/极短内容 | 边界 | v0.4 主需求 |
| S-12 | `og:description` HTML 实体转义 | 边界 | v0.4 主需求 |
| S-13 | `.md` 提示页也包含动态 OG 标签 | 正常 | v0.4 主需求 |
| S-14 | Dashboard 文档标题可点击跳转 | 正常 | 补充需求 3 |
| S-15 | Dashboard 文档列表链接使用 `.md` 后缀 | 正常 | 补充需求 3 |
| S-16 | 文档朗读、暂停、继续与自动停止 | 正常 | 补充需求 4 |

### 发布门禁总表（全量 9 条）

| 必做 ID | 效果（Outcome） | 验证（Verification） |
|---------|-----------------|----------------------|
| M-1 | CLI `push` / 管道输入后终端输出 `✓ Published → https://fastmd.dev/<id>.md` | 人工：执行 `echo "# test" \| fastmd`，观察输出 URL 以 `.md` 结尾 |
| M-2 | `POST /v1/push` 返回的 `url` 字段值为 `https://fastmd.dev/<id>.md` | 自动化：curl POST 带有效 content/token，检查 `response.url` 匹配正则 `\.md$` |
| M-3 | 浏览器访问 `/:id.md` 看到 HTML 页面，含提示文字和 `<pre>` 包裹的原始 Markdown | 自动化：curl（不设 Accept 头）访问 `/:id.md`，检查 `Content-Type` 为 `text/html`，body 包含提示文字和原始文档内容 |
| M-4 | `curl -H "Accept: text/plain"` 访问 `/:id.md` 仍返回 `text/plain` 原始 Markdown | 自动化：curl `-H "Accept: text/plain"` `/:id.md`，检查 `Content-Type: text/plain`、body 与原始内容完全一致 |
| M-5 | 文档 `og:title` 为该文档首个 H1 标题；无 H1 时退回默认格式 | 自动化：发布含 `"# Hello World"` 的文档，curl `/:id`，检查 `og:title` 内容；发布无 H1 的文档验证退回格式 |
| M-6 | 文档 `og:description` 为文档内容开头摘要（约 200 字符），社交平台预览可看到实际内容 | 自动化：发布含已知开头文本的文档，curl `/:id`，检查 `og:description`；验证 HTML 实体被正确转义 |
| M-7 | Dashboard 文档列表中，每个文档标题是可点击的链接，点击后跳转到该文档的 `.md` 页面 | 人工：登录 Dashboard 查看文档列表，点击任意文档标题，确认浏览器跳转到正确的 `.md` 页面 |
| M-8 | `GET /v1/docs` API 返回的 `url` 字段为 `.md` 后缀格式 | 自动化：curl 带有效 token 请求 `GET /v1/docs`，检查每个 `documents[].url` 匹配正则 `\.md$` |
| M-9 | 文档渲染页出现朗读按钮；点击后浏览器朗读文档内容；朗读中按钮可暂停/继续；朗读完成后按钮自动恢复初始状态 | 人工：Chrome/Edge 桌面版打开任意含正文的文档渲染页，点击朗读按钮确认浏览器朗读；点击暂停确认停止；点击继续确认恢复；等待朗读完成确认按钮恢复 |

### Anchor 自查（全量更新）

- [x] 本文档所有内容是否都映射到 Goal Anchor 的「必做」？
      M-1 → S-1/S-2 | M-2 → S-3 | M-3 → S-4/S-7/S-13 | M-4 → S-5 | M-5 → S-8/S-9/S-13 | M-6 → S-10/S-11/S-12/S-13 | M-7 → S-14 | M-8 → S-15 | M-9 → S-16
- [x] 本文档是否引入了 Goal Anchor 「不做」清单里的内容？
      否。N-1 到 N-5 均在正文中明确排除。M-9（朗读）为纯前端增强，不引入服务端 TTS 依赖、不修改渲染页核心布局、不新增 API——均不在"不做"范围内。
- [x] 本文档是否新增了未在 Goal Anchor 里的目标？
      M-9（朗读）是在 v0.4 主需求 Goal Anchor 基础上追加的增量需求，已在上方「补充需求 4」中完整拆解并通过用户确认。不改变 v0.4 原北极星（"分享链接在任何消费场景下都能正确处理"），朗读是对"人类看到可读页面"的增强——让人类不仅能"看"还能"听"。
- [x] 成功判据是否 =「发布门禁全部验证通过」？
      是。发布门禁 M-1 到 M-9 全部验证通过 = 本版成功。

### 逻辑自洽自检（全量更新）

- [x] 0.2 定位与价值能解释 0.3 本次需求为什么值得做？
      是。"CLI-first 发布管道"的核心是"发布 → 拿到链接 → 分享 → 消费"。朗读是对"消费"环节的增强——用户不仅能看文档、还能听文档，强化了"分享出去的内容能被有效消费"这一核心价值。
- [x] 0.3 本次目标能映射到 Goal Anchor 的"必做"？
      是。M-1~M-6 对应主需求 3 个目标，M-7/M-8 对应 Dashboard 一致性，M-9 对应朗读增量。M-9 是在原 PRD 定稿后追加的补充需求，已在上方独立拆解并获得用户确认。
- [x] Goal Anchor 的每条"必做"在「发布门禁」都有效果+验证？
      是。M-1 到 M-9 九条全部对应，发布门禁总表已更新。
- [x] 发布门禁无「只有工程指标、没有效果描述」的行？
      是。每条门禁均以产品语言描述用户可观察的效果（如"终端输出以 .md 结尾"、"浏览器朗读文档内容"、"按钮可暂停/继续"）。
- [x] 每个 BDD 场景都能映射回 0.3 本次目标？
      是。S-1~S-13 → v0.4 主需求目标；S-14/S-15 → 补充需求 3；S-16 → 补充需求 4。
- [x] MVP 三栏的"不做"清单与 0.3 的"不在本次范围"一致？
      是。N-1~N-5 不变。补充需求 4（M-9）为纯前端增量，不引入新 API/存储/依赖，不触及原"不做"边界。
- [x] 全文术语统一（同一概念不出现多个名字）？
      是。"朗读"="浏览器朗读文档内容"、各必做项 ID 全链路一致。

✅ 自洽自检通过（7/7）
