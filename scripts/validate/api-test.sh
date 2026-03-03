#!/bin/bash
# API 接口验证脚本

set -e

BASE_URL="${BASE_URL:-http://localhost:8080}"
RESULTS_DIR="test_results/api"
mkdir -p "$RESULTS_DIR"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

pass() { echo -e "${GREEN}✓${NC} $1"; }
fail() { echo -e "${RED}✗${NC} $1"; echo "  Response: $2"; }
warn() { echo -e "${YELLOW}!${NC} $1"; }
info() { echo "  ℹ $1"; }

# 测试计数器
TESTS_PASSED=0
TESTS_FAILED=0

test_api() {
    local name="$1"
    local method="$2"
    local endpoint="$3"
    local data="$4"
    local expected_status="${5:-200}"

    local url="${BASE_URL}${endpoint}"
    local response
    local http_code

    if [ -n "$data" ]; then
        response=$(curl -sf -w "\n%{http_code}" -X "$method" "$url" \
            -H "Content-Type: application/json" \
            -d "$data" 2>/dev/null || echo -e "\n000")
    else
        response=$(curl -sf -w "\n%{http_code}" -X "$method" "$url" 2>/dev/null || echo -e "\n000")
    fi

    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | sed '$d')

    if [ "$http_code" = "$expected_status" ]; then
        pass "$name (HTTP $http_code)"
        ((TESTS_PASSED++))
        echo "$name: PASS" >> "$RESULTS_DIR/results.log"
    else
        fail "$name (HTTP $http_code, expected $expected_status)" "$body"
        ((TESTS_FAILED++))
        echo "$name: FAIL - $http_code" >> "$RESULTS_DIR/results.log"
    fi
}

test_api_structure() {
    local name="$1"
    local endpoint="$2"
    local jq_check="$3"

    local url="${BASE_URL}${endpoint}"
    local body
    body=$(curl -sf "$url" 2>/dev/null)

    if echo "$body" | jq -e "$jq_check" > /dev/null 2>&1; then
        pass "$name"
        ((TESTS_PASSED++))
    else
        fail "$name - structure check failed: $jq_check" "$body"
        ((TESTS_FAILED++))
    fi
}

echo "=== API 接口验证 ==="
echo "Base URL: $BASE_URL"
echo ""

# ==================== 基础 API ====================

echo "--- Models API ---"
test_api "GET /v1/models" "GET" "/v1/models" "" 200
test_api_structure "Models response has 'object' field" "/v1/models" '.object == "list"'
test_api_structure "Models response has 'data' array" "/v1/models" '.data | type == "array"'

echo ""
echo "--- Chat Completions API ---"

# 获取第一个可用模型
FIRST_MODEL=$(curl -sf "${BASE_URL}/api/admin/models" 2>/dev/null | jq -r '.[0].name // empty')
if [ -z "$FIRST_MODEL" ]; then
    warn "No models configured, skipping chat tests"
else
    info "Using model: $FIRST_MODEL"

    # 非流式请求
    test_api "POST /v1/chat/completions (non-stream)" "POST" "/v1/chat/completions" \
        "{\"model\":\"$FIRST_MODEL\",\"messages\":[{\"role\":\"user\",\"content\":\"hi\"}],\"max_tokens\":10}" 200

    # 检查响应结构
    CHAT_RESPONSE=$(curl -sf "${BASE_URL}/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -d "{\"model\":\"$FIRST_MODEL\",\"messages\":[{\"role\":\"user\",\"content\":\"say ok\"}],\"max_tokens\":5}" 2>/dev/null)

    if echo "$CHAT_RESPONSE" | jq -e '.choices[0].message.content' > /dev/null 2>&1; then
        pass "Chat response has content"
        ((TESTS_PASSED++))
    else
        fail "Chat response missing content" "$CHAT_RESPONSE"
        ((TESTS_FAILED++))
    fi
fi

# Profile 路径测试
test_api "GET /:profile/v1/models" "GET" "/default/v1/models" "" 200

echo ""
echo "--- Embeddings API ---"
# Embeddings 可能不被所有模型支持，标记为可选
EMBEDDING_RESPONSE=$(curl -sf -w "\n%{http_code}" -X POST "${BASE_URL}/v1/embeddings" \
    -H "Content-Type: application/json" \
    -d "{\"model\":\"$FIRST_MODEL\",\"input\":\"test\"}" 2>/dev/null || echo -e "\n000")
EMBEDDING_CODE=$(echo "$EMBEDDING_RESPONSE" | tail -n1)
if [ "$EMBEDDING_CODE" = "200" ]; then
    pass "POST /v1/embeddings (HTTP 200)"
    ((TESTS_PASSED++))
else
    warn "POST /v1/embeddings returned HTTP $EMBEDDING_CODE (model may not support embeddings)"
fi

# ==================== Admin API ====================

echo ""
echo "--- Admin: Profiles ---"
test_api "GET /api/admin/profiles" "GET" "/api/admin/profiles" "" 200
test_api_structure "Profiles is array" "/api/admin/profiles" '. | type == "array"'

echo ""
echo "--- Admin: Providers ---"
test_api "GET /api/admin/providers" "GET" "/api/admin/providers" "" 200
test_api_structure "Providers is array" "/api/admin/providers" '. | type == "array"'

echo ""
echo "--- Admin: Models ---"
test_api "GET /api/admin/models" "GET" "/api/admin/models" "" 200
test_api_structure "Models is array" "/api/admin/models" '. | type == "array"'

echo ""
echo "--- Admin: Stats ---"
test_api "GET /api/admin/stats/dashboard" "GET" "/api/admin/stats/dashboard" "" 200
test_api "GET /api/admin/stats/trend" "GET" "/api/admin/stats/trend" "" 200
test_api "GET /api/admin/stats/all" "GET" "/api/admin/stats/all" "" 200

echo ""
echo "--- Admin: Logs ---"
test_api "GET /api/admin/logs" "GET" "/api/admin/logs" "" 200

echo ""
echo "--- Admin: Settings ---"
test_api "GET /api/admin/settings" "GET" "/api/admin/settings" "" 200

echo ""
echo "--- Admin: Test Endpoint ---"
# 期望 500 因为 provider_id 是假的
test_api "POST /api/admin/test (expect error)" "POST" "/api/admin/test" \
    '{"provider_id":"test","model":"test"}' 500

# ==================== 错误处理 ====================

echo ""
echo "--- Error Handling ---"
test_api "Invalid profile returns 404" "POST" "/invalid_profile/v1/chat/completions" \
    '{"model":"test","messages":[]}' 404

# ==================== 总结 ====================

echo ""
echo "===================="
echo "API 测试结果: $TESTS_PASSED 通过, $TESTS_FAILED 失败"
echo "===================="

# 保存总结
echo "passed: $TESTS_PASSED" > "$RESULTS_DIR/summary.txt"
echo "failed: $TESTS_FAILED" >> "$RESULTS_DIR/summary.txt"
echo "timestamp: $(date -Iseconds)" >> "$RESULTS_DIR/summary.txt"

if [ $TESTS_FAILED -gt 0 ]; then
    exit 1
fi
