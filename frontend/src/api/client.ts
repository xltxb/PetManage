const API_BASE = ''

interface RequestOptions {
  method?: string
  body?: any
  headers?: Record<string, string>
}

interface Category {
  id: number
  merchant_id: number
  parent_id: number | null
  name: string
  sort_order: number
  children?: Category[]
  created_at: string
  updated_at: string
}

function parseJWT(token: string): any {
  try {
    const payload = token.split('.')[1]
    return JSON.parse(atob(payload))
  } catch {
    return {}
  }
}

class ApiClient {
  private token: string | null = null

  setToken(token: string | null) {
    this.token = token
    if (token) {
      localStorage.setItem('access_token', token)
    } else {
      localStorage.removeItem('access_token')
    }
  }

  getToken(): string | null {
    if (!this.token) {
      this.token = localStorage.getItem('access_token')
    }
    return this.token
  }

  async request<T = any>(path: string, options: RequestOptions = {}): Promise<T> {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...options.headers,
    }

    const token = this.getToken()
    if (token) {
      headers['Authorization'] = `Bearer ${token}`
    }

    const res = await fetch(`${API_BASE}${path}`, {
      method: options.method || 'GET',
      headers,
      body: options.body ? JSON.stringify(options.body) : undefined,
    })

    const data = await res.json()

    if (!res.ok) {
      throw new Error(data.message || data.code || `HTTP ${res.status}`)
    }

    return data as T
  }

  async merchantLogin(username: string, password: string) {
    const data = await this.request<{
      access_token: string
      refresh_token: string
      expires_in: number
      must_change_password: boolean
      merchant_name: string
      display_name: string
    }>('/api/v1/merchant/auth/login', {
      method: 'POST',
      body: { username, password },
      headers: {},
    })
    this.setToken(data.access_token)
    const claims = parseJWT(data.access_token)
    return {
      ...data,
      user_id: claims.user_id as number,
      username: claims.username as string,
      merchant_id: claims.merchant_id as number | null,
    }
  }

  async login(username: string, password: string) {
    const data = await this.request<{
      access_token: string
      refresh_token: string
      expires_in: number
      must_change_password: boolean
    }>('/api/v1/auth/login', {
      method: 'POST',
      body: { username, password },
      headers: {}, // no auth header
    })
    this.setToken(data.access_token)
    const claims = parseJWT(data.access_token)
    return {
      ...data,
      user_id: claims.user_id as number,
      username: claims.username as string,
    }
  }

  logout() {
    this.setToken(null)
  }

  getMerchantDashboard() {
    return this.request<{
      today_revenue: number
      today_orders: number
      today_new_members: number
      today_appointments: number
      today_service_complete: number
      stock_warnings: number
      near_expiry_count: number
      expired_count: number
      pending_appointments: number
      birthday_reminders: number
      vaccine_reminder_count: number
      deworming_reminder_count: number
      revenue_trend: number[]
      merchant_id: number
    }>('/api/v1/merchant/dashboard')
  }

  getHealthReminders(params?: { type?: string; days?: number; page?: number; page_size?: number }) {
    const search = new URLSearchParams()
    if (params?.type) search.set('type', params.type)
    if (params?.days) search.set('days', String(params.days))
    if (params?.page) search.set('page', String(params.page))
    if (params?.page_size) search.set('page_size', String(params.page_size))
    const qs = search.toString()
    return this.request<{
      reminders: Array<{
        pet_id: number
        pet_name: string
        member_id: number
        member_name: string
        card_no: string
        reminder_type: string
        item_name: string
        last_date: string
        next_date: string
        days_left: number
        notes: string
      }>
      total: number
      page: number
      page_size: number
    }>(`/api/v1/merchant/pets/health-reminders${qs ? '?' + qs : ''}`)
  }

  getHealthReminderCounts(days?: number) {
    const qs = days ? `?days=${days}` : ''
    return this.request<{
      vaccine_count: number
      deworming_count: number
    }>(`/api/v1/merchant/pets/health-reminders/count${qs}`)
  }

  getInventoryAlerts(params?: { alert_type?: string; page?: number; page_size?: number }) {
    const search = new URLSearchParams()
    if (params?.alert_type) search.set('alert_type', params.alert_type)
    if (params?.page) search.set('page', String(params.page))
    if (params?.page_size) search.set('page_size', String(params.page_size))
    const qs = search.toString()
    return this.request<{
      alerts: Array<{
        id: number
        merchant_id: number
        product_id: number
        name: string
        barcode: string
        stock: number
        alert_stock: number
        expiry_date: string | null
        alert_type: string
        days_left: number | null
        status: string
      }>
      total: number
      page: number
      page_size: number
    }>(`/api/v1/merchant/inventory/alerts${qs ? '?' + qs : ''}`)
  }

  getDashboardOverview(period = 'all') {
    return this.request<{
      total_merchants: number
      active_merchants: number
      new_merchants_period: number
      total_orders: number
      total_transaction: number
      new_members: number
      service_completions: number
      period: string
      metrics: Array<{ value: number; label: string }>
    }>(`/api/v1/dashboard/overview?period=${period}`)
  }

  getShopSettings() {
    return this.request<{
      name: string
      logo_url: string
      address: string
      contact_phone: string
      contact_email: string
      business_hours: string
      notice: string
    }>('/api/v1/merchant/shop-settings')
  }

  updateShopSettings(data: {
    name: string
    address?: string
    contact_phone?: string
    contact_email?: string
    business_hours?: string
    notice?: string
  }) {
    return this.request<{
      name: string
      logo_url: string
      address: string
      contact_phone: string
      contact_email: string
      business_hours: string
      notice: string
    }>('/api/v1/merchant/shop-settings', {
      method: 'PUT',
      body: data,
    })
  }

  getMerchantList(params?: { keyword?: string; status?: string; page?: number; page_size?: number }) {
    const search = new URLSearchParams()
    if (params?.keyword) search.set('keyword', params.keyword)
    if (params?.status) search.set('status', params.status)
    if (params?.page) search.set('page', String(params.page))
    if (params?.page_size) search.set('page_size', String(params.page_size))
    const qs = search.toString()
    return this.request<{
      merchants: Array<{ id: number; name: string; license_number: string; legal_person: string; contact_phone: string; status: string; contract_status?: string; created_at: string }>
      total: number
      page: number
      page_size: number
    }>(`/api/v1/merchants${qs ? '?' + qs : ''}`)
  }

  getMerchantAnalysis(merchantId: number, period = 'all') {
    return this.request<{
      merchant_id: number
      merchant_name: string
      period: string
      today_revenue: number
      today_orders: number
      today_new_members: number
      total_revenue: number
      total_orders: number
      revenue_rank: number
      product_sales_rank: Array<{ product_id: number; product_name: string; quantity: number; revenue: number; rank: number }>
      service_popularity: Array<{ service_id: number; service_name: string; order_count: number; revenue: number; rank: number }>
    }>(`/api/v1/dashboard/merchant/${merchantId}/analysis?period=${period}`)
  }

  getMerchantsRevenueRanking(period = 'all') {
    return this.request<Array<{ merchant_id: number; merchant_name: string; total_revenue: number; rank: number }>>(`/api/v1/dashboard/merchants/ranking?period=${period}`)
  }

  async downloadFile(url: string): Promise<{ blob: Blob; filename: string }> {
    const headers: Record<string, string> = {}
    const token = this.getToken()
    if (token) {
      headers['Authorization'] = `Bearer ${token}`
    }

    const res = await fetch(`${API_BASE}${url}`, { headers })

    if (!res.ok) {
      const data = await res.json().catch(() => ({ message: `HTTP ${res.status}` }))
      throw new Error(data.message || data.code || `HTTP ${res.status}`)
    }

    const disposition = res.headers.get('Content-Disposition') || ''
    const match = disposition.match(/filename="?([^"]+)"?/)
    const filename = match ? match[1] : 'download.xlsx'

    return { blob: await res.blob(), filename }
  }

  // --- Product APIs ---

  getProducts(params?: { keyword?: string; status?: string; page?: number; page_size?: number }) {
    const search = new URLSearchParams()
    if (params?.keyword) search.set('keyword', params.keyword)
    if (params?.status) search.set('status', params.status)
    if (params?.page) search.set('page', String(params.page))
    if (params?.page_size) search.set('page_size', String(params.page_size))
    const qs = search.toString()
    return this.request<any>(`/api/v1/merchant/products${qs ? '?' + qs : ''}`)
  }

  getProduct(id: number) {
    return this.request<any>(`/api/v1/merchant/products/${id}`)
  }

  createProduct(data: any) {
    return this.request<any>('/api/v1/merchant/products', { method: 'POST', body: data })
  }

  updateProduct(id: number, data: any) {
    return this.request<any>(`/api/v1/merchant/products/${id}`, { method: 'PUT', body: data })
  }

  deleteProduct(id: number) {
    return this.request<{ message: string }>(`/api/v1/merchant/products/${id}`, { method: 'DELETE' })
  }

  toggleProductStatus(id: number) {
    return this.request<any>(`/api/v1/merchant/products/${id}/toggle-status`, { method: 'POST' })
  }

  // --- Category APIs ---

  getCategories() {
    return this.request<{ categories: Category[] }>('/api/v1/merchant/categories')
  }

  createCategory(data: { name: string; parent_id?: number | null; sort_order?: number }) {
    return this.request<Category>('/api/v1/merchant/categories', {
      method: 'POST',
      body: data,
    })
  }

  updateCategory(id: number, data: { name: string; parent_id?: number | null; sort_order?: number }) {
    return this.request<Category>(`/api/v1/merchant/categories/${id}`, {
      method: 'PUT',
      body: data,
    })
  }

  deleteCategory(id: number) {
    return this.request<{ message: string }>(`/api/v1/merchant/categories/${id}`, {
      method: 'DELETE',
    })
  }

  // --- Member APIs ---

  getMembers(params?: { keyword?: string; status?: string; page?: number; page_size?: number }) {
    const search = new URLSearchParams()
    if (params?.keyword) search.set('keyword', params.keyword)
    if (params?.status) search.set('status', params.status)
    if (params?.page) search.set('page', String(params.page))
    if (params?.page_size) search.set('page_size', String(params.page_size))
    const qs = search.toString()
    return this.request<any>(`/api/v1/merchant/members${qs ? '?' + qs : ''}`)
  }

  getMember(id: number) {
    return this.request<any>(`/api/v1/merchant/members/${id}`)
  }

  searchMembers(phone: string) {
    return this.request<any>(`/api/v1/merchant/members/search?phone=${encodeURIComponent(phone)}`)
  }

  getMemberQRCodeUrl(id: number, download = false): string {
    const token = this.getToken()
    const params = download ? '?download=1' : ''
    // Return the URL directly for use in <img> tags, since we need auth header
    return `${API_BASE}/api/v1/merchant/members/${id}/qrcode${params}`
  }

  async getMemberQRCodeBlob(id: number): Promise<{ blob: Blob; cardNo: string }> {
    const headers: Record<string, string> = {}
    const token = this.getToken()
    if (token) {
      headers['Authorization'] = `Bearer ${token}`
    }

    const res = await fetch(`${API_BASE}/api/v1/merchant/members/${id}/qrcode?download=1`, { headers })
    if (!res.ok) {
      const data = await res.json().catch(() => ({ message: `HTTP ${res.status}` }))
      throw new Error(data.message || `HTTP ${res.status}`)
    }

    const disposition = res.headers.get('Content-Disposition') || ''
    const match = disposition.match(/filename="?([^"]+)"?/)
    const cardNo = match ? match[1].replace('member_', '').replace('_qrcode.png', '') : 'unknown'

    return { blob: await res.blob(), cardNo }
  }

  createMember(data: { name: string; phone: string; wechat?: string; gender?: string; birthday?: string; address?: string; remark?: string }) {
    return this.request<any>('/api/v1/merchant/members', { method: 'POST', body: data })
  }

  updateMember(id: number, data: Record<string, any>) {
    return this.request<any>(`/api/v1/merchant/members/${id}`, { method: 'PUT', body: data })
  }

  toggleMemberStatus(id: number) {
    return this.request<any>(`/api/v1/merchant/members/${id}/toggle-status`, { method: 'POST' })
  }

  async uploadShopLogo(file: File) {
    const formData = new FormData()
    formData.append('logo', file)

    const headers: Record<string, string> = {}
    const token = this.getToken()
    if (token) {
      headers['Authorization'] = `Bearer ${token}`
    }

    const res = await fetch(`${API_BASE}/api/v1/merchant/shop-settings/logo`, {
      method: 'POST',
      headers,
      body: formData,
    })

    const data = await res.json()
    if (!res.ok) {
      throw new Error(data.message || `HTTP ${res.status}`)
    }
    return data as {
      name: string
      logo_url: string
      address: string
      contact_phone: string
      contact_email: string
      business_hours: string
      notice: string
    }
  }

  // --- POS APIs ---

  posCartCalculate(data: { member_id?: number | null; items: Array<{ product_id?: number; sku_id?: number; service_item_id?: number; quantity: number }> }) {
    return this.request<{
      items: Array<{
        product_id?: number
        sku_id?: number
        sku_spec_info?: Record<string, string>
        service_item_id?: number
        name: string
        barcode?: string
        unit_price_cents: number
        discount_cents: number
        quantity: number
        line_total_cents: number
      }>
      original_cents: number
      discount_cents: number
      payable_cents: number
      member_balance_cents: number
      member_points: number
      max_points_deduct_cents: number
    }>('/api/v1/merchant/pos/cart/calculate', { method: 'POST', body: data })
  }

  posMemberLookup(phone: string) {
    return this.request<{
      member_id: number
      card_no: string
      name: string
      phone: string
      status: string
      balance_cents: number
      points: number
    }>(`/api/v1/merchant/pos/members/lookup?phone=${encodeURIComponent(phone)}`)
  }

  posCheckout(data: {
    member_id?: number | null
    items: Array<{ product_id?: number; sku_id?: number; service_item_id?: number; quantity: number }>
    payments: Array<{ method: string; amount_cents: number; received_cents?: number; coupon_code?: string }>
    order_notes?: string
  }) {
    return this.request<any>('/api/v1/merchant/checkout', { method: 'POST', body: data })
  }

  posCouponVerify(code: string) {
    return this.request<{
      id: number
      code: string
      discount_type: string
      value_cents: number
      min_order_cents: number
      status: string
    }>(`/api/v1/merchant/pos/coupons/verify?code=${encodeURIComponent(code)}`)
  }

  // --- Appointment APIs ---

  getAppointments(params?: { status?: string; page?: number; page_size?: number }) {
    const search = new URLSearchParams()
    if (params?.status) search.set('status', params.status)
    if (params?.page) search.set('page', String(params.page))
    if (params?.page_size) search.set('page_size', String(params.page_size))
    const qs = search.toString()
    return this.request<{
      appointments: Array<{
        id: number
        merchant_id: number
        member_id: number
        pet_id: number
        service_item_id: number
        employee_id: number
        appointment_time: string
        status: string
        remark: string
        created_at: string
        updated_at: string
        member_name: string
        member_phone: string
        pet_name: string
        service_item_name: string
        employee_name: string
      }>
      total: number
      page: number
      page_size: number
    }>(`/api/v1/merchant/appointments${qs ? '?' + qs : ''}`)
  }

  getAppointment(id: number) {
    return this.request<any>(`/api/v1/merchant/appointments/${id}`)
  }

  createAppointment(data: {
    member_id: number
    pet_id: number
    service_item_id: number
    employee_id: number
    appointment_time: string
    remark?: string
  }) {
    return this.request<any>('/api/v1/merchant/appointments', { method: 'POST', body: data })
  }

  getEmployees(params?: { status?: string; page?: number; page_size?: number }) {
    const search = new URLSearchParams()
    if (params?.status) search.set('status', params.status)
    if (params?.page) search.set('page', String(params.page))
    if (params?.page_size) search.set('page_size', String(params.page_size))
    const qs = search.toString()
    return this.request<any>(`/api/v1/merchant/employees${qs ? '?' + qs : ''}`)
  }

  // --- Service APIs ---

  getServiceCategories() {
    return this.request<{ categories: any[] }>('/api/v1/merchant/service-categories')
  }

  getServiceItems(params?: { status?: string; keyword?: string; category_id?: number; page?: number; page_size?: number }) {
    const search = new URLSearchParams()
    if (params?.status) search.set('status', params.status)
    if (params?.keyword) search.set('keyword', params.keyword)
    if (params?.category_id) search.set('category_id', String(params.category_id))
    if (params?.page) search.set('page', String(params.page))
    if (params?.page_size) search.set('page_size', String(params.page_size))
    const qs = search.toString()
    return this.request<any>(`/api/v1/merchant/service-items${qs ? '?' + qs : ''}`)
  }

	  confirmAppointment(id: number) {
	    return this.request<any>(`/api/v1/merchant/appointments/${id}/confirm`, { method: "POST" })
	  }

	  rescheduleAppointment(id: number, data: { new_time: string; reason?: string }) {
	    return this.request<any>(`/api/v1/merchant/appointments/${id}/reschedule`, { method: "POST", body: data })
	  }

	  cancelAppointment(id: number, data: { reason?: string }) {
	    return this.request<any>(`/api/v1/merchant/appointments/${id}/cancel`, { method: "POST", body: data })
	  }

	  getAppointmentChangeLogs(id: number) {
	    return this.request<{ logs: Array<{
	      id: number
	      appointment_id: number
	      action: string
	      old_value: any
	      new_value: any
	      operator_id: number
	      reason: string
	      created_at: string
	    }> }>(`/api/v1/merchant/appointments/${id}/change-logs`)
	  }

	  arriveAppointment(id: number) {
	    return this.request<any>(`/api/v1/merchant/appointments/${id}/arrive`, { method: "POST" })
	  }

	  startAppointment(id: number) {
	    return this.request<any>(`/api/v1/merchant/appointments/${id}/start`, { method: "POST" })
	  }

	  completeAppointment(id: number) {
	    return this.request<any>(`/api/v1/merchant/appointments/${id}/complete`, { method: "POST" })
	  }

	  pickupAppointment(id: number) {
	    return this.request<any>(`/api/v1/merchant/appointments/${id}/pickup`, { method: "POST" })
	  }

	  // --- Order APIs ---

  getOrders(params?: { keyword?: string; status?: string; date_from?: string; date_to?: string; page?: number; page_size?: number }) {
    const search = new URLSearchParams()
    if (params?.keyword) search.set('keyword', params.keyword)
    if (params?.status) search.set('status', params.status)
    if (params?.date_from) search.set('date_from', params.date_from)
    if (params?.date_to) search.set('date_to', params.date_to)
    if (params?.page) search.set('page', String(params.page))
    if (params?.page_size) search.set('page_size', String(params.page_size))
    const qs = search.toString()
    return this.request<{
      orders: Array<{
        id: number
        merchant_id: number
        member_id: number | null
        member_name: string
        total_cents: number
        paid_cents: number
        status: string
        notes: string
        created_at: string
        updated_at: string
      }>
      total: number
      page: number
      page_size: number
    }>(`/api/v1/merchant/orders${qs ? '?' + qs : ''}`)
  }

  getOrder(id: number) {
    return this.request<{
      id: number
      merchant_id: number
      member_id: number | null
      member_name: string
      total_cents: number
      paid_cents: number
      status: string
      notes: string
      items: Array<{
        id: number
        product_id: number | null
        product_name: string
        price_cents: number
        quantity: number
        product_sku_id: number | null
        sku_spec_info?: Record<string, string>
        service_item_id: number | null
      }>
      payments: Array<{
        id: number
        order_id: number
        method: string
        amount_cents: number
        created_at: string
      }>
      refunds: Array<{
        id: number
        order_id: number
        refund_type: string
        reason: string
        amount_cents: number
        status: string
        requested_by: number
        approved_by: number | null
        created_at: string
      }>
      created_at: string
      updated_at: string
    }>(`/api/v1/merchant/orders/${id}`)
  }

  refundOrder(id: number, data: { refund_type: string; reason?: string; items?: Array<{ order_item_id: number; quantity: number }> }) {
    return this.request<{
      refund_id: number
      order_id: number
      amount_cents: number
      status: string
      needs_approval?: boolean
    }>(`/api/v1/merchant/orders/${id}/refund`, { method: 'POST', body: data })
  }

  // --- Schedule APIs ---

	  getSchedules(params?: { employee_id?: number; start_date?: string; end_date?: string }) {
	    const search = new URLSearchParams()
	    if (params?.employee_id) search.set('employee_id', String(params.employee_id))
	    if (params?.start_date) search.set('start_date', params.start_date)
	    if (params?.end_date) search.set('end_date', params.end_date)
	    const qs = search.toString()
	    return this.request<{
	      schedules: Array<{
	        id: number
	        merchant_id: number
	        employee_id: number
	        schedule_date: string
	        shift_type: string
	        created_at: string
	        updated_at: string
	      }>
	    }>(`/api/v1/merchant/schedules${qs ? '?' + qs : ''}`)
	  }

	  upsertSchedule(data: { employee_id: number; schedule_date: string; shift_type: string }) {
	    return this.request<any>('/api/v1/merchant/schedules', { method: 'PUT', body: data })
	  }

	  batchSetSchedules(data: { employee_id: number; schedules: Array<{ date: string; shift_type: string }> }) {
	    return this.request<any>('/api/v1/merchant/schedules/batch', { method: 'POST', body: data })
	  }

	  copyWeekSchedules(data: { from_employee_id: number; to_employee_id: number; from_week_start: string; to_week_start: string }) {
	    return this.request<{ message: string }>('/api/v1/merchant/schedules/copy-week', { method: 'POST', body: data })
	  }

	  getOnDutyEmployees(appointmentTime: string) {
	    return this.request<{
	      employees: Array<{
	        id: number
	        name: string
	        position: string
	      }>
	    }>(`/api/v1/merchant/schedules/on-duty?appointment_time=${encodeURIComponent(appointmentTime)}`)
	  }

	  // --- Verification APIs ---

	  verifyCoupon(code: string, orderId?: number) {
	    return this.request<any>('/api/v1/merchant/verification/coupon', {
	      method: 'POST',
	      body: { code, order_id: orderId || null },
	    })
	  }

	  verifyThirdPartyVoucher(code: string, orderId?: number) {
	    return this.request<any>('/api/v1/merchant/verification/third-party', {
	      method: 'POST',
	      body: { code, order_id: orderId || null },
	    })
	  }

	  verifyServiceCard(code: string, orderId?: number) {
	    return this.request<any>('/api/v1/merchant/verification/service-card', {
	      method: 'POST',
	      body: { code, order_id: orderId || null },
	    })
	  }

	  getVerificationRecords(params?: { type?: string; code?: string; page?: number; page_size?: number }) {
	    const search = new URLSearchParams()
	    if (params?.type) search.set('type', params.type)
	    if (params?.code) search.set('code', params.code)
	    if (params?.page) search.set('page', String(params.page))
	    if (params?.page_size) search.set('page_size', String(params.page_size))
	    const qs = search.toString()
	    return this.request<{
	      records: Array<{
	        id: number
	        merchant_id: number
	        verification_type: string
	        code: string
	        reference_id: number
	        result: string
	        detail: string
	        order_id: number | null
	        verified_by: number
	        verified_at: string
	        created_at: string
	      }>
	      total: number
	      page: number
	      page_size: number
	    }>(`/api/v1/merchant/verification/records${qs ? '?' + qs : ''}`)
	  }

  // --- Receipt Template APIs ---

  getReceiptTemplate() {
    return this.request<{
      merchant_id: number
      logo_url: string
      store_name: string
      contact_phone: string
      contact_address: string
      footer_note: string
      paper_width: string
      show_qrcode: boolean
      created_at: string
      updated_at: string
    }>('/api/v1/merchant/receipt-template')
  }

  updateReceiptTemplate(data: {
    logo_url?: string
    store_name?: string
    contact_phone?: string
    contact_address?: string
    footer_note?: string
    paper_width?: string
    show_qrcode?: boolean
  }) {
    return this.request<any>('/api/v1/merchant/receipt-template', {
      method: 'PUT',
      body: data,
    })
  }

  async uploadReceiptLogo(file: File) {
    const formData = new FormData()
    formData.append('logo', file)
    const headers: Record<string, string> = {}
    const token = this.getToken()
    if (token) {
      headers['Authorization'] = `Bearer ${token}`
    }
    const res = await fetch(`${API_BASE}/api/v1/merchant/receipt-template/logo`, {
      method: 'POST',
      headers,
      body: formData,
    })
    const data = await res.json()
    if (!res.ok) {
      throw new Error(data.message || `HTTP ${res.status}`)
    }
    return data as any
  }

  getOrderReceipt(orderId: number) {
    return this.request<{
      order_id: number
      store_name: string
      store_logo: string
      contact_phone: string
      contact_address: string
      footer_note: string
      member_name: string
      member_phone: string
      items: Array<{ name: string; quantity: number; price_cents: number; total_cents: number }>
      payments: Array<{ method: string; amount_cents: number }>
      subtotal_cents: number
      discount_cents: number
      total_cents: number
      paid_cents: number
      change_cents: number
      notes: string
      created_at: string
    }>(`/api/v1/merchant/orders/${orderId}/receipt`)
  }

}

export const api = new ApiClient()
