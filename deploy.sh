#!/usr/bin/env bash
# JasperLee 端到端网络部署（开源内网穿透）
# 流程：数据库 → 构建前端+后端 → 启动本地服务 → 拉起开源隧道（cloudflared 优先，localtunnel 兜底）
#       → 打印公网 HTTPS 地址，招聘者随时随地可访问
set -e
cd "$(dirname "$0")"

echo "============================================"
echo "  JasperLee - End-to-End Network Deploy"
echo "  (open-source tunnel: cloudflared / localtunnel)"
echo "============================================"

if [ ! -f backend/.env ]; then
  echo "[WARN] backend/.env not found - no GLM key, mock mode"
  echo
fi

# ---- 尝试用 Docker 提供 PostgreSQL 开发库 ----
if command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then
  echo "[0/4] Starting Docker PostgreSQL dev db (jasper@localhost:5432) ..."
  docker compose up -d db || echo "[WARN] docker compose up db failed - fall back to built-in store"
else
  echo "[INFO] Docker not available - fall back to built-in store"
fi

# 1. 前端依赖
if [ ! -d frontend/node_modules ]; then
  echo "[1/4] Installing frontend deps..."
  (cd frontend && npm install --no-fund --no-audit)
else
  echo "[1/4] Frontend deps ready"
fi

# 2. 构建
echo "[2/4] Building frontend + backend..."
(cd frontend && npm run build >/dev/null)
(cd backend && go build -o jasperlee.exe .)

# 3. 启动本地服务
echo "[3/4] Starting local server :8080 ..."
(cd backend && exec ./jasperlee.exe) &
SERVER_PID=$!
trap 'echo; echo "[JasperLee] stopped"; kill $SERVER_PID 2>/dev/null; [ -n "$TUNNEL_PID" ] && kill $TUNNEL_PID 2>/dev/null' INT TERM

# 等待服务就绪
for i in $(seq 1 20); do
  curl -s -o /dev/null http://localhost:8080/api/health 2>/dev/null && break
  sleep 1
done

# 4. 启动开源隧道
TUNNEL_URL=""
TUNNEL_PID=""
if command -v cloudflared >/dev/null 2>&1; then
  echo "[4/4] Starting cloudflared quick tunnel..."
  cloudflared tunnel --url http://localhost:8080 --no-autoupdate >/tmp/jasperlee-tunnel.log 2>&1 &
  TUNNEL_PID=$!
  sleep 7
  TUNNEL_URL=$(grep -oE 'https://[a-z0-9-]+\.trycloudflare\.com' /tmp/jasperlee-tunnel.log | head -1)
elif command -v npx >/dev/null 2>&1; then
  echo "[4/4] Starting localtunnel (via npx)..."
  npx --yes localtunnel --port 8080 >/tmp/jasperlee-tunnel.log 2>&1 &
  TUNNEL_PID=$!
  sleep 9
  TUNNEL_URL=$(grep -oE 'https://[a-z0-9-]+\.loca\.lt' /tmp/jasperlee-tunnel.log | head -1)
else
  echo "[4/4] No tunnel tool found - installing cloudflared via winget..."
  winget install --id Cloudflare.cloudflared --accept-source-agreements --accept-package-agreements >/dev/null 2>&1 || true
  export PATH="$PATH:/c/Program Files (x86)/winget"
  command -v cloudflared >/dev/null 2>&1 && {
    cloudflared tunnel --url http://localhost:8080 --no-autoupdate >/tmp/jasperlee-tunnel.log 2>&1 &
    TUNNEL_PID=$!
    sleep 7
    TUNNEL_URL=$(grep -oE 'https://[a-z0-9-]+\.trycloudflare\.com' /tmp/jasperlee-tunnel.log | head -1)
  }
fi

echo
echo "  ============================================"
echo "   Deployed! Share these with recruiters:"
if [ -n "$TUNNEL_URL" ]; then
  echo "   PUBLIC (internet): $TUNNEL_URL"
else
  echo "   PUBLIC: tunnel failed to get URL yet - check /tmp/jasperlee-tunnel.log"
  echo "           (if no tunnel tool, install one: winget install --id Cloudflare.cloudflared)"
fi
echo "   LAN:    http://$(ipconfig 2>/dev/null | grep -A1 'IPv4' | grep -oE '[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+' | grep -vE '^127\.|^169\.254\.' | head -1):8080"
echo "   LOCAL:  http://localhost:8080"
echo "   Stop:   Ctrl+C"
echo "  ============================================"
echo

# 打开浏览器
cmd //c start "" "http://localhost:8080" >/dev/null 2>&1 || true

wait