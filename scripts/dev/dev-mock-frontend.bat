@echo off
setlocal

rem Usage:
rem   scripts\dev\dev-mock-frontend.bat [FRONT_PORT] [PRISM_PORT] [MOCK_PROXY_PORT]
rem Example:
rem   scripts\dev\dev-mock-frontend.bat 4173 8080 8081

set "FRONT_PORT=%~1"
if "%FRONT_PORT%"=="" set "FRONT_PORT=4173"

set "MOCK_PORT=%~2"
if "%MOCK_PORT%"=="" set "MOCK_PORT=8080"

set "MOCK_PROXY_PORT=%~3"
if "%MOCK_PROXY_PORT%"=="" set "MOCK_PROXY_PORT=8081"

for %%I in ("%~dp0..\..") do set "ROOT=%%~fI"
set "FRONTEND_DIR=%ROOT%\frontend"
set "SWAGGER=%ROOT%\internal\openapi\swagger.yaml"
set "AWGM_DEV_AUTO_KILL=%AWGM_DEV_AUTO_KILL%"
if "%AWGM_DEV_AUTO_KILL%"=="" set "AWGM_DEV_AUTO_KILL=1"

if not exist "%FRONTEND_DIR%\package.json" (
  echo [ERROR] frontend\package.json not found.
  exit /b 1
)

if not exist "%SWAGGER%" (
  echo [ERROR] internal\openapi\swagger.yaml not found.
  exit /b 1
)

echo Checking dev ports...
call :ensure_port_available "%MOCK_PORT%" "Prism mock"
if errorlevel 1 exit /b 1

call :ensure_port_available "%MOCK_PROXY_PORT%" "mock-proxy"
if errorlevel 1 exit /b 1

call :ensure_port_available "%FRONT_PORT%" "Vite frontend"
if errorlevel 1 exit /b 1

echo Starting Prism mock on http://127.0.0.1:%MOCK_PORT% ...
start "AWGM Mock API" cmd.exe /k cd /d "%FRONTEND_DIR%" ^&^& npx -y @stoplight/prism-cli mock "%SWAGGER%" -p %MOCK_PORT% --host 127.0.0.1

echo Waiting for Prism to be ready...
powershell -NoProfile -ExecutionPolicy Bypass -Command "$ErrorActionPreference='SilentlyContinue'; $ok=$false; for($i=0;$i -lt 40;$i++){ try { $r=Invoke-WebRequest -Uri 'http://127.0.0.1:%MOCK_PORT%/health' -UseBasicParsing -TimeoutSec 1; if($r.StatusCode -ge 200){ $ok=$true; break } } catch {}; Start-Sleep -Milliseconds 250 }; if(-not $ok){ exit 1 }"
if errorlevel 1 (
  echo [ERROR] Prism did not become ready on http://127.0.0.1:%MOCK_PORT%
  echo Check "AWGM Mock API" window for details.
  exit /b 1
)

echo Starting stateful mock-proxy on http://127.0.0.1:%MOCK_PROXY_PORT% (upstream Prism: %MOCK_PORT%) ...
start "AWGM Mock Proxy" cmd.exe /k cd /d "%FRONTEND_DIR%" ^&^& set UPSTREAM=http://127.0.0.1:%MOCK_PORT% ^&^& set PORT=%MOCK_PROXY_PORT% ^&^& node scripts/mock-proxy.mjs

echo Waiting for mock-proxy to be ready...
powershell -NoProfile -ExecutionPolicy Bypass -Command "$ErrorActionPreference='SilentlyContinue'; $ok=$false; for($i=0;$i -lt 40;$i++){ try { $r=Invoke-WebRequest -Uri 'http://127.0.0.1:%MOCK_PROXY_PORT%/health' -UseBasicParsing -TimeoutSec 1; if($r.StatusCode -ge 200){ $ok=$true; break } } catch {}; Start-Sleep -Milliseconds 250 }; if(-not $ok){ exit 1 }"
if errorlevel 1 (
  echo [ERROR] mock-proxy did not become ready on http://127.0.0.1:%MOCK_PROXY_PORT%
  echo Check "AWGM Mock Proxy" window for details.
  exit /b 1
)

echo Starting Vite frontend on http://127.0.0.1:%FRONT_PORT% ...
start "AWGM Frontend Mock" cmd.exe /v:on /k cd /d "%FRONTEND_DIR%" ^&^& set VITE_API_STRIP_PREFIX=true ^&^& set VITE_API_TARGET=http://127.0.0.1:%MOCK_PROXY_PORT% ^&^& echo VITE_API_STRIP_PREFIX=!VITE_API_STRIP_PREFIX! ^&^& echo VITE_API_TARGET=!VITE_API_TARGET! ^&^& npx vite dev --host 127.0.0.1 --port %FRONT_PORT% --strictPort

echo.
echo Mock stack started.
echo Frontend: http://127.0.0.1:%FRONT_PORT%
echo Prism API: http://127.0.0.1:%MOCK_PORT%
echo Mock Proxy: http://127.0.0.1:%MOCK_PROXY_PORT%
echo.
echo Close all opened terminal windows to stop.

endlocal
exit /b 0

:ensure_port_available
set "CHECK_PORT=%~1"
set "CHECK_NAME=%~2"
powershell -NoProfile -ExecutionPolicy Bypass -Command ^
  "$ErrorActionPreference='Stop';" ^
  "$port=[int]$env:CHECK_PORT;" ^
  "$name=$env:CHECK_NAME;" ^
  "$autoKill=$env:AWGM_DEV_AUTO_KILL;" ^
  "if ([string]::IsNullOrWhiteSpace($autoKill)) { $autoKill='1' };" ^
  "$listeners=Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue | Where-Object { $_.LocalAddress -in @('127.0.0.1','0.0.0.0','::1','::') };" ^
  "if (-not $listeners) { exit 0 };" ^
  "$owningProcessIds=$listeners | Select-Object -ExpandProperty OwningProcess -Unique;" ^
  "foreach ($processId in $owningProcessIds) {" ^
  "  $proc=Get-CimInstance Win32_Process -Filter \"ProcessId=$processId\";" ^
  "  $cmd=''; if ($proc -and $proc.CommandLine) { $cmd=$proc.CommandLine };" ^
  "  $exe='process'; if ($proc -and $proc.Name) { $exe=$proc.Name };" ^
  "  $isAwgm=($cmd -match 'awg-manager' -or $cmd -match 'mock-proxy\.mjs' -or $cmd -match '@stoplight' -or $cmd -match 'prism-cli' -or $cmd -match 'vite dev' -or $cmd -match 'node_modules.*vite');" ^
  "  if (-not $isAwgm) {" ^
  "    Write-Host \"[ERROR] $name port $port is occupied by non-AWGM process PID $processId ($exe).\";" ^
  "    if ($cmd) { Write-Host \"        $cmd\" };" ^
  "    exit 2;" ^
  "  };" ^
  "  if ($autoKill -eq '0') { Write-Host \"[ERROR] $name port $port is occupied by AWGM dev process PID $processId, but AWGM_DEV_AUTO_KILL=0.\"; exit 2 };" ^
  "  Write-Host \"[INFO] Stopping stale AWGM dev process on port ${port}: PID $processId ($exe)\";" ^
  "  Stop-Process -Id $processId -Force -ErrorAction Stop;" ^
  "};" ^
  "Start-Sleep -Milliseconds 300; exit 0"
exit /b %ERRORLEVEL%
