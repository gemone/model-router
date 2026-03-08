# Model Router 使用示例

本目录包含 Model Router 的各种使用示例。

## 目录结构

```
examples/
├── profiles/           # Profile 配置示例
├── routes/            # 路由配置示例
├── rules/             # 规则配置示例
└── requests/          # API 请求示例
```

## 快速开始

### 1. 基础 Profile（直接模型绑定）

```bash
# 使用 curl 测试
curl http://localhost:8080/api/default/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

### 2. 不同 API 格式

#### OpenAI 格式
```bash
curl http://localhost:8080/api/openai/default/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

#### Claude/Anthropic 格式
```bash
curl http://localhost:8080/api/claude/default/v1/messages \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{
    "model": "claude-3-opus",
    "messages": [{"role": "user", "content": "Hello!"}],
    "max_tokens": 1024
  }'
```

#### Ollama 格式
```bash
# Chat API
curl http://localhost:8080/api/ollama/default/api/chat \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama3",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'

# Generate API
curl http://localhost:8080/api/ollama/default/api/generate \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama3",
    "prompt": "Hello!"
  }'

# List Models
curl http://localhost:8080/api/ollama/default/api/tags
```

## 更多示例

查看各个子目录获取更多详细示例。
