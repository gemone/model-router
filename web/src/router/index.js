import { createRouter, createWebHashHistory } from 'vue-router'
import Dashboard from '@/views/Dashboard.vue'
import Profiles from '@/views/Profiles.vue'
import Providers from '@/views/Providers.vue'
import Models from '@/views/Models.vue'
import Routes from '@/views/Routes.vue'
import Stats from '@/views/Stats.vue'
import Logs from '@/views/Logs.vue'
import ServerLogs from '@/views/ServerLogs.vue'
import Settings from '@/views/Settings.vue'

const routes = [
  {
    path: '/',
    redirect: '/dashboard'
  },
  {
    path: '/dashboard',
    name: 'Dashboard',
    component: Dashboard,
    meta: { title: '仪表盘', icon: 'Odometer' }
  },

  {
    path: '/profiles',
    name: 'Profiles',
    component: Profiles,
    meta: { title: 'Profile 管理', icon: 'Grid' }
  },
  {
    path: '/providers',
    name: 'Providers',
    component: Providers,
    meta: { title: '供应商管理', icon: 'Connection' }
  },
  {
    path: '/models',
    name: 'Models',
    component: Models,
    meta: { title: '模型管理', icon: 'Cpu' }
  },
  {
    path: '/routes',
    name: 'Routes',
    component: Routes,
    meta: { title: '路由策略', icon: 'Share' }
  },
  {
    path: '/stats',
    name: 'Stats',
    component: Stats,
    meta: { title: '统计数据', icon: 'TrendCharts' }
  },
  {
    path: '/logs',
    name: 'Logs',
    component: Logs,
    meta: { title: '请求日志', icon: 'List' }
  },
  {
    path: '/server-logs',
    name: 'ServerLogs',
    component: ServerLogs,
    meta: { title: '服务器日志', icon: 'Monitor' }
  },
  {
    path: '/settings',
    name: 'Settings',
    component: Settings,
    meta: { title: '系统设置', icon: 'Setting' }
  }
]

const router = createRouter({
  history: createWebHashHistory(),
  routes
})

export default router
