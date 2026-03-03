#!/bin/bash
# 多模型对比测试脚本

set -e

OLLAMA_HOST="${OLLAMA_HOST:-http://localhost:11434}"
BASE_URL="${BASE_URL:-http://localhost:8080}"
RESULTS_DIR="test_results/compare"
mkdir -p "$RESULTS_DIR"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

pass() { echo -e "${GREEN}✓${NC} $1"; }
fail() { echo -e "${RED}✗${NC} $1"; }
warn() { echo -e "${YELLOW}!${NC} $1"; }
info() { echo -e "${BLUE}ℹ${NC} $1"; }

echo "=== 多模型对比测试 ==="
echo "Ollama Host: $OLLAMA_HOST"
echo "Backend URL: $BASE_URL"
echo ""

# ==================== 检查 Ollama 可用性 ====================

OLLAMA_STATUS=$(curl -sf -o /dev/null -w "%{http_code}" "$OLLAMA_HOST/api/tags" 2>/dev/null || echo "000")
if [ "$OLLAMA_STATUS" != "200" ]; then
    warn "Ollama service not accessible, skipping comparison"
    echo "skipped: true" > "$RESULTS_DIR/summary.txt"
    exit 0
fi

# ==================== 获取模型列表 ====================

MODELS_JSON=$(curl -sf "$OLLAMA_HOST/api/tags" 2>/dev/null)
MODELS=$(echo "$MODELS_JSON" | jq -r '.models[].name' | head -5)

if [ -z "$MODELS" ]; then
    fail "No models available for comparison"
    exit 1
fi

info "Models to compare:"
echo "$MODELS" | while read line; do
    echo "  - $line"
done

# ==================== 测试场景 ====================

RESULTS_FILE="$RESULTS_DIR/comparison_$(date +%Y%m%d_%H%M%S).json"
echo '{"comparisons": []}' > "$RESULTS_FILE"

run_comparison() {
    local scenario="$1"
    local prompt="$2"

    echo ""
    echo "--- Scenario: $scenario ---"
    echo "Prompt: $prompt"
    echo ""

    echo "  Model           | Latency  | Tokens | Response"
    echo "  ----------------|----------|--------|------------------"

    for model in $MODELS; do
        model=$(echo "$model" | tr -d '"')
        [ -z "$model" ] && continue

        local start=$(python3 -c 'import time; print(int(time.time() * 1000))')

        local response
        response=$(curl -sf --max-time 60 "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -d "{\"model\":\"$model\",\"messages\":[{\"role\":\"user\",\"content\":\"$prompt\"}],\"max_tokens\":50}" \
            2>/dev/null)

        local exit_code=$?
        local end=$(python3 -c 'import time; print(int(time.time() * 1000))')
        local latency=$((end - start))

        if [ $exit_code -eq 0 ] && [ -n "$response" ]; then
            local content=$(echo "$response" | jq -r '.choices[0].message.content // "N/A"' | head -c 30)
            local tokens=$(echo "$response" | jq -r '.usage.total_tokens // 0')

            printf "  %-15s | %6dms | %6s | %s\n" "$model" "$latency" "$tokens" "$content"

            # 记录到 JSON
            local entry=$(cat <<EOF
{
  "scenario": "$scenario",
  "model": "$model",
  "latency_ms": $latency,
  "tokens": $tokens,
  "response": $(echo "$response" | jq '.choices[0].message.content' | head -c 100)
}
EOF
            )
            jq ".comparisons += [$entry]" "$RESULTS_FILE" > "${RESULTS_FILE}.tmp" && mv "${RESULTS_FILE}.tmp" "$RESULTS_FILE"
        else
            printf "  %-15s | %6s | %6s | %s\n" "$model" "TIMEOUT" "-" "-"
        fi
    done
}

# 场景 1: 简单数学
run_comparison "Simple Math" "What is 25 * 4?"

# 场景 2: 代码生成
run_comparison "Code Generation" "Write a Python function to check if a number is prime."

# 场景 3: 推理能力
run_comparison "Reasoning" "If all bloops are bleeps, and some bleeps are blops, can we conclude that some bloops are blops? Explain."

# 场景 4: 创意写作
run_comparison "Creative" "Write a haiku about programming."

# ==================== 统计汇总 ====================

echo ""
echo "--- Summary ---"

if [ -f "$RESULTS_FILE" ]; then
    echo "Detailed results saved to: $RESULTS_FILE"

    # 计算平均延迟
    jq -r '
        .comparisons |
        group_by(.model) |
        .[] |
        {
            model: .[0].model,
            avg_latency: (map(.latency_ms) | add / length),
            tests: length
        } |
        "\(.model): avg \(.avg_latency | floor)ms across \(.tests) tests"
    ' "$RESULTS_FILE"
fi

echo ""
echo "skipped: false" > "$RESULTS_DIR/summary.txt"
echo "results_file: $RESULTS_FILE" >> "$RESULTS_DIR/summary.txt"
echo "timestamp: $(date -Iseconds)" >> "$RESULTS_DIR/summary.txt"
