import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      name: 'home',
      component: () => import('@/views/Dashboard.vue'),
    },
    {
      path: '/login',
      name: 'login',
      component: () => import('@/views/Login.vue'),
    },
    {
      path: '/merchant/login',
      name: 'merchant-login',
      component: () => import('@/views/MerchantLogin.vue'),
    },
    {
      path: '/merchant',
      name: 'merchant',
      component: () => import('@/views/MerchantDashboard.vue'),
    },
  ],
})

export default router
