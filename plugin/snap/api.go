package plugin_snap

import (
	"io"
	"net/http"

	"m7s.live/v5/pkg"
	"m7s.live/v5/pkg/config"
	snap "m7s.live/v5/plugin/snap/pkg"
)

func (t *SnapPlugin) doSnap(rw http.ResponseWriter, r *http.Request) {
	streamPath := r.PathValue("streamPath")
	targetStreamPath := streamPath

	ok := t.Server.Streams.Has(streamPath)
	if !ok {
		http.Error(rw, pkg.ErrNotFound.Error(), http.StatusNotFound)
		return
	}

	pr, pw := io.Pipe()

	flusher, ok := rw.(http.Flusher)
	if !ok {
		http.Error(rw, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	rw.Header().Set("Content-Type", "image/jpeg")
	rw.Header().Set("Transfer-Encoding", "chunked")
	rw.Header().Del("Content-Length")

	done := make(chan error, 1)

	go func() {
		defer pw.Close()
		defer pr.Close()
		defer close(done)

		buf := make([]byte, 32*1024)
		_, err := io.CopyBuffer(rw, pr, buf)
		if err != nil {
			t.Error("write response error", err)
			done <- err
			return
		}
		flusher.Flush()
		done <- nil
	}()

	var transformer *snap.Transformer
	if tm, ok := t.Server.Transforms.Get(targetStreamPath); ok {
		transformer, ok = tm.TransformJob.Transformer.(*snap.Transformer)
		if !ok {
			http.Error(rw, "not a snap transformer", http.StatusInternalServerError)
			return
		}
	} else {
		transformer = snap.NewTransform().(*snap.Transformer)
		transformer.TransformJob.Init(transformer, &t.Plugin, streamPath, config.Transform{
			Output: []config.TransfromOutput{
				{
					Target:     targetStreamPath,
					StreamPath: targetStreamPath,
					Conf:       pw,
				},
			},
		}).WaitStarted()
	}

	transformer.TriggerSnap()

	if err := <-done; err != nil {
		t.Error("snapshot failed", err)
		return
	}
}

func (config *SnapPlugin) RegisterHandler() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"/{streamPath...}": config.doSnap,
	}
}
