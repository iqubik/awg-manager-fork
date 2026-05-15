# Contributing

## Структура проекта

```
awg-manager/
├── cmd/awg-manager/       # точка входа бекенда, docs.go (swag директивы)
├── internal/
│   ├── api/               # HTTP-хендлеры — здесь живут OpenAPI аннотации
│   ├── openapi/           # embed.go + swagger.yaml (АВТОГЕНЕРАТ — не редактировать руками)
│   └── ...                # остальная бизнес-логика
├── frontend/
│   ├── src/               # SvelteKit-приложение
│   ├── scripts/           # mock-сервер, прокси, генераторы иконок
│   └── static/            # openapi.yaml сюда копируется при dev:mock (gitignored)
├── scripts/               # сборочные shell-скрипты
└── openapi.md             # подробный гайд по OpenAPI/Swagger
```

## Окружение разработки

### Бекенд

Требуется Go 1.23. Для запуска бекенда локально — стандартный `go run`:

```bash
go run ./cmd/awg-manager
```

Бекенд слушает порт **8080**.

### Фронтенд

```bash
cd frontend
npm install
npm run dev        # dev-сервер с проксированием на бекенд (порт 8080)
```

Swagger UI доступен по `/dev/api-docs` при запущенном бекенде и dev-сервере.

Если хочется работать с реальным устройством без локального бекенда, можно натравить dev-сервер прямо на роутер — достаточно указать его адрес через переменную окружения:

```bash
cd frontend
VITE_API_TARGET=http://192.168.1.1 npm run dev
```

Все запросы `/api/*` будут проксироваться на роутер, а фронтенд — подхватывать изменения на лету.

## OpenAPI / Swagger

> Подробный гайд — в [`openapi.md`](./openapi.md)

### Главное правило

**Swagger-файл генерируется автоматически из Go-аннотаций. Не редактируй `internal/openapi/swagger.yaml` руками** — изменения будут перезаписаны при следующем `go generate`.

Если ИИ-инструмент предлагает напрямую отредактировать `swagger.yaml` — отклоняй: файл строго автогенерат, правильный путь — аннотации в хендлерах.

### Что делать при добавлении или изменении API

1. Добавляй/обновляй swagger-аннотации над хендлером в `internal/api/*`:

```go
// GetFoo godoc
// @Summary      Краткое описание
// @Tags         foo
// @Produce      json
// @Security     CookieAuth
// @Success      200 {object} FooResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /foo [get]
func (h *FooHandler) GetFoo(w http.ResponseWriter, r *http.Request) { ... }
```

2. В конце работы над фичей (перед коммитом) — пересобирай спеку:

```bash
# В корне репозитория
go generate ./cmd/awg-manager
```

Команда перезаписывает `internal/openapi/swagger.yaml`. Коммить обновлённый файл вместе с изменениями хендлеров.

3. Без аннотаций новый эндпоинт **не попадёт в мок-сервер** и фронтенд не сможет с ним работать в dev:mock режиме.

## Mock-сервер

Для разработки фронтенда без запущенного бекенда используется Prism, который поднимает мок-сервер на основе OpenAPI-спеки.

```bash
cd frontend
npm run dev:mock   # синхронизирует swagger.yaml и запускает Vite + Prism
```

### Важный момент: мок работает только по спеке

Prism отдаёт ответы строго по тому, что описано в `swagger.yaml`. Если эндпоинт не аннотирован или аннотации устарели — он либо не замокается вообще, либо вернёт неверную схему.

**Пример из практики:** фича с параметрами кинетика не мокалась в настройках именно потому, что аннотации отсутствовали. Не повторяй эту ошибку — аннотируй всё, что добавляешь.

### Stateful mock proxy

Если нужен stateful-мок (например, сохранение состояния между запросами), используй `mock-proxy.mjs`:

```bash
# Терминал A — Prism
cd frontend && npm run mock

# Терминал B — stateful прокси
node frontend/scripts/mock-proxy.mjs

# Терминал C — Vite через прокси
cd frontend && VITE_API_TARGET=http://127.0.0.1:8081 npm run dev:mock
```

Подробнее — в [`openapi.md`](./openapi.md#6-stateful-mock-proxy-state-aware-overrides).

## Процесс работы над фичей

1. Реализуй хендлеры в `internal/api/`.
2. Добавь swagger-аннотации ко всем новым и изменённым эндпоинтам, а так же типизируй DTO.
3. Запусти `go generate ./cmd/awg-manager` — убедись, что спека обновилась без ошибок.
4. Проверь фронт в dev:mock режиме — убедись, что мок работает корректно.
5. Закоммить `internal/openapi/swagger.yaml` вместе с остальными изменениями.

## Сборка

IPK-пакет для Entware (Keenetic):

```bash
./scripts/build-ipk.sh [VERSION] [ARCH]
# Поддерживаемые архитектуры: mipsel-3.4, mips-3.4, aarch64-3.10
```

Только бекенд (кросс-компиляция):

```bash
./scripts/build-backend.sh
```

Только фронтенд:

```bash
./scripts/build-frontend.sh
```

## Pull Requests

- Ветки от `develop`, PR — в `develop`.
- В репозитории используется **fast-forward merge** — перед мержем нужно отребейзить ветку на актуальный `develop`:

```bash
git fetch origin
git rebase origin/develop
```

- В описании PR кратко опиши что изменилось и зачем.
- Если добавлял/менял API — убедись, что `swagger.yaml` обновлён и закоммичен.
- Старайся не смешивать рефакторинг и новую функциональность в одном PR.

## Conventional Commits

Проект придерживается [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/).

Формат: `<type>(<scope>): <description>`

Примеры:

```
feat(api): add kinetic parameters endpoint
fix(singbox): correct config merge on update
chore: run go generate, update swagger.yaml
refactor(tunnel): extract WAN detection logic
docs: add CONTRIBUTING.md
```

Распространённые типы: `feat`, `fix`, `refactor`, `chore`, `docs`, `test`, `perf`, `ci`.

**Требование к PR:** выполни одно из двух условий:

- **Все коммиты в ветке** соответствуют конвенции — тогда мержится as-is.
- **Название PR** соответствует конвенции — тогда ставишь squash и итоговый коммит будет по конвенции.
