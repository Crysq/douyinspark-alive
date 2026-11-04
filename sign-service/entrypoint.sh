#!/bin/sh
# 只起虚拟屏 + 签名服务：不开放 VNC，账号登录由控制台完成
set -e
Xvfb :99 -screen 0 1400x900x24 -nolisten tcp &
sleep 1
exec node server.mjs
