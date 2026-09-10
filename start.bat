@echo off
title JasperLee Digital Avatar - Launcher
cd /d "%~dp0"

echo ============================================
echo   JasperLee - Digital Avatar for Recruiters
echo ============================================
echo.

if not exist "backend\.env" (
  echo [WARN] backend\.env not found - no GLM key, will run in mock mode
  echo        see README.md to configure
  echo.
)

rem ---- try to start Docker PostgreSQL dev db ----
set DOCKER_OK=0
docker info >nul 2>&1 && set DOCKER_OK=1
if "%DOCKER_OK%"=="1" (
  echo [0/3] Starting Docker PostgreSQL dev db (jasper@localhost:5432) ...
  docker compose up -d db
  if errorlevel 1 (
    echo [WARN] docker compose up db failed - backend will fall back to built-in store
  ) else (
    echo [0/3] Database ready. Empty the volume with: docker compose down -v
  )
) else (
  echo [INFO] Docker not available - backend will fall back to built-in store
)

if not exist "frontend\node_modules" (
  echo [1/3] Installing frontend deps - first run, may take a while
  pushd frontend
  call npm install --no-fund --no-audit
  popd
) else (
  echo [1/3] Frontend deps ready
)

echo [2/3] Building frontend + backend...
pushd frontend
call npm run build
if errorlevel 1 goto :fail
popd
pushd backend
go build -o jasperlee.exe .
if errorlevel 1 goto :fail
popd
if not exist "backend\jasperlee.exe" goto :fail

for /f "usebackq tokens=1" %%i in (`powershell -NoProfile -Command "(Get-NetIPAddress -AddressFamily IPv4 | Where-Object {$_.IPAddress -notlike '127.*' -and $_.IPAddress -notlike '169.254.*'} | Select-Object -First 1).IPAddress" 2^>nul`) do set LANIP=%%i

echo.
echo  ============================================
echo   Ready!
echo   Local:    http://localhost:8080
if defined LANIP echo   LAN:      http://%LANIP%:8080  - share with recruiters
echo   Stop:     press Ctrl+C or close this window
echo  ============================================
echo.
echo   Starting server... browser will open in a few seconds

REM delay open browser until server ready (about 5s)
start /b cmd /c "ping -n 6 127.0.0.1 >nul & start http://localhost:8080"

REM run server in foreground - close this window to stop
"%~dp0backend\jasperlee.exe"
pause
exit /b 0

:fail
echo.
echo [ERROR] Build failed - check messages above
echo.
pause
exit /b 1