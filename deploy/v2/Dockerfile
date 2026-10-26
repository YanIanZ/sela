# Sela v2: one Go process serving /api/v2 and the static frontend.
# Build from the repository root:
#   docker build -f deploy/v2/Dockerfile --build-arg VERSION=$(git rev-parse --short HEAD) -t sela:v2 .
# Run (AI disabled until a Modal endpoint is configured, see docs/RUNBOOK.md):
#   docker run --rm -p 8080:8080 -e SELA_SECURE_COOKIES=false sela:v2

FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build

FROM golang:1.24.7-alpine AS go
ARG VERSION=dev
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
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
