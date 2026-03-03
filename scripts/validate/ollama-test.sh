#!/bin/bash
# Ollama 模型验证脚本

set -e

OLLAMA_HOST="${OLLAMA_HOST:-http://localhost:11434}"
BASE_URL="${BASE_URL:-http://localhost:8080}"
RESULTS_DIR="test_results/ollama"
mkdir -p "$RESULTS_DIR"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

pass() { echo -e "${GREEN}✓${NC} $1"; }
fail() { echo -e "${RED}✗${NC} $1"; }
warn() { echo -e "${YELLOW}!${NC} $1"; }

TESTS_PASSED=0
TESTS_FAILED=0

echo "=== Ollama 模型验证 ==="
echo "Ollama Host: $OLLAMA_HOST"
echo "Backend URL: $BASE_URL"
echo ""

# ==================== Ollama 服务检查 ====================

echo "--- Ollama Service ---"

OLLAMA_STATUS=$(curl -sf -o /dev/null -w "%{http_code}" "$OLLAMA_HOST/api/tags" 2>/dev/null || echo "000")
if [ "$OLLAMA_STATUS" = "200" ]; then
    pass "Ollama service accessible (HTTP $OLLAMA_STATUS)"
    ((TESTS_PASSED++))
else
    warn "Ollama service not accessible (HTTP $OLLAMA_STATUS)"
    warn "Skipping Ollama tests"
    echo "ollama_unavailable: true" > "$RESULTS_DIR/summary.txt"
    exit 0  # Ollama 是可选的
fi

# ==================== 获取模型列表 ====================

echo ""
echo "--- Available Models ---"

MODELS_JSON=$(curl -sf "$OLLAMA_HOST/api/tags" 2>/dev/null)
echo "$MODELS_JSON" | jq -r '.models[].name' > "$RESULTS_DIR/models.txt"

MODEL_COUNT=$(wc -l < "$RESULTS_DIR/models.txt" | tr -d ' ')
if [ "$MODEL_COUNT" -gt 0 ]; then
    pass "Found $MODEL_COUNT model(s)"
    ((TESTS_PASSED++))
    echo "Available models:"
    cat "$RESULTS_DIR/models.txt" | while read line; do
        echo "  - $line"
    done
else
    fail "No models found"
    ((TESTS_FAILED++))
fi

# ==================== 模型响应测试 ====================

echo ""
echo "--- Model Response Tests ---"

TEST_PROMPT="Say 'OK' if you can hear me."
MAX_TOKENS=10
TIMEOUT=30

test_model() {
    local model="$1"
    local model_clean=$(echo "$model" | tr -d '"')

    echo "Testing: $model_clean"

    local start=$(python3 -c 'import time; print(int(time.time() * 1000))')

    local response
    response=$(curl -sf --max-time $TIMEOUT "$OLLAMA_HOST/api/generate" \
        -H "Content-Type: application/json" \
        -d "{\"model\":\"$model_clean\",\"prompt\":\"$TEST_PROMPT\",\"stream\":false,\"options\":{\"num_predict\":$MAX_TOKENS}}" \
        2>/dev/null)

    local exit_code=$?
    local end=$(python3 -c 'import time; print(int(time.time() * 1000))')
    local latency=$((end - start))

    if [ $exit_code -eq 0 ] && [ -n "$response" ]; then
        local content=$(echo "$response" | jq -r '.response // empty' | head -c 50)
        local done=$(echo "$response" | jq -r '.done // false')

        if [ "$done" = "true" ]; then
            pass "$model_clean: ${latency}ms - '$content'"
            echo "$model_clean: PASS - ${latency}ms" >> "$RESULTS_DIR/results.log"
            ((TESTS_PASSED++))
            return 0
        else
            fail "$model_clean: incomplete response"
            echo "$model_clean: FAIL - incomplete" >> "$RESULTS_DIR/results.log"
            ((TESTS_FAILED++))
            return 1
        fi
    else
        fail "$model_clean: request failed (timeout or error)"
        echo "$model_clean: FAIL - request error" >> "$RESULTS_DIR/results.log"
        ((TESTS_FAILED++))
        return 1
    fi
}

# 测试最多 5 个模型
TESTED=0
MAX_MODELS=5

while read model && [ $TESTED -lt $MAX_MODELS ]; do
    model=$(echo "$model" | tr -d '"')
    if [ -n "$model" ]; then
        test_model "$model"
        ((TESTED++))
    fi
done < "$RESULTS_DIR/models.txt"

# ==================== 通过 Backend 测试 ====================

echo ""
echo "--- Backend Proxy Tests ---"

# 从 Backend API 获取配置的模型名
CONFIGURED_MODEL=$(curl -sf "$BASE_URL/api/admin/models" 2>/dev/null | jq -r '.[0].name // empty')

if [ -n "$CONFIGURED_MODEL" ]; then
    echo "Testing through backend with configured model: $CONFIGURED_MODEL"

    BACKEND_RESPONSE=$(curl -sf --max-time $TIMEOUT "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -d "{\"model\":\"$CONFIGURED_MODEL\",\"messages\":[{\"role\":\"user\",\"content\":\"$TEST_PROMPT\"}],\"max_tokens\":$MAX_TOKENS}" \
        2>/dev/null)

    if [ $? -eq 0 ] && [ -n "$BACKEND_RESPONSE" ]; then
        CONTENT=$(echo "$BACKEND_RESPONSE" | jq -r '.choices[0].message.content // empty' | head -c 50)
        if [ -n "$CONTENT" ]; then
            pass "Backend proxy: $CONTENT"
            ((TESTS_PASSED++))
        else
            fail "Backend proxy: empty response"
            ((TESTS_FAILED++))
        fi
    else
        fail "Backend proxy: request failed"
        ((TESTS_FAILED++))
    fi
else
    warn "No configured model found in backend, skipping backend proxy test"
fi

# ==================== 总结 ====================

echo ""
echo "===================="
echo "Ollama 测试结果: $TESTS_PASSED 通过, $TESTS_FAILED 失败"
echo "===================="

echo "passed: $TESTS_PASSED" > "$RESULTS_DIR/summary.txt"
echo "failed: $TESTS_FAILED" >> "$RESULTS_DIR/summary.txt"
echo "models_tested: $TESTED" >> "$RESULTS_DIR/summary.txt"
echo "timestamp: $(date -Iseconds)" >> "$RESULTS_DIR/summary.txt"

if [ $TESTS_FAILED -gt 0 ]; then
    exit 1
fi
