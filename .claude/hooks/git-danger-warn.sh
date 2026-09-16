#!/bin/bash
# git-danger-warn.sh — PreToolUse hook (Bash: git 危险操作)
# 功能：在破坏性 git 命令执行前输出安全锚点
# 位置：.claude/hooks/git-danger-warn.sh

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$PROJECT_ROOT"

if ! git rev-parse --is-inside-work-tree &>/dev/null; then
  exit 0
fi

HEAD=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BRANCH=$(git branch --show-current 2>/dev/null || echo "detached")
DIRTY=$(git status --porcelain 2>/dev/null | wc -l | tr -d ' ')
STASH_COUNT=$(git stash list 2>/dev/null | wc -l | tr -d ' ')

echo "⚠️ 危险 git 操作检测"
echo "━━━━━━━━━━━━━━━━━━━━"
echo "当前 HEAD: $HEAD ($BRANCH)"
echo "未提交变更: $DIRTY 个文件"
echo "Stash 数量: $STASH_COUNT"
echo ""
echo "安全回退参考："
echo "  git reflog                     # 查看所有操作历史"
echo "  git reset --hard $HEAD   # 回到当前状态"
echo "  git stash                      # 临时保存变更"
echo "━━━━━━━━━━━━━━━━━━━━"
