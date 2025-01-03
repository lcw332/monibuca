package flv

import (
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	m7s "m7s.live/v5"
	"m7s.live/v5/pkg"
	"m7s.live/v5/pkg/config"
	"m7s.live/v5/pkg/task"
	"m7s.live/v5/pkg/util"
	rtmp "m7s.live/v5/plugin/rtmp/pkg"
)

type (
	RecordReader struct {
		m7s.RecordFilePuller
		reader *FlvReader
	}
)

func NewPuller(conf config.Pull) m7s.IPuller {
	if strings.HasPrefix(conf.URL, "http") || strings.HasSuffix(conf.URL, ".flv") {
		p := &Puller{}
		p.SetDescription(task.OwnerTypeKey, "FlvPuller")
		return p
	}
	if conf.Args.Get(util.StartKey) != "" {
		p := &RecordReader{}
		p.SetDescription(task.OwnerTypeKey, "FlvRecordReader")
		return p
	}
	return nil
}

func (p *RecordReader) Run() (err error) {
	pullJob := &p.PullJob
	pullStartTime := p.PullStartTime
	publisher := pullJob.Publisher
	allocator := util.NewScalableMemoryAllocator(1 << 10)
	var ts, tsOffset int64
	var realTime time.Time
	defer allocator.Recycle()
	publisher.OnSeek = func(seekTime time.Time) {
		pullStartTime = seekTime
		p.SetRetry(1, 0)
		if util.UnixTimeReg.MatchString(pullJob.Args.Get(util.EndKey)) {
			pullJob.Args.Set(util.StartKey, strconv.FormatInt(pullStartTime.Unix(), 10))
		} else {
			pullJob.Args.Set(util.StartKey, pullStartTime.Local().Format(util.LocalTimeFormat))
		}
		publisher.Stop(pkg.ErrSeek)
	}
	publisher.OnGetPosition = func() time.Time {
		return realTime
	}
	for i, stream := range p.Streams {
		tsOffset = ts
		if p.File != nil {
			p.File.Close()
		}
		p.File, err = os.Open(stream.FilePath)
		if err != nil {
			continue
		}
		p.reader = NewFlvReader(p.File)
		if err = p.reader.ReadHeader(); err != nil {
			return
		}
		if i == 0 {
			startTimestamp := pullStartTime.Sub(stream.StartTime).Milliseconds()
			if startTimestamp < 0 {
				startTimestamp = 0
			}
			var metaData rtmp.EcmaArray
			if metaData, err = ReadMetaData(p.File); err != nil {
				return
			}
			if keyframes, ok := metaData["keyframes"].(map[string]any); ok {
				filepositions := keyframes["filepositions"].([]float64)
				times := keyframes["times"].([]float64)
				for i, t := range times {
					if int64(t*1000) >= startTimestamp {
						if _, err = p.File.Seek(int64(filepositions[i]), io.SeekStart); err != nil {
							return
						}
						tsOffset = -int64(t * 1000)
						break
					}
				}
			}
		}

		for {
			if p.IsStopped() {
				return p.StopReason()
			}
			if publisher.Paused != nil {
				publisher.Paused.Await()
			}

			var tag *Tag
			if tag, err = p.reader.ReadTag(); err != nil {
				if err == io.EOF {
					err = nil
					break
				}
				return
			}

			ts = int64(tag.Timestamp) + tsOffset
			realTime = stream.StartTime.Add(time.Duration(tag.Timestamp) * time.Millisecond)
			if p.MaxTS > 0 && ts > p.MaxTS {
				return
			}

			switch tag.Type {
			case FLV_TAG_TYPE_AUDIO:
				var audioFrame rtmp.RTMPAudio
				audioFrame.SetAllocator(allocator)
				audioFrame.Timestamp = uint32(ts)
				audioFrame.AddRecycleBytes(tag.Data[:len(tag.Data)-4]) // remove previous tag size
				err = publisher.WriteAudio(&audioFrame)
			case FLV_TAG_TYPE_VIDEO:
				var videoFrame rtmp.RTMPVideo
				videoFrame.SetAllocator(allocator)
				videoFrame.Timestamp = uint32(ts)
				videoFrame.AddRecycleBytes(tag.Data[:len(tag.Data)-4]) // remove previous tag size
				err = publisher.WriteVideo(&videoFrame)
			}
			if err != nil {
				return
			}
		}
	}
	return
}
