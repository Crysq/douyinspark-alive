# DoyinSpark 主应用：Go 后端 + 前端静态资源，单进程部署
FROM golang:1.24-bookworm AS build
WORKDIR /src
COPY server/go.mod server/go.sum ./
ARG GOPROXY=https://goproxy.cn,direct
RUN go mod download
COPY server/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/doyinspark .

FROM node:22-bookworm-slim AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata wget \
    && addgroup -S app && adduser -S app -G app
WORKDIR /app
COPY --from=build /out/doyinspark ./doyinspark
COPY --from=web /web/dist ./public
RUN chown -R app:app /app
USER app
ENV PORT=8787 \
    TZ=Asia/Shanghai \
    PUBLIC_DIR=/app/public
EXPOSE 8787
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
  CMD wget -qO- http://127.0.0.1:${PORT}/health || exit 1
CMD ["./doyinspark"]
