#!/bin/bash
# session-init.sh — SessionStart hook
# 功能：检查 Vikunja 项目任务状态，引导会话进入正确工作流
# 位置：.claude/hooks/session-init.sh

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$PROJECT_ROOT"

# 检查 .veans.yml 是否存在（确认是 veans 管理的项目）
if [ ! -f ".veans.yml" ]; then
  echo "⚠️ 当前项目未配置 .veans.yml，跳过 Vikunja 任务检查"
  exit 0
fi

echo "=== Vikunja 任务状态 ==="

# 获取所有未完成任务并按 bucket 分组展示
TASKS=$(veans list 2>/dev/null || echo "[]")

echo "$TASKS" | node -e "
let d='';
process.stdin.on('data', c => d += c);
process.stdin.on('end', () => {
  const tasks = JSON.parse(d);
  const active = tasks.filter(t => !t.done);

  if (active.length === 0) {
    console.log('📭 项目中没有活跃任务');
    console.log('💡 请先创建任务再开始工作：');
    console.log('   veans create \"<任务标题>\" -s in-progress -d \"<描述>\"');
    console.log('   或从 Plane 同步任务编号：veans create \"[PLN-xxx] 标题\" -s in-progress');
    console.log('');
    console.log('⚠️ 未创建任务前，不应开始编码工作');
  } else {
    const groups = {};
    for (const t of active) {
      const bucket = (t.buckets && t.buckets[0] && t.buckets[0].title) || 'Unknown';
      if (!groups[bucket]) groups[bucket] = [];
      groups[bucket].push(t);
    }

    const icons = { 'Todo': '📝', 'Doing': '🔧', 'In Review': '👀' };
    const counts = Object.entries(groups).map(([k, v]) => k + '=' + v.length).join(' | ');
    console.log('📋 活跃任务统计: ' + counts);

    for (const [bucket, items] of Object.entries(groups)) {
      const icon = icons[bucket] || '📌';
      console.log('');
      console.log(icon + ' ' + bucket + ':');
      for (const t of items) {
        const idx = t.index || t.id || '?';
        console.log('   #' + idx + ' ' + t.title);
      }
    }
  }
});
"
