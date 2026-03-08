#!/bin/bash

# Model Router API 格式测试脚本
# 测试 OpenAI、Claude、Ollama 三种格式

set -e

BASE_URL="${BASE_URL:-http://localhost:8080}"
PROFILE="${PROFILE:-default}"

echo "=========================================="
echo "Testing Model Router API Formats"
echo "Base URL: $BASE_URL"
echo "Profile: $PROFILE"
echo "=========================================="

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# 测试函数
test_api() {
    local name=$1
    local url=$2
    local data=$3

    echo ""
    echo "Testing: $name"
    echo "URL: $url"
    
    if response=$(curl -s -w "\n%{http_code}" "$url" \
        -H "Content-Type: application/json" \
        -d "$data" 2>/dev/null); then
        
        http_code=$(echo "$response" | tail -n1)
        body=$(echo "$response" | sed '$d')
        
        if [ "$http_code" -eq 200 ] || [ "$http_code" -eq 201 ]; then
            echo -e "${GREEN}✓ Success (HTTP $http_code)${NC}"
            echo "Response: $(echo "$body" | head -c 200)..."
        else
            echo -e "${RED}✗ Failed (HTTP $http_code)${NC}"
            echo "Response: $body"
        fi
    else
        echo -e "${RED}✗ Request failed${NC}"
    fi
}

# 1. OpenAI 格式
echo ""
echo "=========================================="
echo "1. OpenAI Format"
echo "=========================================="

test_api "OpenAI Chat Completions" \
    "$BASE_URL/api/$PROFILE/v1/chat/completions" \
    '{"model":"gpt-4","messages":[{"role":"user","content":"Hello!"}]}'

test_api "OpenAI Models List" \
    "$BASE_URL/api/$PROFILE/v1/models" \
    '{}'

# 2. Claude/Anthropic 格式
echo ""
echo "=========================================="
echo "2. Claude/Anthropic Format"
echo "=========================================="

test_api "Claude Messages" \
    "$BASE_URL/api/claude/$PROFILE/v1/messages" \
    '{"model":"claude-3-opus","messages":[{"role":"user","content":"Hello!"}],"max_tokens":1024}'

test_api "Anthropic Messages" \
    "$BASE_URL/api/anthropic/$PROFILE/v1/messages" \
    '{"model":"claude-3-opus","messages":[{"role":"user","content":"Hello!"}],"max_tokens":1024}'

# 3. Ollama 格式
echo ""
echo "=========================================="
echo "3. Ollama Format"
echo "=========================================="

test_api "Ollama Chat" \
    "$BASE_URL/api/ollama/$PROFILE/api/chat" \
    '{"model":"llama3","messages":[{"role":"user","content":"Hello!"}]}'

test_api "Ollama Generate" \
    "$BASE_URL/api/ollama/$PROFILE/api/generate" \
    '{"model":"llama3","prompt":"Hello!"}'

test_api "Ollama List Models" \
    "$BASE_URL/api/ollama/$PROFILE/api/tags" \
    '{}'

# 4. 带规则的请求测试
echo ""
echo "=========================================="
echo "4. Rule-Based Routing Tests"
echo "=========================================="

test_api "Premium User Header" \
    "$BASE_URL/api/$PROFILE/v1/chat/completions" \
    '{"model":"gpt-4","messages":[{"role":"user","content":"Hello!"}]}'

echo ""
echo "Testing with custom headers..."
curl -s "$BASE_URL/api/$PROFILE/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -H "X-User-Tier: premium" \
    -d '{"model":"gpt-4","messages":[{"role":"user","content":"Hello!"}]}' | \
    head -c 200
echo ""

# 5. 流式请求测试
echo ""
echo "=========================================="
echo "5. Streaming Tests"
echo "=========================================="

echo ""
echo "Testing OpenAI Streaming..."
curl -s "$BASE_URL/api/$PROFILE/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -d '{"model":"gpt-4","messages":[{"role":"user","content":"Hi"}],"stream":true}' | \
    head -20

echo ""
echo "Testing Ollama Streaming..."
curl -s "$BASE_URL/api/ollama/$PROFILE/api/chat" \
    -H "Content-Type: application/json" \
    -d '{"model":"llama3","messages":[{"role":"user","content":"Hi"}],"stream":true}' | \
    head -20

echo ""
echo "=========================================="
echo "Tests completed!"
echo "=========================================="
