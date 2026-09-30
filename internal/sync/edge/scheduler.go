package edge

import (
	"context"
	"log"
	"time"
)

// Scheduler 定时触发上传重试。
// 实时上传由业务操作触发，本调度器是网络故障后的兜底。
type Scheduler struct {
	uploader *Uploader
	interval time.Duration
	stop     chan struct{}
	done     chan struct{}
}

// NewScheduler 创建调度器
func NewScheduler(uploader *Uploader, interval time.Duration) *Scheduler {
	return &Scheduler{
		uploader: uploader,
		interval: interval,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// Start 启动调度（非阻塞）
func (s *Scheduler) Start(ctx context.Context) {
	go func() {
		defer close(s.done)

		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-s.stop:
				return
			case <-ticker.C:
				resp, err := s.uploader.Upload(ctx)
				if err != nil {
					log.Printf("⚠️  定时同步失败: %v", err)
					continue
				}
				if resp.Applied > 0 || resp.Rejected > 0 {
					log.Printf("✅ 定时同步: applied=%d rejected=%d", resp.Applied, resp.Rejected)
				}
			}
		}
	}()
}

// Stop 停止调度并等待退出
func (s *Scheduler) Stop() {
	close(s.stop)
	<-s.done
}
