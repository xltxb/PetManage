#!/usr/bin/env bash
# =============================================================================
# AI Harness — status.sh
# 快速查看项目开发状态
# =============================================================================
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_ROOT"

echo ""
echo "════════════════════════════════════════════════════════════"
echo "  AI HARNESS — 项目状态"
echo "════════════════════════════════════════════════════════════"
echo ""

# Git 状态
if git rev-parse --git-dir > /dev/null 2>&1; then
    BRANCH=$(git branch --show-current 2>/dev/null || echo "N/A")
    COMMITS=$(git log --oneline 2>/dev/null | wc -l | tr -d ' ')
    LAST_COMMIT=$(git log --oneline -1 2>/dev/null || echo "无提交")
    echo "  Git 分支:     ${BRANCH}"
    echo "  提交数:       ${COMMITS}"
    echo "  最新提交:     ${LAST_COMMIT}"
else
    echo "  Git:          未初始化"
fi

echo ""

# 功能状态
if [ -f "feature_list.json" ]; then
    python3 -c "
import json
with open('feature_list.json') as f:
    data = json.load(f)
features = data.get('features', [])
total = len(features)
passed = sum(1 for f in features if f.get('passes', False))
pct = (passed / total * 100) if total > 0 else 0

print(f'  功能总数:     {total}')
print(f'  已完成:       {passed} ({pct:.1f}%)')
print(f'  待完成:       {total - passed}')
print()
if total > 0:
    print('  待完成功能:')
    for f in features:
        if not f.get('passes', False):
            print(f'    [{f[\"id\"]}] [{f.get(\"category\", \"\")}] {f[\"description\"][:70]}')
"
else
    echo "  feature_list.json 不存在"
fi

echo ""

# 进度文件
if [ -f "claude-progress.txt" ]; then
    PROGRESS_SIZE=$(wc -l < claude-progress.txt | tr -d ' ')
    echo "  进度日志:     ${PROGRESS_SIZE} 行"
else
    echo "  claude-progress.txt 不存在"
fi

echo ""

# 开发服务器
if curl -sf http://localhost:8000/health > /dev/null 2>&1; then
    echo "  开发服务器:   运行中 (port 8000)"
elif curl -sf http://localhost:3000 > /dev/null 2>&1; then
    echo "  开发服务器:   运行中 (port 3000)"
else
    echo "  开发服务器:   未运行"
fi

echo ""
echo "════════════════════════════════════════════════════════════"
