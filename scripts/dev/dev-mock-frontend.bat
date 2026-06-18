@echo off
setlocal

rem Usage:
rem   scripts\dev\dev-mock-frontend.bat [FRONT_PORT] [PRISM_PORT] [MOCK_PROXY_PORT]
rem Example:
rem   scripts\dev\dev-mock-frontend.bat 5173 8080 8081
rem Behavior:
rem   If a requested port is unavailable, the script automatically tries
rem   the next ports until it finds a usable one.

set "FRONT_PORT=%~1"
if "%FRONT_PORT%"=="" set "FRONT_PORT=5173"

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
call :select_port MOCK_PORT "Prism mock"
if errorlevel 1 exit /b 1

call :select_port MOCK_PROXY_PORT "mock-proxy"
if errorlevel 1 exit /b 1

call :select_port FRONT_PORT "Vite frontend"
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

:select_port
setlocal EnableDelayedExpansion
set "TARGET_VAR=%~1"
set "TARGET_NAME=%~2"
call set "BASE_PORT=%%%TARGET_VAR%%%"
if not defined BASE_PORT (
  echo [ERROR] No base port provided for %TARGET_NAME%.
  exit /b 1
)

set /a TRY_PORT=%BASE_PORT%
set /a MAX_ATTEMPTS=50
set /a ATTEMPT=0

:select_port_loop
set /a ATTEMPT+=1
if /I "%TARGET_VAR%"=="FRONT_PORT" (
  if !ATTEMPT! equ 1 (
    if !TRY_PORT! geq 4111 if !TRY_PORT! leq 4410 (
      echo [INFO] Vite frontend requested port !TRY_PORT! falls into a Windows-restricted range, switching search to 5173+.
      set /a TRY_PORT=5173
    )
  )
)
if /I not "%TARGET_VAR%"=="MOCK_PORT" if "!TRY_PORT!"=="%MOCK_PORT%" (
  set /a TRY_PORT+=1
  goto select_port_loop
)
if /I not "%TARGET_VAR%"=="MOCK_PROXY_PORT" if "!TRY_PORT!"=="%MOCK_PROXY_PORT%" (
  set /a TRY_PORT+=1
  goto select_port_loop
)
if /I not "%TARGET_VAR%"=="FRONT_PORT" if "!TRY_PORT!"=="%FRONT_PORT%" (
  set /a TRY_PORT+=1
  goto select_port_loop
)
call :ensure_port_available "!TRY_PORT!" "%TARGET_NAME%"
if not errorlevel 1 goto select_port_found

if !ATTEMPT! geq !MAX_ATTEMPTS! (
  echo [ERROR] Could not find an available port for %TARGET_NAME% after !MAX_ATTEMPTS! attempts starting from %BASE_PORT%.
  exit /b 1
)

set /a TRY_PORT+=1
goto select_port_loop

:select_port_found
if not "!TRY_PORT!"=="%BASE_PORT%" (
  echo [WARN] %TARGET_NAME% requested port %BASE_PORT% unavailable, using !TRY_PORT! instead.
)
endlocal & set "%~1=%TRY_PORT%"
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
  "Start-Sleep -Milliseconds 300;" ^
  "$listener=$null;" ^
  "try {" ^
  "  $listener=[System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Parse('127.0.0.1'), $port);" ^
  "  $listener.Start();" ^
  "  $listener.Stop();" ^
  "  exit 0;" ^
  "} catch {" ^
  "  if ($listener) { try { $listener.Stop() } catch {} };" ^
  "  Write-Host \"[WARN] $name port $port is unavailable: $($_.Exception.Message)\";" ^
  "  exit 2;" ^
  "}"
exit /b %ERRORLEVEL%
