package plugin_detection

import (
	"context"
	"strings"
	"time"

	task "github.com/langhuihui/gotask"
	"m7s.live/v5"
	"m7s.live/v5/pkg/config"
	pb "m7s.live/v5/plugin/detection/pb"
	detection "m7s.live/v5/plugin/detection/pkg"
)

// 算法ID到名称的映射常量
var algorithmNames = map[uint8]string{
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
	13: "变化检测",
	14: "占道经营识别",
	15: "垃圾堆放识别",
	16: "裸土未覆盖识别",
	17: "建控区违建识别",
	18: "烟火识别",
	19: "光伏板缺陷检测",
	20: "园区夜间入侵检测",
	21: "园区外立面病害识别",
	22: "罂粟识别",
	23: "作物倒伏检测",
	24: "林业侵占",
}

// getAlgorithmName 根据算法ID获取算法名称
func (p *DetectionPlugin) getAlgorithmName(algorithmId uint8) string {
	if name, ok := algorithmNames[algorithmId]; ok {
		return name
	}
	return ""
}

// parseGrpcDuration 解析 gRPC 时间间隔字符串
func parseGrpcDuration(s string) time.Duration {
	if s == "" {
		return time.Second
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return time.Second
	}
	return d
}

// ==================== gRPC 服务实现 ====================

// LaunchDetection 启动图像检测
func (p *DetectionPlugin) LaunchDetection(ctx context.Context, req *pb.LaunchDetectionRequest) (*pb.LaunchDetectionResponse, error) {
	streamPath := req.StreamPath
	if streamPath == "" {
		return &pb.LaunchDetectionResponse{Code: -1, Message: "streamPath is required"}, nil
	}
	if len(req.Configurations) == 0 {
		return &pb.LaunchDetectionResponse{Code: -1, Message: "configurations is required"}, nil
	}

	publisher, err := p.Server.GetPublisher(streamPath)
	if err != nil {
		return &pb.LaunchDetectionResponse{Code: -1, Message: "stream not found"}, nil
	}

	// 停止已存在的 Transform
	if tm, ok := p.Server.Transforms.Get(streamPath); ok && tm != nil {
		tm.TransformJob.Stop(task.ErrTaskComplete)
	}

	// 创建输出配置
	var outputs []config.TransformOutput
	for _, cfg := range req.Configurations {
		conf := detection.SnapConfig{
			SnapMode:       int(cfg.SnapMode),
			TimeInterval:   parseGrpcDuration(cfg.TimerInterval),
			IFrameInterval: int(cfg.IframeInterval),
			AlgorithmId:    []uint8{uint8(cfg.AlgorithmId)},
			ConfThreshold:  []float32{cfg.Threshold},
			FrameCheck:     []int{int(cfg.FrameCheck)},
			IoUThreshold:   []float32{cfg.IouThreshold},
		}
		outputs = append(outputs, config.TransformOutput{
			Target:     streamPath,
			StreamPath: streamPath,
			Conf:       conf,
		})
	}

	// 创建新的 Transformer 实例
	trans := detection.NewTransform().(*detection.Transformer)
	trans.TransformJob.Init(trans, &p.Plugin, publisher, config.Transform{Output: outputs})

	// 设置Dispose回调
	trans.TransformJob.OnDispose(func() {
		closeData := m7s.AlarmInfo{
			StreamPath: streamPath,
			AlarmName:  detection.HookOnDetectionClose,
			AlarmDesc:  "manual_dispose",
		}
		if sender, webhook := p.GetHookSender(detection.HookOnDetectionClose); sender != nil {
			sender(webhook, closeData)
		}
	})
	publisher.OnDispose(func() {
		trans.TransformJob.Stop(task.ErrTaskComplete)
	})

	err = trans.WaitStarted()
	if err != nil {
		return &pb.LaunchDetectionResponse{Code: -1, Message: err.Error()}, nil
	}

	return &pb.LaunchDetectionResponse{Code: 0, Message: "success"}, nil
}

// DisposeDetection 停止图像检测
func (p *DetectionPlugin) DisposeDetection(ctx context.Context, req *pb.DisposeDetectionRequest) (*pb.DisposeDetectionResponse, error) {
	streamPath := req.StreamPath
	if streamPath == "" {
		return &pb.DisposeDetectionResponse{Code: -1, Message: "streamPath is required"}, nil
	}

	if tm, ok := p.Server.Transforms.Get(streamPath); ok && tm.TransformJob != nil {
		tm.TransformJob.Stop(task.ErrTaskComplete)
	}

	return &pb.DisposeDetectionResponse{Code: 0, Message: "success"}, nil
}

// ListDetections 列出所有检测配置
func (p *DetectionPlugin) ListDetections(ctx context.Context, req *pb.ListDetectionsRequest) (*pb.ListDetectionsResponse, error) {
	page := int(req.Page)
	pageSize := int(req.PageSize)
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	var items []*pb.DetectionConfigItem
	for _, transform := range p.Server.Transforms.Items {
		if req.StreamPath != "" && !strings.EqualFold(transform.StreamPath, req.StreamPath) {
			continue
		}

		if transform.TransformJob != nil && transform.TransformJob.Config.Output != nil {
			for _, output := range transform.TransformJob.Config.Output {
				if output.Conf == nil {
					continue
				}
				snapConfig := detection.SnapConfig{}
				switch v := output.Conf.(type) {
				case detection.SnapConfig:
					snapConfig = v
				case map[string]any:
					config.Parse(&snapConfig, v)
				}

				for i, algorithmId := range snapConfig.AlgorithmId {
					if len(req.AlgorithmId) > 0 {
						found := false
						for _, id := range req.AlgorithmId {
							if int32(algorithmId) == id {
								found = true
								break
							}
						}
						if !found {
							continue
						}
					}

					var threshold float32
					if i < len(snapConfig.ConfThreshold) {
						threshold = snapConfig.ConfThreshold[i]
					}
					var frameCheck int
					if i < len(snapConfig.FrameCheck) {
						frameCheck = snapConfig.FrameCheck[i]
					}
					var iouThreshold float32
					if i < len(snapConfig.IoUThreshold) {
						iouThreshold = snapConfig.IoUThreshold[i]
					}

					items = append(items, &pb.DetectionConfigItem{
						AlgorithmId:    int32(algorithmId),
						AlgorithmName:  p.getAlgorithmName(algorithmId),
						StreamPath:     transform.StreamPath,
						Threshold:      threshold,
						SnapMode:       int32(snapConfig.SnapMode),
						TimerInterval:  snapConfig.TimeInterval.String(),
						IframeInterval: int32(snapConfig.IFrameInterval),
						FrameCheck:     int32(frameCheck),
						IouThreshold:   iouThreshold,
					})
				}
			}
		}
	}

	total := len(items)
	startIndex := (page - 1) * pageSize
	endIndex := startIndex + pageSize
	if startIndex >= total {
		startIndex = total
	}
	if endIndex > total {
		endIndex = total
	}

	var pagedItems []*pb.DetectionConfigItem
	if startIndex < len(items) {
		pagedItems = items[startIndex:endIndex]
	} else {
		pagedItems = []*pb.DetectionConfigItem{}
	}

	return &pb.ListDetectionsResponse{
		Code:     0,
		Message:  "success",
		Total:    int32(total),
		Page:     int32(page),
		PageSize: int32(pageSize),
		Data:     pagedItems,
	}, nil
}

// GetDetection 获取单个检测配置
func (p *DetectionPlugin) GetDetection(ctx context.Context, req *pb.GetDetectionRequest) (*pb.GetDetectionResponse, error) {
	streamPath := req.StreamPath
	if streamPath == "" {
		return &pb.GetDetectionResponse{Code: -1, Message: "streamPath is required"}, nil
	}

	transform, ok := p.Server.Transforms.Get(streamPath)
	if !ok || transform.TransformJob == nil {
		return &pb.GetDetectionResponse{Code: -1, Message: "detection not found"}, nil
	}

	var items []*pb.DetectionConfigItem
	if transform.TransformJob.Config.Output != nil {
		for _, output := range transform.TransformJob.Config.Output {
			if output.Conf == nil {
				continue
			}
			snapConfig := detection.SnapConfig{}
			switch v := output.Conf.(type) {
			case detection.SnapConfig:
				snapConfig = v
			case map[string]any:
				config.Parse(&snapConfig, v)
			}

			for i, algorithmId := range snapConfig.AlgorithmId {
				var threshold float32
				if i < len(snapConfig.ConfThreshold) {
					threshold = snapConfig.ConfThreshold[i]
				}
				var frameCheck int
				if i < len(snapConfig.FrameCheck) {
					frameCheck = snapConfig.FrameCheck[i]
				}
				var iouThreshold float32
				if i < len(snapConfig.IoUThreshold) {
					iouThreshold = snapConfig.IoUThreshold[i]
				}

				items = append(items, &pb.DetectionConfigItem{
					AlgorithmId:    int32(algorithmId),
					AlgorithmName:  p.getAlgorithmName(algorithmId),
					StreamPath:     transform.StreamPath,
					Threshold:      threshold,
					SnapMode:       int32(snapConfig.SnapMode),
					TimerInterval:  snapConfig.TimeInterval.String(),
					IframeInterval: int32(snapConfig.IFrameInterval),
					FrameCheck:     int32(frameCheck),
					IouThreshold:   iouThreshold,
				})
			}
		}
	}

	return &pb.GetDetectionResponse{Code: 0, Message: "success", Data: items}, nil
}
