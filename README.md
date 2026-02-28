# Model Router

一个高性能、多 Profile 的模型路由网关，支持 OpenAI、Anthropic、DeepSeek、Ollama 等多种供应商的 API 接入，提供统一的 OpenAI 兼容接口。

## 技术栈

- **后端**: Go 1.23 + Gin + GORM + SQLite
- **前端**: Vue 3.5 + Vite 6 + Element Plus 2.9 + Vue I18n 11
- **测试**: Vitest + @vue/test-utils (前端), Go testing (后端)

## 特性

- **多 Profile 支持**：通过 URI 路径区分不同配置集，如 `/api/default/v1/chat/completions`、`/api/claudecode/v1/chat/completions`
- **高性能透传**：非调试模式下直接流式转发，零拷贝处理，最大化性能
- **智能路由**：支持优先级、加权轮询、自动选择等多种路由策略
- **自动降级**：当主模型不可用时自动切换到后备模型
- **多供应商支持**：OpenAI、Anthropic、Azure、DeepSeek、Ollama、OpenAI Compatible
- **Web 管理界面**：内置 Vue3 管理界面，支持 i18n（中英文）
- **统计监控**：请求量、延迟、错误率等实时监控
- **模型测试**：内置模型连通性测试

## 快速开始

### 安装依赖

```bash
# 安装 Go 依赖
go mod download

# 安装前端依赖
cd web && npm install
```

### 开发模式

```bash
# 同时启动后端和前端
make dev-full

# 或分别启动
make dev-server  # 后端
cd web && npm run dev  # 前端
```

### 生产构建

```bash
# 构建完整应用（包含嵌入的 UI）
make build

# 运行
./bin/model-router
```

### Docker 运行

```bash
docker build -t model-router .
docker run -p 8080:8080 model-router
```

## API 使用

### OpenAI 兼容接口

```bash
# 使用默认 Profile
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'

# 使用指定 Profile
curl http://localhost:8080/api/claudecode/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-opus",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

### 支持的 URI 格式

- `/v1/chat/completions` - 默认 Profile
- `/api/{profile}/v1/chat/completions` - 指定 Profile
- `/api/openai/{profile}/v1/chat/completions` - OpenAI 格式
- `/api/claude/{profile}/v1/messages` - Claude 格式

## 配置

### 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `PORT` | 服务端口 | `8080` |
| `HOST` | 监听地址 | `0.0.0.0` |
| `DB_PATH` | 数据库路径 | `~/.model-router/data.db` |
| `ADMIN_TOKEN` | 管理后台 Token | - |
| `JWT_SECRET` | JWT 密钥 | `model-router-secret` |
| `LOG_LEVEL` | 日志级别 | `info` |
| `ENABLE_STATS` | 启用统计 | `true` |
| `ENABLE_FALLBACK` | 启用自动降级 | `true` |

## 项目结构

```
.
├── cmd/server/          # 服务端入口
├── internal/
│   ├── adapter/         # 供应商适配器
│   │   ├── anthropic/   # Claude 适配器
│   │   ├── deepseek/    # DeepSeek 适配器
│   │   ├── ollama/      # Ollama 适配器
│   │   ├── openai/      # OpenAI 适配器
│   │   └── openai_compatible/  # 通用兼容适配器
│   ├── config/          # 配置管理
│   ├── database/        # 数据库
│   ├── handler/         # HTTP 处理器
│   ├── model/           # 数据模型
│   ├── proxy/           # 高性能代理
│   ├── service/         # 业务逻辑
│   ├── utils/           # 工具函数
│   └── web/             # Web UI 嵌入
├── web/                 # Vue3 前端
└── Makefile
```

## 适配器说明

### 支持的供应商

| 供应商 | 类型 | 特点 |
|--------|------|------|
| OpenAI | `openai` | 原生支持 |
| Anthropic | `anthropic` | Claude 系列，自动格式转换 |
| Azure OpenAI | `azure` | 企业级部署 |
| DeepSeek | `deepseek` | 国产大模型 |
| Ollama | `ollama` | 本地模型，支持自定义模型 |
| OpenAI Compatible | `openai-compatible` | 通用兼容，支持任意 OpenAI-like API |

### 自定义模型

使用 `openai-compatible` 类型可以接入任何兼容 OpenAI API 的服务，支持任意自定义模型名称。

## 开发

### 添加新的适配器

1. 在 `internal/adapter/` 下创建新目录
2. 实现 `adapter.Adapter` 接口
3. 在 `init()` 中注册适配器

### 前端开发

```bash
cd web
npm install
npm run dev
```

前端使用 Vue3 + Element Plus + ECharts + Vue I18n，支持国际化。

### 运行测试

```bash
# Go 测试
make test-go

# 前端测试
make test-web

# 前端测试 UI 模式
make test-web-ui

# 覆盖率报告
make test-web-coverage

# 所有测试
make test
```

### 代码检查

```bash
# 格式化代码
make fmt

# 代码检查
make lint

# 类型检查
make typecheck
```

## License

MIT
