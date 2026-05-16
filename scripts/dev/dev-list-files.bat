@echo off
setlocal EnableExtensions EnableDelayedExpansion

rem Этот скрипт формирует tree.md со структурой файлов репозитория.
rem Нужен, чтобы передать структуру проекта в чат-агент для быстрой навигации по коду.

rem Resolve repo root as parent of this script directory.

pushd "%~dp0.." >nul || (
  echo Failed to resolve repository root.
  exit /b 1
)
set "repo_root=%CD%"
set "basepath=%repo_root%\"
set "out_file=%repo_root%\tree.md"

rem Build file list from repo root while excluding heavy/system folders.
(
  for /f "tokens=*" %%F in ('
    robocopy "%repo_root%" NULL /S /L /NJH /NJS /NS /NC /NDL /XD .git node_modules bin obj build cmd dist entware .svelte-kit prebuilt static coder kmod scripts .github .gocache
  ') do (
    set "filepath=%%F"
    set "rel=!filepath:%basepath%=!"
    for %%A in ("!rel!") do echo(%%~A
  )
) > "%out_file%"

popd >nul
echo Done. File list saved to "%out_file%"
exit /b 0
