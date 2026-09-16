#!/bin/bash
# session-archive.sh — Stop hook
# 功能：会话结束时生成归档摘要
# 位置：.claude/hooks/session-archive.sh
#
# 输出内容：
#   1. 本次会话新增的 git 提交（基于 session 开始时记录的 HEAD）
#   2. 当前未提交的变更
#   3. Vikunja 任务状态
#   4. 文档同步检查

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$PROJECT_ROOT"

echo "=== 会话归档摘要 ==="
echo ""

# --- Git 提交统计 ---
if git rev-parse --is-inside-work-tree &>/dev/null; then
  BRANCH=$(git branch --show-current 2>/dev/null || echo "detached")
  HEAD=$(git rev-parse --short HEAD 2>/dev/null || echo "none")

  echo "📊 Git 状态 ($BRANCH @ $HEAD)"

  # 今日提交
  TODAY_COMMITS=$(git log --oneline --since="midnight" 2>/dev/null | wc -l | tr -d ' ')
  if [ "$TODAY_COMMITS" -gt 0 ]; then
    echo "   今日提交: $TODAY_COMMITS"
    git log --oneline --since="midnight" 2>/dev/null | sed 's/^/   /'
  else
    echo "   今日提交: 0"
  fi

  # 未提交变更
  DIRTY=$(git status --porcelain 2>/dev/null | wc -l | tr -d ' ')
  if [ "$DIRTY" -gt 0 ]; then
    echo ""
    echo "   ⚠️ ${DIRTY} 个文件未提交："
    git status --short 2>/dev/null | head -10 | sed 's/^/   /'
    if [ "$DIRTY" -gt 10 ]; then
      echo "   ... 还有 $((DIRTY - 10)) 个"
    fi
    echo ""
    echo "   💡 请决定是否提交这些变更"
  fi
fi

echo ""

# --- Vikunja 任务状态 ---
if [ -f ".veans.yml" ] && command -v veans &>/dev/null; then
  echo "📋 Vikunja 任务状态"
  TASKS=$(veans list 2>/dev/null || echo "[]")
  echo "$TASKS" | node -e "
    let d='';
    process.stdin.on('data', c => d += c);
    process.stdin.on('end', () => {
      const tasks = JSON.parse(d);
      const active = tasks.filter(t => !t.done);
      console.log('   活跃任务: ' + active.length);
      for (const t of tasks) {
        const bucket = (t.buckets && t.buckets[0] && t.buckets[0].title) || '?';
        const icon = t.done ? '✅' : '🔧';
        console.log('   ' + icon + ' #' + (t.index||t.id||'?') + ' ' + t.title + ' [' + bucket + ']');
      }
    });
  " 2>/dev/null || true
fi

echo ""

# --- 文档同步检查（复用 doc-sync-check 逻辑）---
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
if [ -f "$SCRIPT_DIR/doc-sync-check.sh" ]; then
  bash "$SCRIPT_DIR/doc-sync-check.sh" 2>/dev/null || true
fi

echo "===================="
