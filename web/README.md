# Center Web 管理后台

Center 侧的 Web 管理界面。技术栈：React 18 + TypeScript + Vite 5 + Ant Design 5 + React Router 6。

设计依据：`docs/superpowers/specs/2026-10-12-frontend-admin-design.md`。

## 本地开发

```bash
cd web
npm install
npm run dev          # http://localhost:5173
```

开发服务器把 `/v1` 与 `/health` 代理到 Center。默认目标是 `http://127.0.0.1:8080`，
可用环境变量覆盖：

```bash
VITE_API_PROXY=http://127.0.0.1:8099 npm run dev
```

Center 侧的端口同样可配置（`PORT` 环境变量），两边端口不一致时改这里的代理目标即可，
不要改代码。

## 启动后端

```bash
# 开发库（示例：本机 5433 上的容器）
docker run -d --name silkroad-dev-pg -p 5433:5432 \
  -e POSTGRES_USER=igh -e POSTGRES_PASSWORD=igh_dev_password -e POSTGRES_DB=igh \
  postgres:16-alpine

# 首次启动时 ENV=development 会自动建表
PORT=8099 DATABASE_URL='postgres://igh:igh_dev_password@127.0.0.1:5433/igh?sslmode=disable' \
  go run ./cmd/center-server

# 种子数据（账号：admin/admin123、operator1、inspector1、viewer1，密码均为 password）
DATABASE_URL='postgres://igh:igh_dev_password@127.0.0.1:5433/igh?sslmode=disable' \
  go run ./cmd/seed
```

需要报表页有数据可看时，再灌一次演示数据：

```bash
cat scripts/dev-report-demo.sql | docker exec -i silkroad-dev-pg psql -U igh -d igh
```

## 构建

```bash
npm run build        # 先 tsc -b 再 vite build，产物在 dist/
npm run typecheck    # 只做类型检查
npm run preview      # 预览构建产物
```

## 目录结构

```
src/
  api/           后端接口封装，按资源分文件
    client.ts    axios 实例：注入 token、解包错误信封、401 处理
    auth.ts      登录、当前用户、分页结果转换
    lots.ts      批次
    bobbins.ts   丝锭
    reports.ts   报表聚合
    users.ts     用户
    edges.ts     设备
  auth/          登录态（localStorage）
  layouts/       主布局（侧边导航 + 顶栏）
  pages/         页面
  types/         后端响应封装的类型
```

## 几条不能改错的约定

**聚合值不在前端重算。** 批次的 `progress`、报表的 `qualified_rate` 都由后端计算并下发。
后端已处理除零与三态（NULL 不等于「不合格」）等边界，前端重算必然与后端口径分叉。

**列表的分页信息在顶层 `pagination`，不在 `data` 内。** 这是后端 `PageSuccess` 的形状，
与普通 `ApiResponse` 不同，转换集中在 `api/auth.ts` 的 `toPageResult`。

**401 由请求层统一处理。** `api/client.ts` 在收到 401 时清空会话并派发
`silkroad:unauthorized` 事件，由 `AuthGate` 响应并跳转。不要改成
`window.location.href` —— 那会整页刷新并丢失当前路由。

**只读页面不放写入口。** 批次与丝锭在 Center 是只读的（数据归属单向，Edge 是权威源）。
详情页不出现编辑、删除、状态修正按钮，这不是「未实现」而是设计排除的能力。

**可空分组键必须显式展示。** 报表里的 `operator_id` / `inspector_id` 为 NULL 时渲染为
「未记录」，不是隐藏该行 —— 否则统计数据会静默缺失。

## 已知缺口

| 缺口 | 说明 |
|---|---|
| 用户响应缺 `employee_id` / `last_login_at` | 后端未返回，页面不展示这两列 |
| 设备名称映射 | `GET /v1/edges` 仅管理员可调，其他角色只看得到 UUID 前 8 位 |
| 仓储管理页 | 包装四表在 Center 恒为空，等 Edge 打包环节落地 |
| 质检记录 / 追溯链 | 后端暂无 `bobbin_grades` / `doffings` 读取端点 |
