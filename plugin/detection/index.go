package plugin_detection

import (
	"time"

	"m7s.live/v5"
	detection "m7s.live/v5/plugin/detection/pkg"
)

var (
	_ = m7s.InstallPlugin[DetectionPlugin](m7s.PluginMeta{
		Name:           "Detection",
		Version:        "v0.0.1",
		NewTransformer: detection.NewTransform,
	})
)

type (
	// DetectionPlugin 图像插件
	DetectionPlugin struct {
		m7s.Plugin
		SnapImgFormat  string        `json:"snapImgFormat" default:"jpg" desc:"截图文件格式(jpg/png)"`
		SnapMode       int           `json:"snapMode" default:"0" desc:"截图模式: 0-时间间隔，1-关键帧间隔 2-HTTP请求模式（手动触发）"`
		TimeInterval   time.Duration `json:"timeInterval" default:"1s" desc:"截图时间间隔, 仅在SnapMode为0时生效"`
		IFrameInterval int           `json:"iframeInterval" default:"1" desc:"间隔多少帧截图, 仅在SnapMode为1时生效"`
		AlgorithmId    []uint8       `default:"[]" desc:"算法ID"`
		ConfThreshold  []float32     `default:"[]" desc:"全局置信度配置，与算法ID一一对应"`
		// 对象
		AlgorithmAPI AlgorithmAPI `json:"algorithmApi" default:"{}" desc:"算法API配置"`
		Oss          Oss          `json:"oss" default:"{}" desc:"对象存储公共配置"`
		Bbox         Bbox         `json:"bbox" default:"" desc:"检测框配置"`
	}
	Bbox struct {
		SnapOriginal bool   `json:"snapOriginal" default:"true" desc:"是否保存原始图片"`
		FontPath     string `json:"fontPath" default:"" desc:"水印字体文件路径"`
		FontColor    string `json:"fontColor" default:"red" desc:"截图文字颜色，支持rgba格式"`
		FontSize     uint8  `json:"fontSize" default:"12" desc:"截图字体大小"`
	}
	Oss struct {
		Enable          bool          `default:"false" desc:"是否启用Oss配置" `
		Endpoint        string        `desc:"S3服务端点"`
		Region          string        `desc:"AWS区域" default:"us-east-1"`
		AccessKeyID     string        `desc:"S3访问密钥ID"`
		SecretAccessKey string        `desc:"S3秘密访问密钥"`
		Bucket          string        `desc:"S3存储桶名称"`
		PathPrefix      string        `desc:"文件路径前缀"`
		ForcePathStyle  bool          `desc:"强制路径样式（MinIO需要）"`
		UseSSL          bool          `desc:"是否使用SSL" default:"false"`
		Timeout         time.Duration `desc:"上传超时时间" default:"30s"`
	}

	AlgorithmAPI struct {
		Enable        bool              `json:"enable" default:"false" desc:"是否启用算法分析"`
		Url           string            `json:"url" default:"" desc:"算法服务地址"`
		Method        string            `json:"method" default:"POST" desc:"算法服务请求方式"`
		Headers       map[string]string `json:"headers" default:"{}" desc:"自定义请求头"`
		Timeout       time.Duration     `json:"timeout" default:"30s" desc:"请求超时时间"`
		ApiKey        string            `json:"apiKey" default:"" desc:"认证密钥"`
		RetryCount    int               `json:"retryCount" default:"3" desc:"失败重试次数"`
		RetryInterval time.Duration     `json:"retryInterval" default:"5s" desc:"重试间隔"`
		AsyncMode     bool              `json:"asyncMode" default:"true" desc:"是否异步调用"`
		CallbackURL   string            `json:"callbackURL" default:"" desc:"回调地址"`
	}
)

// Start 插件初始化
func (p *DetectionPlugin) Start() (err error) {
	// 数据库初始化
	if p.DB != nil {
		err = p.DB.AutoMigrate(&detection.DetectionConfig{})
		if err != nil {
			return err
		}
	}
	return
}
