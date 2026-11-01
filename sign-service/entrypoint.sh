#!/bin/sh
# Xvfb 虚拟屏 + x11vnc + noVNC，方便人工在浏览器里操作一次真实发送
set -e
Xvfb :99 -screen 0 1400x900x24 -nolisten tcp &
sleep 1
x11vnc -display :99 -nopw -forever -shared -rfbport 5900 -quiet -bg >/dev/null 2>&1 || true
websockify --web=/usr/share/novnc 6080 127.0.0.1:5900 >/dev/null 2>&1 &
exec node server.mjs
