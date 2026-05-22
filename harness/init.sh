#!/usr/bin/env bash
# =============================================================================
# AI Harness — init.sh
# 项目初始化脚本模板
#
# 用法: ./init.sh
# 此脚本应该在 Session Zero 中由 Initializer Agent 填写
# =============================================================================
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_ROOT"

echo "=== AI Harness 项目初始化 ==="
echo "项目目录: $PROJECT_ROOT"
echo ""

# -----------------------------------------------------------------------------
# 1. 安装依赖
# -----------------------------------------------------------------------------
install_deps() {
    echo "[1/4] 安装依赖..."

    # Python 项目
    if [ -f "requirements.txt" ]; then
        pip install -r requirements.txt
    fi

    # Node.js 项目
    if [ -f "package.json" ]; then
        npm install
    fi

    # Go 项目
    if [ -f "go.mod" ]; then
        go mod download
    fi

    echo "  依赖安装完成"
}

# -----------------------------------------------------------------------------
# 2. 初始化数据库/外部服务 (如需要)
# -----------------------------------------------------------------------------
setup_services() {
    echo "[2/4] 设置外部服务..."

    # Docker Compose
    if [ -f "docker-compose.yml" ] || [ -f "docker-compose.yaml" ]; then
        docker compose up -d
        echo "  Docker 服务已启动"
    fi

    echo "  服务设置完成"
}

# -----------------------------------------------------------------------------
# 3. 运行数据库迁移 (如需要)
# -----------------------------------------------------------------------------
run_migrations() {
    echo "[3/4] 运行数据库迁移..."

    # Alembic
    if command -v alembic &> /dev/null; then
        alembic upgrade head
        echo "  数据库迁移完成"
    fi

    # Prisma
    if command -v npx &> /dev/null && [ -f "prisma/schema.prisma" ]; then
        npx prisma migrate dev
        echo "  Prisma 迁移完成"
    fi

    echo "  迁移完成"
}

# -----------------------------------------------------------------------------
# 4. 启动开发服务器
# -----------------------------------------------------------------------------
start_dev_server() {
    echo "[4/4] 启动开发服务器..."

    # FastAPI/Uvicorn
    if [ -f "main.py" ] && grep -q "FastAPI\|uvicorn" main.py 2>/dev/null; then
        uvicorn main:app --reload --host 0.0.0.0 --port 8000 &
        DEV_PID=$!
        echo "  FastAPI 开发服务器已启动 (PID: $DEV_PID, 端口: 8000)"
    fi

    # Node.js
    if grep -q '"dev"' package.json 2>/dev/null; then
        npm run dev &
        DEV_PID=$!
        echo "  Node.js 开发服务器已启动 (PID: $DEV_PID)"
    fi

    echo "  开发服务器已启动"
}

# -----------------------------------------------------------------------------
# 5. 冒烟测试
# -----------------------------------------------------------------------------
smoke_test() {
    echo ""
    echo "=== 运行冒烟测试 ==="

    # 等待服务器就绪
    sleep 3

    # HTTP 健康检查
    if curl -sf http://localhost:8000/health > /dev/null 2>&1; then
        echo "  ✓ 健康检查通过"
    else
        echo "  ⚠ 健康检查端点不可用 (可能未配置)"
    fi

    # 更多自定义测试...
    echo "  冒烟测试完成"
}

# -----------------------------------------------------------------------------
# 主流程
# -----------------------------------------------------------------------------
main() {
    install_deps
    setup_services
    run_migrations
    start_dev_server
    smoke_test

    echo ""
    echo "=== 初始化完成 ==="
    echo "开发服务器正在运行"
    echo ""
    echo "状态文件:"
    echo "  feature_list.json   — 功能列表"
    echo "  claude-progress.txt — 进度日志"
    echo ""
    echo "运行 coding agent:"
    echo "  claude -p \"\$(cat harness/prompts/coding-agent.txt)\""
}

main "$@"
