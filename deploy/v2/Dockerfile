# Sela: one Go process serving /api/v2, /api/v3 and the static frontend.
# Build from the repository root:
#   docker build -f deploy/v2/Dockerfile --build-arg VERSION=$(git rev-parse --short HEAD) -t sela:v2 .
# The image defaults to SELA_ENV=production, which refuses to start without
# SELA_DATABASE_URL, a prod- KEK (SELA_DATA_KEK, SELA_DATA_KEK_ID) and an https
# SELA_PUBLIC_ORIGIN. Run migrations first with: docker run ... sela:v2 migrate up
# (see docs/operations/RUNBOOK.md). AI stays disabled until a Modal endpoint is configured.

FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build

FROM golang:1.26.10-alpine AS go
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY data/content ./data/content
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/sela ./cmd/sela

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=go /out/sela /sela
COPY --from=web /src/web/dist /web
COPY data/research/corpus/knowledge_base.jsonl data/research/corpus/manifest.json /data/research/corpus/
ENV SELA_ENV=production \
    SELA_ADDR=:8080 \
    SELA_WEB_DIR=/web \
    SELA_RESEARCH_KB=/data/research/corpus/knowledge_base.jsonl \
    SELA_LLM_MODE=disabled
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/sela"]
