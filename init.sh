#!/usr/bin/env bash
# =============================================================================
# Pet Store Management System — init.sh
# 开发环境初始化脚本
#
# 用法: ./init.sh
# =============================================================================
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$PROJECT_ROOT"

echo "=== Pet Store Management System 初始化 ==="
echo "项目目录: $PROJECT_ROOT"
echo ""

install_deps() {
    echo "[1/3] 安装 Go 依赖..."
    if [ -f "go.mod" ]; then
        go mod download
        go mod tidy
    fi
    echo "  依赖安装完成"
}

setup_services() {
    echo "[2/3] 检查外部服务..."
    # Redis
    if command -v redis-cli &> /dev/null && redis-cli ping &> /dev/null; then
        echo "  Redis 已运行"
    else
        echo "  ⚠ Redis 未运行，部分功能不可用"
    fi
    # MySQL
    if command -v mysql &> /dev/null && mysql -u root -e "SELECT 1" &> /dev/null 2>&1; then
        echo "  MySQL 已运行"
    else
        echo "  ⚠ MySQL 未运行，部分功能不可用"
    fi
}

start_server() {
    echo "[3/3] 启动开发服务器..."
    go run cmd/server/main.go &
    DEV_PID=$!
    echo "  开发服务器已启动 (PID: $DEV_PID, 端口: 8080)"

    sleep 2
    if curl -sf http://localhost:8080/health > /dev/null 2>&1; then
        echo "  ✓ 健康检查通过"
    else
        echo "  ⚠ 健康检查端点不可用"
    fi
}

main() {
    install_deps
    setup_services
    start_server

    echo ""
    echo "=== 初始化完成 ==="
    echo ""
    echo "运行 coding agent:"
    echo "  claude -p \"\$(cat harness/prompts/coding-agent.txt)\""
    echo ""
    echo "持续运行模式:"
    echo "  ./harness/runner.sh --auto"
}

main "$@"
