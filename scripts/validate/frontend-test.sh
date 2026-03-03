#!/bin/bash
# 前端页面验证脚本

set -e

FRONTEND_URL="${FRONTEND_URL:-http://localhost:5173}"
BASE_URL="${BASE_URL:-http://localhost:8080}"
RESULTS_DIR="test_results/frontend"
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

echo "=== 前端页面验证 ==="
echo "Frontend URL: $FRONTEND_URL"
echo "Backend URL: $BASE_URL"
echo ""

# ==================== 前端资源检查 ====================

echo "--- Frontend Resources ---"

# 检查前端服务
FRONTEND_STATUS=$(curl -sf -o /dev/null -w "%{http_code}" "$FRONTEND_URL" 2>/dev/null || echo "000")
if [ "$FRONTEND_STATUS" = "200" ]; then
    pass "Frontend server accessible (HTTP $FRONTEND_STATUS)"
    ((TESTS_PASSED++))
else
    warn "Frontend server not accessible (HTTP $FRONTEND_STATUS) - checking dist..."
    # 检查 dist 目录是否存在
    if [ -d "internal/web/dist/index.html" ]; then
        pass "Frontend dist exists, backend will serve it"
        ((TESTS_PASSED++))
    else
        fail "No frontend available"
        ((TESTS_FAILED++))
    fi
fi

# 检查后端提供的静态文件
DIST_STATUS=$(curl -sf -o /dev/null -w "%{http_code}" "$BASE_URL/" 2>/dev/null || echo "000")
if [ "$DIST_STATUS" = "200" ]; then
    pass "Backend serves frontend (HTTP $DIST_STATUS)"
    ((TESTS_PASSED++))
else
    fail "Backend does not serve frontend (HTTP $DIST_STATUS)"
    ((TESTS_FAILED++))
fi

# ==================== 页面路由检查 ====================

echo ""
echo "--- Page Routes ---"

PAGES=(
    "/"
    "/dashboard"
    "/profiles"
    "/providers"
    "/models"
    "/stats"
    "/logs"
    "/settings"
)

for page in "${PAGES[@]}"; do
    # 前端路由会返回相同的 HTML（SPA）
    STATUS=$(curl -sf -o /dev/null -w "%{http_code}" "$BASE_URL$page" 2>/dev/null || echo "000")
    if [ "$STATUS" = "200" ]; then
        pass "Page accessible: $page"
        ((TESTS_PASSED++))
    else
        fail "Page not accessible: $page (HTTP $STATUS)"
        ((TESTS_FAILED++))
    fi
done

# ==================== API 代理检查 ====================

echo ""
echo "--- API Proxy ---"

# 检查前端是否能正确代理 API 请求
PROXY_CHECK=$(curl -sf "$BASE_URL/api/admin/stats/dashboard" 2>/dev/null)
if [ $? -eq 0 ] && [ -n "$PROXY_CHECK" ]; then
    pass "API proxy working"
    ((TESTS_PASSED++))
else
    fail "API proxy not working"
    ((TESTS_FAILED++))
fi

# ==================== 静态资源检查 ====================

echo ""
echo "--- Static Assets ---"

# 检查 JS 和 CSS 资源
INDEX_HTML=$(curl -sf "$BASE_URL/" 2>/dev/null)

if [ -n "$INDEX_HTML" ]; then
    # 提取 JS 文件
    JS_FILES=$(echo "$INDEX_HTML" | grep -oE 'src="[^"]*\.js"' | grep -oE '/assets/[^"]+')
    CSS_FILES=$(echo "$INDEX_HTML" | grep -oE 'href="[^"]*\.css"' | grep -oE '/assets/[^"]+')

    JS_COUNT=$(echo "$JS_FILES" | grep -c "js" || echo "0")
    CSS_COUNT=$(echo "$CSS_FILES" | grep -c "css" || echo "0")

    if [ "$JS_COUNT" -gt 0 ]; then
        pass "Found $JS_COUNT JS file(s)"
        ((TESTS_PASSED++))
    else
        fail "No JS files found"
        ((TESTS_FAILED++))
    fi

    if [ "$CSS_COUNT" -gt 0 ]; then
        pass "Found $CSS_COUNT CSS file(s)"
        ((TESTS_PASSED++))
    else
        fail "No CSS files found"
        ((TESTS_FAILED++))
    fi

    # 验证资源可访问
    for js in $JS_FILES; do
        JS_STATUS=$(curl -sf -o /dev/null -w "%{http_code}" "$BASE_URL$js" 2>/dev/null || echo "000")
        if [ "$JS_STATUS" = "200" ]; then
            pass "JS asset accessible: $(basename $js)"
            ((TESTS_PASSED++))
        else
            fail "JS asset not accessible: $js (HTTP $JS_STATUS)"
            ((TESTS_FAILED++))
        fi
    done
else
    fail "Could not fetch index.html"
    ((TESTS_FAILED++))
fi

# ==================== 总结 ====================

echo ""
echo "===================="
echo "前端测试结果: $TESTS_PASSED 通过, $TESTS_FAILED 失败"
echo "===================="

echo "passed: $TESTS_PASSED" > "$RESULTS_DIR/summary.txt"
echo "failed: $TESTS_FAILED" >> "$RESULTS_DIR/summary.txt"
echo "timestamp: $(date -Iseconds)" >> "$RESULTS_DIR/summary.txt"

if [ $TESTS_FAILED -gt 0 ]; then
    exit 1
fi
