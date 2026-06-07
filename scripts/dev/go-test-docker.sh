#!/usr/bin/env bash
# macOS/Linux-обёртка для backend Go-тестов в Docker (аналог dev-backend-tests.bat).
#
# Примеры:
#   ./scripts/dev/go-test-docker.sh start
#   ./scripts/dev/go-test-docker.sh run ./internal/managed ./internal/api
#   ./scripts/dev/go-test-docker.sh run ./internal/managed -run TestService_Create
#   ./scripts/dev/go-test-docker.sh full
#   ./scripts/dev/go-test-docker.sh coverage
#   ./scripts/dev/go-test-docker.sh shell
#   ./scripts/dev/go-test-docker.sh stop

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
COMPOSE_FILE="$REPO_ROOT/docker-compose.go-test.yml"
CONTAINER_NAME="awgm-go-test-runner"

compose() {
  docker compose -f "$COMPOSE_FILE" "$@"
}

ensure_runner() {
  if ! docker inspect -f '{{.State.Running}}' "$CONTAINER_NAME" 2>/dev/null | grep -qx true; then
    echo "Runner not started. Starting now..."
    compose up -d runner
  fi
}

run_in_runner() {
  ensure_runner
  compose exec -T runner go test -count=1 "$@"
}

usage() {
  cat <<EOF
Usage: $(basename "$0") <command> [args...]

Commands:
  start     Start persistent Docker runner with mounted Go caches
  stop      Stop/remove persistent runner
  status    Show runner container state
  shell     Open bash shell inside runner
  run       Run targeted tests: go test -count=1 <args>
  full      Run full suite: go test -count=1 ./...
  coverage  Run backend coverage and write coverage.out/txt/html

Examples:
  $0 start
  $0 run ./internal/managed ./internal/api
  $0 run ./internal/managed -run TestService_Create
  $0 full
  $0 coverage
  $0 stop
EOF
}

cmd="${1:-}"
shift || true

case "$cmd" in
  start)
    mkdir -p "$REPO_ROOT/.cache/docker-go-test/go-build" "$REPO_ROOT/.cache/docker-go-test/go-mod"
    compose up -d runner
    ;;
  stop)
    compose down
    ;;
  status)
    docker ps -a --filter "name=$CONTAINER_NAME"
    ;;
  shell)
    ensure_runner
    compose exec runner bash
    ;;
  run)
    if [[ $# -eq 0 ]]; then
      echo "Usage: $0 run <go test packages/args>" >&2
      exit 1
    fi
    run_in_runner "$@"
    ;;
  full)
    run_in_runner ./...
    ;;
  coverage)
    ensure_runner
    compose exec -T runner bash -c '
      set -euo pipefail
      go test -count=1 \
        -covermode=atomic \
        -coverpkg=./internal/...,./cmd/... \
        -coverprofile=coverage.out \
        ./internal/... ./cmd/...
      go tool cover -func=coverage.out | tee coverage.txt
      go tool cover -html=coverage.out -o coverage.html
    '
    ;;
  ""|help|-h|--help)
    usage
    ;;
  *)
    echo "Unknown command: $cmd" >&2
    echo >&2
    usage >&2
    exit 1
    ;;
esac
