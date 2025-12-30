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
	"google.golang.org/grpc/metadata"
)

// ==================== 检测客户端 ====================

// DetectionClient 封装检测客户端
type DetectionClient struct {
	URL        string
	Method     string
	APIKey     string
	HTTPClient *http.Client
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

// ==================== HTTP 检测 ====================

// detectHTTP 通过HTTP发送检测请求
func (c *DetectionClient) detectHTTP(req DetectionRequest) (*DetectionResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	httpReq, err := http.NewRequest(c.Method, c.URL, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		httpReq.Header.Set("x-api-key", c.APIKey)
	}

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	var detectionResp DetectionResponse
	if err := json.Unmarshal(respBody, &detectionResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	return &detectionResp, nil
}

// batchDetectHTTP 发送HTTP批量检测请求
func (c *DetectionClient) batchDetectHTTP(reqs []DetectionRequest) (*BatchDetectionResponse, error) {
	batchReq := BatchDetectionRequest{Requests: reqs}

	body, err := json.Marshal(batchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal batch request: %v", err)
	}

	httpReq, err := http.NewRequest(c.Method, c.URL, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create batch request: %v", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		httpReq.Header.Set("X-API-Key", c.APIKey)
	}

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send batch request: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read batch response body: %v", err)
	}

	var batchResp BatchDetectionResponse
	if err := json.Unmarshal(respBody, &batchResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal batch response: %v", err)
	}

	return &batchResp, nil
}

// ==================== gRPC 检测 ====================

// detectGRPC 通过gRPC发送检测请求
func (c *DetectionClient) detectGRPC(req DetectionRequest) (*DetectionResponse, error) {
	grpcReq := &pb.DetectRequest{
		AlgorithmId:   int32(req.AlgorithmID),
		Image:         req.Image,
		ConfThreshold: req.ConfThreshold,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if c.APIKey != "" {
		md := metadata.Pairs("x-api-key", c.APIKey)
		ctx = metadata.NewOutgoingContext(ctx, md)
	}

	grpcResp, err := c.GRPCClient.Detect(ctx, grpcReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send gRPC request: %v", err)
	}

	return convertGRPCResponse(grpcResp), nil
}

// convertGRPCResponse 转换gRPC响应格式
func convertGRPCResponse(grpcResp *pb.DetectResponse) *DetectionResponse {
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

	return &DetectionResponse{
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
}

// ==================== 批量检测 ====================

// BatchDetect 发送批量检测请求
func (c *DetectionClient) BatchDetect(reqs []DetectionRequest) (*BatchDetectionResponse, error) {
	return c.batchDetectHTTP(reqs)
}

// ==================== 辅助方法 ====================

// Close 关闭客户端连接
func (c *DetectionClient) Close() error {
	if c.IsGRPC && c.GRPConn != nil {
		return c.GRPConn.Close()
	}
	return nil
}

// ToCallback 转换为回调结构
func (resp *DetectionResponse) ToCallback(streamPath, remoteAddr, pluginName string, publishId uuid.UUID) *CallbackDetection {
	return &CallbackDetection{
		Event:      "detection",
		StreamPath: streamPath,
		PublishId:  publishId,
		RemoteAddr: remoteAddr,
		Type:       "detection",
		PluginName: pluginName,
		Timestamp:  int(time.Now().UnixMilli()),
		Args: CallbackArgs{
			AlgorithmID:   resp.Data.AlgorithmID,
			AlgorithmName: resp.Data.AlgorithmName,
			Detections:    resp.Data.Detections,
			TotalCount:    resp.Data.TotalCount,
			DetectTime:    resp.Data.DetectTime,
		},
	}
}
