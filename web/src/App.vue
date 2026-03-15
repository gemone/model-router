<template>
  <div class="app-container">
    <el-container v-if="!isLogin" class="main-container">
      <el-aside :width="sidebarWidth" class="sidebar">
        <div class="logo">
          <el-icon class="logo-icon"><Connection /></el-icon>
          <span v-show="!collapsed" class="logo-text">Model Router</span>
        </div>
        <el-menu
          :default-active="activeMenu"
          :collapse="collapsed"
          :collapse-transition="false"
          router
          class="nav-menu"
          background-color="#304156"
          text-color="#bfcbd9"
          active-text-color="#409EFF"
        >
          <el-menu-item v-for="route in routes" :key="route.path" :index="route.path">
            <el-icon>
              <component :is="route.meta.icon" />
            </el-icon>
            <template #title>{{ $t(route.meta?.titleKey) }}</template>
          </el-menu-item>
        </el-menu>
        <div class="sidebar-footer">
          <el-button
            link
            class="collapse-btn"
            @click="toggleSidebar"
          >
            <el-icon>
              <Fold v-if="!collapsed" />
              <Expand v-else />
            </el-icon>
          </el-button>
        </div>
      </el-aside>

      <el-container class="content-container">
        <el-header class="header">
          <div class="header-left">
            <breadcrumb />
          </div>
          <div class="header-right">
            <el-dropdown @command="handleLanguageChange">
              <el-button link>
                <el-icon><Globe /></el-icon>
                {{ currentLanguageLabel }}
              </el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="zh-CN">中文</el-dropdown-item>
                  <el-dropdown-item command="en-US">English</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>

            <el-dropdown @command="handleCommand">
              <el-button link>
                <el-icon><User /></el-icon>
                Admin
              </el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="settings">
                    <el-icon><Setting /></el-icon>
                    {{ $t('nav.settings') }}
                  </el-dropdown-item>
                  <el-dropdown-item divided command="logout">
                    <el-icon><SwitchButton /></el-icon>
                    Logout
                  </el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </div>
        </el-header>

        <el-main class="main-content">
          <router-view v-slot="{ Component }">
            <transition name="fade" mode="out-in">
              <component :is="Component" />
            </transition>
          </router-view>
        </el-main>
      </el-container>
    </el-container>

    <router-view v-else />
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import Breadcrumb from '@/components/Breadcrumb.vue'

const route = useRoute()
const router = useRouter()
const { locale } = useI18n()
const store = useAppStore()

const collapsed = computed(() => store.sidebarCollapsed)
const sidebarWidth = computed(() => collapsed.value ? '64px' : '210px')
const isLogin = computed(() => route.path === '/login')

const activeMenu = computed(() => route.path)

const routes = router.getRoutes().filter(r => r.meta?.titleKey)

const currentLanguage = ref(locale.value)
const currentLanguageLabel = computed(() => {
  return currentLanguage.value === 'zh-CN' ? '中文' : 'English'
})

function toggleSidebar() {
  store.toggleSidebar()
}

function handleLanguageChange(lang) {
  locale.value = lang
  currentLanguage.value = lang
  localStorage.setItem('locale', lang)
}

async function handleCommand(command) {
  switch (command) {
    case 'settings':
      router.push('/settings')
      break
    case 'logout':
      await store.logout()
      router.push('/login')
      break
  }
}

// Initialize locale from localStorage
const savedLocale = localStorage.getItem('locale')
if (savedLocale) {
  locale.value = savedLocale
  currentLanguage.value = savedLocale
}
</script>

<style scoped>
.app-container {
  width: 100%;
  height: 100%;
}

.main-container {
  width: 100%;
  height: 100%;
}

.content-container {
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
}

.sidebar {
  background-color: #304156;
  transition: width 0.3s;
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
}

.logo {
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-size: 18px;
  font-weight: bold;
  border-bottom: 1px solid #1f2d3d;
  flex-shrink: 0;
}

.logo-icon {
  font-size: 24px;
  margin-right: 8px;
}

.logo-text {
  white-space: nowrap;
}

.nav-menu {
  flex: 1;
  border-right: none;
  overflow-y: auto;
}

.sidebar-footer {
  height: 50px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-top: 1px solid #1f2d3d;
  flex-shrink: 0;
}

.collapse-btn {
  color: #bfcbd9;
  font-size: 18px;
}

.collapse-btn:hover {
  color: #fff;
}

.header {
  background-color: #fff;
  box-shadow: 0 1px 4px rgba(0, 21, 41, 0.08);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
  flex-shrink: 0;
  height: 60px;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 16px;
}

.main-content {
  background-color: #f0f2f5;
  padding: 24px;
  overflow-y: auto;
  flex: 1;
}

/* Transition animations */
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.3s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
