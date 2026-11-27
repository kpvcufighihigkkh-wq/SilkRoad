// 生成演示数据：落筒 → 丝饼 → 质检 → 收桶 → 码盘 → 封托，用于页面截图。
//
// 只写开发库；以「批次 + 时间窗」为单位造数，重复执行会新增记录而不是覆盖，
// 因此每次运行前会按 lot_number 前缀先清理本程序造的数据。
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/bobbin"
	"github.com/yourusername/igh-silkroad/internal/database/ent/carton"
	"github.com/yourusername/igh-silkroad/internal/database/ent/doffing"
	"github.com/yourusername/igh-silkroad/internal/database/ent/edge"
	"github.com/yourusername/igh-silkroad/internal/database/ent/lot"
	"github.com/yourusername/igh-silkroad/internal/database/ent/pallet"

	"entgo.io/ent/dialect"
	_ "github.com/lib/pq"
)

const (
	// 每个落筒 12 枚，与客户口径一致。
	bobbinsPerDoffing = 12
	// POY 一车 2 个落筒，FDY 一车 3 个。
	doffingsPerPOY   = 2
	doffingsPerFDY   = 3
	bobbinsPerCarton = 12
	cartonsPerPallet = 4
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://igh:igh_dev_password@localhost:5433/igh?sslmode=disable"
	}

	client, err := ent.Open(dialect.Postgres, dsn)
	if err != nil {
		log.Fatalf("open postgres: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// 1) 修复测试设备名称乱码（编码问题写入的数据，仅演示库）。
	if err := fixEdgeName(ctx, client); err != nil {
		log.Fatalf("fix edge name: %v", err)
	}

	// 2) 三个批次各造一套完整链路。
	if err := seedDemo(ctx, client); err != nil {
		log.Fatalf("seed demo: %v", err)
	}

	log.Println("✅ demo data ready")
}

func fixEdgeName(ctx context.Context, client *ent.Client) error {
	row, err := client.Edge.Query().Where(edge.EdgeCodeEQ("edge-e2e-pallet")).Only(ctx)
	if ent.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if row.EdgeName == "二号边端（端到端验证）" {
		return nil
	}
	_, err = row.Update().SetEdgeName("二号边端（端到端验证）").Save(ctx)
	if err != nil {
		return err
	}
	log.Printf("  ✓ 修复设备名称: %s", row.EdgeCode)
	return nil
}

func seedDemo(ctx context.Context, client *ent.Client) error {
	edgeRow, err := client.Edge.Query().Where(edge.EdgeCodeEQ("edge-001")).Only(ctx)
	if err != nil {
		return fmt.Errorf("query edge-001: %w", err)
	}

	lineRow, err := client.SpinningLine.Query().Where().First(ctx)
	if err != nil {
		return fmt.Errorf("query spinning line: %w", err)
	}

	inspector, err := client.User.Query().Where().First(ctx)
	if err != nil {
		return fmt.Errorf("query inspector: %w", err)
	}

	// 三组演示批次，覆盖 FDY / POY / DTY 三条工艺路线。
	type demo struct {
		lotNumber  string
		plcLot     string
		product    lot.ProductType
		spec       string
		orderCode  string
		planned    int
		doffings   int
		withCarton bool
	}

	demos := []demo{
		{"FDY-2026-002-01", "PLC-2026-002", lot.ProductTypeFDY, "150D/48F", "SO-2026-1101", 4800, 9, false},
		{"POY-2026-003-01", "PLC-2026-003", lot.ProductTypePOY, "250D/72F", "SO-2026-1102", 2400, 6, false},
		{"DTY-2026-004-01", "PLC-2026-004", lot.ProductTypeDTY, "167D/48F", "SO-2026-1103", 1200, 4, true},
	}

	for i, d := range demos {
		if err := seedOne(ctx, client, edgeRow, lineRow, inspector, d.lotNumber, d.plcLot,
			d.product, d.spec, d.orderCode, d.planned, d.doffings, d.withCarton, i); err != nil {
			return fmt.Errorf("seed %s: %w", d.lotNumber, err)
		}
	}
	return nil
}

func seedOne(
	ctx context.Context,
	client *ent.Client,
	edgeRow *ent.Edge,
	lineRow *ent.SpinningLine,
	inspector *ent.User,
	lotNumber, plcLot string,
	product lot.ProductType,
	spec, orderCode string,
	planned, doffingCount int,
	withCarton bool,
	offsetDays int,
) error {
	// 幂等：同名批次已存在则跳过，避免重复造数。
	exists, err := client.Lot.Query().Where(lot.LotNumberEQ(lotNumber)).Exist(ctx)
	if err != nil {
		return err
	}
	if exists {
		log.Printf("  · 跳过已存在的批次 %s", lotNumber)
		return nil
	}

	base := time.Now().AddDate(0, 0, -offsetDays-1).Truncate(time.Minute)

	lotRow, err := client.Lot.Create().
		SetLotNumber(lotNumber).
		SetEdgeID(edgeRow.ID).
		SetPlcLotNumber(plcLot).
		SetOrderCode(orderCode).
		SetProductType(product).
		SetProductSpec(spec).
		SetPlannedQuantity(planned).
		SetActualQuantity(doffingCount * bobbinsPerDoffing).
		SetStatus(lot.StatusInProgress).
		SetStartTime(base).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("create lot: %w", err)
	}
	log.Printf("  ✓ 批次 %s（%d 落筒 × %d 枚）", lotNumber, doffingCount, bobbinsPerDoffing)

	bobbinIDs := make([]uuid.UUID, 0, doffingCount*bobbinsPerDoffing)

	for d := 0; d < doffingCount; d++ {
		doffingTime := base.Add(time.Duration(d*45) * time.Minute)

		// 落筒记录：一个锭位一条（现有模型口径）。
		pos := d%bobbinsPerDoffing + 1
		doffRow, err := client.Doffing.Create().
			SetSpinningLineID(lineRow.ID).
			SetSpinningPosition(pos).
			SetLotID(lotRow.ID).
			SetDoffingTime(doffingTime).
			SetStatus(doffing.StatusConfirmed).
			SetConfirmedAt(doffingTime.Add(2 * time.Minute)).
			SetActualWeight(round3(14.6 + float64(d%5)*0.13)).
			SetBobbinNumber(fmt.Sprintf("%s-B%02d", lotNumber, pos)).
			SetGrade(gradeAt(d)).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("create doffing: %w", err)
		}
		_ = doffRow

		// 一次落筒 12 枚丝饼：位置取 1..12，重量围绕 14.8kg 波动。
		for b := 0; b < bobbinsPerDoffing; b++ {
			pos := b + 1
			net := round3(14.55 + float64((d*7+b*3)%11)*0.085)
			tare := 0.42
			grade := gradeAt(d + b)

			row, err := client.Bobbin.Create().
				SetBobbinNumber(fmt.Sprintf("%s-D%02d-B%02d", lotNumber, d+1, pos)).
				SetLotID(lotRow.ID).
				SetSpinningPosition(pos).
				SetGrossWeight(round3(net + tare)).
				SetNetWeight(net).
				SetTareWeight(tare).
				SetGrade(grade).
				SetStatus(bobbin.StatusCompleted).
				SetCompletedAt(doffingTime).
				Save(ctx)
			if err != nil {
				return fmt.Errorf("create bobbin: %w", err)
			}
			bobbinIDs = append(bobbinIDs, row.ID)

			// 质检：每第 5 枚不合格，其余合格；有一条留未判定（NULL）。
			qualified := true
			var nilFlag *bool = &qualified
			reason := ""
			scores := map[string]interface{}{
				"linear_density": 92, "strength": 88, "elongation": 90,
				"oil_pickup": 95, "appearance": 87,
			}
			details := map[string]interface{}{
				"appearance": "pass", "weight": "pass", "broken_end": "pass",
				"oil_stain": "pass", "hairiness": "pass",
			}
			final := 90.4
			if (d*bobbinsPerDoffing+b)%17 == 5 {
				f := false
				nilFlag = &f
				reason = "毛丝"
				scores["appearance"] = 62
				details["appearance"] = "fail"
				details["hairiness"] = "fail"
				final = 71.5
			} else if (d*bobbinsPerDoffing+b)%19 == 7 {
				nilFlag = nil
				final = 84.0
				details["oil_stain"] = "na"
			}

			q := client.BobbinGrade.Create().
				SetBobbinID(row.ID).
				SetInspectorID(inspector.ID).
				SetGrade(grade).
				SetInspectionDetails(details).
				SetScores(scores).
				SetFinalScore(final).
				SetInspectedAt(doffingTime.Add(8 * time.Minute))
			if nilFlag != nil {
				q = q.SetIsQualified(*nilFlag)
			}
			if reason != "" {
				q = q.SetDefectReason(reason)
			}
			if _, err := q.Save(ctx); err != nil {
				return fmt.Errorf("create grade: %w", err)
			}
		}
	}

	// 收桶：每 9 枚一桶（POY/FDY），DTY 走纸箱。
	if !withCarton {
		if err := seedBarrels(ctx, client, lotRow, bobbinIDs); err != nil {
			return err
		}
	}

	// 码盘：POY/FDY 直接码锭，DTY 先装箱再码垛。
	if withCarton {
		if err := seedCartonPallet(ctx, client, edgeRow, lotRow, bobbinIDs, base); err != nil {
			return err
		}
	} else {
		if err := seedBobbinPallet(ctx, client, edgeRow, lotRow, bobbinIDs, base); err != nil {
			return err
		}
	}

	return nil
}

func seedBarrels(ctx context.Context, client *ent.Client, lotRow *ent.Lot, bobbinIDs []uuid.UUID) error {
	const capacity = 9
	barrelIDs := make([]uuid.UUID, 0)

	for i := 0; i*capacity < len(bobbinIDs); i++ {
		end := (i + 1) * capacity
		if end > len(bobbinIDs) {
			end = len(bobbinIDs)
		}
		chunk := bobbinIDs[i*capacity : end]

		barrelRow, err := client.Barrel.Create().
			SetBarrelNumber(fmt.Sprintf("BRL-%s-%02d", lotRow.LotNumber, i+1)).
			SetLotID(lotRow.ID).
			SetCapacity(capacity).
			SetCurrentCount(len(chunk)).
			SetStatus("full").
			SetFilledAt(time.Now().Add(-time.Duration(20-i) * time.Minute)).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("create barrel: %w", err)
		}
		barrelIDs = append(barrelIDs, barrelRow.ID)

		for pos, id := range chunk {
			if _, err := client.Bobbin.UpdateOneID(id).
				SetBarrelID(barrelRow.ID).
				SetBarrelPosition(pos + 1).
				Save(ctx); err != nil {
				return fmt.Errorf("load bobbin: %w", err)
			}
		}
	}

	// 吊车：每 2 桶一车。
	for i := 0; i < len(barrelIDs); i += 2 {
		builder := client.Module.Create().
			SetModuleNumber(fmt.Sprintf("MDL-%s-%02d", lotRow.LotNumber, i/2+1)).
			SetStatus("warehouse").
			SetRfid(fmt.Sprintf("E200-DEMO-%03d", i/2+1)).
			SetCurrentLocation(fmt.Sprintf("A区%d号货道", i/2+1)).
			SetBarrel1ID(barrelIDs[i])
		if i+1 < len(barrelIDs) {
			builder = builder.SetBarrel2ID(barrelIDs[i+1])
		}
		if _, err := builder.Save(ctx); err != nil {
			return fmt.Errorf("create module: %w", err)
		}
	}
	log.Printf("  ✓ 桶 %d 个 / 吊车 %d 台", len(barrelIDs), (len(barrelIDs)+1)/2)
	return nil
}

func seedBobbinPallet(
	ctx context.Context, client *ent.Client, edgeRow *ent.Edge,
	lotRow *ent.Lot, bobbinIDs []uuid.UUID, base time.Time,
) error {
	const perPallet = 24
	palletIDs := make([]uuid.UUID, 0)

	for i := 0; i*perPallet < len(bobbinIDs); i++ {
		end := (i + 1) * perPallet
		if end > len(bobbinIDs) {
			end = len(bobbinIDs)
		}
		chunk := bobbinIDs[i*perPallet : end]

		net := 0.0
		for _, id := range chunk {
			row, err := client.Bobbin.Get(ctx, id)
			if err != nil {
				return err
			}
			net += row.NetWeight
		}
		tare := 22.5
		status := pallet.StatusCompleted
		if i == 0 && len(bobbinIDs) > perPallet {
			status = pallet.StatusShipped
		}

		palletRow, err := client.Pallet.Create().
			SetPalletCode(fmt.Sprintf("PLT-%s-%02d", lotRow.LotNumber, i+1)).
			SetLotID(lotRow.ID).
			SetEdgeID(edgeRow.ID).
			SetLevel(1).
			SetPalletType(pallet.PalletTypeBobbin).
			SetBobbinsCount(len(chunk)).
			SetCartonsCount(0).
			SetNetWeight(round2(net)).
			SetGrossWeight(round2(net + tare)).
			SetTareWeight(tare).
			SetStatus(status).
			SetCompletedAt(base.Add(time.Duration(240+i*15) * time.Minute)).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("create pallet: %w", err)
		}
		palletIDs = append(palletIDs, palletRow.ID)

		for _, id := range chunk {
			if _, err := client.Bobbin.UpdateOneID(id).
				SetPalletID(palletRow.ID).
				SetStatus(bobbin.StatusPacked).
				Save(ctx); err != nil {
				return fmt.Errorf("pack bobbin: %w", err)
			}
		}
	}
	log.Printf("  ✓ 托盘 %d 个（码锭）", len(palletIDs))
	return nil
}

func seedCartonPallet(
	ctx context.Context, client *ent.Client, edgeRow *ent.Edge,
	lotRow *ent.Lot, bobbinIDs []uuid.UUID, base time.Time,
) error {
	cartonIDs := make([]uuid.UUID, 0)

	for i := 0; i*bobbinsPerCarton < len(bobbinIDs); i++ {
		end := (i + 1) * bobbinsPerCarton
		if end > len(bobbinIDs) {
			end = len(bobbinIDs)
		}
		chunk := bobbinIDs[i*bobbinsPerCarton : end]

		weight := 0.0
		for _, id := range chunk {
			row, err := client.Bobbin.Get(ctx, id)
			if err != nil {
				return err
			}
			weight += row.NetWeight
		}

		cartonRow, err := client.Carton.Create().
			SetCartonNumber(fmt.Sprintf("CTN-%s-%02d", lotRow.LotNumber, i+1)).
			SetEdgeID(edgeRow.ID).
			SetLotID(lotRow.ID).
			SetCapacity(bobbinsPerCarton).
			SetBobbinCount(len(chunk)).
			SetTotalWeight(round2(weight)).
			SetStatus(carton.StatusPacked).
			SetPackedAt(base.Add(time.Duration(300+i*8) * time.Minute)).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("create carton: %w", err)
		}
		cartonIDs = append(cartonIDs, cartonRow.ID)

		for _, id := range chunk {
			if _, err := client.Bobbin.UpdateOneID(id).
				SetCartonID(cartonRow.ID).
				SetStatus(bobbin.StatusPacked).
				Save(ctx); err != nil {
				return fmt.Errorf("pack bobbin into carton: %w", err)
			}
		}
	}

	for i := 0; i*cartonsPerPallet < len(cartonIDs); i++ {
		end := (i + 1) * cartonsPerPallet
		if end > len(cartonIDs) {
			end = len(cartonIDs)
		}
		chunk := cartonIDs[i*cartonsPerPallet : end]

		weight := 0.0
		for _, id := range chunk {
			row, err := client.Carton.Get(ctx, id)
			if err != nil {
				return err
			}
			weight += row.TotalWeight
		}
		tare := 25.0

		palletRow, err := client.Pallet.Create().
			SetPalletCode(fmt.Sprintf("PLT-%s-%02d", lotRow.LotNumber, i+1)).
			SetLotID(lotRow.ID).
			SetEdgeID(edgeRow.ID).
			SetLevel(1).
			SetPalletType(pallet.PalletTypeCarton).
			SetBobbinsCount(len(chunk) * bobbinsPerCarton).
			SetCartonsCount(len(chunk)).
			SetNetWeight(round2(weight)).
			SetGrossWeight(round2(weight + tare)).
			SetTareWeight(tare).
			SetStatus(pallet.StatusCompleted).
			SetCompletedAt(base.Add(time.Duration(420+i*20) * time.Minute)).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("create carton pallet: %w", err)
		}

		for _, id := range chunk {
			if _, err := client.Carton.UpdateOneID(id).
				SetPalletID(palletRow.ID).
				Save(ctx); err != nil {
				return fmt.Errorf("pack carton: %w", err)
			}
		}
	}
	log.Printf("  ✓ 纸箱 %d 个 / 托盘 %d 个（码箱）", len(cartonIDs), (len(cartonIDs)+cartonsPerPallet-1)/cartonsPerPallet)
	return nil
}

// gradeAt 按序号给出有分布的等级，让看板的等级分布不为单一值。
func gradeAt(i int) string {
	switch i % 10 {
	case 0, 1, 2, 3, 4, 5:
		return "A"
	case 6, 7, 8:
		return "B"
	default:
		return "C"
	}
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

func round3(v float64) float64 {
	return float64(int(v*1000+0.5)) / 1000
}

// 确保未使用的导入不会导致编译失败（doffing 包用于枚举常量）。
var _ = doffing.StatusConfirmed
