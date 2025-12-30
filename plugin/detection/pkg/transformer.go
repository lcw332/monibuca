package detection

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
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

// ==================== 截图模式常量 ====================

type SnapMode int

const (
	SnapModeTimeInterval SnapMode = iota
	SnapModeIFrameInterval
	SnapModeManual
)

// ==================== 内部结果类型 ====================

// algorithmResult 存储算法检测结果
type algorithmResult struct {
	algId        uint8
	index        int
	result       *DetectionResponse
	rawImgData   []byte
	rawImgBase64 string
	err          error
}

// ==================== Transformer 结构体 ====================

type Transformer struct {
	task.Job
	TransformJob m7s.TransformJob
}

type SnapTask struct {
	job        *m7s.TransformJob
	ossPlugin  storage.Storage
	config     SnapConfig
	mqttClient MQTTClient
	tracker    *Tracker // 帧跟踪器
}

// Dispose 清理帧历史记录
func (t *SnapTask) Dispose() {
	if t.tracker != nil {
		t.tracker.ClearAllHistory()
	}
}

type AlgTask struct {
	job *m7s.TransformJob
}

// ==================== Transformer 接口实现 ====================

func (t *Transformer) GetTransformJob() *m7s.TransformJob {
	return &t.TransformJob
}

func NewTransform() m7s.ITransformer {
	return &Transformer{}
}

// Start 启动任务
func (t *Transformer) Start() (err error) {
	plugin := t.TransformJob.Plugin

	// 初始化OSS插件
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

	// 初始化算法API配置
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

	// 初始化Bbox配置
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

	// 初始化截图格式配置
	snapImgFormat := plugin.Config.Get("snapImgFormat")
	var globalSnapImgFormat string
	if snapImgFormat != nil {
		globalSnapImgFormat = ""
		switch v := snapImgFormat.File.(type) {
		case string:
			globalSnapImgFormat = v
		}
	}

	// 初始化MQTT配置
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

	// 为每个输出配置创建截图任务
	for _, output := range t.TransformJob.Config.Output {
		var snapTask task.ITask
		var snapConfig SnapConfig

		if output.Conf != nil {
			switch v := output.Conf.(type) {
			case SnapConfig:
				snapConfig = v
			case map[string]any:
				config.Parse(&snapConfig, v)
			}
		}

		// 应用全局配置
		applyGlobalConfig(&snapConfig, globalAlgApi, globalBbox, globalSnapImgFormat, globalMQTTConfig)

		// 创建MQTT客户端
		var mqttClient MQTTClient
		if globalMQTTConfig != nil && globalMQTTConfig.Enable {
			mqttClient, _ = NewMQTTClient(globalMQTTConfig, plugin.Logger)
		}

		// 根据截图模式创建任务
		snapTask = createSnapTask(snapConfig, &t.TransformJob, ossPlugin, mqttClient)
		if snapTask != nil {
			t.AddTask(snapTask)
		}
	}
	return nil
}

// applyGlobalConfig 应用全局配置到任务配置
func applyGlobalConfig(snapConfig *SnapConfig, globalAlgApi *AlgorithmAPI, globalBbox *Bbox, globalSnapImgFormat string, globalMQTTConfig *MQTTConfig) {
	if snapConfig.AlgorithmAPI == nil && globalAlgApi != nil {
		snapConfig.AlgorithmAPI = globalAlgApi
	}
	if snapConfig.Bbox == nil && globalBbox != nil {
		snapConfig.Bbox = globalBbox
	}
	if snapConfig.SnapImgFormat == "" && globalSnapImgFormat != "" {
		snapConfig.SnapImgFormat = globalSnapImgFormat
	}
	if snapConfig.MQTT == nil && globalMQTTConfig != nil {
		snapConfig.MQTT = globalMQTTConfig
	}
}

// createSnapTask 根据截图模式创建任务
func createSnapTask(snapConfig SnapConfig, job *m7s.TransformJob, ossPlugin storage.Storage, mqttClient MQTTClient) task.ITask {
	baseTask := SnapTask{
		config:     snapConfig,
		job:        job,
		ossPlugin:  ossPlugin,
		mqttClient: mqttClient,
		tracker:    NewTracker(),
	}

	var task task.ITask
	switch snapConfig.SnapMode {
	case int(SnapModeTimeInterval):
		task = &TimeSnapTask{
			SnapTask: baseTask,
		}
	case int(SnapModeIFrameInterval):
		task = &IFrameSnapTask{
			SnapTask: baseTask,
		}
	case int(SnapModeManual):
		return nil
	}

	// 设置 Dispose 回调，任务结束时清理帧历史
	if task != nil {
		task.OnDispose(func() {
			baseTask.Dispose()
		})
	}

	return task
}

// ==================== IFrameSnapTask 关键帧间隔截图任务 ====================

type IFrameSnapTask struct {
	task.Task
	SnapTask
	subscriber *m7s.Subscriber
}

// Start 启动关键帧间隔截图任务
func (t *IFrameSnapTask) Start() (err error) {
	subConfig := t.job.Plugin.GetCommonConf().Subscribe
	subConfig.SubType = m7s.SubscribeTypeTransform
	subConfig.IFrameOnly = true
	t.subscriber, err = t.job.Plugin.SubscribeWithConfig(t, t.job.StreamPath, subConfig)
	return
}

// Go 关键帧间隔截图任务执行逻辑
func (t *IFrameSnapTask) Go() (err error) {
	iframeCount := 0
	err = m7s.PlayBlock(t.subscriber, (func(audio *pkg.AVFrame) error)(nil), func(video *format.AnnexB) error {
		iframeCount++
		if iframeCount%t.config.IFrameInterval == 0 {
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

// ==================== TimeSnapTask 定时截图任务 ====================

type TimeSnapTask struct {
	task.TickTask
	SnapTask
}

// GetTickInterval 获取定时截图间隔
func (t *TimeSnapTask) GetTickInterval() time.Duration {
	return t.config.TimeInterval
}

// Tick 定时截图任务执行逻辑
func (t *TimeSnapTask) Tick(any) {
	if t.job.OriginPublisher == nil {
		return
	}

	annexb, err := GetVideoFrame(t.job.OriginPublisher, t.job.Plugin.Server)
	if err != nil {
		t.Error("get video frame failed", "error", err.Error())
		return
	}

	if err := t.saveSnap(annexb, SnapModeTimeInterval); err != nil {
		t.Error("save snapshot failed", "error", err.Error())
	}
}

// ==================== 核心截图逻辑 ====================

// saveSnap 保存截图
func (t *SnapTask) saveSnap(annexb []*format.AnnexB, mode SnapMode) (err error) {
	now := time.Now()

	// 处理视频帧
	imageData, imgInfo, err := t.processVideoFrame(annexb)
	if err != nil {
		return err
	}

	// 调用算法检测
	if t.config.AlgorithmAPI.Enable {
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

	imageData := buf.Bytes()
	if len(imageData) == 0 {
		return nil, ImgInfo{}, errors.New("original image data is empty")
	}

	return imageData, imgInfo, nil
}

// ==================== 算法检测逻辑 ====================

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

	// 处理连续帧检测，获取带标识的检测结果
	trackerResults := t.tracker.ProcessFrameCheck(validResults, t.config)

	// 如果没有有效的检测结果，直接返回
	if len(validResults) == 0 {
		return nil
	}

	return t.handleDetectionResults(validResults, trackerResults, imgInfo, now)
}

// executeParallelDetection 并行执行算法检测
func (t *SnapTask) executeParallelDetection(detectClient *DetectionClient, imageData []byte, base64ImageData string, imgInfo ImgInfo) []*algorithmResult {
	var wg sync.WaitGroup
	results := make(chan *algorithmResult, len(t.config.AlgorithmId))

	for index, algorithmID := range t.config.AlgorithmId {
		wg.Add(1)
		go func(algId uint8, idx int) {
			defer wg.Done()

			result := &algorithmResult{
				algId:        algId,
				index:        idx,
				rawImgBase64: base64ImageData,
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
				t.SendDetectionWebhook(HookOnDetectionError, algId, req, err)
				results <- result
				return
			}

			if !detectResult.IsSuccess() {
				t.job.Plugin.Error("algorithm api request failed or no detections found")
				result.err = errors.New("algorithm api request failed or no detections found")
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

	wg.Wait()
	close(results)

	var validResults []*algorithmResult
	for result := range results {
		if result.err == nil && result.result != nil {
			validResults = append(validResults, result)
		}
	}

	return validResults
}

// ==================== 结果处理 ====================

// handleDetectionResults 处理检测结果
func (t *SnapTask) handleDetectionResults(validResults []*algorithmResult, trackerResults map[uint8]*TrackerResult, imgInfo ImgInfo, now time.Time) error {
	uploadedFiles := make(map[string]struct {
		accessUrl      string
		objKey         string
		processedImage []byte
	})

	publishId := uuid.New()

	for _, result := range validResults {
		// 获取跟踪器结果
		trackerResult := trackerResults[result.algId]
		frameCheck := getFrameCheckForAlgId(t.config, result.algId)

		// 如果有跟踪器结果，应用带标识的检测结果
		if trackerResult != nil && len(trackerResult.MarkedResults) > 0 {
			result.result.Data.Detections = trackerResult.MarkedResults
			result.result.Data.TotalCount = len(trackerResult.MarkedResults)
		}

		callbackEntity := result.result.ToCallback(t.job.StreamPath, "", t.job.Plugin.Meta.Name, publishId)

		// 添加连续帧检测相关信息到回调参数
		callbackEntity.Args.HasFrameCheck = frameCheck > 0
		callbackEntity.Args.FrameCheckCount = frameCheck
		if trackerResult != nil {
			callbackEntity.Args.SameObjTotalCount = trackerResult.SameObjTotalCount
		}

		t.sendMQTTMessage(result, callbackEntity, t.mqttClient, t.config.MQTT)

		processedImage, err := t.drawBoundingBoxes(result.result, result.rawImgData, imgInfo)
		if err != nil {
			t.job.Plugin.Error("draw bounding box error", "error", err.Error())
			continue
		}

		accessUrl, objKey, err := t.uploadToOSSIfNeeded(result, &processedImage, now, uploadedFiles)
		if err != nil {
			t.job.Plugin.Error("upload to OSS failed", "error", err.Error())
			continue
		}

		callbackEntity.Args.ObjectKey = objKey
		if accessUrl != "" {
			callbackEntity.Args.AccessUrl = accessUrl
		} else {
			if t.config.SnapOriginal {
				callbackEntity.Args.ObjectRaw = result.rawImgBase64
			}
			callbackEntity.Args.ObjectArtifacts = SnapFrameToBase64WithFFmpeg(processedImage)
		}

		t.SendDetectionWebhook(HookOnDetectionResult, result.algId, callbackEntity.Args, nil)
	}

	return nil
}

// drawBoundingBoxes 在图像上绘制检测框
func (t *SnapTask) drawBoundingBoxes(detectResult *DetectionResponse, imageData []byte, imgInfo ImgInfo) ([]byte, error) {
	processedImage := imageData
	var err error

	for _, detection := range detectResult.Data.Detections {
		bbox := FloatsToBBox(detection.BBox)
		className := detection.ClassNameCn
		if className == "" {
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

// uploadToOSSIfNeeded 如需要则上传到OSS
func (t *SnapTask) uploadToOSSIfNeeded(result *algorithmResult, bboxImg *[]byte, now time.Time, uploadedFiles map[string]struct {
	accessUrl      string
	objKey         string
	processedImage []byte
}) (string, string, error) {
	if t.ossPlugin == nil {
		return "", "", nil
	}

	key := fmt.Sprintf("%s/alg_%d", strings.ReplaceAll(t.job.StreamPath, "/", "_"), result.algId)
	if uploaded, exists := uploadedFiles[key]; exists {
		return uploaded.accessUrl, uploaded.objKey, nil
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

	_, err = file.Write(*bboxImg)
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
	}{accessUrl, objKey, *bboxImg}

	return accessUrl, objKey, nil
}

// sendMQTTMessage 发送MQTT消息
func (t *SnapTask) sendMQTTMessage(result *algorithmResult, callbackEntity *CallbackDetection, mqttClient MQTTClient, mqttConfig *MQTTConfig) {
	if mqttClient == nil || !mqttClient.IsConnected() {
		return
	}

	for i, topic := range mqttConfig.Pub {
		err := mqttClient.PublishWithIndex(topic, i, result.algId, t.job.StreamPath, callbackEntity)
		if err != nil {
			t.job.Plugin.Error("MQTT publish failed", "error", err.Error())
		}
	}
}

// ==================== Webhook 工具函数 ====================

// getObjectKey 从URL提取对象Key
func getObjectKey(accessUrl string) string {
	parsedURL, err := url.Parse(accessUrl)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(parsedURL.Path, "/")
}

// SendDetectionWebhook 发送检测相关的webhook
func (t *SnapTask) SendDetectionWebhook(webhookType config.HookType, algorithmID uint8, data interface{}, err error) {
	hostname, _ := os.Hostname()
	ipAddr := getLocalIP()

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

// getLocalIP 获取本地IP地址
func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "unknown"
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "unknown"
}

// convertToMap 转换为map
func convertToMap(v interface{}) map[string]interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	b, _ := json.Marshal(v)
	var result map[string]interface{}
	json.Unmarshal(b, &result)
	return result
}
