#!/bin/bash
# Model-Router 持续验证主入口

set -e

BASE_URL="${BASE_URL:-http://localhost:8080}"
FRONTEND_URL="${FRONTEND_URL:-http://localhost:5173}"
OLLAMA_HOST="${OLLAMA_HOST:-http://localhost:11434}"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
RESULTS_DIR="$PROJECT_ROOT/test_results"

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

# 确保目录存在
mkdir -p "$RESULTS_DIR"/{api,frontend,ollama,compare}

# 测试计数
TOTAL_PASSED=0
TOTAL_FAILED=0

run_tests() {
    local category=$1
    local script=$2
    local env_prefix="BASE_URL=$BASE_URL FRONTEND_URL=$FRONTEND_URL OLLAMA_HOST=$OLLAMA_HOST"

    echo ""
    echo -e "${BLUE}════════════════════════════════════════${NC}"
    echo -e "${BLUE}  $category${NC}"
    echo -e "${BLUE}════════════════════════════════════════${NC}"

    if [ -f "$SCRIPT_DIR/$script" ]; then
        cd "$PROJECT_ROOT"
        if eval "$env_prefix bash \"$SCRIPT_DIR/$script\""; then
            return 0
        else
            return 1
        fi
    else
        warn "$script not found, skipping"
        return 0
    fi
}

# ==================== 主流程 ====================

echo ""
echo -e "${BLUE}╔════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║      Model-Router 持续验证                         ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════╝${NC}"
echo ""
echo "Configuration:"
echo "  Backend:  $BASE_URL"
echo "  Frontend: $FRONTEND_URL"
echo "  Ollama:   $OLLAMA_HOST"
echo "  Results:  $RESULTS_DIR"
echo ""

# ==================== 服务状态检查 ====================

echo -e "${BLUE}════════════════════════════════════════${NC}"
echo -e "${BLUE}  Service Status${NC}"
echo -e "${BLUE}════════════════════════════════════════${NC}"

# 检查后端
BACKEND_STATUS=$(curl -sf -o /dev/null -w "%{http_code}" "$BASE_URL/v1/models" 2>/dev/null || echo "000")
if [ "$BACKEND_STATUS" = "200" ]; then
    pass "Backend service: OK (HTTP $BACKEND_STATUS)"
else
    fail "Backend service: NOT OK (HTTP $BACKEND_STATUS)"
    fail "Please start the backend server first: go run cmd/server/main.go"
    exit 1
fi

# 检查 Ollama（可选）
OLLAMA_STATUS=$(curl -sf -o /dev/null -w "%{http_code}" "$OLLAMA_HOST/api/tags" 2>/dev/null || echo "000")
if [ "$OLLAMA_STATUS" = "200" ]; then
    pass "Ollama service: OK (HTTP $OLLAMA_STATUS)"
else
    warn "Ollama service: NOT AVAILABLE (HTTP $OLLAMA_STATUS)"
    warn "Ollama tests will be skipped"
fi

# ==================== 运行测试模块 ====================

MODULES=(
    "API 接口:api-test.sh"
    "前端页面:frontend-test.sh"
    "Ollama 模型:ollama-test.sh"
    "模型对比:model-compare.sh"
)

FAILED_MODULES=0

for module in "${MODULES[@]}"; do
    IFS=':' read -r name script <<< "$module"
    if ! run_tests "$name" "$script"; then
        ((FAILED_MODULES++))
    fi
done

# ==================== 生成报告 ====================

echo ""
echo -e "${BLUE}════════════════════════════════════════${NC}"
echo -e "${BLUE}  Summary Report${NC}"
echo -e "${BLUE}════════════════════════════════════════${NC}"
echo ""

# 收集结果
for dir in api frontend ollama compare; do
    if [ -f "$RESULTS_DIR/$dir/summary.txt" ]; then
        PASSED=$(grep "^passed:" "$RESULTS_DIR/$dir/summary.txt" | cut -d: -f2 | tr -d ' ')
        FAILED=$(grep "^failed:" "$RESULTS_DIR/$dir/summary.txt" | cut -d: -f2 | tr -d ' ')
        [ -n "$PASSED" ] && TOTAL_PASSED=$((TOTAL_PASSED + PASSED))
        [ -n "$FAILED" ] && TOTAL_FAILED=$((TOTAL_FAILED + FAILED))
    fi
done

echo "Test Results:"
echo "  Total Passed: $TOTAL_PASSED"
echo "  Total Failed: $TOTAL_FAILED"
echo "  Failed Modules: $FAILED_MODULES"
echo ""

# 保存最终报告
REPORT_FILE="$RESULTS_DIR/report_$(date +%Y%m%d_%H%M%S).json"
cat > "$REPORT_FILE" <<EOF
{
  "timestamp": "$(date -Iseconds)",
  "config": {
    "base_url": "$BASE_URL",
    "frontend_url": "$FRONTEND_URL",
    "ollama_host": "$OLLAMA_HOST"
  },
  "summary": {
    "total_passed": $TOTAL_PASSED,
    "total_failed": $TOTAL_FAILED,
    "failed_modules": $FAILED_MODULES
  }
}
EOF

echo "Report saved to: $REPORT_FILE"
echo ""

# ==================== 最终状态 ====================

if [ $TOTAL_FAILED -eq 0 ] && [ $FAILED_MODULES -eq 0 ]; then
    echo -e "${GREEN}╔════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║  ✓ All validations passed!                        ║${NC}"
    echo -e "${GREEN}╚════════════════════════════════════════════════════╝${NC}"
    exit 0
else
    echo -e "${RED}╔════════════════════════════════════════════════════╗${NC}"
    echo -e "${RED}║  ✗ Some validations failed                        ║${NC}"
    echo -e "${RED}╚════════════════════════════════════════════════════╝${NC}"
    exit 1
fi
