package plugin_hls

import (
	"m7s.live/v5"
	"m7s.live/v5/plugin/hls/pkg"
)

var _ = m7s.InstallPlugin[HLSPlugin](hls.NewPuller)

type HLSPlugin struct {
	m7s.Plugin
}
