package detection

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/google/uuid"
	"m7s.live/v5/plugin/detection/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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
	ConfThreshold float32 `json:"conf_threshold,omitempty"`
}

// DetectionResult 定义检测结果结构
type DetectionResult struct {
	ClassID     int       `json:"class_id"`
	ClassName   string    `json:"class_name"`
	ClassNameCn string    `json:"class_name_cn"`
	Confidence  float64   `json:"confidence"`
	BBox        []float64 `json:"bbox"`
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
		AccessUrl     string            `json:"access_url,omitempty" desc:"对象存储访问链接"`
		ObjectKey     string            `json:"object_key,omitempty" desc:"对象存储 Key"`
		AlgorithmID   int               `json:"algorithm_id"`
		AlgorithmName string            `json:"algorithm_name"`
		Detections    []DetectionResult `json:"detections"`
		TotalCount    int               `json:"total_count"`
		DetectTime    float64           `json:"detect_time"`
	} `json:"args"`
	PublishId  uuid.UUID `json:"publishId"`
	RemoteAddr string    `json:"remoteAddr"`
	Type       string    `json:"type"`
	PluginName string    `json:"pluginName"`
	Timestamp  int       `json:"timestamp"`
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
	// gRPC相关字段
	GRPCClient pb.DetectionServiceClient
	GRPConn    *grpc.ClientConn
	IsGRPC     bool
}

// NewDetectionClient 创建新的检测客户端
func NewDetectionClient(URL, Method, apiKey string) *DetectionClient {
	client := &DetectionClient{
		URL:    URL,
		Method: Method,
		APIKey: apiKey,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		IsGRPC: false,
	}

	// 如果URL以grpc://开头，则初始化gRPC客户端
	if len(URL) > 7 && URL[:7] == "grpc://" {
		client.IsGRPC = true
		conn, err := grpc.NewClient(URL[7:], grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			// 如果gRPC连接失败，回退到HTTP模式
			client.IsGRPC = false
			return client
		}
		client.GRPConn = conn
		client.GRPCClient = pb.NewDetectionServiceClient(conn)
	}

	return client
}

// Detect 发送检测请求
func (c *DetectionClient) Detect(req DetectionRequest) (*DetectionResponse, error) {
	if c.IsGRPC {
		return c.detectGRPC(req)
	}
	return c.detectHTTP(req)
}

// detectHTTP 通过HTTP发送检测请求
func (c *DetectionClient) detectHTTP(req DetectionRequest) (*DetectionResponse, error) {
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

// detectGRPC 通过gRPC发送检测请求
func (c *DetectionClient) detectGRPC(req DetectionRequest) (*DetectionResponse, error) {
	grpcReq := &pb.DetectRequest{
		AlgorithmId:   int32(req.AlgorithmID),
		Image:         req.Image,
		ConfThreshold: req.ConfThreshold,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ctx = context.WithValue(ctx, "x-api-key", c.APIKey)
	grpcResp, err := c.GRPCClient.Detect(ctx, grpcReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send gRPC request: %v", err)
	}

	// 转换响应格式
	detections := make([]DetectionResult, len(grpcResp.Data.Detections))
	for i, d := range grpcResp.Data.Detections {
		bbox := make([]float64, len(d.Bbox))
		for j, b := range d.Bbox {
			bbox[j] = float64(b)
		}
		detections[i] = DetectionResult{
			ClassID:     int(d.ClassId),
			ClassName:   d.ClassName,
			ClassNameCn: d.ClassNameCn,
			Confidence:  float64(d.Confidence),
			BBox:        bbox,
		}
	}

	detectionResp := &DetectionResponse{
		Code:    int(grpcResp.Code),
		Message: grpcResp.Message,
		Data: struct {
			AlgorithmID   int               `json:"algorithm_id"`
			AlgorithmName string            `json:"algorithm_name"`
			Detections    []DetectionResult `json:"detections"`
			TotalCount    int               `json:"total_count"`
			DetectTime    float64           `json:"detect_time"`
		}{
			AlgorithmID:   int(grpcResp.Data.AlgorithmId),
			AlgorithmName: grpcResp.Data.AlgorithmName,
			Detections:    detections,
			TotalCount:    int(grpcResp.Data.TotalCount),
			DetectTime:    float64(grpcResp.Data.DetectTime),
		},
	}

	return detectionResp, nil
}

// BatchDetect 发送批量检测请求
func (c *DetectionClient) BatchDetect(reqs []DetectionRequest) (*BatchDetectionResponse, error) {
	// gRPC模式下暂不支持批量检测，回退到HTTP实现
	return c.batchDetectHTTP(reqs)
}

// batchDetectHTTP 发送HTTP批量检测请求
func (c *DetectionClient) batchDetectHTTP(reqs []DetectionRequest) (*BatchDetectionResponse, error) {
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
	respBody, err := io.ReadAll(resp.Body)
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

// Close 关闭客户端连接
func (c *DetectionClient) Close() error {
	if c.IsGRPC && c.GRPConn != nil {
		return c.GRPConn.Close()
	}
	return nil
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
func (resp *DetectionResponse) ToCallback(streamPath, remoteAddr, pluginName string, publishId uuid.UUID) *CallbackDetection {
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
