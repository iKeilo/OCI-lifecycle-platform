FROM node:22-bookworm-slim AS frontend
WORKDIR /src

COPY package.json package-lock.json ./
RUN npm ci

COPY index.html tsconfig.json tsconfig.node.json vite.config.ts ./
COPY src ./src
RUN npm run build

FROM golang:1.26-bookworm AS backend
WORKDIR /src/backend

ARG GO_PROXY
COPY backend/go.mod backend/go.sum ./
RUN if [ -n "$GO_PROXY" ]; then go env -w GOPROXY="$GO_PROXY"; fi && go mod download

COPY backend ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/oci-lifecycle-platform ./cmd/server \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/panel-password ./cmd/panel-password

FROM debian:bookworm-slim AS runtime
WORKDIR /app

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl tzdata \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --system --uid 10001 --home-dir /app --shell /usr/sbin/nologin appuser \
    && mkdir -p /app/www /data \
    && chown -R appuser:appuser /app /data

COPY --from=backend /out/oci-lifecycle-platform /app/oci-lifecycle-platform
COPY --from=backend /out/panel-password /app/panel-password
COPY --from=frontend /src/dist /app/www

ENV PORT=8080
ENV STATIC_DIR=/app/www
ENV PROFILE_STORE_FILE=/data/profiles.json
ENV OCI_EXECUTION_MODE=local
ENV TZ=Asia/Shanghai

VOLUME ["/data"]
EXPOSE 8080

USER appuser
CMD ["/app/oci-lifecycle-platform"]
