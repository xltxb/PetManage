-- =====================================================================
-- 爪迹 PawPrint — 示例数据 seed.sql  (依赖 schema.sql 先执行)
-- 全部为演示数据，可替换。密码统一为: pawprint123  (bcrypt)
-- 金额单位: 分。  执行后会重置各表序列。
-- =====================================================================
SET client_encoding = 'UTF8';
\set pwd '''$2b$10$hI3knf0o5Xt21pyemrPVbOQZVRXpnzW2JHcpnn3eA76fbZq5h066q'''

-- ---------------- 门店 ----------------
INSERT INTO stores(id,code,name,timezone,phone,address) VALUES
 (1,'FLAGSHIP','旗舰店','Asia/Shanghai','021-66880088','上海市浦东新区宠物大道1号');

-- ---------------- 角色 ----------------
INSERT INTO roles(id,code,name,is_system) VALUES
 (1,'super_admin','超级管理员',true),
 (2,'store_manager','店长',true),
 (3,'front_desk','前台',true),
 (4,'staff','服务人员',true),
 (5,'finance','财务',true);

-- ---------------- 权限点（节选核心，可扩展） ----------------
INSERT INTO permissions(id,code,module,name) VALUES
 (1,'dashboard:view','dashboard','查看概览'),
 (2,'appointment:view','appointment','查看预约'),
 (3,'appointment:create','appointment','创建预约'),
 (4,'appointment:transition','appointment','预约状态流转'),
 (5,'boarding:view','boarding','查看寄养'),
 (6,'boarding:checkin','boarding','办理入住'),
 (7,'boarding:checkout','boarding','办理退房'),
 (8,'boarding:care','boarding','照护打卡'),
 (9,'pet:view','pet','查看宠物'),
 (10,'pet:edit','pet','编辑宠物'),
 (11,'pet:health','pet','健康记录'),
 (12,'member:view','member','查看会员'),
 (13,'member:edit','member','编辑会员'),
 (14,'member:wallet','member','储值调整'),
 (15,'inventory:view','inventory','查看库存'),
 (16,'inventory:sale','inventory','销售出库'),
 (17,'inventory:purchase','inventory','采购入库'),
 (18,'settlement:create','settlement','创建结算'),
 (19,'settlement:pay','settlement','收银'),
 (20,'finance:view','finance','财务流水'),
 (21,'finance:close','finance','日结'),
 (22,'analytics:view','analytics','数据分析'),
 (23,'user:manage','setting','员工管理'),
 (24,'store:manage','setting','门店管理'),
 (25,'role:manage','setting','角色权限'),
 (26,'setting:manage','setting','系统设置');

-- 角色-权限：super_admin 拥有全部
INSERT INTO role_permissions(role_id,permission_id) SELECT 1,id FROM permissions;
-- store_manager：除 store/role 管理外几乎全部
INSERT INTO role_permissions(role_id,permission_id) SELECT 2,id FROM permissions WHERE code NOT IN ('store:manage','role:manage');
-- front_desk
INSERT INTO role_permissions(role_id,permission_id) SELECT 3,id FROM permissions WHERE code IN
 ('dashboard:view','appointment:view','appointment:create','appointment:transition',
  'boarding:view','boarding:checkin','boarding:checkout','boarding:care',
  'pet:view','pet:edit','member:view','member:edit','member:wallet',
  'inventory:view','inventory:sale','inventory:purchase','settlement:create','settlement:pay','analytics:view');
-- staff
INSERT INTO role_permissions(role_id,permission_id) SELECT 4,id FROM permissions WHERE code IN
 ('appointment:view','appointment:transition','boarding:view','boarding:care','pet:view','pet:health');
-- finance
INSERT INTO role_permissions(role_id,permission_id) SELECT 5,id FROM permissions WHERE code IN
 ('dashboard:view','finance:view','finance:close','settlement:create','settlement:pay','analytics:view',
  'member:view','inventory:view','appointment:view','boarding:view','pet:view');

-- ---------------- 员工 ----------------
INSERT INTO users(id,username,password_hash,display_name,phone,avatar_text,last_store_id) VALUES
 (1,'admin',     :pwd,'超级管理员','13900000000','管',1),
 (2,'linwq',     :pwd,'林晚晴','13900000001','林',1),
 (3,'frontdesk', :pwd,'周敏','13900000002','周',1),
 (4,'zhangm',    :pwd,'张萌','13900000003','张',1),
 (5,'lixue',     :pwd,'李雪','13900000004','李',1),
 (6,'sunwei',    :pwd,'孙伟','13900000005','孙',1),
 (7,'wanghao',   :pwd,'王浩','13900000006','王',1),
 (8,'finance',   :pwd,'钱芳','13900000007','钱',1);

INSERT INTO user_store_roles(user_id,store_id,role_id) VALUES
 (1,1,1),(2,1,2),(3,1,3),(4,1,4),(5,1,4),(6,1,4),(7,1,4),(8,1,5);

-- ---------------- 会员等级 ----------------
INSERT INTO membership_tiers(id,code,name,min_total_spend,discount_rate,points_rate,sort) VALUES
 (1,'normal','普通会员',0,100,1.0,0),
 (2,'silver','银卡会员',200000,98,1.0,1),
 (3,'gold','金卡会员',800000,95,1.5,2),
 (4,'diamond','黑钻会员',2000000,90,2.0,3);

-- ---------------- 会员 ----------------
INSERT INTO customers(id,name,phone,gender,tier_id,wallet_balance,points_balance,total_spend,source,register_store_id,last_visit_at,note) VALUES
 (1,'王梓萱','13800135566',2,4,128000,2860,864000,1,1, now()-interval '2 hour','布丁怕吹风机'),
 (2,'陈睿',  '13988421190',1,3,45000,1240,532000,1,1, now()-interval '2 day',null),
 (3,'周雨桐','13766203344',2,4,210000,3150,1248000,1,1, now()-interval '3 hour',null),
 (4,'刘思远','13599012278',1,2,0,680,224000,1,1, now()-interval '5 day',null),
 (5,'黄家俊','18877335521',1,3,62000,1560,618000,1,1, now()-interval '6 hour',null),
 (6,'吴沛',  '18622408867',1,2,18000,420,186000,1,1, now()-interval '7 day',null),
 (7,'林清',  '15933017788',2,1,4000,90,56000,2,1, now()-interval '21 day',null),
 (8,'赵敏',  '17788902233',2,4,305000,4200,1560000,1,1, now()-interval '1 day',null);

-- ---------------- 宠物 ----------------
INSERT INTO pets(id,customer_id,name,species,breed,gender,neutered,birthday,weight_g,color,chip_no,blood_type,avatar_text,note) VALUES
 (1,1,'布丁',1,'比熊犬',1,true,'2024-01-15',5200,'白色','15600218843','DEA 1.1+','布','对鸡肉轻微过敏·怕吹风机'),
 (2,2,'奥利奥',2,'英国短毛猫',1,true,'2023-03-10',4500,'银渐层','15600218844',null,'奥',null),
 (3,3,'团子',1,'金毛寻回犬',1,false,'2022-06-01',28000,'金色','15600218845',null,'团',null),
 (4,4,'可乐',1,'柯基',1,false,'2025-02-20',9000,'三色','15600218846',null,'可',null),
 (5,5,'Lucky',1,'泰迪',1,true,'2018-05-05',3800,'棕色','15600218847',null,'L',null),
 (6,6,'糯米',1,'萨摩耶',2,false,'2024-04-04',18000,'白色','15600218848',null,'糯',null),
 (7,1,'咪咪',2,'布偶猫',2,true,'2021-08-08',5000,'海豹双色','15600218849',null,'咪',null),
 (8,8,'公主',1,'贵宾',2,true,'2023-09-09',4200,'香槟','15600218850',null,'公',null);

INSERT INTO pet_health_records(pet_id,type,title,performed_at,next_due_at,operator_id,detail) VALUES
 (1,'vaccine','狂犬+八联','2026-03-12','2026-09-12',6,'完成'),
 (1,'deworm','体内外驱虫','2026-05-01',null,6,null),
 (1,'allergy','鸡肉轻微过敏',null,null,6,'避免鸡肉类食物');
INSERT INTO pet_weight_records(pet_id,weight_g,recorded_at) VALUES
 (1,5400,'2025-10-01'),(1,5300,'2025-12-01'),(1,5200,'2026-03-01'),(1,5200,'2026-05-28');

-- ---------------- 服务 ----------------
INSERT INTO service_categories(id,code,name,color,sort) VALUES
 (1,'beauty','美容','#E26B41',0),(2,'wash','洗护','#2A5C4D',1),
 (3,'medical','医疗','#5A83A6',2),(4,'boarding','寄养','#D99A28',3),(5,'retail','零售','#BC5A78',4);

INSERT INTO services(id,category_id,name,default_duration_min,default_price,requires_station) VALUES
 (1,1,'全套SPA·小型犬',90,26800,true),
 (2,1,'造型修剪',60,16800,true),
 (3,2,'基础洗护',45,8800,true),
 (4,3,'疫苗·八联',20,32000,true),
 (5,3,'体内外驱虫',15,12000,true),
 (6,3,'老年犬体检',30,28000,true);

INSERT INTO service_offerings(id,store_id,service_id,price,duration_min,bookable_online,status) VALUES
 (1,1,1,26800,90,true,1),(2,1,2,16800,60,true,1),(3,1,3,8800,45,true,1),
 (4,1,4,32000,20,false,1),(5,1,5,12000,15,true,1),(6,1,6,28000,30,true,1);

INSERT INTO stations(id,store_id,name,type,staff_user_id,color) VALUES
 (1,1,'美容A位','beauty',4,'#E26B41'),
 (2,1,'美容B位','beauty',5,'#BC5A78'),
 (3,1,'医疗室','medical',6,'#2A5C4D'),
 (4,1,'寄养区','boarding',7,'#D99A28');

-- ---------------- 寄养: 房型与笼位 ----------------
INSERT INTO room_types(id,store_id,code,name,price_per_night,capacity,sort) VALUES
 (1,1,'small','小型犬舍',8800,8,0),(2,1,'medium','中型犬舍',12800,6,1),
 (3,1,'large','大型犬舍',16800,4,2),(4,1,'cat','猫舍',9800,4,3),(5,1,'suite','豪华套间',28800,2,4);

-- 24 间笼位
INSERT INTO boarding_rooms(store_id,room_type_id,code,status,sort) VALUES
 (1,1,'S01','occupied',1),(1,1,'S02','occupied',2),(1,1,'S03','occupied',3),(1,1,'S04','occupied',4),
 (1,1,'S05','free',5),(1,1,'S06','occupied',6),(1,1,'S07','free',7),(1,1,'S08','cleaning',8),
 (1,2,'M01','occupied',1),(1,2,'M02','occupied',2),(1,2,'M03','occupied',3),(1,2,'M04','free',4),
 (1,2,'M05','occupied',5),(1,2,'M06','cleaning',6),
 (1,3,'L01','occupied',1),(1,3,'L02','occupied',2),(1,3,'L03','occupied',3),(1,3,'L04','free',4),
 (1,4,'C01','occupied',1),(1,4,'C02','occupied',2),(1,4,'C03','occupied',3),(1,4,'C04','occupied',4),
 (1,5,'P01','occupied',1),(1,5,'P02','occupied',2);

-- 在住寄养订单（演示 5 条）
INSERT INTO boarding_orders(id,store_id,customer_id,pet_id,room_id,room_type_snapshot,price_per_night,status,source,planned_check_in,planned_check_out,actual_check_in,remark) VALUES
 (1,1,1,1,(SELECT id FROM boarding_rooms WHERE code='S01' AND store_id=1),'small',8800,'checked_in',1, now()-interval '2 day', now()+interval '1 day', now()-interval '2 day','标准3晚'),
 (2,1,3,3,(SELECT id FROM boarding_rooms WHERE code='L01' AND store_id=1),'large',16800,'checked_in',1, now()-interval '1 day', now()+interval '2 day', now()-interval '1 day','豪华3晚'),
 (3,1,1,7,(SELECT id FROM boarding_rooms WHERE code='C02' AND store_id=1),'cat',9800,'checked_in',1, now()-interval '4 day', now(), now()-interval '4 day','今日退房'),
 (4,1,5,6,(SELECT id FROM boarding_rooms WHERE code='M01' AND store_id=1),'medium',12800,'checked_in',1, now()-interval '3 day', now()+interval '2 day', now()-interval '3 day','标准5晚'),
 (5,1,2,2,(SELECT id FROM boarding_rooms WHERE code='C01' AND store_id=1),'cat',9800,'checked_in',1, now()-interval '5 day', now()+interval '3 day', now()-interval '5 day','猫舍8晚');

INSERT INTO boarding_care_logs(boarding_order_id,store_id,task,status,done_at,operator_id) VALUES
 (1,1,'feeding','done',now()-interval '3 hour',7),(1,1,'walking','done',now()-interval '2 hour',7),
 (1,1,'medication','pending',null,null),(1,1,'photo','pending',null,null),
 (3,1,'feeding','done',now()-interval '4 hour',7),(3,1,'walking','done',now()-interval '3 hour',7),
 (3,1,'medication','done',now()-interval '2 hour',7),(3,1,'photo','done',now()-interval '1 hour',7);

-- ---------------- 商品与库存 ----------------
INSERT INTO product_categories(id,store_id,name,sort) VALUES (1,null,'主粮',0),(2,null,'猫砂',1),(3,null,'驱虫',2),(4,null,'零食',3);
INSERT INTO products(id,name,category_id,sku,unit,spec,price,cost,status) VALUES
 (1,'皇家幼犬粮 2kg',1,'RC-PUP-2KG','袋','2kg',16800,11000,1),
 (2,'猫砂·膨润土 10L',2,'CL-BEN-10L','袋','10L',4500,2800,1),
 (3,'驱虫滴剂·大型犬',3,'DW-LRG','支','单支',8800,5000,1),
 (4,'洁齿骨·鸡肉味',4,'SN-CHK','包','100g',2900,1500,1);
INSERT INTO inventory(store_id,product_id,quantity,safety_stock) VALUES
 (1,1,6,8),(1,2,3,5),(1,3,8,10),(1,4,12,6);
INSERT INTO stock_transactions(store_id,product_id,type,quantity,balance_after,operator_id,remark,created_at) VALUES
 (1,1,'purchase_in',20,26,8,'初始入库', now()-interval '10 day'),
 (1,1,'sale_out',-20,6,3,'累计销售', now()-interval '1 day'),
 (1,2,'purchase_in',15,18,8,'初始入库', now()-interval '10 day'),
 (1,2,'sale_out',-15,3,3,'累计销售', now()-interval '1 day');
-- 注：系统取"最新库存流水"时应以 (created_at, id) 倒序，id 作并发同刻兜底。

-- ---------------- 今日预约（演示） ----------------
INSERT INTO appointments(id,store_id,customer_id,pet_id,source,status,scheduled_start,scheduled_end,station_id,staff_user_id,total_amount) VALUES
 (1,1,1,1,1,'in_progress',date_trunc('day',now())+interval '9 hour 30 min',date_trunc('day',now())+interval '11 hour',1,4,26800),
 (2,1,2,2,1,'arrived',date_trunc('day',now())+interval '10 hour 15 min',date_trunc('day',now())+interval '10 hour 35 min',3,6,32000),
 (3,1,4,4,1,'pending',date_trunc('day',now())+interval '11 hour',date_trunc('day',now())+interval '11 hour 45 min',2,5,8800),
 (4,1,6,6,2,'pending',date_trunc('day',now())+interval '16 hour',date_trunc('day',now())+interval '17 hour',1,4,16800);
INSERT INTO appointment_items(appointment_id,service_offering_id,service_name,price,duration_min,station_id) VALUES
 (1,1,'全套SPA·小型犬',26800,90,1),(2,4,'疫苗·八联',32000,20,3),
 (3,3,'基础洗护',8800,45,2),(4,2,'造型修剪',16800,60,1);

-- ---------------- 已结算单（用于 Dashboard 营收，近几日） ----------------
INSERT INTO settlements(id,store_id,code,customer_id,biz_type,status,total_amount,discount_amount,paid_amount,operator_id,paid_at) VALUES
 (1,1,'S20260601001',1,'service','paid',26800,0,26800,3, now()-interval '2 hour'),
 (2,1,'S20260601002',2,'service','paid',32000,0,32000,3, now()-interval '1 hour'),
 (3,1,'S20260531001',3,'boarding','paid',50400,0,50400,3, now()-interval '1 day');
INSERT INTO settlement_items(settlement_id,source_type,source_id,name,unit_price,quantity,amount) VALUES
 (1,'appointment',1,'全套SPA·小型犬',26800,1,26800),
 (2,'appointment',2,'疫苗·八联',32000,1,32000),
 (3,'boarding_order',null,'寄养·3晚·大型犬舍',16800,3,50400);
INSERT INTO payments(settlement_id,method,amount,status,paid_at) VALUES
 (1,'wallet',26800,'success',now()-interval '2 hour'),
 (2,'cash',32000,'success',now()-interval '1 hour'),
 (3,'pos',50400,'success',now()-interval '1 day');

-- ---------------- 通知模板 ----------------
INSERT INTO notification_templates(code,channel,title,content) VALUES
 ('appointment_confirmed','inapp','预约成功','您在{storeName}的预约已确认：{serviceName} {time}'),
 ('appointment_confirmed','wechat_mp','预约成功','您在{storeName}的预约已确认：{serviceName} {time}'),
 ('visit_reminder','sms',null,'【爪迹】提醒：{petName}的{serviceName}预约将于{time}开始，请准时到店。'),
 ('vaccine_due','sms',null,'【爪迹】{petName}的{vaccineName}将于{dueDate}到期，请及时安排接种。'),
 ('vaccine_due','wechat_mp','疫苗到期提醒','{petName}的{vaccineName}将于{dueDate}到期'),
 ('boarding_checkout','inapp','寄养退房提醒','{petName}今日计划退房，请准备结算'),
 ('stock_low','inapp','库存预警','{productName} 库存仅剩 {quantity}{unit}，已低于安全库存');

-- ---------------- 系统设置 ----------------
INSERT INTO system_settings(store_id,key,value) VALUES
 (null,'feature.sms_enabled','false'),
 (null,'feature.wechat_enabled','false'),
 (null,'feature.online_booking_enabled','true'),
 (1,'store.business_hours','{"open":"09:00","close":"21:00"}'),
 (1,'boarding.checkout_rule','{"round":"ceil","min_nights":1,"apply_member_discount":false}'),
 (1,'appointment.cancel_deadline_hours','2'),
 (1,'appointment.visit_reminder_hours','24'),
 (1,'pet.vaccine_remind_days','7'),
 (1,'inventory.allow_negative','false'),
 (1,'member.allow_downgrade','false'),
 (1,'member.churn_days','30'),
 (1,'points.rule','{"per_yuan":1,"by_tier_rate":true,"recharge_earn":false}');

-- ---------------- 重置序列 ----------------
SELECT setval(pg_get_serial_sequence('stores','id'), (SELECT max(id) FROM stores));
SELECT setval(pg_get_serial_sequence('users','id'), (SELECT max(id) FROM users));
SELECT setval(pg_get_serial_sequence('roles','id'), (SELECT max(id) FROM roles));
SELECT setval(pg_get_serial_sequence('permissions','id'), (SELECT max(id) FROM permissions));
SELECT setval(pg_get_serial_sequence('membership_tiers','id'), (SELECT max(id) FROM membership_tiers));
SELECT setval(pg_get_serial_sequence('customers','id'), (SELECT max(id) FROM customers));
SELECT setval(pg_get_serial_sequence('pets','id'), (SELECT max(id) FROM pets));
SELECT setval(pg_get_serial_sequence('service_categories','id'), (SELECT max(id) FROM service_categories));
SELECT setval(pg_get_serial_sequence('services','id'), (SELECT max(id) FROM services));
SELECT setval(pg_get_serial_sequence('service_offerings','id'), (SELECT max(id) FROM service_offerings));
SELECT setval(pg_get_serial_sequence('stations','id'), (SELECT max(id) FROM stations));
SELECT setval(pg_get_serial_sequence('room_types','id'), (SELECT max(id) FROM room_types));
SELECT setval(pg_get_serial_sequence('boarding_rooms','id'), (SELECT max(id) FROM boarding_rooms));
SELECT setval(pg_get_serial_sequence('boarding_orders','id'), (SELECT max(id) FROM boarding_orders));
SELECT setval(pg_get_serial_sequence('product_categories','id'), (SELECT max(id) FROM product_categories));
SELECT setval(pg_get_serial_sequence('products','id'), (SELECT max(id) FROM products));
SELECT setval(pg_get_serial_sequence('appointments','id'), (SELECT max(id) FROM appointments));
SELECT setval(pg_get_serial_sequence('settlements','id'), (SELECT max(id) FROM settlements));
