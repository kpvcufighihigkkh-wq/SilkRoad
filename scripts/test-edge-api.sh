#!/bin/bash
# IGH Edge Server API 测试脚本

BASE_URL="http://localhost:8081"
TOKEN=""

# 颜色输出
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "=========================================="
echo "IGH Edge Server API 测试"
echo "=========================================="

# 1. 健康检查
echo -e "\n${YELLOW}[TEST 1]${NC} 健康检查"
HEALTH=$(curl -s "$BASE_URL/health")
if echo "$HEALTH" | grep -q "\"code\":0"; then
    echo -e "${GREEN}✓ 通过${NC}"
    echo "$HEALTH" | head -3
else
    echo -e "${RED}✗ 失败${NC}"
    echo "$HEALTH"
    exit 1
fi

# 2. 登录获取Token
echo -e "\n${YELLOW}[TEST 2]${NC} 用户登录"
LOGIN_RESPONSE=$(curl -s -X POST "$BASE_URL/v1/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"edge","password":"edge123"}')

if echo "$LOGIN_RESPONSE" | grep -q "\"code\":0"; then
    echo -e "${GREEN}✓ 通过${NC}"
    TOKEN=$(echo "$LOGIN_RESPONSE" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
    echo "Token获取成功: ${TOKEN:0:50}..."
else
    echo -e "${RED}✗ 失败${NC}"
    echo "$LOGIN_RESPONSE"
    exit 1
fi

# 3. 创建落纱操作
echo -e "\n${YELLOW}[TEST 3]${NC} 创建落纱操作"
CREATE_RESPONSE=$(curl -s -X POST "$BASE_URL/v1/doffing" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "spinning_line_id": "00000000-0000-0000-0000-000000000001",
    "spinning_position": 1,
    "lot_id": "00000000-0000-0000-0000-000000000002"
  }')

if echo "$CREATE_RESPONSE" | grep -q "\"code\":0"; then
    echo -e "${GREEN}✓ 通过${NC}"
    DOFFING_ID=$(echo "$CREATE_RESPONSE" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
    echo "落纱ID: $DOFFING_ID"
else
    echo -e "${RED}✗ 失败${NC}"
    echo "$CREATE_RESPONSE"
    exit 1
fi

# 4. 查询落纱列表
echo -e "\n${YELLOW}[TEST 4]${NC} 查询落纱列表"
LIST_RESPONSE=$(curl -s "$BASE_URL/v1/doffing?page=1&page_size=10" \
  -H "Authorization: Bearer $TOKEN")

if echo "$LIST_RESPONSE" | grep -q "\"code\":0"; then
    echo -e "${GREEN}✓ 通过${NC}"
    echo "查询成功"
else
    echo -e "${RED}✗ 失败${NC}"
    echo "$LIST_RESPONSE"
    exit 1
fi

# 5. 确认落纱
echo -e "\n${YELLOW}[TEST 5]${NC} 确认落纱"
CONFIRM_RESPONSE=$(curl -s -X PUT "$BASE_URL/v1/doffing/$DOFFING_ID/confirm" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "actual_weight": 5.2,
    "bobbin_number": "BOB-TEST-001",
    "grade": "A"
  }')

if echo "$CONFIRM_RESPONSE" | grep -q "\"code\":0"; then
    echo -e "${GREEN}✓ 通过${NC}"
    echo "确认成功"
else
    echo -e "${RED}✗ 失败${NC}"
    echo "$CONFIRM_RESPONSE"
    exit 1
fi

# 6. 查看确认后的落纱
echo -e "\n${YELLOW}[TEST 6]${NC} 查看落纱详情"
DETAIL_RESPONSE=$(curl -s "$BASE_URL/v1/doffing/$DOFFING_ID" \
  -H "Authorization: Bearer $TOKEN")

if echo "$DETAIL_RESPONSE" | grep -q "\"status\":\"confirmed\""; then
    echo -e "${GREEN}✓ 通过${NC}"
    echo "状态已更新为confirmed"
else
    echo -e "${RED}✗ 失败${NC}"
    echo "$DETAIL_RESPONSE"
    exit 1
fi

echo -e "\n=========================================="
echo -e "${GREEN}所有测试通过！${NC}"
echo "=========================================="
