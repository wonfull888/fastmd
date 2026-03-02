# fastmd v0.1 测试用例

> 开发完成后，按顺序执行所有用例。✅ = 通过 / ❌ = 失败 / 🔲 = 待测

---

## 前置准备

```bash
# 确保服务已启动（本地测试用）
./fastmd-server --port 8080 --db /tmp/test.db

# 设置 BASE_URL
export BASE=http://localhost:8080
```

---

## TC-01 CLI 安装与 Token 生成

| 步骤 | 命令 | 预期结果 | 结果 |
|---|---|---|---|
| 1 | `curl -fsSL https://fastmd.dev/install.sh \| sh` | 下载并安装 fastmd 二进制 | 🔲 |
| 2 | `fastmd --version` | 输出 `fastmd v0.1.0` | 🔲 |
| 3 | `fastmd push` (首次运行) | 自动生成 Token，打印 `Token saved to ~/.config/fastmd/token` | 🔲 |
| 4 | `cat ~/.config/fastmd/token` | 输出 `fmd_live_xxxx` 格式的 Token | 🔲 |

---

## TC-02 创建文档（Push）

### TC-02-1 管道推送

```bash
echo "# Hello fastmd\nThis is a test." | fastmd
```

| 预期 | 结果 |
|---|---|
| 输出包含 URL，格式为 `https://fastmd.dev/xxxx` | 🔲 |
| 短 ID 为 4 位 Base62 字符 | 🔲 |
| HTTP 响应码 200 | 🔲 |

### TC-02-2 文件推送

```bash
echo "# Report\nSome content." > /tmp/test.md
fastmd push /tmp/test.md
```

| 预期 | 结果 |
|---|---|
| 同 TC-02-1，返回有效 URL | 🔲 |

### TC-02-3 空内容

```bash
echo "" | fastmd
```

| 预期 | 结果 |
|---|---|
| 返回错误提示：`Error: content is empty` | 🔲 |

### TC-02-4 超大内容（>1MB）

```bash
python3 -c "print('x' * 1100000)" | fastmd
```

| 预期 | 结果 |
|---|---|
| 返回错误提示：`Error: content exceeds 1MB limit` | 🔲 |

---

## TC-03 查看文档（View）

> 假设 TC-02-1 返回的 ID 为 `abcd`

### TC-03-1 人类模式（浏览器）

```bash
curl -s $BASE/abcd | grep "<h1>"
```

| 预期 | 结果 |
|---|---|
| 返回 HTML，包含 `<h1>Hello fastmd</h1>` | 🔲 |
| Content-Type 为 `text/html` | 🔲 |

### TC-03-2 机器模式（.md 后缀）

```bash
curl -s $BASE/abcd.md
```

| 预期 | 结果 |
|---|---|
| 返回原始 Markdown 文本 | 🔲 |
| Content-Type 为 `text/plain` | 🔲 |
| 内容与推送时完全一致 | 🔲 |

### TC-03-3 机器模式（Accept Header）

```bash
curl -s -H "Accept: text/plain" $BASE/abcd
```

| 预期 | 结果 |
|---|---|
| 返回原始 Markdown 文本 | 🔲 |

### TC-03-4 不存在的 ID

```bash
curl -s -o /dev/null -w "%{http_code}" $BASE/xxxx
```

| 预期 | 结果 |
|---|---|
| 返回 404 状态码 | 🔲 |
| 页面显示友好的 404 提示 | 🔲 |

---

## TC-04 CLI 拉取文档（Get）

### TC-04-1 有 H1 标题，自动命名

```bash
cd /tmp && fastmd get abcd
```

| 预期 | 结果 |
|---|---|
| 本地生成 `hello-fastmd.md` 文件（H1 Slugified） | 🔲 |
| 文件内容与原始 Markdown 一致 | 🔲 |

### TC-04-2 无 H1 标题，用 ID 命名

```bash
echo "No heading here." | fastmd  # 记录返回的 ID，假设为 efgh
fastmd get efgh
```

| 预期 | 结果 |
|---|---|
| 本地生成 `efgh.md` | 🔲 |

---

## TC-05 删除文档（Delete）

### TC-05-1 正常删除（Token 匹配）

```bash
fastmd delete abcd
```

| 预期 | 结果 |
|---|---|
| 输出 `Deleted: abcd` | 🔲 |
| 再次访问 `$BASE/abcd` 返回 404 | 🔲 |

### TC-05-2 Token 不匹配

```bash
# 用错误 token 直接调 API
curl -s -X DELETE -H "Authorization: Bearer fmd_live_wrongtoken" $BASE/v1/efgh
```

| 预期 | 结果 |
|---|---|
| 返回 403 状态码 | 🔲 |
| 文档依然可访问 | 🔲 |

### TC-05-3 删除不存在的文档

```bash
fastmd delete nonexist
```

| 预期 | 结果 |
|---|---|
| 返回错误提示：`Error: document not found` | 🔲 |

---

## TC-06 版本接口

```bash
curl -s $BASE/v1/version
```

| 预期 | 结果 |
|---|---|
| 返回 JSON，含 `version` 字段，值为 `v0.1.0` | 🔲 |
| 含 `install_url` 字段 | 🔲 |

---

## TC-07 网站页面

| 页面 | URL | 检查项 | 结果 |
|---|---|---|---|
| 首页 | `GET /` | 页面包含安装命令 `curl -fsSL ...` | 🔲 |
| 首页 | `GET /` | 安装命令有复制按钮，点击后复制成功 | 🔲 |
| 文档页 | `GET /docs` | 包含所有 API 端点说明 | 🔲 |
| 帮助页 | `GET /help` | 包含 FAQ 条目 | 🔲 |
| 渲染页 | `GET /abcd` | 代码块有语法高亮 | 🔲 |
| 渲染页 | `GET /abcd` | 页面在移动端宽度（375px）不溢出 | 🔲 |
| 404 页 | `GET /notexist` | 友好提示而非空白页 | 🔲 |

---

## TC-08 升级命令

```bash
fastmd upgrade
```

| 预期 | 结果 |
|---|---|
| 重新执行安装脚本，二进制更新完成 | 🔲 |
| 无报错退出 | 🔲 |

---

## TC-09 端到端场景：AI Agent 完整流程

模拟 agent 创建报告 → 人类审阅 → 清理

```bash
# Step 1: Agent 推送报告
ID=$(echo "# Agent Report\n\n## Summary\nTask completed successfully." | fastmd | grep -o '[a-zA-Z0-9]\{4\}$')

# Step 2: 机器消费 raw 数据
curl -s $BASE/${ID}.md | grep "# Agent Report"

# Step 3: 人类浏览器审阅（手动验证）
open https://fastmd.dev/$ID

# Step 4: Agent 完成后清理
fastmd delete $ID
curl -s -o /dev/null -w "%{http_code}" $BASE/$ID  # 预期: 404
```

| 步骤 | 预期 | 结果 |
|---|---|---|
| Step 1 | 返回有效 ID | 🔲 |
| Step 2 | grep 匹配成功 | 🔲 |
| Step 4 | 删除成功，访问返回 404 | 🔲 |

---

## 测试总结

| 模块 | 用例数 | 通过 | 失败 | 待测 |
|---|---|---|---|---|
| CLI 安装 | 4 | - | - | 4 |
| Push | 4 | - | - | 4 |
| View | 4 | - | - | 4 |
| Get | 2 | - | - | 2 |
| Delete | 3 | - | - | 3 |
| Version | 1 | - | - | 1 |
| 网站页面 | 7 | - | - | 7 |
| Upgrade | 2 | - | - | 2 |
| E2E 场景 | 3 | - | - | 3 |
| **合计** | **30** | **-** | **-** | **30** |
