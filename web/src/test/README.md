# 前端测试指南

## 测试结构

```
test/
├── setup.js              # 测试环境设置
├── integration/          # 集成测试
│   └── api.test.js      # API 集成测试
└── README.md            # 本文档

stores/
└── app.test.js          # Store 单元测试

components/
└── Breadcrumb.test.js   # 组件单元测试

views/
└── Dashboard.test.js    # 视图单元测试
```

## 运行测试

```bash
# 运行所有测试
npm test

# 运行测试并监听文件变化
npm run test:watch

# 运行测试并打开 UI
npm run test:ui

# 运行测试并生成覆盖率报告
npm run test:coverage
```

## 测试类型

### 单元测试

测试独立的函数、组件或 store。

```javascript
// stores/app.test.js
import { describe, it, expect } from 'vitest'
import { useAppStore } from './app'

describe('App Store', () => {
  it('should toggle sidebar', () => {
    const store = useAppStore()
    expect(store.sidebarCollapsed).toBe(false)
    store.toggleSidebar()
    expect(store.sidebarCollapsed).toBe(true)
  })
})
```

### 组件测试

测试 Vue 组件的渲染和交互。

```javascript
// components/Breadcrumb.test.js
import { mount } from '@vue/test-utils'
import Breadcrumb from './Breadcrumb.vue'

describe('Breadcrumb', () => {
  it('should render', () => {
    const wrapper = mount(Breadcrumb)
    expect(wrapper.find('.el-breadcrumb').exists()).toBe(true)
  })
})
```

### 集成测试

测试 API 调用和数据流。

```javascript
// test/integration/api.test.js
describe('API Integration', () => {
  it('should fetch profiles', async () => {
    const response = await axios.get('/api/admin/profiles')
    expect(response.data).toBeDefined()
  })
})
```

## 测试工具

- **Vitest**: 测试框架
- **@vue/test-utils**: Vue 组件测试工具
- **happy-dom**: DOM 模拟
- **msw** (可选): API 模拟

## 最佳实践

1. **每个测试独立**: 使用 `beforeEach` 重置状态
2. **模拟外部依赖**: 使用 `vi.mock()` 模拟 axios 等
3. **测试行为而非实现**: 关注用户可见的行为
4. **使用语义化的描述**: `it('should do something when...')`
5. **保持测试简单**: 一个测试只验证一个概念

## 覆盖率目标

- 组件: > 70%
- Store: > 80%
- 工具函数: > 90%
