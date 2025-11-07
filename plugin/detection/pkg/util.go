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

type ImgInfo struct {
	Size   int64
	Width  int
	Height int
}

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
func SnapFrameWithFFmpeg(annexb []*format.AnnexB, output io.Writer, format string) (ImgInfo, error) {
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
		return ImgInfo{}, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return ImgInfo{}, err
	}

	// 启动ffmpeg进程
	if err = cmd.Start(); err != nil {
		return ImgInfo{}, err
	}

	// 将annexb数据写入到ffmpeg的stdin
	for _, annex := range annexb {
		if _, err = annex.WriteTo(stdin); err != nil {
			stdin.Close()
			return ImgInfo{}, err
		}
	}
	stdin.Close()

	// 从ffmpeg的stdout读取图片数据并写入到输出
	var buf bytes.Buffer
	tee := io.TeeReader(stdout, output)
	_, err = io.Copy(&buf, tee)
	if err != nil {
		return ImgInfo{}, err
	}

	// 等待ffmpeg进程结束
	if err = cmd.Wait(); err != nil {
		return ImgInfo{}, err
	}

	// 获取图片信息
	imgData := buf.Bytes()
	imgInfo := ImgInfo{
		Size: int64(len(imgData)),
	}

	// 使用ffprobe获取更多图片信息
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
		return imgInfo, nil // 如果ffprobe失败，至少返回已知信息
	}

	go func() {
		defer probeStdin.Close()
		probeStdin.Write(imgData)
	}()

	var probeOutput bytes.Buffer
	io.Copy(&probeOutput, probeStdout)
	probeCmd.Wait()

	// 解析ffprobe输出
	lines := strings.Split(probeOutput.String(), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "width=") {
			fmt.Sscanf(line, "width=%d", &imgInfo.Width)
		} else if strings.HasPrefix(line, "height=") {
			fmt.Sscanf(line, "height=%d", &imgInfo.Height)
		}
	}

	return imgInfo, nil
}

// DrawDetectionBBox 在图像上绘制检测框和标签
func DrawDetectionBBox(imgBytes []byte, imgInfo *ImgInfo, format string, bbox BBox, label string, confidence float64, fontPath string, fontSize uint8, fontColor string) ([]byte, error) {

	var outputFileFormat string
	switch strings.ToLower(format) {
	case "png":
		outputFileFormat = "png"
	default: // 默认为JPEG格式
		outputFileFormat = "mjpeg"
	}

	// 计算基于图像实际尺寸的像素坐标
	x := bbox.X * float64(imgInfo.Width)
	y := bbox.Y * float64(imgInfo.Height)
	w := (bbox.W - bbox.X) * float64(imgInfo.Width)
	h := (bbox.H - bbox.Y) * float64(imgInfo.Height)

	// 检查系统字体可用性
	hasFontSupport := checkFontSupport() && fontPath != ""

	var filter string
	if hasFontSupport {
		// 转义文本中的特殊字符
		escapedLabel := strings.ReplaceAll(label, "'", "\\'")
		escapedLabel = strings.ReplaceAll(escapedLabel, ":", "\\:")
		labelText := fmt.Sprintf("%s %.2f", escapedLabel, confidence)

		filter = fmt.Sprintf("drawbox=x=%f:y=%f:w=%f:h=%f:color=red:thickness=2,drawtext=fontfile='%s':text='%s':x=%f:y=%f:fontsize=%d:fontcolor=%s",
			x, y, w, h, fontPath, labelText, x, y-30, fontSize, fontColor)
	} else {
		// 仅绘制边框，不添加文本
		filter = fmt.Sprintf("drawbox=x=%f:y=%f:w=%f:h=%f:color=red:thickness=2", x, y, w, h)
	}

	cmd := exec.Command(
		"ffmpeg",
		"-hide_banner",
		"-i", "pipe:0",
		"-vf", filter,
		"-q:v", "2",
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

	// 检查是否有输出数据
	if buf.Len() == 0 {
		return nil, fmt.Errorf("ffmpeg produced no output")
	}

	return buf.Bytes(), nil
}

// checkFontSupport 检查系统字体支持
func checkFontSupport() bool {
	// 简单检查字体支持，可以通过执行简单命令测试
	cmd := exec.Command("ffmpeg", "-hide_banner", "-filters")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// 检查输出中是否包含drawtext过滤器
	return strings.Contains(string(output), "drawtext")
}
