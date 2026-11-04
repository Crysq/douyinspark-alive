# 贡献指南

感谢愿意一起完善这个项目。提 PR 前建议先开 Issue 说明改动方向，避免重复劳动。

## 本地开发

```bash
# 依赖：Go 1.23+、Node 20+、Docker
docker compose up -d mysql redis          # 只起基础设施
cd server && cp .env.example .env && go run .
cd web && npm install && npm run dev      # http://localhost:5173
```

签名服务（登录 / 同步 / 发送）需要 Chromium：

```bash
cd sign-service && npm install && npm start
```

## 提交前自检

```bash
cd server && go vet ./... && go test ./...
cd web && npm run typecheck && npm run build
```

## 代码约定

- 后端按 `handler / service / model / signer` 分层，业务逻辑放 `service`
- 数据库变更走 GORM 自动迁移，注意兼容旧数据
- 前端组件用组合式 API（`<script setup lang="ts">`），样式走 `styles/theme.css` 里的设计变量
- 不提交 `.env`、`sign-service/profiles/`、Cookie、手机号等隐私数据

## 提交信息

```
feat: 新功能
fix: 修复
docs: 文档
chore: 构建/依赖
```

## 行为准则

保持友善，聚焦技术问题。涉及账号安全、平台规则的内容请理性讨论。
