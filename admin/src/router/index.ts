import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const routes = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('../views/auth/Login.vue'),
    meta: { guest: true },
  },
  {
    path: '/',
    component: () => import('../layouts/AppShell.vue'),
    children: [
      { path: '', name: 'Dashboard', component: () => import('../views/dashboard/Dashboard.vue') },
      { path: 'appointments', name: 'Appointments', component: () => import('../views/appointment/AppointmentList.vue') },
      { path: 'boarding', name: 'Boarding', component: () => import('../views/boarding/BoardingList.vue') },
      { path: 'pets', name: 'Pets', component: () => import('../views/pet/PetList.vue') },
      { path: 'members', name: 'Members', component: () => import('../views/member/MemberList.vue') },
      { path: 'inventory', name: 'Inventory', component: () => import('../views/inventory/InventoryList.vue') },
      { path: 'settlements', name: 'Settlements', component: () => import('../views/settlement/SettlementList.vue') },
      { path: 'analytics', name: 'Analytics', component: () => import('../views/analytics/AnalyticsView.vue') },
      { path: 'settings', name: 'Settings', component: () => import('../views/setting/SettingView.vue') },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach((to, _from, next) => {
  const auth = useAuthStore()
  if (to.meta.guest) {
    if (auth.isLoggedIn) return next('/')
    return next()
  }
  if (!auth.isLoggedIn) return next('/login')
  next()
})

export default router
