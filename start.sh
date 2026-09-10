#!/usr/bin/env bash
# JasperLee 数字分身 - 一键启动（数据库 → 构建 → 启动 → 打开浏览器）
set -e
cd "$(dirname "$0")"

echo "============================================"
echo "  JasperLee - 李俊锋的数字分身 - 一键启动"
echo "============================================"

if [ ! -f backend/.env ]; then
  echo "[警告] 未找到 backend/.env（缺少 GLM 密钥），将以 mock 模式运行，配置见 README.md"
  echo
fi

# ---- 尝试用 Docker 提供 PostgreSQL 开发环境（用户核心诉求）----
if command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then
  echo "[0/3] 启动 Docker PostgreSQL 开发库 (jasper/jasper@localhost:5432) ..."
  if docker compose up -d db; then
    echo "[0/3] 数据库已就绪。仅想启库：docker compose up -d db；清空卷：docker compose down -v"
  else
    echo "[警告] docker compose 启动 db 失败——后端将回退到内置存储（不依赖数据库也能跑）"
  fi
else
  echo "[提示] Docker 不可用——后端将回退到内置存储（不依赖数据库也能跑）"
fi

if [ ! -d frontend/node_modules ]; then
  echo "[1/3] 安装前端依赖（首次，可能需要几分钟）..."
  (cd frontend && npm install --no-fund --no-audit)
else
  echo "[1/3] 前端依赖已就绪"
fi

echo "[2/3] 构建前端与后端..."
(cd frontend && npm run build >/dev/null)
(cd backend && go build -o jasperlee.exe .)

echo "[3/3] 启动服务 :8080 ..."
# 以全路径启动 exe（其内部已按 exe 目录解析 .env 与 dist，与 cwd 解耦）
(cd backend && exec ./jasperlee.exe) &
SERVER_PID=$!
trap 'echo; echo "[JasperLee] 已停止"; kill $SERVER_PID 2>/dev/null' INT TERM

# 等待服务就绪（最多 20 秒）
for i in $(seq 1 20); do
  curl -s -o /dev/null http://localhost:8080/api/health 2>/dev/null && break
  sleep 1
done

LANIP=$(ipconfig 2>/dev/null | grep -A1 'IPv4' | grep -oE '[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+' | grep -vE '^127\.|^169\.254\.' | head -1)

echo
echo "  ============================================"
echo "   已启动！"
echo "   本机访问:    http://localhost:8080"
[ -n "$LANIP" ] && echo "   局域网访问:  http://$LANIP:8080 （发给招聘者/同事）"
echo "   停止服务:    按 Ctrl+C"
echo "  ============================================"
echo

# 打开浏览器（Git Bash 下用 cmd 启动）
cmd //c start "" "http://localhost:8080" >/dev/null 2>&1 || true

wait