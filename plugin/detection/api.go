package plugin_detection

import (
	"encoding/json"
	"fmt"
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
	SnapMode       []int     `json:"snapMode" default:"[0]" desc:"截图模式: 0-时间间隔，1-关键帧间隔 2-HTTP请求模式（手动触发）"`
	TimerInterval  []string  `json:"timeInterval" desc:"截图时间间隔, 仅在SnapMode为0时生效"`
	IFrameInterval []int     `json:"iframeInterval" desc:"间隔多少帧截图, 仅在SnapMode为1时生效"`
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
	var timerIntervals []time.Duration

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

	// 如果snapMode未提供，则默认为[0]
	if len(configuration.SnapMode) == 0 {
		configuration.SnapMode = []int{0}
	}

	// 确保数组长度一致
	maxLen := len(configuration.SnapMode)
	if len(configuration.TimerInterval) > maxLen {
		maxLen = len(configuration.TimerInterval)
	}
	if len(configuration.IFrameInterval) > maxLen {
		maxLen = len(configuration.IFrameInterval)
	}

	// 扩展数组至相同长度
	for len(configuration.SnapMode) < maxLen {
		configuration.SnapMode = append(configuration.SnapMode, configuration.SnapMode[len(configuration.SnapMode)-1])
	}
	for len(configuration.TimerInterval) < maxLen {
		configuration.TimerInterval = append(configuration.TimerInterval, "")
	}
	for len(configuration.IFrameInterval) < maxLen {
		configuration.IFrameInterval = append(configuration.IFrameInterval, configuration.IFrameInterval[len(configuration.IFrameInterval)-1])
	}

	// 初始化timerIntervals数组
	timerIntervals = make([]time.Duration, maxLen)

	// 参数校验
	for i, mode := range configuration.SnapMode {
		if mode < 0 || mode > 2 {
			sendError(rw, http.StatusBadRequest, fmt.Sprintf("snapMode[%d] must be between 0 and 2", i))
			return
		}

		if mode == 0 {
			if configuration.TimerInterval[i] == "" {
				// 如果未提供timeInterval，使用默认值1s
				configuration.TimerInterval[i] = "1s"
			}

			duration, err := time.ParseDuration(configuration.TimerInterval[i])
			if err != nil || duration <= 0 {
				sendError(rw, http.StatusBadRequest, fmt.Sprintf("invalid timeInterval[%d] format or value, must be greater than 0", i))
				return
			}
			timerIntervals[i] = duration
		}

		if mode == 1 && configuration.IFrameInterval[i] <= 0 {
			// 如果未提供IFrameInterval，使用默认值1
			if configuration.IFrameInterval[i] == 0 {
				configuration.IFrameInterval[i] = 1
			} else {
				sendError(rw, http.StatusBadRequest, fmt.Sprintf("iframeInterval[%d] must be greater than 0 when snapMode[%d] is 1", i, i))
				return
			}
		}
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

	// 创建输出配置数组
	var outputs []config.TransformOutput
	for i := 0; i < maxLen; i++ {
		// 构造当前输出配置的算法ID和阈值数组
		var algorithmIds []uint8
		var thresholds []float32

		// 如果AlgorithmId和Threshold数组长度大于i，则取对应索引的值
		if len(configuration.AlgorithmId) > i {
			algorithmIds = []uint8{configuration.AlgorithmId[i]}
			if len(configuration.Threshold) > i {
				thresholds = []float32{configuration.Threshold[i]}
			} else {
				thresholds = []float32{0.5} // 默认阈值
			}
		} else if len(configuration.AlgorithmId) > 0 {
			// 如果索引超出范围但数组不为空，则使用第一个元素
			algorithmIds = []uint8{configuration.AlgorithmId[0]}
			if len(configuration.Threshold) > 0 {
				thresholds = []float32{configuration.Threshold[0]}
			} else {
				thresholds = []float32{0.5} // 默认阈值
			}
		} else {
			// 如果没有提供算法配置，使用默认值
			algorithmIds = []uint8{1}
			thresholds = []float32{0.5}
		}

		conf := detection.SnapConfig{
			SnapMode:       configuration.SnapMode[i],
			IFrameInterval: configuration.IFrameInterval[i],
			AlgorithmId:    algorithmIds,
			ConfThreshold:  thresholds,
		}

		if configuration.SnapMode[i] == 0 && i < len(timerIntervals) {
			conf.TimeInterval = timerIntervals[i]
		}

		output := config.TransformOutput{
			Target:     streamPath,
			StreamPath: streamPath,
			Conf:       conf,
		}

		outputs = append(outputs, output)
	}

	// 创建新的 Transformer 实例，并初始化
	trans = detection.NewTransform().(*detection.Transformer)
	trans.TransformJob.Init(trans, &p.Plugin, publisher, config.Transform{
		Output: outputs,
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
