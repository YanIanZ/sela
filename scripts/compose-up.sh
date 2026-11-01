#!/usr/bin/env bash
# Build and start the docker compose stack with the checked-out commit as the
# build version, so /api/v2/health reports the deployed commit (S-001).
# Extra arguments go to `docker compose up` (for example a service name).
set -euo pipefail
cd "$(dirname "$0")/.."
SELA_VERSION="$(git describe --always --dirty)"
export SELA_VERSION
echo "Building sela ${SELA_VERSION}"
docker compose up -d --build "$@"
