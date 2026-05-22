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

echo "=== 宠物店管理系统 初始化 ==="
echo "项目目录: $PROJECT_ROOT"
echo ""

install_deps() {
    echo "[1/4] 安装 Go 依赖..."
    if [ -f "go.mod" ]; then
        go mod download
        go mod tidy
    fi
    echo "  Go 依赖安装完成"

    echo "[2/4] 安装前端依赖..."
    if [ -f "frontend/package.json" ]; then
        cd frontend
        npm install
        cd "$PROJECT_ROOT"
    fi
    echo "  前端依赖安装完成"
}

setup_services() {
    echo "[3/4] 检查外部服务..."
    # Supabase / PostgreSQL
    if command -v psql &> /dev/null && pg_isready &> /dev/null 2>&1; then
        echo "  PostgreSQL 已运行"
    else
        echo "  ⚠ PostgreSQL 未运行 (使用 Supabase 云服务可忽略)"
    fi
    # Redis
    if command -v redis-cli &> /dev/null && redis-cli ping &> /dev/null 2>&1; then
        echo "  Redis 已运行"
    else
        echo "  ⚠ Redis 未运行，部分功能不可用"
    fi
}

start_server() {
    echo "[4/4] 启动开发服务器..."

    # Go 后端
    go run cmd/server/main.go &
    BACKEND_PID=$!
    echo "  后端已启动 (PID: $BACKEND_PID, 端口: 8080)"

    # Vue 3 前端
    cd frontend
    npm run dev &
    FRONTEND_PID=$!
    cd "$PROJECT_ROOT"
    echo "  前端已启动 (PID: $FRONTEND_PID, 端口: 3000)"

    sleep 3
    if curl -sf http://localhost:8080/health > /dev/null 2>&1; then
        echo "  ✓ 后端健康检查通过"
    else
        echo "  ⚠ 后端健康检查端点不可用"
    fi

    if curl -sf http://localhost:3000 > /dev/null 2>&1; then
        echo "  ✓ 前端服务可用"
    else
        echo "  ⚠ 前端服务未响应"
    fi
}

main() {
    install_deps
    setup_services
    start_server

    echo ""
    echo "=== 初始化完成 ==="
    echo "  后端:  http://localhost:8080"
    echo "  前端:  http://localhost:3000"
    echo ""
    echo "运行 coding agent:"
    echo "  claude -p \"\$(cat harness/prompts/coding-agent.txt)\""
    echo ""
    echo "持续运行模式:"
    echo "  ./harness/runner.sh --auto"
}

main "$@"
