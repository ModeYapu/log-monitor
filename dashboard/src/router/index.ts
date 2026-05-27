import { createRouter, createWebHistory, RouteRecordRaw } from 'vue-router'

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    name: 'Overview',
    component: () => import('../views/Overview.vue')
  },
  {
    path: '/logs/:appId?',
    name: 'Logs',
    component: () => import('../views/Logs.vue')
  },
  {
    path: '/performance/:appId?',
    name: 'Performance',
    component: () => import('../views/Performance.vue')
  },
  {
    path: '/alerts/:appId?',
    name: 'Alerts',
    component: () => import('../views/Alerts.vue')
  },
  {
    path: '/settings',
    name: 'Settings',
    component: () => import('../views/Settings.vue')
  },
  {
    path: '/live',
    name: 'Live',
    component: () => import('../views/Live.vue')
  },
  {
    path: '/recordings',
    name: 'Recordings',
    component: () => import('../views/Recordings.vue')
  },
  {
    path: '/recordings/:sessionId',
    name: 'RecordingPlayer',
    component: () => import('../views/Recordings.vue')
  }
]

const router = createRouter({
  history: createWebHistory('/logmon/'),
  routes
})

export default router
