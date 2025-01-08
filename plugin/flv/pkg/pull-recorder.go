package flv

import (
	"errors"
	"io"
	"os"
	"strings"
	"time"

	m7s "m7s.live/v5"
	"m7s.live/v5/pkg/config"
	"m7s.live/v5/pkg/task"
	"m7s.live/v5/pkg/util"
	rtmp "m7s.live/v5/plugin/rtmp/pkg"
)

type (
	RecordReader struct {
		m7s.RecordFilePuller
		reader *util.BufReader
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
		p.Type = "flv"
		p.SetDescription(task.OwnerTypeKey, "FlvRecordReader")
		return p
	}
	return nil
}

func (p *RecordReader) Run() (err error) {
	pullJob := &p.PullJob
	publisher := pullJob.Publisher
	allocator := util.NewScalableMemoryAllocator(1 << 10)
	var tagHeader [11]byte
	var ts, tsOffset int64
	var realTime time.Time
	defer allocator.Recycle()
	publisher.OnGetPosition = func() time.Time {
		return realTime
	}
	for loop := 0; loop < p.Loop; loop++ {
	nextStream:
		for i, stream := range p.Streams {
			tsOffset = ts
			if p.File != nil {
				p.File.Close()
			}
			p.File, err = os.Open(stream.FilePath)
			if err != nil {
				continue
			}
			p.reader = util.NewBufReader(p.File)
			var head util.Memory
			head, err = p.reader.ReadBytes(9)
			if err != nil {
				return
			}
			var flvHead [3]byte
			var version, flag byte
			err = head.NewReader().ReadByteTo(&flvHead[0], &flvHead[1], &flvHead[2], &version, &flag)
			if err != nil {
				return
			}
			if flvHead != [3]byte{'F', 'L', 'V'} {
				return errors.New("not flv file")
			}

			startTimestamp := int64(0)
			if i == 0 {
				startTimestamp = p.PullStartTime.Sub(stream.StartTime).Milliseconds()
				if startTimestamp < 0 {
					startTimestamp = 0
				}
			}

			for {
				if p.IsStopped() {
					return p.StopReason()
				}
				if publisher.Paused != nil {
					publisher.Paused.Await()
				}

				if needSeek, err := p.CheckSeek(); err != nil {
					continue
				} else if needSeek {
					goto nextStream
				}

				if _, err = p.reader.ReadBE(4); err != nil { // previous tag size
					return
				}
				// Read tag header (11 bytes total)
				if err = p.reader.ReadNto(11, tagHeader[:]); err != nil {
					return
				}

				t := tagHeader[0]                                                                      // tag type (1 byte)
				dataSize := int(tagHeader[1])<<16 | int(tagHeader[2])<<8 | int(tagHeader[3])           // data size (3 bytes)
				timestamp := uint32(tagHeader[4])<<16 | uint32(tagHeader[5])<<8 | uint32(tagHeader[6]) // timestamp (3 bytes)
				timestamp |= uint32(tagHeader[7]) << 24                                                // timestamp extended (1 byte)
				// stream id is tagHeader[8:11] (3 bytes), always 0

				var frame rtmp.RTMPData
				frame.SetAllocator(allocator)

				if err = p.reader.ReadNto(dataSize, frame.NextN(dataSize)); err != nil {
					return
				}
				ts = int64(timestamp) + tsOffset
				realTime = stream.StartTime.Add(time.Duration(timestamp) * time.Millisecond)
				if p.MaxTS > 0 && ts > p.MaxTS {
					return
				}

				frame.Timestamp = uint32(ts)
				switch t {
				case FLV_TAG_TYPE_AUDIO:
					if publisher.PubAudio {
						err = publisher.WriteAudio(frame.WrapAudio())
					}
				case FLV_TAG_TYPE_VIDEO:
					if publisher.PubVideo {
						err = publisher.WriteVideo(frame.WrapVideo())
					}
				case FLV_TAG_TYPE_SCRIPT:
					r := frame.NewReader()
					amf := &rtmp.AMF{
						Buffer: util.Buffer(r.ToBytes()),
					}
					frame.Recycle()
					var obj any
					if obj, err = amf.Unmarshal(); err != nil {
						return
					}
					name := obj
					if obj, err = amf.Unmarshal(); err != nil {
						return
					}
					if i == 0 {
						if metaData, ok := obj.(rtmp.EcmaArray); ok {
							if keyframes, ok := metaData["keyframes"].(map[string]any); ok {
								filepositions := keyframes["filepositions"].([]any)
								times := keyframes["times"].([]any)
								currentPos, err := p.File.Seek(0, io.SeekCurrent)
								for i, t := range times {
									if int64(t.(float64)*1000) >= startTimestamp {

										if err != nil {
											return err
										}
										if _, err = p.File.Seek(int64(filepositions[i].(float64)), io.SeekStart); err != nil {
											return err
										}
										tsOffset = -int64(t.(float64) * 1000)
										break
									}
								}
								if _, err = p.File.Seek(currentPos, io.SeekStart); err != nil {
									return err
								}
							}
						}
					} else {
						publisher.Info("script", name, obj)
					}
				}
				if err != nil {
					return
				}
			}
		}
	}
	return
}
