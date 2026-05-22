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
    {
      path: '/merchant/products',
      name: 'merchant-products',
      component: () => import('@/views/MerchantProducts.vue'),
    },
    {
      path: '/merchant/orders',
      name: 'merchant-orders',
      component: () => import('@/views/Placeholder.vue'),
    },
    {
      path: '/merchant/appointments',
      name: 'merchant-appointments',
      component: () => import('@/views/Placeholder.vue'),
    },
    {
      path: '/merchant/members',
      name: 'merchant-members',
      component: () => import('@/views/Placeholder.vue'),
    },
    {
      path: '/merchant/inventory',
      name: 'merchant-inventory',
      component: () => import('@/views/Placeholder.vue'),
    },
    {
      path: '/merchant/services',
      name: 'merchant-services',
      component: () => import('@/views/Placeholder.vue'),
    },
    {
      path: '/merchant/categories',
      name: 'merchant-categories',
      component: () => import('@/views/MerchantCategories.vue'),
    },
    {
      path: '/merchant/settings',
      name: 'merchant-settings',
      component: () => import('@/views/MerchantSettings.vue'),
    },
    {
      path: '/merchants/:id/analysis',
      name: 'merchant-analysis',
      component: () => import('@/views/MerchantAnalysis.vue'),
    },
  ],
})

export default router
