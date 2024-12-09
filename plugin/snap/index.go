package plugin_snap

import (
	m7s "m7s.live/v5"
)

var _ = m7s.InstallPlugin[SnapPlugin]()

type SnapPlugin struct {
	//pb.UnimplementedApiServer
	m7s.Plugin
	LogToFile string
}
