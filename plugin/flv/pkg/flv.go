package flv

import (
	"bufio"
	"io"
	"net"

	"m7s.live/v5/pkg/util"
	rtmp "m7s.live/v5/plugin/rtmp/pkg"
)

const (
	// FLV Tag Type
	FLV_TAG_TYPE_AUDIO  = 0x08
	FLV_TAG_TYPE_VIDEO  = 0x09
	FLV_TAG_TYPE_SCRIPT = 0x12
)

var FLVHead = []byte{'F', 'L', 'V', 0x01, 0x05, 0, 0, 0, 9, 0, 0, 0, 0}

type Tag struct {
	Type      byte
	Data      []byte
	Timestamp uint32
}

type FlvReader struct {
	reader io.Reader
	buf    [11]byte
}

func NewFlvReader(r io.Reader) *FlvReader {
	return &FlvReader{reader: r}
}

func (r *FlvReader) ReadHeader() (err error) {
	var header [13]byte
	if _, err = io.ReadFull(r.reader, header[:]); err != nil {
		return
	}
	if header[0] != 'F' || header[1] != 'L' || header[2] != 'V' {
		return io.ErrUnexpectedEOF
	}
	return
}

func (r *FlvReader) ReadTag() (tag *Tag, err error) {
	tag = &Tag{}
	if _, err = io.ReadFull(r.reader, r.buf[:]); err != nil {
		return
	}
	tmp := util.Buffer(r.buf[:])
	tag.Type = tmp.ReadByte()
	dataSize := tmp.ReadUint24()
	tag.Timestamp = tmp.ReadUint24() | (uint32(tmp.ReadByte()) << 24)
	tmp.ReadUint24() // streamID always 0

	tag.Data = make([]byte, dataSize+4) // +4 for previous tag size
	if _, err = io.ReadFull(r.reader, tag.Data); err != nil {
		return
	}
	return
}

type FlvWriter struct {
	io.Writer
	buf [15]byte
}

func NewFlvWriter(w io.Writer) *FlvWriter {
	return &FlvWriter{Writer: w}
}

func (w *FlvWriter) WriteHeader(hasAudio, hasVideo bool) (err error) {
	var flags byte
	if hasAudio {
		flags |= 0x04
	}
	if hasVideo {
		flags |= 0x01
	}
	_, err = w.Write([]byte{'F', 'L', 'V', 0x01, flags, 0, 0, 0, 9, 0, 0, 0, 0})
	return
}

func (w *FlvWriter) WriteTag(t byte, ts, dataSize uint32, payload ...[]byte) (err error) {
	WriteFLVTagHead(t, ts, dataSize, w.buf[:])
	var buffers net.Buffers = append(append(net.Buffers{w.buf[:11]}, payload...), util.PutBE(w.buf[11:], dataSize+11))
	_, err = buffers.WriteTo(w)
	return
}

func PutFlvTimestamp(header []byte, timestamp uint32) {
	header[4] = byte(timestamp >> 16)
	header[5] = byte(timestamp >> 8)
	header[6] = byte(timestamp)
	header[7] = byte(timestamp >> 24)
}

func WriteFLVTagHead(t uint8, ts, dataSize uint32, b []byte) {
	b[0] = t
	b[1], b[2], b[3] = byte(dataSize>>16), byte(dataSize>>8), byte(dataSize)
	PutFlvTimestamp(b, ts)
}

func ReadMetaData(reader io.Reader) (metaData rtmp.EcmaArray, err error) {
	r := bufio.NewReader(reader)
	_, err = r.Discard(13)
	tagHead := make(util.Buffer, 11)
	_, err = io.ReadFull(r, tagHead)
	if err != nil {
		return
	}
	tmp := tagHead
	t := tmp.ReadByte()
	dataLen := tmp.ReadUint24()
	_, err = r.Discard(4)
	if t == FLV_TAG_TYPE_SCRIPT {
		data := make([]byte, dataLen+4)
		_, err = io.ReadFull(reader, data)
		amf := &rtmp.AMF{
			Buffer: util.Buffer(data[1+2+len("onMetaData") : len(data)-4]),
		}
		var obj any
		obj, err = amf.Unmarshal()
		metaData = obj.(rtmp.EcmaArray)
	}
	return
}
