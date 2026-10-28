#!/usr/bin/env bash
# Rebuild the current checkout and run the configured local v3 environment.
set -euo pipefail
repo_root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$repo_root"
for config in .env.docker-testing .env.pg-testing; do
  if [[ ! -f "$config" ]]; then
    echo "Missing local configuration: $config (see docs/V3_CORE_VALIDATION.md)" >&2
    exit 1
  fi
done
docker info >/dev/null
if ! docker network inspect sela-testing >/dev/null 2>&1; then docker network create sela-testing >/dev/null; fi
if ! docker container inspect sela-postgres-testing >/dev/null 2>&1; then
  docker volume create sela-postgres-testing-data >/dev/null
  docker run -d --name sela-postgres-testing --network sela-testing --env-file .env.pg-testing \
    -v sela-postgres-testing-data:/var/lib/postgresql/data \
    --health-cmd 'pg_isready -U sela -d sela_testing' --health-interval 5s --health-timeout 3s --health-retries 20 postgres:16-alpine >/dev/null
else
  docker start sela-postgres-testing >/dev/null
fi
for attempt in {1..30}; do
  if docker exec sela-postgres-testing pg_isready -U sela -d sela_testing >/dev/null 2>&1; then break; fi
  if [[ "$attempt" == 30 ]]; then echo 'PostgreSQL did not become ready' >&2; exit 1; fi
  sleep 1
done
build_version="$(git describe --always --dirty)"
docker build -f deploy/v2/Dockerfile --build-arg "VERSION=$build_version" -t sela:v3-core .
# The known v2 testing container may own the same local port from an earlier run.
if [[ "$(docker ps --filter name='^sela-v2-testing$' --format '{{.Names}}')" == sela-v2-testing ]]; then docker stop sela-v2-testing >/dev/null; fi
if docker container inspect sela-v3-core-testing >/dev/null 2>&1; then
  docker stop sela-v3-core-testing >/dev/null
  docker rm sela-v3-core-testing >/dev/null
fi
docker run -d --name sela-v3-core-testing --network sela-testing --env-file .env.docker-testing \
  -p 127.0.0.1:8080:8080 sela:v3-core >/dev/null
for attempt in {1..30}; do
  if curl -fsS http://127.0.0.1:8080/api/v2/ready >/dev/null 2>&1; then
    echo "Sela $build_version is ready: http://localhost:8080 (v2 demo: /demo)"
    exit 0
  fi
  sleep 1
done
echo 'Sela did not become ready; inspect docker logs sela-v3-core-testing' >&2
exit 1
