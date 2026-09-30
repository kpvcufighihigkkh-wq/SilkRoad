package edge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/barrel"
	"github.com/yourusername/igh-silkroad/internal/database/ent/bobbin"
	"github.com/yourusername/igh-silkroad/internal/database/ent/doffing"
	"github.com/yourusername/igh-silkroad/internal/database/ent/lot"
	"github.com/yourusername/igh-silkroad/internal/sync/models"
)

const (
	// maxRetryCount 超过此次数的记录不再重试，等待人工介入
	maxRetryCount = 5

	// uploadBatchSize 单次上传的最大条数
	uploadBatchSize = 100
)

// markableTables 需要维护同步状态的表，供 markSynced / markRetried /
// markExhausted 遍历。
//
// 注意：这比实际采集的表更多。未采集的表不会有新记录进入 pending，
// 但历史上（或人工修补后）已存在的 pending 行仍需被正确标记 ——
// 尤其是 markExhausted 要把重试超限的行翻成 failed，否则它们会
// 永远停留在 pending 且不被采集，成为不可见的积压。
var markableTables = []string{"lots", "doffings", "barrels", "bobbins"}

// uploadableEntity 一个可上传实体：表名与采集函数
type uploadableEntity struct {
	table   string
	collect func(ctx context.Context, limit int) ([]models.UploadEntry, error)
}

// Uploader 负责把 Edge 本地的待同步记录推送到 Center
type Uploader struct {
	client    *ent.Client
	edgeID    string
	centerURL string
	token     string
	http      *http.Client

	// uploadMu 保证同一时刻只有一次上传在飞行。
	//
	// 用 TryLock 直接跳过而不是排队等待，因为「串行化并不等于去重」：
	// 一次失败的上传会让记录保持 pending，排在后面的调用者重新读取
	// 后会再次烧掉同一批记录的重试次数。5 次并发与 5 次串行都会把
	// retry_count 从 1 推到 5，直接触发 markExhausted。
	// 跳过则是安全的：记录仍是 pending，下一次触发会重新采集。
	uploadMu sync.Mutex
}

// NewUploader 创建上传器
func NewUploader(client *ent.Client, edgeID, centerURL, token string) *Uploader {
	return &Uploader{
		client:    client,
		edgeID:    edgeID,
		centerURL: centerURL,
		token:     token,
		http:      &http.Client{Timeout: 30 * time.Second},
	}
}

// collectors 返回启用采集的实体，顺序即外键依赖顺序：被引用者先上传。
//
// 只采集 Center 的 handleCreate（center/handler.go:121-130）真正接受的表：
// 目前是 lots 与 bobbins。
//
// 不在此列表中的表不会被采集，因而其记录保持 pending 且 retry_count 不变 ——
// 这是「延后」而非「丢弃」，等 Center 支持后加上来即可同步。
// 反之，采集了却被 Center 以 unknown table 拒绝的记录，会在重试上限后
// 被标记为 failed，而本仓库没有任何回收路径 —— 那才是真正的数据丢失。
//
// 重新加入的条件（两者都必须满足）：
//   - doffings：Center 的 handleCreate 增加 "doffings" 分支
//   - barrels：Center 的 handleCreate 增加 "barrels" 分支
//
// 在此之前不得把它们加回本列表。
func (u *Uploader) collectors() []uploadableEntity {
	return []uploadableEntity{
		{table: "lots", collect: u.collectLots},
		{table: "bobbins", collect: u.BobbinQuery},
	}
}

// CollectPending 收集待同步记录，按外键依赖顺序排列。
// 跳过已达重试上限的记录。
func (u *Uploader) CollectPending(ctx context.Context, limit int) ([]models.UploadEntry, error) {
	entries := make([]models.UploadEntry, 0, limit)

	for _, entity := range u.collectors() {
		remaining := limit - len(entries)
		if remaining <= 0 {
			break
		}

		batch, err := entity.collect(ctx, remaining)
		if err != nil {
			return nil, err
		}
		entries = append(entries, batch...)
	}

	return entries, nil
}

// collectLots 查询待同步批次
func (u *Uploader) collectLots(ctx context.Context, limit int) ([]models.UploadEntry, error) {
	rows, err := u.client.Lot.Query().
		Where(
			lot.SyncStatusEQ(lot.SyncStatusPending),
			lot.SyncRetryCountLT(maxRetryCount),
		).
		Order(ent.Asc(lot.FieldCreatedAt)).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query lots: %w", err)
	}

	out := make([]models.UploadEntry, 0, len(rows))
	for _, r := range rows {
		out = append(out, lotToEntry(r))
	}
	return out, nil
}

// collectDoffings 查询待同步落纱记录。
//
// 当前未被 collectors() 调用：Center 的 handleCreate 还只有 lots/bobbins 分支，
// doffings 上传会被判 unknown table 并永久走向 failed。保留此函数是**重新接入的
// 脚手架** —— Center 的 handleCreate 加上 "doffings" 分支后，把它加回
// collectors() 即可，无需重写采集逻辑（字段映射与 Center 的 createDoffing 对齐）。
// 在此之前不要加回 collectors()。
func (u *Uploader) collectDoffings(ctx context.Context, limit int) ([]models.UploadEntry, error) {
	rows, err := u.client.Doffing.Query().
		Where(
			doffing.SyncStatusEQ(doffing.SyncStatusPending),
			doffing.SyncRetryCountLT(maxRetryCount),
		).
		Order(ent.Asc(doffing.FieldCreatedAt)).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query doffings: %w", err)
	}

	out := make([]models.UploadEntry, 0, len(rows))
	for _, r := range rows {
		out = append(out, models.UploadEntry{
			Table:     "doffings",
			Operation: "create",
			ID:        r.ID,
			Data: map[string]interface{}{
				"id":                r.ID.String(),
				"lot_id":            r.LotID.String(),
				"spinning_line_id":  r.SpinningLineID.String(),
				"spinning_position": r.SpinningPosition,
				"status":            string(r.Status),
			},
			CreatedAt: r.CreatedAt,
		})
	}
	return out, nil
}

// collectBarrels 查询待同步落纱桶。
//
// 当前未被 collectors() 调用，理由同 collectDoffings：Center 的 handleCreate
// 尚未支持 barrels。作为重新接入的脚手架保留，待 Center 侧补上 "barrels"
// 分支后再加回 collectors()。
func (u *Uploader) collectBarrels(ctx context.Context, limit int) ([]models.UploadEntry, error) {
	rows, err := u.client.Barrel.Query().
		Where(
			barrel.SyncStatusEQ(barrel.SyncStatusPending),
			barrel.SyncRetryCountLT(maxRetryCount),
		).
		Order(ent.Asc(barrel.FieldCreatedAt)).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query barrels: %w", err)
	}

	out := make([]models.UploadEntry, 0, len(rows))
	for _, r := range rows {
		out = append(out, models.UploadEntry{
			Table:     "barrels",
			Operation: "create",
			ID:        r.ID,
			Data: map[string]interface{}{
				"id":            r.ID.String(),
				"barrel_number": r.BarrelNumber,
				"lot_id":        r.LotID.String(),
				"capacity":      r.Capacity,
				"current_count": r.CurrentCount,
				"status":        string(r.Status),
			},
			CreatedAt: r.CreatedAt,
		})
	}
	return out, nil
}

// BobbinQuery 查询待同步丝饼（独立方法便于测试与复用）
func (u *Uploader) BobbinQuery(ctx context.Context, limit int) ([]models.UploadEntry, error) {
	rows, err := u.client.Bobbin.Query().
		Where(
			bobbin.SyncStatusEQ(bobbin.SyncStatusPending),
			bobbin.SyncRetryCountLT(maxRetryCount),
		).
		Order(ent.Asc(bobbin.FieldCreatedAt)).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query bobbins: %w", err)
	}

	out := make([]models.UploadEntry, 0, len(rows))
	for _, r := range rows {
		data := map[string]interface{}{
			"id":                r.ID.String(),
			"bobbin_number":     r.BobbinNumber,
			"lot_id":            r.LotID.String(),
			"spinning_position": r.SpinningPosition,
			"gross_weight":      r.GrossWeight,
			"net_weight":        r.NetWeight,
			"status":            string(r.Status),
			"label_printed":     r.LabelPrinted,
		}
		if r.BarrelID != uuid.Nil {
			data["barrel_id"] = r.BarrelID.String()
		}
		if r.BarrelPosition != 0 {
			data["barrel_position"] = r.BarrelPosition
		}
		out = append(out, models.UploadEntry{
			Table:     "bobbins",
			Operation: "create",
			ID:        r.ID,
			Data:      data,
			CreatedAt: r.CreatedAt,
		})
	}

	return out, nil
}

// lotToEntry 把批次记录转为上传条目
func lotToEntry(r *ent.Lot) models.UploadEntry {
	data := map[string]interface{}{
		"id":               r.ID.String(),
		"lot_number":       r.LotNumber,
		"product_type":     string(r.ProductType),
		"planned_quantity": r.PlannedQuantity,
		"actual_quantity":  r.ActualQuantity,
		"status":           string(r.Status),
	}
	if r.PlcLotNumber != "" {
		data["plc_lot_number"] = r.PlcLotNumber
	}
	if r.OrderCode != "" {
		data["order_code"] = r.OrderCode
	}
	if r.ProductSpec != "" {
		data["product_spec"] = r.ProductSpec
	}

	return models.UploadEntry{
		Table:     "lots",
		Operation: "create",
		ID:        r.ID,
		Data:      data,
		CreatedAt: r.CreatedAt,
	}
}

// ErrUploadInProgress 表示已有一次上传在飞行，本次调用被跳过。
//
// 调用方应把它当作「稍后再试」而非失败：被跳过的记录仍是 pending，
// 下一次触发会重新采集。
var ErrUploadInProgress = errors.New("upload already in progress")

// Upload 执行一次上传：收集、发送、按响应逐条更新本地同步状态。
//
// 同一时刻只允许一次上传在飞行；重入时立即返回 ErrUploadInProgress，
// 不排队等待（理由见 uploadMu 字段注释）。
func (u *Uploader) Upload(ctx context.Context) (*models.UploadResponse, error) {
	if !u.uploadMu.TryLock() {
		return nil, ErrUploadInProgress
	}
	defer u.uploadMu.Unlock()

	entries, err := u.CollectPending(ctx, uploadBatchSize)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return &models.UploadResponse{}, nil
	}

	payload := &models.UploadRequest{
		EdgeID:  u.edgeID,
		Entries: entries,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal upload request: %w", err)
	}

	// 必须用 :code 寻址 —— Center 的 authenticateEdge 拿 c.Param("code")
	// 与 token 里的 DeviceID 比对，路径缺段会被判为「凭证与请求设备不符」。
	// edgeID 来自 EDGE_ID 环境变量，转义后再拼路径，避免其中的 / ? # 等
	// 字符破坏路径或注入额外查询参数。
	uploadURL := fmt.Sprintf("%s/v1/edges/%s/upload", u.centerURL, url.PathEscape(u.edgeID))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+u.token)

	resp, err := u.http.Do(httpReq)
	if err != nil {
		// 网络故障：结果未知，整批计入重试，下轮再传
		if markErr := u.markRetried(ctx, perTableIDs(entries)); markErr != nil {
			log.Printf("⚠️  累加重试计数出错: %v", markErr)
		}
		return nil, fmt.Errorf("post upload: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	// 凭证类失败（401 未提供/无效 token、403 未注册或设备不符）不计入重试。
	//
	// 这类失败重发同样的请求必然同样失败，与记录本身无关 —— 若计入
	// retry_count，默认 5m 间隔下约 25 分钟后所有 pending 记录都会被
	// markExhausted 标记为 failed，而本仓库没有回收路径，等于数据丢失。
	// 保持 pending 则配置修好后可继续同步。
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("upload unauthorized: status=%d body=%s（凭证或设备注册问题，不计入重试）",
			resp.StatusCode, string(respBody))
	}

	if resp.StatusCode != http.StatusOK {
		if markErr := u.markRetried(ctx, perTableIDs(entries)); markErr != nil {
			log.Printf("⚠️  累加重试计数出错: %v", markErr)
		}
		return nil, fmt.Errorf("upload rejected: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var parsed struct {
		Code int                   `json:"code"`
		Data models.UploadResponse `json:"data"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		// 响应体不可解析 = 结果未知。同样计入重试，否则这些记录会以
		// retry_count 恒为 0 的状态无限重试，永远到不了 failed。
		if markErr := u.markRetried(ctx, perTableIDs(entries)); markErr != nil {
			log.Printf("⚠️  累加重试计数出错: %v", markErr)
		}
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if err := u.applyOutcome(ctx, entries, &parsed.Data); err != nil {
		log.Printf("⚠️  更新同步状态出错: %v", err)
	}

	return &parsed.Data, nil
}

// applyOutcome 按响应标注本地状态：确认应用成功的标记 synced，其余累加重试计数。
func (u *Uploader) applyOutcome(ctx context.Context, entries []models.UploadEntry, resp *models.UploadResponse) error {
	mark := pendingMarkFrom(entries, resp)

	// perTableIDs 只在有记录时才建 key，因此非空即表示「有值」。
	var errs []error
	if len(mark.synced) > 0 {
		if err := u.markSynced(ctx, mark.synced); err != nil {
			errs = append(errs, fmt.Errorf("mark synced: %w", err))
		}
	}
	if len(mark.retried) > 0 {
		if err := u.markRetried(ctx, mark.retried); err != nil {
			errs = append(errs, fmt.Errorf("mark retried: %w", err))
		}
	}

	return errors.Join(errs...)
}

// pendingMark 一次上传后的本地状态标注（逐表聚合的记录 ID）
type pendingMark struct {
	synced  map[string][]uuid.UUID
	retried map[string][]uuid.UUID
}

// pendingMarkFrom 从 Center 的响应推导逐条标注。
//
// Center 的 HandleUpload 是逐条处理的：失败一条只累加 rejected 并继续
// （handler.go:87-95），因此 applied=2, rejected=1 是可达状态。
// 相应的错误串格式固定为 fmt.Sprintf("%s/%s: %v", entry.Table, entry.ID, err)
// （handler.go:90），即 "<table>/<uuid>: <message>"，据此可以逐条定位被拒记录。
//
// 被拒的记录只累加重试计数、不改状态，因而下轮仍会被收集重试；
// 只有确认应用的记录才标记 synced。
//
// 任何对不上的情形都退化为「整批计重试」：计数不相符、无法解析错误串、
// 解析出的 ID 数与 rejected 不符、或错误串指向本批之外的记录。
// 这是安全的 —— Center 的 create* 先查存在再写入（handler.go:144-151），
// 重发是空操作而非重复插入。多传一轮的代价是一次往返，
// 猜错 synced 的代价是记录被静默丢弃。
func pendingMarkFrom(entries []models.UploadEntry, resp *models.UploadResponse) pendingMark {
	retryAll := func(reason string) pendingMark {
		log.Printf("⚠️  %s，保守处理：整批计入重试", reason)
		return pendingMark{retried: perTableIDs(entries)}
	}

	if resp.Applied+resp.Rejected != len(entries) {
		return retryAll(fmt.Sprintf("响应计数与本批不符（applied=%d rejected=%d 本批=%d）",
			resp.Applied, resp.Rejected, len(entries)))
	}

	if resp.Rejected == 0 {
		return pendingMark{synced: perTableIDs(entries)}
	}

	rejectedIDs, ok := parseRejectedIDs(resp.Errors)
	if !ok || len(rejectedIDs) != resp.Rejected {
		return retryAll(fmt.Sprintf("无法从响应中定位被拒记录（rejected=%d errors=%d）",
			resp.Rejected, len(resp.Errors)))
	}

	batchIDs := make(map[uuid.UUID]struct{}, len(entries))
	for _, e := range entries {
		batchIDs[e.ID] = struct{}{}
	}
	for id := range rejectedIDs {
		if _, inBatch := batchIDs[id]; !inBatch {
			return retryAll(fmt.Sprintf("错误串指向本批之外的记录 %s", id))
		}
	}

	mark := pendingMark{
		synced:  make(map[string][]uuid.UUID, len(entries)),
		retried: make(map[string][]uuid.UUID, len(resp.Errors)),
	}
	for _, e := range entries {
		if _, rejected := rejectedIDs[e.ID]; rejected {
			mark.retried[e.Table] = append(mark.retried[e.Table], e.ID)
			continue
		}
		mark.synced[e.Table] = append(mark.synced[e.Table], e.ID)
	}

	return mark
}

// parseRejectedIDs 从 Center 的错误串中解析被拒记录的 ID。
//
// 错误串形如 "<table>/<uuid>: <message>"（handler.go:90）。
// 只切第一个冒号 —— 错误消息本身可能含冒号（如 "invalid entry id: ..."），
// 一并匹配会解析失败并退化为整批重试。
// 表名需为小写字母，避免把 "invalid entry id" 这类无表名的消息误解析成 ID。
func parseRejectedIDs(errors []string) (map[uuid.UUID]struct{}, bool) {
	out := make(map[uuid.UUID]struct{}, len(errors))

	for _, msg := range errors {
		token, _, found := strings.Cut(msg, ":")
		if !found {
			return nil, false
		}

		table, idStr, found := strings.Cut(strings.TrimSpace(token), "/")
		if !found || !isLowerASCII(table) {
			return nil, false
		}

		id, err := uuid.Parse(strings.TrimSpace(idStr))
		if err != nil {
			return nil, false
		}
		out[id] = struct{}{}
	}

	return out, true
}

// isLowerASCII 判断是否为非空的小写 ASCII 字母串
func isLowerASCII(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}

// perTableIDs 按表名聚合记录 ID。
// 只产出有记录的键；每个键下的列表非空。
func perTableIDs(entries []models.UploadEntry) map[string][]uuid.UUID {
	out := make(map[string][]uuid.UUID)
	for _, e := range entries {
		out[e.Table] = append(out[e.Table], e.ID)
	}
	return out
}

// forEachTable 对 markableTables 中出现在 byTable 的表分别执行 fn
func (u *Uploader) forEachTable(
	ctx context.Context,
	byTable map[string][]uuid.UUID,
	fn func(ctx context.Context, table string, ids []uuid.UUID) error,
) error {
	for _, table := range markableTables {
		ids := byTable[table]
		if len(ids) == 0 {
			continue
		}
		if err := fn(ctx, table, ids); err != nil {
			return err
		}
	}
	return nil
}

// markSynced 把确认应用成功的记录标记为已同步
func (u *Uploader) markSynced(ctx context.Context, byTable map[string][]uuid.UUID) error {
	now := time.Now()

	return u.forEachTable(ctx, byTable, func(ctx context.Context, table string, ids []uuid.UUID) error {
		switch table {
		case "lots":
			return u.client.Lot.Update().
				Where(lot.IDIn(ids...)).
				SetSyncStatus(lot.SyncStatusSynced).
				SetSyncedAt(now).
				Exec(ctx)
		case "doffings":
			return u.client.Doffing.Update().
				Where(doffing.IDIn(ids...)).
				SetSyncStatus(doffing.SyncStatusSynced).
				SetSyncedAt(now).
				Exec(ctx)
		case "barrels":
			return u.client.Barrel.Update().
				Where(barrel.IDIn(ids...)).
				SetSyncStatus(barrel.SyncStatusSynced).
				SetSyncedAt(now).
				Exec(ctx)
		case "bobbins":
			return u.client.Bobbin.Update().
				Where(bobbin.IDIn(ids...)).
				SetSyncStatus(bobbin.SyncStatusSynced).
				SetSyncedAt(now).
				Exec(ctx)
		}
		return nil
	})
}

// markRetried 累加重试计数，并在达上限时标记为 failed。
//
// 不改变 sync_status —— 记录保持 pending，下轮仍会被收集，
// 直到 markExhausted 把它标记为 failed。
func (u *Uploader) markRetried(ctx context.Context, byTable map[string][]uuid.UUID) error {
	err := u.forEachTable(ctx, byTable, func(ctx context.Context, table string, ids []uuid.UUID) error {
		switch table {
		case "lots":
			return u.client.Lot.Update().Where(lot.IDIn(ids...)).AddSyncRetryCount(1).Exec(ctx)
		case "doffings":
			return u.client.Doffing.Update().Where(doffing.IDIn(ids...)).AddSyncRetryCount(1).Exec(ctx)
		case "barrels":
			return u.client.Barrel.Update().Where(barrel.IDIn(ids...)).AddSyncRetryCount(1).Exec(ctx)
		case "bobbins":
			return u.client.Bobbin.Update().Where(bobbin.IDIn(ids...)).AddSyncRetryCount(1).Exec(ctx)
		}
		return nil
	})
	if err != nil {
		return err
	}

	// 达上限的记录标记为 failed，避免无限重试
	return u.markExhausted(ctx)
}

// markExhausted 把重试超限的 pending 记录标记为 failed，
// 避免它们被无限次重新收集。
//
// 覆盖 markableTables 中的全部四张表（而非仅当前被采集的两张）：不被采集的表
// 其 retry_count 不会增长，但历史上（或人工写入）已达上限的行仍需被翻成 failed，
// 否则会永远停在 pending 且无人可见。
func (u *Uploader) markExhausted(ctx context.Context) error {
	if _, err := u.client.Lot.Update().
		Where(
			lot.SyncRetryCountGTE(maxRetryCount),
			lot.SyncStatusEQ(lot.SyncStatusPending),
		).
		SetSyncStatus(lot.SyncStatusFailed).
		Save(ctx); err != nil {
		return err
	}

	if _, err := u.client.Doffing.Update().
		Where(
			doffing.SyncRetryCountGTE(maxRetryCount),
			doffing.SyncStatusEQ(doffing.SyncStatusPending),
		).
		SetSyncStatus(doffing.SyncStatusFailed).
		Save(ctx); err != nil {
		return err
	}

	if _, err := u.client.Barrel.Update().
		Where(
			barrel.SyncRetryCountGTE(maxRetryCount),
			barrel.SyncStatusEQ(barrel.SyncStatusPending),
		).
		SetSyncStatus(barrel.SyncStatusFailed).
		Save(ctx); err != nil {
		return err
	}

	if _, err := u.client.Bobbin.Update().
		Where(
			bobbin.SyncRetryCountGTE(maxRetryCount),
			bobbin.SyncStatusEQ(bobbin.SyncStatusPending),
		).
		SetSyncStatus(bobbin.SyncStatusFailed).
		Save(ctx); err != nil {
		return err
	}

	return nil
}
