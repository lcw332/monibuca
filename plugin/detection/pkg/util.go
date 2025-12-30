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

// ==================== 视频帧处理 ====================

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

// ==================== 图像编解码 ====================

// SnapFrameToBase64WithFFmpeg 使用FFmpeg将视频帧处理为Base64编码的图片
func SnapFrameToBase64WithFFmpeg(buf []byte) string {
	return base64.StdEncoding.EncodeToString(buf)
}

// SnapFrameWithFFmpeg 使用 FFmpeg 处理视频帧并生成截图
func SnapFrameWithFFmpeg(annexb []*format.AnnexB, output io.Writer, format string) (ImgInfo, error) {
	outputFileFormat := getOutputFormat(format)

	cmd := exec.Command(
		"ffmpeg",
		"-hide_banner",
		"-i", "pipe:0",
		"-vf", fmt.Sprintf("select='eq(n,%d)'", len(annexb)-1),
		"-vframes", "1",
		"-f", outputFileFormat,
		"pipe:1",
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return ImgInfo{}, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return ImgInfo{}, err
	}

	if err = cmd.Start(); err != nil {
		return ImgInfo{}, err
	}

	for _, annex := range annexb {
		if _, err = annex.WriteTo(stdin); err != nil {
			stdin.Close()
			return ImgInfo{}, err
		}
	}
	stdin.Close()

	var buf bytes.Buffer
	tee := io.TeeReader(stdout, output)
	_, err = io.Copy(&buf, tee)
	if err != nil {
		return ImgInfo{}, err
	}

	if err = cmd.Wait(); err != nil {
		return ImgInfo{}, err
	}

	imgData := buf.Bytes()
	imgInfo := ImgInfo{
		Size: int64(len(imgData)),
	}

	// 获取图片尺寸信息
	probeCmd := exec.Command(
		"ffprobe",
		"-hide_banner",
		"-v", "error",
		"-show_entries", "stream=width,height",
		"-of", "default=nw=1",
		"pipe:0",
	)

	probeStdin, _ := probeCmd.StdinPipe()
	probeStdout, _ := probeCmd.StdoutPipe()

	if err = probeCmd.Start(); err != nil {
		return imgInfo, nil
	}

	go func() {
		defer probeStdin.Close()
		probeStdin.Write(imgData)
	}()

	var probeOutput bytes.Buffer
	io.Copy(&probeOutput, probeStdout)
	probeCmd.Wait()

	parseProbeOutput(&imgInfo, probeOutput.String())

	return imgInfo, nil
}

// getOutputFormat 根据格式字符串返回FFmpeg输出格式
func getOutputFormat(format string) string {
	switch strings.ToLower(format) {
	case "png":
		return "png"
	default:
		return "mjpeg"
	}
}

// parseProbeOutput 解析ffprobe输出
func parseProbeOutput(imgInfo *ImgInfo, output string) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "width=") {
			fmt.Sscanf(line, "width=%d", &imgInfo.Width)
		} else if strings.HasPrefix(line, "height=") {
			fmt.Sscanf(line, "height=%d", &imgInfo.Height)
		}
	}
}

// ==================== 检测框绘制 ====================

// DrawDetectionBBox 在图像上绘制检测框和标签
func DrawDetectionBBox(imgBytes []byte, imgInfo *ImgInfo, format string, bbox BBox, label string, confidence float64, fontPath string, fontSize uint8, fontColor string) ([]byte, error) {
	outputFileFormat := getOutputFormat(format)

	// 计算基于图像实际尺寸的像素坐标
	x := bbox.X * float64(imgInfo.Width)
	y := bbox.Y * float64(imgInfo.Height)
	w := (bbox.W - bbox.X) * float64(imgInfo.Width)
	h := (bbox.H - bbox.Y) * float64(imgInfo.Height)

	hasFontSupport := checkFontSupport() && fontPath != ""
	filter := buildDrawFilter(x, y, w, h, label, confidence, fontPath, fontSize, fontColor, hasFontSupport)

	return runFFmpegDraw(imgBytes, filter, outputFileFormat)
}

// buildDrawFilter 构建FFmpeg绘制滤镜
func buildDrawFilter(x, y, w, h float64, label string, confidence float64, fontPath string, fontSize uint8, fontColor string, hasFontSupport bool) string {
	if hasFontSupport {
		escapedLabel := strings.ReplaceAll(label, "'", "\\'")
		escapedLabel = strings.ReplaceAll(escapedLabel, ":", "\\:")
		labelText := fmt.Sprintf("%s %.2f", escapedLabel, confidence)
		return fmt.Sprintf("drawbox=x=%f:y=%f:w=%f:h=%f:color=red:thickness=2,drawtext=fontfile='%s':text='%s':x=%f:y=%f:fontsize=%d:fontcolor=%s",
			x, y, w, h, fontPath, labelText, x, y-30, fontSize, fontColor)
	}
	return fmt.Sprintf("drawbox=x=%f:y=%f:w=%f:h=%f:color=red:thickness=2", x, y, w, h)
}

// runFFmpegDraw 执行FFmpeg绘制命令
func runFFmpegDraw(imgBytes []byte, filter, outputFormat string) ([]byte, error) {
	cmd := exec.Command(
		"ffmpeg",
		"-hide_banner",
		"-i", "pipe:0",
		"-vf", filter,
		"-q:v", "2",
		"-f", outputFormat,
		"pipe:1",
	)

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

	if err = cmd.Start(); err != nil {
		return nil, err
	}

	if _, err = stdin.Write(imgBytes); err != nil {
		stdin.Close()
		return nil, err
	}
	stdin.Close()

	var buf bytes.Buffer
	var errBuf bytes.Buffer

	done := make(chan error, 2)
	go func() {
		_, err := io.Copy(&buf, stdout)
		done <- err
	}()
	go func() {
		_, err := io.Copy(&errBuf, stderr)
		done <- err
	}()

	<-done
	<-done

	if err = cmd.Wait(); err != nil {
		return nil, fmt.Errorf("ffmpeg error: %v, stderr: %s", err, errBuf.String())
	}

	if buf.Len() == 0 {
		return nil, fmt.Errorf("ffmpeg produced no output")
	}

	return buf.Bytes(), nil
}

// checkFontSupport 检查系统字体支持
func checkFontSupport() bool {
	cmd := exec.Command("ffmpeg", "-hide_banner", "-filters")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), "drawtext")
}
