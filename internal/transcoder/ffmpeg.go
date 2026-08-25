package transcoder

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/binhy/vistack/internal/core"
	"go.uber.org/zap"
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
	{Height: 240, Bitrate: "500k", MaxRate: "600k", BufSize: "1200k", Profile: "baseline", Preset: "fast", CRF: "23"},
	{Height: 360, Bitrate: "1000k", MaxRate: "1200k", BufSize: "2400k", Profile: "main", Preset: "medium", CRF: "22"},
	{Height: 480, Bitrate: "2000k", MaxRate: "2500k", BufSize: "4000k", Profile: "main", Preset: "medium", CRF: "21"},
	{Height: 720, Bitrate: "4000k", MaxRate: "5000k", BufSize: "8000k", Profile: "high", Preset: "medium", CRF: "20"},
	{Height: 1080, Bitrate: "8000k", MaxRate: "10000k", BufSize: "16000k", Profile: "high", Preset: "slow", CRF: "18"},
	{Height: 1440, Bitrate: "16000k", MaxRate: "20000k", BufSize: "32000k", Profile: "high", Preset: "slow", CRF: "17"},
	{Height: 2160, Bitrate: "35000k", MaxRate: "45000k", BufSize: "70000k", Profile: "high", Preset: "slow", CRF: "16"},
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

	if stream.CodedWidth > 0 && stream.CodedHeight > 0 {
		return stream.CodedWidth, stream.CodedHeight, nil
	}
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

// ExtractVideoFrame 从视频中抽取一帧作为封面
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
func SelectAdaptiveQualities(srcWidth, srcHeight int) []DashQuality {
	if srcHeight <= 0 {
		return nil
	}

	if srcHeight < 360 {
		return []DashQuality{
			{Height: srcHeight, Bitrate: "800k", MaxRate: "1000k", BufSize: "2000k", Profile: "baseline", Preset: "medium", CRF: "23"},
		}
	}

	var candidates []DashQuality
	for _, q := range allQualities {
		if q.Height <= srcHeight {
			candidates = append(candidates, q)
		}
	}

	if len(candidates) == 0 {
		return []DashQuality{
			{Height: srcHeight, Bitrate: "2000k", MaxRate: "2500k", BufSize: "5000k", Profile: "main", Preset: "medium", CRF: "20"},
		}
	}

	var targetHeights []int
	switch {
	case srcHeight >= 2160:
		targetHeights = []int{480, 720, 1080, 1440, 2160}
	case srcHeight >= 1440:
		targetHeights = []int{480, 720, 1080, 1440}
	case srcHeight >= 1080:
		targetHeights = []int{360, 480, 720, 1080}
	case srcHeight >= 720:
		targetHeights = []int{360, 480, 720}
	case srcHeight >= 480:
		targetHeights = []int{240, 360, 480}
	default:
		targetHeights = []int{240, 360}
	}

	selected := filterQualities(candidates, targetHeights)

	if len(selected) == 0 {
		selected = candidates[len(candidates)-1:]
	}

	if len(selected) > 0 && selected[len(selected)-1].Height < srcHeight {
		srcInPreset := false
		for _, q := range allQualities {
			if q.Height == srcHeight {
				srcInPreset = true
				break
			}
		}
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

// ResolveQualities 根据请求的高度列表解析档位；heights 为空则自动选择。
func ResolveQualities(srcWidth, srcHeight int, heights []int32) []DashQuality {
	if len(heights) == 0 {
		return SelectAdaptiveQualities(srcWidth, srcHeight)
	}
	var result []DashQuality
	for _, h := range heights {
		for _, q := range allQualities {
			if q.Height == int(h) {
				result = append(result, q)
				break
			}
		}
	}
	if len(result) == 0 {
		return SelectAdaptiveQualities(srcWidth, srcHeight)
	}
	return result
}

// TranscodeToDASH 自适应转码为 MPEG-DASH；preferred 为空则自动选择档位
func TranscodeToDASH(inputPath, outputDirPath string, preferred []DashQuality) ([]DashQuality, error) {
	absInput, err := filepath.Abs(inputPath)
	if err != nil {
		return nil, fmt.Errorf("resolve input path failed: %w", err)
	}
	absOutputDir, err := filepath.Abs(outputDirPath)
	if err != nil {
		return nil, fmt.Errorf("resolve output dir failed: %w", err)
	}

	srcWidth, srcHeight, err := GetVideoResolution(absInput)
	if err != nil {
		return nil, fmt.Errorf("get video resolution failed: %w", err)
	}

	qualities := preferred
	if len(qualities) == 0 {
		qualities = SelectAdaptiveQualities(srcWidth, srcHeight)
	}
	if len(qualities) == 0 {
		return nil, fmt.Errorf("no suitable quality levels found for resolution %dx%d", srcWidth, srcHeight)
	}

	if core.Logger != nil {
		heights := make([]string, 0, len(qualities))
		for _, q := range qualities {
			heights = append(heights, fmt.Sprintf("%dp(%s)", q.Height, q.Bitrate))
		}
		core.Logger.Info("transcode plan",
			zap.Int("src_width", srcWidth),
			zap.Int("src_height", srcHeight),
			zap.Strings("qualities", heights),
		)
	}

	if err := os.MkdirAll(absOutputDir, 0755); err != nil {
		return nil, fmt.Errorf("create output dir failed: %w", err)
	}

	mpdFile := "manifest.mpd"

	args := []string{
		"-y",
		"-i", absInput,
		"-hide_banner",
	}

	for range qualities {
		args = append(args, "-map", "0:v:0")
	}
	args = append(args, "-map", "0:a:0?")

	for i, q := range qualities {
		var filterChain string
		if q.Height < srcHeight {
			targetWidth, exists := standard169Resolutions[q.Height]
			if !exists {
				targetWidth = int(float64(q.Height)*16.0/9.0 + 0.5)
				if targetWidth%2 != 0 {
					targetWidth++
				}
			}
			filterChain = fmt.Sprintf("scale=%d:%d:flags=lanczos,setsar=1,setdar=16/9", targetWidth, q.Height)
		} else {
			filterChain = "setsar=1,setdar=16/9"
		}

		args = append(args,
			fmt.Sprintf("-filter:v:%d", i), filterChain,
			fmt.Sprintf("-c:v:%d", i), "libx264",
			fmt.Sprintf("-profile:v:%d", i), q.Profile,
			fmt.Sprintf("-preset:v:%d", i), q.Preset,
			fmt.Sprintf("-crf:v:%d", i), q.CRF,
			fmt.Sprintf("-maxrate:v:%d", i), q.MaxRate,
			fmt.Sprintf("-bufsize:v:%d", i), q.BufSize,
			fmt.Sprintf("-pix_fmt:v:%d", i), "yuv420p",
		)
	}

	args = append(args,
		"-r", "30",
		"-vsync", "cfr",
		"-g", "120",
		"-keyint_min", "120",
		"-sc_threshold", "0",
	)

	args = append(args,
		"-c:a", "aac",
		"-b:a", "192k",
		"-ar", "48000",
		"-ac", "2",
	)

	args = append(args,
		"-f", "dash",
		"-seg_duration", "4",
		"-use_template", "1",
		"-use_timeline", "1",
		"-init_seg_name", "init-$RepresentationID$.m4s",
		"-media_seg_name", "chunk-$RepresentationID$-$Number%05d$.m4s",
		"-adaptation_sets", "id=0,streams=v id=1,streams=a",
		"-movflags", "+faststart",
		mpdFile,
	)

	cmd := exec.Command("ffmpeg", args...)
	cmd.Dir = absOutputDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if core.Logger != nil {
		core.Logger.Debug("run ffmpeg", zap.String("dir", absOutputDir), zap.String("cmd", strings.Join(args, " ")))
	}

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf(
			"ffmpeg transcode failed: %w\nstderr:\n%s",
			err,
			stderr.String(),
		)
	}

	if core.Logger != nil {
		core.Logger.Info("transcode completed", zap.String("output", filepath.Join(absOutputDir, mpdFile)))
	}

	return qualities, nil
}
