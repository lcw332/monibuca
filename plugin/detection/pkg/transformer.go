package detection

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
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
		AlgorithmId    []uint8       `default:"[]" desc:"算法ID"`
		ConfThreshold  []float32     `default:"[]" desc:"置信度配置，与算法ID一一对应"`
		AlgorithmAPI   *AlgorithmAPI `json:"algorithmAPI" default:"{}" desc:"算法API配置"`
		Bbox           *Bbox         `json:"bbox" default:"{}" desc:"检测框配置"`
		MQTT           *MQTTConfig   `json:"mqtt" default:"{}" desc:"MQTT配置"`
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

	// algorithmResult 用于存储算法检测结果
	algorithmResult struct {
		// 算法 ID
		algId uint8
		// 索引
		index int
		// 检测结果
		result *DetectionResponse
		// 原图数据
		rawImgData []byte
		// 错误
		err error
	}
)

type Transformer struct {
	task.Job
	TransformJob m7s.TransformJob
}

type SnapTask struct {
	job        *m7s.TransformJob
	ossPlugin  storage.Storage
	config     SnapConfig
	mqttClient MQTTClient
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
	ossEnable := ossConfig.Get("enable")
	var ossPlugin storage.Storage
	if ossConfig != nil && ossEnable.GetValue() == true && ossConfig.File != nil {
		ossPlugin, err = storage.CreateStorage("s3", ossConfig.File)
		if err != nil {
			plugin.Error("create s3 storage failed", "error", err.Error())
			return err
		}
	}

	apiConfig := plugin.Config.Get("algorithmApi")
	apiEnable := apiConfig.Get("enable")
	var globalAlgApi *AlgorithmAPI
	if apiConfig != nil && apiEnable.GetValue() == true && apiConfig.File != nil {
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
	if bboxConfig != nil && bboxConfig.File != nil {
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

	// 初始化MQTT客户端
	mqttConfig := plugin.Config.Get("mqtt")
	mqttEnable := mqttConfig.Get("enable")
	var globalMQTTConfig *MQTTConfig
	if mqttConfig != nil && mqttEnable.GetValue() == true && mqttConfig.File != nil {
		globalMQTTConfig = &MQTTConfig{}
		switch v := mqttConfig.File.(type) {
		case *MQTTConfig:
			globalMQTTConfig = v
		case map[string]any:
			config.Parse(globalMQTTConfig, v)
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

		// 如果 snapConfig 的 mqtt 没有配置则使用全局的 mqtt 配置
		if snapConfig.MQTT == nil && globalMQTTConfig != nil {
			snapConfig.MQTT = globalMQTTConfig
		}

		// 创建MQTT客户端
		var mqttClient MQTTClient
		if globalMQTTConfig != nil && globalMQTTConfig.Enable {
			mqttClient, _ = NewMQTTClient(globalMQTTConfig, plugin.Logger)
		}

		switch snapConfig.SnapMode {
		case int(SnapModeTimeInterval):
			// 时间间隔模式截图逻辑
			timeTask := &TimeSnapTask{
				SnapTask: SnapTask{
					config:     snapConfig,
					job:        &t.TransformJob,
					ossPlugin:  ossPlugin,
					mqttClient: mqttClient,
				},
			}
			task = timeTask
		case int(SnapModeIFrameInterval):
			// 关键帧间隔模式截图逻辑
			iframeTask := &IFrameSnapTask{
				SnapTask: SnapTask{
					config:     snapConfig,
					job:        &t.TransformJob,
					ossPlugin:  ossPlugin,
					mqttClient: mqttClient,
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
	imageData, imgInfo, err := t.processVideoFrame(annexb)
	if err != nil {
		return err
	}

	// 请求yolo算法接口，获取检测结果，然后hook到指定url
	if t.config.AlgorithmAPI.Enable && t.config.AlgorithmAPI.Url != "" {
		return t.processAlgorithmDetection(imageData, imgInfo, now)
	}

	return nil
}

// processVideoFrame 处理视频帧并生成图像数据
func (t *SnapTask) processVideoFrame(annexb []*format.AnnexB) ([]byte, ImgInfo, error) {
	var buf bytes.Buffer
	imgInfo, err := SnapFrameWithFFmpeg(annexb, &buf, t.config.SnapImgFormat)
	if err != nil {
		return nil, ImgInfo{}, fmt.Errorf("process with ffmpeg error: %w", err)
	}

	// 提前编码图片用于并发传输
	imageData := buf.Bytes()
	if len(imageData) == 0 {
		return nil, ImgInfo{}, errors.New("original image data is empty")
	}

	return imageData, imgInfo, nil
}

// processAlgorithmDetection 处理算法检测逻辑
func (t *SnapTask) processAlgorithmDetection(imageData []byte, imgInfo ImgInfo, now time.Time) error {
	base64ImageData := SnapFrameToBase64WithFFmpeg(imageData)

	detectClient := NewDetectionClient(
		t.config.AlgorithmAPI.Url,
		t.config.AlgorithmAPI.Method,
		t.config.AlgorithmAPI.ApiKey,
	)

	// 执行并行算法检测
	validResults := t.executeParallelDetection(detectClient, imageData, base64ImageData, imgInfo)

	// 如果没有有效的检测结果，直接返回
	if len(validResults) == 0 {
		return nil
	}

	// 处理检测结果（上传到OSS、发送MQTT消息和HTTP回调）
	return t.handleDetectionResults(validResults, imgInfo, base64ImageData, now)
}

// executeParallelDetection 并行执行算法检测
func (t *SnapTask) executeParallelDetection(detectClient *DetectionClient, imageData []byte, base64ImageData string, imgInfo ImgInfo) []*algorithmResult {
	var wg sync.WaitGroup
	results := make(chan *algorithmResult, len(t.config.AlgorithmId))

	// 并行执行所有算法检测
	for index, algorithmID := range t.config.AlgorithmId {
		wg.Add(1)
		go func(algId uint8, idx int) {
			defer wg.Done()

			result := &algorithmResult{
				algId: algId,
				index: idx,
			}

			req := DetectionRequest{
				AlgorithmID: algId,
				Image:       base64ImageData,
				ConfThreshold: func() float32 {
					if idx < len(t.config.ConfThreshold) {
						return t.config.ConfThreshold[idx]
					}
					return 0.6
				}(),
			}

			detectResult, err := detectClient.Detect(req)
			if err != nil {
				t.job.Plugin.Error("detect error", "error", err.Error())
				result.err = err
				// 触发 onDetectionError webhook
				t.SendDetectionWebhook(HookOnDetectionError, algId, req, err)
				results <- result
				return
			}

			if !detectResult.IsSuccess() {
				t.job.Plugin.Error("algorithm api request failed or no detections found")
				result.err = errors.New("algorithm api request failed or no detections found")
				// 触发 onDetectionError webhook
				t.SendDetectionWebhook(HookOnDetectionError, algId, req, result.err)
				results <- result
				return
			}

			if !detectResult.hasDetections() {
				results <- result
				return
			}

			result.result = detectResult

			result.rawImgData = imageData
			results <- result
		}(algorithmID, index)
	}

	// 等待所有检测完成
	wg.Wait()
	close(results)

	// 收集所有有效结果
	var validResults []*algorithmResult
	for result := range results {
		if result.err == nil && result.result != nil {
			validResults = append(validResults, result)
		}
	}

	return validResults
}

// drawBoundingBoxes 在图像上绘制检测框
func (t *SnapTask) drawBoundingBoxes(detectResult *DetectionResponse, imageData []byte, imgInfo ImgInfo) ([]byte, error) {
	processedImage := imageData
	var err error

	for _, detection := range detectResult.Data.Detections {
		bbox := FloatsToBBox(detection.BBox)
		className := "unknown"
		if detection.ClassNameCn != "" {
			className = detection.ClassNameCn
		} else if detection.ClassName != "" {
			className = detection.ClassName
		}

		processedImage, err = DrawDetectionBBox(processedImage,
			&imgInfo, t.config.SnapImgFormat, bbox, className, detection.Confidence,
			t.config.Bbox.FontPath, t.config.Bbox.FontSize, t.config.Bbox.FontColor)
		if err != nil {
			t.job.Plugin.Error("draw bounding box error", "error", err.Error())
			return nil, err
		}
	}

	if len(processedImage) == 0 {
		t.job.Plugin.Error("final image data is empty after drawing boxes")
		return nil, errors.New("final image data is empty after drawing boxes")
	}

	return processedImage, nil
}

// handleDetectionResults 处理检测结果（包括上传OSS、发送MQTT消息和HTTP回调）
func (t *SnapTask) handleDetectionResults(validResults []*algorithmResult, imgInfo ImgInfo, imgBase64 string, now time.Time) error {
	// 创建OSS文件映射，避免重复上传相同图像
	uploadedFiles := make(map[string]struct {
		accessUrl      string
		objKey         string
		processedImage []byte
	})

	client := &http.Client{Timeout: 10 * time.Second}
	publishId := uuid.New()

	// 处理每个检测结果
	for _, result := range validResults {
		callbackEntity := result.result.ToCallback(t.job.StreamPath, "", t.job.Plugin.Meta.Name, publishId)
		// 发送MQTT消息
		t.sendMQTTMessage(result, callbackEntity, t.mqttClient, t.config.MQTT)

		// 上传到OSS（如果需要）
		accessUrl, objKey, err := t.uploadToOSSIfNeeded(result, imgInfo, now, uploadedFiles)
		if err != nil {
			t.job.Plugin.Error("upload to OSS failed", "error", err.Error())
			continue
		}

		callbackEntity.Args.ObjectKey = objKey
		if accessUrl != "" {
			callbackEntity.Args.AccessUrl = accessUrl
		} else {
			callbackEntity.Args.ObjectBase64 = imgBase64
		}

		// 发送HTTP回调
		t.sendHTTPCallback(client, callbackEntity)

		// 触发 onDetectionResult webhook
		t.SendDetectionWebhook(HookOnDetectionResult, result.algId, callbackEntity, nil)
	}

	return nil
}

// uploadToOSSIfNeeded 如需要则上传到OSS
func (t *SnapTask) uploadToOSSIfNeeded(result *algorithmResult, imgInfo ImgInfo, now time.Time, uploadedFiles map[string]struct {
	accessUrl      string
	objKey         string
	processedImage []byte
}) (string, string, error) {
	if t.ossPlugin == nil {
		return "", "", nil
	}

	// 检查是否已经上传过相同图像
	key := fmt.Sprintf("%s/alg_%d", strings.ReplaceAll(t.job.StreamPath, "/", "_"), result.algId)
	if uploaded, exists := uploadedFiles[key]; exists {
		return uploaded.accessUrl, uploaded.objKey, nil
	}

	// 绘制边界框（移到这里执行，避免重复绘制）
	processedImage, err := t.drawBoundingBoxes(result.result, result.rawImgData, imgInfo)
	if err != nil {
		return "", "", err
	}

	ossFilename := fmt.Sprintf("%s/alg_%d/%s.%s",
		strings.ReplaceAll(t.job.StreamPath, "/", "_"),
		result.algId,
		now.Format("20060102150405.000"),
		t.config.SnapImgFormat)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	file, err := t.ossPlugin.CreateFile(ctx, ossFilename)
	if err != nil {
		return "", "", err
	}
	defer file.Close()

	_, err = file.Write(processedImage)
	if err != nil {
		return "", "", err
	}

	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return "", "", err
	}

	err = file.Sync()
	if err != nil {
		return "", "", err
	}

	accessUrl, err := t.ossPlugin.GetURL(ctx, ossFilename)
	if err != nil {
		return "", "", err
	}

	objKey := getObjectKey(accessUrl)
	uploadedFiles[key] = struct {
		accessUrl      string
		objKey         string
		processedImage []byte
	}{accessUrl, objKey, processedImage}

	return accessUrl, objKey, nil
}

// sendMQTTMessage 发送MQTT消息
func (t *SnapTask) sendMQTTMessage(result *algorithmResult, callbackEntity *CallbackDetection, mqttClient MQTTClient, mqttConfig *MQTTConfig) {
	if mqttClient == nil || !mqttClient.IsConnected() {
		return
	}

	// 遍历配置中的发布主题并发送消息
	for i, topic := range mqttConfig.Pub {
		err := mqttClient.PublishWithIndex(topic, i, result.algId, t.job.StreamPath, callbackEntity)
		if err != nil {
			t.job.Plugin.Error("MQTT publish failed", "error", err.Error())
		}
	}
}

// sendHTTPCallback 发送HTTP回调
func (t *SnapTask) sendHTTPCallback(client *http.Client, callbackEntity *CallbackDetection) {
	if t.config.AlgorithmAPI.CallbackURL == "" {
		return
	}

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

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		t.job.Plugin.Warn("callback response status not ok", "status", resp.Status, "body", string(body))
	}
}

func getObjectKey(accessUrl string) string {
	// 解析URL并提取路径部分作为object key
	parsedURL, err := url.Parse(accessUrl)
	if err != nil {
		return ""
	}

	// 移除路径开头的斜杠（如果存在）
	path := strings.TrimPrefix(parsedURL.Path, "/")
	return path
}

// SendDetectionWebhook 发送检测相关的webhook
func (t *SnapTask) SendDetectionWebhook(webhookType config.HookType, algorithmID uint8, data interface{}, err error) {
	// 获取服务器信息
	hostname, _ := os.Hostname()
	ipAddr := "unknown"
	addrs, err := net.InterfaceAddrs()
	if err == nil {
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					ipAddr = ipnet.IP.String()
					break
				}
			}
		}
	}

	webhookData := m7s.AlarmInfo{
		StreamPath: t.job.StreamPath,
		ServerInfo: fmt.Sprintf("%s (%s)", hostname, ipAddr),
		AlarmName:  webhookType,
		Data:       convertToMap(data),
	}

	if sender, webhook := t.job.Plugin.GetHookSender(webhookType); sender != nil {
		sender(webhook, webhookData)
	}
}

// 添加辅助函数 convertToMap
func convertToMap(v interface{}) map[string]interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	// 可选：尝试通过 JSON 序列化反序列化转换结构体
	b, _ := json.Marshal(v)
	var result map[string]interface{}
	json.Unmarshal(b, &result)
	return result
}
