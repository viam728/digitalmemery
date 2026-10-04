# JasperLee 代码 Wiki

> **一句话**：给招聘者的个人数字分身 —— 免登录进入，一键申请临时 Key，即可看个人主页、与分身问答（基于知识库 RAG）、浏览资料库、投递材料（投递后数字分身自动阅读并回复）。
>
> 仓库：`github.com/viam728/digitalmemery` ｜ 本 Wiki 基于代码走查生成并随代码维护（最近更新：2026-10-04）。

---

## 目录

1. [项目速览](#1-项目速览)
2. [一分钟上手](#2-一分钟上手)
3. [架构总览](#3-架构总览)
4. [目录结构](#4-目录结构)
5. [后端模块详解](#5-后端模块详解)
6. [前端模块详解](#6-前端模块详解)
7. [API 参考](#7-api-参考)
8. [数据与存储](#8-数据与存储)
9. [关键流程剖析](#9-关键流程剖析)
10. [配置参考](#10-配置参考)
11. [部署与运维](#11-部署与运维)
12. [扩展指南](#12-扩展指南)
13. [已知限制与路线图](#13-已知限制与路线图)
14. [变更记录](#14-变更记录)

---

## 1. 项目速览

| 维度 | 内容 |
|---|---|
| 定位 | 招聘者视角的「个人数字分身」：主页 + 问答 + 资料库 + 投递收件箱 |
| 前端 | React 18 + TypeScript + Vite + Tailwind + Zustand（深色三栏布局） |
| 后端 | Go 1.26 标准库 `net/http`（无第三方框架），单二进制 |
| 存储 | PostgreSQL（自研 pgpool 客户端）+ 降级内存/JSON；文件落磁盘 |
| RAG | 可插拔：`glm`（embedding-3 语义向量）/ `mock`（关键词，开发用）/ `local` / `aliyun` |
| 模型 | OpenAI 兼容流式（默认智谱 GLM）；`mock` 模式零成本离线开发 |
| 部署 | 本机 / Docker Compose / 开源隧道（cloudflared / localtunnel） |

**核心能力**

1. **Key 门控（免登录）**：访客申请临时 Key（默认 100 万 token 额度 + 资料库权限），所有 `/api` 经 `Authorization: Bearer <key>` 鉴权。
2. **对话**：SSE 流式输出；人设「灵魂」提示词（身份 + 行为准则 + SKILL）+ RAG 命中注入 + 面试工作流步骤可见。
3. **资料库**：上传 / 预览 / 下载 / 改名 / 删除、向量化入库、语义检索。
4. **Agent 工作区**：建任务 → 引用资料库文件 → 运行 → 产出 `plan.md` / `result.md` → 产物发布到「Jasper 的空间」。
5. **收件箱自动应答（v1 自动化）**：投递 JD/资料后，数字分身后台自动阅读并生成回复。
6. **管理员后台**：密码进入；看板（运行时长/用量/计数）+ Key 管理 + 收件箱。

---

## 2. 一分钟上手

```bash
# 后端（:8080）——默认 mock 模型与内存存储，零配置可跑
cd backend && go run .

# 前端（:5173，/api 代理到 :8080）
cd frontend && npm install && npm run dev
# 浏览器打开 http://localhost:5173
```

真实模型与数据库（复制 `backend/.env.example` 为 `backend/.env` 填值）：

```ini
MODEL_PROVIDER=glm
MODEL_API_KEY=<智谱key>
MODEL_BASE_URL=https://open.bigmodel.cn/api/paas/v4
MODEL_NAME=glm-4-flash
RAG_PROVIDER=glm          # 或 mock
PGHOST=localhost          # docker compose up -d db 后默认可用
```

> 提示：模型 key 缺失或 `MODEL_PROVIDER=mock` 时自动走 mock 流式（开发/演示不受影响）；Postgres 连不上时自动降级内存存储（`data/store.json`）。

---

## 3. 架构总览

```
┌───────────────────────── 前端（React/Vite，frontend/） ─────────────────────────┐
│  KeyGate 门控 → App（三栏布局）                                                   │
│  ├─ 左：IconRail（导航）+ NavSidebar（会话列表，仅对话栏目渲染）                    │
│  ├─ 中：ChatView ｜ LibraryView ｜ WorkspaceView ｜ AvatarView ｜ InboxView ｜ Admin │
│  └─ 右：RightPanel（工作区文件树 / 文件预览 / RAG 命中）                            │
│  api.ts（fetch + SSE 解析） · store.ts（Zustand 全局态）                          │
└───────────────────────────────────┬──────────────────────────────────────────────┘
                                    │ HTTP / SSE（Authorization: Bearer <key>）
┌───────────────────────────────────▼──────────────────────────────────────────────┐
│ 后端（Go，backend/）                                                              │
│  main.go（路由 + 静态托管 + CORS）                                                │
│  internal/api      HTTP 层：keys / conversations / files / inbox / rag /          │
│                    workspaces / artifacts / models / avatar / admin              │
│  internal/agent    灵魂提示词(soul) / 面试工作流(interview) / 工作区(workspace)    │
│  internal/llm      模型客户端：mock ｜ OpenAI 兼容流式（GLM/DeepSeek）            │
│  internal/rag      分块 → Embedding → 向量检索（glm/mock/local/aliyun 可插拔）    │
│  internal/storage  存储门面：pgStore（PG）⇄ memStore（内存+JSON 降级）           │
│  internal/pgpool   纯标准库 PostgreSQL v3 协议客户端（md5/cleartext 认证）        │
│  internal/config   .env/环境变量配置   internal/models  共享数据模型              │
└───────────────────────────────────┬──────────────────────────────────────────────┘
                                    │
              ┌─────────────────────┴──────────────────────┐
              ▼                                            ▼
   PostgreSQL（可选，compose：5432）              磁盘 data/（uploads、inbox、
   表：conversations/messages/files/…             rag_index.json、store.json）
```

**分层（2026-10-04 起）**：`internal/core` 装配领域服务（存储 / RAG / 模型 / Agent / 人设），与传输无关；`internal/api`（HTTP）与 `internal/mcp`（MCP 协议，stdio + HTTP）是两个**薄传输层**，共享同一套 core——新增对外形态只需再写一个传输层。

**一个请求的一生**（以「提问」为例）：

1. 前端 `streamChat()` → `POST /api/conversations/{id}/messages/stream`；
2. `requireKey` 校验 Bearer Key（+额度硬校验）；读会话历史；
3. RAG 检索 Top-4 命中 + `@` 引用文件正文 → 组装「灵魂 + 资料」system prompt；
4. `llm.Stream` 流式生成 → SSE 事件：`steps` → 多个 `delta` → `usage` → `done`；
5. 助手消息落库（含模型/耗时/token 明细），按用量扣减 Key 额度，返回剩余。

---

## 4. 目录结构

```
digitalmemery/
├─ README.md                  # 项目说明（功能/启动/部署/里程碑）
├─ docs/                      # 文档
│  ├─ 需求书.md               # 产品需求（UI 对照、模块、RAG 调研）
│  ├─ 脚本排障记录.md         # start.bat/start.sh 排障实录（编码/路径问题）
│  └─ 代码Wiki.md             # 本文件
├─ data/                      # 运行时数据（store.json、uploads/、inbox/、rag_index.json）
├─ deploy.sh / deploy.bat     # 一键端到端部署（构建→启动→隧道→打印公网地址）
├─ start.sh / start.bat       # 本机一键启动（构建→启动→打印局域网地址）
├─ Dockerfile                 # 多阶段构建（前端 build + Go build → 单镜像）
├─ docker-compose.yml         # db（postgres:15, md5）+ app（单镜像服务）
├─ frontend/                  # React + Vite + TS + Tailwind + Zustand
│  ├─ public/jasperlee.html   # 内置简介页（左栏底部入口）
│  └─ src/
│     ├─ api.ts               # API 客户端（含 SSE 流解析）
│     ├─ store.ts             # Zustand 全局状态（唯一的业务数据源）
│     ├─ types.ts             # 共享类型
│     ├─ App.tsx              # 布局 + 视图路由 + KeyGate 门控
│     └─ components/          # 组件（见第 6 节）
└─ backend/                   # Go 标准库后端
   ├─ main.go                 # 入口：路由注册、CORS、静态托管
   ├─ .env.example            # 配置示例（真实 .env 不入库）
   └─ internal/
      ├─ api/                 # HTTP 处理器（按域拆分文件）
      ├─ agent/               # 数字分身灵魂/面试工作流/工作区服务
      ├─ config/              # 配置加载（.env + 环境变量）
      ├─ llm/                 # 模型客户端（流式）
      ├─ models/              # 数据结构定义
      ├─ core/                # 领域服务装配（存储/RAG/模型/Agent/人设）——传输无关
      ├─ mcp/                 # MCP 服务（JSON-RPC 2.0；stdio + HTTP，工具/资源）
      ├─ pgpool/              # 纯标准库 PostgreSQL 客户端
      ├─ rag/                 # RAG：分块/Embedding/向量库
      ├─ textract/            # Office（docx）纯文本抽取
      └─ storage/             # 存储门面（pgStore / memStore）
```

---

## 5. 后端模块详解

### 5.1 `main.go` — 入口与路由

- `config.Load()` → `api.New(cfg)`（构造全部服务）→ 注册路由 → `http.ListenAndServe`。
- 中间件：`cors`（允许全源，开发便利）；`serveStatic` 托管 `frontend/dist`（相对**可执行文件**解析，任意 cwd 启动都不 404），非文件请求回退 `index.html`（SPA 路由）。
- 路由清单见 [第 7 节](#7-api-参考)。

### 5.2 `internal/config` — 配置

- `loadDotEnv()`：依次尝试 CWD 与 exe 所在目录的 `.env`，**不覆盖**已存在的环境变量。
- `Config` 字段：`Port`、`DataDir`、`RAGProvider`、`AliyunAPIKey/Endpoint`、`ModelProvider/APIKey/BaseURL/Name/List/HostIP`、`AdminPassword`、`PG*`；`DATABASE_URL` 可一键解析出 PG 连接参数。
- 默认值全部指向「可跑」：`PORT=8080`、`DATA_DIR=../data`、`MODEL_PROVIDER=mock`、`RAG_PROVIDER=mock`、`ADMIN_PASSWORD=feng`。

### 5.3 `internal/models` — 数据模型

`Conversation` / `Message`（含 `ResponseDetails`）/ `FileMeta`（`IsArtifact`、`WorkspaceID`）/ `RagHit` / `Workspace`（+`WorkspaceFile` 树、`Usage`）/ `RefContent` / `Avatar`（主页数据）/ `InboxItem`（含自动应答 `Status/Reply/RepliedAt`）/ `ApiKey`（`Quota/Used/Library/Active`）。

### 5.4 `internal/api` — HTTP 层

| 文件 | 职责 |
|---|---|
| `api.go` | 聚合 `Handler`、中间件工具（`writeJSON/writeErr/bearerKey/requireKey`）、核心处理器：Key 申请/校验、会话 CRUD、`StreamChat`（SSE 对话）、资料库上传/内容/入库、RAG 检索、工作区创建/运行、`maybeAutoTitle`（会话自动命名） |
| `admin.go` | 管理员登录（内存 token，24h）与看板/Key 管理 |
| `files_mgmt.go` | 文件改名/删除（「Jasper 的空间」只读）/下载 |
| `inbox.go` | 收件箱上传/列表/下载/删除 + **`autoReplyInbox` 自动应答** |
| `artifacts.go` | Agent 产物发布（工作区 `output/` → 资料库同一行记录 + `IsArtifact`）+ `ListArtifacts` |
| `avatar.go` | 数字分身主页数据（李俊锋简历/技能/项目/时间线） |
| `social.go` | 平台看板 CRUD（公开读 / 管理员写） |
| `models.go` | 可用模型列表（缓存 > `MODEL_LIST` > `MODEL_NAME`）与切换 |
| `readonly.go` | 只读判定小工具（避免循环依赖的接口包装） |

关键处理器行为细节：

- **`ApplyKey`**：生成随机 Key（16B hex），默认 `quota=1_000_000`、`library=true`。
- **`StreamChat`**：额度硬校验 → 落用户消息 → 先发 `steps` 事件（面试工作流：识别考察点/定位项目）→ 流式 `delta` → `usage/remaining` → `done`；助手消息含 `model/elapsedMs/tokens/agentType`。
- **`UploadInbox`**：保存 multipart 到 `data/inbox`，落库后 **`go h.autoReplyInbox(...)` 异步生成回复**（不阻塞上传响应）。
- **`RunWorkspace`**：收集引用文件 + RAG 命中 → `agent.RunWithSoul` → `publishArtifacts` → 扣额度。

### 5.5 `internal/agent` — 数字分身灵魂与工作流

- **`soul.go`**：`SoulIdentity`（我是谁：李俊锋数字分身）+ `SoulPolicy`（行为准则）+ `JasperSkills`（5 个 Skill：面试应答/项目深挖/岗位匹配/技术方案/自我介绍）→ `SoulSystemPrompt(knowledge, refs, task)` 组装注入。
- **`interview.go`**：`PlanInterview`（纯规则，可解释：问题 → 考察点 / 命中 SKILL / 选用真实项目）+ `RunInterview`（组装 → 流式 → 评估产物）+ `InterviewReport`。
- **`workspace.go`**：`WorkspaceSvc`——建工作区、挂引用、`RunWithSoul`（引用+RAG → 灵魂提示 → 模型 → `plan.md`/`result.md` 挂树）、`buildTree`（右侧文件树）。

### 5.6 `internal/llm` — 模型客户端

- `Client`：`provider=mock` 或 key 为空 → `mockStream`（逐字模拟，零成本）；否则 `deepseekStream`（OpenAI 兼容 `/chat/completions` 流式，逐行解析 `data:`，提取 `usage`）。
- `MODEL_HOST_IP` 直连机制：DNS 被污染时把 baseURL 的 host 换成 IP + 显式 `Host: open.bigmodel.cn`（仅此类场景跳过证书主机名校验）。
- `models.go`：`ListModels`/`RefreshModels`（启动后台拉取一次，缓存供 `/api/models`）。

### 5.7 `internal/rag` — 检索增强

- **分块**：`Chunk(text, 500, 50)` 定长 + 重叠。
- **抽象**：`EmbeddingProvider`（`glm`/`aliyun`/`local`/`mock`）、`VectorStore`（`MemVectorStore` / `AliyunVectorStore`）。
- **GLM Embedding**：`embedding-3`，15s 超时 + 1 次重试；失败优雅降级（不注入命中）。
- **MemVectorStore**：优先余弦相似度；无向量时关键词重叠兜底（**存在正命中时过滤零命中片段**，降噪）。索引落盘 `data/rag_index.json`，重启自动加载。
- **启动引导**：`bootstrapIndex` 后台异步把未入库的文本类文档自动向量化（跳过 pdf/docx/image），保证访客首次提问即可命中知识库。

### 5.8 `internal/storage` — 存储门面

- **`facade.go`**：`Store` 统一接口 → 优先 `pgStore`，失败降级 `memStore`（方法签名一致，api 层无感）。
- **`store.go`（memStore）**：内存 + `data/store.json` 持久化；种子文件（关于我 6 篇 + 简历 PDF）保底存在；合并持久化数据与种子。
- **`dbstore.go`（pgStore）**：建表幂等（含存量库 `ALTER ADD COLUMN IF NOT EXISTS`）；文件以整对象 jsonb 存；`ConsumeTokens` 用单条原子 UPDATE 防竞态；供 RAG 落 `rag_chunks`。
- **Key 语义**：`quota=0` 表示不限；`ConsumeTokens` 超出额度返回 false（对话前还有硬校验拦截）。

### 5.9 `internal/pgpool` — 纯标准库 PG 客户端

- 手写 PostgreSQL v3 线协议：启动握手、`AuthenticationOk/Cleartext/MD5`、简单查询（`Q`）与文本结果扫描。
- 单连接 + 互斥（低并发场景足够）；**不支持** SSL/TLS、扩展查询协议（本项目用不到）；不支持 SCRAM 认证（compose 已显式配 `POSTGRES_HOST_AUTH_METHOD: md5` 配合）。
- 所有值经单引号转义（值均由应用自身控制）。

### 5.10 `internal/textract` — Office 文档抽取

- `DocxText([]byte)`：解压 docx（zip）→ 解析 `word/document.xml` → 按段落拼接纯文本，零第三方依赖。
- 接入点：资料库 `ReadContent`（入库/@引用/收件箱自动应答读取）、`/api/files/{id}/content`（docx 直接给文本预览）、启动引导 `bootstrapIndex`。
- PDF 暂不支持（格式复杂，保留原文下载），由 roadmap 跟踪。

### 5.11 `internal/core` — 领域服务装配（传输无关）

- `Core{ Cfg, Store, RAG, LLM, Agent }`：在 `New(cfg)` 中统一装配，并触发启动期副作用（模型默认值 / 知识库引导 `bootstrapIndex` / 模型列表刷新）。
- `CurrentModel()` 与 `AvatarData()`（人设数据）也在此层，供所有传输层复用。
- 价值：HTTP 与 MCP 共享同一套实例与行为；`internal/api` 的 `New` 退化为「core + 传输胶水」。

### 5.12 `internal/mcp` — MCP 服务（对外挂载）

- 零第三方依赖的 Model Context Protocol 服务：JSON-RPC 2.0 信封 + MCP 生命周期（`initialize` → `notifications/initialized`）。
- 双传输：`ServeStdio`（行分隔 JSON，供宿主托管）与 `ServeHTTP`（`POST /mcp` 返回 JSON）。
- 能力：`tools/list` / `tools/call`（7 个工具）+ `resources/list` / `resources/read`（资料库文件，`jasperlee://materials/{id}`）；协议版本 `2025-06-18`。
- 详见 [MCP.md](MCP.md)。

---

## 6. 前端模块详解

| 模块 | 职责 |
|---|---|
| `App.tsx` | 布局壳：TopBar + IconRail + NavSidebar（仅「对话」栏目渲染）+ 主视图 + RightPanel；无 Key 时显示 KeyGate |
| `api.ts` | 全部 HTTP 调用 + `streamChat`（SSE 解析：delta/steps/usage/remaining/done/error）；`jl_api_key`、`jl_admin_token` 存 localStorage |
| `store.ts` | Zustand 全局态：key/会话/消息/模型/资料/产物/工作区/收件箱/主页/管理员；所有动作只消费真实后端数据（无 mock 假数据） |
| `components/KeyGate.tsx` | 门控页：申请新 Key / 粘贴已有 Key |
| `components/common/TopBar.tsx` | 顶栏：模型选择器 / 视图标题 / 操作按钮 |
| `components/Sidebar/` | `IconRail`（窄图标栏：新建/视图切换；Agent 入口已合并进「对话」，Ask/Agent 模式在对话内切换；底部「设置」上方为博客系统入口——跳转 MyShow，地址优先取平台看板「个人博客」链接，其次 `VITE_BLOG_URL`，兜底本机开发默认）+ `NavSidebar`（置顶「新对话」+ 会话列表：重命名/删除/置顶；仅「对话」栏目渲染） |
| `components/Chat/ChatView.tsx` | 对话主界面：空态建议、`/` 快捷指令（/help /new /intro /skills /projects /contact /resume /search /library /inbox /home）、`@` 引用资料库文件、Ask/Agent 模式切换（对话内统一入口） |
| `components/Chat/MessageBubble.tsx` | 消息气泡（含 Response details：模型/状态/耗时/token） |
| `components/Library/LibraryView.tsx` + `fileDisplay.ts` | 资料库：拖拽/点击上传、表格、入库按钮、预览、语义检索 |
| `components/Workspace/WorkspaceView.tsx` | Agent 工作区：新建任务（选引用）→ 运行 → 状态/用量/文件树（引用标黄、产出可预览） |
| `components/Avatar/AvatarView.tsx` | 数字分身主页（技能/项目/时间线/简历下载） |
| `components/Board/BoardView.tsx` | 平台看板（分类卡片 + 双击就地编辑；管理员可增删） |
| `components/Inbox/InboxView.tsx` | 投递收件箱：上传 + **自动应答展示**（等待/回复/失败三态） |
| `components/Admin/AdminView.tsx` | 管理员后台：看板 + Key 管理 + 收件箱 |
| `components/right/RightPanel.tsx` | 右栏三模式：files（工作区树）/ preview（文件预览，含图片/PDF）/ rag（检索命中） |

---

## 7. API 参考

鉴权：除 `health`、`keys/apply`、`avatar`、`models`（列表）与管理员登录外，全部需要 `Authorization: Bearer <访客Key>`；资料库相关（files/inbox/artifacts/rag）还需要 Key 的 `library=true`；管理员接口用登录返回的 admin token。

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/health` | 健康检查（rag/llm provider） |
| POST | `/api/keys/apply` | 申请临时 Key `{label?, library?}` |
| GET | `/api/keys/me` | 校验 Key，返回剩余额度 |
| GET | `/api/conversations` | 会话列表（按访客 Key 隔离） |
| POST | `/api/conversations` | 新建会话 `{title}`（归属当前 Key） |
| PATCH | `/api/conversations/{id}` | 改名 / 置顶 `{title?, pinned?}` |
| DELETE | `/api/conversations/{id}` | 删除会话及消息 |
| GET | `/api/conversations/{id}/messages` | 消息列表 |
| POST | `/api/conversations/{id}/messages/stream` | **SSE 对话** `{content, refs?, mode?}`；事件：steps/delta/usage/remaining/done/error |
| GET | `/api/files` | 资料库列表（按访客可见性过滤） |
| POST | `/api/files/upload` | 上传文件（multipart `file`） |
| GET | `/api/files/{id}/content` | 预览内容（文本 UTF-8 / docx 抽取正文 / 其他二进制原始字节） |
| GET | `/api/files/{id}/download` | 附件下载 |
| PATCH | `/api/files/{id}` | 改名（仅「我分享的」） |
| DELETE | `/api/files/{id}` | 删除（仅「我分享的」） |
| POST | `/api/files/{id}/ingest` | 向量化入库 |
| POST | `/api/inbox/upload` | 投递材料（multipart：`file`+`name`+`note`）→ **触发自动应答** |
| GET | `/api/inbox` | 收件箱列表（按访客隔离；含 `status/reply/repliedAt`） |
| GET | `/api/inbox/{id}/download` | 下载投递文件 |
| DELETE | `/api/inbox/{id}` | 删除投递 |
| POST | `/api/rag/query` | 语义检索 `{query}` → 命中片段 |
| GET | `/api/models` | 可用模型列表 `{default, models}` |
| POST | `/api/models/switch` | 切换默认模型 `{model}` |
| GET | `/api/artifacts` | Jasper 的空间（Agent 产物） |
| GET | `/api/workspaces/{id}` | 工作区（不存在时返回示例树） |
| POST | `/api/workspaces` | 新建工作区 `{title?, model?, refs?}` |
| POST | `/api/workspaces/{id}/run` | 运行任务 `{prompt, expose?}` → `{workspace, artifacts}` |
| GET | `/api/avatar` | 主页数据（无需鉴权） |
| GET | `/api/social` | 平台看板（公开浏览） |
| POST | `/api/social` | 新增平台条目（管理员） |
| PATCH | `/api/social/{id}` | 编辑平台条目（管理员） |
| DELETE | `/api/social/{id}` | 删除平台条目（管理员） |
| POST | `/mcp` | **MCP 服务**（JSON-RPC 2.0：initialize / tools / resources，无独立鉴权） |
| POST | `/api/admin/login` | 管理员登录 `{password}` → `{token}` |
| GET | `/api/admin/stats` | 看板（运行时长/计数/额度） |
| GET | `/api/admin/keys` | Key 列表 |
| PATCH | `/api/admin/keys/{key}` | 启用/停用/调额度 `{active?, quota?}` |
| POST | `/api/admin/keys/{key}/renew` | 重置用量 |
| DELETE | `/api/admin/keys/{key}` | 删除 Key |

---

## 8. 数据与存储

### 降级策略（重要）

```
启动 → newPostgres(cfg) 成功？ ─是→ pgStore（真实库）
                              └否→ memStore（内存 + data/store.json 持久化）
```

两种实现对 `api` 层完全同签名（`storage.Store` 门面），日志会打印用了哪种。

### PostgreSQL 表结构（`dbstore.go` 幂等建表）

| 表 | 关键列 |
|---|---|
| `conversations` | id / title / kind / pinned / created_at / updated_at |
| `messages` | id / conversation_id / role / content / created_at / model / details(jsonb) / refs(jsonb) |
| `files` | id / data(jsonb 整对象) / is_artifact |
| `workspaces` | id / data(jsonb) |
| `api_keys` | key / label / quota / used / library / active / created_at / last_used_at |
| `inbox` | id / data(jsonb) |
| `rag_chunks` | seq / file_id / file_name / chunk / vec(jsonb) |

### 磁盘目录（`DATA_DIR`，默认 `data/`）

- `uploads/{id}{ext}`：资料库文件正文（种子文件无磁盘文件时回退内置示例内容）。
- `inbox/{id}{ext}`：访客投递文件。
- `rag_index.json`：memStore 向量索引落盘（glm/mock/local 模式）。
- `store.json`：memStore 全量持久化（会话/文件/Key/收件箱）。

### 归属与隔离模型（2026-10-04 起）

- 会话 / 工作区：按 `OwnerKey`（访客 Key）隔离；越权一律 404（不暴露存在性）。
- 文件：「我的资料」「Jasper 的空间」为公开共享（所有访客可读、不可改）；「我分享的」上传件仅上传者可见/可操作；历史无归属文件兼容可见。
- 收件箱：仅投递者可见本人条目。
- 产物：「Jasper 的空间」的 Agent 产物按工作区归属可见（无工作区关联的历史产物兼容可见）。

---

## 9. 关键流程剖析

### 9.1 Key 申请与鉴权

```
访客 → POST /api/keys/apply → NewKey(1_000_000, library=true) → 前端存 localStorage
后续请求 → Authorization: Bearer <key> → requireKey（Active 校验）
对话完成 → ConsumeTokens(用量)（对话前有额度硬校验：用尽返回 402 明确提示）
管理员 → /api/admin/login（密码）→ 内存 token（24h）
```

### 9.2 对话流式应答（SSE）

```
POST /api/conversations/{id}/messages/stream
  ├─ 额度硬校验（402 即停，不落消息）
  ├─ 会话归属校验（非归属者 404，不暴露存在性）
  ├─ maybeAutoTitle（默认标题 → 首条消息命名，≤18 字）
  ├─ 历史消息 + 本轮用户消息落库
  ├─ RAG.Query(content, top4) + @引用文件正文（截断 6000 字）
  ├─ PlanInterview → SoulSystemPrompt(资料 + 引用 + 任务)
  └─ SSE: {"steps":[...], "model":...} → {"delta":"..."}×N → {"usage":n,"remaining":m} → {"done":true}
        → 助手消息落库（details：模型/耗时/token）→ ConsumeTokens
```

### 9.3 资料库上传与向量化

```
POST /api/files/upload（multipart）→ data/uploads/{id}{ext} → AddFile（Path=我分享的）
POST /api/files/{id}/ingest → ReadContent → Chunk(500/50) → Embed → VectorStore.Index → MarkIngested
（服务启动时 bootstrapIndex 会把未入库的文本类种子/存量文件自动入库）
```

### 9.4 Agent 工作区运行与产物发布

```
POST /api/workspaces {title, refs[]} → 引用文件挂树（Referenced 标记）
POST /api/workspaces/{id}/run {prompt}
  ├─ collectRefs（树中引用的资料正文）+ RAG 命中
  ├─ RunWithSoul → plan.md / result.md → 状态 done + usage
  └─ publishArtifacts → output/ 节点落资料库同一行记录（IsArtifact=true, Path=Jasper 的空间）
      → 自动暴露到「Jasper 的空间」；扣减 Key 额度
```

### 9.5 收件箱自动应答（v1）

```
POST /api/inbox/upload → 保存 + AddInbox(status=pending) → 立即响应（不阻塞）
  └─（异步）autoReplyInbox：
       status=replying → 读文本类文件正文（docx 抽取、其余二进制仅留言）→ RAG 命中
       → SoulSystemPrompt → 生成回复 → status=replied + Reply/RepliedAt 落库
       失败 → status=failed；成功后按用量扣减上传者 Key 额度
前端 InboxView：上传后轮询刷新，展示「正在阅读并自动回复… / 自动回复 / 失败」三态
```

### 9.6 面试应答工作流（规则可解释）

`PlanInterview(question)`：问题 → 考察点（自我介绍/项目/技能/JD 匹配/Agent 能力/后端能力/联系方式）+ 命中 SKILL + 选用真实项目（InkBloom / BeYoung / LLM 问答系统 / D-S 工具），随 `steps` 事件先行下发到前端展示。

---

## 10. 配置参考

| 变量 | 默认 | 说明 |
|---|---|---|
| `PORT` | `8080` | 后端端口 |
| `DATA_DIR` | `../data` | 数据目录 |
| `MODEL_PROVIDER` | `mock` | `mock` / `glm` / 任意 OpenAI 兼容 |
| `MODEL_API_KEY` | — | 模型密钥（mock/缺省时自动降级） |
| `MODEL_BASE_URL` | `https://api.deepseek.com` | OpenAI 兼容端点 |
| `MODEL_NAME` | `deepseek-chat` | 默认模型（启动自动配 GLM 全集） |
| `MODEL_LIST` | 空 | 显式模型清单（逗号分隔），留空自动 |
| `MODEL_HOST_IP` | 空 | DNS 污染时直连 IP（配 `Host` 头） |
| `RAG_PROVIDER` | `mock` | `glm` / `mock` / `local` / `aliyun` |
| `ALIYUN_API_KEY` / `ALIYUN_ENDPOINT` | 空 | aliyun 模式用 |
| `ADMIN_PASSWORD` | `feng` | 管理员密码 |
| `PGHOST/PGPORT/PGUSER/PGPASSWORD/PGDATABASE` | localhost/5432/jasper×3 | 连接库（可用 `DATABASE_URL` 替代） |

---

## 11. 部署与运维

### A. 本机

```bash
cd backend && go run .          # 或 go build -o jasperlee.exe . && ./jasperlee.exe
cd frontend && npm run dev      # 开发；或 npm run build（产物给后端静态托管）
```

**MCP 模式**：`./jasperlee -mcp`（stdio，供宿主进程托管）；HTTP 入口为 `POST /mcp`；宿主接入说明见 [MCP.md](MCP.md)。

### B. Docker Compose

```bash
# 仅开发数据库
docker compose up -d db                      # postgres:15，md5 认证，jasper/jasper/jasper

# 一键容器化（app 依赖 db 健康检查；模型/RAG 配置经 backend/.env 注入）
docker compose up --build -d                 # 访问 http://localhost:8080
docker compose logs -f app                   # 看日志
docker compose down -v                       # 停止并清空数据卷
```

### C. 开源隧道（不买服务器）

```bash
bash deploy.sh    # Git Bash / Linux：构建→启动→cloudflared（优先）/localtunnel（兜底）→打印公网地址
deploy.bat        # Windows 双击
bash start.sh / start.bat  # 仅本机+局域网启动
```

### 排障要点（来自 `docs/脚本排障记录.md`）

- `start.bat` 必须是纯 ASCII（cmd 按 GBK 解析 utf-8 中文会吞行）。
- cmd 延时用 `ping -n`（Git Bash 的 GNU `timeout` 会遮蔽 cmd 的 `timeout /t`）。
- 后端静态目录与 `.env` 均相对 **exe 所在目录**解析，任意 cwd 启动都可用。
- 向量化引导在后台执行（网络慢不阻塞启动）；GLM embedding 15s 超时 + 1 次重试，失败优雅降级。

---

## 12. 扩展指南

**加一个新 API**：在 `internal/api` 新建/扩展处理函数 → `main.go` 注册路由。

**接一个新的 Embedding/向量库**：实现 `rag.EmbeddingProvider` / `rag.VectorStore` 接口 → 在 `rag.New` 的 switch 挂上 provider 名。

**接一个新模型提供方**：OpenAI 兼容的直接改 `MODEL_BASE_URL/NAME`；非兼容的扩展 `llm.Client`（保持 `Stream()` 签名）。

**加一个新前端视图**：`types.ts` 的 `View` 加枚举 → 组件放 `components/` → `App.tsx` 注册 → `IconRail`/`store.setView` 接入口。

**扩展自动应答（路线建议）**：`autoReplyInbox` 目前是「读取→检索→生成→落库」最小闭环，可插入：JD 结构化解析、生成岗位匹配报告文件并 `publishArtifacts` 到「Jasper 的空间」、向投递者返回深链。

---

## 13. 已知限制与路线图

| # | 现状 | 影响 | 建议 |
|---|---|---|---|
| 1 | 收件箱自动应答不生成产物文件 | 回复仅在收件箱展示 | 可生成匹配度报告并发布（可选） |
| 2 | PDF 不做文本抽取（docx 已支持；本机 Go module 代理不可达，暂无法引入解析库） | PDF 仅下载、正文不入库 | 环境允许时接入 PDF 解析（本地代理 / 离线 vendor） |
| 3 | pgpool 不支持 SCRAM & SSL | 连默认配置的 PG14+ 会降级内存 | 补 SCRAM-SHA-256 或文档化 md5 要求（compose 已配 md5） |
| 4 | 访客 Key 永久有效（无 TTL） | 泄露后可被长期使用 | 增加过期时间/重置策略（后续） |
| 5 | 记忆系统 v1 | 人设/RAG 在线；无访客维度记忆 | 会话/收件箱记忆隔离 + 摘要记忆（M5） |
| 6 | 冒烟验证依赖会话临时脚本（.cowork-temp） | 回归需重建脚本 | 可固化到 scripts/（可选） |
| 7 | mock 模型为固定腔调 | 开发演示可读性一般 | 可让 mock 也走灵魂模板（低成本） |
| 8 | 前端离线时快捷键回执不落库 | 刷新后丢失 | 已有真实会话兜底，影响小 |

---

## 14. 变更记录

**2026-10-04（本 Wiki 生成同批改造）**

- 新增：收件箱**自动应答**（后端 `autoReplyInbox` + 存储 `SetInboxReply` + 前端三态展示与轮询）。
- 新增：会话**自动命名**（`maybeAutoTitle`，默认标题自动取首条消息 ≤18 字）。
- 新增：对话/工作区**额度硬校验**（用尽返回 402 明确文案；前端优先展示后端错误体）。
- 优化：RAG 关键词模式**零命中降噪**（存在正命中时过滤零分片段）。
- 优化：`/api/artifacts` 空结果返回 `[]` 而非 `null`。
- 新增：本 Code Wiki（`docs/代码Wiki.md`）。

**2026-10-04（第二波）**

- 新增：**访客维度会话/工作区隔离**——`Conversation`/`Workspace` 增加 `OwnerKey`；列表/读写/运行仅限归属者，越权一律 404；PG `conversations` 表幂等新增 `owner_key` 列。
- 修复：`UpdateConversation` 变量重声明（`ok := true` → `ok = true`）。

**2026-10-04（第三波）**

- 新增：**docx（Word）纯文本抽取**（`internal/textract`）——资料库上传的 docx 可入库/检索/引用；右栏预览直接显示正文；收件箱 docx 投递可被自动应答阅读。
- 新增：**SSE 心跳**——对话流每 15s 发送注释帧 `: ping`，防隧道/代理断开长连接（前端解析器自动忽略）。（已在 35s 长流上实测观察到 2 次心跳）

**2026-10-04（第四波）**

- 新增：**文件与收件箱的访客隔离**——`FileMeta`/`InboxItem` 增加 `OwnerKey`；「我分享的」上传件与收件箱条目仅归属者可见/可操作（列表/预览/下载/入库/改名/删除、@引用与工作区引用过滤均生效，越权 403）；Jasper 公开资料对全部访客可见。

**2026-10-04（第五波）**

- 新增：**Agent 产物归属过滤**——`/api/artifacts` 仅透出当前 Key 自己任务产生的产物；无工作区关联的历史产物兼容可见。
- 新增：**收件箱自动应答额度守卫**——Key 额度用尽时不再生成回复（`status=failed`），防绕过对话额度限制刷模型。

**2026-10-04（第六波）**

- 架构：抽出传输无关的 `internal/core`（领域服务装配），`internal/api` 变薄，与新增的 `internal/mcp` 共享同一 core。
- 新增：**MCP 服务**（零依赖 JSON-RPC 2.0）——stdio（`-mcp`）与 HTTP（`POST /mcp`）双传输；7 个工具 + 资料库资源；协议版本 2025-06-18。对接说明见 `docs/MCP.md`。
- 人设数据（`AvatarData`）从 HTTP 层迁入 core，供两种传输复用。

**2026-10-04（第七波）**

- 新增：**平台看板**（媒体 / 博客 / 练习平台的账号与链接）——`GET /api/social` 公开浏览，管理员可增删改（`POST/PATCH/DELETE /api/social`）；23 个预置平台（代码/社区/博客/练习/社交五类）。
- 前端新增「平台看板」栏目：分类卡片 + 品牌色徽标 + **双击就地编辑**（管理员）+ 「添加平台」。
- 存储：memStore / pgStore 增加平台看板持久化（`store.json` 的 `social` / `social_links` 表）；补单元测试。

**更早**：M0 框架 → M1 对话（SSE/Key 计量/模型选择）→ M2 资料库（上传/预览/入库/RAG）→ M3 Agent 工作区（引用挂载/产物发布/Response details）→ Docker/PG/隧道部署 → 脚本排障（编码/路径/异步引导）。

---

*本 Wiki 力求与代码同步；改动架构、API、配置时请顺手更新对应章节。*
