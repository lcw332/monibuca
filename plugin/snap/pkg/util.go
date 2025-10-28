package snap

import (
	"fmt"
	"io"
	"m7s.live/v5/plugin/snap/pkg/compress"
	"os/exec"

	m7s "m7s.live/v5"
	"m7s.live/v5/pkg"
	"m7s.live/v5/pkg/format"
)

// GetVideoFrame 获取视频帧数据
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

// ProcessWithFFmpeg 使用 FFmpeg 处理视频帧并生成截图
func ProcessWithFFmpeg(annexb []*format.AnnexB, output io.Writer) error {
	// 创建ffmpeg命令，使用select过滤器选择最后一帧
	cmd := exec.Command("ffmpeg", "-hide_banner", "-i", "pipe:0", "-vf", fmt.Sprintf("select='eq(n,%d)'", len(annexb)-1), "-vframes", "1", "-f", "mjpeg", "pipe:1")
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

// ImgCompressWithFFmpeg 使用 FFmpeg 处理截图，将截图进行压缩
func ImgCompressWithFFmpeg(input io.Reader, output io.Writer, config *SnapConfig) error {
	if config == nil {
		return fmt.Errorf("invalid config")
	}

	resolution := config.ImgCompress.Resolution
	var scaleWidth, scaleHeight int
	if resolution != "" {
		n, err := fmt.Sscanf(resolution, "%d:%d", &scaleWidth, &scaleHeight)
		if err != nil || n != 2 {
			return fmt.Errorf("invalid resolution format: %s", resolution)
		}
	} else {
		// 设置默认宽高以防万一
		scaleWidth, scaleHeight = 1920, 1080
	}

	// 图像缩放处理
	var vf string
	switch config.ImgCompress.Method {
	// 填充黑边
	case compress.LetterBox:
		vf = fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2:color=black,setsar=1",
			scaleWidth, scaleHeight, scaleWidth, scaleHeight)
	// 裁剪
	case compress.Crop:
		vf = fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d,setsar=1",
			scaleWidth, scaleHeight, scaleWidth, scaleHeight)
	// 拉伸
	case compress.Stretch:
		vf = fmt.Sprintf("scale=%d:%d,setsar=1", scaleWidth, scaleHeight)
	// 默认使用填充黑边
	default:
		vf = fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2:color=black,setsar=1",
			scaleWidth, scaleHeight, scaleWidth, scaleHeight)
	}

	// 创建ffmpeg命令，用于图像压缩
	cmd := exec.Command("ffmpeg",
		"-hide_banner",
		"-i",
		"pipe:0",
		"-vf",
		vf,
		"-q:v",
		fmt.Sprintf("%d", config.ImgCompress.Quality),
		"-f",
		"mjpeg",
		"pipe:1",
	)

	// 获取输入和输出pipe
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	defer stdin.Close()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	// 启动ffmpeg进程
	if err = cmd.Start(); err != nil {
		return err
	}

	// 并发写入输入数据并读取输出结果
	done := make(chan error, 1)
	go func() {
		_, err := io.Copy(stdin, input)
		stdin.Close() // 关闭 stdin 提醒子进程停止读取
		done <- err
	}()

	// 从ffmpeg的stdout读取压缩后的图片数据并写入到 output
	if _, err = io.Copy(output, stdout); err != nil {
		<-done // 等待写入协程结束
		return err
	}

	// 等待写入协程完成
	if err := <-done; err != nil {
		return err
	}

	// 等待ffmpeg进程结束
	return cmd.Wait()
}
