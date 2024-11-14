# 介绍
monibuca 是一款纯 go 开发的扩展性极强的高性能流媒体服务器开发框架

# 使用
```go
package main
import (
	"context"

	"m7s.live/v5"
	_ "m7s.live/v5/plugin/debug"
	_ "m7s.live/v5/plugin/flv"
	_ "m7s.live/v5/plugin/rtmp"
)

func main() {
	m7s.Run(context.Background(), "config.yaml")
}

```
## 构建标签

| 标签 | 描述              |
|-----------|-----------------|
| disable_rm | 禁用内存池           |
| sqlite | 启用 sqlite       |
| sqliteCGO | 启用 sqlite cgo版本 |
| mysql | 启用 mysql       |
| postgres | 启用 postgres       |
| duckdb | 启用 duckdb       |
| taskpanic | 抛出 panic，用于测试   |


## 更多示例

查看 example 目录

# 创建插件

到 plugin 目录下查看 README_CN.md

# Prometheus

```yaml
scrape_configs:
  - job_name: "monibuca"
    metrics_path: "/api/metrics"
    static_configs:
      - targets: ["localhost:8080"]
```
