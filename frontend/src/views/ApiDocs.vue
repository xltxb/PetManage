<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { api } from '@/api/client'

interface APIParam {
  name: string
  in: string
  type: string
  required: boolean
  description: string
}

interface APIExample {
  content_type: string
  example: any
}

interface APIResponse {
  status: number
  description: string
  example?: any
}

interface APIEndpoint {
  method: string
  path: string
  summary: string
  description: string
  auth: string
  params?: APIParam[]
  request_body?: APIExample
  responses: APIResponse[]
}

interface APIModule {
  name: string
  description: string
  endpoints: APIEndpoint[]
}

interface APIDoc {
  version: string
  title: string
  modules: APIModule[]
}

const docs = ref<APIDoc | null>(null)
const activeModule = ref('')
const activeEndpoint = ref<string>('')
const loading = ref(true)

// Debug mode
const debugMethod = ref('GET')
const debugUrl = ref('')
const debugHeaders = ref('{\n  "Content-Type": "application/json"\n}')
const debugBody = ref('')
const debugResponse = ref('')
const debugStatus = ref<number | null>(null)
const debugLoading = ref(false)
const debugError = ref('')

// Auth context for debugging
const authContext = ref('none')
const authToken = ref('')

const methodColors: Record<string, string> = {
  GET: 'bg-blue-100 text-blue-700',
  POST: 'bg-green-100 text-green-700',
  PUT: 'bg-amber-100 text-amber-700',
  DELETE: 'bg-red-100 text-red-700',
  PATCH: 'bg-purple-100 text-purple-700',
}

function selectModule(name: string) {
  activeModule.value = name
  activeEndpoint.value = ''
}

function selectEndpoint(ep: APIEndpoint, modName: string) {
  activeModule.value = modName
  const key = `${ep.method} ${ep.path}`
  activeEndpoint.value = key
  // Auto-fill debug panel
  debugMethod.value = ep.method || 'GET'
  debugUrl.value = ep.path || ''
  debugBody.value = ep.request_body ? JSON.stringify(ep.request_body.example, null, 2) : ''
  debugResponse.value = ''
  debugStatus.value = null
  debugError.value = ''
}

const selectedEndpoint = computed(() => {
  if (!docs.value || !activeModule.value || !activeEndpoint.value) return null
  const mod = docs.value.modules.find(m => m.name === activeModule.value)
  if (!mod) return null
  return mod.endpoints.find(ep => `${ep.method} ${ep.path}` === activeEndpoint.value) || null
})

const moduleEndpoints = computed(() => {
  if (!docs.value || !activeModule.value) return []
  const mod = docs.value.modules.find(m => m.name === activeModule.value)
  return mod ? mod.endpoints : []
})

function formatJSON(obj: any): string {
  try {
    return JSON.stringify(obj, null, 2)
  } catch {
    return String(obj)
  }
}

async function sendDebugRequest() {
  debugLoading.value = true
  debugResponse.value = ''
  debugStatus.value = null
  debugError.value = ''

  try {
    const headers: Record<string, string> = {}
    try {
      const parsed = JSON.parse(debugHeaders.value)
      Object.assign(headers, parsed)
    } catch {
      debugError.value = 'Headers JSON格式错误'
      debugLoading.value = false
      return
    }

    if (authContext.value === 'platform' || authContext.value === 'merchant') {
      let token = authToken.value
      if (!token) {
        // Auto-fetch token
        token = api.getToken() || ''
      }
      if (token) {
        headers['Authorization'] = `Bearer ${token}`
      }
    }

    const options: RequestInit = {
      method: debugMethod.value,
      headers,
    }

    if (['POST', 'PUT', 'PATCH'].includes(debugMethod.value) && debugBody.value) {
      options.body = debugBody.value
    }

    const url = debugUrl.value.startsWith('/') ? debugUrl.value : `/${debugUrl.value}`
    const res = await fetch(url, options)
    debugStatus.value = res.status

    const contentType = res.headers.get('content-type') || ''
    if (contentType.includes('application/json')) {
      const json = await res.json()
      debugResponse.value = JSON.stringify(json, null, 2)
    } else {
      const text = await res.text()
      debugResponse.value = text
    }
  } catch (e: any) {
    debugError.value = e.message
  } finally {
    debugLoading.value = false
  }
}

onMounted(async () => {
  try {
    docs.value = await api.getApiDocs()
    if (docs.value.modules.length > 0) {
      activeModule.value = docs.value.modules[0].name
    }
  } catch (e: any) {
    console.error('Failed to load API docs', e)
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="api-docs">
    <!-- Header -->
    <div class="docs-header">
      <h1 class="docs-title">API 文档与调试工具</h1>
      <div class="docs-meta">
        <span class="version-badge" v-if="docs">v{{ docs.version }}</span>
        <a href="/api/v1/api-docs" target="_blank" class="json-link" v-if="docs">JSON</a>
      </div>
    </div>

    <div v-if="loading" class="loading-state">加载中...</div>

    <div v-else-if="docs" class="docs-body">
      <!-- Left Sidebar -->
      <aside class="docs-sidebar">
        <div
          v-for="mod in docs.modules"
          :key="mod.name"
          class="sidebar-module"
          :class="{ active: activeModule === mod.name }"
          @click="selectModule(mod.name)"
        >
          <div class="sidebar-module-name">{{ mod.name }}</div>
          <div class="sidebar-module-count">{{ mod.endpoints.length }} 接口</div>
        </div>
      </aside>

      <!-- Main Content -->
      <main class="docs-main">
        <!-- Module Info -->
        <template v-if="activeModule">
          <div class="module-header" v-if="docs.modules.find(m => m.name === activeModule)">
            <h2>{{ activeModule }}</h2>
            <p class="module-desc">{{ docs.modules.find(m => m.name === activeModule)?.description }}</p>
          </div>

          <!-- Endpoint List -->
          <div class="endpoint-list">
            <div
              v-for="ep in moduleEndpoints"
              :key="ep.method + ep.path"
              class="endpoint-item"
              :class="{ active: activeEndpoint === (ep.method + ' ' + ep.path) }"
              @click="selectEndpoint(ep, activeModule)"
            >
              <span v-if="ep.method" class="method-badge" :class="methodColors[ep.method] || 'bg-gray-100 text-gray-700'">
                {{ ep.method }}
              </span>
              <span class="endpoint-path">{{ ep.path }}</span>
              <span class="endpoint-summary">{{ ep.summary }}</span>
            </div>
          </div>

          <!-- Endpoint Detail -->
          <div v-if="selectedEndpoint" class="endpoint-detail">
            <h3>{{ selectedEndpoint.summary }}</h3>
            <div class="detail-meta">
              <span v-if="selectedEndpoint.method" class="method-badge" :class="methodColors[selectedEndpoint.method] || 'bg-gray-100 text-gray-700'">
                {{ selectedEndpoint.method }}
              </span>
              <code class="detail-path">{{ selectedEndpoint.path }}</code>
              <span v-if="selectedEndpoint.auth" class="auth-badge">{{ selectedEndpoint.auth }}</span>
            </div>
            <p class="detail-desc">{{ selectedEndpoint.description }}</p>

            <!-- Parameters -->
            <div v-if="selectedEndpoint.params && selectedEndpoint.params.length" class="detail-section">
              <h4>请求参数</h4>
              <table class="params-table">
                <thead>
                  <tr><th>参数名</th><th>位置</th><th>类型</th><th>必填</th><th>说明</th></tr>
                </thead>
                <tbody>
                  <tr v-for="p in selectedEndpoint.params" :key="p.name">
                    <td><code>{{ p.name }}</code></td>
                    <td><span class="param-in">{{ p.in }}</span></td>
                    <td>{{ p.type }}</td>
                    <td>{{ p.required ? '✓' : '-' }}</td>
                    <td>{{ p.description }}</td>
                  </tr>
                </tbody>
              </table>
            </div>

            <!-- Request Body -->
            <div v-if="selectedEndpoint.request_body" class="detail-section">
              <h4>请求示例</h4>
              <pre class="code-block"><code>{{ formatJSON(selectedEndpoint.request_body.example) }}</code></pre>
            </div>

            <!-- Responses -->
            <div class="detail-section">
              <h4>响应说明</h4>
              <div v-for="resp in selectedEndpoint.responses" :key="resp.status" class="response-item">
                <span class="status-badge" :class="resp.status < 400 ? 'status-ok' : 'status-err'">
                  {{ resp.status }}
                </span>
                <span class="resp-desc">{{ resp.description }}</span>
                <pre v-if="resp.example" class="code-block code-sm"><code>{{ formatJSON(resp.example) }}</code></pre>
              </div>
            </div>
          </div>
        </template>
      </main>

      <!-- Debug Panel -->
      <aside class="docs-debug">
        <h3 class="debug-title">在线调试</h3>

        <!-- Auth Context -->
        <div class="debug-section">
          <label class="debug-label">认证上下文</label>
          <select v-model="authContext" class="debug-select">
            <option value="none">无认证</option>
            <option value="platform">平台管理员 Token</option>
            <option value="merchant">商户管理员 Token</option>
          </select>
          <input
            v-if="authContext !== 'none'"
            v-model="authToken"
            type="text"
            class="debug-input"
            placeholder="手动输入 Token（留空使用当前登录Token）"
          />
        </div>

        <!-- Method & URL -->
        <div class="debug-section">
          <label class="debug-label">请求</label>
          <div class="debug-url-row">
            <select v-model="debugMethod" class="debug-method">
              <option>GET</option>
              <option>POST</option>
              <option>PUT</option>
              <option>DELETE</option>
              <option>PATCH</option>
            </select>
            <input v-model="debugUrl" type="text" class="debug-input flex-1" placeholder="/api/v1/..." />
          </div>
        </div>

        <!-- Headers -->
        <div class="debug-section">
          <label class="debug-label">Headers</label>
          <textarea v-model="debugHeaders" class="debug-textarea debug-textarea-sm" rows="4"></textarea>
        </div>

        <!-- Body -->
        <div class="debug-section" v-if="['POST', 'PUT', 'PATCH'].includes(debugMethod)">
          <label class="debug-label">Body</label>
          <textarea v-model="debugBody" class="debug-textarea" rows="8" placeholder='{"key": "value"}'></textarea>
        </div>

        <!-- Send Button -->
        <button @click="sendDebugRequest" :disabled="debugLoading" class="debug-send-btn">
          {{ debugLoading ? '发送中...' : '发送请求' }}
        </button>

        <!-- Response -->
        <div v-if="debugStatus !== null || debugError" class="debug-section">
          <label class="debug-label">响应</label>
          <div v-if="debugError" class="debug-error">{{ debugError }}</div>
          <div v-else>
            <div class="debug-status-line">
              <span class="status-badge" :class="debugStatus && debugStatus < 400 ? 'status-ok' : 'status-err'">
                {{ debugStatus }}
              </span>
            </div>
            <pre class="debug-response"><code>{{ debugResponse }}</code></pre>
          </div>
        </div>
      </aside>
    </div>
  </div>
</template>

<style scoped>
.api-docs {
  height: calc(100vh - 64px);
  display: flex;
  flex-direction: column;
  background: #f8f9fa;
}

.docs-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 24px;
  background: #fff;
  border-bottom: 1px solid #e5e7eb;
}

.docs-title {
  font-size: 20px;
  font-weight: 600;
  margin: 0;
}

.docs-meta {
  display: flex;
  align-items: center;
  gap: 12px;
}

.version-badge {
  background: #e0e7ff;
  color: #3730a3;
  font-size: 12px;
  padding: 2px 8px;
  border-radius: 4px;
}

.json-link {
  font-size: 12px;
  color: #6b7280;
  text-decoration: none;
  padding: 2px 8px;
  border: 1px solid #d1d5db;
  border-radius: 4px;
}
.json-link:hover { background: #f3f4f6; }

.loading-state {
  text-align: center;
  padding: 40px;
  color: #9ca3af;
}

.docs-body {
  display: flex;
  flex: 1;
  overflow: hidden;
}

/* Sidebar */
.docs-sidebar {
  width: 220px;
  min-width: 220px;
  background: #fff;
  border-right: 1px solid #e5e7eb;
  overflow-y: auto;
  padding: 8px 0;
}

.sidebar-module {
  padding: 8px 16px;
  cursor: pointer;
  transition: background 0.15s;
  border-left: 3px solid transparent;
}
.sidebar-module:hover { background: #f3f4f6; }
.sidebar-module.active {
  background: #eff6ff;
  border-left-color: #2563eb;
}
.sidebar-module-name {
  font-size: 13px;
  font-weight: 500;
  color: #1f2937;
}
.sidebar-module-count {
  font-size: 11px;
  color: #9ca3af;
  margin-top: 2px;
}

/* Main Content */
.docs-main {
  flex: 1;
  overflow-y: auto;
  padding: 24px;
}

.module-header {
  margin-bottom: 20px;
}
.module-header h2 {
  font-size: 18px;
  font-weight: 600;
  margin: 0 0 4px 0;
}
.module-desc {
  color: #6b7280;
  font-size: 13px;
  margin: 0;
}

/* Endpoint List */
.endpoint-list {
  margin-bottom: 24px;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  overflow: hidden;
  background: #fff;
}

.endpoint-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 16px;
  cursor: pointer;
  border-bottom: 1px solid #f3f4f6;
  transition: background 0.15s;
}
.endpoint-item:last-child { border-bottom: none; }
.endpoint-item:hover { background: #f9fafb; }
.endpoint-item.active { background: #eff6ff; }

.method-badge {
  font-size: 11px;
  font-weight: 600;
  padding: 2px 6px;
  border-radius: 4px;
  min-width: 48px;
  text-align: center;
  font-family: monospace;
}

.endpoint-path {
  font-family: monospace;
  font-size: 13px;
  color: #374151;
  flex: 1;
}

.endpoint-summary {
  font-size: 12px;
  color: #9ca3af;
  white-space: nowrap;
}

/* Endpoint Detail */
.endpoint-detail {
  background: #fff;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  padding: 24px;
}

.endpoint-detail h3 {
  font-size: 16px;
  font-weight: 600;
  margin: 0 0 12px 0;
}

.detail-meta {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
}

.detail-path {
  font-family: monospace;
  font-size: 14px;
  color: #1f2937;
}

.auth-badge {
  font-size: 11px;
  background: #fef3c7;
  color: #92400e;
  padding: 2px 8px;
  border-radius: 4px;
}

.detail-desc {
  color: #4b5563;
  font-size: 13px;
  line-height: 1.6;
  margin-bottom: 16px;
}

.detail-section {
  margin-top: 20px;
}
.detail-section h4 {
  font-size: 13px;
  font-weight: 600;
  color: #374151;
  margin: 0 0 8px 0;
}

/* Params Table */
.params-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
}
.params-table th {
  text-align: left;
  padding: 6px 10px;
  background: #f9fafb;
  color: #6b7280;
  font-weight: 500;
  border-bottom: 1px solid #e5e7eb;
}
.params-table td {
  padding: 6px 10px;
  border-bottom: 1px solid #f3f4f6;
}
.params-table code {
  background: #f3f4f6;
  padding: 1px 4px;
  border-radius: 3px;
  font-size: 12px;
}
.param-in {
  background: #e0e7ff;
  color: #3730a3;
  font-size: 10px;
  padding: 1px 4px;
  border-radius: 3px;
}

/* Code blocks */
.code-block {
  background: #1e293b;
  color: #e2e8f0;
  padding: 12px 16px;
  border-radius: 6px;
  font-size: 12px;
  overflow-x: auto;
  margin: 0;
}
.code-block.code-sm {
  margin-top: 6px;
  font-size: 11px;
  padding: 8px 12px;
}

/* Response items */
.response-item {
  margin-bottom: 12px;
}
.status-badge {
  display: inline-block;
  font-size: 12px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 4px;
  margin-right: 8px;
}
.status-ok { background: #d1fae5; color: #065f46; }
.status-err { background: #fee2e2; color: #991b1b; }
.resp-desc {
  font-size: 13px;
  color: #4b5563;
}

/* Debug Panel */
.docs-debug {
  width: 360px;
  min-width: 360px;
  background: #fff;
  border-left: 1px solid #e5e7eb;
  overflow-y: auto;
  padding: 16px;
}

.debug-title {
  font-size: 15px;
  font-weight: 600;
  margin: 0 0 16px 0;
  padding-bottom: 8px;
  border-bottom: 1px solid #e5e7eb;
}

.debug-section {
  margin-bottom: 12px;
}

.debug-label {
  display: block;
  font-size: 11px;
  font-weight: 500;
  color: #6b7280;
  margin-bottom: 4px;
}

.debug-select,
.debug-input {
  width: 100%;
  padding: 6px 10px;
  font-size: 12px;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  background: #fff;
  box-sizing: border-box;
}

.debug-url-row {
  display: flex;
  gap: 6px;
}

.debug-method {
  width: 80px;
  padding: 6px 4px;
  font-size: 12px;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  background: #fff;
  font-family: monospace;
}

.flex-1 { flex: 1; }

.debug-textarea {
  width: 100%;
  padding: 8px 10px;
  font-size: 11px;
  font-family: monospace;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  resize: vertical;
  box-sizing: border-box;
}
.debug-textarea-sm {
  font-size: 10px;
}

.debug-send-btn {
  width: 100%;
  padding: 8px;
  font-size: 13px;
  font-weight: 500;
  color: #fff;
  background: #2563eb;
  border: none;
  border-radius: 6px;
  cursor: pointer;
  margin: 8px 0;
}
.debug-send-btn:hover { background: #1d4ed8; }
.debug-send-btn:disabled { background: #93c5fd; cursor: not-allowed; }

.debug-error {
  background: #fee2e2;
  color: #991b1b;
  padding: 8px 12px;
  border-radius: 6px;
  font-size: 12px;
}

.debug-status-line {
  margin-bottom: 6px;
}

.debug-response {
  background: #1e293b;
  color: #e2e8f0;
  padding: 10px 12px;
  border-radius: 6px;
  font-size: 11px;
  max-height: 300px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-all;
  margin: 0;
}
</style>
