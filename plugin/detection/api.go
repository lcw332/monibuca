package plugin_detection

import (
	"encoding/json"
	"net/http"
	"time"

	task "github.com/langhuihui/gotask"
	"m7s.live/v5/pkg/config"
	detection "m7s.live/v5/plugin/detection/pkg"
)

type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func sendResponse(rw http.ResponseWriter, code int, message string, data interface{}) {
	response := APIResponse{
		Code:    code,
		Message: message,
		Data:    data,
	}
	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(response)
}

func sendSuccess(rw http.ResponseWriter, data interface{}) {
	sendResponse(rw, 0, "success", data)
}

func sendError(rw http.ResponseWriter, code int, message string) {
	sendResponse(rw, code, message, nil)
}

func (p *DetectionPlugin) RegisterHandler() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"/list":    p.listConfig,
		"/launch":  p.launchDetection,
		"/dispose": p.disposeDetection,
	}
}

type UpdateDetectRequest struct {
	StreamPath    string        `json:"streamPath" desc:"流地址"`
	Configuration Configuration `json:"configuration" desc:"配置文件"`
}

type Configuration struct {
	SnapMode       int       `json:"snapMode" default:"0" desc:"截图模式: 0-时间间隔，1-关键帧间隔 2-HTTP请求模式（手动触发）"`
	TimerInterval  string    `json:"timeInterval" desc:"截图时间间隔, 仅在SnapMode为0时生效"`
	IFrameInterval int       `json:"iframeInterval" desc:"间隔多少帧截图, 仅在SnapMode为1时生效"`
	AlgorithmId    []uint8   `json:"algorithmId" default:"[]" desc:"算法ID"`
	Threshold      []float32 `json:"threshold" default:"[]" desc:"置信度配置，与算法ID一一对应"`
}

// launchDetection 启动图像检测
func (p *DetectionPlugin) launchDetection(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendError(rw, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req UpdateDetectRequest
	var timerInterval time.Duration

	// 从请求中解析流路径参数
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(rw, http.StatusBadRequest, "Invalid JSON format")
		return
	}

	// 从请求中解析流路径参数
	streamPath := req.StreamPath
	if streamPath == "" {
		sendError(rw, http.StatusBadRequest, "streamPath is required")
		return
	}

	configuration := req.Configuration
	// 参数校验
	if configuration.SnapMode < 0 || configuration.SnapMode > 2 {
		sendError(rw, http.StatusBadRequest, "snapMode must be between 0 and 2")
		return
	}

	if configuration.SnapMode == 0 {
		duration, err := time.ParseDuration(configuration.TimerInterval)
		if err != nil || duration <= 0 {
			sendError(rw, http.StatusBadRequest, "invalid timeInterval format or value, must be greater than 0")
			return
		}
		timerInterval = duration
	}
	if configuration.SnapMode == 1 && configuration.IFrameInterval <= 0 {
		sendError(rw, http.StatusBadRequest, "iframeInterval must be greater than 0 when snapMode is 1")
		return
	}

	if len(configuration.AlgorithmId) != len(configuration.Threshold) {
		sendError(rw, http.StatusBadRequest, "algorithmId and threshold must have the same length")
		return
	}

	for _, threshold := range configuration.Threshold {
		if threshold < 0 || threshold > 1 {
			sendError(rw, http.StatusBadRequest, "confThreshold must be between 0 and 1")
			return
		}
	}

	publisher, err := p.Server.GetPublisher(streamPath)
	if err != nil {
		sendError(rw, http.StatusNotFound, "stream not found")
		return
	}

	var trans *detection.Transformer
	// TODO: 支持正则匹配流名称
	if tm, ok := p.Server.Transforms.Get(streamPath); ok && tm != nil {
		// 如果能找到之前关联的 Transform, 则先停止并移除它
		tm.TransformJob.Stop(task.ErrTaskComplete)
		p.Logger.Debug("remove transform")
	}

	// 创建新的 Transformer 实例，并初始化
	trans = detection.NewTransform().(*detection.Transformer)
	trans.TransformJob.Init(trans, &p.Plugin, publisher, config.Transform{
		Output: []config.TransformOutput{
			{
				Target:     streamPath,
				StreamPath: streamPath,
				Conf: detection.SnapConfig{
					SnapMode:       configuration.SnapMode,
					TimeInterval:   timerInterval,
					IFrameInterval: configuration.IFrameInterval,
					AlgorithmId:    configuration.AlgorithmId,
					ConfThreshold:  configuration.Threshold,
				},
			},
		},
	})

	err = trans.WaitStarted()
	if err != nil {
		sendError(rw, http.StatusInternalServerError, err.Error())
		return
	}

	sendSuccess(rw, nil)
}

// disposeDetection 停止图像检测
func (p *DetectionPlugin) disposeDetection(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		sendError(rw, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		StreamPath string `json:"streamPath"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(rw, http.StatusBadRequest, "Invalid JSON format")
		return
	}

	streamPath := req.StreamPath
	if streamPath == "" {
		sendError(rw, http.StatusBadRequest, "streamPath is required")
		return
	}

	_, err := p.Server.GetPublisher(streamPath)
	if err != nil {
		sendError(rw, http.StatusNotFound, "stream not found")
		return
	}

	if tm, ok := p.Server.Transforms.Get(streamPath); ok && tm != nil {
		tm.TransformJob.Stop(task.ErrTaskComplete)
		p.Logger.Debug("remove transform")
	}

	sendSuccess(rw, nil)
}

// listConfig 列出所有算法配置
func (p *DetectionPlugin) listConfig(rw http.ResponseWriter, r *http.Request) {
	configs := make([]Configuration, 0)
	// TODO: 实现获取配置列表逻辑
	sendSuccess(rw, configs)
}
