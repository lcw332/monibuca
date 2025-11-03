package detection

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"m7s.live/v5"
	"m7s.live/v5/pkg"
	"m7s.live/v5/pkg/format"
)

type BBox struct {
	X, Y, W, H float64
}

// FloatsToBBox float 数组转 bbox
func FloatsToBBox(values []float64) BBox {
	if len(values) < 4 {
		return BBox{}
	}
	return BBox{
		X: values[0],
		Y: values[1],
		W: values[2],
		H: values[3],
	}
}

// GetVideoFrame 获取当前视频帧
func GetVideoFrame(publisher *m7s.Publisher, server *m7s.Server) ([]*format.AnnexB, error) {
	if publisher.VideoTrack.AVTrack == nil {
		return nil, pkg.ErrNotFound
	}

	// 等待视频就绪
	if err := publisher.VideoTrack.WaitReady(); err != nil {
		return nil, err
	}

	// 创建读取器并等待 I 帧
	reader := pkg.NewAVRingReader(publisher.VideoTrack.AVTrack, "snapshot")
	if err := reader.StartRead(publisher.VideoTrack.GetIDR()); err != nil {
		return nil, err
	}
	defer reader.StopRead()

	var annexbList []*format.AnnexB

	for lastFrameSequence := publisher.VideoTrack.AVTrack.LastValue.Sequence; reader.Value.Sequence <= lastFrameSequence; reader.ReadNext() {
		var annexb format.AnnexB
		annexb.ICodecCtx = reader.Value.GetBase()
		err := pkg.ConvertFrameType(reader.Value.Wraps[0], &annexb)
		if err != nil {
			return nil, err
		}
		annexbList = append(annexbList, &annexb)
	}
	return annexbList, nil
}

// SnapFrameToBase64WithFFmpeg 使用FFmpeg将视频帧处理为Base64编码的图片
func SnapFrameToBase64WithFFmpeg(buf []byte) string {
	return base64.StdEncoding.EncodeToString(buf)
}

// SnapFrameWithFFmpeg 使用 FFmpeg 处理视频帧并生成截图
func SnapFrameWithFFmpeg(annexb []*format.AnnexB, output io.Writer, format string) error {
	// 根据format参数确定输出格式
	var outputFileFormat string
	switch strings.ToLower(format) {
	case "png":
		outputFileFormat = "png"
	default: // 默认为JPEG格式
		outputFileFormat = "mjpeg"
	}

	// 创建ffmpeg命令，使用select过滤器选择最后一帧
	cmd := exec.Command(
		"ffmpeg",
		"-hide_banner",
		"-i",
		"pipe:0",
		"-vf",
		fmt.Sprintf("select='eq(n,%d)'", len(annexb)-1),
		"-vframes",
		"1",
		"-f",
		outputFileFormat,
		"pipe:1",
	)

	// 获取输入和输出pipe
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	// 启动ffmpeg进程
	if err = cmd.Start(); err != nil {
		return err
	}

	// 将annexb数据写入到ffmpeg的stdin
	for _, annex := range annexb {
		if _, err = annex.WriteTo(stdin); err != nil {
			stdin.Close()
			return err
		}
	}
	stdin.Close()

	// 从ffmpeg的stdout读取图片数据并写入到输出
	if _, err = io.Copy(output, stdout); err != nil {
		return err
	}

	// 等待ffmpeg进程结束
	return cmd.Wait()
}

// DrawDetectionBBox 在图像上绘制检测框和标签
func DrawDetectionBBox(imgBytes []byte, format string, bbox BBox, label string, confidence float64) ([]byte, error) {
	// 转义文本中的特殊字符
	escapedLabel := strings.ReplaceAll(label, "'", "\\'")
	escapedLabel = strings.ReplaceAll(escapedLabel, ":", "\\:")

	// 根据format参数确定输出格式
	var outputFileFormat string
	var qualityOption string
	switch strings.ToLower(format) {
	case "png":
		outputFileFormat = "png"
		qualityOption = "-q:v" // PNG是无损格式，使用质量参数控制压缩
	default: // 默认为JPEG格式
		outputFileFormat = "mjpeg"
		qualityOption = "-q:v"
	}

	cmd := exec.Command(
		"ffmpeg",
		"-hide_banner",
		"-i", "pipe:0",
		"-vf", fmt.Sprintf("drawbox=x=%f*iw:y=%f*ih:w=%f*iw:h=%f*ih:color=red:thickness=2",
			bbox.X, bbox.Y, bbox.W, bbox.H),
		qualityOption, "2", // JPEG质量或PNG压缩级别
		"-f", outputFileFormat,
		"pipe:1",
	)

	// 获取输入和输出pipe
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	// 启动ffmpeg进程
	if err = cmd.Start(); err != nil {
		return nil, err
	}

	// 将图像数据写入到ffmpeg的stdin
	if _, err = stdin.Write(imgBytes); err != nil {
		stdin.Close()
		return nil, err
	}
	stdin.Close()

	// 从ffmpeg的stdout读取处理后的图像数据
	var buf bytes.Buffer
	var errBuf bytes.Buffer

	// 并行读取stdout和stderr
	done := make(chan error, 2)
	go func() {
		_, err := io.Copy(&buf, stdout)
		done <- err
	}()
	go func() {
		_, err := io.Copy(&errBuf, stderr)
		done <- err
	}()

	// 等待复制完成
	<-done
	<-done

	// 等待ffmpeg进程结束
	if err = cmd.Wait(); err != nil {
		return nil, fmt.Errorf("ffmpeg error: %v, stderr: %s", err, errBuf.String())
	}

	return buf.Bytes(), nil
}
