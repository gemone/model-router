import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createRouter, createWebHistory } from 'vue-router'
import Breadcrumb from './Breadcrumb.vue'

const createWrapper = (routePath = '/') => {
  const router = createRouter({
    history: createWebHistory(),
    routes: [
      { path: '/', name: 'Home', meta: { title: 'Home' } },
      { path: '/dashboard', name: 'Dashboard', meta: { title: 'Dashboard' } },
      { path: '/profiles', name: 'Profiles', meta: { title: 'Profiles' } },
    ],
  })

  return mount(Breadcrumb, {
    global: {
      plugins: [router],
      mocks: {
        $t: (key) => key,
      },
    },
  })
}

describe('Breadcrumb', () => {
  it('should render breadcrumb with Home', () => {
    const wrapper = createWrapper('/')
    expect(wrapper.find('.el-breadcrumb').exists()).toBe(true)
  })

  it('should display correct translation key', async () => {
    const router = createRouter({
      history: createWebHistory(),
      routes: [
        { path: '/dashboard', name: 'Dashboard', meta: { title: 'Dashboard' } },
      ],
    })

    const wrapper = mount(Breadcrumb, {
      global: {
        plugins: [router],
        mocks: {
          $t: vi.fn((key) => `translated:${key}`),
        },
      },
    })

    await router.push('/dashboard')
    await router.isReady()

    expect(wrapper.text()).toContain('translated:')
  })
})
