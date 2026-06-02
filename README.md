# PawPrint 爪迹

PawPrint 是一个面向宠物店的经营管理 SaaS 重建项目。当前仓库以 `files/` 中的产品文档、数据模型、API 规范和测试用例为事实来源，按“后端基础设施 + 安全核心 + 业务模块 + 管理后台”的方式逐步实现。

## 当前代码概览

项目采用前后端分离结构：

```text
.
├── admin/                  # Vue 3 + TypeScript 管理后台
├── backend/                # Go + Gin + GORM 后端服务
├── docs/superpowers/       # 重建设计与阶段实施计划
└── files/                  # 产品规格、OpenAPI、数据库 schema、seed、设计稿
```

后端当前实现了基础服务入口、配置加载、统一响应、错误码、分页、金额和时间工具包，以及认证、RBAC、多门店隔离、限流、幂等、CORS、日志、恢复和 trace id 等中间件。

业务模块位于 `backend/internal/module/`，每个模块基本遵循：

```text
model.go -> repo.go -> service.go -> handler.go -> router.go
```

已存在模块包括：

- `auth`: 登录、刷新 token、切换门店、登出
- `dashboard`: 门店经营概览
- `appointment`: 预约列表、创建、详情、状态流转、取消、可用时段
- `boarding`: 寄养入住、退房、照护记录
- `pet`: 宠物档案、健康记录、体重记录
- `member`: 会员列表、会员详情、储值充值和调整
- `inventory`: 销售出库、采购入库、库存调整、库存预警
- `settlement`: 结算单创建、支付、退款、作废
- `notification`: 通知发送入口
- `analytics`: 数据分析报表
- `setting`: 系统设置读取和更新

管理后台位于 `admin/`，使用 Vue 3、Vite、Pinia、Vue Router、Axios 和 Tailwind CSS。当前页面覆盖登录、仪表盘、预约、寄养、宠物、会员、库存、结算、数据分析和设置。

## 技术栈

后端：

- Go module: `pawprint/backend`
- Gin
- GORM
- PostgreSQL 15+
- Redis 7+
- JWT: access token + refresh token
- Docker / Docker Compose

前端：

- Vue 3
- TypeScript
- Vite
- Pinia
- Vue Router
- Axios
- Tailwind CSS

## 本地运行

### 1. 启动数据库和 Redis

```bash
cd backend
docker compose up -d
```

`backend/docker-compose.yml` 会启动：

- PostgreSQL: `localhost:5432`
- Redis: `localhost:6379`

### 2. 准备后端环境变量

```bash
cd backend
cp .env.example .env
```

默认 `.env.example`：

```env
APP_ENV=dev
HTTP_PORT=8080
DB_DSN=postgres://pawprint:pawprint@localhost:5432/pawprint?sslmode=disable
REDIS_ADDR=localhost:6379
JWT_ACCESS_SECRET=change-me-access-secret
JWT_REFRESH_SECRET=change-me-refresh-secret
JWT_ACCESS_TTL=2h
JWT_REFRESH_TTL=720h
DEFAULT_TIMEZONE=Asia/Shanghai
```

生产或共享环境中必须替换 `JWT_ACCESS_SECRET` 和 `JWT_REFRESH_SECRET`。

### 3. 初始化数据库

当前仓库提供 SQL 迁移文件，但没有封装迁移 CLI。可以直接用 `psql` 执行：

```bash
export DB_DSN='postgres://pawprint:pawprint@localhost:5432/pawprint?sslmode=disable'
psql "$DB_DSN" -f backend/migrations/000001_init_schema.up.sql
psql "$DB_DSN" -f backend/migrations/000002_seed_data.up.sql
```

种子数据中的演示账号密码统一为：

```text
用户名: admin
密码: pawprint123
```

其他演示用户也使用相同密码，例如 `frontdesk`、`finance`。

### 4. 启动后端

```bash
cd backend
set -a
source .env
set +a
make dev
```

服务默认监听：

```text
http://localhost:8080
```

健康检查：

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

### 5. 启动管理后台

```bash
cd admin
npm install
npm run dev
```

前端默认监听：

```text
http://localhost:5173
```

Vite 开发环境会把 `/api` 代理到 `http://localhost:8080`。

## API 约定

API base path:

```text
/api/v1
```

认证：

```text
Authorization: Bearer <access_token>
X-Store-Id: <store_id>
```

统一响应结构：

```json
{
  "code": 0,
  "message": "ok",
  "data": {}
}
```

核心路由：

```text
POST /api/v1/auth/login
POST /api/v1/auth/refresh
POST /api/v1/auth/switch-store
POST /api/v1/auth/logout

GET  /api/v1/dashboard/summary
GET  /api/v1/appointments
POST /api/v1/appointments
GET  /api/v1/appointments/:id
POST /api/v1/appointments/:id/transitions
POST /api/v1/appointments/:id/cancel
GET  /api/v1/appointments/available-slots

GET  /api/v1/boarding-orders
POST /api/v1/boarding-orders/check-in
GET  /api/v1/boarding-orders/:id
POST /api/v1/boarding-orders/:id/check-out
GET  /api/v1/boarding-orders/:id/care-logs
POST /api/v1/boarding-orders/:id/care-logs

POST /api/v1/pets
GET  /api/v1/pets/:id
POST /api/v1/pets/:id/health
POST /api/v1/pets/:id/weights
GET  /api/v1/customers/:id/pets

GET  /api/v1/customers
GET  /api/v1/customers/:id
POST /api/v1/customers/:id/wallet
PUT  /api/v1/customers/:id/wallet

POST /api/v1/inventory/sale-out
POST /api/v1/inventory/purchase-in
POST /api/v1/inventory/adjust
GET  /api/v1/inventory/alerts

GET  /api/v1/settlements
POST /api/v1/settlements
POST /api/v1/settlements/:id/pay
POST /api/v1/settlements/:id/refund
POST /api/v1/settlements/:id/void

POST /api/v1/notifications/send
GET  /api/v1/analytics/report
GET  /api/v1/settings
GET  /api/v1/settings/:key
PUT  /api/v1/settings/:key
```

## 测试和构建

后端：

```bash
cd backend
make test
make lint
make build
```

前端：

```bash
cd admin
npm run build
```

当前常用验证门禁：

```bash
cd backend && go test ./...
cd admin && npm run build
```

## 数据和业务约束

核心业务约束来自 `files/PawPrint宠物店SaaS开发文档.md`、`files/schema.sql` 和 `files/测试用例.md`：

- 金额统一以“分”为单位存储和计算
- 时间统一存储为 UTC，展示层按门店时区转换
- 多门店数据隔离由 `X-Store-Id` 和 store-scope 中间件约束
- 主要业务表使用 `deleted_at` 软删除
- 预约、寄养、结算等模块通过状态机控制合法流转
- 写操作设计上支持幂等键 `Idempotency-Key`

## 资料入口

- 产品开发文档: `files/PawPrint宠物店SaaS开发文档.md`
- OpenAPI: `files/openapi.yaml`
- 数据库 schema: `files/schema.sql`
- 演示数据: `files/seed.sql`
- 测试用例: `files/测试用例.md`
- 重建设计: `docs/superpowers/specs/2026-06-02-pawprint-rebuild-design.md`
- 阶段计划: `docs/superpowers/plans/2026-06-02-pawprint-phase1-2.md`

## 开发注意事项

- 后端业务接口默认需要 JWT 和 `X-Store-Id`。
- `super_admin` 可在 store-scope 中使用 `X-Store-Id: *` 做跨门店查询，但具体 repo 层仍需按业务明确处理。
- 新增后端模块时，优先保持 `handler -> service -> repo` 的依赖方向。
- 新增业务行为时优先补测试，尤其是状态机、金额、库存、储值和门店隔离逻辑。
- `backend/docker-compose.yml` 当前只编排 PostgreSQL 和 Redis，不是完整应用部署编排。
