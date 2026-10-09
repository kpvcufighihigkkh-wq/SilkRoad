package edge

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/yourusername/igh-silkroad/internal/database/ent/bobbin"
	"github.com/yourusername/igh-silkroad/internal/database/ent/bobbingrade"
	"github.com/yourusername/igh-silkroad/internal/database/ent/doffing"
	"github.com/yourusername/igh-silkroad/internal/database/ent/lot"
)

// ResyncFailed 把指定表中重试超限的记录重置为待同步。
//
// 用途：网络长期故障后记录被标记 failed —— 在此之前仓库没有任何回收路径，
// failed 即等于数据终点。修好环境后由运维显式触发，归还完整重试预算。
//
// 「重试超限」有两种形态，本函数两者都收：
//   - status = failed：已耗尽预算并被 markExhausted 翻牌的行；
//   - status = pending 且 retry_count >= 上限：计数已满却尚未被翻牌的行。
//     采集器的 `retry_count < 上限` 过滤把它们挡在门外，若只匹配 failed，
//     它们会被本端点漏掉而永久卡在 pending。
//
// 重置同时清空 synced_at 并把 sync_retry_count 归零：只改状态而不重置计数，
// 计数已达上限的记录仍被采集器的过滤挡在门外，而 markExhausted 又会立刻
// 把它们打回 failed。
//
// table 为空表示全部可采集表；未知表名返回错误（而非静默 no-op）。
func (u *Uploader) ResyncFailed(ctx context.Context, table string) (int, error) {
	return u.resetByTable(ctx, table, resetOptions{
		status:                       "failed",
		includeRetryExhaustedPending: true,
	})
}

// ResyncAll 把指定表中全部 synced 记录重置为待同步（全量重传）。
//
// 用途：载荷新增字段后，存量已 synced 的记录不会自动重传，其新字段在 Center
// 永远为空；各实体虽有 synced_at，却没有与 updated_at 的比较机制，无法自动
// 识别陈旧记录，故只能显式全量重传。Center 侧 upsert 或「同 ID 视为已处理」，
// 重传不产生重复行。
//
// 只匹配 synced：本模式是「重传已同步的存量」，不承担回收超限记录的职责
// （那是 ResyncFailed 的契约）。
//
// table 为空表示全部可采集表；未知表名返回错误。
func (u *Uploader) ResyncAll(ctx context.Context, table string) (int, error) {
	return u.resetByTable(ctx, table, resetOptions{status: "synced"})
}

// ErrUnknownTable 表示请求的表名不在可采集集合内。
//
// 单独成哨兵值而非裸 fmt.Errorf，是为让 HTTP 层能用 errors.Is 区分
// 「调用方给错表名」与「数据库出错」：前者是客户端错误（400），
// 后者才是服务端错误（500）。一律返 500 会污染错误率看板，
// 让运维误以为服务坏了。
var ErrUnknownTable = errors.New("unknown table")

// resetOptions 重置操作的匹配条件。
type resetOptions struct {
	// status 要重置的同步状态。
	status string

	// includeRetryExhaustedPending 为真时，额外匹配
	// 「pending 且 retry_count >= 上限」的行。
	//
	// 这类行不在 status 的取值里，却同样属于「重试超限」：采集器按
	// `retry_count < 上限` 过滤，它们被挡在门外，既不前进也不会被
	// markExhausted 翻牌（见 Upload 中该调用的说明）。
	includeRetryExhaustedPending bool
}

// orRetryExhausted 在需要时把「pending 且计数超限」并入基础谓词。
//
// 四个谓词构造器由调用方注入：生成的谓词类型按实体各不相同
// （predicate.Lot / predicate.Bobbin …），无法共用一个非泛型函数。
func orRetryExhausted[P any](
	base, pending, retryGTE P,
	or func(...P) P,
	and func(...P) P,
	enabled bool,
) P {
	if !enabled {
		return base
	}
	return or(base, and(pending, retryGTE))
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
	extra := opts.includeRetryExhaustedPending

	switch table {
	case "lots":
		return u.client.Lot.Update().
			Where(orRetryExhausted(
				lot.SyncStatusEQ(lot.SyncStatus(opts.status)),
				lot.SyncStatusEQ(lot.SyncStatusPending),
				lot.SyncRetryCountGTE(maxRetryCount),
				lot.Or, lot.And, extra)).
			SetSyncStatus(lot.SyncStatusPending).
			SetSyncRetryCount(0).
			ClearSyncedAt().
			Save(ctx)
	case "doffings":
		return u.client.Doffing.Update().
			Where(orRetryExhausted(
				doffing.SyncStatusEQ(doffing.SyncStatus(opts.status)),
				doffing.SyncStatusEQ(doffing.SyncStatusPending),
				doffing.SyncRetryCountGTE(maxRetryCount),
				doffing.Or, doffing.And, extra)).
			SetSyncStatus(doffing.SyncStatusPending).
			SetSyncRetryCount(0).
			ClearSyncedAt().
			Save(ctx)
	case "bobbins":
		return u.client.Bobbin.Update().
			Where(orRetryExhausted(
				bobbin.SyncStatusEQ(bobbin.SyncStatus(opts.status)),
				bobbin.SyncStatusEQ(bobbin.SyncStatusPending),
				bobbin.SyncRetryCountGTE(maxRetryCount),
				bobbin.Or, bobbin.And, extra)).
			SetSyncStatus(bobbin.SyncStatusPending).
			SetSyncRetryCount(0).
			ClearSyncedAt().
			Save(ctx)
	case "bobbin_grades":
		return u.client.BobbinGrade.Update().
			Where(orRetryExhausted(
				bobbingrade.SyncStatusEQ(bobbingrade.SyncStatus(opts.status)),
				bobbingrade.SyncStatusEQ(bobbingrade.SyncStatusPending),
				bobbingrade.SyncRetryCountGTE(maxRetryCount),
				bobbingrade.Or, bobbingrade.And, extra)).
			SetSyncStatus(bobbingrade.SyncStatusPending).
			SetSyncRetryCount(0).
			ClearSyncedAt().
			Save(ctx)
	}

	return 0, fmt.Errorf("%w %q（可重置的只有 collectors() 中的可采集表：%s）",
		ErrUnknownTable, table, strings.Join(collectableTables(u), ", "))
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
