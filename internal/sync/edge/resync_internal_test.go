package edge

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
)

// 本文件把「重置端点接受的表」与「真正会被采集的表」绑在一起。
//
// collectors() 是可采集表的权威定义 —— 采集器实际遍历的就是它。
// resetByTable 另有一份白名单（resetOneTable 的 switch），两边同源在代码里
// 无处保证，而这条对应关系的两头都会出错：
//   - 白名单多一张（如 barrels）：重置它只会造出**永远排不出去**的积压 ——
//     采集器不取它、retry_count 不增长、markExhausted 也碰不到它；
//   - 白名单少一张：该表的 failed 记录仍无回收路径，正是本端点要堵的洞。
//
// 因此这里双向绑定：每张可采集表都必须能被实际重置；每张 markableTables 里
// 却不在 collectors() 中的表都必须被拒绝。新增表只改 collectors() 而漏改
// resetOneTable（或反之）时，本用例变红。

// TestResync_EveryCollectableTableIsResettable 逐表实证：collectors() 里的
// 每一张表，resetOneTable 都能真的重置掉一行。
//
// 只测 lots 时，删掉 bobbin_grades 的 switch 分支不会有任何测试变红 ——
// 重置端点会走进 default 返回 unknown table，而那条路径在外部包测试里
// 表现为「调用报错」，与「表名非法」不可区分。
func TestResync_EveryCollectableTableIsResettable(t *testing.T) {
	client := newInternalEdgeClient(t)
	ctx := context.Background()
	u := NewUploader(client, "edge-001", "http://center:8080", "test-token")

	// seeders 必须与 collectors() 同集：本 map 只按「怎样造一行 failed 记录」
	// 这一件事区分各表。
	seeders := map[string]func() uuid.UUID{
		"lots": func() uuid.UUID {
			row, err := client.Lot.Create().
				SetLotNumber("LOT-RESYNC-BIND").
				SetProductType("FDY").
				SetPlannedQuantity(1).
				SetSyncStatus("failed").
				SetSyncRetryCount(maxRetryCount).
				Save(ctx)
			if err != nil {
				t.Fatalf("seed lot: %v", err)
			}
			return row.ID
		},
		"doffings": func() uuid.UUID {
			parent, err := client.Lot.Create().
				SetLotNumber("LOT-RESYNC-BIND-DOFFING").
				SetProductType("FDY").
				SetPlannedQuantity(1).
				Save(ctx)
			if err != nil {
				t.Fatalf("seed parent lot: %v", err)
			}
			row, err := client.Doffing.Create().
				SetSpinningLineID(uuid.New()).
				SetSpinningPosition(1).
				SetLotID(parent.ID).
				SetSyncStatus("failed").
				SetSyncRetryCount(maxRetryCount).
				Save(ctx)
			if err != nil {
				t.Fatalf("seed doffing: %v", err)
			}
			return row.ID
		},
		// 桶引用批次：先造父批次，再建 failed 的桶。
		"barrels": func() uuid.UUID {
			parent, err := client.Lot.Create().
				SetLotNumber("LOT-RESYNC-BIND-BARREL").
				SetProductType("FDY").
				SetPlannedQuantity(1).
				Save(ctx)
			if err != nil {
				t.Fatalf("seed parent lot: %v", err)
			}
			row, err := client.Barrel.Create().
				SetBarrelNumber("BRL-RESYNC-BIND").
				SetLotID(parent.ID).
				SetSyncStatus("failed").
				SetSyncRetryCount(maxRetryCount).
				Save(ctx)
			if err != nil {
				t.Fatalf("seed barrel: %v", err)
			}
			return row.ID
		},
		// 纸箱引用批次：先造父批次，再建 failed 的纸箱。
		"cartons": func() uuid.UUID {
			parent, err := client.Lot.Create().
				SetLotNumber("LOT-RESYNC-BIND-CARTON").
				SetProductType("DTY").
				SetPlannedQuantity(1).
				Save(ctx)
			if err != nil {
				t.Fatalf("seed parent lot: %v", err)
			}
			row, err := client.Carton.Create().
				SetCartonNumber("CTN-RESYNC-BIND").
				SetLotID(parent.ID).
				SetSyncStatus("failed").
				SetSyncRetryCount(maxRetryCount).
				Save(ctx)
			if err != nil {
				t.Fatalf("seed carton: %v", err)
			}
			return row.ID
		},
		// 吊车无外键：直接建一行 failed 记录。
		"modules": func() uuid.UUID {
			row, err := client.Module.Create().
				SetModuleNumber("MDL-RESYNC-BIND").
				SetSyncStatus("failed").
				SetSyncRetryCount(maxRetryCount).
				Save(ctx)
			if err != nil {
				t.Fatalf("seed module: %v", err)
			}
			return row.ID
		},
		// 托盘引用批次：先造父批次，再建 failed 的托盘。
		"pallets": func() uuid.UUID {
			parent, err := client.Lot.Create().
				SetLotNumber("LOT-RESYNC-BIND-PALLET").
				SetProductType("FDY").
				SetPlannedQuantity(1).
				Save(ctx)
			if err != nil {
				t.Fatalf("seed parent lot: %v", err)
			}
			row, err := client.Pallet.Create().
				SetPalletCode("PLT-RESYNC-BIND").
				SetLotID(parent.ID).
				SetSyncStatus("failed").
				SetSyncRetryCount(maxRetryCount).
				Save(ctx)
			if err != nil {
				t.Fatalf("seed pallet: %v", err)
			}
			return row.ID
		},
		"bobbins": func() uuid.UUID {
			parent, err := client.Lot.Create().
				SetLotNumber("LOT-RESYNC-BIND-BOBBIN").
				SetProductType("FDY").
				SetPlannedQuantity(1).
				Save(ctx)
			if err != nil {
				t.Fatalf("seed parent lot: %v", err)
			}
			row, err := client.Bobbin.Create().
				SetBobbinNumber("BOBBIN-RESYNC-BIND").
				SetLotID(parent.ID).
				SetSpinningPosition(1).
				SetGrossWeight(1.5).
				SetNetWeight(1.2).
				SetSyncStatus("failed").
				SetSyncRetryCount(maxRetryCount).
				Save(ctx)
			if err != nil {
				t.Fatalf("seed bobbin: %v", err)
			}
			return row.ID
		},
		// 无外键：孤儿质检记录（父丝饼从未同步）正是 Ruling V 的场景。
		"bobbin_grades": func() uuid.UUID {
			row, err := client.BobbinGrade.Create().
				SetBobbinID(uuid.New()).
				SetGrade("A").
				SetSyncStatus("failed").
				SetSyncRetryCount(maxRetryCount).
				Save(ctx)
			if err != nil {
				t.Fatalf("seed bobbin grade: %v", err)
			}
			return row.ID
		},
	}

	collectable := make([]string, 0, 4)
	for _, entity := range u.collectors() {
		collectable = append(collectable, entity.table)
	}

	if len(seeders) != len(collectable) {
		t.Fatalf("seeder 数 = %d, collectors() = %d（两者必须同集）",
			len(seeders), len(collectable))
	}
	for _, table := range collectable {
		if _, ok := seeders[table]; !ok {
			t.Fatalf("collectors() 中的 %q 没有 seeder，本用例无法验证它可被重置", table)
		}
	}
	for table := range seeders {
		found := false
		for _, c := range collectable {
			if c == table {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("seeder %q 不在 collectors() 中，seeders 多了一张表", table)
		}
	}

	opts := resetOptions{status: "failed"}
	for _, table := range collectable {
		id := seeders[table]()

		n, err := u.resetOneTable(ctx, table, opts)
		if err != nil {
			t.Errorf("resetOneTable(%q) 报错：%v —— collectors() 认识它，重置端点却不认识", table, err)
			continue
		}
		if n != 1 {
			t.Errorf("resetOneTable(%q) 影响行数 = %d, want 1", table, n)
		}

		status, retries := reloadSyncState(t, client, ctx, table, id)
		if status != "pending" {
			t.Errorf("resetOneTable(%q) 后 SyncStatus = %q, want %q", table, status, "pending")
		}
		if retries != 0 {
			t.Errorf("resetOneTable(%q) 后 SyncRetryCount = %d, want 0", table, retries)
		}
	}
}

// TestResync_RejectsNonCollectableTables 反向绑定：markableTables 里存在、
// 却不被 collectors() 采集的表（当前是 barrels）必须被重置端点拒绝。
//
// 重置一张从不被采集的表只会造出永远排不出去的积压 —— 采集器不取它、
// retry_count 不增长、markExhausted 也碰不到它，记录永久停在 pending 且
// 无人可见。这正是 uploader.go 中 collectors() 注释立下的原则。
//
// 本用例从 markableTables 与 collectors() 两张活表推导「不该被接受」的集合，
// 不硬编码具体表名：将来若某张表被移出采集，它也会落进这个集合，
// 从而强制「移出采集」与「移出重置白名单」两件事同时发生。
//
// 当前两者完全重合（六张表全部采集），故本用例只剩一个断言：
// 一旦出现「可标记但不可采集」的表，它必须被重置端点拒绝。
// 若哪天有人把某张表移出 collectors() 却忘了移出 resync 白名单，
// checked 会从 0 变成 1 且拒绝检查生效。
func TestResync_RejectsNonCollectableTables(t *testing.T) {
	client := newInternalEdgeClient(t)
	ctx := context.Background()
	u := NewUploader(client, "edge-001", "http://center:8080", "test-token")

	collectable := make(map[string]bool, len(u.collectors()))
	for _, entity := range u.collectors() {
		collectable[entity.table] = true
	}

	opts := resetOptions{status: "failed"}
	for _, table := range markableTables {
		if collectable[table] {
			continue
		}
		if _, err := u.resetByTable(ctx, table, opts); err == nil {
			t.Errorf("%q 不被采集，重置端点却接受了它（会产生永远排不出去的积压）", table)
		}
	}

	// 反向：每张可采集表都必须被重置端点接受，否则该表的 failed 记录
	// 永远没有回收路径。这一半由 TestResync_EveryCollectableTableIsResettable
	// 实证，这里只断言两张表集合的包含关系，防止有人只改一边。
	for _, entity := range u.collectors() {
		found := false
		for _, table := range markableTables {
			if table == entity.table {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("可采集表 %q 不在 markableTables 中：其记录同步后不会被标记",
				entity.table)
		}
	}
}

// reloadSyncState 读回一张表某行的同步状态（表名 → 对应实体的分派）。
func reloadSyncState(t *testing.T, client *ent.Client, ctx context.Context, table string, id uuid.UUID) (string, int) {
	t.Helper()

	switch table {
	case "lots":
		row, err := client.Lot.Get(ctx, id)
		if err != nil {
			t.Fatalf("reload lots: %v", err)
		}
		return string(row.SyncStatus), row.SyncRetryCount
	case "doffings":
		row, err := client.Doffing.Get(ctx, id)
		if err != nil {
			t.Fatalf("reload doffings: %v", err)
		}
		return string(row.SyncStatus), row.SyncRetryCount
	case "barrels":
		row, err := client.Barrel.Get(ctx, id)
		if err != nil {
			t.Fatalf("reload barrels: %v", err)
		}
		return string(row.SyncStatus), row.SyncRetryCount
	case "cartons":
		row, err := client.Carton.Get(ctx, id)
		if err != nil {
			t.Fatalf("reload cartons: %v", err)
		}
		return string(row.SyncStatus), row.SyncRetryCount
	case "modules":
		row, err := client.Module.Get(ctx, id)
		if err != nil {
			t.Fatalf("reload modules: %v", err)
		}
		return string(row.SyncStatus), row.SyncRetryCount
	case "pallets":
		row, err := client.Pallet.Get(ctx, id)
		if err != nil {
			t.Fatalf("reload pallets: %v", err)
		}
		return string(row.SyncStatus), row.SyncRetryCount
	case "bobbins":
		row, err := client.Bobbin.Get(ctx, id)
		if err != nil {
			t.Fatalf("reload bobbins: %v", err)
		}
		return string(row.SyncStatus), row.SyncRetryCount
	case "bobbin_grades":
		row, err := client.BobbinGrade.Get(ctx, id)
		if err != nil {
			t.Fatalf("reload bobbin_grades: %v", err)
		}
		return string(row.SyncStatus), row.SyncRetryCount
	}

	t.Fatalf("reloadSyncState 未覆盖表 %q —— 与 collectors() 不同步", table)
	return "", 0
}
