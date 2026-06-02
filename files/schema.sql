-- =====================================================================
-- 爪迹 PawPrint 宠物店管理 SaaS — 数据库 Schema (PostgreSQL 15+)
-- 约定: 金额 BIGINT(分); 时间 timestamptz(UTC); 软删除 deleted_at;
--       枚举用 varchar + CHECK; 与开发文档 §6 数据模型一一对应。
-- =====================================================================
SET client_encoding = 'UTF8';

-- ---------- 通用触发器: 自动维护 updated_at ----------
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN NEW.updated_at = now(); RETURN NEW; END; $$ LANGUAGE plpgsql;

-- =========================== 组织与权限 ===============================
CREATE TABLE stores (
  id           BIGSERIAL PRIMARY KEY,
  code         VARCHAR(32)  NOT NULL UNIQUE,
  name         VARCHAR(64)  NOT NULL,
  timezone     VARCHAR(40)  NOT NULL DEFAULT 'Asia/Shanghai',
  phone        VARCHAR(20),
  address      VARCHAR(255),
  status       SMALLINT     NOT NULL DEFAULT 1,
  created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
  updated_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
  deleted_at   TIMESTAMPTZ
);

CREATE TABLE users (
  id            BIGSERIAL PRIMARY KEY,
  username      VARCHAR(64)  NOT NULL UNIQUE,
  password_hash VARCHAR(255) NOT NULL,
  display_name  VARCHAR(64)  NOT NULL,
  phone         VARCHAR(20)  UNIQUE,
  avatar_text   VARCHAR(4),
  status        SMALLINT     NOT NULL DEFAULT 1,
  last_store_id BIGINT REFERENCES stores(id),
  created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
  deleted_at    TIMESTAMPTZ
);

CREATE TABLE roles (
  id        BIGSERIAL PRIMARY KEY,
  code      VARCHAR(32) NOT NULL UNIQUE,   -- super_admin/store_manager/front_desk/staff/finance
  name      VARCHAR(32) NOT NULL,
  is_system BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE permissions (
  id     BIGSERIAL PRIMARY KEY,
  code   VARCHAR(64) NOT NULL UNIQUE,      -- e.g. appointment:create
  module VARCHAR(32) NOT NULL,
  name   VARCHAR(64) NOT NULL
);

CREATE TABLE role_permissions (
  role_id       BIGINT NOT NULL REFERENCES roles(id),
  permission_id BIGINT NOT NULL REFERENCES permissions(id),
  PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE user_store_roles (
  id       BIGSERIAL PRIMARY KEY,
  user_id  BIGINT NOT NULL REFERENCES users(id),
  store_id BIGINT NOT NULL REFERENCES stores(id),
  role_id  BIGINT NOT NULL REFERENCES roles(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (user_id, store_id)
);

-- =========================== 会员与宠物 ===============================
CREATE TABLE membership_tiers (
  id              BIGSERIAL PRIMARY KEY,
  code            VARCHAR(16) NOT NULL UNIQUE,  -- normal/silver/gold/diamond
  name            VARCHAR(32) NOT NULL,
  min_total_spend BIGINT  NOT NULL DEFAULT 0,   -- 达标累计消费(分)
  discount_rate   SMALLINT NOT NULL DEFAULT 100,-- 95 = 95折
  points_rate     NUMERIC(4,2) NOT NULL DEFAULT 1.0,
  sort            SMALLINT NOT NULL DEFAULT 0
);

CREATE TABLE customers (
  id                BIGSERIAL PRIMARY KEY,
  name              VARCHAR(64) NOT NULL,
  phone             VARCHAR(20) NOT NULL UNIQUE,
  gender            SMALLINT NOT NULL DEFAULT 0,
  tier_id           BIGINT REFERENCES membership_tiers(id),
  wallet_balance    BIGINT NOT NULL DEFAULT 0,
  points_balance    BIGINT NOT NULL DEFAULT 0,
  total_spend       BIGINT NOT NULL DEFAULT 0,
  source            SMALLINT NOT NULL DEFAULT 1, -- 1到店/2小程序
  wechat_openid     VARCHAR(64) UNIQUE,
  register_store_id BIGINT REFERENCES stores(id),
  last_visit_at     TIMESTAMPTZ,
  note              TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);

CREATE TABLE wallet_transactions (
  id            BIGSERIAL PRIMARY KEY,
  customer_id   BIGINT NOT NULL REFERENCES customers(id),
  store_id      BIGINT NOT NULL REFERENCES stores(id),
  type          VARCHAR(16) NOT NULL CHECK (type IN ('recharge','consume','refund','adjust')),
  amount        BIGINT NOT NULL,           -- 有符号
  balance_after BIGINT NOT NULL,
  ref_type      VARCHAR(32),
  ref_id        BIGINT,
  operator_id   BIGINT REFERENCES users(id),
  remark        VARCHAR(255),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_wallet_tx_customer ON wallet_transactions(customer_id, created_at DESC);

CREATE TABLE points_transactions (
  id            BIGSERIAL PRIMARY KEY,
  customer_id   BIGINT NOT NULL REFERENCES customers(id),
  store_id      BIGINT NOT NULL REFERENCES stores(id),
  type          VARCHAR(16) NOT NULL CHECK (type IN ('earn','redeem','adjust','expire')),
  amount        BIGINT NOT NULL,
  balance_after BIGINT NOT NULL,
  ref_type      VARCHAR(32),
  ref_id        BIGINT,
  operator_id   BIGINT REFERENCES users(id),
  remark        VARCHAR(255),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_points_tx_customer ON points_transactions(customer_id, created_at DESC);

CREATE TABLE pets (
  id          BIGSERIAL PRIMARY KEY,
  customer_id BIGINT NOT NULL REFERENCES customers(id),
  name        VARCHAR(64) NOT NULL,
  species     SMALLINT NOT NULL DEFAULT 1,  -- 1犬/2猫/9其他
  breed       VARCHAR(64),
  gender      SMALLINT NOT NULL DEFAULT 0,
  neutered    BOOLEAN NOT NULL DEFAULT false,
  birthday    DATE,
  weight_g    INT,
  color       VARCHAR(32),
  chip_no     VARCHAR(40),
  blood_type  VARCHAR(16),
  avatar_text VARCHAR(4),
  status      SMALLINT NOT NULL DEFAULT 1,  -- 1正常/2离世/0停用
  note        TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);
CREATE INDEX idx_pets_customer ON pets(customer_id);
CREATE INDEX idx_pets_chip ON pets(chip_no);

CREATE TABLE pet_health_records (
  id           BIGSERIAL PRIMARY KEY,
  pet_id       BIGINT NOT NULL REFERENCES pets(id),
  type         VARCHAR(16) NOT NULL CHECK (type IN ('vaccine','deworm','exam','allergy','other')),
  title        VARCHAR(128) NOT NULL,
  performed_at DATE,
  next_due_at  DATE,
  operator_id  BIGINT REFERENCES users(id),
  detail       TEXT,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_health_due ON pet_health_records(next_due_at) WHERE next_due_at IS NOT NULL;

CREATE TABLE pet_weight_records (
  id          BIGSERIAL PRIMARY KEY,
  pet_id      BIGINT NOT NULL REFERENCES pets(id),
  weight_g    INT NOT NULL,
  recorded_at DATE NOT NULL DEFAULT CURRENT_DATE
);

-- =========================== 服务与预约 ===============================
CREATE TABLE service_categories (
  id    BIGSERIAL PRIMARY KEY,
  code  VARCHAR(16) NOT NULL UNIQUE, -- beauty/wash/medical/boarding/retail
  name  VARCHAR(32) NOT NULL,
  color VARCHAR(16),
  sort  SMALLINT NOT NULL DEFAULT 0
);

CREATE TABLE services (
  id                   BIGSERIAL PRIMARY KEY,
  category_id          BIGINT NOT NULL REFERENCES service_categories(id),
  name                 VARCHAR(64) NOT NULL,
  default_duration_min INT NOT NULL DEFAULT 60,
  default_price        BIGINT NOT NULL DEFAULT 0,
  requires_station     BOOLEAN NOT NULL DEFAULT true,
  status               SMALLINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);

CREATE TABLE service_offerings (
  id              BIGSERIAL PRIMARY KEY,
  store_id        BIGINT NOT NULL REFERENCES stores(id),
  service_id      BIGINT NOT NULL REFERENCES services(id),
  price           BIGINT NOT NULL,
  duration_min    INT NOT NULL,
  bookable_online BOOLEAN NOT NULL DEFAULT false,
  status          SMALLINT NOT NULL DEFAULT 1,
  UNIQUE (store_id, service_id)
);

CREATE TABLE stations (
  id            BIGSERIAL PRIMARY KEY,
  store_id      BIGINT NOT NULL REFERENCES stores(id),
  name          VARCHAR(64) NOT NULL,
  type          VARCHAR(16) NOT NULL DEFAULT 'general',
  staff_user_id BIGINT REFERENCES users(id),
  color         VARCHAR(16),
  status        SMALLINT NOT NULL DEFAULT 1,
  deleted_at    TIMESTAMPTZ
);
CREATE INDEX idx_stations_store ON stations(store_id);

CREATE TABLE appointments (
  id              BIGSERIAL PRIMARY KEY,
  store_id        BIGINT NOT NULL REFERENCES stores(id),
  customer_id     BIGINT REFERENCES customers(id),
  pet_id          BIGINT REFERENCES pets(id),
  source          SMALLINT NOT NULL DEFAULT 1,
  status          VARCHAR(16) NOT NULL DEFAULT 'pending'
                  CHECK (status IN ('pending','arrived','in_progress','completed','cancelled','no_show')),
  scheduled_start TIMESTAMPTZ NOT NULL,
  scheduled_end   TIMESTAMPTZ NOT NULL,
  station_id      BIGINT REFERENCES stations(id),
  staff_user_id   BIGINT REFERENCES users(id),
  contact_name    VARCHAR(64),
  contact_phone   VARCHAR(20),
  total_amount    BIGINT NOT NULL DEFAULT 0,
  remark          VARCHAR(255),
  cancelled_reason VARCHAR(255),
  created_by      BIGINT REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);
CREATE INDEX idx_appt_store_time ON appointments(store_id, scheduled_start);
CREATE INDEX idx_appt_station_time ON appointments(station_id, scheduled_start, scheduled_end);

CREATE TABLE appointment_items (
  id                  BIGSERIAL PRIMARY KEY,
  appointment_id      BIGINT NOT NULL REFERENCES appointments(id),
  service_offering_id BIGINT NOT NULL REFERENCES service_offerings(id),
  service_name        VARCHAR(64) NOT NULL,
  price               BIGINT NOT NULL,
  duration_min        INT NOT NULL,
  station_id          BIGINT REFERENCES stations(id)
);

-- =========================== 寄养业务 =================================
CREATE TABLE room_types (
  id              BIGSERIAL PRIMARY KEY,
  store_id        BIGINT NOT NULL REFERENCES stores(id),
  code            VARCHAR(16) NOT NULL, -- small/medium/large/cat/suite
  name            VARCHAR(32) NOT NULL,
  price_per_night BIGINT NOT NULL,
  capacity        INT NOT NULL DEFAULT 0,
  sort            SMALLINT NOT NULL DEFAULT 0,
  UNIQUE (store_id, code)
);

CREATE TABLE boarding_rooms (
  id           BIGSERIAL PRIMARY KEY,
  store_id     BIGINT NOT NULL REFERENCES stores(id),
  room_type_id BIGINT NOT NULL REFERENCES room_types(id),
  code         VARCHAR(16) NOT NULL,  -- S01...
  status       VARCHAR(16) NOT NULL DEFAULT 'free'
               CHECK (status IN ('free','occupied','cleaning','maintenance')),
  sort         SMALLINT NOT NULL DEFAULT 0,
  UNIQUE (store_id, code)
);

CREATE TABLE boarding_orders (
  id                  BIGSERIAL PRIMARY KEY,
  store_id            BIGINT NOT NULL REFERENCES stores(id),
  customer_id         BIGINT NOT NULL REFERENCES customers(id),
  pet_id              BIGINT NOT NULL REFERENCES pets(id),
  room_id             BIGINT REFERENCES boarding_rooms(id),
  room_type_snapshot  VARCHAR(32) NOT NULL,
  price_per_night     BIGINT NOT NULL,
  status              VARCHAR(16) NOT NULL DEFAULT 'booked'
                      CHECK (status IN ('booked','checked_in','checked_out','cancelled')),
  source              SMALLINT NOT NULL DEFAULT 1,
  planned_check_in    TIMESTAMPTZ NOT NULL,
  planned_check_out   TIMESTAMPTZ NOT NULL,
  actual_check_in     TIMESTAMPTZ,
  actual_check_out    TIMESTAMPTZ,
  nights              INT,
  total_amount        BIGINT,
  settlement_id       BIGINT,
  remark              TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);
CREATE INDEX idx_boarding_store_status ON boarding_orders(store_id, status);

CREATE TABLE boarding_care_logs (
  id                BIGSERIAL PRIMARY KEY,
  boarding_order_id BIGINT NOT NULL REFERENCES boarding_orders(id),
  store_id          BIGINT NOT NULL REFERENCES stores(id),
  task              VARCHAR(16) NOT NULL CHECK (task IN ('feeding','walking','medication','photo')),
  status            VARCHAR(8) NOT NULL DEFAULT 'pending' CHECK (status IN ('done','pending')),
  done_at           TIMESTAMPTZ,
  operator_id       BIGINT REFERENCES users(id),
  note              VARCHAR(255),
  photo_url         VARCHAR(255),
  log_date          DATE NOT NULL DEFAULT CURRENT_DATE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_care_order_date ON boarding_care_logs(boarding_order_id, log_date);

-- =========================== 商品与库存 ===============================
CREATE TABLE product_categories (
  id       BIGSERIAL PRIMARY KEY,
  store_id BIGINT REFERENCES stores(id),
  name     VARCHAR(32) NOT NULL,
  sort     SMALLINT NOT NULL DEFAULT 0
);

CREATE TABLE products (
  id          BIGSERIAL PRIMARY KEY,
  name        VARCHAR(64) NOT NULL,
  category_id BIGINT REFERENCES product_categories(id),
  sku         VARCHAR(64) UNIQUE,
  unit        VARCHAR(8),
  spec        VARCHAR(64),
  price       BIGINT NOT NULL DEFAULT 0,
  cost        BIGINT,
  status      SMALLINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);

CREATE TABLE inventory (
  id           BIGSERIAL PRIMARY KEY,
  store_id     BIGINT NOT NULL REFERENCES stores(id),
  product_id   BIGINT NOT NULL REFERENCES products(id),
  quantity     INT NOT NULL DEFAULT 0,
  safety_stock INT NOT NULL DEFAULT 0,
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (store_id, product_id)
);

CREATE TABLE stock_transactions (
  id            BIGSERIAL PRIMARY KEY,
  store_id      BIGINT NOT NULL REFERENCES stores(id),
  product_id    BIGINT NOT NULL REFERENCES products(id),
  type          VARCHAR(16) NOT NULL
                CHECK (type IN ('purchase_in','sale_out','service_out','adjust','transfer')),
  quantity      INT NOT NULL,         -- 有符号
  balance_after INT NOT NULL,
  ref_type      VARCHAR(32),
  ref_id        BIGINT,
  operator_id   BIGINT REFERENCES users(id),
  remark        VARCHAR(255),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_stock_tx ON stock_transactions(store_id, product_id, created_at DESC);

CREATE TABLE purchase_orders (
  id          BIGSERIAL PRIMARY KEY,
  store_id    BIGINT NOT NULL REFERENCES stores(id),
  code        VARCHAR(32) NOT NULL UNIQUE,
  status      VARCHAR(16) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','received')),
  total_cost  BIGINT NOT NULL DEFAULT 0,
  operator_id BIGINT REFERENCES users(id),
  received_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE purchase_order_items (
  id                BIGSERIAL PRIMARY KEY,
  purchase_order_id BIGINT NOT NULL REFERENCES purchase_orders(id),
  product_id        BIGINT NOT NULL REFERENCES products(id),
  quantity          INT NOT NULL,
  cost              BIGINT NOT NULL
);

-- =========================== 结算与财务 ===============================
CREATE TABLE settlements (
  id              BIGSERIAL PRIMARY KEY,
  store_id        BIGINT NOT NULL REFERENCES stores(id),
  code            VARCHAR(32) NOT NULL UNIQUE,
  customer_id     BIGINT REFERENCES customers(id),
  biz_type        VARCHAR(16) NOT NULL
                  CHECK (biz_type IN ('service','boarding','retail','recharge','mixed')),
  status          VARCHAR(16) NOT NULL DEFAULT 'unpaid'
                  CHECK (status IN ('unpaid','paid','refunded','void')),
  total_amount    BIGINT NOT NULL DEFAULT 0,
  discount_amount BIGINT NOT NULL DEFAULT 0,
  paid_amount     BIGINT NOT NULL DEFAULT 0,
  operator_id     BIGINT REFERENCES users(id),
  paid_at         TIMESTAMPTZ,
  remark          VARCHAR(255),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_settle_store_time ON settlements(store_id, created_at DESC);
CREATE INDEX idx_settle_paid ON settlements(store_id, paid_at) WHERE status='paid';

CREATE TABLE settlement_items (
  id           BIGSERIAL PRIMARY KEY,
  settlement_id BIGINT NOT NULL REFERENCES settlements(id),
  source_type  VARCHAR(16) NOT NULL CHECK (source_type IN ('appointment','boarding_order','product','recharge')),
  source_id    BIGINT,
  name         VARCHAR(128) NOT NULL,
  unit_price   BIGINT NOT NULL,
  quantity     INT NOT NULL DEFAULT 1,
  amount       BIGINT NOT NULL
);

CREATE TABLE payments (
  id            BIGSERIAL PRIMARY KEY,
  settlement_id BIGINT NOT NULL REFERENCES settlements(id),
  method        VARCHAR(16) NOT NULL CHECK (method IN ('wechat','alipay','pos','cash','wallet')),
  amount        BIGINT NOT NULL,
  status        VARCHAR(16) NOT NULL DEFAULT 'pending'
                CHECK (status IN ('pending','success','failed','refunded')),
  trade_no      VARCHAR(64),
  paid_at       TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- =========================== 通知/打印/审计/设置 ======================
CREATE TABLE notification_templates (
  id      BIGSERIAL PRIMARY KEY,
  code    VARCHAR(32) NOT NULL,
  channel VARCHAR(16) NOT NULL CHECK (channel IN ('inapp','sms','wechat_mp')),
  title   VARCHAR(128),
  content TEXT NOT NULL,
  status  SMALLINT NOT NULL DEFAULT 1,
  UNIQUE (code, channel)
);

CREATE TABLE notification_logs (
  id            BIGSERIAL PRIMARY KEY,
  store_id      BIGINT REFERENCES stores(id),
  customer_id   BIGINT REFERENCES customers(id),
  template_code VARCHAR(32) NOT NULL,
  channel       VARCHAR(16) NOT NULL,
  payload       JSONB,
  status        VARCHAR(16) NOT NULL DEFAULT 'pending'
                CHECK (status IN ('pending','sent','failed','skipped')),
  error         VARCHAR(255),
  retry_count   SMALLINT NOT NULL DEFAULT 0,
  scheduled_at  TIMESTAMPTZ,
  sent_at       TIMESTAMPTZ,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_notif_pending ON notification_logs(status, scheduled_at) WHERE status='pending';

CREATE TABLE print_jobs (
  id          BIGSERIAL PRIMARY KEY,
  store_id    BIGINT NOT NULL REFERENCES stores(id),
  type        VARCHAR(16) NOT NULL CHECK (type IN ('receipt','label')),
  ref_type    VARCHAR(32),
  ref_id      BIGINT,
  content     JSONB NOT NULL,
  status      VARCHAR(16) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','printed','failed')),
  printer_name VARCHAR(64),
  operator_id BIGINT REFERENCES users(id),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE audit_logs (
  id          BIGSERIAL PRIMARY KEY,
  store_id    BIGINT REFERENCES stores(id),
  user_id     BIGINT REFERENCES users(id),
  action      VARCHAR(64) NOT NULL,
  target_type VARCHAR(32),
  target_id   BIGINT,
  detail      JSONB,
  ip          VARCHAR(45),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_store_time ON audit_logs(store_id, created_at DESC);

CREATE TABLE system_settings (
  id         BIGSERIAL PRIMARY KEY,
  store_id   BIGINT REFERENCES stores(id),  -- NULL = 全局
  key        VARCHAR(64) NOT NULL,
  value      JSONB NOT NULL,
  updated_by BIGINT REFERENCES users(id),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (store_id, key)
);

-- ---------- updated_at 触发器挂载（核心表） ----------
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['stores','users','customers','pets','services','appointments',
    'boarding_orders','products','settlements'] LOOP
    EXECUTE format('CREATE TRIGGER trg_%s_updated BEFORE UPDATE ON %I FOR EACH ROW EXECUTE FUNCTION set_updated_at();', t, t);
  END LOOP;
END $$;
