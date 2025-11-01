package pkg

// DetectionConfig 检测配置表（单表实现）
type DetectionConfig struct {
	ID            uint   `gorm:"primaryKey" json:"id"`
	Name          string `gorm:"not null; uniqueIndex" json:"name"` // 配置名称
	StreamURL     string `gorm:"not null" json:"stream_url"`        // 流地址
	AlgorithmConf string `gorm:"not null" json:"algorithm_conf"`    // 算法ID列表，用逗号分隔存储多个ID
	Configs       string `json:"configs"`                           // 算法配置参数(JSON数组格式)，与AlgorithmIDs顺序对应
	Status        int    `gorm:"default:1" json:"status"`           // 状态: 0-禁用, 1-启用
	Description   string `json:"description"`                       // 描述信息
	CreatedAt     int64  `json:"created_at"`                        // 创建时间
	UpdatedAt     int64  `json:"updated_at"`                        // 更新时间
}

// TableName 指定表名
func (DetectionConfig) TableName() string {
	return "detection_configs"
}
