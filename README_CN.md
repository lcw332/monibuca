# Monibuca v5

<a id="readme-top"></a>

[![Contributors][contributors-shield]][contributors-url]
[![Forks][forks-shield]][forks-url]
[![Stargazers][stars-shield]][stars-url]
[![Issues][issues-shield]][issues-url]
[![AGPL License][license-shield]][license-url]
[![Go Reference](https://pkg.go.dev/badge/m7s.live/v5.svg)](https://pkg.go.dev/m7s.live/v5)

<br />
<div align="center">
  <a href="https://monibuca.com">
    <img src="https://monibuca.com/svg/logo.svg" alt="Logo" width="200">
  </a>

  <h1 align="center">Monibuca v5</h1>
  <p align="center">
    强大的纯 Go 开发的流媒体服务器开发框架
    <br />
    <a href="https://monibuca.com"><strong>官方网站 »</strong></a>
    <br />
    <br />
    <a href="https://github.com/langhuihui/monibuca/issues">报告问题</a>
    ·
    <a href="https://github.com/langhuihui/monibuca/issues">功能建议</a>
  </p>
</div>

<!-- 目录 -->
<details>
  <summary>目录</summary>
  <ol>
    <li><a href="#项目介绍">项目介绍</a></li>
    <li><a href="#快速开始">快速开始</a></li>
    <li><a href="#使用示例">使用示例</a></li>
    <li><a href="#构建选项">构建选项</a></li>
    <li><a href="#监控系统">监控系统</a></li>
    <li><a href="#插件开发">插件开发</a></li>
    <li><a href="#贡献指南">贡献指南</a></li>
    <li><a href="#许可证">许可证</a></li>
  </ol>
</details>

## 项目介绍

Monibuca（简称 m7s）是一款纯 Go 开发的开源流媒体服务器开发框架。它具有以下特点：

- 🚀 **高性能** - 无锁设计、部分手动管理内存、多核计算
- ⚡ **低延迟** - 0 等待转发、全链路亚秒级延迟
- 📦 **插件化** - 按需加载，无限扩展能力
- 🔧 **灵活性** - 高度可配置，满足各种流媒体场景需求
- 💪 **可扩展** - 支持分布式部署，轻松应对大规模场景
- 🔍 **调试友好** - 内置调试插件，支持实时性能监控和分析
- 🎥 **媒体处理** - 支持截图、转码、SEI 数据处理
- 🔄 **集群能力** - 内置级联和房间管理功能
- 🎮 **预览功能** - 支持视频预览、分屏预览、自定义分屏
- 🔐 **安全加密** - 提供加密传输和流鉴权能力
- 📊 **性能监控** - 支持压力测试和性能指标采集
- 📝 **日志管理** - 日志轮转、自动清理、自定义扩展
- 🎬 **录制回放** - 支持 MP4、HLS、FLV 格式录制、倍速播放、拖拽快进、暂停能力
- ⏱️ **动态时移** - 动态缓存设计，支持直播时移回看
- 🌐 **远程调用** - 支持 gRPC 接口，方便跨语言集成
- 🏷️ **流别名** - 支持动态设置流别名，灵活管理多路流，实现导播功能
- 🤖 **AI 能力** - 集成推理引擎，支持 ONNX 模型，支持自定义的前置处理，后置处理，以及画框
- 🪝 **WebHook** - 支持订阅流的生命周期事件，实现业务系统联动
- 🔒 **私有协议** - 支持自定义私有协议，满足特殊业务需求

- 🔄 **多协议支持**：RTMP、RTSP、HTTP-FLV、WS-FLV、HLS、WebRTC、GB28181、ONVIF、SRT

<p align="right">(<a href="#readme-top">返回顶部</a>)</p>

## 快速开始

### 环境要求

- Go 1.23 或更高版本
- 了解基本的流媒体协议

### 运行默认配置

```bash
cd example/default
go run -tags sqlite main.go
```
### UI 界面

将 admin.zip （不要解压）放在和配置文件相同目录下。

然后访问 http://localhost:8080 即可。


<p align="right">(<a href="#readme-top">返回顶部</a>)</p>

## 使用示例

更多示例请查看 [example](./example/READEME_CN.md) 文档。

<p align="right">(<a href="#readme-top">返回顶部</a>)</p>

## 构建选项

可以使用以下构建标签来自定义构建：

| 构建标签 | 描述 |
|----------|------|
| disable_rm | 禁用内存池 |
| sqlite | 启用 SQLite 存储 |
| sqliteCGO | 启用 SQLite CGO 版本 |
| mysql | 启用 MySQL 存储 |
| postgres | 启用 PostgreSQL 存储 |
| duckdb | 启用 DuckDB 存储 |
| taskpanic | 抛出 panic（用于测试） |

<p align="right">(<a href="#readme-top">返回顶部</a>)</p>

## 监控系统

Monibuca 内置支持 Prometheus 监控。在 Prometheus 配置中添加：

```yaml
scrape_configs:
  - job_name: "monibuca"
    metrics_path: "/api/metrics"
    static_configs:
      - targets: ["localhost:8080"]
```

<p align="right">(<a href="#readme-top">返回顶部</a>)</p>

## 插件开发

Monibuca 支持通过插件扩展功能。查看[插件开发指南](./plugin/README_CN.md)了解详情。

<p align="right">(<a href="#readme-top">返回顶部</a>)</p>

## 贡献指南

我们非常欢迎社区贡献，您的参与将使开源社区变得更加精彩！

1. Fork 本项目
2. 创建您的特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交您的修改 (`git commit -m '添加一些特性'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 发起 Pull Request

<p align="right">(<a href="#readme-top">返回顶部</a>)</p>

## 许可证

本项目采用 AGPL 许可证，详见 [LICENSE](./LICENSE) 文件。

<p align="right">(<a href="#readme-top">返回顶部</a>)</p>

## 联系方式

- 微信公众号：不卡科技
- QQ群：751639168
- QQ频道：p0qq0crz08

<p align="right">(<a href="#readme-top">返回顶部</a>)</p>

<!-- MARKDOWN LINKS & IMAGES -->
[contributors-shield]: https://img.shields.io/github/contributors/langhuihui/monibuca.svg?style=for-the-badge
[contributors-url]: https://github.com/langhuihui/monibuca/graphs/contributors
[forks-shield]: https://img.shields.io/github/forks/langhuihui/monibuca.svg?style=for-the-badge
[forks-url]: https://github.com/langhuihui/monibuca/network/members
[stars-shield]: https://img.shields.io/github/stars/langhuihui/monibuca.svg?style=for-the-badge
[stars-url]: https://github.com/langhuihui/monibuca/stargazers
[issues-shield]: https://img.shields.io/github/issues/langhuihui/monibuca.svg?style=for-the-badge
[issues-url]: https://github.com/langhuihui/monibuca/issues
[license-shield]: https://img.shields.io/github/license/langhuihui/monibuca.svg?style=for-the-badge
[license-url]: https://github.com/langhuihui/monibuca/blob/v5/LICENSE
