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
// 用途：判断一次写入是否只动了同步状态自身。markSynced 的写入恰好落在这个
// 集合内（它 SetSyncStatus / SetSyncedAt），必须被排除，否则它把记录标成
// synced 之后会立刻被本 hook 拦回 pending，形成无限重传。
//
// markRetried 走的是另一条路径，不要以为它被这里"处理"了：它用
// AddSyncRetryCount 累加计数，而 Fields() 不报告 additive mutation 的字段
// （AddSyncRetryCount 存在 addsync_retry_count，不是 sync_retry_count）。
// 所以 hook 在那条路径上只观察到 [updated_at]，同样落进本集合、走跳过分支。
// 这正是期望行为：重试计数归同步器管，hook 不该插手归零 —— 否则计数永远
// 涨不上去，就变成了无限重试。
//
// updated_at 必须在内，且它才是关键的一项：生成代码在每次 Update 的
// defaults() 里自动写入 updated_at（见 lot_update.go 的 Save → defaults
// → withHooks 顺序），所以哪怕调用方只 SetSyncStatus，hook 观察到的
// 字段也是 [sync_status sync_retry_count updated_at]。漏掉它，
// 「只改同步字段」的判定永远不成立，上面说的无限重传立刻会发生。
//
// 代价：手工只改 updated_at 的写入不会被重新置脏。Edge 运行时里唯一的
// updated_at 写入者是同步器自身，而它必然同时改动 sync_status 或
// retry_count；但这不是全仓库的保证 —— cmd/test-db/main.go 就是一个不碰
// 同步字段的业务写入者（SetActualQuantity），只是它不参与 Edge 运行时，
// 挂了本 hook 也不会被它触发。新增写入点时不要假设"只有同步器写 updated_at"。
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
// 只处理采集器认识的表。当前 collectors() 只含 lots / bobbins
// （uploader.go:93-99），doffings 的采集函数 collectDoffings 已存在但**未接入**，
// 要等 Center 的 handleCreate 支持 "doffings" 才能加回。
//
// 因此 doffings 的 setter 分支属于"提前就位"：它置的脏现在没有消费者，会停在
// pending 不被上传。这不构成数据丢失 —— 记录仍在本地，且重试计数不会增长，
// 不会触发 markExhausted 把它翻成 failed，等采集接入后即可正常上传。
//
// 反方向的危险才是要防的：**给采集器不认识的表**置 pending 会造出永远排不出去
// 的积压 —— 采集器不取它、retry_count 恒为 0、markExhausted 也碰不到它，
// 记录永久停在 pending 且无人可见。新增实体时必须同步扩展本函数与
// collectors()，两边保持一致。
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
// 类型 switch 覆盖 lots / bobbins / doffings 三张有业务写入的表。枚举常量在
// 各自生成包中，不在 ent 根包（ent.BobbinSyncStatusPending 不存在）。
//
// 三张表的覆盖范围与 collectors() 当前的两张并不重合，这是有意的：doffings
// 的分支先就位（见 NewDirtyMarker 的说明），而 barrel 虽然也在 markableTables
// 里、却被本 switch 排除，因为它现在没有业务写入路径。新增实体时必须同时
// 扩展本函数与 collectors()。
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
