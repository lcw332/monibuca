package detection

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	task "github.com/langhuihui/gotask"
	"m7s.live/v5"
	"m7s.live/v5/pkg"
	"m7s.live/v5/pkg/config"
	"m7s.live/v5/pkg/format"
	"m7s.live/v5/pkg/storage"
)

type SnapMode int

const (
	SnapModeTimeInterval SnapMode = iota
	SnapModeIFrameInterval
	SnapModeManual
)

type (
	SnapConfig struct {
		SnapImgFormat  string        `json:"snapImgFormat" default:"jpg" desc:"截图文件格式(jpg/png)"`
		SnapMode       int           `json:"snapMode" default:"0" desc:"截图模式: 0-时间间隔，1-关键帧间隔 2-HTTP请求模式（手动触发）"`
		TimeInterval   time.Duration `json:"timeInterval" default:"1s" desc:"截图时间间隔, 仅在SnapMode为0时生效"`
		IFrameInterval int           `json:"iframeInterval" default:"1" desc:"间隔多少帧截图, 仅在SnapMode为1时生效"`
		SavePath       string        `json:"savePath" desc:"截图保存路径"`
		FontPath       string        `json:"fontPath" default:"" desc:"检测框字体文件路径"`
		AlgorithmId    []uint8       `default:"1:26" desc:"算法ID"`
		ConfThreshold  []float32     `default:"0.5" desc:"置信度配置，与算法ID一一对应"`
		AlgorithmAPI   *AlgorithmAPI `json:"algorithmAPI" default:"{}" desc:"算法API配置"`
		Bbox           *Bbox         `json:"bbox" default:"{}" desc:"检测框配置"`
	}

	Bbox struct {
		SnapOriginal bool   `json:"snapOriginal" default:"true" desc:"是否保存原始图片"`
		FontPath     string `json:"fontPath" default:"" desc:"水印字体文件路径"`
		FontColor    string `json:"fontColor" default:"red" desc:"截图文字颜色，支持rgba格式"`
		FontSize     uint8  `json:"fontSize" default:"12" desc:"截图字体大小"`
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

type Transformer struct {
	task.Job
	TransformJob m7s.TransformJob
}

type SnapTask struct {
	job       *m7s.TransformJob
	ossPlugin storage.Storage
	config    SnapConfig
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
func (t *Transformer) Start() (err error) {
	plugin := t.TransformJob.Plugin

	ossConfig := plugin.Config.Get("oss")
	var ossPlugin storage.Storage
	if ossConfig != nil {
		ossPlugin, err = storage.CreateStorage("s3", ossConfig.File)
		if err != nil {
			return err
		}
	}

	apiConfig := plugin.Config.Get("algorithmApi")
	var globalAlgApi *AlgorithmAPI
	if apiConfig != nil {
		globalAlgApi = &AlgorithmAPI{}
		switch v := apiConfig.File.(type) {
		case *AlgorithmAPI:
			globalAlgApi = v
		case map[string]any:
			config.Parse(globalAlgApi, v)
		}
	}

	bboxConfig := plugin.Config.Get("bbox")
	var globalBbox *Bbox
	if bboxConfig != nil {
		globalBbox = &Bbox{}
		switch v := bboxConfig.File.(type) {
		case *Bbox:
			globalBbox = v
		case map[string]any:
			config.Parse(globalBbox, v)
		}
	}

	snapImgFormat := plugin.Config.Get("snapImgFormat")
	var globalSnapImgFormat string
	if snapImgFormat != nil {
		globalSnapImgFormat = ""
		switch v := snapImgFormat.File.(type) {
		case string:
			globalSnapImgFormat = v
		}
	}

	// 为每个输出配置创建一个截图任务
	for _, output := range t.TransformJob.Config.Output {
		var task task.ITask
		var snapConfig SnapConfig

		if output.Conf != nil {
			switch v := output.Conf.(type) {
			case SnapConfig:
				snapConfig = v
			case map[string]any:
				config.Parse(&snapConfig, v)
			}
		}

		// 如果 snapConfig 的 algorithmApi没有配置则使用全局的 api 配置
		if snapConfig.AlgorithmAPI == nil && apiConfig != nil {
			snapConfig.AlgorithmAPI = globalAlgApi
		}

		// 如果 snapConfig 的 bbox 没有配置则使用全局的 bbox 配置
		if snapConfig.Bbox == nil && globalBbox != nil {
			snapConfig.Bbox = globalBbox
		}

		if snapConfig.SnapImgFormat == "" && globalSnapImgFormat != "" {
			snapConfig.SnapImgFormat = globalSnapImgFormat
		}

		switch snapConfig.SnapMode {
		case int(SnapModeTimeInterval):
			// 时间间隔模式截图逻辑
			timeTask := &TimeSnapTask{
				SnapTask: SnapTask{
					config:    snapConfig,
					job:       &t.TransformJob,
					ossPlugin: ossPlugin,
				},
			}
			task = timeTask
		case int(SnapModeIFrameInterval):
			// 关键帧间隔模式截图逻辑
			iframeTask := &IFrameSnapTask{
				SnapTask: SnapTask{
					config:    snapConfig,
					job:       &t.TransformJob,
					ossPlugin: ossPlugin,
				},
			}
			task = iframeTask
		case int(SnapModeManual):
			// 手动触发模式截图逻辑
		}
		if task != nil {
			t.AddTask(task)
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

// Go #TaskRunner 帧间隔截图任务执行逻辑
func (t *IFrameSnapTask) Go() (err error) {
	iframeCount := 0
	err = m7s.PlayBlock(t.subscriber, (func(audio *pkg.AVFrame) error)(nil), func(video *format.AnnexB) error {
		iframeCount++
		if iframeCount%t.config.IFrameInterval == 0 {
			// 原始分辨率
			t.Logger.Debug("video info", video.GetInfo())
			if err := t.saveSnap([]*format.AnnexB{video}, SnapModeIFrameInterval); err != nil {
				t.Error("save snapshot failed", "error", err.Error())
			}
		}
		return nil
	})
	if err != nil {
		t.Error("iframe interval snap error", "error", err.Error())
	}
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
	// 如果没有publisher，直接返回
	if nil == t.job.OriginPublisher {
		return
	}

	// 获取视频帧
	annexb, err := GetVideoFrame(t.job.OriginPublisher, t.job.Plugin.Server)
	if err != nil {
		t.Error("get video frame failed", "error", err.Error())
		return
	}

	if err := t.saveSnap(annexb, SnapModeTimeInterval); err != nil {
		t.Error("save snapshot failed", "error", err.Error())
	}
}

// saveSnap 保存截图，核心实现逻辑
func (t *SnapTask) saveSnap(annexb []*format.AnnexB, mode SnapMode) (err error) {
	// 生成文件名
	now := time.Now()
	// 处理视频帧
	var buf bytes.Buffer
	imgInfo, err := SnapFrameWithFFmpeg(annexb, &buf, t.config.SnapImgFormat)
	if err != nil {
		return fmt.Errorf("process with ffmpeg error: %w", err)
	}

	// 提前编码图片用于并发传输
	imageData := buf.Bytes()
	if len(imageData) == 0 {
		return errors.New("original image data is empty")
	}

	// 请求yolo算法接口，获取检测结果，然后hook到指定url
	if t.config.AlgorithmAPI.Enable && t.config.AlgorithmAPI.Url != "" {
		base64ImageData := SnapFrameToBase64WithFFmpeg(imageData)
		var wg sync.WaitGroup
		client := &http.Client{Timeout: 10 * time.Second} // 设置全局HTTP客户端带超时

		for index, algorithmID := range t.config.AlgorithmId {
			wg.Add(1)
			go func(id uint8, idx int, imgInfo ImgInfo) {
				defer wg.Done()

				detectClient := NewDetectionClient(
					t.config.AlgorithmAPI.Url,
					t.config.AlgorithmAPI.Method,
					t.config.AlgorithmAPI.ApiKey,
				)

				req := DetectionRequest{
					AlgorithmID: id,
					Image:       base64ImageData,
					ConfThreshold: func() float32 {
						if idx < len(t.config.ConfThreshold) {
							return t.config.ConfThreshold[idx]
						}
						return 0.6
					}(),
				}

				result, err := detectClient.Detect(req)
				if err != nil {
					t.job.Plugin.Error("detect error", "error", err.Error())
					return
				}
				if !result.IsSuccess() {
					t.job.Plugin.Error("algorithm api request failed or no detections found")
					return
				}

				if !result.hasDetections() {
					return
				}

				// 绘制边界框
				processedImage := imageData
				for _, detection := range result.Data.Detections {
					bbox := FloatsToBBox(detection.BBox)
					processedImage, err = DrawDetectionBBox(processedImage,
						&imgInfo, t.config.SnapImgFormat, bbox, detection.ClassName, detection.Confidence, t.config.Bbox.FontPath, t.config.Bbox.FontSize,
						t.config.Bbox.FontColor)
					if err != nil {
						t.job.Plugin.Error("draw bounding box error", "error", err.Error())
						continue
					}
				}

				if len(processedImage) == 0 {
					t.job.Plugin.Error("final image data is empty after drawing boxes")
					return
				}

				var accessUrl string
				if t.ossPlugin != nil {
					ossFilename := fmt.Sprintf("%s/alg_%d/%s.%s",
						strings.ReplaceAll(t.job.StreamPath, "/", "_"),
						id,
						now.Format("20060102150405.000"),
						t.config.SnapImgFormat)

					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()

					file, err := t.ossPlugin.CreateFile(ctx, ossFilename)
					if err != nil {
						t.job.Plugin.Error("create file error", "error", err.Error())
						return
					}
					defer file.Close()

					_, err = file.Write(processedImage)
					if err != nil {
						t.job.Plugin.Error("write file error", "error", err.Error())
						return
					}

					_, err = file.Seek(0, io.SeekStart)
					if err != nil {
						t.job.Plugin.Error("seek file error", "error", err.Error())
						return
					}

					err = file.Sync()
					if err != nil {
						t.job.Plugin.Error("sync file error", "error", err.Error())
						return
					}

					accessUrl, _ = t.ossPlugin.GetURL(ctx, ossFilename)
				}

				callbackEntity := result.ToCallback(t.job.StreamPath, "", t.job.Plugin.Meta.Name, 0)
				callbackEntity.Args.AccessUrl = accessUrl

				if t.config.AlgorithmAPI.CallbackURL != "" {
					jsonData, marshalErr := json.Marshal(callbackEntity)
					if marshalErr != nil {
						t.job.Plugin.Warn("marshal callback entity error", "error", marshalErr.Error())
						return
					}

					resp, postErr := client.Post(t.config.AlgorithmAPI.CallbackURL, "application/json", bytes.NewReader(jsonData))
					if postErr != nil {
						t.job.Plugin.Error("callback error", "error", postErr.Error())
						return
					}
					defer resp.Body.Close()

					body, _ := io.ReadAll(resp.Body)
					if resp.StatusCode >= 300 {
						t.job.Plugin.Warn("callback response status not ok", "status", resp.Status, "body", string(body))
					}
				}
			}(algorithmID, index, imgInfo)
		}
		wg.Wait()
	}

	return nil
}

// updateConfig 更新算法配置
func (t *SnapTask) updateConfig(steamPath string, config *SnapConfig) {
	t.config = *config
	// 关闭当前任务, 启动新的任务
	t.job.Dispose()
	// 根据 SnapMode 重新构造对应的任务类型
	var newTask task.ITask
	switch config.SnapMode {
	case int(SnapModeTimeInterval):
		newTask = &TimeSnapTask{
			SnapTask: SnapTask{
				config:    *config,
				job:       t.job,
				ossPlugin: t.ossPlugin,
			},
		}
	case int(SnapModeIFrameInterval):
		newTask = &IFrameSnapTask{
			SnapTask: SnapTask{
				config:    *config,
				job:       t.job,
				ossPlugin: t.ossPlugin,
			},
		}
	case int(SnapModeManual):
		// 手动模式暂不支持动态更新任务
		return
	}

	if newTask != nil {
		t.job.AddTask(newTask)
	}
}
