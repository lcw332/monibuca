package plugin_detection

import "net/http"

func (p *DetectionPlugin) RegisterHandler() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		//"/{streamPath...}":               p.doSnap,
		//"/query/{streamPath...}":         p.querySnap,
		//"/batch/{streamPath...}":         p.batchSnap,
		//"/batchplayback/{streamPath...}": p.batchPlayBack,
	}
}
