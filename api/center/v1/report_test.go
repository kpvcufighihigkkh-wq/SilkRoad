package v1_test

import (
	"testing"
	"time"

	centerv1 "github.com/yourusername/igh-silkroad/api/center/v1"
)

// TestParseTimeRange 钉住报表端点的区间语义：
// 缺省最近七天、start 闭 end 开、非法输入与过宽区间必须报错而不是被静默修正。
func TestParseTimeRange(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name      string
		start     string
		end       string
		wantErr   bool
		wantStart time.Time
		wantEnd   time.Time
	}{
		{"都不传 → 最近七天", "", "", false,
			now.Add(-7 * 24 * time.Hour), now},
		{"只传 start", "2026-11-01T00:00:00Z", "", false,
			time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), now},
		{"只传 end", "", "2026-11-05T00:00:00Z", false,
			time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)},
		{"start 晚于 end", "2026-11-09T00:00:00Z", "2026-11-08T00:00:00Z", true, time.Time{}, time.Time{}},
		{"start 等于 end", "2026-11-09T00:00:00Z", "2026-11-09T00:00:00Z", true, time.Time{}, time.Time{}},
		{"start 格式非法", "2026/11/01", "", true, time.Time{}, time.Time{}},
		{"end 格式非法", "", "not-a-time", true, time.Time{}, time.Time{}},
		{"跨度 367 天", "2025-11-08T00:00:00Z", "2026-11-10T00:00:00Z", true, time.Time{}, time.Time{}},
		{"跨度恰好 366 天", "2025-11-09T00:00:00Z", "2026-11-10T00:00:00Z", false,
			time.Date(2025, 11, 9, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)},
		{"带偏移量的时间统一转 UTC", "2026-11-01T08:00:00+08:00", "2026-11-02T08:00:00+08:00", false,
			time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start, end, err := centerv1.ParseTimeRange(tc.start, tc.end, now)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got start=%v end=%v", start, end)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !start.Equal(tc.wantStart) || !end.Equal(tc.wantEnd) {
				t.Errorf("got [%v, %v), want [%v, %v)", start, end, tc.wantStart, tc.wantEnd)
			}
			if start.Location() != time.UTC || end.Location() != time.UTC {
				t.Errorf("want UTC, got %v and %v", start.Location(), end.Location())
			}
		})
	}
}

// TestParseTopN 钉住 top 参数的缺省值与上限。
// 越界一律报错：静默截断会让前端误以为拿到了自己想要的数量。
func TestParseTopN(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		want    int
		wantErr bool
	}{
		{"不传取缺省", "", 10, false},
		{"下界", "1", 1, false},
		{"上界", "50", 50, false},
		{"零", "0", 0, true},
		{"负数", "-1", 0, true},
		{"超过上限", "51", 0, true},
		{"非数字", "abc", 0, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := centerv1.ParseTopN(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got %d", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %d, want %d", got, tc.want)
			}
		})
	}
}
