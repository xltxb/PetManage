<template>
  <div class="flex h-screen">
    <!-- Sidebar -->
    <aside class="w-60 bg-[var(--color-sidebar)] text-[var(--color-canvas)] flex flex-col">
      <div class="p-5 text-xl font-bold tracking-wide border-b border-white/10">
        🐾 爪迹 PawPrint
      </div>
      <nav class="flex-1 p-3 space-y-1 overflow-y-auto">
        <router-link v-for="item in navItems" :key="item.to" :to="item.to"
          class="flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm hover:bg-white/10 transition-colors"
          active-class="bg-[var(--color-coral)] text-white">
          <span>{{ item.icon }}</span>
          <span>{{ item.label }}</span>
        </router-link>
      </nav>
      <div class="p-3 border-t border-white/10 text-xs text-white/50">
        {{ auth.currentRole }} · 旗舰店
      </div>
    </aside>
    <!-- Main -->
    <main class="flex-1 overflow-y-auto bg-[var(--color-canvas)]">
      <header class="h-14 bg-[var(--color-surface)] border-b border-black/5 flex items-center justify-between px-6">
        <h1 class="text-lg font-semibold text-[var(--color-ink)]">{{ pageTitle }}</h1>
        <div class="flex items-center gap-3">
          <select v-model="selectedStore" @change="switchStore"
            class="text-sm bg-transparent border rounded px-2 py-1 text-[var(--color-ink)]">
            <option v-for="s in auth.stores" :key="s.id" :value="s.id">{{ s.name }}</option>
          </select>
          <button @click="auth.logout" class="text-sm text-[var(--color-berry)] hover:underline">退出</button>
        </div>
      </header>
      <div class="p-6">
        <router-view />
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const route = useRoute()
const auth = useAuthStore()
const selectedStore = ref(auth.currentStoreId)

const navItems = [
  { to: '/', label: '经营概览', icon: '📊' },
  { to: '/appointments', label: '预约管理', icon: '📅' },
  { to: '/boarding', label: '寄养业务', icon: '🏠' },
  { to: '/pets', label: '宠物档案', icon: '🐕' },
  { to: '/members', label: '会员客户', icon: '👥' },
  { to: '/inventory', label: '商品库存', icon: '📦' },
  { to: '/settlements', label: '结算收银', icon: '💰' },
  { to: '/analytics', label: '数据分析', icon: '📈' },
  { to: '/settings', label: '系统设置', icon: '⚙️' },
]

const pageTitle = computed(() => {
  const item = navItems.find((n) => n.to === route.path)
  return item?.label || '爪迹'
})

function switchStore() {
  auth.switchStore(selectedStore.value)
  window.location.reload()
}
</script>
