package main

import (
	"context"
	"flag"
	"m7s.live/pro"
	_ "m7s.live/pro/plugin/cascade"
	_ "m7s.live/pro/plugin/console"
	_ "m7s.live/pro/plugin/debug"
	_ "m7s.live/pro/plugin/flv"
	_ "m7s.live/pro/plugin/logrotate"
	_ "m7s.live/pro/plugin/monitor"
	_ "m7s.live/pro/plugin/rtmp"
	_ "m7s.live/pro/plugin/rtsp"
	_ "m7s.live/pro/plugin/stress"
	_ "m7s.live/pro/plugin/webrtc"
)

func main() {
	ctx := context.Background()
	conf1 := flag.String("c1", "", "config1 file")
	conf2 := flag.String("c2", "", "config2 file")
	flag.Parse()
	go m7s.Run(ctx, *conf2)
	m7s.Run(ctx, *conf1)
}
