package hls

import (
	"context"
	"net/http"
	"sync"

	"m7s.live/v5"
	"m7s.live/v5/pkg/config"
	"m7s.live/v5/pkg/task"
)

type Puller struct {
	m7s.HTTPFilePuller
	Video       M3u8Info
	Audio       M3u8Info
	TsHead      http.Header     `json:"-" yaml:"-"` //用于提供cookie等特殊身份的http头
	SaveContext context.Context `json:"-" yaml:"-"` //用来保存ts文件到服务器
	memoryTs    sync.Map
}

func NewPuller(_ config.Pull) m7s.IPuller {
	p := &Puller{}
	p.SetDescription(task.OwnerTypeKey, "HLSPuller")
	return p
}

func (p *Puller) GetTs(key string) (any, bool) {
	return p.memoryTs.Load(key)
}

func (p *Puller) Run() (err error) {
	return
}
