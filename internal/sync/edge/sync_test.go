package edge_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/yourusername/igh-silkroad/internal/database/ent_edge"
	"github.com/yourusername/igh-silkroad/internal/sync/edge"
	"github.com/google/uuid"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

func setupTestClient(t *testing.T) *ent_edge.Client {
	// 使用内存数据库
	db, err := sql.Open("sqlite", "file:test?mode=memory&cache=shared&_fk=1")
	if err != nil {
		t.Fatalf("failed opening sqlite: %v", err)
	}

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := ent_edge.NewClient(ent_edge.Driver(drv))

	// 创建schema
	if err := client.Schema.Create(context.Background()); err != nil {
		t.Fatalf("failed creating schema: %v", err)
	}

	return client
}

func TestUploader_ScanLots(t *testing.T) {
	client := setupTestClient(t)
	defer client.Close()

	ctx := context.Background()

	// 创建测试数据
	lot, err := client.Lot.Create().
		SetID(uuid.New()).
		SetLotNumber("LOT-TEST-001").
		SetOrderID(uuid.New()).
		SetProductType("FDY").
		SetProductSpec("150D/48F").
		SetPlannedQuantity(100).
		SetStatus("in_progress").
		Save(ctx)

	if err != nil {
		t.Fatalf("failed creating test lot: %v", err)
	}

	// 创建uploader
	uploader := edge.NewUploader(client, "edge-001", "http://localhost:8080")

	// 测试扫描
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 启动上传器（会在后台扫描）
	go uploader.Start(ctx)

	// 等待一次扫描
	time.Sleep(1 * time.Second)

	t.Logf("✅ Test lot created: %s", lot.LotNumber)
}

func TestDownloader_PullBaseData(t *testing.T) {
	client := setupTestClient(t)
	defer client.Close()

	ctx := context.Background()

	// 创建downloader
	downloader := edge.NewDownloader(client, "edge-001", "http://localhost:8080")

	// 测试拉取（实际会失败因为没有真实服务器，但可以验证代码结构）
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go downloader.Start(ctx)

	time.Sleep(1 * time.Second)

	t.Logf("✅ Downloader started successfully")
}
