# 数据同步模块

本模块实现了Edge（边缘端）与Center（中心端）之间的双向数据同步机制。

## 架构概述

```
┌─────────────────┐         ┌─────────────────┐
│   Edge (边端)   │         │ Center (中心端) │
│                 │         │                 │
│  ┌───────────┐  │         │  ┌───────────┐  │
│  │ Uploader  │──┼────────►│  │  Handler  │  │
│  └───────────┘  │  上传    │  └───────────┘  │
│                 │         │                 │
│  ┌───────────┐  │         │  ┌───────────┐  │
│  │Downloader │◄─┼─────────┤  │ Provider  │  │
│  └───────────┘  │  下发    │  └───────────┘  │
│                 │         │                 │
│   SQLite DB     │         │ PostgreSQL DB   │
└─────────────────┘         └─────────────────┘
```

## 同步策略

### Edge → Center（数据上传）

- **触发方式**: 定时轮询（默认5秒）
- **批量大小**: 100条/批次
- **上传顺序**: 按外键依赖顺序（lots → bobbins → bobbin_grades）
- **幂等性**: 服务端检测重复ID，跳过已存在记录
- **游标管理**: 每个表维护独立的上传游标

**上传流程：**
1. 扫描SQLite中 `synced=false` 的记录
2. 按FK依赖顺序组装批次
3. POST到 `/edges/{id}/upload`
4. 根据响应中的 `applied` 数量判断是否推进游标
5. 标记已同步记录 `synced=true`

### Center → Edge（配置下发）

- **触发方式**: 定时拉取（默认30分钟）
- **拉取内容**: 产品配置、线体配置、项目信息
- **合并策略**: Upsert（存在则更新，不存在则创建）
- **时间戳**: 记录 `synced_at` 用于追踪更新时间

**下发流程：**
1. GET `/base-data/{table}` 请求基础数据
2. 接收JSON格式的记录列表
3. 本地Upsert到SQLite
4. 标记同步时间戳

## 使用示例

### Edge端启动同步

```go
package main

import (
    "context"
    "log"
    
    "github.com/yourusername/igh-silkroad/internal/database/ent_edge"
    "github.com/yourusername/igh-silkroad/internal/sync/edge"
)

func main() {
    // 初始化Edge数据库
    client, _ := ent_edge.Open("sqlite", "file:edge.db?cache=shared&_fk=1")
    defer client.Close()
    
    ctx := context.Background()
    
    // 创建上传器
    uploader := edge.NewUploader(client, "edge-001", "http://center:8080")
    go uploader.Start(ctx)
    
    // 创建下载器
    downloader := edge.NewDownloader(client, "edge-001", "http://center:8080")
    go downloader.Start(ctx)
    
    log.Println("✅ Sync modules started")
    
    // 保持运行
    select {}
}
```

### Center端处理上传

```go
package main

import (
    "context"
    "encoding/json"
    "log"
    "net/http"
    
    "github.com/yourusername/igh-silkroad/internal/database/ent"
    "github.com/yourusername/igh-silkroad/internal/sync/center"
    "github.com/yourusername/igh-silkroad/internal/sync/models"
)

func main() {
    // 初始化Center数据库
    client, _ := ent.Open("postgres", "postgres://user:pass@localhost/igh")
    defer client.Close()
    
    handler := center.NewUploadHandler(client)
    provider := center.NewBaseDataProvider(client)
    
    // 上传端点
    http.HandleFunc("/edges/upload", func(w http.ResponseWriter, r *http.Request) {
        var req models.UploadRequest
        json.NewDecoder(r.Body).Decode(&req)
        
        resp, err := handler.HandleUpload(r.Context(), &req)
        if err != nil {
            http.Error(w, err.Error(), 500)
            return
        }
        
        json.NewEncoder(w).Encode(resp)
    })
    
    // 下发端点
    http.HandleFunc("/base-data", func(w http.ResponseWriter, r *http.Request) {
        var req models.BaseDataPullRequest
        json.NewDecoder(r.Body).Decode(&req)
        
        resp, err := provider.HandlePullRequest(r.Context(), &req)
        if err != nil {
            http.Error(w, err.Error(), 500)
            return
        }
        
        json.NewEncoder(w).Encode(resp)
    })
    
    log.Println("✅ Center sync server started on :8080")
    http.ListenAndServe(":8080", nil)
}
```

## 冲突处理

当前实现采用 **Server Wins（服务器优先）** 策略：

- 相同ID记录已存在时，跳过Edge端的上传
- Edge端的修改不会覆盖Center端的数据
- 通过 `version` 字段可扩展为版本比较策略

未来可扩展策略：
- **Edge Wins**: 边端数据覆盖中心端
- **Merge**: 字段级合并（需要定义合并规则）
- **Last Write Wins**: 根据时间戳决定

## 离线支持

- Edge端具备完全离线运行能力
- Center不可用时，数据持续累积在SQLite
- 恢复连接后，自动从上次游标位置继续上传
- `synced=false` 标记确保不遗漏任何变更

## 性能考虑

- **批量处理**: 减少网络往返次数
- **游标推进**: 避免重复扫描已同步数据
- **索引优化**: `synced` 字段建立索引加速查询
- **连接池**: 复用HTTP连接
- **超时控制**: 避免长时间阻塞

## 测试

```bash
# 运行Edge端测试
go test ./internal/sync/edge -v

# 运行Center端测试（需要PostgreSQL）
go test ./internal/sync/center -v
```

## 待实现功能

- [ ] HTTP客户端实现（当前为占位符）
- [ ] 认证与授权（Edge Token）
- [ ] 压缩传输（gzip）
- [ ] 增量同步优化
- [ ] 冲突检测与解决策略
- [ ] 同步监控与告警
- [ ] 失败重试机制
- [ ] 网络断线重连
