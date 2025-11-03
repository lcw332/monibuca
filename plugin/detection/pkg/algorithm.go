package detection

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

// AlgorithmMap 定义算法映射关系
type AlgorithmId map[int]string

var AlgorithmMap = AlgorithmId{
	1:  "松线虫害识别",
	2:  "河道淤积识别",
	3:  "漂浮物识别",
	4:  "游泳涉水识别",
	5:  "车牌识别",
	6:  "交通拥堵识别",
	7:  "路面破损识别",
	8:  "路面污染",
	9:  "人群聚集识别",
	10: "非法垂钓识别",
	11: "施工识别",
	12: "秸秆焚烧",
	14: "占道经营识别",
	15: "垃圾堆放识别",
	16: "裸土未覆盖识别",
	18: "烟火识别",
	19: "光伏板缺陷检测",
	20: "园区夜间入侵检测",
	21: "外立面病害识别",
	22: "罂粟识别",
	24: "林业侵占",
}

// DetectionRequest 定义请求结构体
type DetectionRequest struct {
	AlgorithmID   uint8   `json:"algorithm_id"`
	Image         string  `json:"image"`
	ConfThreshold float64 `json:"conf_threshold,omitempty"`
}

// DetectionResult 定义检测结果结构
type DetectionResult struct {
	ClassID    int       `json:"class_id"`
	ClassName  string    `json:"class_name"`
	Confidence float64   `json:"confidence"`
	BBox       []float64 `json:"bbox"`
}

// DetectionResponse 定义响应结构体
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

// CallbackDetection 回调结构体
type CallbackDetection struct {
	Event      string `json:"event"`
	StreamPath string `json:"streamPath"`
	Args       struct {
		AccessUrl     string            `json:"access_url" desc:"对象存储访问链接"`
		AlgorithmID   int               `json:"algorithm_id"`
		AlgorithmName string            `json:"algorithm_name"`
		Detections    []DetectionResult `json:"detections"`
		TotalCount    int               `json:"total_count"`
		DetectTime    float64           `json:"detect_time"`
	} `json:"args"`
	PublishId  uint32 `json:"publishId"`
	RemoteAddr string `json:"remoteAddr"`
	Type       string `json:"type"`
	PluginName string `json:"pluginName"`
	Timestamp  int    `json:"timestamp"`
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

// DetectionClient 封装检测客户端
type DetectionClient struct {
	URL        string
	Method     string
	APIKey     string
	HTTPClient *http.Client
}

// NewDetectionClient 创建新的检测客户端
func NewDetectionClient(URL, Method, apiKey string) *DetectionClient {
	return &DetectionClient{
		URL:    URL,
		Method: Method,
		APIKey: apiKey,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Detect 发送检测请求
func (c *DetectionClient) Detect(req DetectionRequest) (*DetectionResponse, error) {
	// 序列化请求体
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	// 创建HTTP请求
	httpReq, err := http.NewRequest(c.Method, c.URL, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// 设置请求头
	httpReq.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		httpReq.Header.Set("x-api-key", c.APIKey)
	}

	// 添加自定义请求头
	// 这里可以扩展支持从配置中读取自定义请求头

	// 发送请求
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// 解析响应
	var detectionResp DetectionResponse
	if err := json.Unmarshal(respBody, &detectionResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	return &detectionResp, nil
}

// BatchDetect 发送批量检测请求
func (c *DetectionClient) BatchDetect(reqs []DetectionRequest) (*BatchDetectionResponse, error) {
	batchReq := BatchDetectionRequest{
		Requests: reqs,
	}

	// 序列化请求体
	body, err := json.Marshal(batchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal batch request: %v", err)
	}

	// 创建HTTP请求
	httpReq, err := http.NewRequest(c.Method, c.URL, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create batch request: %v", err)
	}

	// 设置请求头
	httpReq.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		httpReq.Header.Set("X-API-Key", c.APIKey)
	}

	// 发送请求
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send batch request: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read batch response body: %v", err)
	}

	// 解析响应
	var batchResp BatchDetectionResponse
	if err := json.Unmarshal(respBody, &batchResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal batch response: %v", err)
	}

	return &batchResp, nil
}

// IsSuccess 判断响应是否成功
func (resp *DetectionResponse) IsSuccess() bool {
	return resp.Code == 200
}

// hasDetections 判断响应中是否有检测结果
func (resp *DetectionResponse) hasDetections() bool {
	return len(resp.Data.Detections) > 0
}

// ToCallback converts a DetectionResponse to a CallbackDetection
func (resp *DetectionResponse) ToCallback(streamPath, remoteAddr, pluginName string, publishId uint32) *CallbackDetection {
	callback := &CallbackDetection{
		Event:      "detection",
		StreamPath: streamPath,
		PublishId:  publishId,
		RemoteAddr: remoteAddr,
		Type:       "detection",
		PluginName: pluginName,
		Timestamp:  int(time.Now().Unix()),
	}

	callback.Args.AlgorithmID = resp.Data.AlgorithmID
	callback.Args.AlgorithmName = resp.Data.AlgorithmName
	callback.Args.Detections = resp.Data.Detections
	callback.Args.TotalCount = resp.Data.TotalCount
	callback.Args.DetectTime = resp.Data.DetectTime
	return callback
}
