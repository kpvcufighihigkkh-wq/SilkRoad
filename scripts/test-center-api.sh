#!/bin/bash
# IGH Center Server API 测试脚本

BASE_URL="http://localhost:8080"
TOKEN=""

# 颜色输出
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "=========================================="
echo "IGH Center Server API 测试"
echo "=========================================="

# 1. 健康检查
echo -e "\n${YELLOW}[TEST 1]${NC} 健康检查"
HEALTH=$(curl -s "$BASE_URL/health")
if echo "$HEALTH" | grep -q "\"code\":0"; then
    echo -e "${GREEN}✓ 通过${NC}"
else
    echo -e "${RED}✗ 失败${NC}"
    echo "$HEALTH"
    exit 1
fi

# 2. 登录获取Token
echo -e "\n${YELLOW}[TEST 2]${NC} 管理员登录"
LOGIN_RESPONSE=$(curl -s -X POST "$BASE_URL/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}')

if echo "$LOGIN_RESPONSE" | grep -q "\"code\":0"; then
    echo -e "${GREEN}✓ 通过${NC}"
    TOKEN=$(echo "$LOGIN_RESPONSE" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
    echo "Token获取成功"
else
    echo -e "${RED}✗ 失败${NC}"
    echo "$LOGIN_RESPONSE"
    exit 1
fi

# 3. 创建项目
echo -e "\n${YELLOW}[TEST 3]${NC} 创建项目"
PROJECT_RESPONSE=$(curl -s -X POST "$BASE_URL/v1/projects" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "project_name": "测试项目-API",
    "customer_name": "测试客户",
    "description": "API测试项目"
  }')

if echo "$PROJECT_RESPONSE" | grep -q "\"code\":0"; then
    echo -e "${GREEN}✓ 通过${NC}"
    PROJECT_ID=$(echo "$PROJECT_RESPONSE" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
    echo "项目ID: $PROJECT_ID"
else
    echo -e "${RED}✗ 失败${NC}"
    echo "$PROJECT_RESPONSE"
    exit 1
fi

# 4. 创建订单
echo -e "\n${YELLOW}[TEST 4]${NC} 创建订单"
ORDER_RESPONSE=$(curl -s -X POST "$BASE_URL/v1/orders" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"order_number\": \"ORD-TEST-$(date +%s)\",
    \"project_id\": \"$PROJECT_ID\",
    \"product_type\": \"FDY\",
    \"product_spec\": \"150D/48F\",
    \"order_quantity\": 1000
  }")

if echo "$ORDER_RESPONSE" | grep -q "\"code\":0"; then
    echo -e "${GREEN}✓ 通过${NC}"
    ORDER_ID=$(echo "$ORDER_RESPONSE" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
    echo "订单ID: $ORDER_ID"
else
    echo -e "${RED}✗ 失败${NC}"
    echo "$ORDER_RESPONSE"
    exit 1
fi

# 5. 创建批次
echo -e "\n${YELLOW}[TEST 5]${NC} 创建批次"
LOT_RESPONSE=$(curl -s -X POST "$BASE_URL/v1/lots" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"lot_number\": \"LOT-TEST-$(date +%s)\",
    \"order_id\": \"$ORDER_ID\",
    \"planned_quantity\": 100
  }")

if echo "$LOT_RESPONSE" | grep -q "\"code\":0"; then
    echo -e "${GREEN}✓ 通过${NC}"
    LOT_ID=$(echo "$LOT_RESPONSE" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
    echo "批次ID: $LOT_ID"
else
    echo -e "${RED}✗ 失败${NC}"
    echo "$LOT_RESPONSE"
    exit 1
fi

# 6. 创建丝锭
echo -e "\n${YELLOW}[TEST 6]${NC} 创建丝锭"
BOBBIN_RESPONSE=$(curl -s -X POST "$BASE_URL/v1/bobbins" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"bobbin_number\": \"BOB-TEST-$(date +%s)\",
    \"lot_id\": \"$LOT_ID\",
    \"spinning_position\": 1,
    \"gross_weight\": 5.8,
    \"net_weight\": 5.2,
    \"tare_weight\": 0.6,
    \"grade\": \"A\"
  }")

if echo "$BOBBIN_RESPONSE" | grep -q "\"code\":0"; then
    echo -e "${GREEN}✓ 通过${NC}"
    BOBBIN_ID=$(echo "$BOBBIN_RESPONSE" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
    echo "丝锭ID: $BOBBIN_ID"
else
    echo -e "${RED}✗ 失败${NC}"
    echo "$BOBBIN_RESPONSE"
    exit 1
fi

# 7. 查询订单列表
echo -e "\n${YELLOW}[TEST 7]${NC} 查询订单列表"
ORDERS_LIST=$(curl -s "$BASE_URL/v1/orders?page=1&page_size=10" \
  -H "Authorization: Bearer $TOKEN")

if echo "$ORDERS_LIST" | grep -q "\"code\":0"; then
    echo -e "${GREEN}✓ 通过${NC}"
else
    echo -e "${RED}✗ 失败${NC}"
    echo "$ORDERS_LIST"
    exit 1
fi

# 8. 查询丝锭列表
echo -e "\n${YELLOW}[TEST 8]${NC} 查询丝锭列表"
BOBBINS_LIST=$(curl -s "$BASE_URL/v1/bobbins?page=1&page_size=10" \
  -H "Authorization: Bearer $TOKEN")

if echo "$BOBBINS_LIST" | grep -q "\"code\":0"; then
    echo -e "${GREEN}✓ 通过${NC}"
else
    echo -e "${RED}✗ 失败${NC}"
    echo "$BOBBINS_LIST"
    exit 1
fi

# 9. 获取当前用户信息
echo -e "\n${YELLOW}[TEST 9]${NC} 获取用户信息"
USER_INFO=$(curl -s "$BASE_URL/v1/users/me" \
  -H "Authorization: Bearer $TOKEN")

if echo "$USER_INFO" | grep -q "\"username\":\"admin\""; then
    echo -e "${GREEN}✓ 通过${NC}"
else
    echo -e "${RED}✗ 失败${NC}"
    echo "$USER_INFO"
    exit 1
fi

echo -e "\n=========================================="
echo -e "${GREEN}所有测试通过！${NC}"
echo "=========================================="
echo ""
echo "创建的测试数据："
echo "  项目ID: $PROJECT_ID"
echo "  订单ID: $ORDER_ID"
echo "  批次ID: $LOT_ID"
echo "  丝锭ID: $BOBBIN_ID"
