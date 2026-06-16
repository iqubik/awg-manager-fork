Вот теперь нормально — есть реальный CI. Разбираю **по фактическому workflow**, а не фантазиям.

---

# 📦 Что у тебя реально сейчас

Ты используешь **один большой pipeline** в:

```text
.github/workflows/build.yml
```

Он включает:

```text
jobs:
- frontend
- version
- build (matrix)
- test
- pre-release
```

👉 Это **полноценный release pipeline**, а не просто CI.

---

# ❗ КЛЮЧЕВАЯ ПРОБЛЕМА

Ты сейчас используешь CI, который:

```text
- тянет release из upstream логики
- публикует pre-release
- зависит от внешнего repo (hoaxisr)
```

### Конкретно:

```bash
gh release download latest
```

```bash
RELEASE_BASE_URL=http://repo.hoaxisr.ru/...
```

👉 Это НЕ твой форк. Это чужая инфраструктура.

---

# ❌ VERDICT ТЕКУЩЕГО CI

```text
НЕ АВТОНОМЕН
ЗАВЯЗАН НА HOAXISR
НЕ КОНТРОЛИРУЕТСЯ ТОБОЙ
```

---

# 🎯 ЦЕЛЬ

Сделать:

```text
1. CI (тесты + сборка) — всегда
2. Release — только если ты хочешь
3. Убрать зависимость от repo.hoaxisr.ru
```

---

# 🔧 ПОЛНОЕ ЗАДАНИЕ АГЕНТУ

---

# 🧩 ЗАДАЧА 1 — разделить CI и release

## Сейчас

```yaml
on:
  push:
    branches: [ develop ]
```

👉 ВСЁ запускается на каждый push

---

## Сделать

### CI:

```yaml
on:
  push:
    branches: [ develop ]
  pull_request:
    branches: [ develop ]
```

---

### Release (pre-release job)

ОГРАНИЧИТЬ:

```yaml
if: github.event_name == 'workflow_dispatch'
```

ИЛИ:

```yaml
if: startsWith(github.ref, 'refs/tags/')
```

---

## 📌 Результат

```text
CI → всегда
Release → только вручную или по тегу
```

---

# 🧩 ЗАДАЧА 2 — УБРАТЬ зависимость от hoaxisr repo

## ❌ Найдено

```yaml
RELEASE_BASE_URL: http://repo.hoaxisr.ru/develop/singbox/...
```



---

## Что это значит

```text
Ты качаешь бинарь sing-box не из своего CI,
а из чужого репозитория
```

---

## ✅ Варианты

### Вариант A (быстро)

ЗАКОММЕНТИТЬ:

```yaml
RELEASE_BASE_URL
```

и всегда билдить:

```bash
./scripts/build-singbox.sh
```

---

### Вариант B (правильно)

```text
использовать GitHub Releases твоего форка
```

(но это отдельная задача)

---

# 🧩 ЗАДАЧА 3 — FIX backend tests

## Сейчас

```bash
go test ./...
```



---

## Исправить

```bash
go test ./internal/... -count=1 \
  -covermode=atomic \
  -coverpkg=./internal/... \
  -coverprofile=coverage.out
```

---

## Почему

```text
./... → гоняет лишнее
internal/... → только твоя логика
```

---

# 🧩 ЗАДАЧА 4 — frontend check отсутствует ❗

## Сейчас есть

```yaml
- run: npx vitest run
```



---

## ❌ НЕТ

```bash
npm run check
```

---

## Добавить ОБЯЗАТЕЛЬНО

```yaml
- name: Check frontend (types + a11y)
  working-directory: frontend
  run: npm run check
```

---

# 🧩 ЗАДАЧА 5 — pre-release ломает независимость

## Сейчас

```bash
gh release download latest
```



---

## Проблема

```text
зависимость от предыдущих release upstream
```

---

## FIX

В pre-release job:

```bash
# УДАЛИТЬ:
gh release download latest
```

---

И заменить на:

```bash
START="${GITHUB_SHA}~1"
```

---

# 🧩 ЗАДАЧА 6 — permissions избыточны

## Сейчас

```yaml
permissions:
  contents: write
```



---

## FIX

### Для CI:

```yaml
permissions:
  contents: read
```

---

### Только для release job:

```yaml
permissions:
  contents: write
```

---

# 🧩 ЗАДАЧА 7 — version job ломает fork-логику

## Сейчас

```bash
gh release download latest
```



---

## Проблема

```text
версия зависит от предыдущих release upstream
```

---

## FIX (минимум)

Заменить:

```bash
REV=1
```

и УДАЛИТЬ весь блок с `gh release download`

---

# 🧩 ЗАДАЧА 8 — оставить cache (это хорошо)

```yaml
actions/cache
```

✔ оставить без изменений

---

# 🧪 ПРИЁМКА

Агент обязан показать:

---

## 1. CI

```text
✔ PR → запускается pipeline
✔ frontend check проходит
✔ backend tests проходят
```

---

## 2. НЕТ

```text
❌ repo.hoaxisr.ru
❌ gh release download
❌ зависимости от upstream
```

---

## 3. Release

```text
✔ не запускается на каждый push
✔ работает только вручную или по тегу
```

---

# 🧾 КОММИТ

```text
ci: decouple fork from upstream release pipeline and fix test/check steps
```

---

# 💬 КОММЕНТ

```text
CI/CD адаптирован под форк:

- отделён CI от release (pre-release больше не запускается на каждый push)
- удалены зависимости от внешнего repo.hoaxisr.ru и gh release download
- backend тесты ограничены internal пакетами
- добавлен обязательный npm run check для frontend
- исправлены permissions (write только для release)

Pipeline теперь полностью автономен и воспроизводим внутри форка.
```

---

# 🧠 КОРОТКО

```text
Сейчас у тебя не CI — у тебя чужой release pipeline.

После правок:
→ будет нормальный автономный CI
```

---

Если хочешь дальше — могу:

* выдать **готовый patch build.yml**
* или сделать **минимальный safe-режим CI (без сборки IPK вообще)**
