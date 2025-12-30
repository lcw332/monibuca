package detection

import (
	"time"

	"github.com/google/uuid"
	"m7s.live/v5/pkg/config"
)

// ==================== 配置模型 ====================

// SnapConfig 截图检测配置
type SnapConfig struct {
	SnapImgFormat  string        `json:"snapImgFormat" default:"jpg" desc:"截图文件格式(jpg/png)"`
	SnapMode       int           `json:"snapMode" default:"0" desc:"截图模式: 0-时间间隔，1-关键帧间隔 2-HTTP请求模式（手动触发）"`
	TimeInterval   time.Duration `json:"timeInterval" default:"1s" desc:"截图时间间隔, 仅在SnapMode为0时生效"`
	IFrameInterval int           `json:"iframeInterval" default:"1" desc:"间隔多少帧截图, 仅在SnapMode为1时生效"`
	SnapOriginal   bool          `json:"snapOriginal" default:"false" desc:"是否保存原始图片"`
	SavePath       string        `json:"savePath" desc:"截图保存路径"`
	FontPath       string        `json:"fontPath" default:"" desc:"检测框字体文件路径"`
	AlgorithmId    []uint8       `default:"[]" desc:"算法ID"`
	ConfThreshold  []float32     `default:"[]" desc:"置信度配置，与算法ID一一对应"`
	FrameCheck     []int         `default:"[]" desc:"连续帧检测次数，与算法ID一一对应，如设置为3则目标需连续出现3次才上报"`
	IoUThreshold   []float32     `default:"[0.5]" desc:"IoU阈值，与算法ID一一对应，用于判断是否为同一目标，范围0.1-0.9"`
	AlgorithmAPI   *AlgorithmAPI `json:"algorithmAPI" default:"{}" desc:"算法API配置"`
	Bbox           *Bbox         `json:"bbox" default:"{}" desc:"检测框配置"`
	MQTT           *MQTTConfig   `json:"mqtt" default:"{}" desc:"MQTT配置"`
}

// Bbox 检测框配置
type Bbox struct {
	FontPath  string `json:"fontPath" default:"" desc:"水印字体文件路径"`
	FontColor string `json:"fontColor" default:"red" desc:"截图文字颜色，支持rgba格式"`
	FontSize  uint8  `json:"fontSize" default:"12" desc:"截图字体大小"`
}

// AlgorithmAPI 算法API配置
type AlgorithmAPI struct {
	Enable        bool              `json:"enable" default:"false" desc:"是否启用算法分析"`
	Url           string            `json:"url" default:"" desc:"算法服务地址"`
	Method        string            `json:"method" default:"POST" desc:"算法服务请求方式"`
	Headers       map[string]string `json:"headers" default:"{}" desc:"自定义请求头"`
	Timeout       time.Duration     `json:"timeout" default:"30s" desc:"请求超时时间"`
	ApiKey        string            `json:"apiKey" default:"" desc:"认证密钥"`
	RetryCount    int               `json:"retryCount" default:"3" desc:"失败重试次数"`
	RetryInterval time.Duration     `json:"retryInterval" default:"5s" desc:"重试间隔"`
	AsyncMode     bool              `json:"asyncMode" default:"true" desc:"是否异步调用"`
}

// MQTTConfig MQTT配置
type MQTTConfig struct {
	Enable         bool     `json:"enable" default:"false" desc:"是否启用MQTT"`
	Broker         string   `json:"broker" desc:"MQTT Broker地址"`
	ClientID       string   `json:"clientID" desc:"客户端ID"`
	Username       string   `json:"username" desc:"用户名"`
	Password       string   `json:"password" desc:"密码"`
	TopicPrefix    string   `json:"topicPrefix" default:"" desc:"主题前缀"`
	Sub            []string `json:"sub" default:"[]" desc:"订阅主题模板"`
	Pub            []string `json:"pub" default:"[]" desc:"发布主题模板"`
	Qos            []int    `json:"qos" default:"[0]" desc:"QoS等级"`
	Retained       bool     `json:"retained" default:"false" desc:"是否保留消息"`
	KeepAlive      int      `json:"keepAlive" default:"30" desc:"保活时间(秒)"`
	ConnectRetry   bool     `json:"connectRetry" default:"true" desc:"是否重连"`
	ConnectRetryMs int      `json:"connectRetryMs" default:"5000" desc:"重连间隔(毫秒)"`
	Format         string   `json:"format" default:"json" desc:"消息格式(json/pb)"`
}

// Oss 对象存储配置
type Oss struct {
	Enable          bool          `default:"false" desc:"是否启用Oss配置"`
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

// ==================== 检测结果模型 ====================

// DetectionRequest 检测请求
type DetectionRequest struct {
	AlgorithmID   uint8   `json:"algorithm_id"`
	Image         string  `json:"image"`
	ConfThreshold float32 `json:"conf_threshold,omitempty"`
}

// DetectionResult 单个检测结果
type DetectionResult struct {
	ClassID          int       `json:"class_id"`
	ClassName        string    `json:"class_name"`
	ClassNameCn      string    `json:"class_name_cn"`
	Confidence       float64   `json:"confidence"`
	BBox             []float64 `json:"bbox"`
	PlateNumber      string    `json:"plate_number,omitempty"`      // 车牌号码
	PlateType        string    `json:"plate_type,omitempty"`        // 车牌类型
	PlateConfidence  float64   `json:"plate_confidence,omitempty"`  // 车牌置信度
	ConsecutiveCount int       `json:"consecutive_count,omitempty"` // 连续帧检测次数
	IsSameObj        bool      `json:"is_same_obj,omitempty"`       // 是否为连续检测达标目标
}

// DetectionResponse 检测响应
type DetectionResponse struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
	Data      struct {
		AlgorithmID   int               `json:"algorithm_id"`
		AlgorithmName string            `json:"algorithm_name"`
		Detections    []DetectionResult `json:"detections"`
		TotalCount    int               `json:"total_count"`
		DetectTime    float64           `json:"detect_time"`
	} `json:"data"`
}

// BatchDetectionRequest 批量检测请求
type BatchDetectionRequest struct {
	Requests []DetectionRequest `json:"requests"`
}

// BatchDetectionResponse 批量检测响应
type BatchDetectionResponse struct {
	Code      int                 `json:"code"`
	Message   string              `json:"message"`
	RequestID string              `json:"request_id,omitempty"`
	Data      []DetectionResponse `json:"data"`
}

// ==================== 回调模型 ====================

// CallbackArgs 回调参数
type CallbackArgs struct {
	AccessUrl         string            `json:"access_url,omitempty"`
	ObjectKey         string            `json:"object_key,omitempty"`
	ObjectRaw         string            `json:"object_raw,omitempty"`
	ObjectArtifacts   string            `json:"object_artifacts,omitempty"`
	AlgorithmID       int               `json:"algorithm_id"`
	AlgorithmName     string            `json:"algorithm_name"`
	Detections        []DetectionResult `json:"detections"`
	TotalCount        int               `json:"total_count"`
	DetectTime        float64           `json:"detect_time"`
	HasFrameCheck     bool              `json:"has_frame_check,omitempty"`      // 是否启用了连续帧检测
	FrameCheckCount   int               `json:"frame_check_count,omitempty"`    // 连续帧检测阈值
	SameObjTotalCount int               `json:"same_obj_total_count,omitempty"` // 连续检测达标的目标数量
}

// CallbackDetection 回调数据结构
type CallbackDetection struct {
	Event      string       `json:"event"`
	StreamPath string       `json:"streamPath"`
	Args       CallbackArgs `json:"args"`
	PublishId  uuid.UUID    `json:"publishId"`
	RemoteAddr string       `json:"remoteAddr"`
	Type       string       `json:"type"`
	PluginName string       `json:"pluginName"`
	Timestamp  int          `json:"timestamp"`
}

// ==================== 数据库模型 ====================

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

// TableName 指定表名
func (DetectionConfig) TableName() string {
	return "detection_configs"
}

// ==================== Webhook模型 ====================

// WebhookConfig Webhook配置
type WebhookConfig struct {
	OnDetectionInit   *config.Webhook `json:"onDetectionInit" desc:"检测初始化时触发的webhook"`
	OnDetectionResult *config.Webhook `json:"onDetectionResult" desc:"检测结果产生时触发的webhook"`
	OnDetectionError  *config.Webhook `json:"onDetectionError" desc:"检测出错时触发的webhook"`
	OnDetectionClose  *config.Webhook `json:"onDetectionClose" desc:"检测关闭时触发的webhook"`
}

// Hook常量
const (
	HookOnDetectionInit   config.HookType = "on_detection_init"
	HookOnDetectionResult config.HookType = "on_detection_result"
	HookOnDetectionError  config.HookType = "on_detection_error"
	HookOnDetectionClose  config.HookType = "on_detection_close"
)

// ==================== 图像模型 ====================

// ImgInfo 图片信息
type ImgInfo struct {
	Size   int64
	Width  int
	Height int
}

// BBox 边界框 (x, y, w, h)
type BBox struct {
	X, Y, W, H float64
}

// ==================== 辅助方法 ====================

// IsSuccess 判断响应是否成功
func (resp *DetectionResponse) IsSuccess() bool {
	return resp.Code == 200
}

// hasDetections 判断响应中是否有检测结果
func (resp *DetectionResponse) hasDetections() bool {
	return len(resp.Data.Detections) > 0
}

// FloatsToBBox float 数组转 BBox
func FloatsToBBox(values []float64) BBox {
	if len(values) < 4 {
		return BBox{}
	}
	return BBox{
		X: values[0],
		Y: values[1],
		W: values[2],
		H: values[3],
	}
}
