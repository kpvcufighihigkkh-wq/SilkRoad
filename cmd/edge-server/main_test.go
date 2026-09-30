package main

import (
	"testing"
	"time"
)

// SYNC_INTERVAL 的非法取值必须回退到默认值。
//
// "0"、"0s"、"-5m" 都能通过 time.ParseDuration（err == nil），
// 但非正值传给 time.NewTicker 会在调度 goroutine 内 panic
// （"non-positive interval for NewTicker"）并终止进程 ——
// 而 SYNC_INTERVAL=0 是运维读作「关掉它」的合理解释。
func TestParseSyncInterval(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want time.Duration
	}{
		{"未配置", "", defaultSyncInterval},
		{"合法值", "30s", 30 * time.Second},
		{"合法值-分钟", "2m", 2 * time.Minute},
		{"零", "0", defaultSyncInterval},
		{"零秒", "0s", defaultSyncInterval},
		{"负数", "-5m", defaultSyncInterval},
		{"无法解析", "abc", defaultSyncInterval},
		{"纯数字无单位", "5", defaultSyncInterval},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseSyncInterval(tc.raw)
			if got != tc.want {
				t.Errorf("parseSyncInterval(%q) = %s, want %s", tc.raw, got, tc.want)
			}
			// 回退值必须能安全交给 NewTicker —— 这正是本检查要防的 panic
			if got <= 0 {
				t.Fatalf("parseSyncInterval(%q) 返回非正值 %s，会在 NewTicker 处 panic", tc.raw, got)
			}
		})
	}
}
