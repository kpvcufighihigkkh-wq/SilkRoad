#!/bin/bash
# session-archive.sh — Stop hook
# 功能：会话结束时简洁摘要（仅在有重要信息时输出）
# 位置：.claude/hooks/session-archive.sh
#
# 输出策略：
#   1. 只在有未提交变更时提醒
#   2. 只在有代码变更但文档未更新时提醒文档同步
#   3. 避免每次都输出冗长信息

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$PROJECT_ROOT"

# 检查是否在 git 仓库中
if ! git rev-parse --is-inside-work-tree &>/dev/null; then
  exit 0
fi

# 获取未提交变更数量
DIRTY=$(git status --porcelain 2>/dev/null | wc -l | tr -d ' ')

# 只在有未提交变更时才输出
if [ "$DIRTY" -gt 0 ]; then
  echo ""
  echo "⚠️ 会话结束提醒：${DIRTY} 个文件未提交"
  git status --short 2>/dev/null | head -5 | sed 's/^/   /'
  if [ "$DIRTY" -gt 5 ]; then
    echo "   ... 还有 $((DIRTY - 5)) 个文件"
  fi
  echo ""

  # --- 文档同步检查（仅在有代码变更时）---
  SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
  if [ -f "$SCRIPT_DIR/doc-sync-check.sh" ]; then
    bash "$SCRIPT_DIR/doc-sync-check.sh" 2>/dev/null || true
  fi
fi
