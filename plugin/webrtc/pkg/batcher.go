package webrtc

import "encoding/json"

type SignalType string

const (
	SignalTypeSubscribe SignalType = "subscribe"
	SignalTypePublish   SignalType = "publish"
	SignalTypeUnpublish SignalType = "unpublish"
	SignalTypeAnswer    SignalType = "answer"
)

type Signal struct {
	Type       SignalType `json:"type"`
	StreamList []string   `json:"streamList"`
	Offer      string     `json:"offer"`
	Answer     string     `json:"answer"`
	StreamPath string     `json:"streamPath"`
}

type SignalStreamPath struct {
	Type       string `json:"type"`
	StreamPath string `json:"streamPath"`
}

func NewRemoveSingal(streamPath string) string {
	s := SignalStreamPath{
		Type:       "remove",
		StreamPath: streamPath,
	}
	b, _ := json.Marshal(s)
	return string(b)
}

type SignalSDP struct {
	Type string `json:"type"`
	SDP  string `json:"sdp"`
}

func NewAnswerSingal(sdp string) string {
	s := SignalSDP{
		Type: "answer",
		SDP:  sdp,
	}
	b, _ := json.Marshal(s)
	return string(b)
}

type SignalError struct {
	Type       string `json:"type"`
	Message    string `json:"message"`
	StreamPath string `json:"streamPath,omitempty"`
}

func NewErrorSignal(message string, streamPath string) string {
	s := SignalError{
		Type:       "error",
		Message:    message,
		StreamPath: streamPath,
	}
	b, _ := json.Marshal(s)
	return string(b)
}
