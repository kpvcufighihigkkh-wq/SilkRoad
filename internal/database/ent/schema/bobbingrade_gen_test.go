package schema_test

import (
	"testing"

	"github.com/yourusername/igh-silkroad/internal/database/ent/bobbingrade"
)

// BobbinGrade 必须已被 ent 生成，且带同步字段。
//
// 背景：本实体此前因 Annotations() 签名不匹配被 ent 静默跳过，
// 生成包里根本没有它，故这里用生成包的存在性作为回归防线。
func TestBobbinGrade_IsGeneratedWithSyncFields(t *testing.T) {
	if got := bobbingrade.FieldSyncStatus; got != "sync_status" {
		t.Errorf("FieldSyncStatus = %q, want %q", got, "sync_status")
	}
	if got := bobbingrade.FieldSyncedAt; got != "synced_at" {
		t.Errorf("FieldSyncedAt = %q, want %q", got, "synced_at")
	}
	if got := bobbingrade.FieldSyncRetryCount; got != "sync_retry_count" {
		t.Errorf("FieldSyncRetryCount = %q, want %q", got, "sync_retry_count")
	}
}
