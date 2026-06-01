# PawPrint 爪迹 — 从零重建设计方案

> 版本: v1.0 · 日期: 2026-06-02 · 状态: 待评审
> 本文档为重建项目的实施设计，配套设计文档见 `files/` 目录。

---

## 0. 背景与目标

### 0.1 当前状态

- 工作树中 289 个源文件已被删除（未提交）
- `files/` 目录包含完整的产品规格文档（开发文档、数据模型、测试用例、API 规范、设计稿）
- 开发文档 (PawPrint宠物店SaaS开发文档.md) 是**唯一事实来源**

### 0.2 目标

以 `files/` 中的文档为唯一真相来源，采用**基础设施先行 → 模块逐层叠加**的策略，使用 TDD 方法论从零重建 PawPrint 宠物店经营管理系统。

### 0.3 约束（来自开发文档 §0）

1. 以开发文档 + 配套文件为准，设计稿仅作视觉参考
2. 演示数据可替换，表结构/枚举/状态机/校验规则才是约束
3. 遇到未覆盖细节自行决策并记录于 `DECISIONS.md`
4. 禁止擅自扩大范围
5. 金额一律以"分"（bigint）存储与计算
6. 所有时间存储为 UTC（timestamptz），展示层按门店时区转换
7. 多门店数据隔离是强约束（store-scope 中间件）
8. 软删除（deleted_at）为核心业务表统一策略

---

## 1. 技术栈（强约束，不可替换）

| 层 | 选型 |
|----|------|
| 管理后台前端 | Vue 3 + TypeScript + Vite + Tailwind CSS + Pinia + Vue Router + Axios |
| 顾客端 | 微信小程序原生 |
| 后端 | Go 1.22+ + Gin + GORM + golang-migrate |
| 数据库 | PostgreSQL 15+ |
| 缓存/队列 | Redis 7+ |
| 鉴权 | JWT (access 2h + refresh 720h) |
| 部署 | Docker + Docker Compose（单机私有化） |
| 反向代理 | Nginx + Let's Encrypt |

---

## 2. 项目结构

### 2.1 后端

```
backend/
├── cmd/server/main.go
├── internal/
│   ├── config/            # 配置加载（env + yaml）
│   ├── router/            # Gin 路由注册 + 中间件挂载
│   ├── middleware/        # auth, rbac, store-scope, logger, recovery, ratelimit, idempotency, cors
│   ├── module/
│   │   ├── auth/          # handler.go / service.go / repo.go / dto.go / model.go
│   │   ├── dashboard/
│   │   ├── appointment/
│   │   ├── boarding/
│   │   ├── pet/
│   │   ├── member/
│   │   ├── inventory/
│   │   ├── settlement/
│   │   ├── notification/
│   │   ├── payment/       # 预留：接口+stub 适配器
│   │   └── setting/
│   ├── pkg/
│   │   ├── response/      # 统一响应包络
│   │   ├── errcode/       # 错误码常量 + 映射
│   │   ├── apperr/        # 应用错误类型
│   │   ├── pagination/    # 分页解析与包装
│   │   ├── money/         # 分↔元转换
│   │   ├── validator/     # 自定义校验器
│   │   ├── dbutil/        # 事务 helper
│   │   └── timeutil/      # 时区转换工具
│   └── job/               # 定时任务
├── migrations/            # *.up.sql / *.down.sql
├── api/openapi.yaml
└── docker-compose.yml
```

### 2.2 前端（管理后台）

```
admin/
├── src/
│   ├── api/          # axios 封装 + 各模块接口
│   ├── router/       # 路由 + 权限守卫
│   ├── stores/       # Pinia: auth, store(当前门店)
│   ├── layouts/      # AppShell（侧边栏 + 顶栏）
│   ├── views/        # 每模块一个目录
│   ├── components/   # KPI卡、表格、看板、时间线
│   ├── styles/       # Tailwind + 设计令牌
│   └── utils/        # money, datetime, v-perm 指令
└── tailwind.config.ts
```

### 2.3 模块内部标准结构

每个 module 遵循统一内部结构：

```
module/<name>/
├── model.go           # GORM model + 枚举常量 + status 定义
├── dto.go             # 请求/响应结构体 + 验证标签
├── repo.go            # 数据访问接口 + GORM 实现 (含 store_id 过滤)
├── service.go         # 业务逻辑接口 + 实现 (事务/状态机/校验)
├── handler.go         # Gin handler (参数绑定 → 调 service → 返回响应)
├── service_test.go    # ← TDD 从这里开始 (红)
├── handler_test.go    # HTTP 层集成测试
└── router.go          # 路由注册
```

**依赖方向**: `handler → service (接口) → repo (接口)`，严格单向。

---

## 3. 实施阶段

### Phase 1: 骨架搭建

**目标**: 可运行的空服务 + 数据库 + 测试基础设施

| # | 任务 | 产出 |
|---|------|------|
| 1.1 | go mod init + Vue 3 脚手架 + Docker Compose | 项目骨架 |
| 1.2 | golang-migrate 集成，schema.sql → 迁移文件 | 数据库就绪 |
| 1.3 | `pkg/` 通用包：response, errcode, apperr, pagination, money, validator, timeutil | 基础设施 |
| 1.4 | config 系统：env + yaml 加载 + 必填校验 | 配置管理 |
| 1.5 | 入口 main.go + `/healthz` + `/readyz` | 可运行服务 |
| 1.6 | Makefile + CI workflow + test helper | 开发工具链 |

**TDD 切入点**: `pkg/money`、`pkg/response`、`pkg/pagination`、`config` 全部先写测试。

---

### Phase 2: 认证与安全核心

**目标**: 完整的认证/RBAC/多门店隔离/审计体系，所有安全测试通过后再进入业务模块。

| # | 任务 | 关键测试 |
|---|------|---------|
| 2.1 | JWT 认证：login/refresh/switch-store/logout + 登录锁定 | TC-AUTH-01~04 |
| 2.2 | Auth middleware：解析 JWT → 注入 context | token 无效/过期/缺失 → 401 |
| 2.3 | RBAC 中间件：`module:action` 粒度权限校验 | TC-RBAC-01~04 |
| 2.4 | Store-scope 中间件：`X-Store-Id` 解析 + 门店过滤 + repo 注入 | TC-ISO-01~03 |
| 2.5 | 基础设施中间件：logger, recovery, ratelimit, idempotency, CORS, trace-id | 限流/幂等/panic 恢复 |
| 2.6 | 审计日志：中间件 + 手动埋点，用户管理 CRUD | 敏感操作落库 |

**安全门禁**: 所有 TC-AUTH、TC-RBAC、TC-ISO 用例 100% 通过后，方可进入业务模块。

---

### Phase 3-10: 业务模块按 P0 顺序交付

每个模块遵循同一 TDD 循环：
1. **RED**: 写 `service_test.go`，定义接口行为
2. **GREEN**: 最小实现 `service.go` → `repo.go` → `model.go`
3. **GREEN**: `handler.go` + `handler_test.go`
4. **GREEN**: `router.go` 路由注册
5. **REFACTOR**: 消除重复，提取公共模式

| Phase | 模块 | 关键测试点 | 估计测试数 |
|-------|------|-----------|-----------|
| 3 | **M2 Dashboard** | 日营收/在店宠物/库存预警/门店隔离/时间窗计算 | ~15 |
| 4 | **M3 预约** | 资源冲突检测/状态机 6 态迁移/小程序预约/取消时限 | ~25 |
| 5 | **M4 寄养** | 入住退房/计费算法 ceil/笼位状态机/照护打卡 | ~20 |
| 6 | **M5 宠物** | 档案CRUD/健康记录/体重曲线/年龄实时计算 | ~12 |
| 7 | **M6 会员CRM** | 储值充值消费/积分 earn 规则/等级自动升级/流水一致性 | ~25 |
| 8 | **M7 库存** | 并发扣减行锁/安全库存预警/采购入库/流水一致性对账 | ~18 |
| 9 | **M9 结算** | 多态结算/支付方式/退款红冲/幂等/副作用原子性 | ~20 |
| 10 | **M11 通知** | 触发点落库/模板渲染/渠道跳过/scheduled 任务扫描 | ~12 |

---

## 4. 关键业务规则实现

### 4.1 状态机模式

所有状态迁移使用统一的 map-based 模式：

```go
type TransitionMap map[string][]string

func (m TransitionMap) Validate(from, to string) bool {
    allowed, ok := m[from]
    if !ok { return false }
    for _, s := range allowed { if s == to { return true } }
    return false
}
```

- 预约状态机: `pending → arrived → in_progress → completed` (含 cancel/no_show 分支)
- 寄养状态机: `booked → checked_in → checked_out` (含 cancel 分支)
- 结算状态机: `unpaid → paid → refunded` (含 void 分支)
- 非法迁移返回 **409 + code=3001**

### 4.2 寄养计费

```
nights = ceil((actual_check_out - actual_check_in) / 24h)
total_amount = nights × price_per_night
最少 1 晚, 寄养不参与会员折扣
```

### 4.3 会员等级升级

每次结算 `paid` 后，用 `customers.total_spend` 与 `membership_tiers.min_total_spend` 比较，取满足条件的最高等级。只升不降（`member.allow_downgrade=false`）

### 4.4 积分规则

每消费 1 元得 `tier.points_rate` 积分（向下取整）。仅对实付金额计积分，储值充值不计。

### 4.5 库存并发

出库在事务内 `SELECT ... FOR UPDATE` on inventory row，库存不足返回 **422 + code=3003**。

### 4.6 储值一致性

`customers.wallet_balance` 变更与 `wallet_transactions` 同事务；`balance_after` 为该笔后快照。

---

## 5. API 约定

- Base URL: `/api/v1`
- 统一响应: `{ "code": 0, "message": "ok", "data": { ... }, "trace_id": "..." }`
- 分页: `page`(从1) + `page_size`(默认20, 最大100)，返回 `{ list, total, page, page_size }`
- 幂等: 所有写操作支持 `Idempotency-Key` 头
- 鉴权: `Authorization: Bearer <access>` + `X-Store-Id: <storeId|*>`
- 时间: ISO8601 带时区
- 金额: 字段名以 `_amount`/`_balance` 结尾，单位分（整数）

### 错误码

| code | HTTP | 含义 |
|------|------|------|
| 0 | 200 | 成功 |
| 1001 | 401 | 未认证/Token 失效 |
| 1002 | 403 | 无权限 |
| 1003 | 403 | 跨门店访问被拒 |
| 2001 | 400 | 参数校验失败 |
| 2002 | 404 | 资源不存在 |
| 3001 | 409 | 状态机非法迁移 |
| 3002 | 422 | 资源时段冲突 |
| 3003 | 422 | 库存不足 |
| 3004 | 422 | 储值不足 |
| 4001 | 501 | 支付未启用 |
| 5000 | 500 | 服务器内部错误 |

---

## 6. 测试策略

### 6.1 测试金字塔

```
        ┌─────┐
        │ E2E │  ~7  TC-E2E-01~07 端到端主流程
        ├─────┤
        │ INT │  ~50 handler_test 集成测试 (HTTP 层)
        ├─────┤
        │UNIT │  ~150 service_test 单元测试 (业务逻辑层)
        └─────┘
```

### 6.2 覆盖率要求

- 核心 service ≥70% (开发文档 §15)
- 关键状态机/计费/库存/储值: **100% 分支覆盖**
- 权限矩阵 + 门店隔离: 每个角色 × 关键操作 全覆盖

### 6.3 TDD 纪律

```
RED   → 写失败的测试 (只测接口行为, 不测实现细节)
GREEN → 最小实现让测试通过
REFACTOR → 消除重复, 改进结构, 测试仍绿

每个模块严格按此循环, 不跳步。
```

### 6.4 对账 SQL（自动化断言）

```sql
-- 库存一致性 (应为0行不一致)
SELECT i.product_id FROM inventory i JOIN LATERAL (
  SELECT balance_after FROM stock_transactions s
  WHERE s.store_id=i.store_id AND s.product_id=i.product_id
  ORDER BY created_at DESC, id DESC LIMIT 1) t ON true
WHERE i.quantity <> t.balance_after;

-- 储值一致性 (应为0行)
SELECT c.id FROM customers c JOIN LATERAL (
  SELECT balance_after FROM wallet_transactions w
  WHERE w.customer_id=c.id ORDER BY created_at DESC, id DESC LIMIT 1) t ON true
WHERE c.wallet_balance <> t.balance_after;
```

---

## 7. 设计令牌（必须与设计稿一致）

```css
/* 颜色 */
--canvas: #F4EEE2; --surface: #FFFDF8; --ink: #231E18;
--coral: #E26B41; --coral-d: #C9542C;     /* 主色 */
--pine: #2A5C4D;  --honey: #D99A28;       /* 辅助 */
--sky: #5A83A6;   --berry: #BC5A78;
--sidebar: #211B14;

/* 状态色映射 */
进行中/主操作=coral; 已完成/健康/在住=pine;
待办/寄养=honey; 线上/退房=sky; 提醒/风险=berry;

/* 字体 */
品牌/拉丁: Bricolage Grotesque;
中文标题: Noto Serif CJK SC;
中文正文: Noto Sans CJK SC;
数字/金额: Geist Mono;

/* 圆角 */ 10/14/20px
/* 间距基准 */ 18px 网格
```

---

## 8. 验收标准 (DoD)

每个模块完成须同时满足：
1. 接口符合 openapi 且通过对应测试用例
2. 权限矩阵生效（越权返回 403）
3. 门店隔离生效
4. 关键写操作有审计/流水
5. 单元测试覆盖核心 service ≥70%，关键状态机/计费/库存/储值 100% 覆盖分支
6. 演示数据可一键导入并跑通主流程

### 端到端主流程验收

1. 登录→切门店→Dashboard 数据与 seed 对账
2. 建预约（冲突拦截）→到店→开始→完成→结算→库存/积分/累计消费/等级变化→小票
3. 寄养入住→照护→退房计费（晚数/金额）→结算→笼位转清洁
4. 充值→钱包支付→余额/流水一致
5. 采购入库→销售至安全库存下→预警
6. 小程序自助预约→后台可见→通知落库
7. 越权 + 跨门店访问全部被正确拒绝（403/1002/1003）

---

## 9. 里程碑

### 第一阶段 (骨架 + 安全)
- 项目骨架 + 数据库迁移 + 通用包
- 认证 + RBAC + 门店隔离 + 审计
- 所有安全测试通过

### 第二阶段 (核心业务 P0)
- M2 Dashboard → M3 预约 → M4 寄养 → M5 宠物
- M6 会员CRM → M7 库存 → M9 结算 → M11 通知

### 第三阶段 (完善)
- P1 模块: M8 数据分析, M9 财务日结, M10 系统设置
- 短信 + 公众号真实发送
- 支付适配器接入
- 前端管理后台完整
- 微信小程序
- E2E 全流程自动化

---

## 10. 备注

- 开发中如遇文档未覆盖细节，按 §0.3 自行决策并记录于 `DECISIONS.md`
- 不得新增文档未列出的业务模块
- 不得将"预留"标记的能力做成真实实现
- 所有金额使用 bigint 分，时间使用 timestamptz UTC
