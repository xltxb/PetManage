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
      component: () => import('@/views/OrderList.vue'),
    },
    {
      path: '/merchant/appointments',
      name: 'merchant-appointments',
      component: () => import('@/views/AppointmentList.vue'),
    },
    {
      path: '/merchant/members',
      name: 'merchant-members',
      component: () => import('@/views/MemberList.vue'),
    },
    {
      path: '/merchant/members/:id',
      name: 'merchant-member-detail',
      component: () => import('@/views/MemberDetail.vue'),
    },
    {
      path: '/merchant/health-reminders',
      name: 'merchant-health-reminders',
      component: () => import('@/views/HealthReminders.vue'),
    },
    {
      path: '/merchant/inventory',
      name: 'merchant-inventory',
      component: () => import('@/views/Placeholder.vue'),
    },
    {
      path: '/merchant/inventory/alerts',
      name: 'merchant-inventory-alerts',
      component: () => import('@/views/InventoryAlerts.vue'),
    },
    {
      path: '/merchant/services',
      name: 'merchant-services',
      component: () => import('@/views/Placeholder.vue'),
    },
    {
      path: '/merchant/pos',
      name: 'merchant-pos',
      component: () => import('@/views/MerchantPos.vue'),
    },
    {
      path: '/merchant/categories',
      name: 'merchant-categories',
      component: () => import('@/views/MerchantCategories.vue'),
    },
    {
      path: '/merchant/roles',
      name: 'merchant-roles',
      component: () => import('@/views/MerchantRoles.vue'),
    },
    {
      path: '/merchant/schedules',
      name: 'merchant-schedules',
      component: () => import('@/views/ScheduleCalendar.vue'),
    },
    {
      path: '/merchant/settings',
      name: 'merchant-settings',
      component: () => import('@/views/MerchantSettings.vue'),
    },
    {
      path: '/merchant/receipt-template',
      name: 'merchant-receipt-template',
      component: () => import('@/views/ReceiptTemplate.vue'),
    },
    {
      path: '/merchant/verification',
      name: 'merchant-verification',
      component: () => import('@/views/Verification.vue'),
    },
    {
      path: '/merchants/:id/analysis',
      name: 'merchant-analysis',
      component: () => import('@/views/MerchantAnalysis.vue'),
    },
  ],
})

export default router
