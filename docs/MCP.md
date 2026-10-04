# JasperLee MCP 服务（对外挂载）

> 把数字分身的能力以标准 **Model Context Protocol（MCP）** 暴露给外部宿主：
> Claude Desktop / Codex / Cherry Studio / 自研插件均可直接接入。
> 实现为零第三方依赖（纯 Go 标准库），与项目「小体量 + 高成熟度」的定位一致。

## 1. 两种传输

| 传输 | 启动方式 | 适用场景 |
|---|---|---|
| stdio | `jasperlee -mcp` | 宿主以子进程方式托管（桌面客户端标准方式） |
| HTTP | 正常启动服务，`POST /mcp` | 局域网 / 远程调用（JSON-RPC 2.0 over HTTP） |

两者共享同一套领域服务（`internal/core`）与工具实现，行为一致。

## 2. 快速开始

构建：

```bash
cd backend
go build -o jasperlee .        # Windows: go build -o jasperlee.exe .
```

stdio（宿主拉起）：

```bash
./jasperlee -mcp
```

HTTP（服务已在本机 8080 运行时）：

```bash
curl -s -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
```

## 3. 宿主配置示例（mcpServers）

```json
{
  "mcpServers": {
    "jasperlee": {
      "command": "C:\\Repository\\Develop\\aicoding\\digitalmemery\\backend\\jasperlee.exe",
      "args": ["-mcp"]
    }
  }
}
```

说明：

- 配置（模型 Key / RAG / 数据目录）来自可执行文件同目录或当前工作目录的 `.env`（开发期即 `backend/.env`）。
- 云模型（智谱 GLM）与 `embedding-3` 需要网络可达 `open.bigmodel.cn`。

## 4. 能力清单

工具（tools）：

| 名称 | 说明 | 入参 |
|---|---|---|
| `ask_jasper` | 知识库问答（RAG + 人设），返回完整回答 | `question`（必需）、`topK?` |
| `search_knowledge` | 语义检索知识库，返回命中片段与来源 | `query`（必需）、`topK?` |
| `list_materials` | 列出资料库文件（id/名称/类型/大小/入库状态） | — |
| `read_material` | 读取文件正文（文本 / docx 抽取） | `id` 或 `name` |
| `get_profile` | 个人主页数据（技能/项目/时间线/简历/联系方式） | — |
| `submit_inbox` | 向收件箱投递材料（外部入口） | `content`（必需）、`name?`、`note?`、`fileName?` |
| `run_task` | 以 Agent 方式执行任务（生成 plan/result） | `prompt`（必需）、`refs?` |

资源（resources）：资料库文件以 `jasperlee://materials/{id}` 暴露，支持 `resources/list` 与 `resources/read`。

## 5. 协议要点

- JSON-RPC 2.0；MCP 协议版本 `2025-06-18`；服务名 `jasperlee`。
- 生命周期：`initialize` → `notifications/initialized`（通知无响应，HTTP 返回 202）。
- 方法：`ping` / `tools/list` / `tools/call` / `resources/list` / `resources/read`。
- 工具执行错误按 MCP 约定返回 `result.isError=true`；协议错误返回 JSON-RPC `error`（-32700 / -32600 / -32601 / -32602 / -32603）。
- stdio：行分隔 JSON（一行一条消息）；日志走 stderr，stdout 仅协议消息。

## 6. 安全

- stdio 传输天然进程内隔离，推荐桌面宿主使用。
- HTTP `/mcp` 与主应用共用端口、**不带独立鉴权**：请仅在本机/内网暴露，或置于带鉴权的反向代理之后。
