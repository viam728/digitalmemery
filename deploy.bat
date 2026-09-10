@echo off
title JasperLee Digital Avatar - Deploy (Tunnel)
cd /d "%~dp0"

echo ============================================
echo   JasperLee - End-to-End Network Deploy
echo   open-source tunnel: cloudflared / localtunnel
echo ============================================
echo.

if not exist "backend\.env" (
  echo [WARN] backend\.env not found - no GLM key, mock mode
  echo.
)

rem ---- try to start Docker PostgreSQL dev db ----
set DOCKER_OK=0
docker info >nul 2>&1 && set DOCKER_OK=1
if "%DOCKER_OK%"=="1" (
  echo [0/4] Starting Docker PostgreSQL dev db (jasper@localhost:5432) ...
  docker compose up -d db
  if errorlevel 1 (
    echo [WARN] docker compose up db failed - backend will fall back to built-in store
  ) else (
    echo [0/4] Database ready. Empty the volume with: docker compose down -v
  )
) else (
  echo [INFO] Docker not available - backend will fall back to built-in store
)

if not exist "frontend\node_modules" (
  echo [1/4] Installing frontend deps - first run, may take a while
  pushd frontend
  call npm install --no-fund --no-audit
  popd
) else (
  echo [1/4] Frontend deps ready
)

echo [2/4] Building frontend + backend...
pushd frontend
call npm run build
if errorlevel 1 goto :fail
popd
pushd backend
go build -o jasperlee.exe .
if errorlevel 1 goto :fail
popd
if not exist "backend\jasperlee.exe" goto :fail

echo [3/4] Starting local server :8080 ...
start /b cmd /c "ping -n 6 127.0.0.1 >nul & start http://localhost:8080"
"%~dp0backend\jasperlee.exe" >nul 2>&1

rem if server exits we reach here only after it is killed
echo [4/4] Starting tunnel...

set TUNNEL_URL=
where cloudflared >nul 2>&1
if errorlevel 1 (
  echo       cloudflared not found - trying localtunnel via npx
  start /b cmd /c "npx --yes localtunnel --port 8080 > %TEMP%\jl-tunnel.log 2>&1"
  ping -n 12 127.0.0.1 >nul
  for /f "usebackq tokens=*" %%i in (`findstr /R /C:"loca.lt" %TEMP%\jl-tunnel.log`) do set TUNNEL_URL=%%i
) else (
  start /b cmd /c "cloudflared tunnel --url http://localhost:8080 --no-autoupdate > %TEMP%\jl-tunnel.log 2>&1"
  ping -n 12 127.0.0.1 >nul
  for /f "usebackq tokens=1" %%i in (`findstr /R /C:"trycloudflare.com" %TEMP%\jl-tunnel.log`) do set TUNNEL_URL=%%i
)

echo.
echo  ============================================
echo   Deployed! Share these with recruiters:
if defined TUNNEL_URL echo   PUBLIC: %TUNNEL_URL%
if not defined TUNNEL_URL echo   PUBLIC: tunnel URL not captured yet - see %TEMP%\jl-tunnel.log
echo   LOCAL:  http://localhost:8080
echo   Stop:   close this window / kill jasperlee.exe
echo  ============================================
echo.
pause
exit /b 0

:fail
echo.
echo [ERROR] Build failed - check messages above
echo.
pause
exit /b 1