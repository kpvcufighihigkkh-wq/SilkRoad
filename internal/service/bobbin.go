package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/bobbin"
)

// BobbinService 丝锭服务
type BobbinService struct {
	client *ent.Client
}

// NewBobbinService 创建丝锭服务
func NewBobbinService(client *ent.Client) *BobbinService {
	return &BobbinService{
		client: client,
	}
}

// CreateBobbinRequest 创建丝锭请求
type CreateBobbinRequest struct {
	BobbinNumber     string  `json:"bobbin_number" binding:"required"`
	LotID            string  `json:"lot_id" binding:"required"`
	SpinningPosition int     `json:"spinning_position" binding:"required,min=1"`
	GrossWeight      float64 `json:"gross_weight" binding:"required"`
	NetWeight        float64 `json:"net_weight" binding:"required"`
	TareWeight       float64 `json:"tare_weight"`
	Grade            string  `json:"grade"`
}

// BobbinResponse 丝锭响应
type BobbinResponse struct {
	ID               string  `json:"id"`
	BobbinNumber     string  `json:"bobbin_number"`
	LotID            string  `json:"lot_id"`
	SpinningPosition int     `json:"spinning_position"`
	GrossWeight      float64 `json:"gross_weight"`
	NetWeight        float64 `json:"net_weight"`
	TareWeight       float64 `json:"tare_weight"`
	Grade            string  `json:"grade,omitempty"`
	// PalletID 为 null 表示尚未码盘。托盘详情页靠它列出盘内丝锭，
	// 缺了这个字段就只剩一个计数，看不到是哪几枚。
	PalletID *string `json:"pallet_id"`
	// BarrelID / CartonID 同理：null 表示不在桶里 / 不在箱里。
	// 桶与纸箱详情页靠它们列出内容物；三者为互斥的上级容器。
	BarrelID     *string `json:"barrel_id"`
	CartonID     *string `json:"carton_id"`
	Status       string  `json:"status"`
	LabelPrinted bool    `json:"label_printed"`
	PrintedAt    *string `json:"printed_at,omitempty"`
	CompletedAt  *string `json:"completed_at,omitempty"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

// CreateBobbin 创建丝锭
func (s *BobbinService) CreateBobbin(ctx context.Context, req *CreateBobbinRequest) (*BobbinResponse, error) {
	lotID, err := uuid.Parse(req.LotID)
	if err != nil {
		return nil, err
	}

	builder := s.client.Bobbin.Create().
		SetBobbinNumber(req.BobbinNumber).
		SetLotID(lotID).
		SetSpinningPosition(req.SpinningPosition).
		SetGrossWeight(req.GrossWeight).
		SetNetWeight(req.NetWeight).
		SetStatus(bobbin.Status("producing")).
		SetLabelPrinted(false)

	if req.TareWeight > 0 {
		builder.SetTareWeight(req.TareWeight)
	}

	if req.Grade != "" {
		builder.SetGrade(req.Grade)
	}

	bobbin, err := builder.Save(ctx)
	if err != nil {
		return nil, err
	}

	return s.toBobbinResponse(bobbin), nil
}

// GetBobbin 获取丝锭详情
func (s *BobbinService) GetBobbin(ctx context.Context, id string) (*BobbinResponse, error) {
	bobbinID, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}

	bobbin, err := s.client.Bobbin.Get(ctx, bobbinID)
	if err != nil {
		return nil, err
	}

	return s.toBobbinResponse(bobbin), nil
}

// ListBobbins 查询丝锭列表
func (s *BobbinService) ListBobbins(ctx context.Context, page, pageSize int, lotID, status string) ([]*BobbinResponse, int, error) {
	query := s.client.Bobbin.Query()

	// 过滤条件
	if lotID != "" {
		id, err := uuid.Parse(lotID)
		if err == nil {
			query = query.Where(bobbin.LotIDEQ(id))
		}
	}

	if status != "" {
		query = query.Where(bobbin.StatusEQ(bobbin.Status(status)))
	}

	// 查询总数
	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	// 分页查询
	bobbins, err := query.
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Order(ent.Desc("created_at")).
		All(ctx)

	if err != nil {
		return nil, 0, err
	}

	var result []*BobbinResponse
	for _, b := range bobbins {
		result = append(result, s.toBobbinResponse(b))
	}

	return result, total, nil
}

// UpdateBobbinStatus 更新丝锭状态
func (s *BobbinService) UpdateBobbinStatus(ctx context.Context, id, status string) error {
	bobbinID, err := uuid.Parse(id)
	if err != nil {
		return &ServiceError{Code: 10001, Message: "无效的丝锭ID"}
	}

	// 先校验枚举：生成代码在 save 前才跑 StatusValidator，其错误没有可判别的
	// 类型，冒到 HTTP 层会成为 500 —— 而路由的 @Failure 400 承诺的是入参错误。
	// 提前校验，把这条路径收敛到与其他入参错误一致的 10001。
	if err := bobbin.StatusValidator(bobbin.Status(status)); err != nil {
		return &ServiceError{Code: 10001, Message: "无效的丝锭状态：" + status}
	}

	b, err := s.client.Bobbin.Get(ctx, bobbinID)
	if err != nil {
		if ent.IsNotFound(err) {
			return &ServiceError{Code: 40001, Message: "丝锭不存在"}
		}
		return err
	}

	return b.Update().
		SetStatus(bobbin.Status(status)).
		Exec(ctx)
}

// MarkBobbinPrinted 标记丝锭已打印
func (s *BobbinService) MarkBobbinPrinted(ctx context.Context, id string) error {
	bobbinID, err := uuid.Parse(id)
	if err != nil {
		return &ServiceError{Code: 10001, Message: "无效的丝锭ID"}
	}

	b, err := s.client.Bobbin.Get(ctx, bobbinID)
	if err != nil {
		if ent.IsNotFound(err) {
			return &ServiceError{Code: 40001, Message: "丝锭不存在"}
		}
		return err
	}

	return b.Update().
		SetLabelPrinted(true).
		Exec(ctx)
}

// DeleteBobbin 删除丝锭
func (s *BobbinService) DeleteBobbin(ctx context.Context, id string) error {
	bobbinID, err := uuid.Parse(id)
	if err != nil {
		return err
	}

	return s.client.Bobbin.DeleteOneID(bobbinID).Exec(ctx)
}

// toBobbinResponse 转换为响应格式
func (s *BobbinService) toBobbinResponse(b *ent.Bobbin) *BobbinResponse {
	resp := &BobbinResponse{
		ID:               b.ID.String(),
		BobbinNumber:     b.BobbinNumber,
		LotID:            b.LotID.String(),
		SpinningPosition: b.SpinningPosition,
		GrossWeight:      b.GrossWeight,
		NetWeight:        b.NetWeight,
		TareWeight:       b.TareWeight,
		Status:           string(b.Status),
		LabelPrinted:     b.LabelPrinted,
		CreatedAt:        b.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:        b.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	if b.Grade != "" {
		resp.Grade = b.Grade
	}

	// pallet_id 在 schema 里是 Optional：未码盘时为零值，转成 JSON null，
	// 避免前端把「未码盘」显示成一串全零 UUID。
	if b.PalletID != uuid.Nil {
		str := b.PalletID.String()
		resp.PalletID = &str
	}
	if b.BarrelID != uuid.Nil {
		str := b.BarrelID.String()
		resp.BarrelID = &str
	}
	if b.CartonID != uuid.Nil {
		str := b.CartonID.String()
		resp.CartonID = &str
	}

	if !b.PrintedAt.IsZero() {
		str := b.PrintedAt.Format("2006-01-02T15:04:05Z07:00")
		resp.PrintedAt = &str
	}

	if !b.CompletedAt.IsZero() {
		str := b.CompletedAt.Format("2006-01-02T15:04:05Z07:00")
		resp.CompletedAt = &str
	}

	return resp
}

// WeighBobbinRequest 丝锭称重请求
type WeighBobbinRequest struct {
	GrossWeight float64 `json:"gross_weight" binding:"required,gt=0"`
	TareWeight  float64 `json:"tare_weight" binding:"required,gte=0"`
	NetWeight   float64 `json:"net_weight" binding:"required,gt=0"`
}

// InspectBobbinRequest 丝锭质检请求
type InspectBobbinRequest struct {
	Grade       string `json:"grade" binding:"required"`
	InspectorID string `json:"inspector_id"`
	DefectNote  string `json:"defect_note"`
	// IsQualified 是现场陈述的合格性，可选。
	//
	// 用指针以区分「未提供」与「明确为 false」：未提供时该列落 schema
	// 默认值 true（见 schema/bobbingrade.go 的 Default(true)）—— 合格线
	// 是业务决策，不由本服务推断。
	IsQualified *bool `json:"is_qualified"`

	// Scores 五项等级类型分，可选但一旦提供必须齐全（见 validateScores）。
	Scores *InspectionScores `json:"scores"`
	// FinalScore 综合分，可选。与 Scores 独立：现场可能只给综合分，
	// 也可能五项齐全但另有加权公式算出的综合分。
	FinalScore *float64 `json:"final_score"`
	// InspectionDetails 质检项判定，可选。
	//
	// 键是质检项名（外观、重量、断头等），值是 pass/fail/na 三态。
	// 用限定词表而非自由文本：报表要按项统计合格率，自由文本会让
	// 「合格 / OK / 通过」变成三个不同的分组。
	InspectionDetails map[string]string `json:"inspection_details"`
}

// InspectionScores 五项等级类型分。
//
// 键与 grade.grade_type 枚举一一对应（sorting/weight/final/vision/knitting），
// 见 schema/grade.go。用显式结构体而非 map：现场表单少填一项时要能报错，
// map 里少一个键无从分辨「没这项」与「漏填」。
type InspectionScores struct {
	Sorting  *float64 `json:"sorting"`
	Weight   *float64 `json:"weight"`
	Final    *float64 `json:"final"`
	Vision   *float64 `json:"vision"`
	Knitting *float64 `json:"knitting"`
}

// inspectionDetailVerdicts 是质检项取值的限定词表。
var inspectionDetailVerdicts = map[string]struct{}{
	"pass": {},
	"fail": {},
	"na":   {},
}

// scoreMax 是单项分与综合分的上限。
//
// 现场评分口径是百分制。上限存在的意义不是业务限制，而是拦住把
// 「重量克数」误填进分数字段的输入错误。
const scoreMax = 100

// validateScores 校验五项分：要么整块缺席，要么五项齐全且各自在 [0,100]。
//
// 不做部分填充：少一项的分数集无法算「五维度合格率」，而缺项会被
// SUM/AVG 当成 0，把「没评」算成「评了 0 分」。
func validateScores(s *InspectionScores) error {
	if s == nil {
		return nil
	}

	fields := []struct {
		name string
		val  *float64
	}{
		{"sorting", s.Sorting},
		{"weight", s.Weight},
		{"final", s.Final},
		{"vision", s.Vision},
		{"knitting", s.Knitting},
	}
	for _, f := range fields {
		if f.val == nil {
			return &ServiceError{Code: 10001, Message: "评分缺少项：" + f.name + "（提供 scores 时五项必须齐全）"}
		}
		if *f.val < 0 || *f.val > scoreMax {
			return &ServiceError{Code: 10001, Message: "评分越界：" + f.name + " 必须在 0 到 100 之间"}
		}
	}

	return nil
}

// toScoreMap 把五项分转成落库用的 map。
//
// key 用 snake_case 而非枚举原值：vision/knitting 两边一致，但 weight/final
// 是 Go 关键字之外的普通词，保持与 grade_type 同名的好处是报表可直接
// 用 grade_type 拼键。故这里直接用枚举值。
func (s *InspectionScores) toScoreMap() map[string]interface{} {
	return map[string]interface{}{
		"sorting":  *s.Sorting,
		"weight":   *s.Weight,
		"final":    *s.Final,
		"vision":   *s.Vision,
		"knitting": *s.Knitting,
	}
}

// WeighBobbin 丝锭称重
func (s *BobbinService) WeighBobbin(ctx context.Context, id string, req *WeighBobbinRequest) error {
	bobbinID, err := uuid.Parse(id)
	if err != nil {
		return &ServiceError{Code: 10001, Message: "无效的丝锭ID"}
	}

	b, err := s.client.Bobbin.Get(ctx, bobbinID)
	if err != nil {
		if ent.IsNotFound(err) {
			return &ServiceError{Code: 40001, Message: "丝锭不存在"}
		}
		return err
	}

	// 验证重量逻辑：净重 = 毛重 - 皮重
	expectedNet := req.GrossWeight - req.TareWeight
	if abs(expectedNet-req.NetWeight) > 0.01 { // 允许0.01kg误差
		return &ServiceError{Code: 10001, Message: "重量数据不一致：净重应等于毛重减去皮重"}
	}

	return b.Update().
		SetGrossWeight(req.GrossWeight).
		SetTareWeight(req.TareWeight).
		SetNetWeight(req.NetWeight).
		Exec(ctx)
}

// InspectBobbin 丝锭质检
func (s *BobbinService) InspectBobbin(ctx context.Context, id string, req *InspectBobbinRequest) error {
	bobbinID, err := uuid.Parse(id)
	if err != nil {
		return &ServiceError{Code: 10001, Message: "无效的丝锭ID"}
	}

	b, err := s.client.Bobbin.Get(ctx, bobbinID)
	if err != nil {
		if ent.IsNotFound(err) {
			return &ServiceError{Code: 40001, Message: "丝锭不存在"}
		}
		return err
	}

	// 质检员与缺陷原因是可选字段：空串时传 nil，让列保持 NULL。
	var inspectorID *uuid.UUID
	if req.InspectorID != "" {
		parsed, err := uuid.Parse(req.InspectorID)
		if err != nil {
			return &ServiceError{Code: 10001, Message: "无效的质检员ID"}
		}
		inspectorID = &parsed
	}
	var defectReason *string
	if req.DefectNote != "" {
		defectReason = &req.DefectNote
	}

	// 评分与质检项明细：可选，但一旦提供就必须合法。
	//
	// 校验放在任何写库之前：InspectBobbin 会先更新 bobbin.grade 再插历史行，
	// 若在插入时才发现评分不合法，丝饼的等级已经被改了而质检记录没有落 ——
	// 现场看到的是「等级变了但查不到这条质检」。
	if err := validateScores(req.Scores); err != nil {
		return err
	}
	if req.FinalScore != nil && (*req.FinalScore < 0 || *req.FinalScore > scoreMax) {
		return &ServiceError{Code: 10001, Message: "综合分必须在 0 到 100 之间"}
	}
	for item, verdict := range req.InspectionDetails {
		if item == "" {
			return &ServiceError{Code: 10001, Message: "质检项名称不能为空"}
		}
		if _, ok := inspectionDetailVerdicts[verdict]; !ok {
			return &ServiceError{Code: 10001, Message: "质检项 " + item + " 的取值必须是 pass/fail/na，得到：" + verdict}
		}
	}

	if err := b.Update().SetGrade(req.Grade).Exec(ctx); err != nil {
		return err
	}

	// 保留完整质检历史（每次质检新增一条），同时把最新等级同步到 bobbin.grade。
	// 不覆盖历史：缺陷原因、评分等在现场只产生一次，丢失即不可恢复。
	//
	// is_qualified 用 Nillable 版本透传现场陈述：未提供（nil）时不写值，
	// 该列落 schema 默认值 true。
	builder := s.client.BobbinGrade.Create().
		SetBobbinID(bobbinID).
		SetGrade(req.Grade).
		SetNillableIsQualified(req.IsQualified).
		SetNillableInspectorID(inspectorID).
		SetNillableDefectReason(defectReason).
		SetNillableFinalScore(req.FinalScore)

	// 评分与质检项明细只在有值时才写：两列都是可空的 JSONB，
	// 写入空 map 会让「未录入」变成「录入了一个空对象」，报表按项展开时
	// 无从区分。
	if req.Scores != nil {
		builder.SetScores(req.Scores.toScoreMap())
	}
	if len(req.InspectionDetails) > 0 {
		details := make(map[string]interface{}, len(req.InspectionDetails))
		for k, v := range req.InspectionDetails {
			details[k] = v
		}
		builder.SetInspectionDetails(details)
	}

	if _, err := builder.Save(ctx); err != nil {
		return err
	}

	return nil
}

// abs 返回浮点数的绝对值
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
