package service

import (
	"context"

	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/order"
	"github.com/google/uuid"
)

// OrderService 订单服务
type OrderService struct {
	client *ent.Client
}

// NewOrderService 创建订单服务
func NewOrderService(client *ent.Client) *OrderService {
	return &OrderService{
		client: client,
	}
}

// CreateOrderRequest 创建订单请求
type CreateOrderRequest struct {
	OrderNumber   string  `json:"order_number" binding:"required"`
	ProjectID     string  `json:"project_id" binding:"required"`
	CustomerName  string  `json:"customer_name" binding:"required"`
	ProductType   string  `json:"product_type" binding:"required"`
	ProductSpec   string  `json:"product_spec" binding:"required"`
	OrderQuantity int     `json:"order_quantity" binding:"required,min=1"`
	TargetWeight  float64 `json:"target_weight"`
}

// OrderResponse 订单响应
type OrderResponse struct {
	ID            string  `json:"id"`
	OrderNumber   string  `json:"order_number"`
	ProjectID     string  `json:"project_id"`
	CustomerName  string  `json:"customer_name"`
	ProductType   string  `json:"product_type"`
	ProductSpec   string  `json:"product_spec"`
	Status        string  `json:"status"`
	OrderQuantity int     `json:"order_quantity"`
	Progress      float64 `json:"progress"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
}

// CreateOrder 创建订单
func (s *OrderService) CreateOrder(ctx context.Context, req *CreateOrderRequest) (*OrderResponse, error) {
	projectID, err := uuid.Parse(req.ProjectID)
	if err != nil {
		return nil, err
	}

	order, err := s.client.Order.Create().
		SetOrderNumber(req.OrderNumber).
		SetProjectID(projectID).
		SetProductType(order.ProductType(req.ProductType)).
		SetProductSpec(req.ProductSpec).
		SetTargetQuantity(req.OrderQuantity).
		SetStatus(order.Status("pending")).
		Save(ctx)

	if err != nil {
		return nil, err
	}

	return &OrderResponse{
		ID:            order.ID.String(),
		OrderNumber:   order.OrderNumber,
		ProjectID:     order.ProjectID.String(),
		CustomerName:  "", // 不再存储在Order表中
		ProductType:   string(order.ProductType),
		ProductSpec:   order.ProductSpec,
		Status:        string(order.Status),
		OrderQuantity: order.TargetQuantity,
		Progress:      0,
		CreatedAt:     order.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:     order.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}

// GetOrder 获取订单详情
func (s *OrderService) GetOrder(ctx context.Context, id string) (*OrderResponse, error) {
	orderID, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}

	order, err := s.client.Order.Get(ctx, orderID)
	if err != nil {
		return nil, err
	}

	return &OrderResponse{
		ID:            order.ID.String(),
		OrderNumber:   order.OrderNumber,
		ProjectID:     order.ProjectID.String(),
		CustomerName:  "", // 不再存储在Order表中
		ProductType:   string(order.ProductType),
		ProductSpec:   order.ProductSpec,
		Status:        string(order.Status),
		OrderQuantity: order.TargetQuantity,
		Progress:      0, // TODO: 计算进度
		CreatedAt:     order.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:     order.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}

// ListOrders 查询订单列表
func (s *OrderService) ListOrders(ctx context.Context, page, pageSize int, status string) ([]*OrderResponse, int, error) {
	query := s.client.Order.Query()

	// 过滤条件
	if status != "" {
		query = query.Where()
	}

	// 查询总数
	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	// 分页查询
	orders, err := query.
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Order(ent.Desc("created_at")).
		All(ctx)

	if err != nil {
		return nil, 0, err
	}

	var result []*OrderResponse
	for _, order := range orders {
		result = append(result, &OrderResponse{
			ID:            order.ID.String(),
			OrderNumber:   order.OrderNumber,
			ProjectID:     order.ProjectID.String(),
			CustomerName:  "", // 不再存储在Order表中
			ProductType:   string(order.ProductType),
			ProductSpec:   order.ProductSpec,
			Status:        string(order.Status),
			OrderQuantity: order.TargetQuantity,
			Progress:      0,
			CreatedAt:     order.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:     order.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	return result, total, nil
}

// DeleteOrder 删除订单
func (s *OrderService) DeleteOrder(ctx context.Context, id string) error {
	orderID, err := uuid.Parse(id)
	if err != nil {
		return err
	}

	return s.client.Order.DeleteOneID(orderID).Exec(ctx)
}
