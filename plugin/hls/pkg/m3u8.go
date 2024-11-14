package hls

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/url"
	"sync"

	"github.com/quangngotan95/go-m3u8/m3u8"
)

type M3u8Info struct {
	Req       *http.Request
	M3U8Count int    //一共拉取的m3u8文件数量
	TSCount   int    //一共拉取的ts文件数量
	LastM3u8  string //最后一个m3u8文件内容
}

type TSDownloader struct {
	client *http.Client
	url    *url.URL
	req    *http.Request
	res    *http.Response
	wg     sync.WaitGroup
	err    error
	dur    float64
}

func (p *TSDownloader) Start() {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		if tsRes, err := p.client.Do(p.req); err == nil {
			p.res = tsRes
		} else {
			p.err = err
		}
	}()
}

func readM3U8(res *http.Response) (playlist *m3u8.Playlist, err error) {
	var reader io.Reader = res.Body
	if res.Header.Get("Content-Encoding") == "gzip" {
		reader, err = gzip.NewReader(reader)
	}
	if err == nil {
		playlist, err = m3u8.Read(reader)
	}
	if err != nil {
		// HLSPlugin.Error("readM3U8", zap.Error(err))
	}
	return
}
