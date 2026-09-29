# 数据库迁移指南

## 概述

本文档说明如何执行IGH Silkroad的数据库迁移，包括Center PostgreSQL数据库和Edge SQLite数据库。

## 迁移工具

位置：`cmd/migrate/main.go`  
编译后：`bin/migrate.exe`

### 功能特性

- ✅ 支持Center (PostgreSQL) 和 Edge (SQLite) 数据库
- ✅ 自动创建新表和修改现有表结构
- ✅ 删除不再使用的列和索引
- ✅ 数据库统计信息打印
- ✅ 提供手动清理旧表的SQL提示

### 命令行参数

```bash
migrate [flags] [center|edge] [dsn]

Flags:
  -drop-old    显示手动删除旧表的SQL提示
  -dry-run     保留但暂不支持（显示警告）
```

## Edge数据库迁移（SQLite）

### 默认配置

```bash
# 使用默认DSN
./bin/migrate.exe edge

# 等同于
./bin/migrate.exe edge "file:edge.db?cache=shared&_fk=1"
```

### 自定义配置

```bash
# 指定数据库文件路径
./bin/migrate.exe edge "file:/path/to/edge.db?cache=shared&_fk=1"

# 显示清理提示
./bin/migrate.exe -drop-old edge
```

### 迁移结果

✅ **已验证成功** (2026-09-29)

```
=== Edge Database Migration ===
DSN: file:edge-test.db?cache=shared&_fk=1

[1/2] Running schema migration...
  Note: Old tables (orders, projects) will be kept but disconnected
  Use manual cleanup if needed: DROP TABLE orders; DROP TABLE projects;

[2/2] Database statistics:
  Lots           : 0 records
  Doffings       : 0 records
  Bobbins        : 0 records
  Users          : 0 records

✅ Edge database schema migrated successfully
```

## Center数据库迁移（PostgreSQL）

### 前置条件

1. **安装PostgreSQL** (推荐版本 14+)
   - Windows: https://www.postgresql.org/download/windows/
   - macOS: `brew install postgresql@14`
   - Linux: `sudo apt install postgresql-14`

2. **启动PostgreSQL服务**
   ```bash
   # Windows (服务)
   net start postgresql-x64-14
   
   # macOS
   brew services start postgresql@14
   
   # Linux
   sudo systemctl start postgresql
   ```

3. **创建数据库和用户**
   ```bash
   # 连接到PostgreSQL
   psql -U postgres
   
   # 创建用户和数据库
   CREATE USER igh WITH PASSWORD 'igh';
   CREATE DATABASE igh OWNER igh;
   GRANT ALL PRIVILEGES ON DATABASE igh TO igh;
   
   # 退出
   \q
   ```

### 执行迁移

```bash
# 使用默认DSN
./bin/migrate.exe center

# 等同于
./bin/migrate.exe center "postgres://igh:igh@localhost:5432/igh?sslmode=disable"

# 使用自定义DSN
./bin/migrate.exe center "postgres://user:pass@host:port/dbname?sslmode=disable"

# 显示清理提示
./bin/migrate.exe -drop-old center
```

### 预期结果

```
=== Center Database Migration ===
DSN: postgres://igh:****@localhost:5432/igh?sslmode=disable

[1/2] Running schema migration...
  Note: Old tables (orders, projects) will be kept but disconnected
  Use manual cleanup if needed: DROP TABLE orders CASCADE; DROP TABLE projects CASCADE;

[2/2] Database statistics:
  Edges          : 0 records
  SpinningLines  : 0 records
  Lots           : 0 records
  Doffings       : 0 records
  Barrels        : 0 records
  Bobbins        : 0 records
  Pallets        : 0 records
  Modules        : 0 records
  Grades         : 0 records
  Users          : 0 records

✅ Center database schema migrated successfully
```

## 数据库Schema变更

### 删除的表
- ❌ `orders` - 系统中不存在订单管理
- ❌ `projects` - 系统中不存在项目管理

### 新增的表
- ➕ `barrels` - 落纱桶（载具）
- ➕ `modules` - 吊车（载具）
- ➕ `edges` - 边端设备注册
- ➕ `grades` - 统一等级表

### 修改的表
- 🔧 `lots` - 删除order_id，添加edge_id、plc_lot_number
- 🔧 `bobbins` - 添加barrel_id、barrel_position
- 🔧 `pallets` - 重命名字段，添加level、palletizer_id
- 🔧 `doffings` - 添加Lot和Barrel关联
- 🔧 `spinning_lines` - 添加edge_id关联

### 数据流
```
PLC → Edge → Lot → Doffing → Barrel → Bobbin → Pallet
```

## 手动清理旧表（可选）

如果需要清理不再使用的旧表：

### PostgreSQL
```sql
DROP TABLE IF EXISTS orders CASCADE;
DROP TABLE IF EXISTS projects CASCADE;
```

### SQLite
```sql
DROP TABLE IF EXISTS orders;
DROP TABLE IF EXISTS projects;
```

## 故障排查

### PostgreSQL连接失败

**错误**: `password authentication failed for user "igh"`

**解决方案**:
1. 确认PostgreSQL服务已启动
2. 确认用户和密码正确
3. 检查pg_hba.conf配置允许本地连接
4. 重新创建用户：
   ```sql
   DROP USER IF EXISTS igh;
   CREATE USER igh WITH PASSWORD 'igh';
   GRANT ALL PRIVILEGES ON DATABASE igh TO igh;
   ```

**错误**: `database "igh" does not exist`

**解决方案**:
```sql
CREATE DATABASE igh OWNER igh;
```

### SQLite外键错误

**错误**: `foreign key mismatch`

**解决方案**:
- 删除旧的数据库文件，重新创建
- 或者使用迁移工具自动修复

### 驱动问题

**错误**: `sql: unknown driver "sqlite3"`

**解决方案**:
- 确保使用`CGO_ENABLED=0`编译
- 已导入`modernc.org/sqlite`驱动

## 技术细节

### 使用的驱动
- **PostgreSQL**: `github.com/lib/pq`
- **SQLite**: `modernc.org/sqlite` (纯Go实现，无需CGO)

### 迁移选项
```go
schema.WithDropIndex(true)   // 删除不再使用的索引
schema.WithDropColumn(true)  // 删除不再使用的列
```

### 外键约束
- PostgreSQL: 支持CASCADE删除
- SQLite: 需要手动启用外键 `PRAGMA foreign_keys = ON;`

## 相关文档

- [V2业务模型设计](./design/v2-model-design.md)
- [数据库Schema重构](../internal/database/ent/schema/)
- [Ent迁移文档](https://entgo.io/docs/migrate/)

## 提交记录

- `526128c` - V2业务模型设计文档
- `a368678` - Schema重构（删除Project/Order）
- `2539bc8` - 生成Ent代码
- `a5fc312` - 增强数据库迁移工具
- `dd616dc` - 修复迁移工具编译错误
- `aec81e7` - 修复Edge数据库迁移

## 注意事项

⚠️ **生产环境迁移建议**:
1. 提前备份数据库
2. 在测试环境先验证迁移
3. 选择低峰期执行
4. 准备回滚方案
5. 监控迁移过程和结果

⚠️ **数据保护**:
- 迁移工具不会删除现有数据
- 旧表(orders, projects)会保留但不再使用
- 可以手动验证后再删除旧表
