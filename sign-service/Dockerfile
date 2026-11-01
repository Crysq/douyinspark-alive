# 签名服务：容器内用 Xvfb 跑 headful Chromium
#
# 为什么不 headless：实测 headless 会被抖音跳到验证码中间页，byted_acrawler 不加载，
# 签名拿不到。headful + 虚拟显示才稳定。
FROM node:22-bookworm-slim

ENV DEBIAN_FRONTEND=noninteractive \
    TZ=Asia/Shanghai \
    CHROME_PATH=/usr/bin/chromium \
    HEADLESS=false \
    DISPLAY=:99 \
    PROFILE_DIR=/app/profiles \
    PORT=9100

RUN apt-get update && apt-get install -y --no-install-recommends \
      chromium xvfb x11-utils xauth dumb-init \
      fonts-noto-cjk fonts-noto-color-emoji ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm ci --omit=dev --no-audit --no-fund && npm cache clean --force
COPY server.mjs ./
RUN mkdir -p /app/profiles

HEALTHCHECK --interval=30s --timeout=5s --start-period=40s --retries=3 \
  CMD node -e "fetch('http://127.0.0.1:'+(process.env.PORT||9100)+'/health').then(r=>process.exit(r.ok?0:1)).catch(()=>process.exit(1))"

EXPOSE 9100
ENTRYPOINT ["dumb-init", "--"]
CMD ["xvfb-run", "-a", "-s", "-screen 0 1400x900x24", "node", "server.mjs"]
