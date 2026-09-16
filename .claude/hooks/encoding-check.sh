#!/bin/bash
# encoding-check.sh — PreToolUse hook (git commit) + git pre-commit hook
# 功能：提交前校验暂存区文件的编码格式
# 位置：.claude/hooks/encoding-check.sh
#
# 检查项：
#   1. 文本文件必须是 UTF-8 编码（允许 ASCII，ASCII 是 UTF-8 子集）
#   2. 不允许 UTF-8 BOM
#   3. 不允许 CRLF 行尾进入仓库
#   4. 中文内容完整性检查（检测损坏的 UTF-8 序列）

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$PROJECT_ROOT"

if ! git rev-parse --is-inside-work-tree &>/dev/null; then
  exit 0
fi

ERRORS=0
WARNINGS=0

# 获取暂存区中的文本文件（排除二进制）
STAGED_FILES=$(git diff --cached --name-only --diff-filter=ACMR 2>/dev/null || git diff --name-only 2>/dev/null || echo "")

if [ -z "$STAGED_FILES" ]; then
  # 没有暂存文件时，检查所有已修改文件
  STAGED_FILES=$(git diff --name-only 2>/dev/null || echo "")
fi

if [ -z "$STAGED_FILES" ]; then
  exit 0
fi

echo "=== 编码格式检查 ==="

while IFS= read -r filepath; do
  [ -z "$filepath" ] && continue
  [ ! -f "$filepath" ] && continue

  # 跳过二进制文件
  if file -bi "$filepath" 2>/dev/null | grep -q "binary"; then
    continue
  fi

  MIME=$(file -bi "$filepath" 2>/dev/null || echo "unknown")
  CHARSET=$(echo "$MIME" | sed -n 's/.*charset=\([^ ;]*\).*/\1/p')

  # 如果 sed 没提取到，尝试简单的 awk
  if [ -z "$CHARSET" ]; then
    CHARSET=$(echo "$MIME" | awk -F'charset=' '{print $2}' | awk '{print $1}')
  fi

  # 默认值
  if [ -z "$CHARSET" ]; then
    CHARSET="unknown"
  fi

  # 检查 1: 编码必须是 UTF-8 或 ASCII
  if [[ "$CHARSET" != "utf-8" && "$CHARSET" != "us-ascii" && "$CHARSET" != "ascii" ]]; then
    echo "❌ $filepath: 编码 $CHARSET (需要 UTF-8)"
    ERRORS=$((ERRORS + 1))
  fi

  # 检查 2: UTF-8 BOM
  if head -c 3 "$filepath" 2>/dev/null | od -An -tx1 2>/dev/null | head -1 | grep -q "ef bb bf"; then
    echo "❌ $filepath: 包含 UTF-8 BOM (需要移除)"
    ERRORS=$((ERRORS + 1))
  fi

  # 检查 3: CRLF 行尾
  if grep -q $'\r' "$filepath" 2>/dev/null; then
    echo "⚠️ $filepath: 包含 CRLF 行尾 (应为 LF)"
    WARNINGS=$((WARNINGS + 1))
  fi

  # 检查 4: 损坏的 UTF-8 序列（使用 file 命令，iconv 在 Git Bash 中不可用）
  # 如果 file 报告编码是 utf-8 但实际内容有问题，file 会标记为 unknown
  FILE_RESULT=$(file -b "$filepath" 2>/dev/null)
  if echo "$FILE_RESULT" | grep -qi "with very long lines\|cannot"; then
    echo "⚠️ $filepath: 文件格式异常"
    WARNINGS=$((WARNINGS + 1))
  fi

  # 额外：检查是否有 UTF-8 替换字符 (U+FFFD 表示损坏)
  if grep -q $'\xef\xbf\xbd' "$filepath" 2>/dev/null; then
    echo "❌ $filepath: 包含 UTF-8 替换字符 U+FFFD (表示原始字节损坏)"
    ERRORS=$((ERRORS + 1))
  fi

done <<< "$STAGED_FILES"

# 汇总
if [ "$ERRORS" -gt 0 ]; then
  echo ""
  echo "🚫 发现 $ERRORS 个编码错误，$WARNINGS 个警告"
  echo "   请修复后再提交"
  exit 1
elif [ "$WARNINGS" -gt 0 ]; then
  echo "⚠️ $WARNINGS 个警告（CRLF），.gitattributes 会在下次 checkout 时自动修正"
else
  echo "✅ 编码检查通过"
fi
