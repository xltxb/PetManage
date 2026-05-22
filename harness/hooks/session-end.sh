#!/usr/bin/env bash
# =============================================================================
# AI Harness — session-end.sh
# 每次 Coding Session 结束时的清理和检查 Hook
#
# 配置: 在 .claude/settings.json 中添加 Stop 钩子:
#   "hooks": {
#     "Stop": [
#       { "command": "bash harness/hooks/session-end.sh" }
#     ]
#   }
# =============================================================================
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$PROJECT_ROOT"

echo "[Harness Hook] 会话结束检查..."

# 1. 检查未提交的改动
if ! git diff --quiet 2>/dev/null || ! git diff --cached --quiet 2>/dev/null; then
    echo "  ⚠ 警告: 存在未提交的改动!"
    echo ""
    echo "  未暂存的文件:"
    git diff --name-only 2>/dev/null || true
    echo ""
    echo "  已暂存但未提交的文件:"
    git diff --cached --name-only 2>/dev/null || true
    echo ""
    echo "  请提交你的改动: git add -A && git commit -m \"...\""
else
    echo "  ✓ 工作区干净，所有改动已提交"
fi

# 2. 检查 feature_list.json 是否存在
if [ -f "feature_list.json" ]; then
    FEATURE_COUNT=$(python3 -c "
import json
with open('feature_list.json') as f:
    data = json.load(f)
features = data.get('features', [])
total = len(features)
passed = sum(1 for f in features if f.get('passes', False))
print(f'{total},{passed}')
" 2>/dev/null || echo "0,0")
    TOTAL=$(echo "$FEATURE_COUNT" | cut -d, -f1)
    PASSED=$(echo "$FEATURE_COUNT" | cut -d, -f2)
    echo "  ✓ 功能进度: ${PASSED}/${TOTAL} 完成"
else
    echo "  ⚠ 警告: feature_list.json 不存在"
fi

# 3. 检查 claude-progress.txt 最后更新时间
if [ -f "claude-progress.txt" ]; then
    LAST_MODIFIED=$(stat -f "%Sm" claude-progress.txt 2>/dev/null || stat -c "%y" claude-progress.txt 2>/dev/null || echo "unknown")
    echo "  ✓ progress.txt 最后更新: ${LAST_MODIFIED}"
else
    echo "  ⚠ 警告: claude-progress.txt 不存在"
fi

# 4. 检查开发服务器状态
if curl -sf http://localhost:8000/health > /dev/null 2>&1; then
    echo "  ✓ 开发服务器运行中 (port 8000)"
else
    echo "  ⚠ 开发服务器未响应 (port 8000)"
fi

echo "[Harness Hook] 检查完成"
