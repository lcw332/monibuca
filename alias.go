package m7s

import (
	"context"
	"net/url"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"
	"m7s.live/v5/pb"
)

type AliasStream struct {
	*Publisher `gorm:"-:all"`
	AutoRemove bool
	StreamPath string
	Alias      string
}

func (a *AliasStream) GetKey() string {
	return a.Alias
}

// StreamAliasDB 用于存储流别名的数据库模型
type StreamAliasDB struct {
	ID        uint           `gorm:"primarykey"`
	CreatedAt time.Time      `yaml:"-"`
	UpdatedAt time.Time      `yaml:"-"`
	DeletedAt gorm.DeletedAt `yaml:"-"`
	AliasStream
}

func (StreamAliasDB) TableName() string {
	return "stream_alias"
}

func (s *Server) initStreamAlias() {
	if s.DB == nil {
		return
	}
	var aliases []StreamAliasDB
	s.DB.Find(&aliases)
	for _, alias := range aliases {
		s.AliasStreams.Add(&alias.AliasStream)
		if publisher, ok := s.Streams.Get(alias.StreamPath); ok {
			alias.Publisher = publisher
		}
	}
}

func (s *Server) GetStreamAlias(ctx context.Context, req *emptypb.Empty) (res *pb.StreamAliasListResponse, err error) {
	res = &pb.StreamAliasListResponse{}
	s.Streams.Call(func() error {
		for alias := range s.AliasStreams.Range {
			info := &pb.StreamAlias{
				StreamPath: alias.StreamPath,
				Alias:      alias.Alias,
				AutoRemove: alias.AutoRemove,
			}
			if s.Streams.Has(alias.Alias) {
				info.Status = 2
			} else if alias.Publisher != nil {
				info.Status = 1
			}
			res.Data = append(res.Data, info)
		}
		return nil
	})
	return
}

func (s *Server) SetStreamAlias(ctx context.Context, req *pb.SetStreamAliasRequest) (res *pb.SuccessResponse, err error) {
	res = &pb.SuccessResponse{}
	s.Streams.Call(func() error {
		if req.StreamPath != "" {
			u, err := url.Parse(req.StreamPath)
			if err != nil {
				return err
			}
			req.StreamPath = strings.TrimPrefix(u.Path, "/")
			publisher, canReplace := s.Streams.Get(req.StreamPath)
			if !canReplace {
				defer s.OnSubscribe(req.StreamPath, u.Query())
			}
			if aliasInfo, ok := s.AliasStreams.Get(req.Alias); ok { //modify alias
				aliasInfo.AutoRemove = req.AutoRemove
				if aliasInfo.StreamPath != req.StreamPath {
					aliasInfo.StreamPath = req.StreamPath
					if canReplace {
						if aliasInfo.Publisher != nil {
							aliasInfo.TransferSubscribers(publisher) // replace stream
						} else {
							s.Waiting.WakeUp(req.Alias, publisher)
						}
					}
				}
				// 更新数据库中的别名
				if s.DB != nil {
					var dbAlias StreamAliasDB
					s.DB.Where("alias = ?", req.Alias).First(&dbAlias)
					dbAlias.StreamPath = req.StreamPath
					dbAlias.AutoRemove = req.AutoRemove
					s.DB.Save(&dbAlias)
				}
			} else { // create alias
				aliasInfo := &AliasStream{
					AutoRemove: req.AutoRemove,
					StreamPath: req.StreamPath,
					Alias:      req.Alias,
				}
				var pubId uint32
				s.AliasStreams.Add(aliasInfo)
				aliasStream, ok := s.Streams.Get(aliasInfo.Alias)
				if canReplace {
					aliasInfo.Publisher = publisher
					if ok {
						aliasStream.TransferSubscribers(publisher) // replace stream
					} else {
						s.Waiting.WakeUp(req.Alias, publisher)
					}
				} else {
					aliasInfo.Publisher = aliasStream
				}
				if aliasInfo.Publisher != nil {
					pubId = aliasInfo.Publisher.ID
				}
				// 保存到数据库
				if s.DB != nil {
					s.DB.Create(&StreamAliasDB{
						AliasStream: *aliasInfo,
					})
				}
				s.Info("add alias", "alias", req.Alias, "streamPath", req.StreamPath, "replace", ok && canReplace, "pub", pubId)
			}
		} else {
			s.Info("remove alias", "alias", req.Alias)
			if aliasStream, ok := s.AliasStreams.Get(req.Alias); ok {
				s.AliasStreams.Remove(aliasStream)
				// 从数据库中删除
				if s.DB != nil {
					s.DB.Where("alias = ?", req.Alias).Delete(&StreamAliasDB{})
				}
				if aliasStream.Publisher != nil {
					if publisher, hasTarget := s.Streams.Get(req.Alias); hasTarget { // restore stream
						aliasStream.TransferSubscribers(publisher)
					}
				} else {
					var args url.Values
					for sub := range aliasStream.Publisher.SubscriberRange {
						if sub.StreamPath == req.Alias {
							aliasStream.Publisher.RemoveSubscriber(sub)
							s.Waiting.Wait(sub)
							args = sub.Args
						}
					}
					if args != nil {
						s.OnSubscribe(req.Alias, args)
					}
				}
			}
		}
		return nil
	})
	return
}
