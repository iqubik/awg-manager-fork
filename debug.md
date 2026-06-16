# DEBUG: Сборка IPK (руководство по командам для слабых ИИ-агентов)

Ниже ровно те сценарии, по которым уже были успешно собраны пакеты:
- `awg-manager_2.6.3_mipsel-3.4-kn.ipk` (MIPS)
- `awg-manager_2.6.3_aarch64-3.10-kn.ipk` (Filogic 820 / ARM64)

## 1. Где запускать

Откройте PowerShell в **корне репозитория** (там, где лежат `scripts`, `VERSION`, `go.mod`).

## 2. Быстрая проверка перед сборкой

```powershell
go version
Get-ChildItem scripts
```

Ожидается:
- `go version go1.23.12 windows/amd64` (или другой `go1.23.x`)
- в `scripts` есть `build-ipk.sh`, `build-backend.sh`, `build-frontend.sh`

## 3. Команда сборки IPK для MIPS (однострочная, без хардкода)

```powershell
$b="$(Split-Path -Parent (Split-Path -Parent (Get-Command git).Source))\bin\bash.exe";$w=(Get-Location).Path;$u="/$($w[0].ToString().ToLowerInvariant())"+$w.Substring(2).Replace('\','/');&$b -lc "cd '$u' && ./scripts/build-ipk.sh mipsel-3.4"
```

## 4. Что должно получиться

В конце лога должна быть строка вида:

```text
IPK package created: dist/awg-manager_2.6.3_mipsel-3.4-kn.ipk
```

Проверка файла:

```powershell
Get-Item .\dist\awg-manager_2.6.3_mipsel-3.4-kn.ipk
```

## 5. Команда сборки IPK для Filogic 820 (ARM64)

Filogic 820 собираем как `aarch64-3.10`.

```powershell
$b="$(Split-Path -Parent (Split-Path -Parent (Get-Command git).Source))\bin\bash.exe";$w=(Get-Location).Path;$u="/$($w[0].ToString().ToLowerInvariant())"+$w.Substring(2).Replace('\','/');&$b -lc "cd '$u' && ./scripts/build-ipk.sh aarch64-3.10"
```

Ожидаемая строка в конце:

```text
IPK package created: dist/awg-manager_2.6.3_aarch64-3.10-kn.ipk
```

Проверка файла:

```powershell
Get-Item .\dist\awg-manager_2.6.3_aarch64-3.10-kn.ipk
```

## 6. Если сборка падает с Bash ошибкой на Windows

Ошибка:

```text
fatal error - couldn't create signal pipe, Win32 error 5
```

Что делать:
- перезапустить PowerShell/терминал с повышенными правами
- повторить нужную однострочную команду из п.3 или п.5

## 7. Если ругается на CRLF в shell-скриптах

Проверить `.gitattributes`:

```text
*.sh text eol=lf
```

И пересохранить `scripts/*.sh` в LF (не CRLF), затем снова выполнить сборку.

## 8. Замечания

- Предупреждения Svelte/a11y при `npm run build` допустимы, если итоговый `.ipk` создан.
- Для Keenetic MIPS целевой арх — `mipsel-3.4`.
- Для Filogic 820 целевой арх — `aarch64-3.10`.
- Версия пакета берётся из файла `VERSION`.
- **Как это работает:** Команда сама находит `git.exe` в системе, от него добирается до `bash.exe` из состава Git for Windows, конвертирует текущую папку в Unix‑путь и запускает сборочный скрипт. WSL‑bash не используется.

## 9. Установка IPK на роутер (если файл уже в `/opt/tmp`)

Пример для Filogic 820:
`/opt/tmp/awg-manager_2.6.3_aarch64-3.10-kn.ipk`

Команды на роутере:

```sh
# остановить сервис
/opt/etc/init.d/S99awg-manager stop

# установить/переустановить пакет
opkg install /opt/tmp/awg-manager_2.6.3_aarch64-3.10-kn.ipk --force-reinstall

# запустить сервис
/opt/etc/init.d/S99awg-manager start

# проверить статус
/opt/etc/init.d/S99awg-manager status
```

## 10. Обновление программы на роутере из консоли (без потери данных)

Фронтенд обновляет программу через API `/api/system/update/apply`, которое скачивает IPK из GitHub релизов и устанавливает его через `opkg install`. Данные не теряются, так как конфиги хранятся в `/opt/etc` и `/opt/var`, которые opkg не трогает.

Чтобы обновить вручную из консоли роутера:

1. **Найти URL IPK для вашей архитектуры:**
   - Перейдите на https://github.com/hoaxisr/awg-manager/releases
   - Скачайте подходящий `.ipk` файл (например, `awg-manager_2.8.3_mipsel-3.4-kn.ipk` для MIPS Keenetic или `awg-manager_2.8.3_aarch64-3.10-kn.ipk` для ARM64 Filogic).

2. **Скопировать IPK на роутер:**
   - Используйте `scp` или загрузите по HTTP в `/opt/tmp/`.

3. **Команды обновления на роутере:**
   ```sh
   # Остановить сервис (рекомендуется)
   /opt/etc/init.d/S99awg-manager stop

   # Установить новый IPK (автоматически обновит существующий пакет)
   opkg install /opt/tmp/awg-manager_2.8.3_mipsel-3.4-kn.ipk

   # Запустить сервис
   /opt/etc/init.d/S99awg-manager start

   # Проверить статус
   /opt/etc/init.d/S99awg-manager status

   # Очистить временный файл
   rm /opt/tmp/awg-manager_2.8.3_mipsel-3.4-kn.ipk
   ```

**Примечания:**
- Сервис перезапускается автоматически после установки пакета.
- Если обновление прервётся, данные останутся нетронутыми.
- Для автоматического обновления используйте фронтенд (кнопка "Обновить").
- Версия берётся из файла `VERSION` в репозитории.

---

## 11. Проверки и тесты backend на Win11 (правильный Linux-рантайм)

### Приоритет запуска (обязательно)

Для backend-тестов в этом репозитории **всегда сначала использовать**:

```powershell
scripts\dev\dev-backend-tests.bat
```

И только если этот скрипт недоступен/сломался локально — использовать `docker run ... go test ...` как fallback.

Порядок приоритета:

1. `scripts\dev\dev-backend-tests.bat` (основной путь, приоритетный)
2. `docker run ... go test ...` (запасной путь)

### Зачем это нужно

На Win11 часть backend-тестов (особенно под Linux/Keenetic) может давать ложные падения, если запускать их:
- напрямую через `go test` в PowerShell (Windows-бинарь `go.exe` пытается выполнить Linux test binary),
- через WSL без установленного Go,
- через Git Bash с `bash -lc` (login-shell может сломать `PATH` внутри контейнера).

Надёжный способ в проекте: запускать тесты через `scripts\dev\dev-backend-tests.bat`, который уже обслуживает правильное окружение.  
Прямой `docker run` — только запасной вариант.

### Рекомендуемые команды (через project script)

Из корня репозитория:

```powershell
scripts\dev\dev-backend-tests.bat status
scripts\dev\dev-backend-tests.bat start
scripts\dev\dev-backend-tests.bat run ./internal/orchestrator
scripts\dev\dev-backend-tests.bat full
scripts\dev\dev-backend-tests.bat stop
```

### Fallback: базовая команда Docker (только если script недоступен)

Из корня репозитория:

```powershell
docker run --rm -v "$(Get-Location):/src" -w /src golang:1.24-bullseye bash -c 'go test ./internal/orchestrator'
```

### Важно для Codex/песочницы

Если тесты запускаются из Codex-агента с sandbox-ограничениями, Docker-команды нужно выполнять **с выходом из песочницы** (escalated permissions).  
Иначе возможна ошибка доступа к Docker daemon:

```text
permission denied while trying to connect to the docker API at npipe:////./pipe/docker_engine
```

Ожидаемый результат:

```text
ok  	github.com/hoaxisr/awg-manager/internal/orchestrator	0.0xxs
```

### Fallback: точечный запуск одного теста

```powershell
docker run --rm -v "$(Get-Location):/src" -w /src golang:1.24-bullseye bash -c 'go test ./internal/orchestrator -run TestDecide_Reconnect_ASCSoftRestart_MonitoringRestartedOnce'
```

### Важный нюанс: `bash -c`, не `bash -lc`

Использовать нужно `bash -c`.  
`bash -lc` в этом окружении может обнулить/урезать `PATH`, и тогда даже в образе `golang:*` появляется ошибка:

```text
bash: line 1: go: command not found
```

### Быстрый self-check окружения контейнера

Если сомневаетесь, что Go виден:

```powershell
docker run --rm -v "$(Get-Location):/src" -w /src golang:1.24-bullseye bash -c 'command -v go && go version'
```

Ожидается:
- путь `/usr/local/go/bin/go`
- версия `go1.24.x linux/amd64`

### Типовые ошибки и что они значат

1. `%1 is not a valid Win32 application`  
Причина: Windows `go.exe` собрал Linux test binary и пытается запустить его в Windows.

2. `go: command not found` в WSL  
Причина: в конкретном WSL-дистрибутиве не установлен Go.

3. `go: command not found` в `golang:*` контейнере  
Обычно это следствие `bash -lc` (сломанный `PATH`), переключиться на `bash -c`.

4. `fatal error - couldn't create signal pipe, Win32 error 5`  
Это проблема запуска Git Bash/прав, не проблема кода теста.

### Практический вывод для проекта

- Сборка IPK остаётся через Git Bash (как в `scripts/build-all-ipk.bat`).
- На **Windows 11** (локальная разработка) backend-тесты под Linux/Keenetic выполняем **только** через Docker `golang:*` + `bash -c` (или через `dev-backend-tests.bat`).
- На **нативном Linux**, в WSL с установленным Go и в CI — можно и нужно использовать обычный `go test ./...` напрямую (это эталон).
- Автоматизация на Windows — через `scripts\dev\dev-backend-tests.bat` (постоянный контейнер + кэши).

### Подход к запуску backend-тестов (обязательно)

Полный прогон `go test ./...` в Docker — дорогая операция (обычно 5+ минут).  
Чтобы не терять время и не перегружать цикл отладки, используем строгий порядок:

1. Сначала **точечные тесты** только по изменённым пакетам/файлам.
2. Если точечные тесты падают — **фикс и повтор только точечных** тестов.
3. Полный `go test ./...` запускать **только на финише**, когда точечные тесты уже зелёные.
4. Если упал полный прогон:
   - не гонять его по кругу;
   - выделить проблемный пакет/тест;
   - отлаживать его точечно;
   - после фикса снова один финальный полный прогон.

Рекомендуемая последовательность:

```powershell
# 1) Точечно (пример)
docker run --rm -v "$(Get-Location):/src" -w /src golang:1.24-bullseye bash -c 'go test ./internal/managed ./internal/api'

# 2) Отдельно упавший пакет/тест (пример)
docker run --rm -v "$(Get-Location):/src" -w /src golang:1.24-bullseye bash -c 'go test ./internal/sys/httpdownload -run TestReader_EmitsAfterByteThreshold'

# 3) Только в конце — полный прогон
docker run --rm -v "$(Get-Location):/src" -w /src golang:1.24-bullseye bash -c 'go test ./...'
```

### Более прогрессивный путь запуска backend-тестов

Теперь в проекте есть ускоренный раннер:

```powershell
scripts\dev\dev-backend-tests.bat
```

Что он делает:
- держит постоянный Docker-контейнер для тестов (без пересоздания на каждый запуск);
- использует постоянные кэши `GOCACHE` и `GOMODCACHE`;
- поддерживает точечные и полные прогоны в едином формате;
- по умолчанию запускает тесты с `-count=1`, чтобы не получать ложноположительные результаты из test-cache.
- Если запустить без аргументов, он сразу выполняет `full` (`go test -count=1 ./...`).

Базовые команды:

```powershell
# поднять раннер (один раз на сессию)
scripts\dev\dev-backend-tests.bat start

# статус раннера
scripts\dev\dev-backend-tests.bat status

# открыть интерактивную bash внутри раннера (для ручной отладки, go list, ps и т.д.)
scripts\dev\dev-backend-tests.bat shell

# точечный прогон пакета
scripts\dev\dev-backend-tests.bat run ./internal/managed

# точечный прогон одного теста
scripts\dev\dev-backend-tests.bat run ./internal/sys/httpdownload -run TestReader_EmitsAfterByteThreshold

# полный прогон (запускается и по умолчанию без аргументов)
scripts\dev\dev-backend-tests.bat full

# остановить раннер после работы
scripts\dev\dev-backend-tests.bat stop
```

### Coverage baseline через dev-runner

Для локального baseline покрытия backend используйте отдельную команду раннера:

```powershell
scripts\dev\dev-backend-tests.bat coverage
```

Что делает команда:
- запускает backend-тесты с `-count=1`;
- собирает профиль покрытия `coverage.out`;
- генерирует сводку `go tool cover -func` в `coverage.txt`;
- генерирует HTML-отчёт `coverage.html` для визуального просмотра.

Быстрая проверка после прогона:

```powershell
Test-Path coverage.out
Test-Path coverage.txt
Test-Path coverage.html
Get-Content coverage.txt | Select-String "total:"
```

Рекомендуемый workflow:
1. Во время отладки использовать `run` только по изменённым пакетам/тестам.
2. `full` запускать один раз в конце, когда точечные прогоны уже зелёные.
3. Если `full` упал — снова вернуться к точечному `run`, исправить, и только потом повторить `full`.

### Тесты в CI (GitHub Actions)

В настоящем Linux-окружении (ubuntu-latest в GitHub Actions) тесты выполняются **нативно** (без Docker):

- Backend: `go test ./...`
- Frontend: `cd frontend && npm ci && npx vitest run`

См. job `test` в `.github/workflows/build.yml`.

CI — это эталонная проверка для всего, что зависит от Linux/Keenetic (ndms, iptables, sing-box, ASC и т.д.). Прогоняется автоматически на каждый push и PR.

---

## 12. Swagger, mock-server и правило для новых endpoint

Важно: в проекте OpenAPI-спека **автогенерируется из Go-аннотаций**.
Источник автогена:

- `cmd/awg-manager/docs.go`
- директива: `//go:generate ... swag ... -o ../../internal/openapi --ot yaml`

Это означает:

1. Для каждого нового endpoint (или изменения старого) в `internal/api/*.go` нужно обновлять swagger-аннотации (`@Summary`, `@Tags`, `@Param`, `@Success`, `@Failure`, `@Router` и т.д.).
2. В конце работы по фиче обязательно запускать:

```powershell
go generate ./cmd/awg-manager
```

3. Если изменился файл `internal/openapi/swagger.yaml`, его нужно включать в коммит вместе с кодом endpoint.

Практическое правило:

- Не редактировать `internal/openapi/swagger.yaml` руками.
- Если swagger «поправлен вручную», это ошибка процесса: правильный источник истины — Go-аннотации в `internal/api`.

Почему это критично:

- Mock-server/UI docs опираются на актуальную OpenAPI-спеку.
- Если аннотации не обновлены и `go generate` не прогнан, новые поля/эндпоинты (например, данные роутера в Настройках) не мокируются корректно.

### Mock KeeneticOS и extended ASC (frontend mock-proxy)

Локальный `frontend/scripts/mock-proxy.mjs` (порт **8081**) подменяет часть ответов поверх Prism. Для ASC важно, какую версию KeeneticOS «видит» UI:

| Профиль | `supportsExtendedASC` | Поля ASC в mock |
|---------|----------------------|-----------------|
| **5.1** (дефолт) | `true` | Jc–H4, S1–S4, I1–I5 |
| **5.0** | `false` | только Jc–H4, S1–S2 |

По умолчанию mock стартует с **KeeneticOS 5.1** (extended ASC включён).

**Зафиксировать версию при запуске** (до старта mock-proxy / `yarn dev`):

```bash
MOCK_KEENETIC_OS=5.0 yarn dev   # базовый ASC (9 полей)
MOCK_KEENETIC_OS=5.1 yarn dev   # extended ASC (16 полей), то же что дефолт
```

**Переключить в runtime** (mock-proxy уже запущен):

```bash
# Явно 5.0 или 5.1
curl -X POST http://127.0.0.1:8081/__mock/keenetic-os \
  -H 'Content-Type: application/json' -d '{"version":"5.0"}'

curl -X POST http://127.0.0.1:8081/__mock/keenetic-os \
  -H 'Content-Type: application/json' -d '{"version":"5.1"}'

# Сброс к дефолту (5.1 или значение из MOCK_KEENETIC_OS, если было задано при старте)
curl -X POST http://127.0.0.1:8081/__mock/keenetic-os
```

Текущее состояние также видно в `GET /__mock/capabilities` → `state.keeneticOS`, `state.supportsExtendedASC`.

`POST /__mock/reset-runtime` сбрасывает runtime-фикстуры и возвращает KeeneticOS к дефолту.

Затронутые mock-endpoint'ы ASC:

- `GET/PUT /managed-servers/{id}/asc`
- `GET/PUT /system-tunnels/asc?name=...`
- `GET /system/info` (`supportsExtendedASC`, `supportsHRanges`, `firmwareVersion`)

---

## 13. Git safety (RO по умолчанию)

Правило безопасности для работы ИИ-агента:

- По умолчанию Git используется **только в read-only режиме**.
- Любые изменяющие Git-действия (`commit`, `push`, `merge`, `rebase`, `reset`, `checkout -b`, удаление веток, правка истории) выполнять **только после прямого явного запроса пользователя**.

Разрешено без отдельного запроса:

- `git status`, `git log`, `git show`, `git diff`, `git branch --show-current`, `git remote -v` и другие команды чтения.

Запрещено без отдельного запроса:

- Любые команды, меняющие рабочее дерево, индекс, коммиты, ветки или удалённый репозиторий.

---

## 14. `git diff` и почему файл может быть `0 байт`

Если выполнить:

```powershell
git diff > diff.md
```

и `diff.md` получился `0 байт`, это обычно означает:

- нет **unstaged** изменений (рабочее дерево чистое), или
- все изменения уже добавлены в индекс (`staged`), и `git diff` их не показывает.

Полезные команды:

```powershell
# 1) Только unstaged изменения (рабочее дерево)
git diff > diff_unstaged.md

# 2) Только staged изменения (индекс)
git diff --cached > diff_staged.md

# 3) Все изменения относительно последнего коммита (unstaged + staged)
git diff HEAD > diff_all.md

# 4) Быстрая проверка текущего состояния
git status
```

Практика для проекта:

- если нужен «полный дифф текущей работы» — используйте `git diff HEAD > diff_all.md`;
- если нужен дифф «только что ещё не добавлено» — `git diff > diff_unstaged.md`;
- если нужен дифф «что уже в staged» — `git diff --cached > diff_staged.md`.

---

## 15. Рекомендации по запуску frontend-тестов (Win11/PowerShell)

Все frontend-проверки и тесты запускаются из папки `frontend`.

### Что есть в `frontend/package.json`

Основные команды:

```json
"scripts": {
  "dev": "vite dev",
  "dev:mock": "npm run sync:openapi && VITE_API_STRIP_PREFIX=1 VITE_API_TARGET=http://127.0.0.1:8081 vite dev",
  "dev:mock:proxy": "npm run sync:openapi && node scripts/mock-stack.mjs",
  "mock": "node scripts/mock-api.mjs",
  "mock:dynamic": "node scripts/mock-api.mjs --dynamic",
  "build": "svelte-kit sync && vite build",
  "preview": "vite preview",
  "check": "svelte-kit sync && svelte-check --tsconfig ./tsconfig.json",
  "test": "vitest run"
}
```

### Проверка типов / Svelte / a11y (svelte-check)

```powershell
cd frontend
& "C:\Program Files\nodejs\npm.cmd" run check
```

Запускает `svelte-kit sync && svelte-check --tsconfig ./tsconfig.json`.
Выводит количество ошибок и предупреждений. Цель — 0 errors, 0 warnings.

### Запуск vitest (unit-тесты компонентов и утилит)

```powershell
cd frontend
npm exec -- vitest run
```

- Через `npm run test` это тот же запуск `vitest run`.
- Без параметров — прогоняет **все** тесты (23 файла, 177+ тестов на май 2026).
- Конкретный файл:
  ```powershell
  cd frontend; npm exec -- vitest run src/lib/utils/singboxInlineRules.test.ts
  ```
- Конкретный тест по названию:
  ```powershell
  npm exec -- vitest run ... -t "название теста или часть"
  ```

**Полная проверка фронтенда (перед коммитом / PR):**

1. `npm run check`
2. `npm run test` или `npm exec -- vitest run`

Оба шага должны завершаться успешно (зелёный).

Примечание: `npm run check` и `npm run test` уже есть в `frontend/package.json`; `npm run test` запускает `vitest run`.

---

Вердикт: **отделяться можно, но не через постоянный merge upstream/develop в свой develop**.

То есть это уже не “обычный fork с подтягиванием upstream”, а **свой продуктовый develop + выборочный импорт upstream**.

## Правильная схема

У тебя должны быть 2 remote:

```powershell
git remote -v

git remote add upstream https://github.com/hoaxisr/awg-manager.git
git fetch origin
git fetch upstream
```

Где:

```text
origin   = iqubik/awg-manager-fork
upstream = hoaxisr/awg-manager
```

Твой основной рабочий ствол:

```text
origin/develop
```

Авторский источник:

```text
upstream/develop
```

## Главное правило

**Не делай регулярно:**

```powershell
git merge upstream/develop
```

и не делай:

```powershell
git rebase upstream/develop
```

пока ты уже пошёл своим путём.

Почему: upstream сейчас несёт 44 коммита, там есть `VERSION`, `OpenAPI`, backend, большие UI-переделки. Слепой merge превратит твой develop в полукопию авторского с конфликтами и случайным перетиранием твоих решений.

## Рабочий процесс

### 1. Обновлять ссылки

Перед анализом:

```powershell
git fetch origin
git fetch upstream
git checkout develop
git pull --ff-only origin develop
```

### 2. Смотреть, насколько разошлись

```powershell
git rev-list --left-right --count upstream/develop...origin/develop
```

Покажет:

```text
сколько коммитов только в upstream
сколько коммитов только у тебя
```

Подробно:

```powershell
git log --oneline --left-right --cherry-pick upstream/develop...origin/develop
```

### 3. PR автора анализировать отдельно

Да, **можно продолжать анализировать PR в авторском репо**.

Правильный путь:

```text
PR в hoaxisr/awg-manager
→ читаем diff PR
→ проверяем scope
→ решаем: нужно / не нужно
→ переносим только нужное в свою ветку
```

Не “автор смержил — значит мне тоже надо”.

## Как забирать конкретный PR автора

### Вариант А — если PR ещё открыт

Создаёшь локальную ветку под импорт:

```powershell
git fetch upstream pull/303/head:import/upstream-pr-303
git checkout import/upstream-pr-303
```

Смотришь diff относительно авторского develop:

```powershell
git diff upstream/develop...import/upstream-pr-303 --stat
git diff upstream/develop...import/upstream-pr-303
```

Потом переносишь в свой develop не merge’ом, а через отдельную интеграционную ветку:

```powershell
git checkout develop
git pull --ff-only origin develop
git checkout -b integrate/upstream-pr-303
```

Дальше либо cherry-pick коммитов:

```powershell
git cherry-pick <commit_sha>
```

либо, если PR большой и конфликтный, лучше руками сделать patch по нужным файлам.

Проверка:

```powershell
cd frontend
npm run check
npm run build
```

Потом merge в свой develop через свой PR.

### Вариант Б — если PR уже смержен в авторский develop

Ищешь merge/squash commit в upstream:

```powershell
git log --oneline upstream/develop
```

Потом:

```powershell
git checkout develop
git pull --ff-only origin develop
git checkout -b integrate/upstream-commit-XXXX
git cherry-pick <commit_sha>
```

Если конфликтует — это нормально. Решать конфликт в пользу твоей архитектуры, а не blindly upstream.

## Как находить правки автора без PR

Это отдельный регулярный аудит.

Команда:

```powershell
git log --oneline --cherry-pick origin/develop...upstream/develop
```

Или только upstream-коммиты, которых нет у тебя:

```powershell
git log --oneline origin/develop..upstream/develop
```

Посмотреть файлы:

```powershell
git diff --stat origin/develop...upstream/develop
git diff --name-status origin/develop...upstream/develop
```

По конкретному файлу:

```powershell
git diff origin/develop...upstream/develop -- frontend/src/lib/components/ui/Toggle.svelte
```

## Как я бы вёл твой fork

Структура:

```text
develop                         твой основной продуктовый develop
integrate/upstream-pr-303        временная ветка под перенос PR
audit/upstream-develop           временная ветка для анализа отличий upstream
vendor/upstream-develop          необязательно, read-only зеркало upstream/develop
```

Правило:

```text
upstream/develop никогда напрямую не merge в develop.
Только cherry-pick / patch / ручной перенос после аудита.
```

## Что делать с авторскими PR

Да, продолжать анализировать PR — правильно.

Но pipeline такой:

```text
1. PR автора открыт/смёржен
2. Смотрим diff
3. Проверяем: нужно ли это твоему fork
4. Проверяем файлы риска:
   - backend/**
   - internal/openapi/**
   - VERSION
   - package-lock
   - frontend/src/app.css
   - общие UI компоненты
5. Если scope чистый — cherry-pick
6. Если scope грязный — переносить только нужные hunks руками
7. Коммитить у себя отдельным commit:
   chore(upstream): import PR 303 selectively
   или
   fix(...): port upstream fix for ...
```

## Что делать с upstream/develop

Раз в день/неделю:

```powershell
git fetch upstream
git diff --name-status origin/develop...upstream/develop
git log --oneline origin/develop..upstream/develop
```

Дальше не “догонять всё”, а разбирать по категориям:

```text
A. bugfix нужен тебе → cherry-pick
B. backend/API изменение → отдельно аудитить
C. VERSION/build bump → обычно игнор
D. UI redesign автора → только если совпадает с твоим направлением
E. тесты полезные → можно забирать отдельно
F. конфликтует с твоей архитектурой → не брать
```

## Мини-протокол для тебя

Перед любой интеграцией:

```powershell
git fetch origin
git fetch upstream
git checkout develop
git pull --ff-only origin develop
git checkout -b integrate/upstream-что-то
```

После переноса:

```powershell
git status
git diff --stat
git diff --name-status
cd frontend
npm run check
npm run build
```

Если backend затронут:

```powershell
go test ./...
```

Потом push:

```powershell
git push -u origin integrate/upstream-что-то
```

И PR в **свой** `iqubik/awg-manager-fork:develop`.

## Коротко

Правильно так:

```text
свой develop = главный
upstream/develop = внешний донор
авторские PR = источник патчей
merge upstream/develop = не делать
cherry-pick/patch = делать
периодический diff upstream/develop = делать
```

Текущий статус: **ветки уже diverged, поэтому режим “я просто подтягиваю upstream” закончился. Теперь нужен режим selective upstream import.**
