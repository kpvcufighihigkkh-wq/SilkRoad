#!/bin/bash
# git-session-check.sh — SessionStart hook
# 功能：报告 git 工作区状态，提醒未提交变更
# 位置：.claude/hooks/git-session-check.sh

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$PROJECT_ROOT"

# 检查是否是 git 仓库
if ! git rev-parse --is-inside-work-tree &>/dev/null; then
  exit 0
fi

echo "=== Git 工作区状态 ==="

BRANCH=$(git branch --show-current 2>/dev/null || echo "detached")
HEAD=$(git rev-parse --short HEAD 2>/dev/null || echo "none")
echo "分支: $BRANCH @ $HEAD"

# 统计未提交变更
DIRTY_COUNT=$(git status --porcelain 2>/dev/null | wc -l | tr -d ' ')

if [ "$DIRTY_COUNT" -gt 0 ]; then
  echo "⚠️ ${DIRTY_COUNT} 个文件有未提交变更："
  git status --short 2>/dev/null | head -15
  if [ "$DIRTY_COUNT" -gt 15 ]; then
    echo "   ... 还有 $((DIRTY_COUNT - 15)) 个文件"
  fi
  echo ""
  echo "💡 建议：先提交或 stash 这些变更，确保可回退"
else
  echo "✅ 工作区干净"
fi

echo ""
echo "最近 5 次提交："
git log --oneline -5 2>/dev/null || echo "  (无提交记录)"
