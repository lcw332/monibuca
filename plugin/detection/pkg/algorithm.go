package pkg

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

// AlgorithmMap 定义算法映射关系
var AlgorithmMap = map[int]string{
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
	AlgorithmID   int     `json:"algorithm_id"`
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
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		AlgorithmID   int               `json:"algorithm_id"`
		AlgorithmName string            `json:"algorithm_name"`
		Detections    []DetectionResult `json:"detections"`
		TotalCount    int               `json:"total_count"`
		DetectTime    float64           `json:"detect_time"`
	} `json:"data"`
}

// DetectionClient 封装检测客户端
type DetectionClient struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// NewDetectionClient 创建新的检测客户端
func NewDetectionClient(baseURL, apiKey string) *DetectionClient {
	return &DetectionClient{
		BaseURL: baseURL,
		APIKey:  apiKey,
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
	httpReq, err := http.NewRequest("POST", c.BaseURL+"/api/v1/detect", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// 设置请求头
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-Key", c.APIKey)

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

// ImageToBase64 将图片文件转换为base64编码（无前缀）
func ImageToBase64(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return "", err
	}

	// 检查文件大小
	if fileInfo.Size() > 2*1024*1024 { // 2MB
		return "", fmt.Errorf("image file too large: %d bytes", fileInfo.Size())
	}

	// 读取文件内容
	fileBytes, err := ioutil.ReadAll(file)
	if err != nil {
		return "", err
	}

	// 转换为base64
	encoded := base64.StdEncoding.EncodeToString(fileBytes)
	return encoded, nil
}
