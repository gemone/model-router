# Model Router

<div align="center">

![Version](https://img.shields.io/badge/version-1.0.0-blue)
![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)
![Vue](https://img.shields.io/badge/Vue-3.5+-4FC08D?logo=vue.js)
![License](https://img.shields.io/badge/license-MIT-green)

一个高性能、多 Profile 的模型路由网关

支持 OpenAI、Anthropic、DeepSeek、Ollama 等多种供应商的 API 接入，提供统一的 OpenAI 兼容接口

[功能特性](#功能特性) • [快速开始](#快速开始) • [API 文档](#api-文档) • [配置说明](#配置说明) • [Web 管理界面](#web-管理界面)

</div>

## 功能特性

### 🚀 核心特性
- **多 Profile 支持** - 通过 URI 路径区分不同配置集，实现多租户隔离
- **高性能透传** - 非调试模式下直接流式转发，零拷贝处理，最大化性能
- **智能路由** - 支持优先级、加权轮询、自动选择等多种路由策略
- **自动降级** - 当主模型不可用时自动切换到后备模型，保障服务可用性

### 🔌 供应商支持
| 供应商 | 类型 | 特点 |
|--------|------|------|
| OpenAI | `openai` | 原生支持 GPT-4、GPT-3.5 系列 |
| Anthropic | `anthropic` | Claude 系列，自动格式转换 |
| Azure OpenAI | `azure` | 企业级部署支持 |
| DeepSeek | `deepseek` | 国产大模型 |
| Ollama | `ollama` | 本地模型部署 |
| OpenAI Compatible | `openai-compatible` | 通用兼容，支持任意 OpenAI-like API |

### 🎨 Web 管理界面
- **Dashboard** - 实时数据监控和统计图表
- **Profile 管理** - 可视化管理多个配置集
- **供应商管理** - 添加、测试、管理 API 供应商
- **模型管理** - 配置模型参数和能力标签
- **路由策略** - 配置智能路由规则
- **统计报表** - 多维度数据分析
- **请求日志** - 完整的请求追踪和调试
- **系统设置** - 集中化配置管理

### 📊 统计监控
- 请求量、延迟、错误率实时监控
- 按 Provider/Model 维度的统计报表
- 趋势图表和数据导出
- Token 使用量统计

## 技术栈

### 后端
- **Go 1.26+** - 高性能并发处理
- **Fiber v2** - 快速 HTTP 框架
- **GORM** - ORM 数据库操作
- **SQLite** - 轻量级数据存储

### 前端
- **Vue 3.5** - 渐进式框架
- **Vite 6** - 快速构建工具
- **Element Plus 2.9** - UI 组件库
- **ECharts** - 数据可视化
- **Vue I18n 11** - 国际化支持（中英文）

## 快速开始

### 安装

```bash
# 克隆仓库
git clone https://github.com/gemone/model-router.git
cd model-router

# 安装 Go 依赖
go mod download

# 安装前端依赖
cd web && npm install
```

### 开发模式

```bash
# 方式一：同时启动后端和前端（推荐）
make dev-full

# 方式二：分别启动
make dev-server  # 后端 :8080
cd web && npm run dev  # 前端 :5173
```

### 生产构建

```bash
# 构建完整应用（包含嵌入的 UI）
make build

# 运行
./bin/model-router
```

### Docker 部署

```bash
# 构建镜像
docker build -t model-router:latest .

# 运行容器
docker run -d \
  --name model-router \
  -p 8080:8080 \
  -v $(PWD)/data:/data \
  model-router:latest
```

## 首次使用配置

### 1. 访问管理界面

启动服务后，访问 http://localhost:8080 进入管理界面。

### 2. 添加供应商

1. 进入「供应商管理」页面
2. 点击「新增供应商」
3. 填写供应商信息：
   - **名称**：如 "OpenAI Official"
   - **类型**：选择对应类型（如 openai）
   - **Base URL**：`https://api.openai.com`
   - **API Key**：你的 API 密钥（加密存储）
4. 点击「测试连接」验证配置
5. 保存

### 3. 添加模型

1. 进入「模型管理」页面
2. 点击「新增模型」
3. 填写模型信息：
   - **对外名称**：`gpt-4`（客户端使用的名称）
   - **原始名称**：`gpt-4-turbo-preview`（API 实际调用的模型）
   - **供应商**：选择上一步添加的供应商
   - **能力标签**：勾选支持的特性（函数调用、视觉识别等）
4. 保存

### 4. 创建 Profile

1. 进入「Profile 管理」页面
2. 点击「新增 Profile」
3. 填写 Profile 信息：
   - **名称**：如 "Default"
   - **访问路径**：`default`
   - **选择模型**：勾选要包含的模型
4. 保存

### 5. 测试 API

```bash
curl http://localhost:8080/api/default/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

## API 文档

### 基础端点

```
POST /v1/chat/completions        # 使用默认 Profile
POST /api/{profile}/v1/chat/completions   # 使用指定 Profile
GET  /v1/models                  # 获取模型列表
POST /v1/embeddings              # 嵌入接口
```

### 格式兼容端点

```
# OpenAI 格式
POST /api/openai/{profile}/v1/chat/completions

# Anthropic/Claude 格式（自动转换）
POST /api/claude/{profile}/v1/messages
POST /api/anthropic/{profile}/v1/messages

# 简写格式
POST /{profile}/v1/chat/completions
```

### 请求示例

```bash
# 流式请求
curl http://localhost:8080/api/default/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "Tell me a joke."}
    ],
    "stream": true
  }'

# 带函数调用
curl http://localhost:8080/api/default/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "What is the weather in Boston?"}],
    "tools": [{
      "type": "function",
      "function": {
        "name": "get_weather",
        "parameters": {
          "type": "object",
          "properties": {
            "location": {"type": "string"}
          }
        }
      }
    }]
  }'
```

## 配置说明

### 环境变量

创建 `.env` 文件或设置环境变量：

```bash
# 服务配置
PORT=8080                    # 服务端口
HOST=0.0.0.0                 # 监听地址
LOG_LEVEL=info              # 日志级别: debug/info/warn/error

# 数据库
DB_PATH=~/.model-router/data.db

# 安全
JWT_SECRET=model-router-secret    # JWT 密钥
ADMIN_TOKEN=                       # 管理后台 Token（可选）

# 功能开关
ENABLE_STATS=true           # 启用统计
ENABLE_FALLBACK=true        # 启用自动降级
ENABLE_CORS=true            # 启用 CORS
```

### 路由策略

支持三种路由策略：

| 策略 | 说明 | 适用场景 |
|------|------|----------|
| `priority` | 优先级模式 | 按供应商优先级选择，高优先级优先 |
| `weighted` | 加权轮询 | 按权重分配请求，实现负载均衡 |
| `auto` | 自动选择 | 根据模型可用性自动选择 |

### 自动降级配置

```json
{
  "name": "gpt-4-route",
  "model_pattern": "gpt-4*",
  "strategy": "priority",
  "target_models": ["gpt-4-turbo", "gpt-4"],
  "fallback_enabled": true,
  "fallback_models": ["gpt-3.5-turbo"]
}
```

## Web 管理界面

### Dashboard 仪表盘

- 📊 实时统计卡片（请求数、成功率、延迟）
- 📈 请求趋势图表
- 🥧 热门模型分布
- 📋 最近请求日志

### Profile 管理

- 创建多个独立的配置集
- 每个独立的 URI 路径
- 灵活的模型组合

### 供应商管理

- 支持多种供应商类型
- API 密钥加密存储
- 连接测试功能
- 健康状态监控

### 模型管理

- 自定义模型名称映射
- 能力标签配置
- 价格设置
- 上下文窗口配置

### 统计报表

- 多时间范围统计
- Provider/Model 维度分析
- 数据导出（CSV）

### 请求日志

- 完整请求记录
- 请求/响应详情
- 筛选和搜索
- 性能分析

## 常见问题

### Q: 如何添加自定义供应商？

A: 选择 `openai-compatible` 类型，填写自定义的 Base URL 和 API Key。支持任何兼容 OpenAI API 格式的服务。

### Q: 模型名称映射的作用？

A: **对外名称**是客户端请求时使用的名称，**原始名称**是实际调用供应商 API 时使用的模型名。这样可以：
- 统一不同供应商的模型命名
- 隐藏底层供应商细节
- 灵活切换底层模型

### Q: 如何实现负载均衡？

A:
1. 为同一模型添加多个供应商
2. 设置不同的权重值
3. 选择 `weighted` 路由策略

### Q: 数据存储在哪里？

A: 默认存储在 `~/.model-router/data.db`，可通过 `DB_PATH` 环境变量修改。

### Q: 如何备份数据？

A: 备份 SQLite 数据库文件：
```bash
cp ~/.model-router/data.db ~/.model-router/data.db.backup
```

### Q: 支持 SSL/TLS 吗？

A: 可以通过反向代理（如 Nginx）配置 SSL：

```nginx
server {
    listen 443 ssl;
    server_name your-domain.com;
    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

## 项目结构

```
.
├── cmd/server/              # 服务端入口
├── internal/
│   ├── adapter/             # 供应商适配器
│   │   ├── anthropic/       # Claude 适配器
│   │   ├── deepseek/        # DeepSeek 适配器
│   │   ├── ollama/          # Ollama 适配器
│   │   ├── openai/          # OpenAI 适配器
│   │   └── openai_compatible/  # 通用兼容适配器
│   ├── config/              # 配置管理
│   ├── database/            # 数据库
│   ├── handler/             # HTTP 处理器
│   ├── model/               # 数据模型
│   ├── proxy/               # 高性能代理
│   ├── router/              # 智能路由
│   ├── service/             # 业务逻辑
│   ├── utils/               # 工具函数
│   └── web/                 # Web UI 嵌入
├── web/                     # Vue3 前端
│   ├── src/
│   │   ├── components/      # 组件
│   │   ├── views/           # 页面
│   │   ├── stores/          # 状态管理
│   │   └── i18n/            # 国际化
│   └── dist/                # 构建产物
├── configs/                 # 配置文件
├── Makefile                 # 构建脚本
└── README.md
```

## 开发指南

### 添加新的适配器

1. 在 `internal/adapter/` 下创建新目录
2. 实现 `adapter.Adapter` 接口：
   ```go
   type Adapter interface {
       ChatCompletion(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, error)
       ChatCompletionStream(ctx context.Context, req *ChatCompletionRequest) <-chan *ChatCompletionStreamResponse
       Embeddings(ctx context.Context, req *EmbeddingRequest) (*EmbeddingResponse, error)
   }
   ```
3. 在 `init()` 中注册适配器

### E2E 测试

项目包含完整的 E2E 测试套件，覆盖 API 端点、管理功能、路由策略和集成场景。

```bash
# 启动服务器并运行 E2E 测试
make dev &
sleep 3
make test-e2e

# 或使用 CI 模式（自动启动/停止服务器）
make test-e2e-ci

# 运行特定 E2E 测试套件
make test-e2e-api       # API 端点测试
make test-e2e-admin     # 管理功能测试
make test-e2e-router    # 路由策略测试
```

E2E 测试覆盖以下场景：
- **API 测试**: Chat Completions、Embeddings、Models 列表
- **管理测试**: Profile、Provider、Model 的 CRUD 操作
- **路由测试**: 优先级、加权、自动、延迟、健康度、成本策略
- **降级测试**: 自动降级、级联降级、备用模型
- **集成测试**: 流式响应、压缩功能、错误处理
- **复合模型测试**: 并行/串行策略、聚合方法

### 运行测试

```bash
# Go 单元测试
make test-go

# 前端测试
make test-web

# 前端测试 UI
make test-web-ui

# 覆盖率报告
make test-web-coverage

# E2E 测试（需要服务器运行）
make test-e2e

# E2E 测试（自动启动服务器）
make test-e2e-ci

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

## 构建发布

```bash
# 多平台构建
make release

# 输出文件：
# dist/model-router-darwin-amd64
# dist/model-router-darwin-arm64
# dist/model-router-linux-amd64
# dist/model-router-linux-arm64
# dist/model-router-windows-amd64.exe
```

## License

MIT License - 详见 [LICENSE](LICENSE) 文件

## 贡献

欢迎提交 Issue 和 Pull Request！

## 联系方式

- GitHub Issues: [github.com/gemone/model-router/issues](https://github.com/gemone/model-router/issues)
