package plugin_detection

import (
	"time"

	"m7s.live/v5"
	"m7s.live/v5/pkg/storage"
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
		Algorithms   string    `default:"1~24" desc:"全局算法配置"`
		Threshold    float64   `default:"0.5" desc:"全局阈值"`
		Oss          Oss       `default:"{}" desc:"对象存储公共配置"`
		AlgorithmMap Algorithm `default:"{}" desc:"算法映射配置"`
		ossPlugin    storage.Storage
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
		UseSSL          bool          `desc:"是否使用SSL" default:"true"`
		Timeout         time.Duration `desc:"上传超时时间" default:"30s"`
	}

	Algorithm struct {
		DefaultTimeout    time.Duration `default:"30s" desc:"默认算法超时时间"`
		DefaultRetryCount int           `default:"3" desc:"默认重试次数"`
		MaxImageSize      int           `default:"2097152" desc:"最大图片大小(字节)"`
		EnableBatch       bool          `default:"false" desc:"是否启用批量处理"`
		BatchSize         int           `default:"5" desc:"批量处理大小"`
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
	// 创建对象存储
	if p.Oss.Enable {
		s3Config := map[string]interface{}{
			"endpoint":        p.Oss.Endpoint,
			"region":          p.Oss.Region,
			"accessKeyID":     p.Oss.AccessKeyID,
			"secretAccessKey": p.Oss.SecretAccessKey,
			"bucket":          p.Oss.Bucket,
			"pathPrefix":      p.Oss.PathPrefix,
			"forcePathStyle":  p.Oss.ForcePathStyle,
			"useSSL":          p.Oss.UseSSL,
			"timeout":         p.Oss.Timeout,
		}
		p.ossPlugin, err = storage.CreateStorage("s3", s3Config)
		if err != nil {
			return err
		}
	}

	return
}
