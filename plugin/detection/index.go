package plugin_detection

import (
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
		SnapImgFormat string `json:"snapImgFormat" default:"jpg" desc:"截图文件格式(jpg/png)"`
		SnapOriginal  bool   `json:"snapOriginal" default:"false" desc:"是否保存原始图片"`
		// 对象
		AlgorithmAPI detection.AlgorithmAPI `json:"algorithmApi" default:"{}" desc:"算法API配置"`
		Oss          detection.Oss          `json:"oss" default:"{}" desc:"对象存储公共配置"`
		Bbox         detection.Bbox         `json:"bbox" default:"" desc:"检测框配置"`
		MQTT         detection.MQTTConfig   `json:"mqtt" default:"{}" desc:"MQTT推送配置"`
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
