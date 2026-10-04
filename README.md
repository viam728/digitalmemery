# JasperLee · 李俊锋的数字分身

**定位：给招聘者的个人数字分身。** 招聘者免登录进入，一键申请临时 Key 后可以：
1. **看个人主页**（简历/技能/项目作品/时间线）；
2. **与分身问答**（分身基于「关于我」知识库 RAG 回答，流式输出）；
3. **浏览资料库**（个人作品文档，支持预览/入库/检索）；
4. **上传文件给我**（JD/招聘资料/问题清单，落入收件箱，数字分身自动阅读并回复）。

技术栈：React + Go + Agent，参照 Codex / Cherry Studio 交互范式（深色三栏布局，左导航 + 中主区 + 右上下文/引用面板）。

## 结构

```
digitalmemery/
├─ docs/需求书.md          # 产品需求书（含 UI 对照、模块、RAG 免费 API 评估）
├─ data/                   # 后端数据目录（store.json、上传文件、向量索引）
├─ frontend/               # React + Vite + TS + Tailwind + Zustand
│  └─ public/jasperlee.html # JasperLee 数字分身内置简介 HTML
└─ backend/                # Go 1.26 标准库 net/http
   └─ internal/{api,storage,rag,agent,config,models}
```

## 文档

- [Code Wiki](docs/代码Wiki.md) —— 架构 / 模块 / API / 数据流 / 扩展指南（速查手册）
- [产品需求书](docs/需求书.md)
- [脚本排障记录](docs/脚本排障记录.md)

## 启动

### 后端（:8080）
```bash
cd backend && go run .
# 配置在 backend/.env（含真实 GLM 密钥，勿公开提交）：
#   MODEL_PROVIDER=glm（mock 关闭真实模型；glm 走 OpenAI 兼容）
#   MODEL_API_KEY=<智谱 key>  MODEL_BASE_URL=https://open.bigmodel.cn/api/paas/v4
#   MODEL_NAME=glm-4-flash（免费档，可换 glm-4.5 / glm-4-air）
#   RAG_PROVIDER=mock|local|aliyun
```

### 前端（:5173，代理 /api 到 :8080）
```bash
cd frontend && npm install && npm run dev
```

浏览器打开 http://localhost:5173

## Docker 开发和部署

> 前置：已安装 Docker + Docker Compose（Windows 用 Docker Desktop，Git Bash 里执行 `docker compose ...`）。

### 用法 A：仅数据库开发环境（核心诉求）

只想用 Docker 提供一个真实 PostgreSQL 开发库，后端仍在本机跑：

```bash
# 1) 启动 Postgres（仅 db 服务，端口 5432）
docker compose up -d db

# 2) 本机启动后端（默认连 localhost:5432）
cd backend && go run .
# 连接参数（用户/密码/库均为 jasper）：
#   PGHOST=localhost  PGPORT=5432
#   PGUSER=jasper     PGPASSWORD=jasper  PGDATABASE=jasper
#   （也可以用一条 DATABASE_URL=postgres://jasper:jasper@localhost:5432/jasper 代替）
```

- 开发库连接凭证：用户 `jasper`、密码 `jasper`、库名 `jasper`，认证方式 `md5`（配合后端 `internal/pgpool` 纯标准库客户端）。
- 若 Docker 不可用或连不上数据库，后端会**降级到内置存储**（内存 + `data/` JSON），仍可启动演示。
- 清空数据库数据：`docker compose down -v`（`-v` 同时删除 `pgdata` 数据卷）。

### 用法 B：一键容器化部署

单镜像同时托管后端 + 前端构建产物，db 起来后跑 app：

```bash
docker compose up --build -d
# 访问：http://localhost:8080
```

- `app` 依赖 `db` 健康检查（`service_healthy`）后才启动；后端连 `PGHOST=db`。
- 模型/RAG/管理员配置经 `env_file: ./backend/.env` 在运行时注入；`compose` 的 `environment`（PGHOST=db 等）优先级高于 `env_file`，两者不冲突。
- `backend/.env` 含真实 GLM 密钥，已被 `.dockerignore` 排除，**不会打包进镜像**。
- 相关文件：`Dockerfile`（多阶段）、`docker-compose.yml`、`.dockerignore`。

### 日常运维

```bash
docker compose ps          # 查看服务状态
docker compose logs -f db  # 跟踪数据库日志
docker compose logs -f app # 跟踪后端日志
docker compose down        # 停止（保留卷）
docker compose down -v     # 停止并清空数据卷
```

## 三视图（对照截图）
1. **对话（截图一）** —— 顶部模型/项目选择器；左栏置顶「新对话」；中区空态「今天想做点什么？」；底部输入框（/ 搜索路径、@ 引用文件/会话）。
2. **资料库（截图二）** —— 左导航 + Tab（最近/我的/我共享的）+ 文件表格 + 「了解资料库」卡片；文件可一键**向量化入库**。
3. **Agent 工作区（截图三）** —— 会话流 + `Response details`（模型/状态/耗时/token）；**右侧「文件」面板**树形展示工作区引用文件，可 `@ 引用`。

额外：左栏底部「JasperLee 数字分身」打开内置 HTML 简介页。

## RAG（免费 API 调研结论）
文档入库流程：解析 → 分块（Chunk）→ 向量化（Embedding）→ 检索（Top-K）。

| 方案 | 成本 | 说明 |
|---|---|---|
| 阿里云百炼 + DashVector | 有免费试用额度 | 通义 Text-Embedding-V3，推荐（需 ALIYUN_API_KEY/ENDPOINT） |
| 本地 BGE-M3 | 完全免费 | 离线、零 API 成本，需本地算力 |
| Mock（默认） | 免费 | 关键词打分，开发演示用 |

通过 `EmbeddingProvider` / `VectorStore` 接口可插拔切换，默认 `mock` 即开即用。

## 里程碑
- [x] M0 框架：前后端跑通，三视图 + 右栏文件树
- [x] M1 对话：流式 SSE + Key 计量 + 模型选择器
- [x] M2 资料库：列表/入库 + RAG 检索
- [x] M3 Agent：任务编排 + 引用挂载 + Response details
- [x] M4 数字分身 v1：人设/SKILL + RAG 引导 + 收件箱自动应答 + 会话自动命名
- [ ] M5 记忆增强：访客维度会话隔离、PDF/DOCX 文本抽取、SSE 心跳（规划中）

## 端到端网络部署（开源内网穿透）
不买服务器，用开源隧道把本机服务暴露到公网（cloudflared 优先，localtunnel 兜底）：

```bash
# 一键端到端部署（构建→启动→拉起隧道→打印公网地址）
bash deploy.sh        # Git Bash
deploy.bat            # Windows 双击
```

- 公网地址：`https://xxx.trycloudflare.com` 或 `https://xxx.loca.lt`（脚本自动抓取打印）
- 本机/局域网：http://localhost:8080 / http://<局域网IP>:8080
- 未装穿透工具会自动尝试 `winget install --id Cloudflare.cloudflared` 或走 `npx localtunnel`
- 说明：隧道 URL 需要能连通穿透服务商的网络环境（本机若经代理/内网受限，需在正常公网环境执行）

## 管理员入口

首页底部「管理员入口」，输入密码（默认 `feng`，可在 `.env` 的 `ADMIN_PASSWORD` 修改）进入：
- 系统看板：运行时长、模型/RAG 提供方、文件/会话/消息/收件箱数量、token 总额度与消耗（含使用率进度条）
- Key 申请管理：查看全部申请、启用/停用、重置额度、删除
- 收件箱消息：招聘者上传的文件与留言
与招聘者界面完全分离，普通访问不受影响。

## Key 机制（免登录）

访客打开应用直接进入 Key 门控页：
- **申请新 Key** 自动发放 100 万 token 额度 + 资料库访问权限；
- **粘贴已有 Key** 校验后进入。
所有 `/api` 接口经 `Authorization: Bearer <key>` 鉴权；资料库接口额外校验 `library` 权限。每次对话完成按模型用量扣减 Key 额度（`/api/keys/me` 查询剩余）。
