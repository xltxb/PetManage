# 爪迹 PawPrint · 宠物店智慧经营系统 — 开发文档

> 版本：v1.0（开发交付版） · 语言：简体中文（zh-CN） · 货币：人民币（CNY，分为最小存储单位）
> 本文档为本项目**唯一事实来源（Single Source of Truth）**。配套文件：`openapi.yaml`（接口草案）、`schema.sql`（建库 DDL）、`seed.sql`（示例数据）、`测试用例.md`（验收用例）、`api测试集合.http`（接口冒烟）。

---

## 0. 给开发方（AI）的执行约定 —— 必读

本节定义"边界"与"遇到未覆盖情况时的处理方式"，目的是让开发全程**无需再向产品方确认**。

1. **以本文档 + 配套文件为准**。当本文档与设计稿（PNG/PDF）冲突时，**以本文档为准**；设计稿仅作视觉参考。
2. **演示数据可替换**：设计稿与 `seed.sql` 中的宠物名、客户名、价格、房型数量等均为**演示数据**，不是业务约束。表结构、枚举、状态机、校验规则才是约束。
3. **遇到本文档未明确的细节时**，按以下优先级**自行决策并在代码注释 + `DECISIONS.md` 中记录**，不要停下来等待确认：
   - (a) 遵循本文档已给出的同类规则／命名约定／状态机；
   - (b) 选择**最保守、可逆、不丢数据**的实现（如：宁可软删除不可物理删除；宁可拒绝写入不可静默覆盖）；
   - (c) 遵循各技术栈社区主流最佳实践（Go 项目布局、RESTful、PostgreSQL 范式）。
4. **禁止擅自扩大范围**：不得新增本文档未列出的业务模块、不得引入未约定的第三方收费服务、不得把"本期预留不实现"的能力（如真实支付）做成真实调用。
5. **金额一律以"分"（整数 bigint）存储与计算**，仅在展示层除以 100。禁止用浮点存金额。
6. **所有时间存储为 UTC（timestamptz）**，展示层按门店时区（默认 `Asia/Shanghai`）转换。
7. **多门店数据隔离是强约束**：除"超级管理员"外，任何查询/写入都必须带门店维度过滤（见 §5）。任何遗漏门店过滤的接口视为缺陷。
8. **软删除**：核心业务表统一使用 `deleted_at timestamptz NULL` 软删除，不物理删除（审计与财务追溯需要）。
9. 凡标注 **【P0】** 为首期必交付；**【P1】** 为二期；**【预留】** 为本期只定义接口/表结构、不实现真实逻辑。

---

## 1. 产品概述与范围

爪迹 PawPrint 是面向**宠物门店（可含多家分店）**的一体化经营管理系统，私有化部署。包含**管理后台（Web）**与**顾客端（微信小程序）**两个前端。

### 1.1 模块清单与优先级

| # | 模块 | 优先级 | 说明 |
|---|------|--------|------|
| M1 | 认证与账户 | P0 | 登录、RBAC、门店切换 |
| M2 | 经营概览 Dashboard | P0 | 营业额、预约、在店、会员等汇总 |
| M3 | 预约管理 | P0 | 资源排程看板、后台录入 + 小程序自助预约 |
| M4 | 寄养业务 | P0 | 笼位看板、入住/退房、计费结算、照护记录 |
| M5 | 宠物档案 | P0 | 档案、健康记录、体重、消费历史 |
| M6 | 会员客户（CRM） | P0 | 会员、等级、储值、积分、关怀提醒 |
| M7 | 商品库存 | P0 | 商品、库存自动扣减、安全库存预警、采购入库 |
| M8 | 数据分析 | P1 | 营收趋势、服务占比、时段、复访漏斗 |
| M9 | 财务流水 | P1 | 结算单、收款方式、对账、日结 |
| M10 | 系统设置 | P1 | 门店、角色权限、房型价、积分规则、模板、打印机 |
| M11 | 通知中心 | P0(站内)/P1(短信+公众号) | 触发与发送（见 §10） |

> 注：通知中心的"站内提醒"随 P0 一起做；"短信 + 公众号模板消息"实际发送通道随 P1 交付，但 **P0 阶段必须把触发点与 `notification_logs` 落库**，P1 仅补发送适配器。

### 1.2 顾客端（微信小程序）范围

- 顾客登录（微信授权）、绑定手机号、绑定本人/宠物。
- 在线浏览门店与可预约服务、提交自助预约、查看/取消自己的预约。
- 查看本人会员信息（等级、储值余额、积分）、消费记录。
- 接收公众号模板消息（疫苗到期/到店提醒）。
- **不含**：小程序内支付（预留）、商品商城下单（不在本期范围）。

---

## 2. 技术栈与项目结构

### 2.1 技术栈（强约束，不可替换）

| 层 | 选型 |
|----|------|
| 管理后台前端 | Vue 3 + TypeScript + Vite + Tailwind CSS + Pinia（状态）+ Vue Router + Axios |
| 顾客端 | 微信小程序原生（或 uni-app，二选一由开发方决定并记录于 DECISIONS.md，默认**微信小程序原生**） |
| 后端 | Go 1.22+ + Gin + GORM（ORM）+ golang-migrate（迁移） |
| 数据库 | PostgreSQL 15+ |
| 缓存/队列 | Redis 7+（会话、限流、通知异步队列、库存并发锁） |
| 鉴权 | JWT（access + refresh），见 §9.2 |
| 部署 | Docker + Docker Compose（单机私有化） |
| 反向代理/HTTPS | Nginx + Let's Encrypt（或商户自有证书） |

### 2.2 后端目录结构（建议）

```
backend/
├── cmd/server/main.go
├── internal/
│   ├── config/            # 配置加载（env + yaml）
│   ├── router/            # gin 路由注册
│   ├── middleware/        # auth, rbac, store-scope, logger, recovery, ratelimit
│   ├── module/
│   │   ├── auth/
│   │   ├── store/
│   │   ├── dashboard/
│   │   ├── appointment/
│   │   ├── boarding/
│   │   ├── pet/
│   │   ├── member/
│   │   ├── inventory/
│   │   ├── settlement/    # 结算/财务
│   │   ├── analytics/
│   │   ├── notification/
│   │   ├── payment/       # 预留：接口+stub适配器
│   │   ├── printer/       # 小票/标签
│   │   └── setting/
│   │       # 每个 module 内含 handler.go / service.go / repo.go / dto.go / model.go
│   ├── pkg/               # 通用：response, errcode, pagination, money, validator
│   └── job/               # 定时任务：疫苗到期扫描、库存预警、日结
├── migrations/            # *.up.sql / *.down.sql
├── api/openapi.yaml
└── docker-compose.yml
```

### 2.3 前端目录结构（管理后台，建议）

```
admin/
├── src/
│   ├── api/          # axios 封装 + 各模块接口
│   ├── router/
│   ├── stores/       # pinia: auth, store(当前门店), ...
│   ├── layouts/      # AppShell（侧边栏+顶栏，对应设计稿）
│   ├── views/        # 每个模块一个目录
│   ├── components/   # KPI卡、表格、看板、环图、时间线等
│   ├── styles/       # tailwind + 设计令牌（见 §2.4）
│   └── utils/        # money, datetime, permission 指令 v-perm
└── tailwind.config.ts
```

### 2.4 设计令牌（必须与设计稿一致）

```css
/* 颜色 */
--canvas:#F4EEE2; --surface:#FFFDF8; --ink:#231E18;
--coral:#E26B41; --coral-d:#C9542C;  /* 主色 */
--pine:#2A5C4D;   --honey:#D99A28;   --sky:#5A83A6;  --berry:#BC5A78;
--sidebar:#211B14;
/* 字体 */
品牌/拉丁: Bricolage Grotesque; 中文标题: Noto Serif CJK SC;
中文正文: Noto Sans CJK SC; 数字/金额: Geist Mono;
/* 圆角 */ 10 / 14 / 20 px   /* 间距基准 */ 18px 网格
```
状态色映射：进行中/主操作=coral；已完成/健康/在住=pine；待办/寄养=honey；线上/退房=sky；提醒/风险=berry。

---

## 3. 部署架构与前提（边界写死）

### 3.1 部署形态
- **私有化单机部署**：一套安装服务一个商户（含其名下多门店）。**非多商户 SaaS**，因此**无跨商户租户隔离**需求，但**有门店（store）级数据隔离**（§5）。
- 通过 `docker-compose up` 拉起：`nginx`、`admin`（静态）、`api`（Go）、`postgres`、`redis`。

### 3.2 公网与微信前提（因选择了小程序预约 + 公众号通知，强制要求）
> 这些是"本地部署"与"线上预约/微信通知"共存的必要条件，开发方据此实现，无需再确认：
1. 部署服务器须有**公网可访问的域名**并配置 **HTTPS**（微信小程序与公众号回调强制 HTTPS）。
2. 商户须自备：**微信小程序** AppID/AppSecret、**微信公众号（服务号）** AppID/AppSecret 及模板消息资质、**短信服务**账号（默认阿里云短信，可在设置中切换，接口抽象见 §10.3）。
3. 上述密钥通过**环境变量/系统设置**注入，**不得硬编码**。
4. 若商户暂未提供微信/短信资质：系统须可在"仅站内通知 + 仅后台预约"降级模式下正常运行（通过 `system_settings` 开关控制）。

### 3.3 环境变量（最小集）
```
APP_ENV=prod
HTTP_PORT=8080
DB_DSN=postgres://pawprint:***@postgres:5432/pawprint?sslmode=disable
REDIS_ADDR=redis:6379
JWT_ACCESS_SECRET=***   JWT_REFRESH_SECRET=***
JWT_ACCESS_TTL=2h       JWT_REFRESH_TTL=720h
DEFAULT_TIMEZONE=Asia/Shanghai
WECHAT_MINI_APPID=...    WECHAT_MINI_SECRET=...
WECHAT_MP_APPID=...      WECHAT_MP_SECRET=...
SMS_PROVIDER=aliyun      SMS_ACCESS_KEY=...  SMS_SECRET=...  SMS_SIGN=...
FEATURE_SMS_ENABLED=false      # 资质就绪后置 true
FEATURE_WECHAT_ENABLED=false
FEATURE_ONLINE_BOOKING_ENABLED=true
```

---

## 4. 角色与权限（RBAC）

### 4.1 角色定义（5 种，按门店授权）

| 角色 code | 名称 | 范围 | 说明 |
|-----------|------|------|------|
| `super_admin` | 超级管理员 | 全商户/全门店 | 唯一可跨门店、管理门店与角色权限、系统设置 |
| `store_manager` | 店长 | 被授权的门店 | 本店全部业务 + 本店员工管理 + 查看本店财务 |
| `front_desk` | 前台 | 被授权的门店 | 预约、寄养办理、宠物档案、会员、收银结算、库存查看 |
| `staff` | 服务人员 | 被授权的门店 | 美容/兽医/护理：查看自己的排程、填写服务/照护记录、宠物健康记录 |
| `finance` | 财务 | 被授权的门店 | 财务流水、结算单、对账、日结；只读业务数据 |

> 一个用户可被授权到**多家门店**，且在不同门店可有不同角色（`user_store_roles` 关联表，见数据模型）。登录后默认进入"上次所在门店"，可在顶栏切换有权门店。

### 4.2 权限矩阵（模块 × 操作）

权限粒度采用 `module:action`（如 `appointment:create`）。下表 ✓=可操作，R=只读，空=无权。

| 模块\角色 | super_admin | store_manager | front_desk | staff | finance |
|-----------|:-:|:-:|:-:|:-:|:-:|
| 经营概览 | ✓ | ✓ | R | R(本人相关) | R |
| 预约管理 | ✓ | ✓ | ✓ | R(本人排程)+填写记录 | R |
| 寄养业务 | ✓ | ✓ | ✓ | 填写照护 | R |
| 宠物档案 | ✓ | ✓ | ✓ | R+健康记录写 | R |
| 会员客户 | ✓ | ✓ | ✓ | R | R |
| 储值/积分调整 | ✓ | ✓ | ✓(收银充值) | ✗ | R |
| 商品库存 | ✓ | ✓ | R+销售出库 | ✗ | R |
| 采购入库 | ✓ | ✓ | ✓ | ✗ | R |
| 结算收银 | ✓ | ✓ | ✓ | ✗ | ✓ |
| 财务流水/日结 | ✓ | R | ✗ | ✗ | ✓ |
| 数据分析 | ✓ | ✓ | R | ✗ | R |
| 员工管理 | ✓ | ✓(本店) | ✗ | ✗ | ✗ |
| 门店管理 | ✓ | ✗ | ✗ | ✗ | ✗ |
| 角色/权限配置 | ✓ | ✗ | ✗ | ✗ | ✗ |
| 系统设置 | ✓ | ✓(部分:房型价/模板/打印机) | ✗ | ✗ | ✗ |

> 权限以**后端中间件**为准（前端隐藏按钮仅为体验）。`super_admin` 跳过门店过滤；其余角色一律强制注入门店过滤条件。具体权限点全集见 `seed.sql` 的 `permissions` 表。

---

## 5. 多门店与数据隔离模型

- 顶层实体 `stores`（门店）。除全局基础数据外，**几乎所有业务表都带 `store_id` 外键**。
- **全局/跨门店共享**的数据：`users`（员工账号本身全局，授权按门店）、`customers`（会员，按商户共享，可在多店消费）、`pets`（宠物，归属会员，跨店共享档案）。
- **门店私有**的数据：`appointments`、`boarding_*`、`products`、`inventory`、`stock_transactions`、`settlements`、`stations/rooms`、`service_offerings`（门店可对服务定不同价）等。
- 隔离实现：后端 `store-scope` 中间件从 JWT/请求头 `X-Store-Id` 解析当前门店，校验该用户对该门店有角色授权后，在 repo 层强制追加 `store_id = :currentStoreId`。`super_admin` 可传特殊值 `*` 跨店查询。
- 会员储值余额/积分：**按商户统一账户**（在任意门店通用），但每笔流水记录发生门店 `store_id` 以便门店维度核算。

---

## 6. 数据模型

### 6.1 ER 关系（文字描述）

```
stores 1───* user_store_roles *───1 users
users 1───* user_store_roles ; user_store_roles *───1 roles
roles 1───* role_permissions *───1 permissions

customers 1───* pets
customers 1───* wallet_transactions ; customers 1───* points_transactions
customers *───1 membership_tiers (当前等级)

pets 1───* pet_health_records
pets 1───* pet_weight_records

stores 1───* stations(工位/资源)
stores 1───* service_categories 1───* services 1───* service_offerings(门店定价) ; service_offerings *───1 stores
stores 1───* appointments 1───* appointment_items
appointments *───1 customers ; appointments *───1 pets ; appointment_items *───1 services ; appointment_items *───1 stations

stores 1───* room_types 1───* boarding_rooms
stores 1───* boarding_orders 1───* boarding_care_logs
boarding_orders *───1 customers ; boarding_orders *───1 pets ; boarding_orders *───1 boarding_rooms

stores 1───* product_categories 1───* products
products 1───1 inventory(门店内) ; products(或inventory) 1───* stock_transactions
stores 1───* purchase_orders 1───* purchase_order_items *───1 products

stores 1───* settlements 1───* settlement_items ; settlements 1───* payments
settlement_items 多态来源 → appointment / boarding_order / product_sale / wallet_recharge

stores 1───* notification_logs *───1 notification_templates
stores 1───* print_jobs
* 1───* audit_logs
system_settings(键值，部分 store_id 维度)
```

### 6.2 表定义

> 通用列（除特别说明，所有业务表都含）：`id BIGINT PK（雪花或自增）`、`created_at timestamptz NOT NULL default now()`、`updated_at timestamptz NOT NULL default now()`、`deleted_at timestamptz NULL`。金额列均为 `BIGINT`（单位：分）。完整 DDL 见 `schema.sql`，此处给字段语义与约束。

#### 6.2.1 组织与权限

**stores**（门店）
| 列 | 类型 | 约束/说明 |
|----|------|-----------|
| code | varchar(32) | 唯一，门店编码 |
| name | varchar(64) | 门店名（如"旗舰店"） |
| timezone | varchar(40) | 默认 Asia/Shanghai |
| phone, address | varchar | 可空 |
| status | smallint | 1启用/0停用 |

**users**（员工账号，全局）
| 列 | 类型 | 约束 |
|----|------|------|
| username | varchar(64) | 唯一 |
| password_hash | varchar(255) | bcrypt |
| display_name | varchar(64) | |
| phone | varchar(20) | 唯一(可空) |
| avatar_text | varchar(4) | 头像首字（设计稿用） |
| status | smallint | 1启用/0禁用 |
| last_store_id | bigint | 上次登录门店 |

**roles**（角色）：`code`(唯一,见§4.1)、`name`。预置 5 条，不可删除 `is_system=true`。
**permissions**（权限点）：`code`(如 `appointment:create`)、`module`、`name`。见 seed。
**role_permissions**：`role_id`、`permission_id`（联合唯一）。
**user_store_roles**（用户-门店-角色授权）：`user_id`、`store_id`、`role_id`，联合唯一 `(user_id,store_id)`（一个用户在一个门店仅一个角色）。

#### 6.2.2 会员与宠物

**membership_tiers**（会员等级）：`code`(普通/银卡/金卡/黑钻)、`name`、`min_total_spend`(达标累计消费,分)、`discount_rate`(折扣,0-100整数表示百分比,如95=95折)、`points_rate`(每消费1元得积分数)、`sort`。
**customers**（会员，按商户共享）
| 列 | 类型 | 约束 |
|----|------|------|
| name | varchar(64) | |
| phone | varchar(20) | 唯一，登录/检索键 |
| gender | smallint | 0未知/1男/2女 |
| tier_id | bigint | 当前等级 → membership_tiers |
| wallet_balance | bigint | 储值余额(分)，**只能由 wallet_transactions 改动**，见§4约定 |
| points_balance | bigint | 积分余额 |
| total_spend | bigint | 累计消费(分) |
| source | smallint | 1到店/2小程序 |
| wechat_openid | varchar(64) | 小程序绑定，可空，唯一 |
| register_store_id | bigint | 开卡门店 |
| last_visit_at | timestamptz | 最近到店 |
| note | text | 备注（过敏、偏好等） |

**wallet_transactions**（储值流水，**资金真相**）
| 列 | 类型 | 约束 |
|----|------|------|
| customer_id | bigint | |
| store_id | bigint | 发生门店 |
| type | varchar(16) | recharge充值/consume消费/refund退款/adjust人工调整 |
| amount | bigint | 有符号，分。正=入账，负=出账 |
| balance_after | bigint | 该笔后余额（强一致快照） |
| ref_type, ref_id | | 关联结算单/充值单 |
| operator_id | bigint | 操作员 user |
| remark | varchar(255) | adjust 必填原因 |

**points_transactions**（积分流水）：结构同上，`type` = earn/redeem/adjust/expire。
**pets**（宠物，归属会员，跨店共享）
| 列 | 类型 | 约束 |
|----|------|------|
| customer_id | bigint | 主人 |
| name | varchar(64) | |
| species | smallint | 1犬/2猫/9其他 |
| breed | varchar(64) | 品种 |
| gender | smallint | 0未知/1公/2母 |
| neutered | boolean | 是否绝育 |
| birthday | date | 可空（用于算年龄） |
| weight_g | int | 最新体重(克) |
| color | varchar(32) | |
| chip_no | varchar(40) | 芯片号，手工输入 |
| blood_type | varchar(16) | |
| avatar_text | varchar(4) | |
| status | smallint | 1正常/2离世/0停用 |
| note | text | 过敏/注意事项 |

**pet_health_records**（健康档案）：`pet_id`、`type`(vaccine疫苗/deworm驱虫/exam体检/allergy过敏/other)、`title`、`performed_at date`、`next_due_at date`(可空,用于到期提醒)、`operator_id`、`detail text`。
**pet_weight_records**：`pet_id`、`weight_g int`、`recorded_at date`。

#### 6.2.3 服务与预约

**service_categories**：`code`(beauty美容/wash洗护/medical医疗/boarding寄养/retail零售)、`name`、`color`、`sort`。（用于数据分析"服务占比"）
**services**（服务项目主数据，商户级）：`category_id`、`name`、`default_duration_min int`、`default_price bigint`、`requires_station boolean`、`status`。
**service_offerings**（门店上架与定价，门店级）：`store_id`、`service_id`、`price bigint`、`duration_min int`、`bookable_online boolean`(是否允许小程序预约)、`status`。**小程序只展示 `bookable_online=true` 且 `status=1` 的项目**。
**stations**（工位/资源，门店级）：`store_id`、`name`(如"美容A位")、`type`(beauty/medical/wash/boarding/general)、`staff_user_id`(默认负责人,可空)、`color`、`status`。
**appointments**（预约单，门店级）
| 列 | 类型 | 约束 |
|----|------|------|
| store_id | bigint | |
| customer_id | bigint | 可空（散客）|
| pet_id | bigint | 可空 |
| source | smallint | 1后台/2小程序 |
| status | varchar(16) | 见§8.1 状态机 |
| scheduled_start | timestamptz | 预约开始 |
| scheduled_end | timestamptz | 预约结束（由服务时长推算，可改） |
| station_id | bigint | 可空，资源位 |
| staff_user_id | bigint | 可空，服务人员 |
| contact_name, contact_phone | varchar | 散客冗余联系方式 |
| total_amount | bigint | 预估金额（实际以结算为准） |
| remark | varchar(255) | |
| cancelled_reason | varchar(255) | |
| created_by | bigint | 后台为操作员；小程序为 null |

**appointment_items**（预约明细，一单可多项服务）：`appointment_id`、`service_offering_id`、`service_name`(冗余快照)、`price bigint`(下单快照)、`duration_min`、`station_id`(可空)。

#### 6.2.4 寄养业务

**room_types**（房型，门店级）：`store_id`、`code`(small/medium/large/cat/suite)、`name`(小型犬舍…)、`price_per_night bigint`、`capacity int`(该房型笼位数,冗余)、`sort`。
**boarding_rooms**（笼位，门店级）：`store_id`、`room_type_id`、`code`(如 S01)、`status`(free/occupied/cleaning/maintenance)、`sort`。
**boarding_orders**（寄养订单，门店级）
| 列 | 类型 | 约束 |
|----|------|------|
| store_id | bigint | |
| customer_id, pet_id | bigint | 必填 |
| room_id | bigint | 入住笼位 |
| room_type_snapshot | varchar(32) | 下单房型 |
| price_per_night | bigint | 下单单价快照 |
| status | varchar(16) | 见§8.2 |
| source | smallint | 1后台/2小程序 |
| planned_check_in | timestamptz | 计划入住 |
| planned_check_out | timestamptz | 计划退房 |
| actual_check_in | timestamptz | 实际入住 |
| actual_check_out | timestamptz | 实际退房 |
| nights int | | 计费晚数（结算时确定，见§8.4） |
| total_amount bigint | | 应收（结算时确定） |
| settlement_id bigint | | 关联结算单（退房生成） |
| remark | text | 喂养/用药交代 |

**boarding_care_logs**（每日照护记录）：`boarding_order_id`、`store_id`、`task`(feeding喂食/walking遛弯/medication喂药/photo拍照打卡)、`status`(done/pending)、`done_at timestamptz`、`operator_id`、`note`、`photo_url`(可空)。
> 看板的"今日照护进度"= 当日各 task 的 done/total 聚合。

#### 6.2.5 商品与库存

**product_categories**：`store_id` 可空(商户级)、`name`、`sort`。
**products**（商品，商户级主数据）：`name`、`category_id`、`sku varchar(64)`(唯一)、`unit varchar(8)`(袋/支/包…)、`spec varchar(64)`、`price bigint`(零售价)、`cost bigint`(成本价,可空)、`status`。
**inventory**（门店库存，门店级，唯一 `(store_id,product_id)`）：`store_id`、`product_id`、`quantity int`(当前库存)、`safety_stock int`(安全库存阈值)、`updated_at`。
**stock_transactions**（库存流水，库存唯一真相）：`store_id`、`product_id`、`type`(purchase_in采购入库/sale_out销售出库/service_out服务领用/adjust盘点调整/transfer调拨)、`quantity int`(有符号)、`balance_after int`、`ref_type,ref_id`、`operator_id`、`remark`。
> 库存数量**只能经 `stock_transactions` 变更**并同步 `inventory.quantity`（同事务）。`quantity` 扣减后若 `<= safety_stock` 触发预警事件（§7.4）。
**purchase_orders**（采购入库单，门店级，无供应商管理）：`store_id`、`code`、`status`(draft/received)、`total_cost bigint`、`operator_id`、`received_at`。
**purchase_order_items**：`purchase_order_id`、`product_id`、`quantity int`、`cost bigint`(进价快照)。确认入库时为每个 item 生成 `purchase_in` 流水。

#### 6.2.6 结算与财务

**settlements**（结算单，门店级，收银/退房/充值的统一出口）
| 列 | 类型 | 约束 |
|----|------|------|
| store_id | bigint | |
| code | varchar(32) | 唯一，单号 |
| customer_id | bigint | 可空(散客) |
| biz_type | varchar(16) | service服务/boarding寄养/retail零售/recharge储值充值/mixed混合 |
| status | varchar(16) | 见§8.3 |
| total_amount | bigint | 应收 |
| discount_amount | bigint | 优惠(会员折扣等) |
| paid_amount | bigint | 实收 |
| operator_id | bigint | 收银员 |
| paid_at | timestamptz | |
| remark | varchar(255) | |

**settlement_items**（结算明细，多态来源）：`settlement_id`、`source_type`(appointment/boarding_order/product/recharge)、`source_id`、`name`(快照)、`unit_price bigint`、`quantity int`、`amount bigint`。
**payments**（支付记录，**本期预留**）：`settlement_id`、`method`(wechat/alipay/pos/cash/wallet)、`amount bigint`、`status`(pending/success/failed/refunded)、`trade_no varchar(64)`(第三方流水,预留)、`paid_at`。
> 本期：`method=cash/wallet/pos` 由收银**手工标记**为 `success`；`wechat/alipay` 接口已定义但 service 仅返回"未启用"（§11）。

#### 6.2.7 通知 / 打印 / 审计 / 设置

**notification_templates**：`code`(vaccine_due/visit_reminder/appointment_confirmed/boarding_checkout…)、`channel`(inapp/sms/wechat_mp)、`title`、`content`(含占位符 `{petName}` 等)、`status`。
**notification_logs**：`store_id`、`customer_id`(可空)、`template_code`、`channel`、`payload jsonb`、`status`(pending/sent/failed)、`error`、`sent_at`、`scheduled_at`。
**print_jobs**：`store_id`、`type`(receipt小票/label标签)、`ref_type,ref_id`、`content jsonb`(结构化打印内容)、`status`(pending/printed/failed)、`printer_name`、`operator_id`。
**audit_logs**：`store_id`(可空)、`user_id`、`action`、`target_type`、`target_id`、`detail jsonb`、`ip`、`created_at`。（对：登录、储值/积分调整、结算、库存调整、权限变更等敏感操作强制记录）
**system_settings**：`store_id`(NULL=全局)、`key`、`value jsonb`、`updated_by`。预置键见 §7/§8 引用（如 `boarding.checkout_rule`、`inventory.warn_enabled`、`points.rule`、`feature.*`）。

---

## 7. 各模块功能规格

> 每个模块给出：界面对应（设计稿页）、功能点、关键字段、专属规则。通用 CRUD 不再赘述，重点写规则与边界。

### 7.1 M1 认证与账户【P0】
- 登录：`username + password`，bcrypt 校验；失败 5 次锁定 10 分钟（Redis 计数）。
- 登录成功返回 access(2h)+refresh(720h) 与"有权门店列表"。前端据此渲染门店切换器。
- 门店切换：调用 `POST /auth/switch-store`，校验授权后刷新 token 内 `store_id` claim，并更新 `users.last_store_id`。
- 小程序顾客登录：`code2session` 换取 openid → 匹配/创建 `customers`（`source=2`），绑定手机号（短信验证码或微信手机号授权）。

### 7.2 M2 经营概览 Dashboard【P0】（设计稿 page3）
- KPI（当前门店、当日）：今日营业额（= 当日 `settlements.paid_amount` 合计）、今日预约数、在店宠物数（在住寄养 + 当日 arrived/in_progress 的预约宠物去重）、新增会员数。
- 近 14 天营收柱状图（按日 `paid_amount` 汇总）。
- 今日预约时间线（按 `scheduled_start` 排序，显示状态）。
- 热门服务排行（按 `appointment_items` 当月计数）。
- 库存预警列表（`inventory.quantity <= safety_stock`，按紧急度排序）。
- 会员构成（按 `tier_id` 计数）。
- 所有数据**严格按当前门店**；时间窗按门店时区计算"今日"。

### 7.3 M3 预约管理【P0】（设计稿 page4）
**后台**
- 资源排程看板：列=工位(stations)，行=时间轴；卡片=预约，颜色随服务类目。
- 新建/改约：选会员(或散客)→选宠物→选服务(可多项)→自动算时长与结束时间→选工位/服务人员→选时间，校验**资源冲突**（同 station 同时段不可重叠；§8.1）。
- 状态流转按钮：到店、开始、完成、取消、标记未到。
- 视图：日/周/月。
**小程序自助预约**
- 仅 `service_offerings.bookable_online=true` 的项目可选。
- 顾客选门店→服务→可约时段（系统按工位空闲生成可选时段，粒度 30 分钟，营业时间取 `system_settings: store.business_hours`）。
- 提交后预约 `source=2,status=pending`，触发 `appointment_confirmed` 通知。
- 顾客可在小程序取消（须早于 `scheduled_start` 指定阈值，默认 2 小时，键 `appointment.cancel_deadline_hours`）。

### 7.4 M4 寄养业务【P0】（设计稿 page_寄养业务）
- 笼位占用看板：按 room_type 分组展示 boarding_rooms 及状态（free/occupied/cleaning/maintenance）。
- 办理入住：选会员/宠物→选房型→选空闲笼位→设定计划入住/退房→生成 `boarding_orders(status=booked或checked_in)`；占用笼位置 `occupied`。
- 每日照护：对在住订单按 task 打卡（feeding/walking/medication/photo），写 `boarding_care_logs`。
- 退房：见 §8.4 计费 → 生成结算单 → 收银 → 笼位置 `cleaning`（清洁完成后人工置 `free`）。
- KPI：在住数、今日入住、今日退房、寄养营收（当日寄养类结算 paid_amount）。

### 7.5 M5 宠物档案【P0】（设计稿 page5）
- 档案主信息、健康档案（疫苗/驱虫/体检/过敏，含下次到期 → 驱动提醒）、体重曲线（pet_weight_records）、服务与消费记录（聚合该宠物的 appointment/boarding/settlement_items）。
- 年龄由 `birthday` 实时计算；无生日则显示"未知"。

### 7.6 M6 会员客户 CRM【P0】（设计稿 page6）
- 会员列表/检索（手机号、昵称、芯片号联检）、详情（等级、积分、储值、累计消费、主要宠物、最近到店）。
- 储值充值：收银端发起 → 生成 `recharge` 结算单 → 收款 → 写 `wallet_transactions(type=recharge)` 增加余额。
- 储值消费：结算时若选用钱包支付，写 `wallet_transactions(type=consume, 负数)`。
- 积分：消费成功按 `points.rule` 入账（§8.6）。
- 等级：累计消费达阈值自动升级（§8.5），不自动降级（除非人工调整，键 `member.allow_downgrade=false`）。
- 关怀提醒列表：流失风险（`last_visit_at` 超 N 天，默认 30）、储值不足、等级即将到期等（规则在设置中）。

### 7.7 M7 商品库存【P0】（设计稿 page7）
- 商品主数据（商户级）+ 门店库存（inventory）。
- 销售出库：零售结算时按数量写 `sale_out` 流水并扣减。
- 服务领用：服务/寄养消耗品（如美容用品）可在完成时登记 `service_out`（可选，P1 可简化为手工）。
- 采购入库：录入 purchase_order(draft)→确认收货→为每项生成 `purchase_in` 流水、增加库存、单据置 received。
- 安全库存预警：任一出库后若 `quantity <= safety_stock`，写一条 `notification_logs(inapp, template=stock_low)` 并在 Dashboard 与库存页高亮。

### 7.8 M8 数据分析【P1】（设计稿 page8）
- 月度营收趋势（近12月）、服务类型占比（service_categories 维度）、到店时段分布（按预约/结算小时直方）、客户复访漏斗（按到店次数分桶：1次/2-3次/4-6次/7次+）。
- 全部支持门店过滤 + 时间范围筛选。统计口径以"已支付结算单"为准。

### 7.9 M9 财务流水【P1】
- 结算单列表与详情、退款（生成红冲 settlement，paid_amount 负）、按收款方式汇总、日结（生成当日 `daily_close` 汇总，键 `finance.daily_close`）。
- 财务只读业务，不可改业务单据，只能在结算维度操作。

### 7.10 M10 系统设置【P1】
- 门店管理（super_admin）、员工与授权、角色权限配置（super_admin）、房型与单价、安全库存默认值、会员等级规则、积分规则、通知模板、打印机配置、营业时间、功能开关（feature.*）。

---

## 8. 关键业务规则与状态机（强约束）

### 8.1 预约状态机（appointments.status）
```
pending(待到店) ──arrive──▶ arrived(已到店) ──start──▶ in_progress(进行中) ──complete──▶ completed(已完成)
   │                          │                                                          │
   ├──cancel──▶ cancelled     ├──cancel──▶ cancelled                                    └──(结算)──▶ 生成 settlement
   └──no_show──▶ no_show
```
- 合法迁移仅限上图；非法迁移返回 `409`。
- `completed` 后允许发起结算（service 类）。`cancelled/no_show` 释放占用的工位时段。
- **资源冲突校验**：同一 `station_id` 的 `[scheduled_start, scheduled_end)` 不可与未取消/未完成的预约重叠（completed 不占用未来时段）。冲突返回 `422 RESOURCE_CONFLICT`。

### 8.2 寄养状态机（boarding_orders.status）
```
booked(已预订) ──check_in──▶ checked_in(在住) ──check_out──▶ checked_out(已退房,已结算)
   └──cancel──▶ cancelled         └──cancel(异常)──▶ cancelled(需店长权限,释放笼位)
```
- `check_in`：写 `actual_check_in`，笼位 → occupied。
- `check_out`：按 §8.4 计费 → 生成结算单 → 结算成功后 status=checked_out、写 `actual_check_out`、笼位 → cleaning。
- 笼位状态机：`free ⇄ occupied`（入住/退房）、`occupied → cleaning →(人工)→ free`、`* ↔ maintenance`（店长手动）。

### 8.3 结算状态机（settlements.status）
```
unpaid(待结算) ──pay──▶ paid(已结算) ──refund──▶ refunded(已退款,生成红冲)
      └──void──▶ void(作废,仅unpaid可作废)
```
- `paid` 触发后续副作用（原子事务内）：扣库存(零售)、写储值/积分流水、更新 customer.total_spend、触发等级升级判断、生成小票 print_job。
- 退款生成一条 `paid_amount` 为负的红冲 settlement，并逆向处理积分/库存（库存退回写 `adjust` 流水）。

### 8.4 寄养计费算法（退房结算）
> 规则键 `boarding.checkout_rule`，默认值如下，可在设置中调整，开发按默认实现：
- **晚数 nights** = `ceil((actual_check_out - actual_check_in) / 24h)`，**不足一晚按一晚**，最少 1 晚。
- **应收 total_amount** = `nights × price_per_night`（下单单价快照）。
- 跨房型升级/降级不在本期，整单一种房型。
- 会员折扣：寄养默认**不参与**会员折扣（键 `boarding.apply_member_discount=false`）。
- 生成 `settlement(biz_type=boarding)` + 一条 `settlement_item(source_type=boarding_order)`。

### 8.5 会员等级规则
- 升级判定时机：每次结算 `paid` 后，用 `customers.total_spend` 与 `membership_tiers.min_total_spend` 比较，取满足条件的最高等级。
- 默认阈值（演示，可改）：普通=0、银卡=2000元、金卡=8000元、黑钻=20000元。
- **只升不降**（`member.allow_downgrade=false`）。等级变化写 `audit_logs`。

### 8.6 积分规则
- 规则键 `points.rule`：默认"每消费 1 元得 `tier.points_rate` 积分"（按当前等级倍率，普通=1，银=1，金=1.5，黑钻=2，向下取整）。
- 仅对**实付现金/储值/POS/微信**金额计积分；储值充值本身不计积分（避免双计）。
- 抵扣（redeem）本期可不做（P1），但流水类型与余额字段须就绪。

### 8.7 库存扣减并发与一致性
- 出库在**数据库事务 + 行级锁（`SELECT ... FOR UPDATE` on inventory row）**内执行，或用 Redis 分布式锁兜底，防止超卖。
- 库存不足（`quantity < 出库量`）默认**拒绝**并返回 `422 INSUFFICIENT_STOCK`（键 `inventory.allow_negative=false`）。
- `inventory.quantity` 与最新一条 `stock_transactions.balance_after` 必须一致（对账校验任务每日跑一次）。

### 8.8 储值一致性
- `customers.wallet_balance` 变更必须与 `wallet_transactions` 同事务；`balance_after` 为该笔后快照。
- 钱包支付时若余额不足 → `422 INSUFFICIENT_WALLET`。
- 人工调整(adjust) 需 `front_desk` 以上权限 + 必填原因 + 写审计。

---

## 9. 接口约定（详见 openapi.yaml）

### 9.1 通用
- Base URL：`/api/v1`。所有业务接口需 `Authorization: Bearer <access>` 与 `X-Store-Id: <storeId|*>`。
- 统一响应包络：
```json
{ "code": 0, "message": "ok", "data": { ... }, "trace_id": "..." }
```
`code=0` 成功；非 0 见 §17.1 错误码。HTTP 状态码同时语义化（200/400/401/403/404/409/422/500）。
- 分页：查询参数 `page`(从1)、`page_size`(默认20,最大100)；列表返回 `{ list:[], total, page, page_size }`。
- 排序/筛选：`sort=field,-field2`；筛选用具名参数（如 `status=pending`、`keyword=`）。
- **幂等**：所有写操作支持 `Idempotency-Key` 头（结算、入住、充值必须支持），24h 内同 key 返回首次结果。
- 时间一律 ISO8601 带时区。金额字段名以 `_amount`/`_balance` 结尾，单位分（整数）。

### 9.2 鉴权
- `POST /auth/login` → `{access, refresh, stores:[{id,name,role}]}`
- `POST /auth/refresh`、`POST /auth/switch-store`、`POST /auth/logout`
- 小程序：`POST /wx/auth/login`(code)、`POST /wx/auth/bind-phone`
- access token claims：`uid, store_id, role, perms?`（perms 可懒加载）。

### 9.3 主要资源端点（概览，细节见 openapi）
`/auth/* /stores /users /roles /permissions`
`/customers /customers/{id}/wallet /customers/{id}/points /pets /pets/{id}/health /pets/{id}/weights`
`/services /service-offerings /stations /appointments /appointments/{id}/transitions`
`/room-types /rooms /boarding-orders /boarding-orders/{id}/check-in /check-out /care-logs`
`/products /inventory /stock-transactions /purchase-orders`
`/settlements /settlements/{id}/pay /refund /payments`
`/dashboard/summary /analytics/*`
`/notifications /notification-templates /print-jobs /settings /audit-logs`
小程序前缀 `/wx/*`：`/wx/stores /wx/service-offerings /wx/appointments(我的) /wx/profile`。

---

## 10. 通知系统

### 10.1 触发点（P0 必须落 notification_logs，发送通道随 P1）
| 模板 code | 触发 | 渠道 |
|-----------|------|------|
| `appointment_confirmed` | 预约创建成功 | inapp + wechat_mp |
| `visit_reminder` | 预约前 N 小时（默认24，定时任务扫描） | sms + wechat_mp |
| `vaccine_due` | `pet_health_records.next_due_at` 前 N 天（默认7，每日任务） | sms + wechat_mp |
| `boarding_checkout` | 寄养计划退房当日 | inapp + wechat_mp |
| `stock_low` | 库存≤安全库存 | inapp（仅店员） |

### 10.2 发送流程
- 触发 → 写 `notification_logs(status=pending, scheduled_at)` → 异步 worker 取出 → 按 `FEATURE_SMS_ENABLED/FEATURE_WECHAT_ENABLED` 决定是否真实发送 → 更新 status(sent/failed)+重试（最多3次，指数退避）。
- 渠道未启用时：inapp 始终生效；sms/wechat_mp 记 `status=skipped`。

### 10.3 通道适配器接口（Go）
```go
type Notifier interface {
    Send(ctx context.Context, log NotificationLog) error
    Channel() string // inapp/sms/wechat_mp
}
```
默认实现：`AliyunSMSNotifier`、`WechatMpNotifier`、`InAppNotifier`。短信服务商通过 `SMS_PROVIDER` 可替换（接口不变）。

---

## 11. 支付接口（本期【预留】不实现）
- 定义统一支付网关接口，注册三种适配器（wechat/alipay/pos），本期实现仅返回 `PAYMENT_NOT_ENABLED`。
```go
type PaymentGateway interface {
    Prepay(ctx, PrepayReq) (PrepayResp, error) // 预下单
    Query(ctx, tradeNo string) (PayStatus, error)
    Refund(ctx, RefundReq) (RefundResp, error)
    Method() string
}
```
- 收银结算时：`cash/wallet/pos` 直接由收银手工确认 `payments.status=success`；`wechat/alipay` 调 `Prepay` → 返回未启用错误，前端提示"线上支付未开通，请选择其他方式"。
- `payments` 表、回调路由 `/pay/callback/{provider}`（占位，校验后 501）须就绪，便于二期接入。

---

## 12. 硬件 · 打印机【P0 结构 / 实现取决于现场】
- 支持**小票打印机**（ESC/POS，58/80mm）与**标签打印机**（寄养笼位标签、宠物标签）。
- 后端只产出**结构化打印内容**写 `print_jobs(status=pending, content jsonb)`；实际打印由**前端/本地打印代理**消费（浏览器走可调用本地打印服务的轻量 agent，或 WebUSB/系统打印）。开发方实现一个 `print-agent`（可选 P1）或对接现场已有驱动；后端契约固定为 print_jobs。
- 小票内容字段：门店名、单号、时间、明细(name/qty/price)、合计、优惠、实收、收款方式、会员与积分变动、二维码(可选)。
- 标签内容：宠物名/主人/笼位/入住-退房日期/注意事项。

---

## 13. 顾客端（微信小程序）规格【P0】
- 页面：门店选择、服务列表、预约下单、我的预约（列表/详情/取消）、我的会员（等级/储值/积分）、消费记录、我的宠物、消息（模板消息落地页）。
- 仅消费**已确认开放在线预约**的 service_offerings；时段由后端按工位空闲计算返回。
- 登录与绑定见 §7.1。所有接口走 `/wx/*`，鉴权用顾客维度 JWT（claims: `customer_id`）。
- 不含支付与商城。

---

## 14. 非功能需求
- **性能**：常规列表接口 P95 < 300ms（万级数据）；看板/Dashboard 聚合 < 800ms（可用物化视图或缓存）。
- **安全**：密码 bcrypt(cost≥10)；JWT 双 token；敏感操作审计；接口限流（默认 60 req/min/用户，登录更严）；输入校验（防注入，用 ORM 参数化）；越权防护以 store-scope + RBAC 中间件强制。
- **并发**：库存/储值/结算用事务+行锁；幂等键防重复提交。
- **日志**：结构化 JSON 日志（trace_id 贯穿）；审计独立表。
- **备份**：PostgreSQL 每日全量 + WAL；提供 `pg_dump` 脚本与恢复说明。
- **可观测**：健康检查 `/healthz`、`/readyz`；关键指标可选 Prometheus。
- **数据**：软删除；金额整数分；时间 UTC 存储。

---

## 15. 验收标准（DoD，逐模块）
> 每个模块"完成"须同时满足：(1) 接口符合 openapi 且通过 §测试用例；(2) 权限矩阵生效（越权返回403）；(3) 门店隔离生效；(4) 关键写操作有审计/流水；(5) 单元测试覆盖核心 service ≥70%，关键状态机/计费/库存/储值 100% 覆盖分支；(6) 演示数据可一键导入并跑通主流程。

主流程验收（端到端）：
1. 登录→切门店→查看 Dashboard 数据正确（与 seed 对账）。
2. 新建预约（资源冲突被拦截）→到店→开始→完成→结算→库存/积分/累计消费/等级正确变化→小票 print_job 生成。
3. 寄养：办理入住（笼位占用）→照护打卡→退房计费（晚数与金额按 §8.4）→结算→笼位转清洁。
4. 储值充值→钱包支付消费→余额与流水一致。
5. 采购入库→库存增加；销售出库至安全库存以下→产生预警。
6. 小程序：自助预约成功→后台可见→`appointment_confirmed` 日志生成。
7. 越权与跨门店访问被正确拒绝。

---

## 16. 里程碑
- **P0（首期）**：M1 认证/RBAC、M2 概览、M3 预约(后台+小程序)、M4 寄养、M5 宠物、M6 会员储值积分、M7 库存、通知落库(inapp)、结算与小票 print_job、示例库与测试用例。
- **P1（二期）**：M8 分析、M9 财务/日结、M10 系统设置全功能、短信+公众号真实发送、支付适配器接入、标签/小票打印 agent、积分抵扣。

---

## 17. 附录

### 17.1 错误码（`code` 字段）
| code | HTTP | 含义 |
|------|------|------|
| 0 | 200 | 成功 |
| 1001 | 401 | 未认证/Token 失效 |
| 1002 | 403 | 无权限 |
| 1003 | 403 | 跨门店访问被拒 |
| 2001 | 400 | 参数校验失败 |
| 2002 | 404 | 资源不存在 |
| 3001 | 409 | 状态机非法迁移 |
| 3002 | 422 | RESOURCE_CONFLICT 资源时段冲突 |
| 3003 | 422 | INSUFFICIENT_STOCK 库存不足 |
| 3004 | 422 | INSUFFICIENT_WALLET 储值不足 |
| 4001 | 501 | PAYMENT_NOT_ENABLED 支付未启用 |
| 5000 | 500 | 服务器内部错误 |

### 17.2 术语表
门店(store)、会员(customer)、工位/资源(station)、服务上架(service_offering)、笼位(boarding_room)、房型(room_type)、寄养订单(boarding_order)、照护记录(care_log)、结算单(settlement)、库存流水(stock_transaction)、储值流水(wallet_transaction)。

### 17.3 演示数据说明
`seed.sql` 含 1 商户 / 1 门店（"旗舰店"）/ 5 角色 / 若干员工 / 会员与宠物 / 房型与笼位 / 服务与上架 / 商品与库存 / 示例预约与寄养订单 / 通知模板。**全部为演示数据，价格、数量、姓名均可替换**；唯结构、枚举、规则为约束。

### 17.4 未尽事项的处理（重申 §0.3）
开发中如遇本文档未覆盖的细节，按 §0.3 自行决策并记录于 `DECISIONS.md`，**不回头确认**。严禁新增业务范围或将"预留"做成真实实现。

— 文档结束 —
