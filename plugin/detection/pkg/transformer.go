package detection

import (
	"time"

	task "github.com/langhuihui/gotask"
	"m7s.live/v5"
	"m7s.live/v5/pkg/config"
)

type SnapMode int

const (
	SnapModeTimeInterval SnapMode = iota
	SnapModeIFrameInterval
	SnapModeManual
)

type (
	SnapConfig struct {
		SnapshotFormat string        `desc:"截图文件格式(jpg/png)"`
		SnapMode       SnapMode      `desc:"截图模式: 0-时间间隔，1-关键帧间隔 2-HTTP请求模式（手动触发）"`
		TimeInterval   time.Duration `desc:"截图时间间隔, 仅在SnapMode为0时生效"`
		IFrameInterval int           `desc:"间隔多少帧截图, 仅在SnapMode为1时生效"`
		SavePath       string        `desc:"截图保存路径"`
		AlgorithmAPI   AlgorithmAPI  `default:"{}" desc:"算法API配置"`
		SnapCompress   SnapCompress  `default:"{}" desc:"图片压缩配置"`
		Watermark      Watermark     `default:"{}" desc:"水印配置"`
		MaxSnapshots   int           `default:"100" desc:"最大保存截图数量"`
	}

	AlgorithmAPI struct {
		Enable        bool              `default:"false" desc:"是否启用算法分析"`
		Url           string            `default:"" desc:"算法服务地址"`
		Method        string            `default:"POST" desc:"算法服务请求方式"`
		Headers       map[string]string `default:"{}" desc:"自定义请求头"`
		Timeout       time.Duration     `default:"30s" desc:"请求超时时间"`
		ApiKey        string            `default:"" desc:"认证密钥"`
		RetryCount    int               `default:"3" desc:"失败重试次数"`
		RetryInterval time.Duration     `default:"5s" desc:"重试间隔"`
		AsyncMode     bool              `default:"true" desc:"是否异步调用"`
		CallbackURL   string            `default:"" desc:"回调地址"`
	}

	SnapCompress struct {
		Enable       bool    `default:"false" desc:"是否开启压缩"`
		Quality      float64 `default:"1" desc:"图片压缩质量"`
		MaxSize      int     `default:"2097152" desc:"图片文件大小, 单位字节"`
		Mode         string  `default:"none" desc:"图片裁剪模式: none,letterbox、cover、contain等"`
		TargetWidth  int     `default:"0" desc:"目标宽度(0表示不调整)"`
		TargetHeight int     `default:"0" desc:"目标高度(0表示不调整)"`
	}

	Watermark struct {
		Enable      bool    `default:"false" desc:"是否开启水印"`
		Text        string  `default:"" desc:"水印文字内容"`
		FontPath    string  `default:"" desc:"水印字体文件路径"`
		FontColor   string  `default:"rgba(255,165,0,1)" desc:"水印字体颜色，支持rgba格式"`
		FontSize    float64 `default:"36" desc:"水印字体大小"`
		FontSpacing float64 `default:"2" desc:"水印字体间距"`
		OffsetX     int     `default:"0" desc:"水印位置X"`
		OffsetY     int     `default:"0" desc:"水印位置Y"`
		Opacity     float64 `default:"1.0" desc:"水印透明度(0-1)"`
	}
)

type Transformer struct {
	task.Job
	TransformJob m7s.TransformJob
}

type SnapTask struct {
	job    *m7s.TransformJob
	config SnapConfig
}

type AlgTask struct {
	job *m7s.TransformJob
}

func (t *Transformer) GetTransformJob() *m7s.TransformJob {
	return &t.TransformJob
}

func NewTransform() m7s.ITransformer {
	return &Transformer{}
}

// Start #TaskStarter 启动一个定时任务
func (t *Transformer) Start() error {
	// 为每个输出配置创建一个截图任务
	for _, output := range t.TransformJob.Config.Output {
		//var task task.ITask
		var snapConfig SnapConfig
		t.Logger.Info("output.Conf", output.Conf)
		if output.Conf != nil {
			switch v := output.Conf.(type) {
			case SnapConfig:
				snapConfig = v
			case map[string]any:
				config.Parse(&snapConfig, v)
			}
		}
	}
	return nil
}

// IFrameSnapTask #ITask 帧间隔截图任务
type IFrameSnapTask struct {
	task.Task
	SnapTask
	subscriber *m7s.Subscriber
}

// Start #TaskStarter 启动一个帧间隔截图任务
func (t *IFrameSnapTask) Start() (err error) {
	subConfig := t.job.Plugin.GetCommonConf().Subscribe
	subConfig.SubType = m7s.SubscribeTypeTransform
	subConfig.IFrameOnly = true
	t.subscriber, err = t.job.Plugin.SubscribeWithConfig(t, t.job.StreamPath, subConfig)
	return
}

// TimeSnapTask #ITask 定时截图任务
type TimeSnapTask struct {
	task.TickTask
	SnapTask
}

// GetTickInterval #TaskTicker 获取定时截图间隔
func (t *TimeSnapTask) GetTickInterval() time.Duration {
	return t.config.TimeInterval
}

// Tick #TaskTicker 定时截图任务执行逻辑
func (t *TimeSnapTask) Tick(any) {
	// 获取视频帧
	//annexb, err := GetVideoFrame(t.job.OriginPublisher, t.job.Plugin.Server)
	//if err != nil {
	//	t.Error("get video frame failed", "error", err.Error())
	//	return
	//}

	//if err := t.saveSnap(annexb, SnapModeTimeInterval); err != nil {
	//	t.Error("save snapshot failed", "error", err.Error())
	//}
}
