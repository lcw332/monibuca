package plugin_webrtc

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	. "github.com/pion/webrtc/v3"
	"m7s.live/v5/pkg/task"
	. "m7s.live/v5/plugin/webrtc/pkg"
)

// https://datatracker.ietf.org/doc/html/draft-ietf-wish-whip
func (conf *WebRTCPlugin) servePush(w http.ResponseWriter, r *http.Request) {
	streamPath := r.PathValue("streamPath")
	rawQuery := r.URL.RawQuery
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		auth = auth[len("Bearer "):]
		if rawQuery != "" {
			rawQuery += "&bearer=" + auth
		} else {
			rawQuery = "bearer=" + auth
		}
		conf.Info("push", "stream", streamPath, "bearer", auth)
	}
	w.Header().Set("Content-Type", "application/sdp")
	w.Header().Set("Location", "/webrtc/api/stop/push/"+streamPath)
	w.Header().Set("Access-Control-Allow-Private-Network", "true")
	if rawQuery != "" {
		streamPath += "?" + rawQuery
	}
	bytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	conn := Connection{
		PLI: conf.PLI,
		SDP: string(bytes),
	}
	conn.Logger = conf.Logger
	if conn.PeerConnection, err = conf.api.NewPeerConnection(Configuration{
		ICEServers: conf.ICEServers,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if conn.Publisher, err = conf.Publish(conf.Context, streamPath); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	conn.Publisher.RemoteAddr = r.RemoteAddr
	conf.AddTask(&conn)
	if err = conn.WaitStarted(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := conn.SetRemoteDescription(SessionDescription{Type: SDPTypeOffer, SDP: conn.SDP}); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if answer, err := conn.GetAnswer(); err == nil {
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, answer.SDP)
	} else {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}

func (conf *WebRTCPlugin) servePlay(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/sdp")
	streamPath := r.PathValue("streamPath")
	rawQuery := r.URL.RawQuery
	var conn Connection
	conn.EnableDC = conf.EnableDC
	bytes, err := io.ReadAll(r.Body)
	defer func() {
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
	}()
	if err != nil {
		return
	}
	conn.SDP = string(bytes)
	if conn.PeerConnection, err = conf.api.NewPeerConnection(Configuration{
		ICEServers: conf.ICEServers,
	}); err != nil {
		return
	}
	if rawQuery != "" {
		streamPath += "?" + rawQuery
	}
	if conn.Subscriber, err = conf.Subscribe(conn.Context, streamPath); err != nil {
		return
	}
	conn.Subscriber.RemoteAddr = r.RemoteAddr
	conf.AddTask(&conn)
	if err = conn.WaitStarted(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err = conn.SetRemoteDescription(SessionDescription{Type: SDPTypeOffer, SDP: conn.SDP}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if sdp, err := conn.GetAnswer(); err == nil {
		w.Write([]byte(sdp.SDP))
	} else {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
}

// Batch 通过单个 PeerConnection 实现多个流的推拉
func (conf *WebRTCPlugin) Batch(w http.ResponseWriter, r *http.Request) {
	conn := NewSingleConnection()
	conn.EnableDC = true // Enable DataChannel for signaling
	conn.Logger = conf.Logger
	bytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	conn.SDP = string(bytes)
	if conn.PeerConnection, err = conf.api.NewPeerConnection(Configuration{
		ICEServers: conf.ICEServers,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Create data channel for signaling
	dataChannel, err := conn.PeerConnection.CreateDataChannel("signal", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	dataChannel.OnMessage(func(msg DataChannelMessage) {
		var signal Signal
		if err := json.Unmarshal(msg.Data, &signal); err != nil {
			conf.Error("failed to unmarshal signal", "error", err)
			return
		}

		switch signal.Type {
		case SignalTypePublish:
			if publisher, err := conf.Publish(conf.Context, signal.StreamPath); err == nil {
				conn.Publisher = publisher
				conn.Publisher.RemoteAddr = r.RemoteAddr
				conn.Receive()
				// Renegotiate SDP after successful publish
				if answer, err := conn.GetAnswer(); err == nil {
					dataChannel.SendText(NewAnswerSingal(answer.SDP))
				} else {
					dataChannel.SendText(NewErrorSignal(err.Error(), signal.StreamPath))
				}
			} else {
				dataChannel.SendText(NewErrorSignal(err.Error(), signal.StreamPath))
			}
		case SignalTypeSubscribe:
			if err := conn.SetRemoteDescription(SessionDescription{
				Type: SDPTypeOffer,
				SDP:  signal.Offer,
			}); err != nil {
				dataChannel.SendText(NewErrorSignal("Failed to set remote description: "+err.Error(), ""))
				return
			}
			// First remove subscribers that are not in the new list
			for streamPath := range conn.Subscribers {
				found := false
				for _, newPath := range signal.StreamList {
					if streamPath == newPath {
						found = true
						break
					}
				}
				if !found {
					conn.RemoveSubscriber(streamPath)
				}
			}
			// Then add new subscribers
			for _, streamPath := range signal.StreamList {
				// Skip if already subscribed
				if conn.HasSubscriber(streamPath) {
					continue
				}
				if subscriber, err := conf.Subscribe(conf.Context, streamPath); err == nil {
					subscriber.RemoteAddr = r.RemoteAddr
					conn.AddSubscriber(streamPath, subscriber)
				} else {
					dataChannel.SendText(NewErrorSignal(err.Error(), streamPath))
				}
			}
		case SignalTypeUnpublish:
			// Handle stream removal
			if conn.Publisher != nil && conn.Publisher.StreamPath == signal.StreamPath {
				conn.Publisher.Stop(task.ErrStopByUser)
				conn.Publisher = nil
				// Renegotiate SDP after unpublish
				if answer, err := conn.GetAnswer(); err == nil {
					dataChannel.SendText(NewAnswerSingal(answer.SDP))
				} else {
					dataChannel.SendText(NewErrorSignal(err.Error(), signal.StreamPath))
				}
			}
		case SignalTypeAnswer:
			// Handle received answer from browser
			if err := conn.SetRemoteDescription(SessionDescription{
				Type: SDPTypeAnswer,
				SDP:  signal.Answer,
			}); err != nil {
				dataChannel.SendText(NewErrorSignal("Failed to set remote description: "+err.Error(), ""))
			}
		}
	})

	conf.AddTask(conn)
	if err = conn.WaitStarted(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err = conn.SetRemoteDescription(SessionDescription{Type: SDPTypeOffer, SDP: conn.SDP}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if answer, err := conn.GetAnswer(); err == nil {
		w.Header().Set("Content-Type", "application/sdp")
		w.Write([]byte(answer.SDP))
	} else {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
}
