import { createRouter, createWebHistory, RouteRecordRaw } from 'vue-router'

declare module 'vue-router' {
  interface RouteMeta {
    public?: boolean
    requiresAuth?: boolean
    requiresAdmin?: boolean
  }
}

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('../views/Login.vue'),
    meta: { public: true }
  },
  {
    path: '/',
    name: 'Overview',
    component: () => import('../views/Overview.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/logs/:appId?',
    name: 'Logs',
    component: () => import('../views/Logs.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/performance/:appId?',
    name: 'Performance',
    component: () => import('../views/Performance.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/alerts/:appId?',
    name: 'Alerts',
    component: () => import('../views/Alerts.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/users',
    name: 'UserManagement',
    component: () => import('../views/UserManagement.vue'),
    meta: { requiresAuth: true, requiresAdmin: true }
  },
  {
    path: '/settings',
    name: 'Settings',
    component: () => import('../views/Settings.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/live',
    name: 'Live',
    component: () => import('../views/Live.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/recordings',
    name: 'Recordings',
    component: () => import('../views/Recordings.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/recordings/:sessionId',
    name: 'RecordingPlayer',
    component: () => import('../views/Recordings.vue'),
    meta: { requiresAuth: true }
  }
]

const router = createRouter({
  history: createWebHistory('/logmon/'),
  routes
})

// Navigation guard for authentication
router.beforeEach((to, from, next) => {
  const token = localStorage.getItem('logmon_token')
  const userStr = localStorage.getItem('logmon_user')

  // Public routes don't require authentication
  if (to.meta.public) {
    // If already logged in, redirect to home
    if (token && to.path === '/login') {
      next('/')
      return
    }
    next()
    return
  }

  // Check if token exists
  if (!token) {
    next('/login')
    return
  }

  // Check admin role for admin-only routes
  if (to.meta.requiresAdmin) {
    if (userStr) {
      try {
        const user = JSON.parse(userStr)
        if (user.role !== 'admin') {
          next('/')
          return
        }
      } catch {
        next('/login')
        return
      }
    } else {
      next('/login')
      return
    }
  }

  next()
})

export default router
