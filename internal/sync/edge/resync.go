package edge

import (
	"context"
	"fmt"
	"strings"

	"github.com/yourusername/igh-silkroad/internal/database/ent/bobbin"
	"github.com/yourusername/igh-silkroad/internal/database/ent/bobbingrade"
	"github.com/yourusername/igh-silkroad/internal/database/ent/doffing"
	"github.com/yourusername/igh-silkroad/internal/database/ent/lot"
)

// ResyncFailed 把指定表中已 failed 的记录重置为待同步。
//
// 用途：网络长期故障后记录被标记 failed —— 在此之前仓库没有任何回收路径，
// failed 即等于数据终点。修好环境后由运维显式触发，归还完整重试预算。
//
// 重置同时清空 synced_at 并把 sync_retry_count 归零：采集器的过滤是
// `sync_status = pending` 且 `sync_retry_count < 上限`，只改状态而不重置计数，
// 计数已达上限的记录仍被挡在门外，而 markExhausted 又会立刻把它们打回 failed。
//
// table 为空表示全部可采集表；未知表名返回错误（而非静默 no-op）。
func (u *Uploader) ResyncFailed(ctx context.Context, table string) (int, error) {
	return u.resetByTable(ctx, table, resetOptions{statuses: []string{"failed"}})
}

// ResyncAll 把指定表中全部 synced 记录重置为待同步（全量重传）。
//
// 用途：载荷新增字段后，存量已 synced 的记录不会自动重传，其新字段在 Center
// 永远为空；各实体虽有 synced_at，却没有与 updated_at 的比较机制，无法自动
// 识别陈旧记录，故只能显式全量重传。Center 侧 upsert 或「同 ID 视为已处理」，
// 重传不产生重复行。
//
// table 为空表示全部可采集表；未知表名返回错误。
func (u *Uploader) ResyncAll(ctx context.Context, table string) (int, error) {
	return u.resetByTable(ctx, table, resetOptions{statuses: []string{"synced"}})
}

// resetOptions 重置操作的参数。
//
// statuses 目前只放单个元素，故 resetOneTable 一律用 SyncStatusEQ 过滤，
// 不引入多状态分支。
type resetOptions struct {
	statuses []string
}

// resetByTable 按表分发重置操作。table 为空表示全部可采集表。
//
// 接受哪些表由 resetOneTable 的 switch 决定，而该 switch 必须与 collectors()
// 同集 —— 由 resync_internal_test.go 双向绑定：
//   - 多一张（如 barrels）：重置它只会造出永远排不出去的积压（采集器不取它、
//     retry_count 不增长、markExhausted 也碰不到它）；
//   - 少一张：该表的 failed 记录仍无回收路径。
//
// 这里不单独维护一份白名单：两份列表会各自漂移，而绑定只存在于测试中。
func (u *Uploader) resetByTable(ctx context.Context, table string, opts resetOptions) (int, error) {
	if table == "" {
		total := 0
		for _, entity := range u.collectors() {
			n, err := u.resetOneTable(ctx, entity.table, opts)
			if err != nil {
				return total, fmt.Errorf("reset %s: %w", entity.table, err)
			}
			total += n
		}
		return total, nil
	}

	n, err := u.resetOneTable(ctx, table, opts)
	if err != nil {
		return 0, err
	}
	return n, nil
}

// resetOneTable 对单表执行重置，返回受影响行数。
// 表名不在可采集集合内时返回错误。
func (u *Uploader) resetOneTable(ctx context.Context, table string, opts resetOptions) (int, error) {
	status := opts.statuses[0]

	switch table {
	case "lots":
		return u.client.Lot.Update().
			Where(lot.SyncStatusEQ(lot.SyncStatus(status))).
			SetSyncStatus(lot.SyncStatusPending).
			SetSyncRetryCount(0).
			ClearSyncedAt().
			Save(ctx)
	case "doffings":
		return u.client.Doffing.Update().
			Where(doffing.SyncStatusEQ(doffing.SyncStatus(status))).
			SetSyncStatus(doffing.SyncStatusPending).
			SetSyncRetryCount(0).
			ClearSyncedAt().
			Save(ctx)
	case "bobbins":
		return u.client.Bobbin.Update().
			Where(bobbin.SyncStatusEQ(bobbin.SyncStatus(status))).
			SetSyncStatus(bobbin.SyncStatusPending).
			SetSyncRetryCount(0).
			ClearSyncedAt().
			Save(ctx)
	case "bobbin_grades":
		return u.client.BobbinGrade.Update().
			Where(bobbingrade.SyncStatusEQ(bobbingrade.SyncStatus(status))).
			SetSyncStatus(bobbingrade.SyncStatusPending).
			SetSyncRetryCount(0).
			ClearSyncedAt().
			Save(ctx)
	}

	return 0, fmt.Errorf("unknown table %q（可重置的只有 collectors() 中的可采集表：%s）",
		table, strings.Join(collectableTables(u), ", "))
}

// collectableTables 返回可采集表名，用于错误信息。
func collectableTables(u *Uploader) []string {
	entities := u.collectors()
	out := make([]string, 0, len(entities))
	for _, e := range entities {
		out = append(out, e.table)
	}
	return out
}
