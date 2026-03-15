import { createRouter, createWebHashHistory } from 'vue-router'
import Dashboard from '@/views/Dashboard.vue'
import Profiles from '@/views/Profiles.vue'
import Providers from '@/views/Providers.vue'
import Models from '@/views/Models.vue'
import Routes from '@/views/Routes.vue'
import Rules from '@/views/Rules.vue'
import Stats from '@/views/Stats.vue'
import Logs from '@/views/Logs.vue'
import ServerLogs from '@/views/ServerLogs.vue'
import Settings from '@/views/Settings.vue'
import Login from '@/views/Login.vue'

const routes = [
  {
    path: '/login',
    name: 'Login',
    component: Login,
    meta: { public: true }
  },
  {
    path: '/',
    redirect: '/dashboard'
  },
  {
    path: '/dashboard',
    name: 'Dashboard',
    component: Dashboard,
    meta: { titleKey: 'nav.dashboard', icon: 'Odometer', requiresAuth: true }
  },

  {
    path: '/profiles',
    name: 'Profiles',
    component: Profiles,
    meta: { titleKey: 'nav.profiles', icon: 'Grid', requiresAuth: true }
  },
  {
    path: '/providers',
    name: 'Providers',
    component: Providers,
    meta: { titleKey: 'nav.providers', icon: 'Connection', requiresAuth: true }
  },
  {
    path: '/models',
    name: 'Models',
    component: Models,
    meta: { titleKey: 'nav.models', icon: 'Cpu', requiresAuth: true }
  },
  {
    path: '/routes',
    name: 'Routes',
    component: Routes,
    meta: { titleKey: 'nav.routes', icon: 'Share', requiresAuth: true }
  },
  {
    path: '/rules',
    name: 'Rules',
    component: Rules,
    meta: { titleKey: 'nav.rules', icon: 'Filter', requiresAuth: true }
  },
  {
    path: '/stats',
    name: 'Stats',
    component: Stats,
    meta: { titleKey: 'nav.stats', icon: 'TrendCharts', requiresAuth: true }
  },
  {
    path: '/logs',
    name: 'Logs',
    component: Logs,
    meta: { titleKey: 'nav.logs', icon: 'List', requiresAuth: true }
  },
  {
    path: '/server-logs',
    name: 'ServerLogs',
    component: ServerLogs,
    meta: { titleKey: 'nav.serverlogs', icon: 'Monitor', requiresAuth: true }
  },
  {
    path: '/settings',
    name: 'Settings',
    component: Settings,
    meta: { titleKey: 'nav.settings', icon: 'Setting', requiresAuth: true }
  }
]

const router = createRouter({
  history: createWebHashHistory(),
  routes
})

// Navigation guard for authentication
router.beforeEach((to, from, next) => {
  const token = localStorage.getItem('admin_token')

  if (to.meta.requiresAuth && !token) {
    // Redirect to login if trying to access protected route without token
    next({ path: '/login', query: { redirect: to.fullPath } })
  } else if (to.path === '/login' && token) {
    // Redirect to dashboard if already logged in
    next({ path: '/dashboard' })
  } else {
    next()
  }
})

export default router
