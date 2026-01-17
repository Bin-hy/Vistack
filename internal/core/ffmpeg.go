package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// DashQuality 定义 DASH ABR 档位
type DashQuality struct {
	Height  int
	Bitrate string
	MaxRate string
	BufSize string
	Profile string
	Preset  string
	CRF     string
}

// 标准 16:9 分辨率映射表（宽度精确符合 16:9，避免 SAR 调整）
var standard169Resolutions = map[int]int{
	240:  426,  // 426x240
	360:  640,  // 640x360
	480:  854,  // 854x480 (标准) 或 852 (严格 SAR=1)
	720:  1280, // 1280x720
	1080: 1920, // 1920x1080
	1440: 2560, // 2560x1440
	2160: 3840, // 3840x2160
}
var allQualities = []DashQuality{
	{
		Height:  240,
		Bitrate: "500k",
		MaxRate: "600k",
		BufSize: "1200k",
		Profile: "baseline",
		Preset:  "fast",
		CRF:     "23",
	},
	{
		Height:  360,
		Bitrate: "1000k",
		MaxRate: "1200k",
		BufSize: "2400k",
		Profile: "main",
		Preset:  "medium",
		CRF:     "22",
	},
	{
		Height:  480,
		Bitrate: "2000k",
		MaxRate: "2500k",
		BufSize: "4000k",
		Profile: "main",
		Preset:  "medium",
		CRF:     "21",
	},
	{
		Height:  720,
		Bitrate: "4000k",
		MaxRate: "5000k",
		BufSize: "8000k",
		Profile: "high",
		Preset:  "medium",
		CRF:     "20",
	},
	{
		Height:  1080,
		Bitrate: "8000k",
		MaxRate: "10000k",
		BufSize: "16000k",
		Profile: "high",
		Preset:  "slow",
		CRF:     "18",
	},
	{
		Height:  1440, // 2K
		Bitrate: "16000k",
		MaxRate: "20000k",
		BufSize: "32000k",
		Profile: "high",
		Preset:  "slow",
		CRF:     "17",
	},
	{
		Height:  2160, // 4K
		Bitrate: "35000k",
		MaxRate: "45000k",
		BufSize: "70000k",
		Profile: "high",
		Preset:  "slow",
		CRF:     "16",
	},
}

// VideoStream 视频流信息
type VideoStream struct {
	Width              int    `json:"width"`
	Height             int    `json:"height"`
	CodedWidth         int    `json:"coded_width"`
	CodedHeight        int    `json:"coded_height"`
	DisplayAspectRatio string `json:"display_aspect_ratio"`
	SampleAspectRatio  string `json:"sample_aspect_ratio"`
}

// ProbeResult ffprobe 返回结果
type ProbeResult struct {
	Streams []VideoStream `json:"streams"`
}

// GetVideoResolution 获取视频真实编码分辨率（优先使用 coded_width/height）
func GetVideoResolution(inputPath string) (width, height int, err error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height,coded_width,coded_height",
		"-of", "json",
		inputPath,
	)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return 0, 0, fmt.Errorf("ffprobe failed: %w", err)
	}

	var result ProbeResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return 0, 0, fmt.Errorf("parse ffprobe output failed: %w", err)
	}

	if len(result.Streams) == 0 {
		return 0, 0, fmt.Errorf("no video stream found")
	}

	stream := result.Streams[0]

	// 优先使用 coded_width/height（实际编码分辨率）
	if stream.CodedWidth > 0 && stream.CodedHeight > 0 {
		return stream.CodedWidth, stream.CodedHeight, nil
	}

	// 回退到 width/height
	if stream.Width > 0 && stream.Height > 0 {
		return stream.Width, stream.Height, nil
	}

	return 0, 0, fmt.Errorf("unable to determine video resolution")
}

// GetVideoDuration 获取视频时长（秒）
func GetVideoDuration(inputPath string) (float64, error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		inputPath,
	)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return 0, err
	}

	durationStr := strings.TrimSpace(stdout.String())
	duration, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		return 0, err
	}
	return duration, nil
}

func ExtractVideoFrame(inputPath, outputImagePath string, timeSeconds float64) error {
	absInput, err := filepath.Abs(inputPath)
	if err != nil {
		return fmt.Errorf("resolve input path failed: %w", err)
	}
	absOutput, err := filepath.Abs(outputImagePath)
	if err != nil {
		return fmt.Errorf("resolve output path failed: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absOutput), 0755); err != nil {
		return fmt.Errorf("create output dir failed: %w", err)
	}
	ts := "0"
	if timeSeconds > 0 {
		ts = fmt.Sprintf("%.2f", timeSeconds)
	}
	cmd := exec.Command("ffmpeg",
		"-y",
		"-ss", ts,
		"-i", absInput,
		"-frames:v", "1",
		"-q:v", "2",
		absOutput,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("extract video frame failed: %w\nstderr:\n%s", err, stderr.String())
	}
	return nil
}

// SelectAdaptiveQualities 根据原始视频分辨率选择合适的 ABR 档位
// 策略：
// - 永远不放大（不生成超过原始分辨率的档位）
// - 智能选择梯度档位，确保合理的带宽分布
// - 至少生成 2-4 个档位（如果源分辨率足够）
func SelectAdaptiveQualities(srcWidth, srcHeight int) []DashQuality {
	if srcHeight <= 0 {
		return nil
	}

	// 特殊处理：极低分辨率（< 360p）
	if srcHeight < 360 {
		return []DashQuality{
			{
				Height:  srcHeight,
				Bitrate: "800k",
				MaxRate: "1000k",
				BufSize: "2000k",
				Profile: "baseline",
				Preset:  "medium",
				CRF:     "23",
			},
		}
	}

	// 1. 找到所有小于等于源分辨率的候选档位
	var candidates []DashQuality
	for _, q := range allQualities {
		if q.Height <= srcHeight {
			candidates = append(candidates, q)
		}
	}

	if len(candidates) == 0 {
		// 如果没有合适的预设档位，创建自定义档位
		return []DashQuality{
			{
				Height:  srcHeight,
				Bitrate: "2000k",
				MaxRate: "2500k",
				BufSize: "5000k",
				Profile: "main",
				Preset:  "medium",
				CRF:     "20",
			},
		}
	}

	// 2. 根据源分辨率智能选择目标档位
	var targetHeights []int
	switch {
	case srcHeight >= 2160: // 4K
		targetHeights = []int{480, 720, 1080, 1440, 2160}
	case srcHeight >= 1440: // 2K
		targetHeights = []int{480, 720, 1080, 1440}
	case srcHeight >= 1080: // 1080p
		targetHeights = []int{360, 480, 720, 1080}
	case srcHeight >= 720: // 720p
		targetHeights = []int{360, 480, 720}
	case srcHeight >= 480: // 480p
		targetHeights = []int{240, 360, 480}
	default: // 360p
		targetHeights = []int{240, 360}
	}

	// 3. 从候选档位中筛选目标档位
	selected := filterQualities(candidates, targetHeights)

	// 4. 确保至少有一个档位
	if len(selected) == 0 {
		selected = candidates[len(candidates)-1:]
	}

	// 5. 如果最高档位不是源分辨率，且源分辨率不在预设中，添加源分辨率档位
	if len(selected) > 0 && selected[len(selected)-1].Height < srcHeight {
		// 检查源分辨率是否在预设中
		srcInPreset := false
		for _, q := range allQualities {
			if q.Height == srcHeight {
				srcInPreset = true
				break
			}
		}

		// 如果不在预设中，基于最接近的档位创建源分辨率档位
		if !srcInPreset {
			baseQuality := selected[len(selected)-1]
			selected = append(selected, DashQuality{
				Height:  srcHeight,
				Bitrate: baseQuality.Bitrate,
				MaxRate: baseQuality.MaxRate,
				BufSize: baseQuality.BufSize,
				Profile: baseQuality.Profile,
				Preset:  baseQuality.Preset,
				CRF:     baseQuality.CRF,
			})
		}
	}

	return selected
}

// filterQualities 从候选列表中筛选指定高度的档位
func filterQualities(candidates []DashQuality, heights []int) []DashQuality {
	var result []DashQuality
	for _, h := range heights {
		for _, q := range candidates {
			if q.Height == h {
				result = append(result, q)
				break
			}
		}
	}
	return result
}

// TranscodeToDASH 自适应转码为 MPEG-DASH
func TranscodeToDASH(inputPath, outputDirPath string) ([]DashQuality, error) {
	// 1. 规范化路径
	absInput, err := filepath.Abs(inputPath)
	if err != nil {
		return nil, fmt.Errorf("resolve input path failed: %w", err)
	}
	absOutputDir, err := filepath.Abs(outputDirPath)
	if err != nil {
		return nil, fmt.Errorf("resolve output dir failed: %w", err)
	}

	// 2. 获取原始视频分辨率
	srcWidth, srcHeight, err := GetVideoResolution(absInput)
	if err != nil {
		return nil, fmt.Errorf("get video resolution failed: %w", err)
	}

	// 3. 根据源分辨率智能选择 ABR 档位
	qualities := SelectAdaptiveQualities(srcWidth, srcHeight)

	if len(qualities) == 0 {
		return nil, fmt.Errorf("no suitable quality levels found for resolution %dx%d", srcWidth, srcHeight)
	}

	// 打印选择的档位（用于调试）
	fmt.Printf("Source Resolution: %dx%d\n", srcWidth, srcHeight)
	fmt.Printf("Selected Qualities: ")
	for i, q := range qualities {
		if i > 0 {
			fmt.Print(", ")
		}
		fmt.Printf("%dp(%s)", q.Height, q.Bitrate)
	}
	fmt.Println()

	// 4. 创建输出目录
	if err := os.MkdirAll(absOutputDir, 0755); err != nil {
		return nil, fmt.Errorf("create output dir failed: %w", err)
	}

	mpdFile := "manifest.mpd"

	// 5. 构建 FFmpeg 参数
	args := []string{
		"-y",
		"-i", absInput,
		"-hide_banner",
	}

	// 6. 视频流映射
	for range qualities {
		args = append(args, "-map", "0:v:0")
	}
	args = append(args, "-map", "0:a:0?")

	// 7. 视频编码参数（逐档配置）
	for i, q := range qualities {
		// 缩放滤镜（必须在编码参数之前设置）
		var filterChain string
		if q.Height < srcHeight {
			// 优先使用标准 16:9 分辨率
			targetWidth, exists := standard169Resolutions[q.Height]
			if !exists {
				// 如果不在标准表中，动态计算
				targetWidth = int(float64(q.Height)*16.0/9.0 + 0.5)
				if targetWidth%2 != 0 {
					targetWidth++ // 确保偶数
				}
			}
			// 添加 setdar=16/9 强制显示宽高比为 16:9
			filterChain = fmt.Sprintf("scale=%d:%d:flags=lanczos,setsar=1,setdar=16/9", targetWidth, q.Height)
		} else {
			// 不需要缩放：设置 SAR=1:1 和 DAR=16:9
			filterChain = "setsar=1,setdar=16/9"
		}

		args = append(args,
			// 视频滤镜（必须在 -c:v 之前）
			fmt.Sprintf("-filter:v:%d", i), filterChain,

			// 编码器和质量控制
			fmt.Sprintf("-c:v:%d", i), "libx264",
			fmt.Sprintf("-profile:v:%d", i), q.Profile,
			fmt.Sprintf("-preset:v:%d", i), q.Preset,

			// 码率控制（VBR 模式：使用 CRF + 码率上限）
			fmt.Sprintf("-crf:v:%d", i), q.CRF,
			fmt.Sprintf("-maxrate:v:%d", i), q.MaxRate,
			fmt.Sprintf("-bufsize:v:%d", i), q.BufSize,

			// 像素格式
			fmt.Sprintf("-pix_fmt:v:%d", i), "yuv420p",
		)
	}

	// 8. GOP / 时间轴对齐（DASH 无缝切换的关键）
	args = append(args,
		"-r", "30", // 强制 30fps
		"-vsync", "cfr", // 恒定帧率
		"-g", "120", // GOP 大小 = 4秒 (30fps × 4)
		"-keyint_min", "120", // 最小关键帧间隔
		"-sc_threshold", "0", // 禁用场景切换检测
	)

	// 9. 音频统一转码
	args = append(args,
		"-c:a", "aac",
		"-b:a", "192k",
		"-ar", "48000",
		"-ac", "2",
	)

	// 10. DASH 封装参数（VOD 模式）
	args = append(args,
		"-f", "dash",
		"-seg_duration", "4", // 分片时长 4 秒
		"-use_template", "1", // 使用模板
		"-use_timeline", "1", // 使用时间轴
		"-init_seg_name", "init-$RepresentationID$.m4s", // 初始化分片命名
		"-media_seg_name", "chunk-$RepresentationID$-$Number%05d$.m4s", // 媒体分片命名
		"-adaptation_sets", "id=0,streams=v id=1,streams=a", // 适配集配置
		"-movflags", "+faststart", // 快速启动
		mpdFile,
	)

	// 11. 执行 FFmpeg
	cmd := exec.Command("ffmpeg", args...)
	cmd.Dir = absOutputDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// 打印完整命令（用于调试）
	fmt.Println("\nFFmpeg Command:")
	fmt.Printf("cd %s && ffmpeg %s\n\n", absOutputDir, strings.Join(args, " \\\n  "))

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf(
			"ffmpeg transcode failed: %w\nstderr:\n%s",
			err,
			stderr.String(),
		)
	}

	fmt.Printf("\n✓ Transcode completed successfully\n")
	fmt.Printf("Output: %s/%s\n", absOutputDir, mpdFile)

	return qualities, nil
}
