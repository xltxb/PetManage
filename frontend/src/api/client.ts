const API_BASE = ''

interface RequestOptions {
  method?: string
  body?: any
  headers?: Record<string, string>
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
}

export const api = new ApiClient()
