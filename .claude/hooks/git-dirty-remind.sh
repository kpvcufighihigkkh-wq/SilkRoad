#!/bin/bash
# git-dirty-remind.sh — PreToolUse hook (Write/Edit)
# 功能：当未提交变更超过阈值时提醒 commit
# 位置：.claude/hooks/git-dirty-remind.sh
# 阈值：5 个文件（低于阈值不输出，避免噪音）

set -euo pipefail

THRESHOLD=5

PROJECT_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$PROJECT_ROOT"

if ! git rev-parse --is-inside-work-tree &>/dev/null; then
  exit 0
fi

DIRTY_COUNT=$(git status --porcelain 2>/dev/null | wc -l | tr -d ' ')

if [ "$DIRTY_COUNT" -ge "$THRESHOLD" ]; then
  echo "💡 已有 ${DIRTY_COUNT} 个文件变更未提交（阈值: ${THRESHOLD}）"
  echo "   建议先 commit 保存检查点，确保可随时回退"
  echo "   git add -A && git commit -m \"checkpoint: <描述>\""
fi
