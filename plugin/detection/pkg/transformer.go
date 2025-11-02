package pkg

import (
	"time"

	task "github.com/langhuihui/gotask"
	"m7s.live/v5"
)

type (
	SnapConfig struct {
		SnapshotFormat string        `desc:"截图文件格式(jpg/png)"`
		SnapMode       string        `desc:"截图模式: 0-时间间隔，1-关键帧间隔 2-HTTP请求模式（手动触发）"`
		TimeInterval   time.Duration `desc:"截图时间间隔, 仅在SnapMode为0时生效"`
		IFrameInterval int           `desc:"间隔多少帧截图, 仅在SnapMode为1时生效"`
		SavePath       string        `desc:"截图保存路径"`
		AlgorithmAPI   AlgorithmAPI  `default:"{}" desc:"算法API配置"`
		SnapCompress   SnapCompress  `default:"{}" desc:"图片压缩配置"`
		Watermark      Watermark     `default:"{}" desc:"水印配置"`
	}

	AlgorithmAPI struct {
		Enable        bool          `default:"false" desc:"是否启用算法分析"`
		Url           string        `default:"" desc:"算法服务地址"`
		Method        string        `default:"POST" desc:"算法服务请求方式"`
		Timeout       time.Duration `default:"30s" desc:"请求超时时间"`
		ApiKey        string        `default:"" desc:"认证密钥"`
		RetryCount    int           `default:"3" desc:"失败重试次数"`
		RetryInterval time.Duration `default:"5s" desc:"重试间隔"`
		AsyncMode     bool          `default:"true" desc:"是否异步调用"`
		CallbackURL   string        `default:"" desc:"回调地址"`
	}

	SnapCompress struct {
		Enable       bool    `default:"false" desc:"是否开启压缩"`
		Quality      float64 `default:"1" desc:"图片压缩质量"`
		MaxSize      int     `default:"2097152" desc:"图片文件大小, 单位字节"`
		Mode         string  `default:"none" desc:"图片裁剪模式: none,letterbox、cover、contain等"`
		ResizeWidth  int     `default:"1920" desc:"图片裁剪宽度"`
		ResizeHeight int     `default:"1080" desc:"图片裁剪高度"`
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
	}
)

type Transformer struct {
	task.Job
	TransformJob m7s.TransformJob
}

type SnapTask struct {
	job *m7s.TransformJob
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

// Tick #IChannelTask 定时任务执行逻辑
func (t *Transformer) Tick(a any) {
	//annexb, err := GetVideoFrame(t.job.OriginPublisher, t.job.Plugin.Server)

	panic("implement me")
}

func (t *Transformer) GetTickInterval() time.Duration {
	//TODO implement me
	// 定时的interval
	panic("implement me")
}

func (t *Transformer) GetTicker() *time.Ticker {
	//TODO implement me
	panic("implement me")
}

// Start #TaskStarter 启动一个定时任务
func (t *Transformer) Start() error {
	// 为每个输出配置创建一个截图任务
	for _, output := range t.TransformJob.Config.Output {
		t.Logger.Info("output.StreamPath", output.StreamPath)
	}
	return nil

}
