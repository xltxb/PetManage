#!/usr/bin/env bash
# =============================================================================
# AI Harness — runner.sh
# 持续运行 Coding Agent 的主循环脚本
#
# 用法:
#   ./harness/runner.sh                    # 交互模式，手动确认每个会话
#   ./harness/runner.sh --auto             # 自动模式，连续运行直到所有功能完成
#   ./harness/runner.sh --max-sessions 5   # 限制最大会话数
# =============================================================================
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_ROOT"

# -----------------------------------------------------------------------------
# 配置
# -----------------------------------------------------------------------------
MAX_SESSIONS=0          # 0 = 无限制
AUTO_MODE=false
SESSION_DELAY=2         # 会话间隔秒数
FEATURE_LIST="feature_list.json"
PROGRESS_FILE="claude-progress.txt"
CODING_PROMPT="harness/prompts/coding-agent.txt"

# -----------------------------------------------------------------------------
# 颜色输出
# -----------------------------------------------------------------------------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

log_info()  { echo -e "${BLUE}[INFO]${NC}  $(date '+%H:%M:%S') $*"; }
log_ok()    { echo -e "${GREEN}[OK]${NC}    $(date '+%H:%M:%S') $*"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC}  $(date '+%H:%M:%S') $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $(date '+%H:%M:%S') $*"; }
banner()    { echo -e "${CYAN}$*${NC}"; }

# -----------------------------------------------------------------------------
# 解析参数
# -----------------------------------------------------------------------------
while [[ $# -gt 0 ]]; do
    case "$1" in
        --auto)          AUTO_MODE=true; shift ;;
        --max-sessions)  MAX_SESSIONS="$2"; shift 2 ;;
        *) echo "未知参数: $1"; exit 1 ;;
    esac
done

# -----------------------------------------------------------------------------
# 检查基础设施
# -----------------------------------------------------------------------------
check_prerequisites() {
    if ! command -v claude &> /dev/null; then
        log_error "claude CLI 未安装"
        exit 1
    fi

    if [ ! -f "$FEATURE_LIST" ]; then
        log_error "$FEATURE_LIST 不存在。请先运行 Session Zero 初始化。"
        log_info "运行: claude -p \"\$(cat harness/prompts/initializer.txt)\""
        exit 1
    fi

    if [ ! -f "$CODING_PROMPT" ]; then
        log_error "$CODING_PROMPT 不存在"
        exit 1
    fi

    if ! git rev-parse --git-dir > /dev/null 2>&1; then
        log_error "不是 git 仓库。请先运行 Session Zero 初始化。"
        exit 1
    fi
}

# -----------------------------------------------------------------------------
# 统计功能完成情况
# -----------------------------------------------------------------------------
count_features() {
    local total pending passed
    total=$(python3 -c "
import json
with open('$FEATURE_LIST') as f:
    data = json.load(f)
features = data.get('features', data) if isinstance(data, dict) else data
print(len(features))
" 2>/dev/null || echo "0")

    pending=$(python3 -c "
import json
with open('$FEATURE_LIST') as f:
    data = json.load(f)
features = data.get('features', data) if isinstance(data, dict) else data
print(sum(1 for f in features if not f.get('passes', False)))
" 2>/dev/null || echo "0")

    passed=$((total - pending))
    echo "$total $pending $passed"
}

# -----------------------------------------------------------------------------
# 获取下一个待实现的功能
# -----------------------------------------------------------------------------
get_next_feature() {
    python3 -c "
import json
with open('$FEATURE_LIST') as f:
    data = json.load(f)
features = data.get('features', data) if isinstance(data, dict) else data
pending = [f for f in features if not f.get('passes', False)]
if pending:
    pending.sort(key=lambda f: f.get('priority', 999))
    f = pending[0]
    print(f'{f[\"id\"]}: {f[\"description\"]}')
" 2>/dev/null || echo ""
}

# -----------------------------------------------------------------------------
# 检查是否有未提交的改动
# -----------------------------------------------------------------------------
has_uncommitted_changes() {
    ! git diff --quiet || ! git diff --cached --quiet
}

# -----------------------------------------------------------------------------
# 打印状态仪表盘
# -----------------------------------------------------------------------------
print_dashboard() {
    local counts next_feature
    counts=$(count_features)
    local total pending passed
    read -r total pending passed <<< "$counts"
    next_feature=$(get_next_feature)

    echo ""
    banner "╔══════════════════════════════════════════════════════════════╗"
    banner "║              AI DEVELOPMENT HARNESS — 状态面板               ║"
    banner "╠══════════════════════════════════════════════════════════════╣"
    banner "║  总功能数: ${total}  |  已完成: ${passed}  |  待完成: ${pending}                   ║"
    banner "╠══════════════════════════════════════════════════════════════╣"
    banner "║  下一个: ${next_feature:0:58}║"
    banner "╚══════════════════════════════════════════════════════════════╝"
    echo ""
}

# -----------------------------------------------------------------------------
# 运行一个 Coding Session
# -----------------------------------------------------------------------------
run_session() {
    local session_num="$1"
    local next_feature
    next_feature=$(get_next_feature)

    if [ -z "$next_feature" ]; then
        log_ok "所有功能已完成！"
        return 1
    fi

    log_info "启动 Session #${session_num}"
    log_info "目标功能: ${next_feature}"

    local coding_prompt_text
    coding_prompt_text=$(cat "$CODING_PROMPT")

    local session_prompt="${coding_prompt_text}

================================================================================
当前会话上下文 (Session #${session_num})
================================================================================

当前时间: $(date '+%Y-%m-%d %H:%M:%S')

你需要实现的下一个功能:
  ${next_feature}

请严格按照 coding agent 的流程:
1. 执行启动仪式（读取进度文件、git log、启动服务器）
2. 实现上述功能
3. 端到端测试验证
4. 如果通过，更新 feature_list.json (passes → true)，git commit，更新 progress.txt
5. 如果未通过，记录原因但不要标记为完成
6. 完成后输出 EXIT_SIGNAL: DONE 或 EXIT_SIGNAL: BLOCKED

重要: 只做这一个功能，不要开始下一个。"

    log_info "正在调用 Claude Code..."

    local exit_code=0
    claude -p "$session_prompt" \
        --permission-mode bypassPermissions \
        --allowedTools "Bash,Read,Write,Edit,WebFetch,WebSearch" \
        2>&1 || exit_code=$?

    echo ""
    if [ $exit_code -eq 0 ]; then
        log_ok "Session #${session_num} 成功完成"
    else
        log_warn "Session #${session_num} 以退出码 ${exit_code} 结束"
    fi

    local new_completed
    new_completed=$(count_features | awk '{print $3}')
    local latest_commit
    latest_commit=$(git log --oneline -1 2>/dev/null || echo "无提交")

    log_info "最新提交: ${latest_commit}"
    log_info "已完成功能数: ${new_completed}"

    return 0
}

# -----------------------------------------------------------------------------
# 启动开发服务器
# -----------------------------------------------------------------------------
start_server() {
    if [ -f "init.sh" ]; then
        log_info "启动开发服务器..."
        bash init.sh &
        sleep 3
    fi
}

# -----------------------------------------------------------------------------
# 主循环
# -----------------------------------------------------------------------------
main() {
    banner ""
    banner "╔══════════════════════════════════════════════════════════════╗"
    banner "║     AI DEVELOPMENT HARNESS — 持续运行系统                    ║"
    banner "║     Based on: Effective Harnesses for Long-Running Agents    ║"
    banner "╚══════════════════════════════════════════════════════════════╝"
    banner ""

    check_prerequisites

    local session_num=1
    local total_sessions=0

    while true; do
        print_dashboard

        local next_feature
        next_feature=$(get_next_feature)

        if [ -z "$next_feature" ]; then
            log_ok "所有功能完成！Harness 退出。"
            log_ok "最终统计: 共 ${total_sessions} 个会话"
            exit 0
        fi

        if [ "$MAX_SESSIONS" -gt 0 ] && [ "$total_sessions" -ge "$MAX_SESSIONS" ]; then
            log_warn "达到最大会话数限制 (${MAX_SESSIONS})"
            exit 0
        fi

        # 非自动模式下需要确认
        if [ "$AUTO_MODE" = false ]; then
            echo -n "按 Enter 开始下一个会话 (Ctrl+C 退出)..."
            read -r
        fi

        if has_uncommitted_changes; then
            log_warn "检测到未提交的改动，请先处理"
            git status --short
            if [ "$AUTO_MODE" = false ]; then
                echo -n "继续? [y/N] "
                read -r ans
                [ "$ans" != "y" ] && [ "$ans" != "Y" ] && exit 0
            fi
        fi

        if ! run_session "$session_num"; then
            log_ok "Harness 完成"
            exit 0
        fi

        total_sessions=$((total_sessions + 1))
        session_num=$((session_num + 1))

        log_info "等待 ${SESSION_DELAY} 秒后继续..."
        sleep "$SESSION_DELAY"
    done
}

main "$@"
