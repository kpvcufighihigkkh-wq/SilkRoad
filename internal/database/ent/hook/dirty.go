package hook

import (
	"context"

	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/bobbin"
	"github.com/yourusername/igh-silkroad/internal/database/ent/doffing"
	"github.com/yourusername/igh-silkroad/internal/database/ent/lot"
)

// syncOnlyFields 是「不算业务写入」的字段集合。
//
// 用途：判断一次写入是否只动了同步状态自身。同步器（markSynced /
// markRetried）的写入只涉及这些字段，必须被排除，否则它把记录标成
// synced 之后会立刻被本 hook 拦回 pending，形成无限重传。
//
// updated_at 必须在内，且它才是关键的一项：生成代码在每次 Update 的
// defaults() 里自动写入 updated_at（见 lot_update.go 的 Save → defaults
// → withHooks 顺序），所以哪怕调用方只 SetSyncStatus，hook 观察到的
// 字段也是 [sync_status sync_retry_count updated_at]。漏掉它，
// 「只改同步字段」的判定永远不成立，上面说的无限重传立刻会发生。
//
// 代价：手工只改 updated_at 的写入不会被重新置脏。本仓库没有这样的写入点
// —— 唯一的写入者是同步器自身，而它必然同时改 sync_status 或 retry_count。
var syncOnlyFields = map[string]struct{}{
	"sync_status":      {},
	"synced_at":        {},
	"sync_retry_count": {},
	"updated_at":       {},
}

// NewDirtyMarker 返回一个把业务写入重新标记为待同步的 ent Hook。
//
// 挂在 Edge 侧的 ent.Client 上（client.Use(...)）。挂在 Center 侧没有意义：
// Center 不采集任何数据。
//
// 只处理采集器认得的表（lots / bobbins / doffings）。对采集器不认的表置
// pending 会造出永远排不出去的积压：retry_count 恒为 0，markExhausted
// 也碰不到它们。新增实体时需同步扩展本函数与 uploader 的 collectors()。
func NewDirtyMarker() ent.Hook {
	return func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
			switch m.Op() {
			case ent.OpUpdate, ent.OpUpdateOne, ent.OpDelete, ent.OpDeleteOne:
				if shouldMarkDirty(m) {
					markDirty(m)
				}
			}
			return next.Mutate(ctx, m)
		})
	}
}

// shouldMarkDirty 判断本次写入是否应当置脏。
//
// 若改动的字段全部属于同步元数据，则不置脏（那是同步器自己的写入）。
// 字段为空同样不置脏：删改类 mutation（OpDelete/OpDeleteOne）只带谓词，
// 不带字段，本就不该被置脏。
func shouldMarkDirty(m ent.Mutation) bool {
	fields := m.Fields()
	if len(fields) == 0 {
		return false
	}
	for _, f := range fields {
		if _, ok := syncOnlyFields[f]; !ok {
			return true
		}
	}
	return false
}

// markDirty 把记录重置为待同步并归还完整重试预算。
//
// 计数必须归零：采集器的过滤条件是 sync_status = pending 且
// sync_retry_count < 上限，而 markExhausted 会把「pending 且计数已达上限」
// 的行打回 failed。只置 pending 而不重置计数，会让一条计数已达上限的
// 记录在编辑后立刻被打回 failed —— 编辑再次丢失。
//
// 类型 switch 覆盖三个可采集实体。枚举常量在各自生成包中，
// 不在 ent 根包（ent.BobbinSyncStatusPending 不存在）。
func markDirty(m ent.Mutation) {
	switch mm := m.(type) {
	case *ent.LotMutation:
		mm.SetSyncStatus(lot.SyncStatusPending)
		mm.SetSyncRetryCount(0)
		mm.ClearSyncedAt()
	case *ent.BobbinMutation:
		mm.SetSyncStatus(bobbin.SyncStatusPending)
		mm.SetSyncRetryCount(0)
		mm.ClearSyncedAt()
	case *ent.DoffingMutation:
		mm.SetSyncStatus(doffing.SyncStatusPending)
		mm.SetSyncRetryCount(0)
		mm.ClearSyncedAt()
	}
}
