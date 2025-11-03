package detection

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"io"
	"os/exec"

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
func SnapFrameWithFFmpeg(annexb []*format.AnnexB, output io.Writer) error {
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
		"mjpeg",
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

// DrawBoundingBox 在图像上绘制检测框和标签
func DrawBoundingBox(imgBytes []byte, bbox BBox, label string, confidence float64) ([]byte, error) {
	// 解码图像
	img, _, err := image.Decode(bytes.NewReader(imgBytes))
	if err != nil {
		return nil, fmt.Errorf("decode image failed: %w", err)
	}

	// 创建画布
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	// 转换归一化坐标为像素坐标
	x := int(bbox.X * float64(width))
	y := int(bbox.Y * float64(height))
	w := int(bbox.W * float64(width))
	h := int(bbox.H * float64(height))

	// 创建新图像（RGBA）
	newImg := image.NewRGBA(bounds)
	draw.Draw(newImg, bounds, img, image.Point{}, draw.Src)

	// 设置颜色和字体
	red := color.RGBA{255, 0, 0, 255}
	white := color.RGBA{255, 255, 255, 255}
	//green := color.RGBA{0, 255, 0, 255}

	// 绘制矩形框
	drawRectangle(newImg, x, y, x+w, y+h, red)

	// 绘制标签文字（使用默认字体）
	labelText := fmt.Sprintf("%s %.2f", label, confidence)
	drawText(newImg, x, y-10, labelText, white)

	// 编码为 PNG 字节
	var buf bytes.Buffer
	err = jpeg.Encode(&buf, newImg, &jpeg.Options{
		Quality: 80,
	})
	if err != nil {
		return nil, fmt.Errorf("encode image failed: %w", err)
	}

	return buf.Bytes(), nil
}

func drawRectangle(img *image.RGBA, x1, y1, x2, y2 int, c color.Color) {
	for i := x1; i < x2; i++ {
		img.Set(i, y1, c)
		img.Set(i, y2-1, c)
	}
	for j := y1; j < y2; j++ {
		img.Set(x1, j, c)
		img.Set(x2-1, j, c)
	}
}

func drawText(img *image.RGBA, x, y int, text string, c color.Color) {
	// 简化：仅支持单行文本，无字体支持
	// 实际项目中建议使用 `golang.org/x/image/font` 支持真字体
	// 此处仅示意
	// 可替换为更复杂的字体渲染逻辑
}
