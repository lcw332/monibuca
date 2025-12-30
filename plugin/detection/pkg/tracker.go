package detection

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// ==================== 帧跟踪器 ====================

// trackedObject 跟踪的检测目标
type trackedObject struct {
	ClassName   string
	ClassNameCn string
	BBox        []float64 // [x1, y1, x2, y2]
	Confidence  float64
	Consecutive int   // 连续检测次数
	FirstSeen   int64 // 首次检测时间戳
	LastSeen    int64 // 最近检测时间戳
	Reported    bool  // 是否已上报过
}

// frameHistory 帧历史记录（按算法ID索引）
type frameHistory struct {
	mu        sync.RWMutex
	objects   map[string]*trackedObject // key: className_bboxSignature
	lastFrame int64
}

// TrackerResult 跟踪器处理结果
type TrackerResult struct {
	SameObjTotalCount int               // 连续检测达标的目标数量
	MarkedResults     []DetectionResult // 带标识的检测结果
}

// Tracker 帧跟踪器，用于连续帧检测
type Tracker struct {
	frameHistory map[uint8]*frameHistory
	mu           sync.RWMutex
	// 连续检测超时时间（毫秒），超过此时间未出现的目标会被彻底断开
	disconnectTimeout int64
}

// NewTracker 创建新的帧跟踪器
// disconnectTimeout: 目标断开超时时间（毫秒），默认3分钟
// 目标消失后在此时间内重新出现可以继续连续计数
// 超过此时间则彻底断开，再次出现视为新目标
func NewTracker(disconnectTimeout ...int64) *Tracker {
	timeout := int64(3 * 60 * 1000) // 默认3分钟
	if len(disconnectTimeout) > 0 {
		timeout = disconnectTimeout[0]
	}
	return &Tracker{
		frameHistory:      make(map[uint8]*frameHistory),
		disconnectTimeout: timeout,
	}
}

// getOrCreateHistory 获取或创建指定算法ID的帧历史记录
func (t *Tracker) getOrCreateHistory(algId uint8) *frameHistory {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.frameHistory[algId]; !exists {
		t.frameHistory[algId] = &frameHistory{
			objects:   make(map[string]*trackedObject),
			lastFrame: 0,
		}
	}
	return t.frameHistory[algId]
}

// getFrameCheck 获取指定算法的连续帧检测次数
func getFrameCheck(config SnapConfig, algorithmIndex int) int {
	if algorithmIndex < len(config.FrameCheck) {
		return config.FrameCheck[algorithmIndex]
	}
	if len(config.FrameCheck) > 0 {
		return config.FrameCheck[len(config.FrameCheck)-1]
	}
	return 0
}

// getFrameCheckForAlgId 根据算法ID获取连续帧检测次数
func getFrameCheckForAlgId(config SnapConfig, algId uint8) int {
	for i, id := range config.AlgorithmId {
		if id == algId {
			return getFrameCheck(config, i)
		}
	}
	return 0
}

// getIoUThreshold 获取指定算法的IoU阈值
func getIoUThreshold(config SnapConfig, algorithmIndex int) float64 {
	if algorithmIndex < len(config.IoUThreshold) {
		return float64(config.IoUThreshold[algorithmIndex])
	}
	if len(config.IoUThreshold) > 0 {
		return float64(config.IoUThreshold[len(config.IoUThreshold)-1])
	}
	return 0.5 // 默认值
}

// getIoUThresholdForAlgId 根据算法ID获取IoU阈值
func getIoUThresholdForAlgId(config SnapConfig, algId uint8) float64 {
	for i, id := range config.AlgorithmId {
		if id == algId {
			return getIoUThreshold(config, i)
		}
	}
	return 0.5 // 默认值
}

// BBox2 定义边界框 (x1, y1, x2, y2)
type BBox2 struct {
	X1, Y1, X2, Y2 float64
}

// newBBox2FromFloats 从数组创建BBox2
func newBBox2FromFloats(coords []float64) BBox2 {
	if len(coords) >= 4 {
		return BBox2{X1: coords[0], Y1: coords[1], X2: coords[2], Y2: coords[3]}
	}
	return BBox2{}
}

// calculateIoU 计算两个边界框的IoU (Intersection over Union)
func calculateIoU(box1, box2 BBox2) float64 {
	// 计算交集区域
	intersectX1 := math.Max(box1.X1, box2.X1)
	intersectY1 := math.Max(box1.Y1, box2.Y1)
	intersectX2 := math.Min(box1.X2, box2.X2)
	intersectY2 := math.Min(box1.Y2, box2.Y2)

	// 计算交集面积
	intersectArea := math.Max(0, intersectX2-intersectX1) * math.Max(0, intersectY2-intersectY1)

	// 计算各自面积
	area1 := (box1.X2 - box1.X1) * (box1.Y2 - box1.Y1)
	area2 := (box2.X2 - box2.X1) * (box2.Y2 - box2.Y1)

	// 计算并集面积
	unionArea := area1 + area2 - intersectArea

	if unionArea == 0 {
		return 0
	}

	return intersectArea / unionArea
}

// generateBBoxSignature 生成边界框的特征签名
func generateBBoxSignature(box BBox2) string {
	const precision = 1000
	x1 := int(box.X1 * precision)
	y1 := int(box.Y1 * precision)
	x2 := int(box.X2 * precision)
	y2 := int(box.Y2 * precision)
	return fmt.Sprintf("%d_%d_%d_%d", x1, y1, x2, y2)
}

// ProcessFrameCheck 处理连续帧检测逻辑
// 返回带标识的检测结果和连续检测达标的数量
func (t *Tracker) ProcessFrameCheck(results []*algorithmResult, config SnapConfig) map[uint8]*TrackerResult {
	now := time.Now().UnixMilli()
	trackerResults := make(map[uint8]*TrackerResult)

	for _, result := range results {
		frameCheck := getFrameCheckForAlgId(config, result.algId)
		iouThreshold := getIoUThresholdForAlgId(config, result.algId)

		history := t.getOrCreateHistory(result.algId)
		history.mu.Lock()
		history.lastFrame = now
		currentDetections := result.result.Data.Detections

		// 初始化结果
		trackerResults[result.algId] = &TrackerResult{
			MarkedResults: make([]DetectionResult, len(currentDetections)),
		}

		if len(currentDetections) == 0 {
			// 没有检测到目标，清空跟踪并返回空结果
			history.objects = make(map[string]*trackedObject)
			history.mu.Unlock()
			continue
		}

		seenInThisFrame := make(map[string]bool)

		// 遍历当前检测到的目标
		for i := range currentDetections {
			det := &currentDetections[i]
			box := newBBox2FromFloats(det.BBox)
			classKey := det.ClassName
			if det.ClassNameCn != "" {
				classKey = det.ClassNameCn
			}

			// 查找匹配的目标
			var matchedObj *trackedObject
			var matchedKey string

			for key, obj := range history.objects {
				if obj.ClassName != det.ClassName && obj.ClassNameCn != classKey {
					continue
				}

				// 如果目标已经断开太久（超过超时时间），视为新目标
				if now-obj.LastSeen > t.disconnectTimeout {
					continue
				}

				existingBox := newBBox2FromFloats(obj.BBox)
				iou := calculateIoU(existingBox, box)

				if iou >= iouThreshold {
					matchedObj = obj
					matchedKey = key
					break
				}
			}

			// 复制原始检测结果并添加标识
			markedDet := DetectionResult{
				ClassID:         det.ClassID,
				ClassName:       det.ClassName,
				ClassNameCn:     det.ClassNameCn,
				Confidence:      det.Confidence,
				BBox:            det.BBox,
				PlateNumber:     det.PlateNumber,
				PlateType:       det.PlateType,
				PlateConfidence: det.PlateConfidence,
			}

			if matchedObj != nil {
				// 找到匹配的目标，增加连续检测次数
				matchedObj.Consecutive++
				matchedObj.LastSeen = now
				matchedObj.Confidence = det.Confidence
				matchedObj.BBox = det.BBox
				seenInThisFrame[matchedKey] = true

				// 添加连续帧检测标识
				markedDet.ConsecutiveCount = matchedObj.Consecutive
				if matchedObj.Consecutive >= frameCheck && frameCheck > 0 {
					markedDet.IsSameObj = true
					trackerResults[result.algId].SameObjTotalCount++
				}
			} else {
				// 新目标，创建跟踪记录
				newKey := generateBBoxSignature(box)
				fullKey := classKey + "_" + newKey

				history.objects[fullKey] = &trackedObject{
					ClassName:   det.ClassName,
					ClassNameCn: det.ClassNameCn,
					BBox:        det.BBox,
					Confidence:  det.Confidence,
					Consecutive: 1,
					FirstSeen:   now,
					LastSeen:    now,
					Reported:    false,
				}
				seenInThisFrame[fullKey] = true

				// 新目标的连续检测次数为1
				markedDet.ConsecutiveCount = 1
			}

			trackerResults[result.algId].MarkedResults[i] = markedDet
		}

		// 清理消失的目标
		// - 只有超过 disconnectTimeout 才彻底删除
		// - 目标消失期间保留其连续计数，下次出现继续累加
		// - 这样短暂消失后重新出现可以保持连续计数
		var toDelete []string
		for key, obj := range history.objects {
			absentDuration := now - obj.LastSeen
			if absentDuration > t.disconnectTimeout {
				// 超过超时时间，彻底删除
				toDelete = append(toDelete, key)
			}
			// 注意：不重置 Consecutive，保留以便目标重新出现时继续累加
		}
		for _, key := range toDelete {
			delete(history.objects, key)
		}

		history.mu.Unlock()
	}

	return trackerResults
}

// ClearHistory 清空指定算法ID的帧历史记录
func (t *Tracker) ClearHistory(algId uint8) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if history, exists := t.frameHistory[algId]; exists {
		history.mu.Lock()
		history.objects = make(map[string]*trackedObject)
		history.mu.Unlock()
	}
}

// ClearAllHistory 清空所有帧历史记录
func (t *Tracker) ClearAllHistory() {
	t.mu.Lock()
	defer t.mu.Unlock()

	for algId := range t.frameHistory {
		if history, exists := t.frameHistory[algId]; exists {
			history.mu.Lock()
			history.objects = make(map[string]*trackedObject)
			history.mu.Unlock()
		}
	}
}

// GetTrackedObjectCount 获取指定算法ID的跟踪目标数量
func (t *Tracker) GetTrackedObjectCount(algId uint8) int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if history, exists := t.frameHistory[algId]; exists {
		history.mu.RLock()
		defer history.mu.RUnlock()
		return len(history.objects)
	}
	return 0
}
