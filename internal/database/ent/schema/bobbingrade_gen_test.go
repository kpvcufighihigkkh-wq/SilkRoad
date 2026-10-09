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

// Task 1 立下的「非负」保证必须真实生效。
//
// sync_retry_count 在 schema 中标注了 NonNegative()；如果 ent 仅声明
// 验证器但从未初始化它，AddSyncRetryCount(-1) 会无声地落库负值 ——
// 这正是 Task 1 要防的「存在即有效」假设。
// 这里直接调用已初始化的 SyncRetryCountValidator，确认边界确实被守住。
func TestBobbinGrade_SyncRetryCountValidatorGuardsNonNegative(t *testing.T) {
	// 0 是合法值（初始/归零后的状态）
	if err := bobbingrade.SyncRetryCountValidator(0); err != nil {
		t.Errorf("SyncRetryCountValidator(0) = %v, want nil（0 是合法初始值）", err)
	}
	// -1 违反 NonNegative，必须被拒绝
	if err := bobbingrade.SyncRetryCountValidator(-1); err == nil {
		t.Error("SyncRetryCountValidator(-1) = nil, want non-nil（负值不得通过验证）")
	}
}

