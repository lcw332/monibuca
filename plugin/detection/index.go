package detection

import (
	"context"
	m7s "m7s.live/v5"
	storage "m7s.live/v5/pkg/storage"
	detection "m7s.live/v5/plugin/detection/pkg"
)

var _ = m7s.InstallPlugin[DetectionPlugin](m7s.PluginMeta{
	NewTransformer: detection.NewTransform,
})

// DetectionPlugin 图像插件
type DetectionPlugin struct {
	m7s.Plugin
	Algorithms string  `json:"algorithms" default:"1-24" desc:"全局算法配置"`
	Threshold  float64 `json:"threshold" default:"0.5" desc:"全局阈值"`
	Oss        struct {
		Enable          bool   `json:"enable" desc:"算法识别图片是否开启 OSS 保存"`
		Endpoint        string `json:"endpoint" desc:"OSS服务端点"`
		AccessKeyId     string `json:"accessKeyId" desc:"访问密钥ID"`
		AccessSecretKey string `json:"accessSecretKey" desc:"访问密钥Secret"`
		Bucket          string `json:"bucket" desc:"存储桶名称"`
		PathPrefix      string `json:"pathPrefix" desc:"文件路径前缀"`
		ForcePathStyle  bool   `json:"forcePathStyle" desc:"强制路径样式（MinIO需要）"`
		UseSSL          bool   `json:"useSSL" desc:"是否使用SSL" default:"false"`
		Timeout         int    `json:"timeout" desc:"上传超时时间（秒）" default:"30"`
	} `json:"ossConfig" desc:"OSS存储配置"`
}

// Start 插件初始化
func (p *DetectionPlugin) Start() (err error) {
	// 数据库初始化
	if p.DB != nil {
		err = p.DB.AutoMigrate(&detection.DetectionConfig{})
	}

	// 创建对象存储
	if p.Oss.Enable {
		s, err := storage.CreateStorage("s3", p.Oss)
		if err != nil {
			return err
		}
		_, err = s.CreateFile(context.Background(), "test.txt")
		// TODO: 存储s3Storage供后续使用
	}

	return
}
