#!/bin/bash

# IGH Protobuf 代码生成脚本
# 生成 Go 代码（服务端和客户端）

set -e

# 颜色输出
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${YELLOW}开始生成 Protobuf 代码...${NC}"

# 目录定义
PROTO_DIR="api/proto"
OUT_DIR="internal/generated"

# 检查 protoc 是否安装
if ! command -v protoc &> /dev/null; then
    echo -e "${RED}错误: protoc 未安装${NC}"
    echo "请安装 Protocol Buffers 编译器："
    echo "  - macOS: brew install protobuf"
    echo "  - Windows: https://github.com/protocolbuffers/protobuf/releases"
    echo "  - Linux: apt-get install protobuf-compiler"
    exit 1
fi

# 检查 Go 插件是否安装
if ! command -v protoc-gen-go &> /dev/null; then
    echo -e "${YELLOW}安装 protoc-gen-go...${NC}"
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
fi

if ! command -v protoc-gen-go-grpc &> /dev/null; then
    echo -e "${YELLOW}安装 protoc-gen-go-grpc...${NC}"
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
fi

# 清理旧文件
if [ -d "$OUT_DIR" ]; then
    echo -e "${YELLOW}清理旧生成文件...${NC}"
    rm -rf "$OUT_DIR"
fi

# 创建输出目录
mkdir -p "$OUT_DIR"

# 生成 Go 代码
echo -e "${YELLOW}生成 common/v1...${NC}"
protoc \
  --proto_path="${PROTO_DIR}" \
  --go_out="${OUT_DIR}" \
  --go_opt=paths=source_relative \
  --go-grpc_out="${OUT_DIR}" \
  --go-grpc_opt=paths=source_relative \
  "${PROTO_DIR}/common/v1/common.proto"

echo -e "${YELLOW}生成 center/v1...${NC}"
protoc \
  --proto_path="${PROTO_DIR}" \
  --go_out="${OUT_DIR}" \
  --go_opt=paths=source_relative \
  --go-grpc_out="${OUT_DIR}" \
  --go-grpc_opt=paths=source_relative \
  "${PROTO_DIR}/center/v1/sync.proto"

echo -e "${YELLOW}生成 edge/v1...${NC}"
protoc \
  --proto_path="${PROTO_DIR}" \
  --go_out="${OUT_DIR}" \
  --go_opt=paths=source_relative \
  --go-grpc_out="${OUT_DIR}" \
  --go-grpc_opt=paths=source_relative \
  "${PROTO_DIR}/edge/v1/operations.proto"

# 统计生成文件
GENERATED_FILES=$(find "$OUT_DIR" -name "*.go" | wc -l)

echo ""
echo -e "${GREEN}✅ Protobuf 代码生成完成！${NC}"
echo -e "生成文件: ${GREEN}${GENERATED_FILES}${NC} 个 Go 文件"
echo ""
echo "生成结构："
tree "$OUT_DIR" -L 3 2>/dev/null || find "$OUT_DIR" -type f -name "*.go"

echo ""
echo -e "${GREEN}下一步：${NC}"
echo "  1. 实现 gRPC 服务端: internal/service/"
echo "  2. 实现 gRPC 客户端: internal/client/"
echo "  3. 编写集成测试: test/integration/"
