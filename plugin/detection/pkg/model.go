package detection

import (
	config "m7s.live/v5/pkg/config"
)

// DetectionConfig 检测配置表（单表实现）
type DetectionConfig struct {
	ID            uint   `gorm:"primaryKey" json:"algId"`
	Name          string `gorm:"not null; uniqueIndex" json:"name"`  // 配置名称
	StreamURL     string `gorm:"not null" json:"stream_url"`         // 流地址
	AlgorithmConf string `gorm:"not null" json:"algorithm_conf"`     // 算法ID列表，用逗号分隔存储多个ID
	Configs       string `json:"configs"`                            // 算法配置参数(JSON数组格式)，与AlgorithmIDs顺序对应
	Status        int    `gorm:"default:1" json:"status"`            // 状态: 0-禁用, 1-启用
	Description   string `json:"description"`                        // 描述信息
	CreatedAt     int64  `json:"created_at"`                         // 创建时间
	UpdatedAt     int64  `json:"updated_at"`                         // 更新时间
	AlertEnabled  bool   `gorm:"default:false" json:"alert_enabled"` // 是否启用告警
	AlertWebhook  string `json:"alert_webhook"`                      // 告警回调地址
}

// WebhookConfig Webhook配置
type WebhookConfig struct {
	OnDetectionInit   *config.Webhook `json:"onDetectionInit" desc:"检测初始化时触发的webhook"`
	OnDetectionResult *config.Webhook `json:"onDetectionResult" desc:"检测结果产生时触发的webhook"`
	OnDetectionError  *config.Webhook `json:"onDetectionError" desc:"检测出错时触发的webhook"`
	OnDetectionClose  *config.Webhook `json:"onDetectionClose" desc:"检测关闭时触发的webhook"`
}

// TableName 指定表名
func (DetectionConfig) TableName() string {
	return "detection_configs"
}

const (
	HookOnDetectionInit   config.HookType = "on_detection_init"
	HookOnDetectionResult config.HookType = "on_detection_result"
	HookOnDetectionError  config.HookType = "on_detection_error"
	HookOnDetectionClose  config.HookType = "on_detection_close"
)
