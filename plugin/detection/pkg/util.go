package pkg

import (
	"m7s.live/v5"
	"m7s.live/v5/pkg"
	"m7s.live/v5/pkg/format"
)

// 获取视频帧
func GetVideoFrame(publisher *m7s.Publisher, server *m7s.Server) ([]*format.AnnexB, error) {
	if publisher.VideoTrack.AVTrack == nil {
		return nil, pkg.ErrNotFound
	}

	// 等待视频就绪
	if err := publisher.VideoTrack.WaitReady(); err != nil {
		return nil, err
	}

	// 创建读取器并等待 I 帧
	reader := pkg.NewAVRingReader(publisher.VideoTrack.AVTrack, "snapshot")
	if err := reader.StartRead(publisher.VideoTrack.GetIDR()); err != nil {
		return nil, err
	}
	defer reader.StopRead()

	var annexbList []*format.AnnexB

	for lastFrameSequence := publisher.VideoTrack.AVTrack.LastValue.Sequence; reader.Value.Sequence <= lastFrameSequence; reader.ReadNext() {
		var annexb format.AnnexB
		annexb.ICodecCtx = reader.Value.GetBase()
		err := pkg.ConvertFrameType(reader.Value.Wraps[0], &annexb)
		if err != nil {
			return nil, err
		}
		annexbList = append(annexbList, &annexb)
	}
	return annexbList, nil
}
