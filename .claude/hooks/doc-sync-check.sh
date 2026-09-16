#!/bin/bash
# doc-sync-check.sh — PostToolUse hook (Write/Edit) + Stop hook
# 功能：检测代码变更后设计文档是否同步更新
# 位置：.claude/hooks/doc-sync-check.sh
#
# 逻辑：
#   1. 扫描 git diff 中的代码文件变更（src/、lib/、app/、packages/ 等）
#   2. 检查 docs/ 目录下是否有对应的设计文档
#   3. 如果代码变了但相关文档没变，输出提醒
#
# 触发场景：
#   - 作为 Stop hook：会话结束前检查整体文档一致性
#   - 也可作为 PreCompact hook：压缩前提醒

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$PROJECT_ROOT"

if ! git rev-parse --is-inside-work-tree &>/dev/null; then
  exit 0
fi

# 代码目录模式（根据项目调整）
CODE_PATTERNS="src/|lib/|app/|packages/|cmd/|internal/|api/"
# 文档目录
DOC_DIR="docs/"
# 项目文档文件
DOC_FILES="WORKFLOW.md|CLAUDE.md|README.md|ARCHITECTURE.md"

# 获取已变更但未提交的文件
CHANGED_FILES=$(git status --porcelain 2>/dev/null | awk '{print $NF}')

if [ -z "$CHANGED_FILES" ]; then
  exit 0
fi

# 统计代码文件变更（grep -c 返回计数，无匹配时退出码 1 但仍输出 0）
CODE_CHANGED=$(echo "$CHANGED_FILES" | { grep -cE "$CODE_PATTERNS" || true; })
# 统计文档文件变更
DOC_CHANGED=$(echo "$CHANGED_FILES" | { grep -cE "(${DOC_DIR}|${DOC_FILES})" || true; })

if [ "$CODE_CHANGED" -gt 0 ] && [ "$DOC_CHANGED" -eq 0 ]; then
  echo "📝 文档同步提醒"
  echo "━━━━━━━━━━━━━━━━"
  echo "检测到 ${CODE_CHANGED} 个代码文件变更，但设计文档未更新"
  echo ""
  echo "变更的代码文件："
  echo "$CHANGED_FILES" | { grep -E "$CODE_PATTERNS" 2>/dev/null || true; } | head -10 | sed 's/^/   /'
  echo ""
  echo "请检查以下文档是否需要同步更新："
  echo "   - docs/ 目录下的设计文档"
  echo "   - WORKFLOW.md（工作流变更）"
  echo "   - CLAUDE.md（规则变更）"
  echo "   - README.md（功能说明）"
  echo "   - 相关 API 文档或接口说明"
  echo ""
  echo "⚠️ 代码与文档不一致会导致后续开发出现混乱"
  echo "━━━━━━━━━━━━━━━━"
fi
