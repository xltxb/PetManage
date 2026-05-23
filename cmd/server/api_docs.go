package main

import (
	"encoding/json"
	"net/http"
)

// APIDocParam represents a parameter in an API endpoint.
type APIDocParam struct {
	Name        string `json:"name"`
	In          string `json:"in"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

// APIDocExample is an example request or response body.
type APIDocExample struct {
	ContentType string      `json:"content_type"`
	Example     interface{} `json:"example"`
}

// APIDocResponse represents a possible response.
type APIDocResponse struct {
	Status      int         `json:"status"`
	Description string      `json:"description"`
	Example     interface{} `json:"example,omitempty"`
}

// APIDocEndpoint describes a single API endpoint.
type APIDocEndpoint struct {
	Method      string           `json:"method"`
	Path        string           `json:"path"`
	Summary     string           `json:"summary"`
	Description string           `json:"description"`
	Auth        string           `json:"auth"`
	Params      []APIDocParam    `json:"params,omitempty"`
	RequestBody *APIDocExample   `json:"request_body,omitempty"`
	Responses   []APIDocResponse `json:"responses"`
}

// APIDocModule groups related endpoints.
type APIDocModule struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Endpoints   []APIDocEndpoint `json:"endpoints"`
}

// APIDoc is the top-level documentation structure.
type APIDoc struct {
	Version string        `json:"version"`
	Title   string        `json:"title"`
	Modules []APIDocModule `json:"modules"`
}

func getAPIDocs() APIDoc {
	return APIDoc{
		Version: "1.0.0",
		Title:   "宠物店管理系统 API 文档",
		Modules: []APIDocModule{
			{
				Name:        "认证与用户",
				Description: "平台管理员和商户管理员的登录、Token刷新、密码修改",
				Endpoints: []APIDocEndpoint{
					{
						Method: "POST", Path: "/api/v1/auth/login", Summary: "平台管理员登录",
						Description: "使用平台管理员账号密码登录，返回JWT access_token和refresh_token。",
						Auth: "无", Params: []APIDocParam{
							{Name: "username", In: "body", Type: "string", Required: true, Description: "用户名"},
							{Name: "password", In: "body", Type: "string", Required: true, Description: "密码"},
						},
						RequestBody: &APIDocExample{ContentType: "application/json", Example: map[string]string{"username": "admin", "password": "admin123"}},
						Responses: []APIDocResponse{
							{Status: 200, Description: "登录成功", Example: map[string]interface{}{"access_token": "eyJ...", "refresh_token": "eyJ...", "expires_in": 7200, "token_type": "Bearer", "must_change_password": false}},
							{Status: 401, Description: "密码错误", Example: map[string]string{"code": "INVALID_CREDENTIALS", "message": "invalid username or password"}},
						},
					},
					{
						Method: "POST", Path: "/api/v1/auth/refresh", Summary: "刷新Token",
						Description: "使用refresh_token换取新的access_token。",
						Auth: "无", Params: []APIDocParam{
							{Name: "refresh_token", In: "body", Type: "string", Required: true, Description: "刷新令牌"},
						},
						RequestBody: &APIDocExample{ContentType: "application/json", Example: map[string]string{"refresh_token": "eyJ..."}},
						Responses: []APIDocResponse{
							{Status: 200, Description: "刷新成功", Example: map[string]interface{}{"access_token": "eyJ...", "refresh_token": "eyJ...", "expires_in": 7200}},
							{Status: 401, Description: "Token过期", Example: map[string]string{"code": "TOKEN_EXPIRED", "message": "token has expired"}},
						},
					},
					{
						Method: "POST", Path: "/api/v1/auth/change-password", Summary: "修改密码",
						Description: "修改当前登录用户的密码，需要验证旧密码。",
						Auth: "Bearer Token",
						Params: []APIDocParam{
							{Name: "old_password", In: "body", Type: "string", Required: true, Description: "旧密码"},
							{Name: "new_password", In: "body", Type: "string", Required: true, Description: "新密码（至少6位）"},
						},
						RequestBody: &APIDocExample{ContentType: "application/json", Example: map[string]string{"old_password": "admin123", "new_password": "newPass123"}},
						Responses: []APIDocResponse{
							{Status: 200, Description: "修改成功", Example: map[string]string{"message": "password changed successfully"}},
							{Status: 401, Description: "旧密码错误", Example: map[string]string{"code": "INVALID_CREDENTIALS", "message": "invalid old password"}},
						},
					},
					{
						Method: "POST", Path: "/api/v1/merchant/auth/login", Summary: "商户管理员登录",
						Description: "商户端独立登录接口，仅允许商户管理员账号登录。密码错误3次锁定15分钟。",
						Auth: "无", Params: []APIDocParam{
							{Name: "username", In: "body", Type: "string", Required: true, Description: "商户管理员用户名"},
							{Name: "password", In: "body", Type: "string", Required: true, Description: "密码"},
						},
						RequestBody: &APIDocExample{ContentType: "application/json", Example: map[string]string{"username": "m_SC20240001", "password": "abc123"}},
						Responses: []APIDocResponse{
							{Status: 200, Description: "登录成功", Example: map[string]interface{}{"access_token": "eyJ...", "refresh_token": "eyJ...", "expires_in": 7200, "merchant_name": "月亮宠物医院", "display_name": "张三"}},
							{Status: 429, Description: "账号锁定", Example: map[string]string{"code": "ACCOUNT_LOCKED", "message": "account locked due to too many failed attempts, try again in 15 minutes"}},
							{Status: 403, Description: "商户已冻结/关停", Example: map[string]string{"code": "MERCHANT_FROZEN", "message": "merchant is frozen"}},
						},
					},
				},
			},
			{
				Name: "商户管理", Description: "商户入驻申请、审核、列表、状态管控",
				Endpoints: []APIDocEndpoint{
					{
						Method: "POST", Path: "/api/v1/merchants/apply", Summary: "提交入驻申请",
						Description: "商户在线提交入驻信息，包含商户名称、营业执照号、法人、联系方式、地址。",
						Auth: "无",
						Params: []APIDocParam{
							{Name: "name", In: "body", Type: "string", Required: true, Description: "商户名称"},
							{Name: "license_number", In: "body", Type: "string", Required: true, Description: "营业执照号"},
							{Name: "legal_person", In: "body", Type: "string", Required: true, Description: "法人姓名"},
							{Name: "contact_phone", In: "body", Type: "string", Required: true, Description: "联系电话"},
							{Name: "address", In: "body", Type: "string", Required: true, Description: "门店地址"},
						},
						RequestBody: &APIDocExample{ContentType: "application/json", Example: map[string]string{"name": "星星宠物店", "license_number": "SC20240001", "legal_person": "李四", "contact_phone": "13800000001", "address": "北京市朝阳区xxx路1号"}},
						Responses: []APIDocResponse{
							{Status: 200, Description: "提交成功", Example: map[string]interface{}{"id": 1, "status": "pending"}},
							{Status: 400, Description: "缺少必填字段", Example: map[string]interface{}{"code": "INVALID_PARAMS", "message": "missing required fields: license_number, legal_person"}},
							{Status: 409, Description: "营业执照号重复", Example: map[string]string{"code": "DUPLICATE_LICENSE", "message": "license number already exists: SC20240001"}},
						},
					},
					{
						Method: "GET", Path: "/api/v1/merchants/apply/{id}", Summary: "查询申请状态",
						Description: "查询入驻申请的审核状态和详情。", Auth: "无",
						Params: []APIDocParam{{Name: "id", In: "path", Type: "integer", Required: true, Description: "申请ID"}},
						Responses: []APIDocResponse{
							{Status: 200, Description: "查询成功", Example: map[string]interface{}{"id": 1, "name": "星星宠物店", "status": "pending", "created_at": "2026-05-23T10:00:00Z"}},
							{Status: 404, Description: "申请不存在", Example: map[string]string{"code": "NOT_FOUND", "message": "application not found"}},
						},
					},
					{
						Method: "GET", Path: "/api/v1/merchants", Summary: "商户列表",
						Description: "平台管理员查看全平台商户列表，支持关键字搜索、状态筛选、分页。",
						Auth: "Bearer Token + 平台用户",
						Params: []APIDocParam{
							{Name: "keyword", In: "query", Type: "string", Required: false, Description: "搜索关键字（商户名称模糊匹配）"},
							{Name: "status", In: "query", Type: "string", Required: false, Description: "状态筛选: pending/approved/rejected/frozen/closed"},
							{Name: "page", In: "query", Type: "integer", Required: false, Description: "页码（默认1）"},
							{Name: "page_size", In: "query", Type: "integer", Required: false, Description: "每页条数（默认20，最大100）"},
						},
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"merchants": []interface{}{}, "total": 20, "page": 1, "page_size": 20}},
						},
					},
					{
						Method: "GET", Path: "/api/v1/merchants/pending", Summary: "待审核列表",
						Description: "查看待审核的商户入驻申请，按申请时间排序。",
						Auth: "Bearer Token + 平台用户",
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"merchants": []interface{}{}}},
						},
					},
					{
						Method: "POST", Path: "/api/v1/merchants/{id}/approve", Summary: "通过审核",
						Description: "通过商户入驻审核，自动创建商户管理员账号（用户名: m_{license_number}）。",
						Auth: "Bearer Token + 平台用户 + merchant:manage",
						Params: []APIDocParam{{Name: "id", In: "path", Type: "integer", Required: true, Description: "商户ID"}},
						Responses: []APIDocResponse{
							{Status: 200, Description: "审核通过", Example: map[string]interface{}{"status": "approved", "username": "m_SC20240001", "password": "random123"}},
							{Status: 403, Description: "权限不足", Example: map[string]string{"code": "FORBIDDEN", "message": "insufficient permissions: merchant:manage required"}},
						},
					},
					{
						Method: "POST", Path: "/api/v1/merchants/{id}/reject", Summary: "驳回审核",
						Description: "驳回商户入驻申请，需填写驳回原因。",
						Auth: "Bearer Token + 平台用户",
						Params: []APIDocParam{
							{Name: "id", In: "path", Type: "integer", Required: true, Description: "商户ID"},
							{Name: "reason", In: "body", Type: "string", Required: true, Description: "驳回原因"},
						},
						RequestBody: &APIDocExample{ContentType: "application/json", Example: map[string]string{"reason": "营业执照信息不完整"}},
						Responses: []APIDocResponse{
							{Status: 200, Description: "驳回成功", Example: map[string]interface{}{"status": "rejected", "review_remark": "营业执照信息不完整"}},
						},
					},
					{
						Method: "POST", Path: "/api/v1/merchants/{id}/freeze", Summary: "冻结商户",
						Description: "冻结商户账号，冻结后该商户所有用户无法登录。",
						Auth: "Bearer Token + 平台用户",
						Params: []APIDocParam{{Name: "id", In: "path", Type: "integer", Required: true, Description: "商户ID"}},
						RequestBody: &APIDocExample{ContentType: "application/json", Example: map[string]string{"reason": "违规经营"}},
						Responses: []APIDocResponse{
							{Status: 200, Description: "冻结成功", Example: map[string]interface{}{"status": "frozen"}},
						},
					},
					{
						Method: "POST", Path: "/api/v1/merchants/{id}/unfreeze", Summary: "解冻商户",
						Description: "解冻已冻结的商户。",
						Auth: "Bearer Token + 平台用户",
						Params: []APIDocParam{{Name: "id", In: "path", Type: "integer", Required: true, Description: "商户ID"}},
						Responses: []APIDocResponse{{Status: 200, Description: "解冻成功", Example: map[string]interface{}{"status": "approved"}}},
					},
					{
						Method: "POST", Path: "/api/v1/merchants/{id}/close", Summary: "关停商户",
						Description: "永久关停商户（终态，不可恢复）。",
						Auth: "Bearer Token + 平台用户",
						Params: []APIDocParam{{Name: "id", In: "path", Type: "integer", Required: true, Description: "商户ID"}},
						Responses: []APIDocResponse{{Status: 200, Description: "关停成功", Example: map[string]interface{}{"status": "closed"}}},
					},
				},
			},
			{
				Name: "平台经营大盘", Description: "平台核心经营数据和商户经营分析",
				Endpoints: []APIDocEndpoint{
					{
						Method: "GET", Path: "/api/v1/dashboard/overview", Summary: "经营大盘概览",
						Description: "查看平台经营核心指标（商户总数、活跃商户、新增商户、新增会员等），支持时间维度筛选。",
						Auth: "Bearer Token + 平台用户",
						Params: []APIDocParam{
							{Name: "period", In: "query", Type: "string", Required: false, Description: "时间维度: today/week/month/year/all（默认all）"},
						},
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"metrics": []interface{}{map[string]interface{}{"label": "商户总数", "value": 20}, map[string]interface{}{"label": "活跃商户", "value": 11}}}},
						},
					},
					{
						Method: "GET", Path: "/api/v1/dashboard/merchant/{id}/analysis", Summary: "单商户经营分析",
						Description: "查看单个商户的详细经营数据：营收、订单、会员、商品销售排行、服务热度排行。",
						Auth: "Bearer Token + 平台用户",
						Params: []APIDocParam{
							{Name: "id", In: "path", Type: "integer", Required: true, Description: "商户ID"},
							{Name: "period", In: "query", Type: "string", Required: false, Description: "时间维度: today/week/month/year/all"},
						},
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"today_revenue": 381.00, "today_orders": 1, "today_new_members": 1}},
						},
					},
					{
						Method: "GET", Path: "/api/v1/dashboard/merchants/ranking", Summary: "商户营收排行",
						Description: "按营收金额降序排列所有已通过商户。",
						Auth: "Bearer Token + 平台用户",
						Params: []APIDocParam{
							{Name: "period", In: "query", Type: "string", Required: false, Description: "时间维度: today/week/month/year/all"},
						},
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"ranking": []interface{}{}}},
						},
					},
				},
			},
			{
				Name: "合同管理", Description: "商户合同上传、续签、到期提醒",
				Endpoints: []APIDocEndpoint{
					{
						Method: "POST", Path: "/api/v1/contracts/merchant/{id}", Summary: "上传合同",
						Description: "为商户上传合同文件（PDF/图片），设置有效期。使用multipart/form-data。",
						Auth: "Bearer Token + 平台用户",
						Params: []APIDocParam{
							{Name: "id", In: "path", Type: "integer", Required: true, Description: "商户ID"},
							{Name: "contract_number", In: "form", Type: "string", Required: true, Description: "合同编号"},
							{Name: "start_date", In: "form", Type: "string", Required: true, Description: "开始日期（YYYY-MM-DD）"},
							{Name: "end_date", In: "form", Type: "string", Required: true, Description: "结束日期（YYYY-MM-DD）"},
							{Name: "file", In: "form", Type: "file", Required: true, Description: "合同文件"},
						},
						Responses: []APIDocResponse{
							{Status: 200, Description: "上传成功", Example: map[string]interface{}{"id": 1, "status": "active"}},
						},
					},
					{
						Method: "GET", Path: "/api/v1/contracts/merchant/{id}", Summary: "合同列表",
						Description: "查看商户的所有合同历史。",
						Auth: "Bearer Token + 平台用户",
						Params: []APIDocParam{{Name: "id", In: "path", Type: "integer", Required: true, Description: "商户ID"}},
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"contracts": []interface{}{}}},
						},
					},
					{
						Method: "GET", Path: "/api/v1/contracts/merchant/{id}/current", Summary: "当前合同",
						Description: "查询商户当前生效的合同，含到期天数和到期提醒。",
						Auth: "Bearer Token + 平台用户",
						Params: []APIDocParam{{Name: "id", In: "path", Type: "integer", Required: true, Description: "商户ID"}},
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"days_remaining": 180, "expiry_reminder": false}},
						},
					},
					{
						Method: "POST", Path: "/api/v1/contracts/merchant/{id}/renew", Summary: "续签合同",
						Description: "上传新合同续签，旧合同标记为expired并记录关联关系。",
						Auth: "Bearer Token + 平台用户",
						Params: []APIDocParam{{Name: "id", In: "path", Type: "integer", Required: true, Description: "商户ID"}},
						Responses: []APIDocResponse{
							{Status: 200, Description: "续签成功", Example: map[string]interface{}{"id": 2, "prev_contract_id": 1, "status": "active"}},
						},
					},
				},
			},
			{
				Name: "数据字典", Description: "平台统一管理商品分类、宠物品种等基础数据",
				Endpoints: []APIDocEndpoint{
					{
						Method: "GET", Path: "/api/v1/dict/categories", Summary: "分类树",
						Description: "获取分类树形结构（一级+二级）。",
						Auth: "Bearer Token + 平台用户",
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"categories": []interface{}{map[string]interface{}{"id": 1, "name": "主粮", "level": 1, "children": []interface{}{}}}}},
						},
					},
					{
						Method: "POST", Path: "/api/v1/dict/categories", Summary: "创建分类",
						Description: "创建平台级或商户级数据分类。",
						Auth: "Bearer Token + 平台用户 + dict:manage",
						Params: []APIDocParam{
							{Name: "name", In: "body", Type: "string", Required: true, Description: "分类名称"},
							{Name: "parent_id", In: "body", Type: "integer", Required: false, Description: "父分类ID（创建二级分类时必填）"},
						},
						Responses: []APIDocResponse{{Status: 200, Description: "创建成功", Example: map[string]interface{}{"id": 1, "name": "主粮", "level": 1}}},
					},
					{
						Method: "GET", Path: "/api/v1/dict/breeds", Summary: "品种列表",
						Description: "查询宠物品种列表，支持按宠物类型筛选。",
						Auth: "Bearer Token + 平台用户",
						Params: []APIDocParam{
							{Name: "pet_type", In: "query", Type: "string", Required: false, Description: "宠物类型: dog/cat"},
						},
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"breeds": []interface{}{map[string]interface{}{"id": 1, "name": "金毛", "pet_type": "dog"}}}},
						},
					},
				},
			},
			{
				Name: "平台角色与权限", Description: "角色CRUD、权限分配、平台用户管理",
				Endpoints: []APIDocEndpoint{
					{
						Method: "GET", Path: "/api/v1/platform/permissions", Summary: "可用权限列表",
						Description: "获取所有可分配的权限点列表。",
						Auth: "Bearer Token + 平台用户",
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"permissions": []interface{}{map[string]interface{}{"code": "merchant:view", "name": "商户查看", "category": "商户管理"}}}},
						},
					},
					{
						Method: "GET", Path: "/api/v1/platform/roles", Summary: "角色列表",
						Description: "查询所有平台角色。",
						Auth: "Bearer Token + 平台用户",
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"roles": []interface{}{map[string]interface{}{"id": 1, "name": "超级管理员", "code": "super_admin"}}}},
						},
					},
					{
						Method: "POST", Path: "/api/v1/platform/roles", Summary: "创建角色",
						Description: "创建新的平台角色并配置权限。",
						Auth: "Bearer Token + 平台用户",
						Params: []APIDocParam{
							{Name: "name", In: "body", Type: "string", Required: true, Description: "角色名称"},
							{Name: "code", In: "body", Type: "string", Required: true, Description: "角色编码（唯一）"},
							{Name: "permissions", In: "body", Type: "[]string", Required: true, Description: "权限列表"},
						},
						Responses: []APIDocResponse{{Status: 200, Description: "创建成功", Example: map[string]interface{}{"id": 3, "name": "运营专员", "permissions": []string{"merchant:view", "merchant:manage"}}}},
					},
					{
						Method: "GET", Path: "/api/v1/platform/users", Summary: "平台用户列表",
						Description: "查询平台用户列表。",
						Auth: "Bearer Token + 平台用户",
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"users": []interface{}{}}},
						},
					},
					{
						Method: "POST", Path: "/api/v1/platform/users", Summary: "创建平台用户",
						Description: "创建新的平台用户账号。",
						Auth: "Bearer Token + 平台用户",
						Params: []APIDocParam{
							{Name: "username", In: "body", Type: "string", Required: true, Description: "用户名"},
							{Name: "phone", In: "body", Type: "string", Required: true, Description: "手机号"},
							{Name: "password", In: "body", Type: "string", Required: true, Description: "密码"},
							{Name: "role_id", In: "body", Type: "integer", Required: true, Description: "角色ID"},
						},
						Responses: []APIDocResponse{{Status: 200, Description: "创建成功", Example: map[string]interface{}{"id": 2, "username": "ops_user1"}}},
					},
					{
						Method: "PUT", Path: "/api/v1/platform/users/{id}/role", Summary: "分配角色",
						Description: "为平台用户分配/更改角色。",
						Auth: "Bearer Token + 平台用户",
						Params: []APIDocParam{
							{Name: "id", In: "path", Type: "integer", Required: true, Description: "用户ID"},
							{Name: "role_id", In: "body", Type: "integer", Required: true, Description: "角色ID"},
						},
						Responses: []APIDocResponse{{Status: 200, Description: "成功", Example: map[string]string{"message": "role assigned"}}},
					},
				},
			},
			{
				Name: "系统公告", Description: "公告发布、定向推送、已读/未读追踪",
				Endpoints: []APIDocEndpoint{
					{
						Method: "GET", Path: "/api/v1/announcements", Summary: "公告列表（平台端）",
						Description: "平台端查看所有公告，含已读/未读统计。",
						Auth: "Bearer Token + 平台用户 + announcement:view",
						Params: []APIDocParam{
							{Name: "page", In: "query", Type: "integer", Required: false, Description: "页码"},
							{Name: "page_size", In: "query", Type: "integer", Required: false, Description: "每页条数"},
						},
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"announcements": []interface{}{}, "total": 5}},
						},
					},
					{
						Method: "POST", Path: "/api/v1/announcements", Summary: "创建公告",
						Description: "创建公告，支持全平台/定向推送和定时发布。",
						Auth: "Bearer Token + 平台用户 + announcement:manage",
						Params: []APIDocParam{
							{Name: "title", In: "body", Type: "string", Required: true, Description: "标题"},
							{Name: "content", In: "body", Type: "string", Required: true, Description: "正文"},
							{Name: "scope", In: "body", Type: "string", Required: true, Description: "推送范围: all/merchants"},
							{Name: "merchant_ids", In: "body", Type: "[]integer", Required: false, Description: "定向推送的商户ID列表"},
							{Name: "publish_at", In: "body", Type: "string", Required: false, Description: "定时发布时间（ISO8601）"},
						},
						Responses: []APIDocResponse{{Status: 200, Description: "创建成功", Example: map[string]interface{}{"id": 1, "title": "系统维护通知"}}},
					},
					{
						Method: "GET", Path: "/api/v1/merchant/announcements", Summary: "公告列表（商户端）",
						Description: "商户端查看推送给自己且已发布的公告。",
						Auth: "Bearer Token（商户）",
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"announcements": []interface{}{}}},
						},
					},
					{
						Method: "GET", Path: "/api/v1/merchant/announcements/unread-count", Summary: "未读公告数",
						Description: "获取当前商户的未读公告计数。",
						Auth: "Bearer Token（商户）",
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"unread_count": 3}},
						},
					},
					{
						Method: "POST", Path: "/api/v1/merchant/announcements/{id}/read", Summary: "标记已读",
						Description: "将公告标记为已读。",
						Auth: "Bearer Token（商户）",
						Params: []APIDocParam{{Name: "id", In: "path", Type: "integer", Required: true, Description: "公告ID"}},
						Responses: []APIDocResponse{{Status: 200, Description: "成功", Example: map[string]string{"message": "marked as read"}}},
					},
				},
			},
			{
				Name: "操作日志", Description: "全平台操作行为记录与查询",
				Endpoints: []APIDocEndpoint{
					{
						Method: "GET", Path: "/api/v1/operation-logs", Summary: "操作日志查询",
						Description: "查询平台操作日志，支持按用户、操作类型、时间范围筛选和分页。",
						Auth: "Bearer Token + 平台用户",
						Params: []APIDocParam{
							{Name: "user_id", In: "query", Type: "integer", Required: false, Description: "操作人ID"},
							{Name: "action", In: "query", Type: "string", Required: false, Description: "操作类型"},
							{Name: "target_type", In: "query", Type: "string", Required: false, Description: "目标类型"},
							{Name: "start_time", In: "query", Type: "string", Required: false, Description: "开始时间（ISO8601）"},
							{Name: "end_time", In: "query", Type: "string", Required: false, Description: "结束时间（ISO8601）"},
							{Name: "page", In: "query", Type: "integer", Required: false, Description: "页码"},
							{Name: "page_size", In: "query", Type: "integer", Required: false, Description: "每页条数"},
						},
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"logs": []interface{}{}, "total": 50, "page": 1, "page_size": 20}},
						},
					},
					{
						Method: "GET", Path: "/api/v1/operation-logs/merchant/{id}", Summary: "商户操作日志",
						Description: "查询指定商户的所有操作日志。",
						Auth: "Bearer Token + 平台用户",
						Params: []APIDocParam{{Name: "id", In: "path", Type: "integer", Required: true, Description: "商户ID"}},
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"logs": []interface{}{}}},
						},
					},
				},
			},
			{
				Name: "风控管理", Description: "交易风控规则配置和预警管理",
				Endpoints: []APIDocEndpoint{
					{
						Method: "GET", Path: "/api/v1/risk/rules", Summary: "风控规则列表",
						Description: "查询所有风控规则。",
						Auth: "Bearer Token + 平台用户 + risk:view",
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"rules": []interface{}{}}},
						},
					},
					{
						Method: "POST", Path: "/api/v1/risk/rules", Summary: "创建风控规则",
						Description: "创建新的风控预警规则。",
						Auth: "Bearer Token + 平台用户 + risk:manage",
						Params: []APIDocParam{
							{Name: "name", In: "body", Type: "string", Required: true, Description: "规则名称"},
							{Name: "rule_type", In: "body", Type: "string", Required: true, Description: "类型: large_refund/high_frequency"},
							{Name: "threshold_amount", In: "body", Type: "number", Required: false, Description: "金额阈值（分）"},
							{Name: "threshold_count", In: "body", Type: "integer", Required: false, Description: "笔数阈值"},
							{Name: "time_window_minutes", In: "body", Type: "integer", Required: false, Description: "时间窗口（分钟）"},
						},
						Responses: []APIDocResponse{{Status: 200, Description: "创建成功", Example: map[string]interface{}{"id": 1}}},
					},
					{
						Method: "GET", Path: "/api/v1/risk/alerts", Summary: "预警列表",
						Description: "查询风控预警记录，支持按商户、类型、状态筛选。",
						Auth: "Bearer Token + 平台用户 + risk:view",
						Params: []APIDocParam{
							{Name: "merchant_id", In: "query", Type: "integer", Required: false, Description: "商户ID"},
							{Name: "alert_type", In: "query", Type: "string", Required: false, Description: "预警类型"},
							{Name: "status", In: "query", Type: "string", Required: false, Description: "状态: pending/processed/ignored"},
							{Name: "page", In: "query", Type: "integer", Required: false, Description: "页码"},
							{Name: "page_size", In: "query", Type: "integer", Required: false, Description: "每页条数"},
						},
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"alerts": []interface{}{}, "total": 0}},
						},
					},
				},
			},
			{
				Name: "投诉工单", Description: "投诉工单创建、分配、处理、统计",
				Endpoints: []APIDocEndpoint{
					{
						Method: "POST", Path: "/api/v1/complaints", Summary: "创建投诉工单",
						Description: "创建新的投诉工单，关联商户和投诉类型。",
						Auth: "Bearer Token", Params: []APIDocParam{
							{Name: "merchant_id", In: "body", Type: "integer", Required: true, Description: "商户ID"},
							{Name: "complaint_type", In: "body", Type: "string", Required: true, Description: "类型: service/product/staff/pricing/other"},
							{Name: "description", In: "body", Type: "string", Required: true, Description: "投诉内容"},
						},
						Responses: []APIDocResponse{{Status: 200, Description: "创建成功", Example: map[string]interface{}{"id": 1, "status": "pending"}}},
					},
					{
						Method: "GET", Path: "/api/v1/complaints", Summary: "工单列表",
						Description: "查询投诉工单列表，支持多维度筛选。",
						Auth: "Bearer Token + 平台用户 + complaint:view",
						Params: []APIDocParam{
							{Name: "merchant_id", In: "query", Type: "integer", Required: false, Description: "商户ID"},
							{Name: "status", In: "query", Type: "string", Required: false, Description: "状态筛选"},
							{Name: "page", In: "query", Type: "integer", Required: false, Description: "页码"},
						},
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"tickets": []interface{}{}, "total": 0}},
						},
					},
					{
						Method: "GET", Path: "/api/v1/complaints/stats", Summary: "投诉统计",
						Description: "投诉工单统计数据（总数、各状态计数、商户投诉率排行）。",
						Auth: "Bearer Token + 平台用户 + complaint:view",
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"total": 3, "resolved": 1, "pending": 1}},
						},
					},
				},
			},
			{
				Name: "数据报表", Description: "经营报表和交易报表Excel导出",
				Endpoints: []APIDocEndpoint{
					{
						Method: "GET", Path: "/api/v1/reports/operating", Summary: "导出经营报表",
						Description: "导出全平台商户经营概况Excel报表（商户名称、营业执照号、状态、营收、订单、会员）。",
						Auth: "Bearer Token + 平台用户",
						Params: []APIDocParam{
							{Name: "start_time", In: "query", Type: "string", Required: false, Description: "开始时间"},
							{Name: "end_time", In: "query", Type: "string", Required: false, Description: "结束时间"},
						},
						Responses: []APIDocResponse{
							{Status: 200, Description: "Excel文件下载", Example: "（二进制Excel文件）"},
						},
					},
					{
						Method: "GET", Path: "/api/v1/reports/transactions", Summary: "导出交易报表",
						Description: "导出交易明细Excel报表（订单编号、商户名称、商品明细、金额、支付方式）。",
						Auth: "Bearer Token + 平台用户",
						Params: []APIDocParam{
							{Name: "start_time", In: "query", Type: "string", Required: false, Description: "开始时间"},
							{Name: "end_time", In: "query", Type: "string", Required: false, Description: "结束时间"},
						},
						Responses: []APIDocResponse{
							{Status: 200, Description: "Excel文件下载", Example: "（二进制Excel文件）"},
						},
					},
				},
			},
			{
				Name: "商户经营看板", Description: "商户端首页经营数据和快捷入口",
				Endpoints: []APIDocEndpoint{
					{
						Method: "GET", Path: "/api/v1/merchant/dashboard", Summary: "商户经营看板",
						Description: "获取当前商户今日核心指标（营收、订单、新增会员、预约、服务完成量）和预警信息。",
						Auth: "Bearer Token（商户）",
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"today_revenue": 0, "today_orders": 0, "today_new_members": 1, "today_bookings": 0, "today_services": 0}},
							{Status: 403, Description: "非商户用户", Example: map[string]string{"code": "FORBIDDEN", "message": "merchant access required"}},
						},
					},
				},
			},
			{
				Name: "店铺设置", Description: "商户店铺信息、Logo、营业时间维护",
				Endpoints: []APIDocEndpoint{
					{
						Method: "GET", Path: "/api/v1/merchant/shop-settings", Summary: "获取店铺设置",
						Description: "获取当前商户的店铺基础信息。",
						Auth: "Bearer Token（商户）",
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"name": "星星宠物店", "address": "北京市朝阳区", "contact_phone": "13800000001"}},
						},
					},
					{
						Method: "PUT", Path: "/api/v1/merchant/shop-settings", Summary: "更新店铺设置",
						Description: "更新店铺名称、联系方式、地址、营业时间、公告等。",
						Auth: "Bearer Token（商户）",
						Params: []APIDocParam{
							{Name: "name", In: "body", Type: "string", Required: true, Description: "店铺名称"},
							{Name: "address", In: "body", Type: "string", Required: false, Description: "地址"},
							{Name: "contact_phone", In: "body", Type: "string", Required: false, Description: "联系电话"},
							{Name: "contact_email", In: "body", Type: "string", Required: false, Description: "联系邮箱"},
							{Name: "business_hours", In: "body", Type: "string", Required: false, Description: "营业时间"},
							{Name: "notice", In: "body", Type: "string", Required: false, Description: "门店公告"},
						},
						Responses: []APIDocResponse{
							{Status: 200, Description: "更新成功", Example: map[string]string{"message": "settings updated"}},
							{Status: 400, Description: "缺少必填字段", Example: map[string]string{"code": "INVALID_PARAMS", "message": "store name is required"}},
						},
					},
					{
						Method: "POST", Path: "/api/v1/merchant/shop-settings/logo", Summary: "上传Logo",
						Description: "上传店铺Logo图片（multipart/form-data）。",
						Auth: "Bearer Token（商户）",
						Responses: []APIDocResponse{
							{Status: 200, Description: "上传成功", Example: map[string]interface{}{"logo_url": "/uploads/logos/xxx.png"}},
						},
					},
				},
			},
			{
				Name: "商品管理", Description: "商品CRUD、多规格管理(SKU)",
				Endpoints: []APIDocEndpoint{
					{
						Method: "GET", Path: "/api/v1/merchant/products", Summary: "商品列表",
						Description: "查询当前商户的商品列表，支持关键字搜索和状态筛选。",
						Auth: "Bearer Token（商户）",
						Params: []APIDocParam{
							{Name: "keyword", In: "query", Type: "string", Required: false, Description: "搜索关键字（条码/名称）"},
							{Name: "status", In: "query", Type: "string", Required: false, Description: "状态: active/inactive"},
							{Name: "page", In: "query", Type: "integer", Required: false, Description: "页码"},
							{Name: "page_size", In: "query", Type: "integer", Required: false, Description: "每页条数"},
						},
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"products": []interface{}{}, "total": 10, "page": 1, "page_size": 20}},
						},
					},
					{
						Method: "POST", Path: "/api/v1/merchant/products", Summary: "创建商品",
						Description: "创建新的商品档案。",
						Auth: "Bearer Token（商户）",
						Params: []APIDocParam{
							{Name: "barcode", In: "body", Type: "string", Required: true, Description: "商品条码"},
							{Name: "name", In: "body", Type: "string", Required: true, Description: "商品名称"},
							{Name: "brand", In: "body", Type: "string", Required: false, Description: "品牌"},
							{Name: "specification", In: "body", Type: "string", Required: false, Description: "规格"},
							{Name: "price_cents", In: "body", Type: "integer", Required: true, Description: "售价（分）"},
							{Name: "cost_cents", In: "body", Type: "integer", Required: false, Description: "进价成本（分）"},
							{Name: "stock", In: "body", Type: "integer", Required: false, Description: "库存数量"},
							{Name: "alert_stock", In: "body", Type: "integer", Required: false, Description: "库存预警值"},
							{Name: "category_id", In: "body", Type: "integer", Required: false, Description: "分类ID"},
							{Name: "expiry_date", In: "body", Type: "string", Required: false, Description: "有效期"},
						},
						Responses: []APIDocResponse{{Status: 200, Description: "创建成功", Example: map[string]interface{}{"id": 1, "name": "皇家狗粮"}}},
					},
					{
						Method: "GET", Path: "/api/v1/merchant/products/{id}", Summary: "商品详情",
						Description: "获取商品详情（含SKU列表）。",
						Auth: "Bearer Token（商户）",
						Params: []APIDocParam{{Name: "id", In: "path", Type: "integer", Required: true, Description: "商品ID"}},
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"id": 1, "name": "皇家狗粮", "skus": []interface{}{}}},
						},
					},
					{
						Method: "PUT", Path: "/api/v1/merchant/products/{id}", Summary: "编辑商品",
						Description: "部分更新商品信息（仅更新非空字段）。",
						Auth: "Bearer Token（商户）",
						Params: []APIDocParam{{Name: "id", In: "path", Type: "integer", Required: true, Description: "商品ID"}},
						Responses: []APIDocResponse{{Status: 200, Description: "更新成功"}},
					},
					{
						Method: "DELETE", Path: "/api/v1/merchant/products/{id}", Summary: "删除商品",
						Description: "软删除商品。库存>0或有未完成订单时拒绝删除。",
						Auth: "Bearer Token（商户）",
						Responses: []APIDocResponse{
							{Status: 200, Description: "删除成功"},
							{Status: 400, Description: "无法删除", Example: map[string]string{"code": "INVALID_PARAMS", "message": "cannot delete product with existing stock"}},
						},
					},
					{
						Method: "POST", Path: "/api/v1/merchant/products/{id}/toggle-status", Summary: "上架/下架",
						Description: "切换商品的上架/下架状态。",
						Auth: "Bearer Token（商户）",
						Responses: []APIDocResponse{{Status: 200, Description: "成功", Example: map[string]interface{}{"status": "inactive"}}},
					},
					{
						Method: "POST", Path: "/api/v1/merchant/products/{id}/skus", Summary: "创建SKU",
						Description: "为商品创建规格变体（如口味/重量组合）。",
						Auth: "Bearer Token（商户）",
						Params: []APIDocParam{
							{Name: "sku_code", In: "body", Type: "string", Required: true, Description: "SKU编码"},
							{Name: "spec_info", In: "body", Type: "object", Required: true, Description: "规格信息JSON"},
							{Name: "price_cents", In: "body", Type: "integer", Required: true, Description: "SKU售价（分）"},
							{Name: "stock", In: "body", Type: "integer", Required: false, Description: "SKU库存"},
						},
						Responses: []APIDocResponse{{Status: 200, Description: "创建成功"}},
					},
				},
			},
			{
				Name: "商品分类", Description: "商户商品分类树形管理",
				Endpoints: []APIDocEndpoint{
					{
						Method: "GET", Path: "/api/v1/merchant/categories", Summary: "分类树",
						Description: "获取商户商品分类树形结构。",
						Auth: "Bearer Token（商户）",
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"categories": []interface{}{map[string]interface{}{"id": 1, "name": "主粮", "children": []interface{}{}}}}},
						},
					},
					{
						Method: "POST", Path: "/api/v1/merchant/categories", Summary: "创建分类",
						Description: "创建商品分类（支持一级和二级）。",
						Auth: "Bearer Token（商户）",
						Params: []APIDocParam{
							{Name: "name", In: "body", Type: "string", Required: true, Description: "分类名称"},
							{Name: "parent_id", In: "body", Type: "integer", Required: false, Description: "父分类ID"},
							{Name: "sort_order", In: "body", Type: "integer", Required: false, Description: "排序"},
						},
						Responses: []APIDocResponse{{Status: 200, Description: "创建成功", Example: map[string]interface{}{"id": 1}}},
					},
					{
						Method: "PUT", Path: "/api/v1/merchant/categories/{id}", Summary: "编辑分类",
						Description: "更新分类名称、父级或排序。",
						Auth: "Bearer Token（商户）",
						Responses: []APIDocResponse{{Status: 200, Description: "更新成功"}},
					},
					{
						Method: "DELETE", Path: "/api/v1/merchant/categories/{id}", Summary: "删除分类",
						Description: "软删除分类。有关联商品或子分类时拒绝删除。",
						Auth: "Bearer Token（商户）",
						Responses: []APIDocResponse{
							{Status: 200, Description: "删除成功"},
							{Status: 400, Description: "无法删除", Example: map[string]string{"code": "INVALID_PARAMS", "message": "category has linked products"}},
						},
					},
				},
			},
			{
				Name: "POS收银", Description: "收银开单、购物车计算、会员识别",
				Endpoints: []APIDocEndpoint{
					{
						Method: "POST", Path: "/api/v1/merchant/pos/cart/calculate", Summary: "购物车计算",
						Description: "计算购物车中商品/服务的价格（含会员折扣预览）。",
						Auth: "Bearer Token（商户）",
						Params: []APIDocParam{
							{Name: "items", In: "body", Type: "array", Required: true, Description: "商品/服务列表"},
							{Name: "member_id", In: "body", Type: "integer", Required: false, Description: "会员ID"},
						},
						RequestBody: &APIDocExample{ContentType: "application/json", Example: map[string]interface{}{
							"items":     []map[string]interface{}{{"product_id": 1, "quantity": 2}},
							"member_id": 1,
						}},
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"original_total": 39600, "discount": 1200, "payable": 38400}},
						},
					},
					{
						Method: "GET", Path: "/api/v1/merchant/pos/members/lookup", Summary: "手机号查会员",
						Description: "通过手机号查找会员信息。",
						Auth: "Bearer Token（商户）",
						Params: []APIDocParam{
							{Name: "phone", In: "query", Type: "string", Required: true, Description: "会员手机号"},
						},
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"id": 1, "name": "张三", "card_no": "M202605230001"}},
						},
					},
					{
						Method: "GET", Path: "/api/v1/merchant/pos/coupons/verify", Summary: "优惠券验证",
						Description: "验证优惠券码的有效性和抵扣金额。",
						Auth: "Bearer Token（商户）",
						Params: []APIDocParam{
							{Name: "code", In: "query", Type: "string", Required: true, Description: "优惠券码"},
						},
						Responses: []APIDocResponse{
							{Status: 200, Description: "验证成功", Example: map[string]interface{}{"valid": true, "discount_cents": 3000, "type": "fixed"}},
						},
					},
					{
						Method: "POST", Path: "/api/v1/merchant/checkout", Summary: "收银开单",
						Description: "提交订单并收款。支持商品+服务、多支付方式组合支付。",
						Auth: "Bearer Token（商户）",
						Params: []APIDocParam{
							{Name: "items", In: "body", Type: "array", Required: true, Description: "订单明细"},
							{Name: "member_id", In: "body", Type: "integer", Required: false, Description: "会员ID"},
							{Name: "payments", In: "body", Type: "array", Required: true, Description: "支付明细（method: cash/wechat/alipay/balance/points/coupon）"},
							{Name: "order_notes", In: "body", Type: "string", Required: false, Description: "订单备注"},
						},
						Responses: []APIDocResponse{
							{Status: 200, Description: "开单成功", Example: map[string]interface{}{"order_id": 1, "total_cents": 38100, "paid_cents": 38100, "status": "completed"}},
							{Status: 400, Description: "库存不足", Example: map[string]string{"code": "INVALID_PARAMS", "message": "insufficient stock"}},
						},
					},
				},
			},
			{
				Name: "订单管理", Description: "订单查询和退款处理",
				Endpoints: []APIDocEndpoint{
					{
						Method: "GET", Path: "/api/v1/merchant/orders", Summary: "订单列表",
						Description: "查询商户订单列表，支持按订单号、会员、日期、状态筛选。",
						Auth: "Bearer Token（商户）",
						Params: []APIDocParam{
							{Name: "order_no", In: "query", Type: "string", Required: false, Description: "订单号"},
							{Name: "member_id", In: "query", Type: "integer", Required: false, Description: "会员ID"},
							{Name: "status", In: "query", Type: "string", Required: false, Description: "订单状态"},
							{Name: "page", In: "query", Type: "integer", Required: false, Description: "页码"},
						},
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"orders": []interface{}{}, "total": 0}},
						},
					},
					{
						Method: "GET", Path: "/api/v1/merchant/orders/{id}", Summary: "订单详情",
						Description: "查看订单详情（商品明细、支付明细、优惠明细）。",
						Auth: "Bearer Token（商户）",
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"id": 1, "items": []interface{}{}, "payments": []interface{}{}}},
						},
					},
					{
						Method: "POST", Path: "/api/v1/merchant/orders/{id}/refund", Summary: "退款",
						Description: "对已完成订单发起退款（整单退款），库存自动恢复，并触发大额退款风控检查。",
						Auth: "Bearer Token（商户）",
						Responses: []APIDocResponse{
							{Status: 200, Description: "退款成功", Example: map[string]interface{}{"status": "refunded"}},
						},
					},
				},
			},
			{
				Name: "会员管理", Description: "会员开卡、档案、二维码、等级、储值、积分、标签",
				Endpoints: []APIDocEndpoint{
					{
						Method: "GET", Path: "/api/v1/merchant/members", Summary: "会员列表",
						Description: "查询会员列表，支持关键字搜索、状态筛选、分页。",
						Auth: "Bearer Token（商户）",
						Params: []APIDocParam{
							{Name: "keyword", In: "query", Type: "string", Required: false, Description: "搜索关键字（姓名/手机号）"},
							{Name: "status", In: "query", Type: "string", Required: false, Description: "状态: active/inactive"},
							{Name: "page", In: "query", Type: "integer", Required: false, Description: "页码"},
							{Name: "page_size", In: "query", Type: "integer", Required: false, Description: "每页条数"},
						},
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"members": []interface{}{}, "total": 10}},
						},
					},
					{
						Method: "POST", Path: "/api/v1/merchant/members", Summary: "创建会员",
						Description: "新建会员，自动生成会员卡号（M+年月日+4位流水号）。",
						Auth: "Bearer Token（商户）",
						Params: []APIDocParam{
							{Name: "name", In: "body", Type: "string", Required: true, Description: "姓名"},
							{Name: "phone", In: "body", Type: "string", Required: true, Description: "手机号"},
							{Name: "gender", In: "body", Type: "string", Required: false, Description: "性别: M/F/O"},
							{Name: "birthday", In: "body", Type: "string", Required: false, Description: "生日"},
							{Name: "address", In: "body", Type: "string", Required: false, Description: "地址"},
							{Name: "remark", In: "body", Type: "string", Required: false, Description: "备注"},
						},
						Responses: []APIDocResponse{
							{Status: 200, Description: "创建成功", Example: map[string]interface{}{"id": 1, "card_no": "M202605230001"}},
							{Status: 400, Description: "缺少必填字段", Example: map[string]string{"code": "INVALID_PARAMS", "message": "missing required fields: name, phone"}},
						},
					},
					{
						Method: "GET", Path: "/api/v1/merchant/members/{id}", Summary: "会员详情",
						Description: "查看会员完整信息（基础信息、宠物列表、储值余额、积分、消费记录）。",
						Auth: "Bearer Token（商户）",
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"id": 1, "name": "张三", "card_no": "M202605230001", "pets": []interface{}{}, "balance_cents": 0, "points": 0}},
						},
					},
					{
						Method: "PUT", Path: "/api/v1/merchant/members/{id}", Summary: "编辑会员",
						Description: "部分更新会员信息。",
						Auth: "Bearer Token（商户）",
						Responses: []APIDocResponse{{Status: 200, Description: "更新成功"}},
					},
					{
						Method: "POST", Path: "/api/v1/merchant/members/{id}/toggle-status", Summary: "启用/禁用",
						Description: "切换会员的active/inactive状态。",
						Auth: "Bearer Token（商户）",
						Responses: []APIDocResponse{{Status: 200, Description: "成功"}},
					},
					{
						Method: "POST", Path: "/api/v1/merchant/members/batch-import", Summary: "批量导入",
						Description: "批量导入会员（支持JSON数组或Excel文件）。",
						Auth: "Bearer Token（商户）",
						Responses: []APIDocResponse{
							{Status: 200, Description: "导入完成", Example: map[string]interface{}{"success": 2, "failed": 2, "errors": []interface{}{}},
							},
						},
					},
					{
						Method: "GET", Path: "/api/v1/merchant/members/{id}/qrcode", Summary: "会员二维码",
						Description: "生成会员二维码PNG图片（256x256），支持download=1参数下载。",
						Auth: "Bearer Token（商户）",
						Responses: []APIDocResponse{
							{Status: 200, Description: "PNG图片", Example: "（二进制PNG图片）"},
						},
					},
					{
						Method: "GET", Path: "/api/v1/merchant/members/qrcode/scan", Summary: "扫码识别",
						Description: "扫描会员二维码，验证后返回会员信息。",
						Auth: "Bearer Token（商户）",
						Params: []APIDocParam{
							{Name: "token", In: "query", Type: "string", Required: true, Description: "二维码Token"},
						},
						Responses: []APIDocResponse{
							{Status: 200, Description: "识别成功", Example: map[string]interface{}{"member_id": 1, "name": "张三", "card_no": "M202605230001"}},
						},
					},
				},
			},
			{
				Name: "宠物管理", Description: "宠物档案CRUD和健康提醒",
				Endpoints: []APIDocEndpoint{
					{
						Method: "POST", Path: "/api/v1/merchant/members/{id}/pets", Summary: "添加宠物",
						Description: "为会员添加宠物档案。",
						Auth: "Bearer Token（商户）",
						Params: []APIDocParam{
							{Name: "name", In: "body", Type: "string", Required: true, Description: "宠物名字"},
							{Name: "breed", In: "body", Type: "string", Required: false, Description: "品种"},
							{Name: "gender", In: "body", Type: "string", Required: true, Description: "性别: M/F"},
							{Name: "age", In: "body", Type: "integer", Required: false, Description: "年龄"},
							{Name: "weight", In: "body", Type: "string", Required: false, Description: "体重"},
						},
						Responses: []APIDocResponse{{Status: 200, Description: "创建成功", Example: map[string]interface{}{"id": 1, "name": "旺财"}}},
					},
					{
						Method: "GET", Path: "/api/v1/merchant/members/{id}/pets", Summary: "宠物列表",
						Description: "查询会员的所有宠物。",
						Auth: "Bearer Token（商户）",
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"pets": []interface{}{}}},
						},
					},
				},
			},
			{
				Name: "供应商管理", Description: "供应商档案CRUD和商品关联",
				Endpoints: []APIDocEndpoint{
					{
						Method: "GET", Path: "/api/v1/merchant/suppliers", Summary: "供应商列表",
						Description: "查询供应商列表，支持名称搜索和状态筛选。",
						Auth: "Bearer Token（商户）",
						Params: []APIDocParam{
							{Name: "keyword", In: "query", Type: "string", Required: false, Description: "供应商名称搜索"},
							{Name: "status", In: "query", Type: "string", Required: false, Description: "状态: active/inactive"},
							{Name: "page", In: "query", Type: "integer", Required: false, Description: "页码"},
						},
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"suppliers": []interface{}{}, "total": 5}},
						},
					},
					{
						Method: "POST", Path: "/api/v1/merchant/suppliers", Summary: "创建供应商",
						Description: "新建供应商档案。",
						Auth: "Bearer Token（商户）",
						Params: []APIDocParam{
							{Name: "name", In: "body", Type: "string", Required: true, Description: "供应商名称"},
							{Name: "contact_person", In: "body", Type: "string", Required: true, Description: "联系人"},
							{Name: "contact_phone", In: "body", Type: "string", Required: true, Description: "联系电话"},
						},
						Responses: []APIDocResponse{{Status: 200, Description: "创建成功"}},
					},
					{
						Method: "GET", Path: "/api/v1/merchant/suppliers/{id}", Summary: "供应商详情",
						Description: "查看供应商详情（含关联商品列表）。",
						Auth: "Bearer Token（商户）",
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"id": 1, "name": "伟嘉供应商", "products": []interface{}{}}},
						},
					},
					{
						Method: "POST", Path: "/api/v1/merchant/suppliers/{id}/products", Summary: "关联商品",
						Description: "将商品关联到供应商。",
						Auth: "Bearer Token（商户）",
						Params: []APIDocParam{
							{Name: "product_id", In: "body", Type: "integer", Required: true, Description: "商品ID"},
						},
						Responses: []APIDocResponse{{Status: 200, Description: "关联成功"}},
					},
				},
			},
			{
				Name: "采购管理", Description: "采购单全流程（草稿→提交→确认→入库）",
				Endpoints: []APIDocEndpoint{
					{
						Method: "GET", Path: "/api/v1/merchant/purchases", Summary: "采购单列表",
						Description: "查询采购单列表，支持状态筛选和分页。",
						Auth: "Bearer Token（商户）",
						Params: []APIDocParam{
							{Name: "status", In: "query", Type: "string", Required: false, Description: "状态: draft/submitted/confirmed/received/voided"},
							{Name: "page", In: "query", Type: "integer", Required: false, Description: "页码"},
						},
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"purchases": []interface{}{}, "total": 0}},
						},
					},
					{
						Method: "POST", Path: "/api/v1/merchant/purchases", Summary: "创建采购单",
						Description: "创建采购单草稿。",
						Auth: "Bearer Token（商户）",
						Params: []APIDocParam{
							{Name: "supplier_id", In: "body", Type: "integer", Required: true, Description: "供应商ID"},
							{Name: "items", In: "body", Type: "array", Required: true, Description: "采购商品列表"},
							{Name: "notes", In: "body", Type: "string", Required: false, Description: "备注"},
						},
						Responses: []APIDocResponse{{Status: 200, Description: "创建成功", Example: map[string]interface{}{"id": 1, "status": "draft"}}},
					},
					{
						Method: "POST", Path: "/api/v1/merchant/purchases/{id}/submit", Summary: "提交采购单",
						Description: "提交草稿状态采购单，状态变为待确认。",
						Auth: "Bearer Token（商户）",
					},
					{
						Method: "POST", Path: "/api/v1/merchant/purchases/{id}/receive", Summary: "确认入库",
						Description: "到货后确认入库，自动增加库存并生成库存流水。",
						Auth: "Bearer Token（商户）",
					},
				},
			},
			{
				Name: "库存管理", Description: "仓库管理、出入库、调拨、报损、盘点、预警",
				Endpoints: []APIDocEndpoint{
					{
						Method: "GET", Path: "/api/v1/merchant/inventory/alerts", Summary: "库存预警",
						Description: "获取低库存、临期、过期商品预警列表。",
						Auth: "Bearer Token（商户）",
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"alerts": []interface{}{}, "total": 0}},
						},
					},
					{
						Method: "POST", Path: "/api/v1/merchant/inventory/inbound", Summary: "手动入库",
						Description: "手动入库操作，增加商品库存并生成流水。",
						Auth: "Bearer Token（商户）",
						Params: []APIDocParam{
							{Name: "product_id", In: "body", Type: "integer", Required: true, Description: "商品ID"},
							{Name: "quantity", In: "body", Type: "integer", Required: true, Description: "入库数量"},
							{Name: "notes", In: "body", Type: "string", Required: false, Description: "备注"},
						},
						Responses: []APIDocResponse{{Status: 200, Description: "入库成功"}},
					},
					{
						Method: "POST", Path: "/api/v1/merchant/inventory/outbound", Summary: "手动出库",
						Description: "手动出库操作，减少商品库存并生成流水。",
						Auth: "Bearer Token（商户）",
					},
				},
			},
			{
				Name: "服务管理", Description: "服务分类和项目CRUD",
				Endpoints: []APIDocEndpoint{
					{
						Method: "GET", Path: "/api/v1/merchant/service-categories", Summary: "服务分类",
						Description: "获取服务分类树形结构。",
						Auth: "Bearer Token（商户）",
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"categories": []interface{}{}}},
						},
					},
					{
						Method: "GET", Path: "/api/v1/merchant/service-items", Summary: "服务项目列表",
						Description: "查询服务项目列表（含价格、时长、适用宠物类型）。",
						Auth: "Bearer Token（商户）",
						Params: []APIDocParam{
							{Name: "category_id", In: "query", Type: "integer", Required: false, Description: "分类ID筛选"},
							{Name: "status", In: "query", Type: "string", Required: false, Description: "状态: active/inactive"},
						},
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"items": []interface{}{}}},
						},
					},
					{
						Method: "POST", Path: "/api/v1/merchant/service-items", Summary: "创建服务项目",
						Description: "创建新的服务项目。",
						Auth: "Bearer Token（商户）",
						Params: []APIDocParam{
							{Name: "name", In: "body", Type: "string", Required: true, Description: "服务名称"},
							{Name: "category_id", In: "body", Type: "integer", Required: true, Description: "分类ID"},
							{Name: "duration_minutes", In: "body", Type: "integer", Required: false, Description: "服务时长（分钟）"},
							{Name: "price_cents", In: "body", Type: "integer", Required: true, Description: "标准价格（分）"},
							{Name: "member_price_cents", In: "body", Type: "integer", Required: false, Description: "会员价（分）"},
							{Name: "pet_type", In: "body", Type: "string", Required: false, Description: "适用宠物类型: dog/cat"},
						},
						Responses: []APIDocResponse{{Status: 200, Description: "创建成功"}},
					},
				},
			},
			{
				Name: "员工管理", Description: "员工档案、考勤、提成管理",
				Endpoints: []APIDocEndpoint{
					{
						Method: "GET", Path: "/api/v1/merchant/employees", Summary: "员工列表",
						Description: "查询员工列表，支持岗位/状态筛选和关键字搜索。",
						Auth: "Bearer Token（商户）",
						Params: []APIDocParam{
							{Name: "position", In: "query", Type: "string", Required: false, Description: "岗位筛选"},
							{Name: "status", In: "query", Type: "string", Required: false, Description: "状态: active/inactive"},
							{Name: "keyword", In: "query", Type: "string", Required: false, Description: "搜索关键字"},
						},
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"employees": []interface{}{}, "total": 0}},
						},
					},
					{
						Method: "POST", Path: "/api/v1/merchant/employees", Summary: "创建员工",
						Description: "新建员工档案。",
						Auth: "Bearer Token（商户）",
						Params: []APIDocParam{
							{Name: "name", In: "body", Type: "string", Required: true, Description: "姓名"},
							{Name: "employee_no", In: "body", Type: "string", Required: true, Description: "工号（唯一）"},
							{Name: "position", In: "body", Type: "string", Required: true, Description: "岗位"},
							{Name: "phone", In: "body", Type: "string", Required: false, Description: "手机号"},
						},
						Responses: []APIDocResponse{
							{Status: 200, Description: "创建成功"},
							{Status: 409, Description: "工号重复", Example: map[string]string{"code": "CONFLICT", "message": "employee number already exists: E001"}},
						},
					},
					{
						Method: "POST", Path: "/api/v1/merchant/employees/{id}/resign", Summary: "员工离职",
						Description: "标记员工离职，状态变为inactive并禁用关联账号。",
						Auth: "Bearer Token（商户）",
						Responses: []APIDocResponse{{Status: 200, Description: "操作成功"}},
					},
				},
			},
			{
				Name: "商户角色", Description: "商户端角色权限管理",
				Endpoints: []APIDocEndpoint{
					{
						Method: "GET", Path: "/api/v1/merchant/roles/permissions", Summary: "可用权限列表",
						Description: "获取商户端所有可分配的权限点列表（20项）。",
						Auth: "Bearer Token（商户）",
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"permissions": []interface{}{}}},
						},
					},
					{
						Method: "GET", Path: "/api/v1/merchant/roles", Summary: "角色列表",
						Description: "查询商户端自定义角色。",
						Auth: "Bearer Token（商户）",
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"roles": []interface{}{}}},
						},
					},
					{
						Method: "POST", Path: "/api/v1/merchant/roles", Summary: "创建角色",
						Description: "创建商户端自定义角色并配置权限。",
						Auth: "Bearer Token（商户）",
						Params: []APIDocParam{
							{Name: "name", In: "body", Type: "string", Required: true, Description: "角色名称"},
							{Name: "code", In: "body", Type: "string", Required: true, Description: "角色编码"},
							{Name: "permissions", In: "body", Type: "[]string", Required: true, Description: "权限列表"},
						},
						Responses: []APIDocResponse{{Status: 200, Description: "创建成功"}},
					},
					{
						Method: "POST", Path: "/api/v1/merchant/employees/{id}/assign-role", Summary: "分配角色",
						Description: "为员工分配商户角色。",
						Auth: "Bearer Token（商户）",
						Responses: []APIDocResponse{{Status: 200, Description: "分配成功"}},
					},
					{
						Method: "POST", Path: "/api/v1/merchant/employees/{id}/create-account", Summary: "创建登录账号",
						Description: "为员工创建平台登录账号。",
						Auth: "Bearer Token（商户）",
						Responses: []APIDocResponse{
							{Status: 200, Description: "创建成功", Example: map[string]interface{}{"username": "e_12_E001", "password": "random123"}},
						},
					},
				},
			},
			{
				Name: "预约管理", Description: "预约创建、确认、改期、取消、服务进度管控",
				Endpoints: []APIDocEndpoint{
					{
						Method: "GET", Path: "/api/v1/merchant/appointments", Summary: "预约列表",
						Description: "查询预约列表，支持按日期、状态、技师筛选。",
						Auth: "Bearer Token（商户）",
						Params: []APIDocParam{
							{Name: "date", In: "query", Type: "string", Required: false, Description: "预约日期"},
							{Name: "status", In: "query", Type: "string", Required: false, Description: "状态筛选"},
							{Name: "page", In: "query", Type: "integer", Required: false, Description: "页码"},
						},
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"appointments": []interface{}{}, "total": 0}},
						},
					},
					{
						Method: "POST", Path: "/api/v1/merchant/appointments", Summary: "创建预约",
						Description: "创建新预约（选择会员、宠物、服务项目、时段、技师）。",
						Auth: "Bearer Token（商户）",
						Params: []APIDocParam{
							{Name: "member_id", In: "body", Type: "integer", Required: true, Description: "会员ID"},
							{Name: "pet_id", In: "body", Type: "integer", Required: false, Description: "宠物ID"},
							{Name: "service_item_ids", In: "body", Type: "[]integer", Required: true, Description: "服务项目ID列表"},
							{Name: "appointment_time", In: "body", Type: "string", Required: true, Description: "预约时间"},
							{Name: "employee_id", In: "body", Type: "integer", Required: false, Description: "技师ID"},
						},
						Responses: []APIDocResponse{{Status: 200, Description: "创建成功", Example: map[string]interface{}{"id": 1, "status": "pending"}}},
					},
					{
						Method: "POST", Path: "/api/v1/merchant/appointments/{id}/confirm", Summary: "确认预约",
						Description: "确认预约，状态从pending变为confirmed。",
						Auth: "Bearer Token（商户）",
					},
					{
						Method: "POST", Path: "/api/v1/merchant/appointments/{id}/cancel", Summary: "取消预约",
						Description: "取消预约，技师排班释放。",
						Auth: "Bearer Token（商户）",
						Params: []APIDocParam{
							{Name: "reason", In: "body", Type: "string", Required: false, Description: "取消原因"},
						},
					},
				},
			},
			{
				Name: "排班管理", Description: "员工排班设置和查询",
				Endpoints: []APIDocEndpoint{
					{
						Method: "GET", Path: "/api/v1/merchant/schedules", Summary: "排班列表",
						Description: "查询员工排班表，支持按日期范围和员工筛选。",
						Auth: "Bearer Token（商户）",
						Params: []APIDocParam{
							{Name: "start_date", In: "query", Type: "string", Required: false, Description: "开始日期"},
							{Name: "end_date", In: "query", Type: "string", Required: false, Description: "结束日期"},
							{Name: "employee_id", In: "query", Type: "integer", Required: false, Description: "员工ID"},
						},
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"schedules": []interface{}{}}},
						},
					},
					{
						Method: "POST", Path: "/api/v1/merchant/schedules/batch", Summary: "批量设置排班",
						Description: "批量设置员工排班。",
						Auth: "Bearer Token（商户）",
					},
				},
			},
			{
				Name: "营收与财务", Description: "营收统计、收支流水、应付账款、财务报表",
				Endpoints: []APIDocEndpoint{
					{
						Method: "GET", Path: "/api/v1/merchant/revenue/summary", Summary: "营收统计",
						Description: "按日/周/月统计门店营收（商品营收、服务营收、储值充值、退款）。",
						Auth: "Bearer Token（商户）",
						Params: []APIDocParam{
							{Name: "date", In: "query", Type: "string", Required: false, Description: "日期"},
							{Name: "period", In: "query", Type: "string", Required: false, Description: "统计维度: day/week/month"},
						},
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"product_revenue": 0, "service_revenue": 0, "refund_amount": 0}},
						},
					},
					{
						Method: "GET", Path: "/api/v1/merchant/revenue/transactions", Summary: "收支明细",
						Description: "查询每笔交易的收支明细。",
						Auth: "Bearer Token（商户）",
						Params: []APIDocParam{
							{Name: "method", In: "query", Type: "string", Required: false, Description: "支付方式筛选"},
							{Name: "page", In: "query", Type: "integer", Required: false, Description: "页码"},
						},
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"transactions": []interface{}{}, "total": 0}},
						},
					},
				},
			},
			{
				Name: "财务报表", Description: "利润表、营收明细、销售报表、服务报表、导出",
				Endpoints: []APIDocEndpoint{
					{
						Method: "GET", Path: "/api/v1/merchant/statements/profit", Summary: "利润表",
						Description: "展示营收、成本、费用、毛利、净利。",
						Auth: "Bearer Token（商户）",
						Params: []APIDocParam{
							{Name: "year_month", In: "query", Type: "string", Required: false, Description: "年月（YYYY-MM）"},
						},
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"revenue": 0, "cost": 0, "gross_profit": 0, "net_profit": 0}},
						},
					},
					{
						Method: "GET", Path: "/api/v1/merchant/statements/product-sales", Summary: "商品销售报表",
						Description: "展示各类别/各商品销售数量和金额。",
						Auth: "Bearer Token（商户）",
					},
					{
						Method: "GET", Path: "/api/v1/merchant/statements/service-performance", Summary: "服务业绩报表",
						Description: "展示各服务项目的完成量和金额、技师排行。",
						Auth: "Bearer Token（商户）",
					},
				},
			},
			{
				Name: "其他商户功能", Description: "考勤、提成、通知、优惠券、促销、次卡、评价、日结、小票",
				Endpoints: []APIDocEndpoint{
					{
						Method: "POST", Path: "/api/v1/merchant/attendance/check-in", Summary: "签到",
						Description: "员工打卡签到。",
						Auth: "Bearer Token（商户）",
						Responses: []APIDocResponse{{Status: 200, Description: "签到成功"}},
					},
					{
						Method: "POST", Path: "/api/v1/merchant/attendance/check-out", Summary: "签退",
						Description: "员工下班签退。",
						Auth: "Bearer Token（商户）",
						Responses: []APIDocResponse{{Status: 200, Description: "签退成功"}},
					},
					{
						Method: "GET", Path: "/api/v1/merchant/coupons/templates", Summary: "优惠券模板列表",
						Description: "查询优惠券模板。",
						Auth: "Bearer Token（商户）",
					},
					{
						Method: "GET", Path: "/api/v1/merchant/promotions", Summary: "促销活动列表",
						Description: "查询促销活动。",
						Auth: "Bearer Token（商户）",
					},
					{
						Method: "GET", Path: "/api/v1/merchant/reviews", Summary: "评价列表",
						Description: "查看客户评价，支持好评/差评筛选。",
						Auth: "Bearer Token（商户）",
					},
					{
						Method: "GET", Path: "/api/v1/merchant/shift/today", Summary: "今日交班",
						Description: "查看今日交班报表。",
						Auth: "Bearer Token（商户）",
					},
					{
						Method: "GET", Path: "/api/v1/merchant/receipt-template", Summary: "小票模板",
						Description: "获取小票打印模板配置。",
						Auth: "Bearer Token（商户）",
					},
					{
						Method: "POST", Path: "/api/v1/merchant/verification/coupon", Summary: "优惠券核销",
						Description: "核销优惠券码。",
						Auth: "Bearer Token（商户）",
					},
				},
			},
			{
				Name: "开放平台 — 认证", Description: "开发者入驻、Token获取、签名验证",
				Endpoints: []APIDocEndpoint{
					{
						Method: "POST", Path: "/api/v1/open/developers/apply", Summary: "开发者入驻申请",
						Description: "第三方开发者提交入驻申请。",
						Auth: "无",
						Params: []APIDocParam{
							{Name: "company_name", In: "body", Type: "string", Required: true, Description: "企业名称"},
							{Name: "contact_person", In: "body", Type: "string", Required: true, Description: "联系人"},
							{Name: "contact_phone", In: "body", Type: "string", Required: true, Description: "联系电话"},
							{Name: "purpose", In: "body", Type: "string", Required: true, Description: "对接用途"},
							{Name: "callback_url", In: "body", Type: "string", Required: true, Description: "回调地址"},
						},
						Responses: []APIDocResponse{{Status: 200, Description: "提交成功", Example: map[string]interface{}{"id": 1, "status": "pending"}}},
					},
					{
						Method: "POST", Path: "/api/v1/open/token", Summary: "获取AccessToken",
						Description: "使用AppKey+AppSecret+HMAC-SHA256签名获取AccessToken（有效期2小时）。",
						Auth: "AppKey + AppSecret签名",
						Params: []APIDocParam{
							{Name: "app_key", In: "body", Type: "string", Required: true, Description: "应用Key"},
							{Name: "timestamp", In: "body", Type: "string", Required: true, Description: "Unix时间戳"},
							{Name: "nonce", In: "body", Type: "string", Required: true, Description: "随机字符串"},
							{Name: "signature", In: "body", Type: "string", Required: true, Description: "HMAC-SHA256签名"},
						},
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"access_token": "eyJ...", "expires_in": 7200}},
							{Status: 401, Description: "签名错误", Example: map[string]string{"code": "SIGNATURE_INVALID", "message": "invalid signature"}},
						},
					},
				},
			},
			{
				Name: "开放平台 — 基础信息", Description: "店铺信息、商品、服务、品种查询",
				Endpoints: []APIDocEndpoint{
					{
						Method: "GET", Path: "/api/open/v1/shop/info", Summary: "店铺信息",
						Description: "获取店铺名称、地址、营业时间、Logo。",
						Auth: "Bearer Token（开放平台）",
						Responses: []APIDocResponse{
							{Status: 200, Description: "成功", Example: map[string]interface{}{"name": "星星宠物店", "address": "北京市朝阳区", "business_hours": "09:00-21:00"}},
						},
					},
					{
						Method: "GET", Path: "/api/open/v1/products", Summary: "商品列表",
						Description: "查询商品分页列表，支持分类筛选。",
						Auth: "Bearer Token（开放平台）",
						Params: []APIDocParam{
							{Name: "category_id", In: "query", Type: "integer", Required: false, Description: "分类ID"},
							{Name: "page", In: "query", Type: "integer", Required: false, Description: "页码"},
							{Name: "page_size", In: "query", Type: "integer", Required: false, Description: "每页条数"},
						},
					},
					{
						Method: "GET", Path: "/api/open/v1/products/{id}", Summary: "商品详情",
						Description: "获取商品详情（含多规格SKU）。",
						Auth: "Bearer Token（开放平台）",
					},
					{
						Method: "GET", Path: "/api/open/v1/services", Summary: "服务列表",
						Description: "查询服务项目列表（含价格和时长）。",
						Auth: "Bearer Token（开放平台）",
					},
					{
						Method: "GET", Path: "/api/open/v1/breeds", Summary: "品种查询",
						Description: "查询宠物品种列表。",
						Auth: "Bearer Token（开放平台）",
						Params: []APIDocParam{
							{Name: "type", In: "query", Type: "string", Required: false, Description: "宠物类型: dog/cat"},
						},
					},
				},
			},
			{
				Name: "开放平台 — 会员与宠物", Description: "会员注册、查询、更新，宠物档案管理",
				Endpoints: []APIDocEndpoint{
					{
						Method: "POST", Path: "/api/open/v1/members/register", Summary: "会员注册",
						Description: "注册新会员，返回会员ID。",
						Auth: "Bearer Token（开放平台）",
						Params: []APIDocParam{
							{Name: "name", In: "body", Type: "string", Required: true, Description: "姓名"},
							{Name: "phone", In: "body", Type: "string", Required: true, Description: "手机号"},
						},
						Responses: []APIDocResponse{{Status: 200, Description: "注册成功", Example: map[string]interface{}{"member_id": 1}}},
					},
					{
						Method: "GET", Path: "/api/open/v1/members/{id}", Summary: "会员信息查询",
						Description: "查询会员信息（含等级、储值、积分）。",
						Auth: "Bearer Token（开放平台）",
					},
					{
						Method: "PUT", Path: "/api/open/v1/members/{id}", Summary: "更新会员信息",
						Description: "更新会员基础信息。",
						Auth: "Bearer Token（开放平台）",
					},
					{
						Method: "POST", Path: "/api/open/v1/members/{id}/pets", Summary: "添加宠物",
						Description: "为会员添加宠物档案。",
						Auth: "Bearer Token（开放平台）",
					},
					{
						Method: "GET", Path: "/api/open/v1/members/{id}/pets", Summary: "宠物列表",
						Description: "查询会员的宠物列表。",
						Auth: "Bearer Token（开放平台）",
					},
				},
			},
			{
				Name: "开放平台 — 预约服务", Description: "预约创建、查询、修改、取消、技师忙闲",
				Endpoints: []APIDocEndpoint{
					{
						Method: "POST", Path: "/api/open/v1/bookings", Summary: "创建预约",
						Description: "创建预约，返回预约ID。",
						Auth: "Bearer Token（开放平台）",
						Params: []APIDocParam{
							{Name: "member_id", In: "body", Type: "integer", Required: true, Description: "会员ID"},
							{Name: "pet_id", In: "body", Type: "integer", Required: false, Description: "宠物ID"},
							{Name: "service_item_ids", In: "body", Type: "[]integer", Required: true, Description: "服务项目ID列表"},
							{Name: "appointment_time", In: "body", Type: "string", Required: true, Description: "预约时间"},
						},
						Responses: []APIDocResponse{{Status: 200, Description: "创建成功", Example: map[string]interface{}{"booking_id": 1}}},
					},
					{
						Method: "GET", Path: "/api/open/v1/bookings", Summary: "预约列表",
						Description: "查询会员的预约列表。",
						Auth: "Bearer Token（开放平台）",
						Params: []APIDocParam{
							{Name: "member_id", In: "query", Type: "integer", Required: true, Description: "会员ID"},
						},
					},
					{
						Method: "GET", Path: "/api/open/v1/technicians/{id}/availability", Summary: "技师可用时段",
						Description: "查询指定日期技师的可用时间段。",
						Auth: "Bearer Token（开放平台）",
						Params: []APIDocParam{
							{Name: "date", In: "query", Type: "string", Required: true, Description: "查询日期（YYYY-MM-DD）"},
						},
					},
				},
			},
			{
				Name: "开放平台 — 订单与支付", Description: "订单创建、查询、支付回调、退款",
				Endpoints: []APIDocEndpoint{
					{
						Method: "POST", Path: "/api/open/v1/orders", Summary: "创建订单",
						Description: "创建订单，返回订单号和支付参数。",
						Auth: "Bearer Token（开放平台）",
						Params: []APIDocParam{
							{Name: "member_id", In: "body", Type: "integer", Required: true, Description: "会员ID"},
							{Name: "items", In: "body", Type: "array", Required: true, Description: "订单明细"},
							{Name: "payments", In: "body", Type: "array", Required: true, Description: "支付明细"},
						},
						Responses: []APIDocResponse{
							{Status: 200, Description: "创建成功", Example: map[string]interface{}{"order_no": "ORD202605230001", "order_id": 1}},
						},
					},
					{
						Method: "GET", Path: "/api/open/v1/orders/{id}", Summary: "订单详情",
						Description: "查询订单详情和支付状态。",
						Auth: "Bearer Token（开放平台）",
					},
					{
						Method: "POST", Path: "/api/open/v1/orders/{id}/pay-callback", Summary: "支付回调",
						Description: "接收支付平台回调通知，自动更新订单状态。",
						Auth: "Bearer Token（开放平台）",
					},
					{
						Method: "POST", Path: "/api/open/v1/orders/{id}/refund", Summary: "退款申请",
						Description: "发起退款申请。",
						Auth: "Bearer Token（开放平台）",
					},
				},
			},
			{
				Name: "开放平台 — 营销核销", Description: "优惠券领取/核销、团购核销、活动查询",
				Endpoints: []APIDocEndpoint{
					{
						Method: "POST", Path: "/api/open/v1/coupons/{id}/claim", Summary: "领取优惠券",
						Description: "会员领取指定优惠券。",
						Auth: "Bearer Token（开放平台）",
					},
					{
						Method: "POST", Path: "/api/open/v1/coupons/verify", Summary: "核销优惠券",
						Description: "验证并核销优惠券码，返回抵扣金额。",
						Auth: "Bearer Token（开放平台）",
					},
					{
						Method: "GET", Path: "/api/open/v1/activities", Summary: "活动列表",
						Description: "查询进行中的促销活动。",
						Auth: "Bearer Token（开放平台）",
					},
					{
						Method: "POST", Path: "/api/open/v1/groupon/verify", Summary: "团购券核销",
						Description: "核销第三方团购券（如美团券），验证券码有效性。",
						Auth: "Bearer Token（开放平台）",
					},
				},
			},
			{
				Name: "API监控", Description: "实时接口调用量、成功率、异常监控（F076）",
				Endpoints: []APIDocEndpoint{
					{
						Method: "GET", Path: "/api/v1/monitor/endpoints", Summary: "接口监控",
						Description: "获取各接口的调用量、成功率、P95响应时间。",
						Auth: "Bearer Token + 平台用户",
						Params: []APIDocParam{
							{Name: "period", In: "query", Type: "string", Required: false, Description: "时间范围: 1h/24h/7d"},
						},
					},
					{
						Method: "GET", Path: "/api/v1/monitor/developers", Summary: "开发者统计",
						Description: "按开发者维度统计API调用数据。",
						Auth: "Bearer Token + 平台用户",
					},
					{
						Method: "GET", Path: "/api/v1/monitor/anomalies", Summary: "异常预警",
						Description: "获取接口异常预警列表。",
						Auth: "Bearer Token + 平台用户",
					},
				},
			},
			{
				Name: "通用错误码", Description: "所有接口通用的错误码说明",
				Endpoints: []APIDocEndpoint{
					{
						Method: "", Path: "INVALID_PARAMS (400)", Summary: "参数校验失败",
						Description: "请求参数缺失或格式不正确。", Auth: "",
						Responses: []APIDocResponse{
							{Status: 400, Example: map[string]interface{}{"code": "INVALID_PARAMS", "message": "missing required fields: name, phone", "data": nil, "request_id": "uuid"}},
						},
					},
					{
						Method: "", Path: "UNAUTHORIZED (401)", Summary: "未认证",
						Description: "缺少有效的Bearer Token。", Auth: "",
						Responses: []APIDocResponse{
							{Status: 401, Example: map[string]interface{}{"code": "UNAUTHORIZED", "message": "authorization header required", "data": nil, "request_id": "uuid"}},
						},
					},
					{
						Method: "", Path: "TOKEN_EXPIRED (401)", Summary: "Token已过期",
						Description: "Access Token或Refresh Token已过期。", Auth: "",
						Responses: []APIDocResponse{
							{Status: 401, Example: map[string]interface{}{"code": "TOKEN_EXPIRED", "message": "token has expired", "data": nil, "request_id": "uuid"}},
						},
					},
					{
						Method: "", Path: "INVALID_CREDENTIALS (401)", Summary: "凭据无效",
						Description: "用户名或密码错误。", Auth: "",
						Responses: []APIDocResponse{
							{Status: 401, Example: map[string]interface{}{"code": "INVALID_CREDENTIALS", "message": "invalid username or password", "data": nil, "request_id": "uuid"}},
						},
					},
					{
						Method: "", Path: "FORBIDDEN (403)", Summary: "权限不足",
						Description: "无访问该资源的权限。", Auth: "",
						Responses: []APIDocResponse{
							{Status: 403, Example: map[string]interface{}{"code": "FORBIDDEN", "message": "insufficient permissions: merchant:manage required", "data": nil, "request_id": "uuid"}},
						},
					},
					{
						Method: "", Path: "NOT_FOUND (404)", Summary: "资源不存在",
						Description: "请求的资源不存在。", Auth: "",
						Responses: []APIDocResponse{
							{Status: 404, Example: map[string]interface{}{"code": "NOT_FOUND", "message": "resource not found", "data": nil, "request_id": "uuid"}},
						},
					},
					{
						Method: "", Path: "CONFLICT (409)", Summary: "资源冲突",
						Description: "数据重复或业务规则冲突。", Auth: "",
						Responses: []APIDocResponse{
							{Status: 409, Example: map[string]interface{}{"code": "CONFLICT", "message": "employee number already exists: E001", "data": nil, "request_id": "uuid"}},
						},
					},
					{
						Method: "", Path: "INTERNAL_ERROR (500)", Summary: "内部错误",
						Description: "服务器内部错误，不暴露细节。", Auth: "",
						Responses: []APIDocResponse{
							{Status: 500, Example: map[string]interface{}{"code": "INTERNAL_ERROR", "message": "internal server error", "data": nil, "request_id": "uuid"}},
						},
					},
				},
			},
		},
	}
}

func makeAPIDocsHandler() http.HandlerFunc {
	docs := getAPIDocs()
	data, _ := json.Marshal(docs)
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}
}
